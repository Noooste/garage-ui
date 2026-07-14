// Command seed bulk-loads a Garage/S3 bucket with millions of small objects
// for testing the object browser and search features.
//
// Keys are a deterministic function of a global index, so a run is resumable:
// re-run with -start=<last index> and any re-uploaded key simply overwrites.
// Layout: pets/<species>/<breed>/<name>-<NNNNNN>.dat
//
//	cd backend && go run ./cmd/seed              # full 3,000,000 objects
//	go run ./cmd/seed -count 10000               # quick smoke test
//	go run ./cmd/seed -start 1500000             # resume from index 1.5M
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// species -> breeds. Real words so keys are searchable by substring
// (e.g. "golden", "retriever", "siamese"). Flattened into speciesBreed at init.
var taxonomy = []struct {
	species string
	breeds  []string
}{
	{"dogs", []string{"labrador", "golden-retriever", "german-shepherd", "bulldog", "poodle", "beagle", "rottweiler", "dachshund", "husky", "chihuahua"}},
	{"cats", []string{"siamese", "persian", "maine-coon", "bengal", "ragdoll", "sphynx", "british-shorthair", "abyssinian"}},
	{"birds", []string{"parrot", "canary", "cockatiel", "budgie", "finch", "macaw", "lovebird"}},
	{"fish", []string{"goldfish", "guppy", "betta", "angelfish", "tetra", "molly"}},
	{"rabbits", []string{"holland-lop", "netherland-dwarf", "rex", "lionhead", "flemish-giant"}},
	{"hamsters", []string{"syrian", "dwarf-campbell", "roborovski", "chinese"}},
	{"reptiles", []string{"leopard-gecko", "iguana", "bearded-dragon", "corn-snake", "box-turtle"}},
	{"horses", []string{"arabian", "thoroughbred", "mustang", "clydesdale", "appaloosa"}},
	{"guinea-pigs", []string{"american", "abyssinian", "peruvian", "silkie"}},
	{"ferrets", []string{"sable", "albino", "cinnamon", "chocolate"}},
}

// petNames are the leaf file names. ~50 common pet names.
var petNames = []string{
	"buddy", "luna", "max", "bella", "charlie", "lucy", "cooper", "daisy", "rocky", "molly",
	"bailey", "sadie", "duke", "maggie", "bear", "sophie", "tucker", "chloe", "oliver", "lola",
	"jack", "zoe", "toby", "ruby", "teddy", "rosie", "milo", "gracie", "oscar", "coco",
	"leo", "penny", "rex", "willow", "sam", "honey", "gus", "ginger", "murphy", "olive",
	"jasper", "hazel", "finn", "ivy", "louie", "pepper", "ziggy", "nala", "apollo", "cleo",
}

type speciesBreedPair struct{ species, breed string }

var speciesBreed []speciesBreedPair

func init() {
	for _, t := range taxonomy {
		for _, b := range t.breeds {
			speciesBreed = append(speciesBreed, speciesBreedPair{t.species, b})
		}
	}
}

// keyFor maps a global index to a unique object key with even folder fill.
// folderIdx selects the species/breed folder; within selects (name, suffix)
// inside that folder. The mapping is a bijection, so keys never collide.
func keyFor(i int64) string {
	c := int64(len(speciesBreed))
	n := int64(len(petNames))
	folderIdx := i % c
	within := i / c
	name := petNames[within%n]
	suffix := within / n
	sb := speciesBreed[folderIdx]
	return fmt.Sprintf("pets/%s/%s/%s-%06d.dat", sb.species, sb.breed, name, suffix)
}

func main() {
	var (
		endpoint    = flag.String("endpoint", "localhost:3900", "S3 endpoint host:port")
		bucket      = flag.String("bucket", "test", "target bucket")
		region      = flag.String("region", "garage", "S3 region")
		accessKey   = flag.String("access-key", "GK4b706791e6efb7bc00a99c69", "S3 access key")
		secretKey   = flag.String("secret-key", "cdb665539872887e4fca34841ad2ebd79cda7af2302b500097262ec030123b14", "S3 secret key")
		count       = flag.Int64("count", 3_000_000, "total dataset size (upper index, exclusive)")
		start       = flag.Int64("start", 0, "start index (resume point)")
		size        = flag.Int("size", 4096, "bytes per object")
		concurrency = flag.Int("concurrency", 64, "concurrent upload workers")
		secure      = flag.Bool("secure", false, "use HTTPS")
	)
	flag.Parse()

	client, err := minio.New(*endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(*accessKey, *secretKey, ""),
		Secure:       *secure,
		Region:       *region,
		BucketLookup: minio.BucketLookupPath, // Garage needs path-style
	})
	if err != nil {
		log.Fatalf("client init: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	total := *count - *start
	if total <= 0 {
		log.Fatalf("nothing to do: start=%d >= count=%d", *start, *count)
	}
	log.Printf("seeding bucket %q: indices [%d,%d) = %d objects of %d bytes, concurrency=%d, folders=%d",
		*bucket, *start, *count, total, *size, *concurrency, len(speciesBreed))

	var (
		cursor    = *start
		done      int64
		errCount  int64
		wg        sync.WaitGroup
		startTime = time.Now()
	)

	// Progress reporter.
	reportDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		var last int64
		lastT := startTime
		for {
			select {
			case <-reportDone:
				return
			case now := <-ticker.C:
				d := atomic.LoadInt64(&done)
				cur := atomic.LoadInt64(&cursor)
				instRate := float64(d-last) / now.Sub(lastT).Seconds()
				last, lastT = d, now
				var eta time.Duration
				if instRate > 0 {
					eta = time.Duration(float64(total-d)/instRate) * time.Second
				}
				log.Printf("progress: %d/%d (%.1f%%) | %.0f obj/s | errors=%d | next-index=%d | eta=%s",
					d, total, 100*float64(d)/float64(total), instRate,
					atomic.LoadInt64(&errCount), cur, eta.Round(time.Second))
			}
		}
	}()

	opts := minio.PutObjectOptions{ContentType: "application/octet-stream", DisableMultipart: true}

	for w := 0; w < *concurrency; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			// One random, incompressible buffer per worker (Garage compresses
			// per-object, so reuse is fine and avoids per-object allocation).
			buf := make([]byte, *size)
			r := rand.New(rand.NewSource(int64(1000 + worker)))
			for j := range buf {
				buf[j] = byte(r.Intn(256))
			}

			for {
				idx := atomic.AddInt64(&cursor, 1) - 1
				if idx >= *count {
					return
				}
				if ctx.Err() != nil {
					return
				}
				key := keyFor(idx)
				var putErr error
				for attempt := 0; attempt < 3; attempt++ {
					_, putErr = client.PutObject(ctx, *bucket, key, bytes.NewReader(buf), int64(*size), opts)
					if putErr == nil || ctx.Err() != nil {
						break
					}
					time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
				}
				if putErr != nil {
					if n := atomic.AddInt64(&errCount, 1); n <= 10 {
						log.Printf("put %q failed: %v", key, putErr)
					}
					continue
				}
				atomic.AddInt64(&done, 1)
			}
		}(w)
	}

	wg.Wait()
	close(reportDone)

	elapsed := time.Since(startTime)
	d := atomic.LoadInt64(&done)
	log.Printf("DONE: uploaded %d/%d objects in %s (%.0f obj/s), errors=%d",
		d, total, elapsed.Round(time.Second), float64(d)/elapsed.Seconds(), atomic.LoadInt64(&errCount))
	if ctx.Err() != nil {
		log.Printf("interrupted; resume with -start=%d", atomic.LoadInt64(&cursor))
	}
	if atomic.LoadInt64(&errCount) > 0 {
		os.Exit(1)
	}
}
