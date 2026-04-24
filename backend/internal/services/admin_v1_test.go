package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
)

func newV1TestServer(t *testing.T, handler http.Handler) *GarageV1AdminService {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewGarageV1AdminService(&config.GarageConfig{
		AdminEndpoint: srv.URL,
		AdminToken:    "test-token",
	}, "")
}

func TestV1_ListKeys(t *testing.T) {
	items := []models.ListKeysResponseItem{{ID: "GK1", Name: "key1"}}
	svc := newV1TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/key" || r.URL.Query().Get("list") == "" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	}))
	result, err := svc.ListKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].ID != "GK1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestV1_GetClusterHealth(t *testing.T) {
	svc := newV1TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"healthy","knownNodes":3,"connectedNodes":3,"storageNodes":3,"storageNodesOk":3,"partitions":256,"partitionsQuorum":256,"partitionsAllOk":256}`))
	}))
	health, err := svc.GetClusterHealth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if health.StorageNodesUp != 3 {
		t.Fatalf("expected StorageNodesUp=3, got %d", health.StorageNodesUp)
	}
}

func TestV1_GetClusterStatistics_Unsupported(t *testing.T) {
	svc := newV1TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make any HTTP request for unsupported operations")
	}))
	_, err := svc.GetClusterStatistics(context.Background())
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}

func TestV1_GetNodeInfo_Unsupported(t *testing.T) {
	svc := newV1TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make any HTTP request for unsupported operations")
	}))
	_, err := svc.GetNodeInfo(context.Background(), "abc")
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}

func TestV1_GetNodeStatistics_Unsupported(t *testing.T) {
	svc := newV1TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make any HTTP request for unsupported operations")
	}))
	_, err := svc.GetNodeStatistics(context.Background(), "abc")
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}

// v1RecordingHandler returns a handler that records the request and responds with JSON.
func v1RecordingHandler(t *testing.T, status int, body any) (http.Handler, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.rawURL = r.URL.RequestURI()
		rec.auth = r.Header.Get("Authorization")
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			rec.body = b
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			json.NewEncoder(w).Encode(body)
		}
	})
	return h, rec
}

func newV1RecordingServer(t *testing.T, status int, body any) (*GarageV1AdminService, *recordedRequest) {
	t.Helper()
	h, rec := v1RecordingHandler(t, status, body)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	svc := NewGarageV1AdminService(&config.GarageConfig{
		AdminEndpoint: srv.URL,
		AdminToken:    "test-token",
	}, "")
	return svc, rec
}

func TestV1_CreateKey(t *testing.T) {
	name := "mykey"
	want := &models.GarageKeyInfo{AccessKeyID: "GK1", Name: name}
	svc, rec := newV1RecordingServer(t, 200, want)

	got, err := svc.CreateKey(context.Background(), models.CreateKeyRequest{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodPost || rec.path != "/v1/key" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if got.AccessKeyID != "GK1" {
		t.Errorf("AccessKeyID = %q, want GK1", got.AccessKeyID)
	}
}

func TestV1_GetKeyInfo(t *testing.T) {
	want := &models.GarageKeyInfo{AccessKeyID: "ABC"}
	svc, rec := newV1RecordingServer(t, 200, want)

	got, err := svc.GetKeyInfo(context.Background(), "ABC", true)
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodGet {
		t.Errorf("method = %q, want GET", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=ABC") || !strings.Contains(rec.rawURL, "showSecretKey=true") {
		t.Errorf("rawURL = %q, want id=ABC&showSecretKey=true", rec.rawURL)
	}
	if got.AccessKeyID != "ABC" {
		t.Errorf("got %q, want ABC", got.AccessKeyID)
	}
}

func TestV1_UpdateKey(t *testing.T) {
	want := &models.GarageKeyInfo{AccessKeyID: "K1"}
	svc, rec := newV1RecordingServer(t, 200, want)

	_, err := svc.UpdateKey(context.Background(), "K1", models.UpdateKeyRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodPost {
		t.Errorf("method = %q, want POST", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=K1") {
		t.Errorf("rawURL = %q, want id=K1", rec.rawURL)
	}
}

func TestV1_DeleteKey(t *testing.T) {
	svc, rec := newV1RecordingServer(t, 200, nil)

	err := svc.DeleteKey(context.Background(), "K1")
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=K1") {
		t.Errorf("rawURL = %q, want id=K1", rec.rawURL)
	}
}

func TestV1_ImportKey(t *testing.T) {
	want := &models.GarageKeyInfo{AccessKeyID: "GKimported"}
	svc, rec := newV1RecordingServer(t, 200, want)

	got, err := svc.ImportKey(context.Background(), models.ImportKeyRequest{
		AccessKeyID:     "GKimported",
		SecretAccessKey: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodPost || rec.path != "/v1/key/import" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if got.AccessKeyID != "GKimported" {
		t.Errorf("got %q", got.AccessKeyID)
	}
}

func TestV1_ListBuckets(t *testing.T) {
	want := []models.ListBucketsResponseItem{{ID: "b1", GlobalAliases: []string{"mybucket"}}}
	svc, rec := newV1RecordingServer(t, 200, want)

	got, err := svc.ListBuckets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodGet || rec.path != "/v1/bucket" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if len(got) != 1 || got[0].ID != "b1" {
		t.Errorf("got %+v", got)
	}
}

func TestV1_GetBucketInfo(t *testing.T) {
	want := &models.GarageBucketInfo{ID: "b1"}
	svc, rec := newV1RecordingServer(t, 200, want)

	got, err := svc.GetBucketInfo(context.Background(), "b1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.rawURL, "id=b1") {
		t.Errorf("rawURL = %q", rec.rawURL)
	}
	if got.ID != "b1" {
		t.Errorf("got %q", got.ID)
	}
}

func TestV1_GetBucketInfoByAlias(t *testing.T) {
	want := &models.GarageBucketInfo{ID: "b2"}
	svc, rec := newV1RecordingServer(t, 200, want)

	got, err := svc.GetBucketInfoByAlias(context.Background(), "myalias")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.rawURL, "globalAlias=myalias") {
		t.Errorf("rawURL = %q", rec.rawURL)
	}
	if got.ID != "b2" {
		t.Errorf("got %q", got.ID)
	}
}

func TestV1_CreateBucket(t *testing.T) {
	want := &models.GarageBucketInfo{ID: "newb"}
	svc, rec := newV1RecordingServer(t, 200, want)

	alias := "test-bucket"
	got, err := svc.CreateBucket(context.Background(), models.CreateBucketAdminRequest{GlobalAlias: &alias})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodPost || rec.path != "/v1/bucket" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if got.ID != "newb" {
		t.Errorf("got %q", got.ID)
	}
}

func TestV1_UpdateBucket(t *testing.T) {
	want := &models.GarageBucketInfo{ID: "b1"}
	svc, rec := newV1RecordingServer(t, 200, want)

	_, err := svc.UpdateBucket(context.Background(), "b1", models.UpdateBucketRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodPut {
		t.Errorf("method = %q, want PUT", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=b1") {
		t.Errorf("rawURL = %q", rec.rawURL)
	}
}

func TestV1_DeleteBucket(t *testing.T) {
	svc, rec := newV1RecordingServer(t, 200, nil)

	err := svc.DeleteBucket(context.Background(), "b1")
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=b1") {
		t.Errorf("rawURL = %q", rec.rawURL)
	}
}

func TestV1_AllowBucketKey(t *testing.T) {
	want := &models.GarageBucketInfo{ID: "b1"}
	svc, rec := newV1RecordingServer(t, 200, want)

	_, err := svc.AllowBucketKey(context.Background(), models.BucketKeyPermRequest{
		BucketID: "b1", AccessKeyID: "k1",
		Permissions: models.BucketKeyPermission{Read: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodPost || rec.path != "/v1/bucket/allow" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestV1_DenyBucketKey(t *testing.T) {
	want := &models.GarageBucketInfo{ID: "b1"}
	svc, rec := newV1RecordingServer(t, 200, want)

	_, err := svc.DenyBucketKey(context.Background(), models.BucketKeyPermRequest{
		BucketID: "b1", AccessKeyID: "k1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodPost || rec.path != "/v1/bucket/deny" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestV1_AddBucketAlias_Global(t *testing.T) {
	want := &models.GarageBucketInfo{ID: "b1"}
	svc, rec := newV1RecordingServer(t, 200, want)

	alias := "myalias"
	_, err := svc.AddBucketAlias(context.Background(), models.AddBucketAliasRequest{
		BucketID: "b1", GlobalAlias: &alias,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodPut || rec.path != "/v1/bucket/alias/global" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if !strings.Contains(rec.rawURL, "id=b1") || !strings.Contains(rec.rawURL, "alias=myalias") {
		t.Errorf("rawURL = %q", rec.rawURL)
	}
}

func TestV1_AddBucketAlias_Local(t *testing.T) {
	want := &models.GarageBucketInfo{ID: "b1"}
	svc, rec := newV1RecordingServer(t, 200, want)

	alias := "localname"
	keyID := "GK1"
	_, err := svc.AddBucketAlias(context.Background(), models.AddBucketAliasRequest{
		BucketID: "b1", LocalAlias: &alias, AccessKeyID: &keyID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodPut || rec.path != "/v1/bucket/alias/local" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestV1_RemoveBucketAlias_Global(t *testing.T) {
	want := &models.GarageBucketInfo{ID: "b1"}
	svc, rec := newV1RecordingServer(t, 200, want)

	alias := "myalias"
	_, err := svc.RemoveBucketAlias(context.Background(), models.RemoveBucketAliasRequest{
		BucketID: "b1", GlobalAlias: &alias,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodDelete || rec.path != "/v1/bucket/alias/global" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestV1_GetClusterStatus(t *testing.T) {
	raw := map[string]any{
		"node":          "abc123",
		"garageVersion": "1.3.0",
		"knownNodes": []map[string]any{
			{"id": "n1", "addr": "1.2.3.4:3901", "isUp": true, "hostname": "node1"},
			{"id": "n2", "addr": "5.6.7.8:3901", "isUp": false, "lastSeenSecsAgo": 60, "hostname": "node2"},
		},
		"layout": map[string]any{"version": 3},
	}
	svc, rec := newV1RecordingServer(t, 200, raw)

	got, err := svc.GetClusterStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rec.path != "/v1/status" {
		t.Errorf("path = %q, want /v1/status", rec.path)
	}
	if got.LayoutVersion != 3 {
		t.Errorf("LayoutVersion = %d, want 3", got.LayoutVersion)
	}
	if len(got.Nodes) != 2 {
		t.Fatalf("len(Nodes) = %d, want 2", len(got.Nodes))
	}
	if got.Nodes[0].ID != "n1" || !got.Nodes[0].IsUp {
		t.Errorf("Nodes[0] = %+v", got.Nodes[0])
	}
	if got.Nodes[1].ID != "n2" || got.Nodes[1].IsUp {
		t.Errorf("Nodes[1] = %+v", got.Nodes[1])
	}
}

func TestV1_HealthCheck(t *testing.T) {
	svc, rec := newV1RecordingServer(t, 200, nil)

	err := svc.HealthCheck(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rec.path != "/health" {
		t.Errorf("path = %q, want /health", rec.path)
	}
}

func TestV1_ErrorPaths(t *testing.T) {
	// Server that returns 500 for all requests to exercise error branches.
	srv500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	t.Cleanup(srv500.Close)
	svc := NewGarageV1AdminService(&config.GarageConfig{
		AdminEndpoint: srv500.URL,
		AdminToken:    "tok",
	}, "")

	ctx := context.Background()

	if _, err := svc.ListKeys(ctx); err == nil {
		t.Error("ListKeys: expected error")
	}
	if _, err := svc.CreateKey(ctx, models.CreateKeyRequest{}); err == nil {
		t.Error("CreateKey: expected error")
	}
	if _, err := svc.GetKeyInfo(ctx, "k", false); err == nil {
		t.Error("GetKeyInfo: expected error")
	}
	if _, err := svc.UpdateKey(ctx, "k", models.UpdateKeyRequest{}); err == nil {
		t.Error("UpdateKey: expected error")
	}
	if err := svc.DeleteKey(ctx, "k"); err == nil {
		t.Error("DeleteKey: expected error")
	}
	if _, err := svc.ImportKey(ctx, models.ImportKeyRequest{}); err == nil {
		t.Error("ImportKey: expected error")
	}
	if _, err := svc.ListBuckets(ctx); err == nil {
		t.Error("ListBuckets: expected error")
	}
	if _, err := svc.GetBucketInfo(ctx, "b"); err == nil {
		t.Error("GetBucketInfo: expected error")
	}
	if _, err := svc.GetBucketInfoByAlias(ctx, "a"); err == nil {
		t.Error("GetBucketInfoByAlias: expected error")
	}
	if _, err := svc.CreateBucket(ctx, models.CreateBucketAdminRequest{}); err == nil {
		t.Error("CreateBucket: expected error")
	}
	if _, err := svc.UpdateBucket(ctx, "b", models.UpdateBucketRequest{}); err == nil {
		t.Error("UpdateBucket: expected error")
	}
	if err := svc.DeleteBucket(ctx, "b"); err == nil {
		t.Error("DeleteBucket: expected error")
	}
	if _, err := svc.AllowBucketKey(ctx, models.BucketKeyPermRequest{}); err == nil {
		t.Error("AllowBucketKey: expected error")
	}
	if _, err := svc.DenyBucketKey(ctx, models.BucketKeyPermRequest{}); err == nil {
		t.Error("DenyBucketKey: expected error")
	}
	alias := "a"
	if _, err := svc.AddBucketAlias(ctx, models.AddBucketAliasRequest{BucketID: "b", GlobalAlias: &alias}); err == nil {
		t.Error("AddBucketAlias: expected error")
	}
	if _, err := svc.RemoveBucketAlias(ctx, models.RemoveBucketAliasRequest{BucketID: "b", GlobalAlias: &alias}); err == nil {
		t.Error("RemoveBucketAlias: expected error")
	}
	if _, err := svc.GetClusterHealth(ctx); err == nil {
		t.Error("GetClusterHealth: expected error")
	}
	if _, err := svc.GetClusterStatus(ctx); err == nil {
		t.Error("GetClusterStatus: expected error")
	}
	if err := svc.HealthCheck(ctx); err == nil {
		t.Error("HealthCheck: expected error")
	}
	if _, err := svc.GetMetrics(ctx); err == nil {
		t.Error("GetMetrics: expected error")
	}
}

func TestV1_RequestFailurePaths(t *testing.T) {
	// Use a cancelled context to make doRequest fail immediately (no retry wait).
	svc, _ := newV1RecordingServer(t, 200, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	if _, err := svc.ListKeys(ctx); err == nil {
		t.Error("ListKeys: expected error")
	}
	if _, err := svc.CreateKey(ctx, models.CreateKeyRequest{}); err == nil {
		t.Error("CreateKey: expected error")
	}
	if _, err := svc.GetKeyInfo(ctx, "k", false); err == nil {
		t.Error("GetKeyInfo: expected error")
	}
	if _, err := svc.UpdateKey(ctx, "k", models.UpdateKeyRequest{}); err == nil {
		t.Error("UpdateKey: expected error")
	}
	if err := svc.DeleteKey(ctx, "k"); err == nil {
		t.Error("DeleteKey: expected error")
	}
	if _, err := svc.ImportKey(ctx, models.ImportKeyRequest{}); err == nil {
		t.Error("ImportKey: expected error")
	}
	if _, err := svc.ListBuckets(ctx); err == nil {
		t.Error("ListBuckets: expected error")
	}
	if _, err := svc.GetBucketInfo(ctx, "b"); err == nil {
		t.Error("GetBucketInfo: expected error")
	}
	if _, err := svc.GetBucketInfoByAlias(ctx, "a"); err == nil {
		t.Error("GetBucketInfoByAlias: expected error")
	}
	if _, err := svc.CreateBucket(ctx, models.CreateBucketAdminRequest{}); err == nil {
		t.Error("CreateBucket: expected error")
	}
	if _, err := svc.UpdateBucket(ctx, "b", models.UpdateBucketRequest{}); err == nil {
		t.Error("UpdateBucket: expected error")
	}
	if err := svc.DeleteBucket(ctx, "b"); err == nil {
		t.Error("DeleteBucket: expected error")
	}
	if _, err := svc.AllowBucketKey(ctx, models.BucketKeyPermRequest{}); err == nil {
		t.Error("AllowBucketKey: expected error")
	}
	if _, err := svc.DenyBucketKey(ctx, models.BucketKeyPermRequest{}); err == nil {
		t.Error("DenyBucketKey: expected error")
	}
	alias := "a"
	if _, err := svc.AddBucketAlias(ctx, models.AddBucketAliasRequest{BucketID: "b", GlobalAlias: &alias}); err == nil {
		t.Error("AddBucketAlias: expected error")
	}
	if _, err := svc.RemoveBucketAlias(ctx, models.RemoveBucketAliasRequest{BucketID: "b", GlobalAlias: &alias}); err == nil {
		t.Error("RemoveBucketAlias: expected error")
	}
	if _, err := svc.GetClusterHealth(ctx); err == nil {
		t.Error("GetClusterHealth: expected error")
	}
	if _, err := svc.GetClusterStatus(ctx); err == nil {
		t.Error("GetClusterStatus: expected error")
	}
	if err := svc.HealthCheck(ctx); err == nil {
		t.Error("HealthCheck: expected error")
	}
	if _, err := svc.GetMetrics(ctx); err == nil {
		t.Error("GetMetrics: expected error")
	}
}

func TestV1_AddBucketAlias_MissingFields(t *testing.T) {
	svc, _ := newV1RecordingServer(t, 200, nil)
	// Neither globalAlias nor localAlias set
	_, err := svc.AddBucketAlias(context.Background(), models.AddBucketAliasRequest{BucketID: "b"})
	if err == nil {
		t.Fatal("expected error for missing alias fields")
	}
}

func TestV1_RemoveBucketAlias_MissingFields(t *testing.T) {
	svc, _ := newV1RecordingServer(t, 200, nil)
	_, err := svc.RemoveBucketAlias(context.Background(), models.RemoveBucketAliasRequest{BucketID: "b"})
	if err == nil {
		t.Fatal("expected error for missing alias fields")
	}
}

func TestV1_RemoveBucketAlias_Local(t *testing.T) {
	want := &models.GarageBucketInfo{ID: "b1"}
	svc, rec := newV1RecordingServer(t, 200, want)
	alias := "localname"
	keyID := "GK1"
	_, err := svc.RemoveBucketAlias(context.Background(), models.RemoveBucketAliasRequest{
		BucketID: "b1", LocalAlias: &alias, AccessKeyID: &keyID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodDelete || rec.path != "/v1/bucket/alias/local" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestV1_GetMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte("garage_up 1\n"))
	}))
	t.Cleanup(srv.Close)
	svc := NewGarageV1AdminService(&config.GarageConfig{
		AdminEndpoint: srv.URL,
		AdminToken:    "tok",
	}, "")

	got, err := svc.GetMetrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "garage_up") {
		t.Errorf("got %q", got)
	}
}
