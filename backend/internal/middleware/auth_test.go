package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/config"
	logpkg "Noooste/garage-ui/pkg/logger"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
)

// newAuthTestApp builds a fiber.App with RequestID + Logging (buffer-backed) +
// AuthMiddleware + a trivial /protected handler that echoes username/auth
// method into the JSON body so tests can assert on locals.
func newAuthTestApp(t *testing.T, buf *bytes.Buffer, authCfg *config.AuthConfig, svc *auth.Service) *fiber.App {
	t.Helper()
	base := zerolog.New(buf)
	app := fiber.New()
	app.Use(RequestID())
	app.Use(Logging(base))
	app.Use(AuthMiddleware(authCfg, svc))
	app.Get("/protected", func(c fiber.Ctx) error {
		uname, _ := c.Locals("username").(string)
		email, _ := c.Locals("email").(string)
		logpkg.FromCtx(c.Context()).Info().Msg("in_handler")
		return c.JSON(fiber.Map{
			"ok":       true,
			"username": uname,
			"email":    email,
		})
	})
	return app
}

// newAuthSvc returns an *auth.Service with the given auth config, JWT
// service initialized, OIDC disabled unless the caller wires it.
func newAuthSvc(t *testing.T, authCfg *config.AuthConfig) *auth.Service {
	t.Helper()
	svc, err := auth.NewAuthService(authCfg, &config.ServerConfig{})
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	return svc
}

// findLine returns the first parsed log line whose "message" field equals msg,
// failing the test if no such line is present.
func findLine(t *testing.T, buf *bytes.Buffer, msg string) map[string]any {
	t.Helper()
	for _, line := range parseLines(t, buf) {
		if line["message"] == msg {
			return line
		}
	}
	t.Fatalf("no %q log line: %s", msg, buf.String())
	return nil
}

func TestAuthMiddleware_BothDisabled_AllowsRequest(t *testing.T) {
	authCfg := &config.AuthConfig{
		Admin: config.AdminAuthConfig{Enabled: false},
		OIDC:  config.OIDCConfig{Enabled: false},
	}
	svc := newAuthSvc(t, authCfg)

	var buf bytes.Buffer
	app := newAuthTestApp(t, &buf, authCfg, svc)

	req := httptest.NewRequest("GET", "/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("ok = %v, want true", body["ok"])
	}
	if body["username"] != "" {
		t.Errorf("username should be empty when auth disabled, got %q", body["username"])
	}
}

func newAdminCfg() *config.AuthConfig {
	return &config.AuthConfig{
		Admin: config.AdminAuthConfig{Enabled: true, Username: "admin", Password: "pw"},
		OIDC:  config.OIDCConfig{Enabled: false},
	}
}

func TestAuthMiddleware_Admin_BearerValid_AllowsAndEnrichesLogger(t *testing.T) {
	authCfg := newAdminCfg()
	svc := newAuthSvc(t, authCfg)
	tok, err := svc.GenerateSessionToken(&auth.UserInfo{Username: "admin", Email: "a@b"})
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	var buf bytes.Buffer
	app := newAuthTestApp(t, &buf, authCfg, svc)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["username"] != "admin" {
		t.Errorf("username = %v, want admin", body["username"])
	}
	if body["email"] != "a@b" {
		t.Errorf("email = %v, want a@b", body["email"])
	}

	// Enriched handler log line should carry user_id and auth_method=admin.
	access := findLine(t, &buf, "in_handler")
	if access["user_id"] != "admin" {
		t.Errorf("user_id = %v, want admin", access["user_id"])
	}
	if access["auth_method"] != "admin" {
		t.Errorf("auth_method = %v, want admin", access["auth_method"])
	}
}

func TestAuthMiddleware_Admin_BearerInvalid_Returns401(t *testing.T) {
	authCfg := newAdminCfg()
	svc := newAuthSvc(t, authCfg)

	var buf bytes.Buffer
	app := newAuthTestApp(t, &buf, authCfg, svc)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	var env struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if env.Success {
		t.Error("success should be false")
	}
	if env.Error.Code != "UNAUTHORIZED" {
		t.Errorf("error.code = %q, want UNAUTHORIZED", env.Error.Code)
	}

	// Warn log should carry reason=no_valid_credentials without leaking the token.
	warn := findLine(t, &buf, "authentication_failed")
	if warn["reason"] != "no_valid_credentials" {
		t.Errorf("reason = %v", warn["reason"])
	}
	if warn["level"] != "warn" {
		t.Errorf("level = %v, want warn", warn["level"])
	}
	if strings.Contains(buf.String(), "not-a-real-token") {
		t.Error("token value must not appear in logs")
	}
}

func TestAuthMiddleware_Admin_NoAuthHeader_Returns401(t *testing.T) {
	authCfg := newAdminCfg()
	svc := newAuthSvc(t, authCfg)

	var buf bytes.Buffer
	app := newAuthTestApp(t, &buf, authCfg, svc)

	req := httptest.NewRequest("GET", "/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthMiddleware_Admin_NonBearerScheme_Returns401(t *testing.T) {
	authCfg := newAdminCfg()
	svc := newAuthSvc(t, authCfg)

	var buf bytes.Buffer
	app := newAuthTestApp(t, &buf, authCfg, svc)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwdw==")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func newOIDCCfg(cookieName string) *config.AuthConfig {
	return &config.AuthConfig{
		Admin: config.AdminAuthConfig{Enabled: false},
		OIDC: config.OIDCConfig{
			Enabled:    true,
			CookieName: cookieName,
		},
	}
}

// newOIDCSvc returns a Service whose OIDC is *not* initialized (no dialing
// needed), which is fine because AuthMiddleware's OIDC branch only calls
// ValidateSessionToken — a pure JWT operation.
func newOIDCSvc(t *testing.T) *auth.Service {
	t.Helper()
	return newAuthSvc(t, &config.AuthConfig{OIDC: config.OIDCConfig{Enabled: false}})
}

func TestAuthMiddleware_OIDC_ValidCookie_Allows(t *testing.T) {
	cfg := newOIDCCfg("session")
	svc := newOIDCSvc(t)

	tok, err := svc.GenerateSessionToken(&auth.UserInfo{Username: "alice", Email: "a@x"})
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	var buf bytes.Buffer
	app := newAuthTestApp(t, &buf, cfg, svc)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: tok})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	access := findLine(t, &buf, "in_handler")
	if access["auth_method"] != "oidc" {
		t.Errorf("auth_method = %v, want oidc", access["auth_method"])
	}
	if access["user_id"] != "alice" {
		t.Errorf("user_id = %v, want alice", access["user_id"])
	}
}

func TestAuthMiddleware_OIDC_InvalidCookie_Returns401(t *testing.T) {
	cfg := newOIDCCfg("session")
	svc := newOIDCSvc(t)

	var buf bytes.Buffer
	app := newAuthTestApp(t, &buf, cfg, svc)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "garbage"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthMiddleware_OIDC_NoCookie_Returns401(t *testing.T) {
	cfg := newOIDCCfg("session")
	svc := newOIDCSvc(t)

	var buf bytes.Buffer
	app := newAuthTestApp(t, &buf, cfg, svc)

	req := httptest.NewRequest("GET", "/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func newBothCfg(cookieName string) *config.AuthConfig {
	return &config.AuthConfig{
		Admin: config.AdminAuthConfig{Enabled: true, Username: "admin", Password: "pw"},
		OIDC: config.OIDCConfig{
			Enabled:    true,
			CookieName: cookieName,
		},
	}
}

func TestAuthMiddleware_Both_BearerValid_AdminPathWins(t *testing.T) {
	cfg := newBothCfg("session")
	// OIDC disabled on the Service is fine — see Task 3 rationale.
	svc := newAuthSvc(t, &config.AuthConfig{Admin: cfg.Admin})

	tok, err := svc.GenerateSessionToken(&auth.UserInfo{Username: "admin"})
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	var buf bytes.Buffer
	app := newAuthTestApp(t, &buf, cfg, svc)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	// Also set a valid OIDC cookie; admin should still win.
	cookieTok, _ := svc.GenerateSessionToken(&auth.UserInfo{Username: "alice"})
	req.AddCookie(&http.Cookie{Name: "session", Value: cookieTok})

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	access := findLine(t, &buf, "in_handler")
	if access["auth_method"] != "admin" {
		t.Errorf("auth_method = %v, want admin", access["auth_method"])
	}
	if access["user_id"] != "admin" {
		t.Errorf("user_id = %v, want admin", access["user_id"])
	}
}

func TestAuthMiddleware_Both_BearerInvalid_FallsThroughToOIDCCookie(t *testing.T) {
	cfg := newBothCfg("session")
	svc := newAuthSvc(t, &config.AuthConfig{Admin: cfg.Admin})

	cookieTok, err := svc.GenerateSessionToken(&auth.UserInfo{Username: "alice"})
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	var buf bytes.Buffer
	app := newAuthTestApp(t, &buf, cfg, svc)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer bogus")
	req.AddCookie(&http.Cookie{Name: "session", Value: cookieTok})

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200 (OIDC fallback)", resp.StatusCode)
	}

	access := findLine(t, &buf, "in_handler")
	if access["auth_method"] != "oidc" {
		t.Errorf("auth_method = %v, want oidc", access["auth_method"])
	}
}

func TestAuthMiddleware_Both_AllInvalid_Returns401WithCombinedMethodLabel(t *testing.T) {
	cfg := newBothCfg("session")
	svc := newAuthSvc(t, &config.AuthConfig{Admin: cfg.Admin})

	var buf bytes.Buffer
	app := newAuthTestApp(t, &buf, cfg, svc)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer bogus")
	req.AddCookie(&http.Cookie{Name: "session", Value: "also-bogus"})

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}

	warn := findLine(t, &buf, "authentication_failed")
	if warn["auth_method"] != "admin+oidc" {
		t.Errorf("auth_method = %v, want admin+oidc", warn["auth_method"])
	}
}

// newPreviewTokenApp mirrors the production object GET route shape. Note it
// registers AuthMiddleware via a bare app.Use(), same as routes.go's /api/v1
// group cascade: bucket and key are read from the raw request path
// (previewRouteParts), not from c.Params("bucket")/c.Params("*"), precisely
// because those Fiber route params are not yet bound when a Use()-registered
// middleware executes ahead of the specific :bucket/* route match. routes.go
// registers AuthMiddleware a second time directly on the object route too;
// this test only needs one registration to exercise the same path-parsing
// code the real group cascade hits first.
func newPreviewTokenApp(t *testing.T, authCfg *config.AuthConfig, svc *auth.Service) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Use(AuthMiddleware(authCfg, svc))
	handler := func(c fiber.Ctx) error {
		claims, _ := c.Locals(auth.PreviewTokenLocalsKey).(*auth.PreviewClaims)
		if claims == nil {
			return c.SendString("no-claims")
		}
		return c.SendString("claims:" + claims.Bucket + "/" + claims.Key)
	}
	app.Get("/api/v1/buckets/:bucket/objects/*", handler)
	app.Delete("/api/v1/buckets/:bucket/objects/*", handler)
	return app
}

func previewAuthConfig() *config.AuthConfig {
	return &config.AuthConfig{
		Admin: config.AdminAuthConfig{Enabled: true, Username: "admin", Password: "pw"},
	}
}

func TestAuthMiddleware_ValidPreviewTokenAllowsObjectGET(t *testing.T) {
	authCfg := previewAuthConfig()
	svc := newAuthSvc(t, authCfg)
	app := newPreviewTokenApp(t, authCfg, svc)

	token, _, err := svc.MintPreviewToken("b1", "dir/clip.mp4", time.Minute)
	if err != nil {
		t.Fatalf("MintPreviewToken: %v", err)
	}
	req := httptest.NewRequest("GET", "/api/v1/buckets/b1/objects/dir%2Fclip.mp4?pt="+url.QueryEscape(token), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "claims:b1/dir/clip.mp4" {
		t.Errorf("body = %q, want the preview claims set in locals", body)
	}
}

func TestAuthMiddleware_PreviewTokenRejections(t *testing.T) {
	authCfg := previewAuthConfig()
	svc := newAuthSvc(t, authCfg)
	app := newPreviewTokenApp(t, authCfg, svc)

	good, _, err := svc.MintPreviewToken("b1", "k.mp4", time.Minute)
	if err != nil {
		t.Fatalf("MintPreviewToken: %v", err)
	}
	expired, _, err := svc.MintPreviewToken("b1", "k.mp4", -time.Minute)
	if err != nil {
		t.Fatalf("MintPreviewToken: %v", err)
	}
	// A token whose claimed key genuinely ends in a slash. On the wire the SPA
	// sends this as one percent-encoded segment ("dir%2F"); this case instead
	// sends the ambiguous raw form ("dir/"). c.Path() keeps the trailing
	// slash, but the served c.Params("*") would be trimmed to "dir", so the
	// token would name "dir/" while the handler serves "dir": a different
	// object. previewObjectKey refuses the raw-trailing-slash form, so this
	// falls through to normal auth and 401s.
	trailingDir, _, err := svc.MintPreviewToken("b1", "dir/", time.Minute)
	if err != nil {
		t.Fatalf("MintPreviewToken: %v", err)
	}
	// A token for a key ending in "/metadata/". Decoded it is "x/metadata/",
	// whose HasSuffix "/metadata" is false because of the trailing slash, so
	// the subroute guard would not fire; but the served c.Params("*") is
	// trimmed to "x/metadata" and routes to the /metadata subroute. The
	// raw-trailing-slash refusal blocks this before either divergence matters.
	trailingMeta, _, err := svc.MintPreviewToken("b1", "x/metadata/", time.Minute)
	if err != nil {
		t.Fatalf("MintPreviewToken: %v", err)
	}

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "wrong key", method: "GET", path: "/api/v1/buckets/b1/objects/other.mp4?pt=" + url.QueryEscape(good)},
		{name: "wrong bucket", method: "GET", path: "/api/v1/buckets/b2/objects/k.mp4?pt=" + url.QueryEscape(good)},
		{name: "expired", method: "GET", path: "/api/v1/buckets/b1/objects/k.mp4?pt=" + url.QueryEscape(expired)},
		{name: "metadata subroute", method: "GET", path: "/api/v1/buckets/b1/objects/k.mp4%2Fmetadata?pt=" + url.QueryEscape(good)},
		{name: "presign subroute", method: "GET", path: "/api/v1/buckets/b1/objects/k.mp4%2Fpresign?pt=" + url.QueryEscape(good)},
		{name: "preview-url subroute", method: "GET", path: "/api/v1/buckets/b1/objects/k.mp4%2Fpreview-url?pt=" + url.QueryEscape(good)},
		{name: "delete method", method: "DELETE", path: "/api/v1/buckets/b1/objects/k.mp4?pt=" + url.QueryEscape(good)},
		{name: "garbage token", method: "GET", path: "/api/v1/buckets/b1/objects/k.mp4?pt=garbage"},
		{name: "raw trailing slash key", method: "GET", path: "/api/v1/buckets/b1/objects/dir/?pt=" + url.QueryEscape(trailingDir)},
		{name: "raw trailing slash reaching metadata subroute", method: "GET", path: "/api/v1/buckets/b1/objects/x/metadata/?pt=" + url.QueryEscape(trailingMeta)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(tc.method, tc.path, nil))
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			if resp.StatusCode != 401 {
				t.Errorf("status = %d, want 401 fallthrough to normal auth", resp.StatusCode)
			}
		})
	}
}

// TestPreviewRouteParts exercises previewRouteParts directly against every
// branch of its path-shape parsing: no "/api/v1/buckets/" prefix, a bucket
// segment with nothing after it, a bucket segment followed by something
// other than "objects/", and the well formed shape. This parsing runs in
// place of Fiber's :bucket/* param binding (see the comment on
// previewRouteParts), so its edge cases need direct coverage independent of
// AuthMiddleware's own tests.
func TestPreviewRouteParts(t *testing.T) {
	app := fiber.New()
	var gotBucket, gotKey string
	app.Get("/*", func(c fiber.Ctx) error {
		gotBucket, gotKey = previewRouteParts(c)
		return c.SendString("ok")
	})

	cases := []struct {
		name       string
		path       string
		wantBucket string
		wantKey    string
	}{
		{name: "no buckets prefix", path: "/other/path", wantBucket: "", wantKey: ""},
		{name: "bucket segment with no trailing slash", path: "/api/v1/buckets/mybucket", wantBucket: "", wantKey: ""},
		{name: "segment after bucket is not objects", path: "/api/v1/buckets/mybucket/permissions", wantBucket: "", wantKey: ""},
		{name: "well formed", path: "/api/v1/buckets/mybucket/objects/dir%2Fclip.mp4", wantBucket: "mybucket", wantKey: "dir%2Fclip.mp4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotBucket, gotKey = "unset", "unset"
			resp, err := app.Test(httptest.NewRequest("GET", tc.path, nil))
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			if resp.StatusCode != 200 {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			if gotBucket != tc.wantBucket || gotKey != tc.wantKey {
				t.Errorf("previewRouteParts(%q) = (%q, %q), want (%q, %q)", tc.path, gotBucket, gotKey, tc.wantBucket, tc.wantKey)
			}
		})
	}
}

// TestPreviewObjectKey covers previewObjectKey's own branches beyond what
// the AuthMiddleware rejection tests exercise incidentally: a plain key with
// no reserved suffix decodes normally, and a path that previewRouteParts
// can't parse at all yields "".
func TestPreviewObjectKey(t *testing.T) {
	app := fiber.New()
	var got string
	app.Get("/*", func(c fiber.Ctx) error {
		got = previewObjectKey(c)
		return c.SendString("ok")
	})

	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "plain key decodes", path: "/api/v1/buckets/b/objects/dir%2Fclip.mp4", want: "dir/clip.mp4"},
		{name: "unparseable route returns empty", path: "/not-a-bucket-route", want: ""},
		// A key that genuinely ends in a slash is legitimate when the SPA sends
		// it as one encoded segment ("dir%2F"): raw has no literal trailing
		// slash, so it is accepted and decodes to "dir/", matching the served
		// key. Only the raw-trailing-slash form is refused.
		{name: "encoded trailing slash accepted", path: "/api/v1/buckets/b/objects/dir%2F", want: "dir/"},
		// The ambiguous raw-trailing-slash form is refused (returns "").
		{name: "raw trailing slash refused", path: "/api/v1/buckets/b/objects/dir/", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got = "unset"
			resp, err := app.Test(httptest.NewRequest("GET", tc.path, nil))
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			if resp.StatusCode != 200 {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			if got != tc.want {
				t.Errorf("previewObjectKey(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
