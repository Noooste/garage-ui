package services

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
)

// newAdminTestServer wires an httptest.Server (with the supplied handler) to a
// fresh *GarageV2AdminService configured with a known bearer token.
func newAdminTestServer(t *testing.T, handler http.Handler) (*GarageV2AdminService, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	svc := NewGarageV2AdminService(&config.GarageConfig{
		AdminEndpoint: srv.URL,
		AdminToken:    "test-token-xyz",
	}, "")
	return svc, srv
}

// jsonHandler returns an http.Handler that writes the supplied status code and
// JSON-encoded body.
func jsonHandler(t *testing.T, status int, body any) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body == nil {
			return
		}
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("server: encode response: %v", err)
		}
	})
}

type recordedRequest struct {
	method string
	path   string
	rawURL string
	auth   string
	body   []byte
}

func recordingHandler(t *testing.T, status int, respBody any) (http.Handler, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.rawURL = r.URL.RequestURI()
		rec.auth = r.Header.Get("Authorization")
		if r.Body != nil {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("server: read body: %v", err)
			}
			rec.body = b
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if respBody != nil {
			_ = json.NewEncoder(w).Encode(respBody)
		}
	})
	return h, rec
}

func TestHealthCheck_Success(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, nil)
	svc, _ := newAdminTestServer(t, h)

	if err := svc.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if rec.method != http.MethodGet {
		t.Errorf("method = %q, want GET", rec.method)
	}
	if rec.path != "/health" {
		t.Errorf("path = %q, want /health", rec.path)
	}
	if rec.auth != "Bearer test-token-xyz" {
		t.Errorf("Authorization = %q, want Bearer test-token-xyz", rec.auth)
	}
}

func TestHealthCheck_Non2xxReturnsError(t *testing.T) {
	h := jsonHandler(t, http.StatusServiceUnavailable, map[string]string{"error": "down"})
	svc, _ := newAdminTestServer(t, h)

	err := svc.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error for 503, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention status code, got %v", err)
	}
}

func TestListKeys_Success(t *testing.T) {
	want := []models.ListKeysResponseItem{
		{ID: "k1", Name: "alice", Expired: false},
		{ID: "k2", Name: "bob", Expired: true},
	}
	h, rec := recordingHandler(t, http.StatusOK, want)
	svc, _ := newAdminTestServer(t, h)

	got, err := svc.ListKeys(context.Background())
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if rec.method != http.MethodGet || rec.path != "/v2/ListKeys" {
		t.Errorf("request = %s %s, want GET /v2/ListKeys", rec.method, rec.path)
	}
	if len(got) != 2 || got[0].ID != "k1" || got[1].Name != "bob" {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCreateKey_PostsBodyAndDecodesResponse(t *testing.T) {
	name := "service-account"
	want := &models.GarageKeyInfo{
		AccessKeyID: "GK123",
		Name:        name,
	}
	h, rec := recordingHandler(t, http.StatusOK, want)
	svc, _ := newAdminTestServer(t, h)

	req := models.CreateKeyRequest{Name: &name, NeverExpires: true}
	got, err := svc.CreateKey(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/v2/CreateKey" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	var sent models.CreateKeyRequest
	if err := json.Unmarshal(rec.body, &sent); err != nil {
		t.Fatalf("unmarshal sent body: %v (raw=%q)", err, rec.body)
	}
	if sent.Name == nil || *sent.Name != name {
		t.Errorf("sent name = %v, want %q", sent.Name, name)
	}
	if !sent.NeverExpires {
		t.Errorf("NeverExpires should round-trip true")
	}
	if got.AccessKeyID != want.AccessKeyID {
		t.Errorf("AccessKeyID = %q, want %q", got.AccessKeyID, want.AccessKeyID)
	}
}

func TestGetKeyInfo_PassesShowSecretQueryParam(t *testing.T) {
	cases := []struct {
		name         string
		showSecret   bool
		wantInRawURL string
	}{
		{"without secret", false, "?id=ABC"},
		{"with secret", true, "?id=ABC&showSecretKey=true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, rec := recordingHandler(t, http.StatusOK, &models.GarageKeyInfo{AccessKeyID: "ABC"})
			svc, _ := newAdminTestServer(t, h)

			_, err := svc.GetKeyInfo(context.Background(), "ABC", tc.showSecret)
			if err != nil {
				t.Fatalf("GetKeyInfo: %v", err)
			}
			if rec.method != http.MethodGet {
				t.Errorf("method = %q, want GET", rec.method)
			}
			if !strings.Contains(rec.rawURL, tc.wantInRawURL) {
				t.Errorf("raw URL = %q, want substring %q", rec.rawURL, tc.wantInRawURL)
			}
		})
	}
}

func TestUpdateKey_UsesIDInQuery(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, &models.GarageKeyInfo{AccessKeyID: "K"})
	svc, _ := newAdminTestServer(t, h)

	if _, err := svc.UpdateKey(context.Background(), "K", models.UpdateKeyRequest{}); err != nil {
		t.Fatalf("UpdateKey: %v", err)
	}
	if rec.method != http.MethodPost {
		t.Errorf("method = %q, want POST", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=K") {
		t.Errorf("raw URL = %q, want substring id=K", rec.rawURL)
	}
}

func TestDeleteKey_NoBodyOnSuccess(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, nil)
	svc, _ := newAdminTestServer(t, h)

	if err := svc.DeleteKey(context.Background(), "K"); err != nil {
		t.Fatalf("DeleteKey: %v", err)
	}
	if rec.method != http.MethodPost {
		t.Errorf("method = %q, want POST", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=K") {
		t.Errorf("raw URL = %q", rec.rawURL)
	}
}

func TestImportKey_PostsToImportEndpoint(t *testing.T) {
	want := &models.GarageKeyInfo{AccessKeyID: "imported"}
	h, rec := recordingHandler(t, http.StatusOK, want)
	svc, _ := newAdminTestServer(t, h)

	req := models.ImportKeyRequest{
		AccessKeyID:     "imported",
		SecretAccessKey: "shhh",
	}
	got, err := svc.ImportKey(context.Background(), req)
	if err != nil {
		t.Fatalf("ImportKey: %v", err)
	}
	if rec.path != "/v2/ImportKey" {
		t.Errorf("path = %q", rec.path)
	}
	if got.AccessKeyID != "imported" {
		t.Errorf("AccessKeyID = %q, want imported", got.AccessKeyID)
	}
}

func TestListBuckets_Success(t *testing.T) {
	want := []models.ListBucketsResponseItem{{ID: "b1"}, {ID: "b2"}}
	h, rec := recordingHandler(t, http.StatusOK, want)
	svc, _ := newAdminTestServer(t, h)

	got, err := svc.ListBuckets(context.Background())
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if rec.path != "/v2/ListBuckets" {
		t.Errorf("path = %q", rec.path)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestGetBucketInfo_UsesIDQuery(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, &models.GarageBucketInfo{ID: "B"})
	svc, _ := newAdminTestServer(t, h)
	if _, err := svc.GetBucketInfo(context.Background(), "B"); err != nil {
		t.Fatalf("GetBucketInfo: %v", err)
	}
	if !strings.Contains(rec.rawURL, "id=B") {
		t.Errorf("raw URL = %q", rec.rawURL)
	}
}

func TestGetBucketInfoByAlias_UsesGlobalAliasQuery(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, &models.GarageBucketInfo{ID: "B", GlobalAliases: []string{"my-bucket"}})
	svc, _ := newAdminTestServer(t, h)
	got, err := svc.GetBucketInfoByAlias(context.Background(), "my-bucket")
	if err != nil {
		t.Fatalf("GetBucketInfoByAlias: %v", err)
	}
	if !strings.Contains(rec.rawURL, "globalAlias=my-bucket") {
		t.Errorf("raw URL = %q", rec.rawURL)
	}
	if got.ID != "B" {
		t.Errorf("ID = %q, want B", got.ID)
	}
}

func TestCreateBucket_PostsBody(t *testing.T) {
	want := &models.GarageBucketInfo{ID: "new"}
	h, rec := recordingHandler(t, http.StatusOK, want)
	svc, _ := newAdminTestServer(t, h)

	if _, err := svc.CreateBucket(context.Background(), models.CreateBucketAdminRequest{}); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/v2/CreateBucket" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestUpdateBucket_UsesIDQueryAndPostsBody(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, &models.GarageBucketInfo{ID: "B"})
	svc, _ := newAdminTestServer(t, h)
	if _, err := svc.UpdateBucket(context.Background(), "B", models.UpdateBucketRequest{}); err != nil {
		t.Fatalf("UpdateBucket: %v", err)
	}
	if rec.method != http.MethodPost {
		t.Errorf("method = %q", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=B") {
		t.Errorf("raw URL = %q", rec.rawURL)
	}
}

func TestDeleteBucket_PostWithIDQuery(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, nil)
	svc, _ := newAdminTestServer(t, h)
	if err := svc.DeleteBucket(context.Background(), "B"); err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}
	if rec.method != http.MethodPost {
		t.Errorf("method = %q", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=B") {
		t.Errorf("raw URL = %q", rec.rawURL)
	}
}

func TestBucketAliasAndPermissionEndpoints(t *testing.T) {
	cases := []struct {
		name string
		fn   func(s *GarageV2AdminService) error
		path string
	}{
		{
			name: "AddBucketAlias",
			fn: func(s *GarageV2AdminService) error {
				_, err := s.AddBucketAlias(context.Background(), models.AddBucketAliasRequest{})
				return err
			},
			path: "/v2/AddBucketAlias",
		},
		{
			name: "RemoveBucketAlias",
			fn: func(s *GarageV2AdminService) error {
				_, err := s.RemoveBucketAlias(context.Background(), models.RemoveBucketAliasRequest{})
				return err
			},
			path: "/v2/RemoveBucketAlias",
		},
		{
			name: "AllowBucketKey",
			fn: func(s *GarageV2AdminService) error {
				_, err := s.AllowBucketKey(context.Background(), models.BucketKeyPermRequest{})
				return err
			},
			path: "/v2/AllowBucketKey",
		},
		{
			name: "DenyBucketKey",
			fn: func(s *GarageV2AdminService) error {
				_, err := s.DenyBucketKey(context.Background(), models.BucketKeyPermRequest{})
				return err
			},
			path: "/v2/DenyBucketKey",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, rec := recordingHandler(t, http.StatusOK, &models.GarageBucketInfo{ID: "B"})
			svc, _ := newAdminTestServer(t, h)
			if err := tc.fn(svc); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if rec.path != tc.path || rec.method != http.MethodPost {
				t.Errorf("request = %s %s, want POST %s", rec.method, rec.path, tc.path)
			}
		})
	}
}

func TestClusterEndpoints(t *testing.T) {
	cases := []struct {
		name string
		fn   func(s *GarageV2AdminService) error
		path string
	}{
		{
			name: "GetClusterHealth",
			fn: func(s *GarageV2AdminService) error {
				_, err := s.GetClusterHealth(context.Background())
				return err
			},
			path: "/v2/GetClusterHealth",
		},
		{
			name: "GetClusterStatus",
			fn: func(s *GarageV2AdminService) error {
				_, err := s.GetClusterStatus(context.Background())
				return err
			},
			path: "/v2/GetClusterStatus",
		},
		{
			name: "GetClusterStatistics",
			fn: func(s *GarageV2AdminService) error {
				_, err := s.GetClusterStatistics(context.Background())
				return err
			},
			path: "/v2/GetClusterStatistics",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, rec := recordingHandler(t, http.StatusOK, struct{}{})
			svc, _ := newAdminTestServer(t, h)
			if err := tc.fn(svc); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if rec.method != http.MethodGet {
				t.Errorf("method = %q, want GET", rec.method)
			}
			if rec.path != tc.path {
				t.Errorf("path = %q, want %q", rec.path, tc.path)
			}
		})
	}
}

func TestNodeEndpoints_NodeIDInQuery(t *testing.T) {
	cases := []struct {
		name string
		fn   func(s *GarageV2AdminService, id string) error
		path string
	}{
		{
			name: "GetNodeInfo",
			fn: func(s *GarageV2AdminService, id string) error {
				_, err := s.GetNodeInfo(context.Background(), id)
				return err
			},
			path: "/v2/GetNodeInfo",
		},
		{
			name: "GetNodeStatistics",
			fn: func(s *GarageV2AdminService, id string) error {
				_, err := s.GetNodeStatistics(context.Background(), id)
				return err
			},
			path: "/v2/GetNodeStatistics",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, rec := recordingHandler(t, http.StatusOK, struct{}{})
			svc, _ := newAdminTestServer(t, h)
			if err := tc.fn(svc, "node-7"); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if rec.path != tc.path {
				t.Errorf("path = %q, want %q", rec.path, tc.path)
			}
			if !strings.Contains(rec.rawURL, "node=node-7") {
				t.Errorf("raw URL = %q, want substring node=node-7", rec.rawURL)
			}
		})
	}
}

func TestGetMetrics_ReturnsRawBodyOn2xx(t *testing.T) {
	body := "# HELP garage_up Number of running nodes\ngarage_up 3\n"
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	})
	svc, _ := newAdminTestServer(t, h)

	got, err := svc.GetMetrics(context.Background())
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	if got != body {
		t.Errorf("GetMetrics body mismatch:\n got %q\nwant %q", got, body)
	}
}

func TestGetMetrics_Non2xxReturnsErrorWithStatus(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "forbidden")
	})
	svc, _ := newAdminTestServer(t, h)

	_, err := svc.GetMetrics(context.Background())
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention 403, got %v", err)
	}
}

func TestDoRequest_Non2xxBodyEchoedInError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"bad request"}`)
	})
	svc, _ := newAdminTestServer(t, h)

	_, err := svc.ListKeys(context.Background())
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "bad request") {
		t.Errorf("error %v should contain 400 and the response body", err)
	}
}

func TestDoRequest_MalformedJSONReturnsDecodeError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "not-json-at-all")
	})
	svc, _ := newAdminTestServer(t, h)

	_, err := svc.ListKeys(context.Background())
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("error %v should mention decoding", err)
	}
}

// TestAllMethods_Non2xxReturnsError exercises the decodeResponse error branch
// of every admin method by pointing them at a server that always returns 500.
// This is a single sweep over the near-identical "if err := decodeResponse ...
// return nil, fmt.Errorf(...)" branches that each wrapper repeats.
func TestAllMethods_Non2xxReturnsError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	svc, _ := newAdminTestServer(t, h)
	ctx := context.Background()

	calls := map[string]func() error{
		"ListKeys":             func() error { _, err := svc.ListKeys(ctx); return err },
		"CreateKey":            func() error { _, err := svc.CreateKey(ctx, models.CreateKeyRequest{}); return err },
		"GetKeyInfo":           func() error { _, err := svc.GetKeyInfo(ctx, "k", false); return err },
		"UpdateKey":            func() error { _, err := svc.UpdateKey(ctx, "k", models.UpdateKeyRequest{}); return err },
		"DeleteKey":            func() error { return svc.DeleteKey(ctx, "k") },
		"ImportKey":            func() error { _, err := svc.ImportKey(ctx, models.ImportKeyRequest{}); return err },
		"ListBuckets":          func() error { _, err := svc.ListBuckets(ctx); return err },
		"GetBucketInfo":        func() error { _, err := svc.GetBucketInfo(ctx, "b"); return err },
		"GetBucketInfoByAlias": func() error { _, err := svc.GetBucketInfoByAlias(ctx, "b"); return err },
		"CreateBucket":         func() error { _, err := svc.CreateBucket(ctx, models.CreateBucketAdminRequest{}); return err },
		"UpdateBucket":         func() error { _, err := svc.UpdateBucket(ctx, "b", models.UpdateBucketRequest{}); return err },
		"DeleteBucket":         func() error { return svc.DeleteBucket(ctx, "b") },
		"AddBucketAlias":       func() error { _, err := svc.AddBucketAlias(ctx, models.AddBucketAliasRequest{}); return err },
		"RemoveBucketAlias":    func() error { _, err := svc.RemoveBucketAlias(ctx, models.RemoveBucketAliasRequest{}); return err },
		"AllowBucketKey":       func() error { _, err := svc.AllowBucketKey(ctx, models.BucketKeyPermRequest{}); return err },
		"DenyBucketKey":        func() error { _, err := svc.DenyBucketKey(ctx, models.BucketKeyPermRequest{}); return err },
		"GetClusterHealth":     func() error { _, err := svc.GetClusterHealth(ctx); return err },
		"GetClusterStatus":     func() error { _, err := svc.GetClusterStatus(ctx); return err },
		"GetClusterStatistics": func() error { _, err := svc.GetClusterStatistics(ctx); return err },
		"GetNodeInfo":          func() error { _, err := svc.GetNodeInfo(ctx, "n"); return err },
		"GetNodeStatistics":    func() error { _, err := svc.GetNodeStatistics(ctx, "n"); return err },
		"HealthCheck":          func() error { return svc.HealthCheck(ctx) },
	}

	for name, fn := range calls {
		t.Run(name, func(t *testing.T) {
			if err := fn(); err == nil {
				t.Fatalf("%s: expected error on 500, got nil", name)
			}
		})
	}
}

// TestDebugLogLevelEnablesSessionLog exercises the NewGarageV2AdminService
// branch that enables azuretls' session logging when logLevel == "debug".
func TestDebugLogLevelEnablesSessionLog(t *testing.T) {
	svc := NewGarageV2AdminService(&config.GarageConfig{
		AdminEndpoint: "http://127.0.0.1:1",
		AdminToken:    "t",
	}, "debug")
	if svc == nil || svc.httpClient == nil {
		t.Fatal("expected service with configured http client")
	}
}

func TestDoRequest_RetriesExhaustOnConnectionRefused(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	svc := NewGarageV2AdminService(&config.GarageConfig{
		AdminEndpoint: "http://" + addr,
		AdminToken:    "irrelevant",
	}, "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	_, err = svc.ListKeys(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error after retries exhaust, got nil")
	}
	if !strings.Contains(err.Error(), "max retries") {
		t.Errorf("expected max-retries error, got %v", err)
	}
	if elapsed < 200*time.Millisecond {
		t.Errorf("retries returned in %v — backoff loop not engaged", elapsed)
	}
}
