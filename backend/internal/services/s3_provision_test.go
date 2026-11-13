package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
	logpkg "Noooste/garage-ui/pkg/logger"
	"Noooste/garage-ui/pkg/utils"

	"github.com/rs/zerolog"
)

const provisionSecret = "SUPER-SECRET-VALUE-0123456789"

// fakeGarage is a small stateful stand-in for the Garage admin API: keys,
// buckets and the key<->bucket permission matrix.
type fakeGarage struct {
	mu          sync.Mutex
	keys        map[string]*models.ListKeysResponseItem // by id
	bucketKeys  map[string]map[string]models.BucketKeyPermission
	bucketIDs   map[string]string // alias -> id
	failCreate  bool
	failAllow   bool
	creates     atomic.Int32
	allows      atomic.Int32
	listKeys    atomic.Int32
	createReqs  []models.CreateKeyRequest
	allowReqs   []models.BucketKeyPermRequest
	nextKeySeed int
}

func newFakeGarage(buckets ...string) *fakeGarage {
	g := &fakeGarage{
		keys:       map[string]*models.ListKeysResponseItem{},
		bucketKeys: map[string]map[string]models.BucketKeyPermission{},
		bucketIDs:  map[string]string{},
	}
	for _, b := range buckets {
		g.bucketIDs[b] = "id-" + b
		g.bucketKeys["id-"+b] = map[string]models.BucketKeyPermission{}
	}
	return g
}

func (g *fakeGarage) addKey(id, name string) {
	g.keys[id] = &models.ListKeysResponseItem{ID: id, Name: name}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (g *fakeGarage) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		id, ok := g.bucketIDs[r.URL.Query().Get("globalAlias")]
		if !ok {
			http.Error(w, `{"code":"NoSuchBucket"}`, http.StatusNotFound)
			return
		}
		info := models.GarageBucketInfo{ID: id, Keys: []models.BucketKeyInfo{}}
		for kid, p := range g.bucketKeys[id] {
			info.Keys = append(info.Keys, models.BucketKeyInfo{AccessKeyID: kid, Permissions: p})
		}
		writeJSON(w, info)
	})
	mux.HandleFunc("/v2/ListKeys", func(w http.ResponseWriter, r *http.Request) {
		g.listKeys.Add(1)
		g.mu.Lock()
		defer g.mu.Unlock()
		out := []models.ListKeysResponseItem{}
		for _, k := range g.keys {
			out = append(out, *k)
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("/v2/CreateKey", func(w http.ResponseWriter, r *http.Request) {
		g.creates.Add(1)
		var req models.CreateKeyRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		g.mu.Lock()
		defer g.mu.Unlock()
		g.createReqs = append(g.createReqs, req)
		if g.failCreate {
			http.Error(w, `{"code":"Forbidden"}`, http.StatusForbidden)
			return
		}
		g.nextKeySeed++
		id := fmt.Sprintf("GKnewkey%012d", g.nextKeySeed)
		name := ""
		if req.Name != nil {
			name = *req.Name
		}
		g.addKey(id, name)
		writeJSON(w, models.GarageKeyInfo{AccessKeyID: id, Name: name})
	})
	mux.HandleFunc("/v2/GetKeyInfo", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		g.mu.Lock()
		k, ok := g.keys[id]
		g.mu.Unlock()
		if !ok {
			http.Error(w, `{"code":"NoSuchAccessKey"}`, http.StatusNotFound)
			return
		}
		secret := provisionSecret
		writeJSON(w, models.GarageKeyInfo{AccessKeyID: k.ID, Name: k.Name, SecretAccessKey: &secret})
	})
	mux.HandleFunc("/v2/AllowBucketKey", func(w http.ResponseWriter, r *http.Request) {
		g.allows.Add(1)
		var req models.BucketKeyPermRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		g.mu.Lock()
		defer g.mu.Unlock()
		g.allowReqs = append(g.allowReqs, req)
		if g.failAllow {
			http.Error(w, `{"code":"Forbidden"}`, http.StatusForbidden)
			return
		}
		p := g.bucketKeys[req.BucketID][req.AccessKeyID]
		p.Read = p.Read || req.Permissions.Read
		p.Write = p.Write || req.Permissions.Write
		p.Owner = p.Owner || req.Permissions.Owner
		g.bucketKeys[req.BucketID][req.AccessKeyID] = p
		writeJSON(w, models.GarageBucketInfo{ID: req.BucketID})
	})
	return mux
}

// provisioningS3 builds an S3Service backed by g with the option set as given.
func provisioningS3(t *testing.T, g *fakeGarage, enabled bool, keyName string) *S3Service {
	t.Helper()
	srv := httptest.NewServer(g.handler())
	t.Cleanup(srv.Close)
	admin := NewGarageV2AdminService(&config.GarageConfig{AdminEndpoint: srv.URL, AdminToken: "test-token"}, "")
	return NewS3Service(&config.GarageConfig{
		Endpoint:                  "garage:3900",
		Region:                    "garage",
		AutoProvisionBucketAccess: enabled,
		UIKeyName:                 keyName,
	}, admin)
}

func credValues(t *testing.T, s3 *S3Service, bucket string, op Operation) (string, string) {
	t.Helper()
	creds, err := s3.getBucketCredentials(context.Background(), bucket, op)
	if err != nil {
		t.Fatalf("getBucketCredentials: %v", err)
	}
	v, err := creds.GetWithContext(nil)
	if err != nil {
		t.Fatalf("creds: %v", err)
	}
	return v.AccessKeyID, v.SecretAccessKey
}

func TestAutoProvision_NoKeys_CreatesKeyAndGrantsReadWrite(t *testing.T) {
	bucket := uniqueBucket(t)
	g := newFakeGarage(bucket)
	s3 := provisioningS3(t, g, true, "my-ui-key")

	ak, sk := credValues(t, s3, bucket, OpRead)
	if !strings.HasPrefix(ak, "GKnewkey") || sk != provisionSecret {
		t.Errorf("creds = (%q, %q)", ak, sk)
	}
	if g.creates.Load() != 1 || g.allows.Load() != 1 {
		t.Fatalf("creates=%d allows=%d, want 1 and 1", g.creates.Load(), g.allows.Load())
	}

	// Key: named as configured, no global permissions requested.
	cr := g.createReqs[0]
	if cr.Name == nil || *cr.Name != "my-ui-key" || cr.Allow != nil || !cr.NeverExpires {
		t.Errorf("CreateKey request = %+v", cr)
	}
	// Grant: read+write on that bucket, never owner.
	ar := g.allowReqs[0]
	if ar.BucketID != "id-"+bucket || ar.AccessKeyID != ak || !ar.Permissions.Read || !ar.Permissions.Write || ar.Permissions.Owner {
		t.Errorf("AllowBucketKey request = %+v", ar)
	}

	// Write creds come from the cache without any further admin call.
	for _, op := range []Operation{OpWrite, OpRead | OpWrite, OpRead} {
		credValues(t, s3, bucket, op)
	}
	if g.creates.Load() != 1 || g.allows.Load() != 1 || g.listKeys.Load() != 1 {
		t.Errorf("extra admin calls after caching: creates=%d allows=%d list=%d", g.creates.Load(), g.allows.Load(), g.listKeys.Load())
	}
}

func TestAutoProvision_DefaultKeyName(t *testing.T) {
	bucket := uniqueBucket(t)
	g := newFakeGarage(bucket)
	s3 := provisioningS3(t, g, true, "")
	credValues(t, s3, bucket, OpRead)
	if n := g.createReqs[0].Name; n == nil || *n != "garage-ui" {
		t.Errorf("name = %v, want garage-ui", n)
	}
}

func TestAutoProvision_ExistingKeyIsReusedAcrossBuckets(t *testing.T) {
	b1 := uniqueBucket(t) + "-1"
	b2 := uniqueBucket(t) + "-2"
	t.Cleanup(func() { s3Forget(b1); s3Forget(b2) })
	g := newFakeGarage(b1, b2)
	g.addKey("GKexisting000000001", "garage-ui")
	g.addKey("GKother0000000002", "someone-else")
	s3 := provisioningS3(t, g, true, "garage-ui")

	for _, b := range []string{b1, b2} {
		if ak, _ := credValues(t, s3, b, OpRead|OpWrite); ak != "GKexisting000000001" {
			t.Errorf("bucket %s: key = %q", b, ak)
		}
	}
	if g.creates.Load() != 0 {
		t.Errorf("CreateKey called %d times, want 0", g.creates.Load())
	}
	if g.allows.Load() != 2 {
		t.Errorf("AllowBucketKey called %d times, want 2 (one per bucket)", g.allows.Load())
	}
}

func TestAutoProvision_ExpiredKeyWithSameNameIsNotReused(t *testing.T) {
	bucket := uniqueBucket(t)
	g := newFakeGarage(bucket)
	g.addKey("GKexpired0000000001", "garage-ui")
	g.keys["GKexpired0000000001"].Expired = true
	s3 := provisioningS3(t, g, true, "garage-ui")

	if ak, _ := credValues(t, s3, bucket, OpRead); ak == "GKexpired0000000001" {
		t.Error("expired key was reused")
	}
	if g.creates.Load() != 1 {
		t.Errorf("creates = %d, want 1", g.creates.Load())
	}
}

func TestAutoProvision_ConcurrentFirstAccessCreatesAndGrantsOnce(t *testing.T) {
	bucket := uniqueBucket(t)
	g := newFakeGarage(bucket)
	s3 := provisioningS3(t, g, true, "garage-ui")

	ops := []Operation{OpRead, OpWrite, OpRead | OpWrite}
	var wg sync.WaitGroup
	errs := make(chan error, 30)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(op Operation) {
			defer wg.Done()
			if _, err := s3.getBucketCredentials(context.Background(), bucket, op); err != nil {
				errs <- err
			}
		}(ops[i%len(ops)])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("request failed: %v", err)
	}
	if g.creates.Load() != 1 || g.allows.Load() != 1 {
		t.Errorf("creates=%d allows=%d, want exactly 1 each", g.creates.Load(), g.allows.Load())
	}
}

func TestAutoProvision_ConcurrentAcrossBucketsCreatesKeyOnce(t *testing.T) {
	var buckets []string
	for i := 0; i < 5; i++ {
		b := fmt.Sprintf("%s-%d", uniqueBucket(t), i)
		b2 := b
		t.Cleanup(func() { s3Forget(b2) })
		buckets = append(buckets, b)
	}
	g := newFakeGarage(buckets...)
	s3 := provisioningS3(t, g, true, "garage-ui")

	var wg sync.WaitGroup
	for _, b := range buckets {
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func(b string) {
				defer wg.Done()
				if _, err := s3.getBucketCredentials(context.Background(), b, OpRead|OpWrite); err != nil {
					t.Errorf("bucket %s: %v", b, err)
				}
			}(b)
		}
	}
	wg.Wait()
	if g.creates.Load() != 1 || g.allows.Load() != 5 {
		t.Errorf("creates=%d allows=%d, want 1 and 5", g.creates.Load(), g.allows.Load())
	}
}

func TestAutoProvision_ReadOnlyUIKeyIsToppedUpForWrite(t *testing.T) {
	bucket := uniqueBucket(t)
	g := newFakeGarage(bucket)
	g.addKey("GKui0000000000001", "garage-ui")
	g.bucketKeys["id-"+bucket]["GKui0000000000001"] = models.BucketKeyPermission{Read: true}
	s3 := provisioningS3(t, g, true, "garage-ui")

	// Read works as before, no provisioning.
	credValues(t, s3, bucket, OpRead)
	if g.allows.Load() != 0 {
		t.Fatalf("AllowBucketKey called for a read the key already satisfies")
	}
	// Write triggers the top-up on that same key.
	if ak, _ := credValues(t, s3, bucket, OpWrite); ak != "GKui0000000000001" {
		t.Errorf("key = %q", ak)
	}
	if g.creates.Load() != 0 || g.allows.Load() != 1 {
		t.Fatalf("creates=%d allows=%d", g.creates.Load(), g.allows.Load())
	}
	p := g.allowReqs[0].Permissions
	if !p.Read || !p.Write || p.Owner {
		t.Errorf("permissions = %+v", p)
	}
}

func TestAutoProvision_OptionOff_KeepsOriginalError(t *testing.T) {
	bucket := uniqueBucket(t)
	g := newFakeGarage(bucket)
	s3 := provisioningS3(t, g, false, "garage-ui")

	_, err := s3.getBucketCredentials(context.Background(), bucket, OpRead)
	if err == nil || err.Error() != "no valid credentials found for bucket "+bucket {
		t.Fatalf("err = %v", err)
	}
	if g.listKeys.Load()+g.creates.Load()+g.allows.Load() != 0 {
		t.Error("admin key/grant endpoints were called with the option off")
	}
}

func TestAutoProvision_OptionOff_ReadOnlyKeyStillRejectsWrite(t *testing.T) {
	bucket := uniqueBucket(t)
	g := newFakeGarage(bucket)
	g.addKey("GKro", "ro")
	g.bucketKeys["id-"+bucket]["GKro"] = models.BucketKeyPermission{Read: true}
	s3 := provisioningS3(t, g, false, "garage-ui")

	if _, err := s3.getBucketCredentials(context.Background(), bucket, OpWrite); err == nil ||
		!strings.Contains(err.Error(), "no valid credentials found") {
		t.Fatalf("err = %v", err)
	}
	if g.allows.Load() != 0 || g.creates.Load() != 0 {
		t.Error("provisioning ran with the option off")
	}
}

func TestAutoProvision_UnknownBucketDoesNotProvision(t *testing.T) {
	g := newFakeGarage()
	s3 := provisioningS3(t, g, true, "garage-ui")

	_, err := s3.getBucketCredentials(context.Background(), "no-such-bucket-"+t.Name(), OpRead)
	if err == nil || !strings.Contains(err.Error(), "failed to get bucket info") {
		t.Fatalf("err = %v", err)
	}
	if g.listKeys.Load()+g.creates.Load()+g.allows.Load() != 0 {
		t.Error("provisioning ran for a bucket that does not exist")
	}
}

func TestAutoProvision_AdminFailureIsClearAndNotCached(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail func(*fakeGarage)
		want string
	}{
		{"create key", func(g *fakeGarage) { g.failCreate = true }, "failed to create key"},
		{"grant", func(g *fakeGarage) { g.failAllow = true }, "failed to grant key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bucket := uniqueBucket(t)
			g := newFakeGarage(bucket)
			tc.fail(g)
			s3 := provisioningS3(t, g, true, "garage-ui")

			_, err := s3.getBucketCredentials(context.Background(), bucket, OpRead)
			if err == nil {
				t.Fatal("expected error")
			}
			for _, w := range []string{"no valid credentials found for bucket " + bucket, "auto-provisioning access failed", tc.want} {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q lacks %q", err, w)
				}
			}
			if strings.Contains(err.Error(), provisionSecret) {
				t.Error("error leaks the secret")
			}

			// Nothing was cached as success: once the failure is gone the next
			// call provisions normally.
			g.mu.Lock()
			g.failCreate, g.failAllow = false, false
			g.mu.Unlock()
			if _, err := s3.getBucketCredentials(context.Background(), bucket, OpRead); err != nil {
				t.Fatalf("retry after recovery: %v", err)
			}
		})
	}
}

func TestAutoProvision_LogsActionsWithoutSecrets(t *testing.T) {
	bucket := uniqueBucket(t)
	g := newFakeGarage(bucket)
	s3 := provisioningS3(t, g, true, "garage-ui")

	var buf bytes.Buffer
	ctx := logpkg.IntoCtx(context.Background(), zerolog.New(&buf).Level(zerolog.DebugLevel))
	creds, err := s3.getBucketCredentials(ctx, bucket, OpRead|OpWrite)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := creds.GetWithContext(nil)

	out := buf.String()
	if !strings.Contains(out, "auto-provisioned bucket access: created the Garage UI key") ||
		!strings.Contains(out, "auto-provisioned bucket access: granted read/write") ||
		!strings.Contains(out, bucket) {
		t.Errorf("missing info logs:\n%s", out)
	}
	if strings.Contains(out, provisionSecret) {
		t.Errorf("secret key found in logs:\n%s", out)
	}
	if strings.Contains(out, v.AccessKeyID) {
		t.Errorf("full access key id found in logs (expected redacted):\n%s", out)
	}
}

func s3Forget(bucket string) {
	for _, op := range []Operation{OpRead, OpWrite, OpRead | OpWrite} {
		utils.GlobalCache.Delete(fmt.Sprintf("key:%s:%d", bucket, op))
	}
}
