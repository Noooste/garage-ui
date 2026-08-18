package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"Noooste/garage-ui/internal/authz"
	"Noooste/garage-ui/internal/config"
)

// Issue #107: with server.base_path set, the app can be reached through a
// reverse proxy that routes by path.
//
// The contract is "accept the prefix, don't require it": routes stay at the
// root, an incoming prefix is stripped, and everything the browser is told
// (asset base, router basename, OIDC redirect URI, login redirects) carries
// the prefix. That covers both proxy families — the ones that strip the mount
// point before forwarding (tailscale serve, Traefik StripPrefix, k8s
// rewrite-target) and the ones that pass the full path through (nginx
// proxy_pass without a URI part).

const testBasePath = "/garage-ui"

// newBasePathApp builds the fully-wired app with a base path and all auth
// methods that add routes enabled.
func newBasePathApp(t *testing.T, basePath string) *routeFixture {
	t.Helper()
	return newTestApp(t, func(c *config.Config) {
		c.Server.BasePath = basePath
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "u"
		c.Auth.Admin.Password = "p"
		c.Auth.Token.Enabled = true
		c.Auth.MetricsPublic = true
	})
}

// notFound reports whether a request 404s, i.e. no route matched at all.
func notFound(t *testing.T, f *routeFixture, method, path string) bool {
	t.Helper()
	resp, err := f.App.Test(httptest.NewRequest(method, path, nil))
	if err != nil {
		t.Fatalf("app.Test(%s %s): %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusNotFound
}

func TestRoutes_BasePath_RoutesAnswerUnderBothPaths(t *testing.T) {
	f := newBasePathApp(t, testBasePath)

	// 401/405/400 all prove a route matched; only 404 means "not registered".
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/health"},
		{http.MethodGet, "/api/v1/health"},
		{http.MethodGet, "/api/v1/capabilities"},
		{http.MethodGet, "/api/v1/buckets/"},
		{http.MethodPost, "/api/v1/buckets/"},
		{http.MethodGet, "/api/v1/buckets/b/objects/"},
		{http.MethodGet, "/api/v1/buckets/b/objects/some/deep/key.txt"},
		{http.MethodDelete, "/api/v1/buckets/b/objects/some/deep/key.txt"},
		{http.MethodGet, "/api/v1/users/"},
		{http.MethodGet, "/api/v1/cluster/health"},
		{http.MethodGet, "/api/v1/monitoring/metrics"},
		{http.MethodGet, "/auth/config"},
		{http.MethodPost, "/auth/login"},
		{http.MethodPost, "/auth/login-token"},
		{http.MethodGet, "/auth/me"},
		{http.MethodGet, "/metrics"},
		{http.MethodGet, "/docs/index.html"},
	}

	for _, r := range routes {
		// A proxy that forwards the prefix.
		if notFound(t, f, r.method, testBasePath+r.path) {
			t.Errorf("%s %s%s returned 404 — the prefix is not accepted", r.method, testBasePath, r.path)
		}
		// A proxy that strips it, and container probes hitting the port directly.
		if notFound(t, f, r.method, r.path) {
			t.Errorf("%s %s returned 404 — a stripping proxy would break", r.method, r.path)
		}
	}
}

// Same status and body either way: the prefix is a routing detail, not a
// different endpoint.
func TestRoutes_BasePath_PrefixedAndStrippedAgree(t *testing.T) {
	f := newBasePathApp(t, testBasePath)

	for _, path := range []string{"/health", "/api/v1/health", "/auth/config", "/api/v1/buckets/"} {
		prefixed, _ := getBody(t, f, testBasePath+path)
		stripped, _ := getBody(t, f, path)

		if prefixed != stripped {
			t.Errorf("%s: status %d with prefix vs %d without", path, prefixed, stripped)
		}
	}

	// Body equality only where the payload is deterministic — /health carries a
	// timestamp.
	_, prefixedBody := getBody(t, f, testBasePath+"/auth/config")
	_, strippedBody := getBody(t, f, "/auth/config")
	if prefixedBody != strippedBody {
		t.Errorf("/auth/config body differs between the two paths:\n%s\n%s", prefixedBody, strippedBody)
	}
}

func getBody(t *testing.T, f *routeFixture, path string) (int, string) {
	t.Helper()
	resp, err := f.App.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatalf("app.Test(%s): %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(body)
}

// Without a base path nothing may move.
func TestRoutes_NoBasePath_RoutesStayAtRoot(t *testing.T) {
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "u"
		c.Auth.Admin.Password = "p"
	})

	for _, path := range []string{"/health", "/api/v1/health", "/auth/config"} {
		expectStatus(t, f.App, httptest.NewRequest(http.MethodGet, path, nil), http.StatusOK)
	}

	// And an unconfigured prefix is not silently accepted.
	if !notFound(t, f, http.MethodGet, "/garage-ui/api/v1/health") {
		t.Error("/garage-ui/api/v1/health matched without a base path configured")
	}
}

// An unnormalized value from an env var or a templated Helm chart must behave
// exactly like the canonical form.
func TestRoutes_BasePath_UnnormalizedValueBehavesTheSame(t *testing.T) {
	f := newBasePathApp(t, "garage-ui/")

	expectStatus(t, f.App, httptest.NewRequest(http.MethodGet, testBasePath+"/api/v1/health", nil), http.StatusOK)
	expectStatus(t, f.App, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil), http.StatusOK)
}

func TestRoutes_BasePath_NestedPrefix(t *testing.T) {
	f := newBasePathApp(t, "/admin/garage-ui")

	expectStatus(t, f.App, httptest.NewRequest(http.MethodGet, "/admin/garage-ui/api/v1/health", nil), http.StatusOK)

	// A partial prefix is not an entry point.
	if !notFound(t, f, http.MethodGet, "/admin/api/v1/health") {
		t.Error("/admin/api/v1/health matched — only the full prefix may be stripped")
	}
}

// The prefix must only be stripped once. Without a re-entry guard, a base path
// that is also a legal first segment of the stripped path would be eaten twice
// and the request would land on the wrong route.
func TestRoutes_BasePath_StripsThePrefixOnlyOnce(t *testing.T) {
	f := newTestApp(t, func(c *config.Config) {
		c.Server.BasePath = "/api"
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "u"
		c.Auth.Admin.Password = "p"
	})

	// "/api" + "/api/v1/health" — stripping twice would leave "/v1/health".
	expectStatus(t, f.App, httptest.NewRequest(http.MethodGet, "/api/api/v1/health", nil), http.StatusOK)

	if !notFound(t, f, http.MethodGet, "/api/api/api/v1/health") {
		t.Error("a doubly-prefixed path matched — the prefix was stripped more than once")
	}
}

// A prefix must match on a segment boundary, not as a string prefix.
func TestRoutes_BasePath_DoesNotStripPartialSegment(t *testing.T) {
	f := newBasePathApp(t, testBasePath)

	if !notFound(t, f, http.MethodGet, "/garage-ui-other/api/v1/health") {
		t.Error("/garage-ui-other/... was treated as prefixed")
	}
}

// Auth still applies to prefixed requests: the strip must happen before
// routing, not around it.
func TestRoutes_BasePath_APIStillRequiresAuth(t *testing.T) {
	f := newBasePathApp(t, testBasePath)

	for _, path := range []string{
		"/api/v1/buckets/",
		"/api/v1/users/",
		"/api/v1/buckets/b/objects/deep/key.txt",
	} {
		expectStatus(t, f.App, httptest.NewRequest(http.MethodGet, testBasePath+path, nil), http.StatusUnauthorized)
	}
}

// Routes stay at the root, so the fail-closed authz guard keeps working
// unchanged — this locks in that the subpath support did not move /api/v1 out
// from under it.
func TestRoutes_BasePath_AuthzCoverageGuardStillApplies(t *testing.T) {
	f := newBasePathApp(t, testBasePath)

	if err := authz.VerifyRouteCoverage(f.App); err != nil {
		t.Fatalf("VerifyRouteCoverage: %v", err)
	}

	var apiRoutes int
	for _, route := range f.App.GetRoutes() {
		if strings.HasPrefix(route.Path, "/api/v1") {
			apiRoutes++
		}
	}
	if apiRoutes == 0 {
		t.Fatal("no /api/v1 routes registered — the coverage guard would pass vacuously")
	}
}

func TestRoutes_BasePath_OIDCRedirectsCarryThePrefix(t *testing.T) {
	iss := newTestIssuer(t)
	f := newTestApp(t, func(c *config.Config) {
		c.Server.RootURL = "https://host.ts.net"
		c.Server.BasePath = testBasePath
		c.Auth.OIDC = config.OIDCConfig{
			Enabled:           true,
			ClientID:          iss.ClientID,
			ClientSecret:      "secret",
			IssuerURL:         iss.Server.URL,
			Scopes:            []string{"openid", "profile", "email"},
			UsernameAttribute: "preferred_username",
			EmailAttribute:    "email",
			NameAttribute:     "name",
			RoleAttributePath: "resource_access.test-client.roles",
			CookieName:        "session",
			CookieHTTPOnly:    true,
			CookieSameSite:    "Lax",
			SessionMaxAge:     3600,
		}
	})

	// The IdP is sent the public callback URL, which includes the prefix.
	const wantRedirectURI = "redirect_uri=https%3A%2F%2Fhost.ts.net%2Fgarage-ui%2Fauth%2Foidc%2Fcallback"
	for _, loginPath := range []string{testBasePath + "/auth/oidc/login", "/auth/oidc/login"} {
		resp, err := f.App.Test(httptest.NewRequest(http.MethodGet, loginPath, nil))
		if err != nil {
			t.Fatalf("app.Test(%s): %v", loginPath, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			t.Fatalf("%s: OIDC login route not reachable", loginPath)
		}
		if loc := resp.Header.Get("Location"); !strings.Contains(loc, wantRedirectURI) {
			t.Errorf("%s: authorize redirect %q does not carry %q", loginPath, loc, wantRedirectURI)
		}
	}

	// The callback returns the browser to the SPA login route. That URL goes
	// through the proxy, so it needs the prefix even though the callback
	// itself arrived stripped.
	for _, callbackPath := range []string{testBasePath + "/auth/oidc/callback", "/auth/oidc/callback"} {
		state := oidcState(t, f)
		resp, err := f.App.Test(httptest.NewRequest(http.MethodGet, callbackPath+"?state="+state+"&code=c", nil))
		if err != nil {
			t.Fatalf("app.Test(%s): %v", callbackPath, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("%s: status = %d, want 303", callbackPath, resp.StatusCode)
		}
		if got, want := resp.Header.Get("Location"), testBasePath+"/login?login=success"; got != want {
			t.Errorf("%s: Location = %q, want %q", callbackPath, got, want)
		}
	}
}

// --- SPA fallback -----------------------------------------------------------

// writeFrontend creates ./frontend/dist in a temp cwd and returns the dir.
func writeFrontend(t *testing.T, indexHTML string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SPA fallback tests skipped on Windows due to file-handle cleanup race")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	dist := filepath.Join(dir, "frontend", "dist", "assets")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend", "dist", "index.html"), []byte(indexHTML), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dist, "app.js"), []byte("console.log('app')"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	return dir
}

const testIndexHTML = `<!doctype html><html><head><base href="/"><meta name="garage-ui-base-path" content=""><title>spa</title></head><body><div id="root"></div><script type="module" src="./assets/app.js"></script></body></html>`

func TestRoutes_BasePath_SPAShellCarriesInjectedPrefix(t *testing.T) {
	writeFrontend(t, testIndexHTML)
	f := newBasePathApp(t, testBasePath)

	// Both the prefixed spelling and the stripped one — a browser behind
	// tailscale serve produces the latter at the backend.
	for _, path := range []string{
		testBasePath,
		testBasePath + "/",
		testBasePath + "/buckets",
		testBasePath + "/buckets/my-bucket/objects/deep/key.txt",
		"/",
		"/buckets",
		"/buckets/my-bucket/objects/deep/key.txt",
	} {
		status, body := getBody(t, f, path)
		if status != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200 (SPA shell)", path, status)
		}
		if !strings.Contains(body, `<base href="/garage-ui/">`) {
			t.Errorf("GET %s: index.html not rewritten with the base href:\n%s", path, body)
		}
		if !strings.Contains(body, `<meta name="garage-ui-base-path" content="/garage-ui">`) {
			t.Errorf("GET %s: base path not exposed to the SPA:\n%s", path, body)
		}
	}
}

func TestRoutes_BasePath_ServesAssetsEitherWay(t *testing.T) {
	writeFrontend(t, testIndexHTML)
	f := newBasePathApp(t, testBasePath)

	for _, path := range []string{testBasePath + "/assets/app.js", "/assets/app.js"} {
		resp, err := f.App.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatalf("app.Test(%s): %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			t.Fatalf("GET %s = %d, want 200", path, resp.StatusCode)
		}
		if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
			t.Errorf("GET %s: Cache-Control = %q, want the immutable asset policy", path, cc)
		}
		_ = resp.Body.Close()
	}
}

// The skip-list is evaluated after stripping, so unknown API paths must not
// answer 200 with the SPA shell — the client's error handling depends on it.
func TestRoutes_BasePath_UnknownAPIPathsDoNotFallThroughToSPA(t *testing.T) {
	writeFrontend(t, testIndexHTML)
	f := newBasePathApp(t, testBasePath)

	for _, path := range []string{
		"/api/v1/definitely-not-a-route",
		"/auth/nope",
		"/docs-missing-file",
	} {
		for _, full := range []string{testBasePath + path, path} {
			_, body := getBody(t, f, full)
			if strings.Contains(body, `<div id="root">`) {
				t.Errorf("GET %s served the SPA shell instead of an error", full)
			}
		}
	}
}

// Mirrors TestRoutes_MetricsPublic_Disabled_WithSPA_Returns404 for a subpath
// deployment: a scrape must fail loudly with 404 instead of silently receiving
// the SPA shell with a 200.
func TestRoutes_BasePath_MetricsDisabled_DoesNotServeSPA(t *testing.T) {
	writeFrontend(t, testIndexHTML)
	f := newTestApp(t, func(c *config.Config) {
		c.Server.BasePath = testBasePath
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "u"
		c.Auth.Admin.Password = "p"
		// MetricsPublic defaults to false → no /metrics route registered.
	})

	for _, path := range []string{testBasePath + "/metrics", "/metrics"} {
		expectStatus(t, f.App, httptest.NewRequest(http.MethodGet, path, nil), http.StatusNotFound)
	}
}

// Serving from the root must keep the historical behaviour, including an
// index.html that is not rewritten into a subpath.
func TestRoutes_NoBasePath_SPAIndexKeepsRootBase(t *testing.T) {
	writeFrontend(t, testIndexHTML)
	f := newTestApp(t, nil)

	status, body := getBody(t, f, "/deep/spa/route")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !strings.Contains(body, `<base href="/">`) {
		t.Errorf("root deployment should keep <base href=\"/\">, got: %s", body)
	}
	if !strings.Contains(body, `<meta name="garage-ui-base-path" content="">`) {
		t.Errorf("root deployment should expose an empty base path, got: %s", body)
	}
}

// --- InjectBasePath ---------------------------------------------------------

func TestInjectBasePath(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		basePath string
		want     []string
		notWant  []string
	}{
		{
			name:     "replaces existing tags",
			html:     testIndexHTML,
			basePath: "/garage-ui",
			want: []string{
				`<base href="/garage-ui/">`,
				`<meta name="garage-ui-base-path" content="/garage-ui">`,
			},
			notWant: []string{`<base href="/">`},
		},
		{
			name:     "root path restores the defaults",
			html:     `<!doctype html><html><head><base href="/garage-ui/"><meta name="garage-ui-base-path" content="/garage-ui"></head><body></body></html>`,
			basePath: "",
			want: []string{
				`<base href="/">`,
				`<meta name="garage-ui-base-path" content="">`,
			},
		},
		{
			name:     "inserts missing tags after head",
			html:     `<!doctype html><html><head><title>t</title></head><body></body></html>`,
			basePath: "/garage-ui",
			want: []string{
				`<base href="/garage-ui/">`,
				`<meta name="garage-ui-base-path" content="/garage-ui">`,
				`<title>t</title>`,
			},
		},
		{
			name:     "handles attributes and mixed case",
			html:     `<HTML><HEAD lang="en"><BASE HREF="/" data-x="1"><META NAME="garage-ui-base-path" CONTENT=""></HEAD></HTML>`,
			basePath: "/garage-ui",
			want: []string{
				`<base href="/garage-ui/">`,
				`<meta name="garage-ui-base-path" content="/garage-ui">`,
			},
			notWant: []string{`HREF="/"`},
		},
		{
			name:     "nested prefix",
			html:     testIndexHTML,
			basePath: "/admin/garage-ui",
			want:     []string{`<base href="/admin/garage-ui/">`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(InjectBasePath([]byte(tc.html), tc.basePath))
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("output does not contain %q:\n%s", want, got)
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(got, notWant) {
					t.Errorf("output still contains %q:\n%s", notWant, got)
				}
			}
		})
	}
}

// Injection runs on every request, so applying it to already-rewritten HTML
// must not stack tags.
func TestInjectBasePath_Idempotent(t *testing.T) {
	once := InjectBasePath([]byte(testIndexHTML), testBasePath)
	twice := InjectBasePath(once, testBasePath)

	if string(once) != string(twice) {
		t.Errorf("not idempotent:\nonce:  %s\ntwice: %s", once, twice)
	}
	if n := strings.Count(string(twice), "<base "); n != 1 {
		t.Errorf("found %d <base> tags, want 1", n)
	}
	if n := strings.Count(string(twice), "garage-ui-base-path"); n != 1 {
		t.Errorf("found %d base-path meta tags, want 1", n)
	}
}

func TestInjectBasePath_NoHeadElement(t *testing.T) {
	got := string(InjectBasePath([]byte(`<div id="root"></div>`), testBasePath))
	if !strings.Contains(got, `<base href="/garage-ui/">`) {
		t.Errorf("base tag missing for head-less document: %s", got)
	}
	if !strings.Contains(got, `<div id="root">`) {
		t.Errorf("original markup lost: %s", got)
	}
}
