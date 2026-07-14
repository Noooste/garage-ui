package routes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/authz"
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/handlers"
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"
	"Noooste/garage-ui/internal/services/mocks"

	"github.com/gofiber/fiber/v3"
)

// newTestAppWithAuthz builds a fully-wired fiber.App via SetupRoutes, reusing
// newTestApp's fixture (disabled-policy authz middleware, admin auth enabled
// so every /api/v1 route is reachable), and returns the *fiber.App directly
// for route-table inspection.
func newTestAppWithAuthz(t *testing.T) *fiber.App {
	t.Helper()
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
	})
	return f.App
}

// TestEveryAPIRouteDeclaresPermission is the CI-level fail-closed guarantee:
// a new /api/v1 route without an authz.Require declaration fails this test
// (and would also refuse to boot via the same check in main).
func TestEveryAPIRouteDeclaresPermission(t *testing.T) {
	app := newTestAppWithAuthz(t)
	if err := authz.VerifyRouteCoverage(app); err != nil {
		t.Fatalf("route coverage: %v", err)
	}
}

// newEnabledPolicyFixture builds a SetupRoutes app with an ENABLED
// access-control policy (one team, claim "g-t", bucket prefix "allowed-",
// permissions bucket.list + object.list + object.read) and returns the
// fixture plus a Bearer session token for a member of that team. Admin auth
// is enabled so AuthMiddleware accepts the Bearer JWT; the resolver trusts
// the signed AuthMethod claim ("oidc"), so the user resolves through the
// team policy, not as the synthetic admin.
func newEnabledPolicyFixture(t *testing.T) (*routeFixture, string) {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			Environment: "test",
		},
		Auth: config.AuthConfig{
			Admin: config.AdminAuthConfig{
				Enabled:  true,
				Username: "admin",
				Password: "pw",
			},
		},
		CORS: config.CORSConfig{},
		AccessControl: &config.AccessControlConfig{
			Teams: []config.TeamConfig{{
				Name:        "team-t",
				ClaimValues: []string{"g-t"},
				Bindings: []config.BindingConfig{{
					BucketPrefixes: []string{"allowed-"},
					Permissions:    []string{"bucket.list", "object.list", "object.read"},
				}},
			}},
		},
	}

	svc, err := auth.NewAuthService(&cfg.Auth, &cfg.Server)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}

	policy, err := authz.CompilePolicy(cfg.AccessControl)
	if err != nil {
		t.Fatalf("CompilePolicy: %v", err)
	}
	az := authz.NewMiddleware(policy, authz.NewTeamResolver(policy, nil), authz.NewAuthorizer())

	admin := &mocks.AdminMock{}
	s3 := &mocks.S3Mock{}

	app := fiber.New()
	SetupRoutes(
		app,
		cfg,
		svc,
		handlers.NewHealthHandler("test"),
		handlers.NewBucketHandler(admin, s3),
		handlers.NewObjectHandler(s3, svc),
		handlers.NewUserHandler(admin),
		handlers.NewClusterHandler(admin),
		handlers.NewMonitoringHandler(admin, s3),
		handlers.NewCapabilitiesHandler("v2", services.CapabilitiesV2(), false),
		az,
	)

	token, err := svc.GenerateSessionToken(&auth.UserInfo{
		Username:   "team-user",
		Email:      "team-user@example.com",
		Teams:      []string{"g-t"},
		AuthMethod: "oidc",
	})
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	return &routeFixture{App: app, Admin: admin, S3: s3, Auth: svc, Cfg: cfg}, token
}

// TestWildcardObjectRoutes_EnforceAuthzViaGroupCascade locks in the Fiber
// behavior the wildcard wiring relies on: the api group's .Use() middlewares
// (AuthMiddleware, ResolveSubject) cascade by path prefix onto the wildcard
// object routes registered directly on app, so those routes do NOT repeat
// ResolveSubject themselves. If the cascade ever stopped covering them, the
// allowed-bucket request below would 403 with reason no_subject instead of
// reaching the handler.
func TestWildcardObjectRoutes_EnforceAuthzViaGroupCascade(t *testing.T) {
	f, token := newEnabledPolicyFixture(t)

	f.S3.GetObjectFn = func(_ context.Context, _, key string) (io.ReadCloser, *models.ObjectInfo, error) {
		return io.NopCloser(strings.NewReader("hello")), &models.ObjectInfo{Key: key, Size: 5, ContentType: "text/plain"}, nil
	}

	do := func(method, path string) int {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := f.App.Test(req)
		if err != nil {
			t.Fatalf("app.Test(%s %s): %v", method, path, err)
		}
		return resp.StatusCode
	}

	// In-scope bucket + held permission (object.read) → the request passes
	// Require and reaches the handler. This is the discriminating assertion:
	// it can only succeed if the group-level ResolveSubject ran for this
	// wildcard route (no subject → Require denies everything).
	if code := do("GET", "/api/v1/buckets/allowed-data/objects/somekey"); code != 200 {
		t.Errorf("GET allowed bucket: status = %d, want 200 (cascaded ResolveSubject + Require allow)", code)
	}

	// Out-of-scope bucket → default deny.
	if code := do("GET", "/api/v1/buckets/denied-data/objects/somekey"); code != 403 {
		t.Errorf("GET denied bucket: status = %d, want 403", code)
	}

	// DELETE requires object.delete, which the team does not hold, so it is denied
	// even on an in-scope bucket.
	if code := do("DELETE", "/api/v1/buckets/allowed-data/objects/somekey"); code != 403 {
		t.Errorf("DELETE allowed bucket without object.delete: status = %d, want 403", code)
	}

	// HEAD wildcard is denied on an out-of-scope bucket too.
	if code := do("HEAD", "/api/v1/buckets/denied-data/objects/somekey"); code != 403 {
		t.Errorf("HEAD denied bucket: status = %d, want 403", code)
	}
}

// TestListBuckets_HTTPFiltersByPolicyAndAddsEffectivePermissions is the
// HTTP-level companion to handlers.TestListBuckets_MapsAliasesAndStats: it
// drives GET /api/v1/buckets through the full authz-wired route (not just the
// handler in isolation) for a team-scoped session, with the mocked admin
// service returning buckets both inside and outside the team's "allowed-"
// prefix. Only in-scope buckets should come back, each carrying the caller's
// effective_permissions.
func TestListBuckets_HTTPFiltersByPolicyAndAddsEffectivePermissions(t *testing.T) {
	f, token := newEnabledPolicyFixture(t)

	f.Admin.ListBucketsFn = func(_ context.Context) ([]models.ListBucketsResponseItem, error) {
		return []models.ListBucketsResponseItem{
			{ID: "id-a", Created: time.Unix(0, 0), GlobalAliases: []string{"allowed-a"}},
			{ID: "id-b", Created: time.Unix(0, 0), GlobalAliases: []string{"allowed-b"}},
			{ID: "id-x", Created: time.Unix(0, 0), GlobalAliases: []string{"denied-x"}},
		}, nil
	}
	f.Admin.GetBucketInfoByAliasFn = func(_ context.Context, alias string) (*models.GarageBucketInfo, error) {
		return &models.GarageBucketInfo{ID: alias, Objects: 0, Bytes: 0}, nil
	}

	req := httptest.NewRequest("GET", "/api/v1/buckets", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Data models.BucketListResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Count != 2 {
		t.Fatalf("count = %d, want 2 (denied-x filtered out): %+v", body.Data.Count, body.Data.Buckets)
	}

	seen := map[string]bool{}
	for _, b := range body.Data.Buckets {
		seen[b.Name] = true
		if !strings.HasPrefix(b.Name, "allowed-") {
			t.Errorf("bucket %q returned, want only allowed-* buckets", b.Name)
		}
		if len(b.EffectivePermissions) == 0 {
			t.Errorf("bucket %q: effective_permissions missing", b.Name)
		}
	}
	if !seen["allowed-a"] || !seen["allowed-b"] {
		t.Errorf("missing expected buckets, got: %+v", body.Data.Buckets)
	}
	if seen["denied-x"] {
		t.Error("denied-x should not be visible to a team without bucket.list on that prefix")
	}
}

// TestPreviewTokenGrantsObjectGET exercises the full production chain: group
// cascade AuthMiddleware accepts the token, ResolveSubject finds no user,
// and Require allows via the preview claims instead of a subject.
func TestPreviewTokenGrantsObjectGET(t *testing.T) {
	f, _ := newEnabledPolicyFixture(t)

	// The full-object body echoes the key the handler was actually asked to
	// serve (the decoded c.Params("*")). Asserting the streamed body equals
	// the exact key the token was minted for makes any future divergence
	// between the validated key and the served key fail loudly here rather
	// than hide behind a constant body.
	const mintedKey = "media/clip.mp4"
	f.S3.GetObjectFn = func(_ context.Context, _, key string) (io.ReadCloser, *models.ObjectInfo, error) {
		return io.NopCloser(strings.NewReader(key)), &models.ObjectInfo{Key: key, Size: int64(len(key)), ContentType: "video/mp4", LastModified: time.Now()}, nil
	}
	f.S3.GetObjectMetadataFn = func(_ context.Context, _, key string) (*models.ObjectInfo, error) {
		return &models.ObjectInfo{Key: key, Size: 5, ContentType: "video/mp4", LastModified: time.Now()}, nil
	}
	f.S3.GetObjectRangeFn = func(_ context.Context, _, _ string, start, end int64) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("ell")), nil
	}

	token, _, err := f.Auth.MintPreviewToken("allowed-data", mintedKey, time.Minute)
	if err != nil {
		t.Fatalf("MintPreviewToken: %v", err)
	}
	tokenized := "/api/v1/buckets/allowed-data/objects/media%2Fclip.mp4?pt=" + url.QueryEscape(token)

	// No Authorization header anywhere in this test.
	do := func(method, path, rangeHeader string) *http.Response {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		if rangeHeader != "" {
			req.Header.Set("Range", rangeHeader)
		}
		resp, err := f.App.Test(req)
		if err != nil {
			t.Fatalf("app.Test(%s %s): %v", method, path, err)
		}
		return resp
	}

	resp := do("GET", tokenized, "")
	if resp.StatusCode != 200 {
		t.Fatalf("tokenized GET: status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != mintedKey {
		t.Errorf("served key = %q, want %q (validated key must equal served key)", body, mintedKey)
	}

	// Seeking works through the same token.
	resp = do("GET", tokenized, "bytes=1-3")
	if resp.StatusCode != 206 {
		t.Errorf("tokenized ranged GET: status = %d, want 206", resp.StatusCode)
	}

	// The token never opens the JSON subroutes or other objects.
	if resp := do("GET", "/api/v1/buckets/allowed-data/objects/media%2Fclip.mp4%2Fmetadata?pt="+url.QueryEscape(token), ""); resp.StatusCode != 401 {
		t.Errorf("metadata with token: status = %d, want 401", resp.StatusCode)
	}
	if resp := do("GET", "/api/v1/buckets/allowed-data/objects/other.mp4?pt="+url.QueryEscape(token), ""); resp.StatusCode != 401 {
		t.Errorf("other object with token: status = %d, want 401", resp.StatusCode)
	}
	if resp := do("GET", "/api/v1/buckets/allowed-data/objects/media%2Fclip.mp4", ""); resp.StatusCode != 401 {
		t.Errorf("no token, no auth: status = %d, want 401", resp.StatusCode)
	}
}
