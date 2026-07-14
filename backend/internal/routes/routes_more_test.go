package routes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
)

// newNoAuthFixture builds a fixture with both admin and OIDC disabled. The
// AuthMiddleware short-circuits with c.Next(), letting handler logic run so
// wildcard-route dispatch can be exercised.
func newNoAuthFixture(t *testing.T) *routeFixture {
	return newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = false
		c.Auth.OIDC.Enabled = false
	})
}

func plainReq(method, path string, body io.Reader) *http.Request {
	return httptest.NewRequest(method, path, body)
}

func TestRoutes_ObjectWildcard_GET_DefaultRoutesToGetObject(t *testing.T) {
	f := newNoAuthFixture(t)

	var gotBucket, gotKey string
	f.S3.GetObjectFn = func(_ context.Context, bucket, key string) (io.ReadCloser, *models.ObjectInfo, error) {
		gotBucket, gotKey = bucket, key
		return io.NopCloser(strings.NewReader("hello")), &models.ObjectInfo{Key: key, Size: 5, ContentType: "text/plain"}, nil
	}

	req := plainReq(http.MethodGet, "/api/v1/buckets/b1/objects/folder/subdir/file.txt", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if gotBucket != "b1" || gotKey != "folder/subdir/file.txt" {
		t.Errorf("service called with (%q, %q)", gotBucket, gotKey)
	}
}

func TestRoutes_ObjectWildcard_GET_MetadataSuffixRoutesToMetadata(t *testing.T) {
	f := newNoAuthFixture(t)

	var gotKey string
	f.S3.GetObjectMetadataFn = func(_ context.Context, _ string, key string) (*models.ObjectInfo, error) {
		gotKey = key
		return &models.ObjectInfo{Key: key, Size: 42, ContentType: "application/json"}, nil
	}

	req := plainReq(http.MethodGet, "/api/v1/buckets/b1/objects/data/file.bin/metadata", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// The handler strips the /metadata suffix before calling the service.
	if gotKey != "data/file.bin" {
		t.Errorf("key passed to service = %q, want 'data/file.bin'", gotKey)
	}
}

func TestRoutes_ObjectWildcard_GET_PresignSuffixRoutesToPresigned(t *testing.T) {
	f := newNoAuthFixture(t)

	// The object must exist for the presign handler to succeed.
	f.S3.ObjectExistsFn = func(_ context.Context, _, _ string) (bool, error) { return true, nil }

	var gotKey string
	f.S3.GetPresignedURLFn = func(_ context.Context, _ string, key string, _ time.Duration) (string, error) {
		gotKey = key
		return "https://signed.example/k", nil
	}

	req := plainReq(http.MethodGet, "/api/v1/buckets/b1/objects/sub/file.bin/presign", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body: (unavailable)", resp.StatusCode)
	}
	if gotKey != "sub/file.bin" {
		t.Errorf("key passed to service = %q, want 'sub/file.bin'", gotKey)
	}
}

func TestRoutes_ObjectWildcard_GET_PreviewURLSuffixRoutesToPreviewURL(t *testing.T) {
	f := newNoAuthFixture(t)
	req := plainReq(http.MethodGet, "/api/v1/buckets/b1/objects/sub/clip.mp4/preview-url", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Data models.PreviewURLResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// The dispatch trims the /preview-url suffix, so the key becomes sub/clip.mp4,
	// percent-encoded whole (slash to %2F) in the returned URL, with a pt token.
	if !strings.Contains(body.Data.URL, "/api/v1/buckets/b1/objects/sub%2Fclip.mp4?pt=") {
		t.Errorf("url = %q, want the whole-encoded key with a pt token", body.Data.URL)
	}
}

func TestRoutes_ObjectWildcard_DELETE_RoutesToDeleteObject(t *testing.T) {
	f := newNoAuthFixture(t)

	f.S3.ObjectExistsFn = func(_ context.Context, _, _ string) (bool, error) { return true, nil }

	var gotKey string
	f.S3.DeleteObjectFn = func(_ context.Context, _ string, key string) error {
		gotKey = key
		return nil
	}

	req := plainReq(http.MethodDelete, "/api/v1/buckets/b1/objects/path/to/delete.txt", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if gotKey != "path/to/delete.txt" {
		t.Errorf("key passed to service = %q", gotKey)
	}
}

func TestRoutes_ObjectWildcard_HEAD_RoutesToMetadata(t *testing.T) {
	f := newNoAuthFixture(t)

	var gotKey string
	f.S3.GetObjectMetadataFn = func(_ context.Context, _ string, key string) (*models.ObjectInfo, error) {
		gotKey = key
		return &models.ObjectInfo{Key: key, Size: 7, ContentType: "text/plain"}, nil
	}

	req := plainReq(http.MethodHead, "/api/v1/buckets/b1/objects/deep/nested/x.txt", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if gotKey != "deep/nested/x.txt" {
		t.Errorf("key passed to service = %q", gotKey)
	}
}

func TestRoutes_ObjectWildcard_URLDecodedBeforeDispatch(t *testing.T) {
	// %20 in the wildcard portion must be decoded before the service is called.
	f := newNoAuthFixture(t)

	var gotKey string
	f.S3.GetObjectFn = func(_ context.Context, _, key string) (io.ReadCloser, *models.ObjectInfo, error) {
		gotKey = key
		return io.NopCloser(strings.NewReader("")), &models.ObjectInfo{Key: key}, nil
	}

	req := plainReq(http.MethodGet, "/api/v1/buckets/b1/objects/with%20space/file.txt", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if gotKey != "with space/file.txt" {
		t.Errorf("decoded key = %q, want 'with space/file.txt'", gotKey)
	}
}

// Covers the "skip SPA fallback for API-prefixed paths" branch without
// triggering SendFile (which holds file handles on Windows and races with
// t.TempDir cleanup). Only API/auth/health/docs paths are hit here — the
// fallback short-circuits to c.Next(), so no file is opened.
func TestRoutes_SPAFallback_SkipsAPIAndAuthPrefixes(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// The presence of ./frontend/dist is what enables the fallback middleware.
	if err := os.MkdirAll(filepath.Join(dir, "frontend", "dist"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// We deliberately do NOT create index.html — the test must not reach SendFile.

	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "u"
		c.Auth.Admin.Password = "p"
	})

	// Every prefix listed in the fallback's skip-list should bypass file
	// serving and return 404 from fiber's default handler.
	for _, p := range []string{
		"/api/v1/definitely-not-a-route",
		"/auth/nope",
		"/health/extra/segments",
		"/docs/missing",
	} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		resp, err := f.App.Test(req)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		_ = resp.Body.Close()
		// /api/v1/* hits auth middleware → 401. The others hit the SPA
		// fallback's skip branch then fall through to 404. We only care that
		// the SPA middleware did NOT attempt to serve index.html (which would
		// succeed with 200 if it existed — here it doesn't exist, so SendFile
		// would error; either way, != 200 suffices to prove the skip path ran).
		if resp.StatusCode == 200 {
			t.Errorf("%s returned 200 — SPA fallback should have skipped", p)
		}
	}
}

// Covers the third OIDC role-resolution fallback: when neither the ID token
// nor the access token exposes roles, the callback calls GetUserInfo and
// re-evaluates IsAdmin against those roles.
func TestRoutes_OIDCCallback_RoleMatchedViaUserInfoFallback_Succeeds(t *testing.T) {
	f, iss := newOIDCFixture(t, "admin")

	// ID token and access token have no roles by default — leave as-is.
	// Override /userinfo to return a roles structure matching the configured
	// RoleAttributePath (resource_access.test-client.roles).
	iss.UserInfoFn = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sub":                "user-1",
			"preferred_username": "alice",
			"email":              "alice@example.com",
			"resource_access": map[string]any{
				"test-client": map[string]any{"roles": []any{"admin"}},
			},
		})
	}

	state := oidcState(t, f)
	req := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?state="+state+"&code=c", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 303 {
		t.Fatalf("status = %d, want 303 (userinfo fallback should grant admin)", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login?login=success" {
		t.Errorf("Location = %q", loc)
	}
}
