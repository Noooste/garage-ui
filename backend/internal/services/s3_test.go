package services

import (
	"context"
	"encoding/json"
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
		utils.GlobalCache.Delete("key:" + name)
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

	creds, err := s3.getBucketCredentials(context.Background(), bucket)
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
		if _, err := s3.getBucketCredentials(context.Background(), bucket); err != nil {
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

func TestGetBucketCredentials_SkipsKeysWithoutReadOrWrite(t *testing.T) {
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

	creds, err := s3.getBucketCredentials(context.Background(), bucket)
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

	_, err := s3.getBucketCredentials(context.Background(), bucket)
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

	creds, err := s3.getBucketCredentials(context.Background(), bucket)
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

	_, err := s3.getBucketCredentials(context.Background(), bucket)
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
