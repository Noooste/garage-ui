package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/authz"
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/handlers"
	"Noooste/garage-ui/internal/services"
	"Noooste/garage-ui/internal/services/mocks"

	"github.com/gofiber/fiber/v3"
)

// routeFixture bundles everything a routes test needs.
type routeFixture struct {
	App   *fiber.App
	Admin *mocks.AdminMock
	S3    *mocks.S3Mock
	Auth  *auth.Service
	Cfg   *config.Config
}

// newTestApp builds a fully-wired fiber.App via SetupRoutes. The cfgMutator
// lets each test flip Admin/OIDC/CORS flags before the auth.Service is
// constructed. If the mutator sets OIDC.Enabled=true it MUST set
// OIDC.IssuerURL + Scopes + AdminRole + ClientID so NewAuthService can dial
// the issuer — typically via the testIssuer fixture.
func newTestApp(t *testing.T, cfgMutator func(*config.Config)) *routeFixture {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			Environment: "test",
		},
		Auth: config.AuthConfig{},
		CORS: config.CORSConfig{},
	}
	if cfgMutator != nil {
		cfgMutator(cfg)
	}

	svc, err := auth.NewAuthService(&cfg.Auth, &cfg.Server)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}

	admin := &mocks.AdminMock{}
	s3 := &mocks.S3Mock{}

	policy, err := authz.CompilePolicy(nil)
	if err != nil {
		t.Fatalf("CompilePolicy: %v", err)
	}
	az := authz.NewMiddleware(policy, authz.NewTeamResolver(policy, nil), authz.NewAuthorizer())

	app := fiber.New()
	SetupRoutes(
		app,
		cfg,
		svc,
		handlers.NewHealthHandler("test"),
		handlers.NewBucketHandler(admin, s3),
		handlers.NewObjectHandler(s3),
		handlers.NewUserHandler(admin),
		handlers.NewClusterHandler(admin),
		handlers.NewMonitoringHandler(admin, s3),
		handlers.NewCapabilitiesHandler("v2", services.CapabilitiesV2(), false),
		az,
	)

	return &routeFixture{App: app, Admin: admin, S3: s3, Auth: svc, Cfg: cfg}
}

// expectStatus sends req and asserts the status code.
func expectStatus(t *testing.T, app *fiber.App, req *http.Request, want int) *http.Response {
	t.Helper()
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test(%s %s): %v", req.Method, req.URL.Path, err)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s %s: status = %d, want %d", req.Method, req.URL.Path, resp.StatusCode, want)
	}
	return resp
}

func TestRoutes_Registered_NoAuth(t *testing.T) {
	// No auth: every route resolves; auth-specific routes return 404.
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = false
		c.Auth.OIDC.Enabled = false
	})

	// Public routes reachable → not 404. Status may be anything except 404.
	for _, tc := range []struct {
		method, path string
	}{
		{"GET", "/health"},
		{"GET", "/api/v1/health"},
		{"GET", "/auth/config"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp, err := f.App.Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		if resp.StatusCode == 404 {
			t.Errorf("%s %s returned 404 — route not registered", tc.method, tc.path)
		}
	}

	// Auth-specific routes must 404 when both auth methods disabled.
	for _, tc := range []struct {
		method, path string
	}{
		{"POST", "/auth/login"},
		{"GET", "/auth/me"},
		{"GET", "/auth/oidc/login"},
		{"GET", "/auth/oidc/callback"},
		{"POST", "/auth/oidc/logout"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		expectStatus(t, f.App, req, 404)
	}
}

func TestRoutes_Registered_AdminOnly(t *testing.T) {
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
	})

	// /auth/login and /auth/me present; /auth/oidc/* not.
	for _, tc := range []struct {
		method, path string
	}{
		{"POST", "/auth/login"},
		{"GET", "/auth/me"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp, err := f.App.Test(req)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if resp.StatusCode == 404 {
			t.Errorf("%s %s returned 404", tc.method, tc.path)
		}
	}
	for _, tc := range []struct {
		method, path string
	}{
		{"GET", "/auth/oidc/login"},
		{"GET", "/auth/oidc/callback"},
		{"POST", "/auth/oidc/logout"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		expectStatus(t, f.App, req, 404)
	}
}

func TestRoutes_UnknownPath_Returns404(t *testing.T) {
	f := newTestApp(t, nil)
	req := httptest.NewRequest("GET", "/this/does/not/exist", nil)
	expectStatus(t, f.App, req, 404)
}

func TestRoutes_AllAPIRoutesRegistered(t *testing.T) {
	// With Admin enabled, hit every path/method declared in SetupRoutes.
	// A route being "registered" = status != 404. Specific behavior depends
	// on auth (401) or mock setup (500 from errNotConfigured) — we only
	// care that fiber routed the request.
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
	})

	cases := []struct {
		method, path string
	}{
		// Buckets
		{"GET", "/api/v1/buckets/"},
		{"POST", "/api/v1/buckets/"},
		{"GET", "/api/v1/buckets/b1"},
		{"DELETE", "/api/v1/buckets/b1"},
		{"POST", "/api/v1/buckets/b1/permissions"},
		{"PUT", "/api/v1/buckets/b1/website"},
		// Objects (listing + uploads)
		{"GET", "/api/v1/buckets/b1/objects/"},
		{"POST", "/api/v1/buckets/b1/objects/"},
		{"POST", "/api/v1/buckets/b1/objects/upload-multiple"},
		{"POST", "/api/v1/buckets/b1/objects/delete-multiple"},
		// Object wildcard routes
		{"GET", "/api/v1/buckets/b1/objects/folder/file.txt"},
		{"GET", "/api/v1/buckets/b1/objects/folder/file.txt/metadata"},
		{"GET", "/api/v1/buckets/b1/objects/folder/file.txt/presign"},
		{"DELETE", "/api/v1/buckets/b1/objects/folder/file.txt"},
		{"HEAD", "/api/v1/buckets/b1/objects/folder/file.txt"},
		// Users
		{"GET", "/api/v1/users/"},
		{"POST", "/api/v1/users/"},
		{"GET", "/api/v1/users/AKIA"},
		{"GET", "/api/v1/users/AKIA/secret"},
		{"DELETE", "/api/v1/users/AKIA"},
		{"PATCH", "/api/v1/users/AKIA"},
		// Cluster
		{"GET", "/api/v1/cluster/health"},
		{"GET", "/api/v1/cluster/status"},
		{"GET", "/api/v1/cluster/statistics"},
		{"GET", "/api/v1/cluster/nodes/n1"},
		{"GET", "/api/v1/cluster/nodes/n1/statistics"},
		// Monitoring
		{"GET", "/api/v1/monitoring/metrics"},
		{"GET", "/api/v1/monitoring/admin-health"},
		{"GET", "/api/v1/monitoring/dashboard"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			resp, err := f.App.Test(req)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if resp.StatusCode == 404 {
				t.Errorf("route not registered (404): %s %s", tc.method, tc.path)
			}
		})
	}
}

func TestRoutes_AuthRequired_For_APIRoutes(t *testing.T) {
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
	})

	// Unauthenticated requests to /api/v1/* must return 401 — auth
	// middleware must run before the handler.
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/v1/buckets/"},
		{"GET", "/api/v1/users/"},
		{"GET", "/api/v1/cluster/health"},
		{"GET", "/api/v1/monitoring/metrics"},
		// Object wildcard routes register AuthMiddleware separately — prove
		// that wiring isn't missed for any of GET/DELETE/HEAD.
		{"GET", "/api/v1/buckets/b/objects/k"},
		{"DELETE", "/api/v1/buckets/b/objects/k"},
		{"HEAD", "/api/v1/buckets/b/objects/k"},
	} {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			expectStatus(t, f.App, req, 401)
		})
	}
}

func TestRoutes_NoAuth_On_PublicRoutes(t *testing.T) {
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
	})
	for _, tc := range []struct{ method, path string }{
		{"GET", "/health"},
		{"GET", "/api/v1/health"},
		{"GET", "/auth/config"},
	} {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			resp, err := f.App.Test(req)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if resp.StatusCode == 401 {
				t.Errorf("public route unexpectedly returned 401: %s", tc.path)
			}
		})
	}
}

func TestRoutes_CORS_Preflight_PassesBeforeAuth(t *testing.T) {
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
		c.CORS = config.CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"https://ui.example"},
			AllowedMethods: []string{"GET", "POST"},
		}
	})

	req := httptest.NewRequest("OPTIONS", "/api/v1/buckets/", nil)
	req.Header.Set("Origin", "https://ui.example")
	resp := expectStatus(t, f.App, req, 204)
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://ui.example" {
		t.Errorf("Allow-Origin = %q — CORS should have run before auth", got)
	}
}

// newOIDCFixture builds a route fixture with OIDC enabled pointing at a
// running testIssuer. The `adminRole` is applied to cfg.Auth.OIDC.AdminRole
// (pass empty to disable the role gate).
func newOIDCFixture(t *testing.T, adminRole string) (*routeFixture, *testIssuer) {
	t.Helper()
	iss := newTestIssuer(t)
	f := newTestApp(t, func(c *config.Config) {
		c.Server.RootURL = "https://app.example"
		c.Auth.OIDC = config.OIDCConfig{
			Enabled:           true,
			ClientID:          iss.ClientID,
			ClientSecret:      "secret",
			IssuerURL:         iss.Server.URL,
			Scopes:            []string{"openid", "profile", "email"},
			AdminRole:         adminRole,
			UsernameAttribute: "preferred_username",
			EmailAttribute:    "email",
			NameAttribute:     "name",
			RoleAttributePath: "resource_access.test-client.roles",
			CookieName:        "session",
			CookieSecure:      false,
			CookieHTTPOnly:    true,
			CookieSameSite:    "Lax",
			SessionMaxAge:     3600,
		}
	})
	return f, iss
}

func TestRoutes_OIDCLogin_RedirectsToAuthorizeEndpoint(t *testing.T) {
	f, iss := newOIDCFixture(t, "admin")

	req := httptest.NewRequest("GET", "/auth/oidc/login", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 303 {
		t.Fatalf("status = %d, want 303", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, iss.Server.URL+"/authorize") {
		t.Errorf("Location = %q, want prefix %s/authorize", loc, iss.Server.URL)
	}
	if !strings.Contains(loc, "state=") {
		t.Errorf("Location missing state param: %s", loc)
	}
	if !strings.Contains(loc, "client_id=test-client") {
		t.Errorf("Location missing client_id: %s", loc)
	}
}

func TestRoutes_Registered_OIDCOnly(t *testing.T) {
	f, _ := newOIDCFixture(t, "admin")

	// /auth/oidc/* registered
	for _, tc := range []struct{ method, path string }{
		{"GET", "/auth/oidc/login"},
		{"GET", "/auth/oidc/callback"},
		{"POST", "/auth/oidc/logout"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp, err := f.App.Test(req)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if resp.StatusCode == 404 {
			t.Errorf("%s %s not registered", tc.method, tc.path)
		}
	}

	// /auth/login must be 404 — admin disabled.
	req := httptest.NewRequest("POST", "/auth/login", nil)
	expectStatus(t, f.App, req, 404)
}

// oidcState returns a fresh state token minted by the fixture's auth service.
func oidcState(t *testing.T, f *routeFixture) string {
	t.Helper()
	s, err := f.Auth.GenerateStateToken()
	if err != nil {
		t.Fatalf("GenerateStateToken: %v", err)
	}
	return s
}

func TestRoutes_OIDCCallback_MissingState_Returns400(t *testing.T) {
	f, _ := newOIDCFixture(t, "admin")
	req := httptest.NewRequest("GET", "/auth/oidc/callback", nil)
	expectStatus(t, f.App, req, 400)
}

func TestRoutes_OIDCCallback_InvalidState_Returns400(t *testing.T) {
	f, _ := newOIDCFixture(t, "admin")
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state=not-a-valid-state&code=c", nil)
	expectStatus(t, f.App, req, 400)
}

func TestRoutes_OIDCCallback_MissingCode_Returns400(t *testing.T) {
	f, _ := newOIDCFixture(t, "admin")
	state := oidcState(t, f)
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state="+state, nil)
	expectStatus(t, f.App, req, 400)
}

func TestRoutes_OIDCCallback_TokenExchangeFails_Returns401(t *testing.T) {
	f, iss := newOIDCFixture(t, "admin")
	iss.TokenError = "invalid_grant"
	defer func() { iss.TokenError = "" }()

	state := oidcState(t, f)
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state="+state+"&code=c", nil)
	expectStatus(t, f.App, req, 401)
}

func TestRoutes_OIDCCallback_MissingIDToken_Returns401(t *testing.T) {
	f, iss := newOIDCFixture(t, "admin")
	iss.OmitIDToken = true
	defer func() { iss.OmitIDToken = false }()

	state := oidcState(t, f)
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state="+state+"&code=c", nil)
	expectStatus(t, f.App, req, 401)
}

func TestRoutes_OIDCCallback_BadIDTokenSignature_Returns401(t *testing.T) {
	f, iss := newOIDCFixture(t, "admin")
	iss.SignIDTokenWithWrongKey = true
	defer func() { iss.SignIDTokenWithWrongKey = false }()

	state := oidcState(t, f)
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state="+state+"&code=c", nil)
	expectStatus(t, f.App, req, 401)
}

func TestRoutes_OIDCCallback_RoleGateDenies_Returns403(t *testing.T) {
	f, _ := newOIDCFixture(t, "admin") // AdminRole set; no roles anywhere
	state := oidcState(t, f)
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state="+state+"&code=c", nil)
	expectStatus(t, f.App, req, 403)
}

func TestRoutes_OIDCCallback_NoRoleGate_HappyPathSetsCookieAndRedirects(t *testing.T) {
	f, _ := newOIDCFixture(t, "") // AdminRole empty → role gate skipped
	state := oidcState(t, f)

	req := httptest.NewRequest("GET", "/auth/oidc/callback?state="+state+"&code=c", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 303 {
		t.Fatalf("status = %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login?login=success" {
		t.Errorf("Location = %q", loc)
	}

	cookies := resp.Cookies()
	var sess *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sess = c
			break
		}
	}
	if sess == nil {
		t.Fatalf("no session cookie set: %+v", cookies)
	}
	if sess.Value == "" {
		t.Error("session cookie value empty")
	}
	if !sess.HttpOnly {
		t.Error("session cookie should be HttpOnly")
	}
	if sess.MaxAge != 3600 {
		t.Errorf("MaxAge = %d, want 3600", sess.MaxAge)
	}
}

func TestRoutes_OIDCCallback_RoleMatchedViaAccessTokenFallback_Succeeds(t *testing.T) {
	f, iss := newOIDCFixture(t, "admin")
	// Inject role into the access token so ExtractRolesFromAccessToken returns [admin].
	iss.DefaultAccessClaims = map[string]any{
		"iss": iss.Server.URL,
		"sub": "user-1",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"resource_access": map[string]any{
			"test-client": map[string]any{"roles": []any{"admin"}},
		},
	}

	state := oidcState(t, f)
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state="+state+"&code=c", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 303 {
		t.Fatalf("status = %d, want 303 (access-token role fallback should match)", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login?login=success" {
		t.Errorf("Location = %q", loc)
	}
	var sess *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			sess = c
		}
	}
	if sess == nil || sess.Value == "" {
		t.Fatalf("expected session cookie with value, got %+v", sess)
	}
}

func TestRoutes_OIDCLogout_ClearsCookieAndReturns200(t *testing.T) {
	f, _ := newOIDCFixture(t, "admin")

	req := httptest.NewRequest("POST", "/auth/oidc/logout", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["success"] != true {
		t.Errorf("success = %v, want true", body["success"])
	}
	if body["message"] != "Logged out successfully" {
		t.Errorf("message = %v", body["message"])
	}

	var sess *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			sess = c
		}
	}
	if sess == nil {
		t.Fatalf("expected session cookie in response")
	}
	if sess.MaxAge != -1 {
		t.Errorf("MaxAge = %d, want -1 (cookie cleared)", sess.MaxAge)
	}
	if sess.Value != "" {
		t.Errorf("cookie Value = %q, want empty", sess.Value)
	}
}

func TestRoutes_SPAFallback_NoFrontendDir_DoesNotMount(t *testing.T) {
	// Chdir into an empty temp dir — frontend/dist does not exist, so the
	// SPA fallback is not registered.
	t.Chdir(t.TempDir())

	f := newTestApp(t, nil)

	req := httptest.NewRequest("GET", "/random/spa/path", nil)
	expectStatus(t, f.App, req, 404)
}

func TestRoutes_SPAFallback_WithFrontend_ServesIndexForUnknownPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Fiber's c.SendFile holds a handle on the served file; Windows refuses
		// t.TempDir RemoveAll cleanup. Behavior itself is platform-agnostic and
		// covered on Linux in CI.
		t.Skip("SPA fallback test skipped on Windows due to file-handle cleanup race")
	}
	dir := t.TempDir()
	t.Chdir(dir)

	// Create ./frontend/dist/index.html
	if err := os.MkdirAll(filepath.Join(dir, "frontend", "dist"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	index := filepath.Join(dir, "frontend", "dist", "index.html")
	if err := os.WriteFile(index, []byte("<!doctype html><title>spa</title>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	f := newTestApp(t, nil)

	// Unknown SPA path → serves index.html.
	req := httptest.NewRequest("GET", "/deep/spa/route", nil)
	resp := expectStatus(t, f.App, req, 200)
	body := make([]byte, 64)
	n, _ := resp.Body.Read(body)
	if !strings.Contains(string(body[:n]), "spa") {
		t.Errorf("body = %q, want index.html content", string(body[:n]))
	}

	// API prefix is skipped by the fallback → still 404 for unknown API path.
	req2 := httptest.NewRequest("GET", "/api/v1/definitely-not-a-route", nil)
	expectStatus(t, f.App, req2, 404)
}
