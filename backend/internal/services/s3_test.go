package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/pkg/utils"
)

func TestNewS3Service_StripsHTTPPrefix(t *testing.T) {
	cfg := &config.GarageConfig{
		Endpoint: "http://garage:3900",
		Region:   "garage",
	}
	_ = NewS3Service(cfg, nil)
	if cfg.Endpoint != "garage:3900" {
		t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, "garage:3900")
	}
	if cfg.UseSSL {
		t.Error("UseSSL should remain false for http://")
	}
}

func TestNewS3Service_StripsHTTPSPrefixAndEnablesSSL(t *testing.T) {
	cfg := &config.GarageConfig{
		Endpoint: "https://garage.example.com",
		Region:   "garage",
	}
	_ = NewS3Service(cfg, nil)
	if cfg.Endpoint != "garage.example.com" {
		t.Errorf("Endpoint = %q", cfg.Endpoint)
	}
	if !cfg.UseSSL {
		t.Error("UseSSL should be flipped to true for https://")
	}
}

func TestNewS3Service_LeavesBareHostUnchanged(t *testing.T) {
	cfg := &config.GarageConfig{
		Endpoint: "garage:3900",
		Region:   "garage",
		UseSSL:   true, // pre-set; should remain true
	}
	_ = NewS3Service(cfg, nil)
	if cfg.Endpoint != "garage:3900" {
		t.Errorf("Endpoint mutated unexpectedly: %q", cfg.Endpoint)
	}
	if !cfg.UseSSL {
		t.Error("pre-set UseSSL must be preserved")
	}
}

// adminBackedS3 wires an S3Service to a fresh GarageV2AdminService that talks
// to the supplied http.Handler.
func adminBackedS3(t *testing.T, handler http.Handler) (*S3Service, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	admin := NewGarageV2AdminService(&config.GarageConfig{
		AdminEndpoint: srv.URL,
		AdminToken:    "test-token",
	}, "")
	s3 := NewS3Service(&config.GarageConfig{
		Endpoint: "garage:3900",
		Region:   "garage",
	}, admin)
	return s3, srv
}

// uniqueBucket returns a per-subtest bucket name so cached credentials from
// one test don't leak into another via utils.GlobalCache.
func uniqueBucket(t *testing.T) string {
	t.Helper()
	name := "test-bucket-" + t.Name()
	t.Cleanup(func() {
		for _, op := range []Operation{OpRead, OpWrite, OpRead | OpWrite} {
			utils.GlobalCache.Delete(fmt.Sprintf("key:%s:%d", name, op))
		}
	})
	return name
}

func TestGetBucketCredentials_HappyPath(t *testing.T) {
	bucket := uniqueBucket(t)
	secret := "the-secret"

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&models.GarageBucketInfo{
			ID: "bid",
			Keys: []models.BucketKeyInfo{
				{
					AccessKeyID: "AK",
					Permissions: models.BucketKeyPermission{Read: true, Write: true},
				},
			},
		})
	})
	mux.HandleFunc("/v2/GetKeyInfo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&models.GarageKeyInfo{
			AccessKeyID:     "AK",
			SecretAccessKey: &secret,
		})
	})
	s3, _ := adminBackedS3(t, mux)

	creds, err := s3.getBucketCredentials(context.Background(), bucket, OpRead|OpWrite)
	if err != nil {
		t.Fatalf("getBucketCredentials: %v", err)
	}
	if creds == nil {
		t.Fatal("creds is nil")
	}
	v, err := creds.GetWithContext(nil)
	if err != nil {
		t.Fatalf("creds.GetWithContext: %v", err)
	}
	if v.AccessKeyID != "AK" || v.SecretAccessKey != secret {
		t.Errorf("got AK=%q SK=%q, want AK SK=%q", v.AccessKeyID, v.SecretAccessKey, secret)
	}
}

func TestGetBucketCredentials_CachesAcrossCalls(t *testing.T) {
	bucket := uniqueBucket(t)
	secret := "cache-secret"

	var bucketCalls, keyCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		bucketCalls++
		_ = json.NewEncoder(w).Encode(&models.GarageBucketInfo{
			ID: "bid",
			Keys: []models.BucketKeyInfo{
				{AccessKeyID: "AK", Permissions: models.BucketKeyPermission{Read: true, Write: true}},
			},
		})
	})
	mux.HandleFunc("/v2/GetKeyInfo", func(w http.ResponseWriter, r *http.Request) {
		keyCalls++
		_ = json.NewEncoder(w).Encode(&models.GarageKeyInfo{
			AccessKeyID:     "AK",
			SecretAccessKey: &secret,
		})
	})
	s3, _ := adminBackedS3(t, mux)

	for i := range 3 {
		if _, err := s3.getBucketCredentials(context.Background(), bucket, OpRead|OpWrite); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if bucketCalls != 1 {
		t.Errorf("GetBucketInfo called %d times, want 1 (cache should serve calls 2-3)", bucketCalls)
	}
	if keyCalls != 1 {
		t.Errorf("GetKeyInfo called %d times, want 1", keyCalls)
	}
}

func TestGetBucketCredentials_RWKeyWarmsAllTiers(t *testing.T) {
	bucket := uniqueBucket(t)
	secret := "rw-secret"

	var bucketCalls, keyCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		bucketCalls++
		_ = json.NewEncoder(w).Encode(&models.GarageBucketInfo{
			ID: "bid",
			Keys: []models.BucketKeyInfo{
				{AccessKeyID: "RW", Permissions: models.BucketKeyPermission{Read: true, Write: true}},
			},
		})
	})
	mux.HandleFunc("/v2/GetKeyInfo", func(w http.ResponseWriter, r *http.Request) {
		keyCalls++
		_ = json.NewEncoder(w).Encode(&models.GarageKeyInfo{
			AccessKeyID:     "RW",
			SecretAccessKey: &secret,
		})
	})
	s3, _ := adminBackedS3(t, mux)

	// Prime via OpRead — should populate OpRead, OpWrite, and OpRead|OpWrite.
	if _, err := s3.getBucketCredentials(context.Background(), bucket, OpRead); err != nil {
		t.Fatalf("prime OpRead: %v", err)
	}
	for _, op := range []Operation{OpWrite, OpRead | OpWrite, OpRead} {
		if _, err := s3.getBucketCredentials(context.Background(), bucket, op); err != nil {
			t.Fatalf("op %d: %v", op, err)
		}
	}
	if bucketCalls != 1 {
		t.Errorf("GetBucketInfo called %d times, want 1 (RW key should warm every tier)", bucketCalls)
	}
	if keyCalls != 1 {
		t.Errorf("GetKeyInfo called %d times, want 1", keyCalls)
	}
}

// A read-only key must NOT populate the write or RW cache slots, otherwise an
// OpWrite call would receive credentials the cluster will reject.
func TestGetBucketCredentials_ReadOnlyKeyDoesNotPoisonWriteCache(t *testing.T) {
	bucket := uniqueBucket(t)
	secret := "ro-secret"

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageBucketInfo{
			ID: "bid",
			Keys: []models.BucketKeyInfo{
				{AccessKeyID: "READ-ONLY", Permissions: models.BucketKeyPermission{Read: true, Write: false}},
			},
		})
	})
	mux.HandleFunc("/v2/GetKeyInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageKeyInfo{
			AccessKeyID:     "READ-ONLY",
			SecretAccessKey: &secret,
		})
	})
	s3, _ := adminBackedS3(t, mux)

	// Warm OpRead cache with the read-only key.
	if _, err := s3.getBucketCredentials(context.Background(), bucket, OpRead); err != nil {
		t.Fatalf("prime OpRead: %v", err)
	}
	// OpWrite must still fail — the read-only key must not have leaked into
	// the write cache slot.
	if _, err := s3.getBucketCredentials(context.Background(), bucket, OpWrite); err == nil {
		t.Fatal("OpWrite served credentials from a read-only key; cache was poisoned")
	}
}

func TestGetBucketCredentials_OpReadWriteSkipsKeysMissingAnyBit(t *testing.T) {
	bucket := uniqueBucket(t)
	secret := "good-secret"

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageBucketInfo{
			ID: "bid",
			Keys: []models.BucketKeyInfo{
				{AccessKeyID: "READ-ONLY", Permissions: models.BucketKeyPermission{Read: true, Write: false}},
				{AccessKeyID: "WRITE-ONLY", Permissions: models.BucketKeyPermission{Read: false, Write: true}},
				{AccessKeyID: "NO-PERMS", Permissions: models.BucketKeyPermission{}},
				{AccessKeyID: "RW", Permissions: models.BucketKeyPermission{Read: true, Write: true}},
			},
		})
	})
	mux.HandleFunc("/v2/GetKeyInfo", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") != "RW" {
			t.Errorf("GetKeyInfo called with id=%q, want RW", r.URL.Query().Get("id"))
		}
		_ = json.NewEncoder(w).Encode(&models.GarageKeyInfo{
			AccessKeyID:     "RW",
			SecretAccessKey: &secret,
		})
	})
	s3, _ := adminBackedS3(t, mux)

	creds, err := s3.getBucketCredentials(context.Background(), bucket, OpRead|OpWrite)
	if err != nil {
		t.Fatalf("getBucketCredentials: %v", err)
	}
	v, err := creds.GetWithContext(nil)
	if err != nil {
		t.Fatalf("creds.GetWithContext: %v", err)
	}
	if v.AccessKeyID != "RW" {
		t.Errorf("AccessKeyID = %q, want RW", v.AccessKeyID)
	}
}

// Regression test for issue #44: read-only buckets must remain browsable when
// no read+write key is assigned. Before the fix, this case returned
// "no valid credentials found for bucket music" and the UI broke entirely.
func TestGetBucketCredentials_ReadOnlyFallsBackToReadKey(t *testing.T) {
	bucket := uniqueBucket(t)
	secret := "ro-secret"

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageBucketInfo{
			ID: "bid",
			Keys: []models.BucketKeyInfo{
				{AccessKeyID: "READ-ONLY", Permissions: models.BucketKeyPermission{Read: true, Write: false}},
			},
		})
	})
	mux.HandleFunc("/v2/GetKeyInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageKeyInfo{
			AccessKeyID:     "READ-ONLY",
			SecretAccessKey: &secret,
		})
	})
	s3, _ := adminBackedS3(t, mux)

	creds, err := s3.getBucketCredentials(context.Background(), bucket, OpRead)
	if err != nil {
		t.Fatalf("getBucketCredentials: %v", err)
	}
	v, err := creds.GetWithContext(nil)
	if err != nil {
		t.Fatalf("creds.GetWithContext: %v", err)
	}
	if v.AccessKeyID != "READ-ONLY" {
		t.Errorf("AccessKeyID = %q, want READ-ONLY", v.AccessKeyID)
	}
}

// Even with only a read-only key available, asking for write credentials must
// still fail loudly so uploads/deletes return a meaningful error instead of
// silently using a key that the cluster will reject.
func TestGetBucketCredentials_ReadOnlyBucketRejectsWriteRequest(t *testing.T) {
	bucket := uniqueBucket(t)
	secret := "ro-secret"

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageBucketInfo{
			ID: "bid",
			Keys: []models.BucketKeyInfo{
				{AccessKeyID: "READ-ONLY", Permissions: models.BucketKeyPermission{Read: true, Write: false}},
			},
		})
	})
	mux.HandleFunc("/v2/GetKeyInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageKeyInfo{
			AccessKeyID:     "READ-ONLY",
			SecretAccessKey: &secret,
		})
	})
	s3, _ := adminBackedS3(t, mux)

	_, err := s3.getBucketCredentials(context.Background(), bucket, OpRead|OpWrite)
	if err == nil {
		t.Fatal("expected error when only a read-only key exists, got nil")
	}
	if !strings.Contains(err.Error(), "no valid credentials") {
		t.Errorf("expected 'no valid credentials' in error, got %v", err)
	}
}

// Mirror of issue #44 for write-only buckets: uploads must still succeed with a
// write-only key, even though no key grants read access.
func TestGetBucketCredentials_WriteOnlyFallsBackToWriteKey(t *testing.T) {
	bucket := uniqueBucket(t)
	secret := "wo-secret"

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageBucketInfo{
			ID: "bid",
			Keys: []models.BucketKeyInfo{
				{AccessKeyID: "WRITE-ONLY", Permissions: models.BucketKeyPermission{Read: false, Write: true}},
			},
		})
	})
	mux.HandleFunc("/v2/GetKeyInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageKeyInfo{
			AccessKeyID:     "WRITE-ONLY",
			SecretAccessKey: &secret,
		})
	})
	s3, _ := adminBackedS3(t, mux)

	creds, err := s3.getBucketCredentials(context.Background(), bucket, OpWrite)
	if err != nil {
		t.Fatalf("getBucketCredentials: %v", err)
	}
	v, err := creds.GetWithContext(nil)
	if err != nil {
		t.Fatalf("creds.GetWithContext: %v", err)
	}
	if v.AccessKeyID != "WRITE-ONLY" {
		t.Errorf("AccessKeyID = %q, want WRITE-ONLY", v.AccessKeyID)
	}
}

func TestGetBucketCredentials_NoEligibleKeyReturnsError(t *testing.T) {
	bucket := uniqueBucket(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageBucketInfo{
			ID:   "bid",
			Keys: []models.BucketKeyInfo{}, // no keys at all
		})
	})
	mux.HandleFunc("/v2/GetKeyInfo", func(w http.ResponseWriter, r *http.Request) {
		t.Error("GetKeyInfo should not be called when no keys exist")
	})
	s3, _ := adminBackedS3(t, mux)

	_, err := s3.getBucketCredentials(context.Background(), bucket, OpRead)
	if err == nil {
		t.Fatal("expected error when bucket has no keys, got nil")
	}
	if !strings.Contains(err.Error(), "no valid credentials") {
		t.Errorf("expected 'no valid credentials' in error, got %v", err)
	}
}

func TestGetBucketCredentials_KeyWithoutSecretIsSkipped(t *testing.T) {
	bucket := uniqueBucket(t)
	secret := "good"

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageBucketInfo{
			ID: "bid",
			Keys: []models.BucketKeyInfo{
				{AccessKeyID: "FIRST-RW", Permissions: models.BucketKeyPermission{Read: true, Write: true}},
				{AccessKeyID: "SECOND-RW", Permissions: models.BucketKeyPermission{Read: true, Write: true}},
			},
		})
	})
	mux.HandleFunc("/v2/GetKeyInfo", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("id") {
		case "FIRST-RW":
			_ = json.NewEncoder(w).Encode(&models.GarageKeyInfo{AccessKeyID: "FIRST-RW", SecretAccessKey: nil})
		case "SECOND-RW":
			_ = json.NewEncoder(w).Encode(&models.GarageKeyInfo{AccessKeyID: "SECOND-RW", SecretAccessKey: &secret})
		default:
			t.Errorf("unexpected GetKeyInfo id: %q", r.URL.Query().Get("id"))
		}
	})
	s3, _ := adminBackedS3(t, mux)

	creds, err := s3.getBucketCredentials(context.Background(), bucket, OpRead|OpWrite)
	if err != nil {
		t.Fatalf("getBucketCredentials: %v", err)
	}
	v, err := creds.GetWithContext(nil)
	if err != nil {
		t.Fatalf("creds.GetWithContext: %v", err)
	}
	if v.AccessKeyID != "SECOND-RW" {
		t.Errorf("AccessKeyID = %q, want SECOND-RW (loop must skip FIRST-RW's nil secret)", v.AccessKeyID)
	}
}

func TestGetBucketCredentials_AdminErrorPropagates(t *testing.T) {
	bucket := uniqueBucket(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	})
	s3, _ := adminBackedS3(t, mux)

	_, err := s3.getBucketCredentials(context.Background(), bucket, OpRead|OpWrite)
	if err == nil {
		t.Fatal("expected error when admin call fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get bucket info") {
		t.Errorf("expected wrapped 'failed to get bucket info' error, got %v", err)
	}
}

func TestGetBucketStatistics_HappyPath(t *testing.T) {
	bucket := uniqueBucket(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "globalAlias=") {
			t.Errorf("expected globalAlias query, got %q", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(&models.GarageBucketInfo{
			ID:      "bid",
			Objects: 42,
			Bytes:   123_456,
		})
	})
	s3, _ := adminBackedS3(t, mux)

	stats, err := s3.GetBucketStatistics(context.Background(), bucket)
	if err != nil {
		t.Fatalf("GetBucketStatistics: %v", err)
	}
	if stats.ObjectCount != 42 || stats.TotalSize != 123_456 {
		t.Errorf("got %+v, want ObjectCount=42 TotalSize=123456", stats)
	}
}

func TestGetBucketStatistics_AdminErrorPropagates(t *testing.T) {
	bucket := uniqueBucket(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"no such bucket"}`))
	})
	s3, _ := adminBackedS3(t, mux)

	_, err := s3.GetBucketStatistics(context.Background(), bucket)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get bucket info") {
		t.Errorf("expected 'failed to get bucket info' wrap, got %v", err)
	}
}
