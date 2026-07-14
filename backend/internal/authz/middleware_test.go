package authz

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/config"

	"github.com/gofiber/fiber/v3"
)

func middlewareFixture(t *testing.T) *Middleware {
	t.Helper()
	policy, err := CompilePolicy(&config.AccessControlConfig{
		Teams: []config.TeamConfig{{
			Name:        "backend",
			ClaimValues: []string{"g-backend"},
			Bindings: []config.BindingConfig{{
				BucketPrefixes: []string{"backend-"},
				Permissions:    []string{"bucket.read", "bucket.create", "object.read"},
			}},
			ClusterPermissions: []string{"cluster.status"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewMiddleware(policy, NewTeamResolver(policy, []string{"garage-admin"}), NewAuthorizer())
}

func newTestApp(m *Middleware, userInfo *auth.UserInfo) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error { // stand-in for AuthMiddleware
		if userInfo != nil {
			c.Locals("userInfo", userInfo)
		}
		return c.Next()
	})
	app.Use(m.ResolveSubject())
	app.Get("/api/v1/buckets/:name", m.Require(BucketFromParam("name"), PermBucketRead), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	app.Post("/api/v1/buckets", m.Require(BucketFromBody(), PermBucketCreate), func(c fiber.Ctx) error {
		return c.SendString("created")
	})
	app.Get("/api/v1/cluster/status", m.Require(ScopeNone, PermClusterStatus), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	return app
}

func doReq(t *testing.T, app *fiber.App, method, path, body string) int {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestRequireAllowsMatchingTeam(t *testing.T) {
	m := middlewareFixture(t)
	app := newTestApp(m, &auth.UserInfo{Email: "a@x", AuthMethod: "oidc", Teams: []string{"g-backend"}})
	if code := doReq(t, app, "GET", "/api/v1/buckets/backend-api", ""); code != 200 {
		t.Errorf("matching bucket: status %d, want 200", code)
	}
	if code := doReq(t, app, "GET", "/api/v1/cluster/status", ""); code != 200 {
		t.Errorf("cluster.status: status %d, want 200", code)
	}
}

func TestRequireDeniesOutOfScope(t *testing.T) {
	m := middlewareFixture(t)
	app := newTestApp(m, &auth.UserInfo{Email: "a@x", AuthMethod: "oidc", Teams: []string{"g-backend"}})
	if code := doReq(t, app, "GET", "/api/v1/buckets/data-warehouse", ""); code != 403 {
		t.Errorf("non-matching bucket: status %d, want 403", code)
	}
}

func TestRequireBucketFromBody(t *testing.T) {
	m := middlewareFixture(t)
	app := newTestApp(m, &auth.UserInfo{Email: "a@x", AuthMethod: "oidc", Teams: []string{"g-backend"}})
	if code := doReq(t, app, "POST", "/api/v1/buckets", `{"name":"backend-new"}`); code != 200 {
		t.Errorf("create with matching prefix: status %d, want 200", code)
	}
	if code := doReq(t, app, "POST", "/api/v1/buckets", `{"name":"other-new"}`); code != 403 {
		t.Errorf("create with foreign prefix: status %d, want 403", code)
	}
}

func TestRequireDefaultDenyZeroTeamUser(t *testing.T) {
	m := middlewareFixture(t)
	app := newTestApp(m, &auth.UserInfo{Email: "z@x", AuthMethod: "oidc"})
	if code := doReq(t, app, "GET", "/api/v1/buckets/backend-api", ""); code != 403 {
		t.Errorf("zero-team user: status %d, want 403", code)
	}
}

func TestRequirePassthroughWhenDisabled(t *testing.T) {
	policy, _ := CompilePolicy(nil)
	m := NewMiddleware(policy, NewTeamResolver(policy, nil), NewAuthorizer())
	// No userInfo at all. Disabled access control must not require a subject.
	app := newTestApp(m, nil)
	if code := doReq(t, app, "GET", "/api/v1/buckets/anything", ""); code != 200 {
		t.Errorf("disabled: status %d, want 200", code)
	}
}

func TestMiddlewareEnabledReflectsPolicy(t *testing.T) {
	if !middlewareFixture(t).Enabled() {
		t.Error("Enabled() = false for a configured policy, want true")
	}
	disabled, _ := CompilePolicy(nil)
	m := NewMiddleware(disabled, NewTeamResolver(disabled, nil), NewAuthorizer())
	if m.Enabled() {
		t.Error("Enabled() = true for a nil (disabled) policy, want false")
	}
}

func TestResolveSubjectWithoutUserInfoDenies(t *testing.T) {
	// Enabled middleware, but auth set no userInfo local: ResolveSubject leaves
	// no subject, and Require then denies for want of one.
	m := middlewareFixture(t)
	app := newTestApp(m, nil)
	if code := doReq(t, app, "GET", "/api/v1/buckets/backend-api", ""); code != 403 {
		t.Errorf("enabled + no userInfo: status %d, want 403", code)
	}
}

func TestRequireBucketFromBodyMalformedBody(t *testing.T) {
	m := middlewareFixture(t)
	app := newTestApp(m, &auth.UserInfo{Email: "a@x", AuthMethod: "oidc", Teams: []string{"g-backend"}})
	// Malformed JSON: BucketFromBody's bind fails and returns an empty resource.
	// An empty bucket is an unscoped check, so a team holding bucket.create in
	// any binding is allowed (this is what proves the resource came back empty:
	// a non-empty foreign bucket name would be denied instead).
	if code := doReq(t, app, "POST", "/api/v1/buckets", "{not-json"); code != 200 {
		t.Errorf("malformed body: status %d, want 200 (empty resource, any_binding)", code)
	}
}

func TestRequireUnknownPermissionPanics(t *testing.T) {
	m := middlewareFixture(t)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Require with an unknown permission: want panic, got none")
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "unknown permission") {
			t.Errorf("panic value = %v, want message containing %q", r, "unknown permission")
		}
	}()
	m.Require(ScopeNone, "bogus.permission")
}

func TestVerifyRouteCoverage(t *testing.T) {
	m := middlewareFixture(t)

	covered := fiber.New()
	covered.Get("/api/v1/capabilities", func(c fiber.Ctx) error { return nil }) // exempt
	covered.Get("/api/v1/health", func(c fiber.Ctx) error { return nil })       // exempt
	covered.Get("/api/v1/x", m.Require(ScopeNone, PermClusterStatus), func(c fiber.Ctx) error { return nil })
	covered.Get("/other", func(c fiber.Ctx) error { return nil }) // outside /api/v1
	if err := VerifyRouteCoverage(covered); err != nil {
		t.Errorf("covered app: %v, want nil", err)
	}

	uncovered := fiber.New()
	uncovered.Get("/api/v1/naked", func(c fiber.Ctx) error { return nil })
	err := VerifyRouteCoverage(uncovered)
	if err == nil {
		t.Fatal("uncovered app: want error, got nil")
	}
	if !strings.Contains(err.Error(), "/api/v1/naked") {
		t.Errorf("error should name the naked route: %v", err)
	}
}

func TestVerifyRouteCoverage_GroupUseBookkeepingExempt(t *testing.T) {
	// Group-level .Use() middleware produces synthetic per-method entries at
	// exactly the bare group prefix; those aren't endpoints and must not trip
	// the coverage check as long as the real routes carry Require.
	m := middlewareFixture(t)
	app := fiber.New()
	api := app.Group("/api/v1")
	api.Use(func(c fiber.Ctx) error { return c.Next() }) // stand-in for AuthMiddleware
	api.Use(m.ResolveSubject())
	api.Get("/x", m.Require(ScopeNone, PermClusterStatus), func(c fiber.Ctx) error { return nil })
	if err := VerifyRouteCoverage(app); err != nil {
		t.Errorf("group-level Use at bare prefix should be exempt: %v", err)
	}
}

func TestRequireZeroPermissionsPanics(t *testing.T) {
	m := middlewareFixture(t)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Require with zero perms: want panic, got none")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "at least one permission required") {
			t.Errorf("panic value = %v, want message containing %q", r, "at least one permission required")
		}
	}()
	m.Require(ScopeNone)
}

func TestVerifyRouteCoverage_UseRegisteredEndpointFlagged(t *testing.T) {
	// A .Use()-registered route at a DEEPER path under /api/v1 is a reachable
	// endpoint (Fiber runs it for every method with that prefix); it must get
	// the same fail-closed treatment as a normal route.
	app := fiber.New()
	api := app.Group("/api/v1")
	api.Use(func(c fiber.Ctx) error { return c.Next() }) // bare-prefix middleware stays exempt
	api.Use("/sneaky", func(c fiber.Ctx) error { return c.SendString("terminal") })
	err := VerifyRouteCoverage(app)
	if err == nil {
		t.Fatal("use-registered endpoint under /api/v1: want error, got nil")
	}
	if !strings.Contains(err.Error(), "/api/v1/sneaky") {
		t.Errorf("error should name /api/v1/sneaky: %v", err)
	}
}
