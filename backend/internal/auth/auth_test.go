package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Noooste/garage-ui/internal/config"
)

func TestExtractRolesFromAccessToken_Keycloak(t *testing.T) {
	payload := map[string]interface{}{
		"resource_access": map[string]interface{}{
			"GarageAdminUi": map[string]interface{}{
				"roles": []interface{}{"admin"},
			},
			"account": map[string]interface{}{
				"roles": []interface{}{"view-profile"},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	token := "header." + base64.RawURLEncoding.EncodeToString(raw) + ".sig"

	svc := &Service{
		authConfig: &config.AuthConfig{
			OIDC: config.OIDCConfig{
				RoleAttributePath: "resource_access.GarageAdminUi.roles",
			},
		},
	}

	roles := svc.ExtractRolesFromAccessToken(token)
	if len(roles) != 1 || roles[0] != "admin" {
		t.Fatalf("expected [admin], got %v", roles)
	}
}

func TestExtractRolesFromAccessToken_NoRoleClaim(t *testing.T) {
	payload := map[string]interface{}{"sub": "user"}
	raw, _ := json.Marshal(payload)
	token := "header." + base64.RawURLEncoding.EncodeToString(raw) + ".sig"

	svc := &Service{
		authConfig: &config.AuthConfig{
			OIDC: config.OIDCConfig{RoleAttributePath: "resource_access.GarageAdminUi.roles"},
		},
	}

	if roles := svc.ExtractRolesFromAccessToken(token); roles != nil {
		t.Fatalf("expected nil, got %v", roles)
	}
}

func TestExtractRolesFromAccessToken_Malformed(t *testing.T) {
	svc := &Service{
		authConfig: &config.AuthConfig{
			OIDC: config.OIDCConfig{RoleAttributePath: "resource_access.GarageAdminUi.roles"},
		},
	}
	if roles := svc.ExtractRolesFromAccessToken("not-a-jwt"); roles != nil {
		t.Fatalf("expected nil for malformed token, got %v", roles)
	}
	if roles := svc.ExtractRolesFromAccessToken(""); roles != nil {
		t.Fatalf("expected nil for empty token, got %v", roles)
	}
}

// Regression for issue #16: admin-only deployments never set
// OIDC.SessionMaxAge, leaving it at 0. Before the fix, that produced a JWT
// whose exp == iat, so every request after /auth/login was rejected as
// expired and the client saw UNAUTHORIZED.
func TestGenerateSessionToken_ZeroSessionMaxAge_IsNotImmediatelyExpired(t *testing.T) {
	jwtSvc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}

	svc := &Service{
		authConfig: &config.AuthConfig{
			Admin: config.AdminAuthConfig{Enabled: true, Username: "admin", Password: "pw"},
			// OIDC disabled; SessionMaxAge left at zero value.
		},
		jwtService: jwtSvc,
	}

	token, err := svc.GenerateSessionToken(&UserInfo{Username: "admin"})
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	if _, err := svc.ValidateSessionToken(token); err != nil {
		t.Fatalf("freshly issued admin session token failed validation: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Task 5: ValidateBasicAuth
// (ParseBasicAuth was removed from production in commit d0040be; nothing to test.)
// ---------------------------------------------------------------------------

func TestValidateBasicAuth(t *testing.T) {
	svc := &Service{
		authConfig: &config.AuthConfig{
			Admin: config.AdminAuthConfig{
				Enabled:  true,
				Username: "admin",
				Password: "correct-horse",
			},
		},
	}

	tests := []struct {
		name string
		user string
		pass string
		want bool
	}{
		{"correct credentials", "admin", "correct-horse", true},
		{"wrong password", "admin", "nope", false},
		{"wrong username", "root", "correct-horse", false},
		{"both wrong", "x", "y", false},
		{"empty username", "", "correct-horse", false},
		{"empty password", "admin", "", false},
		{"both empty", "", "", false},
		{"username prefix attack", "admi", "correct-horse", false},
		{"password prefix attack", "admin", "correct-hors", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := svc.ValidateBasicAuth(tc.user, tc.pass); got != tc.want {
				t.Errorf("ValidateBasicAuth(%q,%q) = %v, want %v", tc.user, tc.pass, got, tc.want)
			}
		})
	}
}

func TestValidateBasicAuth_AdminDisabledStillComparesAgainstEmpty(t *testing.T) {
	// When admin is disabled, the configured username/password are typically
	// empty strings. ValidateBasicAuth itself does not gate on Enabled (that
	// happens in middleware). Pin that behavior so a future refactor can't
	// silently change semantics.
	svc := &Service{
		authConfig: &config.AuthConfig{
			Admin: config.AdminAuthConfig{Enabled: false},
		},
	}
	if !svc.ValidateBasicAuth("", "") {
		t.Error("empty creds should match empty configured creds")
	}
	if svc.ValidateBasicAuth("anything", "") {
		t.Error("non-empty user should not match empty configured user")
	}
}

// ---------------------------------------------------------------------------
// Task 6: OIDC initOIDC cases
// ---------------------------------------------------------------------------

// newDiscoveryServer returns an httptest.Server that serves a minimal but
// valid OIDC discovery document and a JWKS endpoint (empty key set is fine
// for init — we are not verifying any token here). The discovery document's
// `issuer` field MUST equal the server's URL or oidc.NewProvider rejects it.
func newDiscoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]any{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/auth",
			"token_endpoint":                        srv.URL + "/token",
			"jwks_uri":                              srv.URL + "/jwks",
			"userinfo_endpoint":                     srv.URL + "/userinfo",
			"id_token_signing_alg_values_supported": []string{"RS256", "EdDSA"},
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestNewAuthService_OIDCDisabled_DoesNotInitProvider(t *testing.T) {
	svc, err := NewAuthService(
		&config.AuthConfig{
			Admin: config.AdminAuthConfig{Enabled: true, Username: "u", Password: "p"},
			OIDC:  config.OIDCConfig{Enabled: false},
		},
		&config.ServerConfig{},
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	if svc.oidcProvider != nil {
		t.Error("oidcProvider should be nil when OIDC disabled")
	}
	if svc.oidcVerifier != nil {
		t.Error("oidcVerifier should be nil when OIDC disabled")
	}
	if svc.oauth2Config != nil {
		t.Error("oauth2Config should be nil when OIDC disabled")
	}
	if svc.jwtService == nil {
		t.Error("jwtService must always be initialized")
	}
}

func TestNewAuthService_OIDCEnabled_DiscoversProvider(t *testing.T) {
	disco := newDiscoveryServer(t)

	authCfg := &config.AuthConfig{
		OIDC: config.OIDCConfig{
			Enabled:   true,
			ClientID:  "test-client",
			IssuerURL: disco.URL,
			Scopes:    []string{"openid", "profile"},
			AdminRole: "admin",
		},
	}
	srvCfg := &config.ServerConfig{
		RootURL: "https://garage-ui.example",
	}

	svc, err := NewAuthService(authCfg, srvCfg)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	if svc.oidcProvider == nil {
		t.Fatal("oidcProvider not initialized")
	}
	if svc.oidcVerifier == nil {
		t.Fatal("oidcVerifier not initialized")
	}
	if svc.oauth2Config == nil {
		t.Fatal("oauth2Config not initialized")
	}
	if svc.oauth2Config.ClientID != "test-client" {
		t.Errorf("ClientID = %q, want test-client", svc.oauth2Config.ClientID)
	}
	wantRedirect := "https://garage-ui.example/auth/oidc/callback"
	if svc.oauth2Config.RedirectURL != wantRedirect {
		t.Errorf("RedirectURL = %q, want %q", svc.oauth2Config.RedirectURL, wantRedirect)
	}
	if len(svc.oauth2Config.Scopes) != 2 {
		t.Errorf("Scopes length = %d, want 2", len(svc.oauth2Config.Scopes))
	}
	// Endpoint should be wired from the discovery doc.
	if svc.oauth2Config.Endpoint.AuthURL != disco.URL+"/auth" {
		t.Errorf("Endpoint.AuthURL = %q", svc.oauth2Config.Endpoint.AuthURL)
	}
	if svc.oauth2Config.Endpoint.TokenURL != disco.URL+"/token" {
		t.Errorf("Endpoint.TokenURL = %q", svc.oauth2Config.Endpoint.TokenURL)
	}
}

// newTLSDiscoveryServer is the same as newDiscoveryServer but serves the OIDC
// discovery document over HTTPS using httptest's self-signed certificate. The
// cert is not signed by any system-trusted CA, so any HTTP client without
// InsecureSkipVerify (or the cert pinned) will fail to connect.
func newTLSDiscoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]any{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/auth",
			"token_endpoint":                        srv.URL + "/token",
			"jwks_uri":                              srv.URL + "/jwks",
			"userinfo_endpoint":                     srv.URL + "/userinfo",
			"id_token_signing_alg_values_supported": []string{"RS256", "EdDSA"},
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	})
	srv = httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestNewAuthService_OIDCEnabled_SelfSignedIssuer_FailsWithoutTLSSkipVerify(t *testing.T) {
	disco := newTLSDiscoveryServer(t)

	authCfg := &config.AuthConfig{
		OIDC: config.OIDCConfig{
			Enabled:       true,
			ClientID:      "test-client",
			IssuerURL:     disco.URL,
			Scopes:        []string{"openid"},
			TLSSkipVerify: false,
		},
	}
	srvCfg := &config.ServerConfig{RootURL: "https://garage-ui.example"}

	_, err := NewAuthService(authCfg, srvCfg)
	if err == nil {
		t.Fatal("expected TLS verification error from self-signed issuer, got nil")
	}
	if !strings.Contains(err.Error(), "failed to initialize OIDC") {
		t.Errorf("expected wrapping error, got %v", err)
	}
}

func TestNewAuthService_OIDCEnabled_SelfSignedIssuer_SucceedsWithTLSSkipVerify(t *testing.T) {
	disco := newTLSDiscoveryServer(t)

	authCfg := &config.AuthConfig{
		OIDC: config.OIDCConfig{
			Enabled:       true,
			ClientID:      "test-client",
			IssuerURL:     disco.URL,
			Scopes:        []string{"openid"},
			TLSSkipVerify: true,
		},
	}
	srvCfg := &config.ServerConfig{RootURL: "https://garage-ui.example"}

	svc, err := NewAuthService(authCfg, srvCfg)
	if err != nil {
		t.Fatalf("NewAuthService with tls_skip_verify=true should succeed: %v", err)
	}
	if svc.oidcProvider == nil {
		t.Fatal("oidcProvider not initialized")
	}
	if svc.oidcClient == nil {
		t.Fatal("oidcClient should be set when tls_skip_verify=true")
	}
}

func TestNewAuthService_OIDCEnabled_BadIssuerURLReturnsError(t *testing.T) {
	authCfg := &config.AuthConfig{
		OIDC: config.OIDCConfig{
			Enabled:   true,
			ClientID:  "test-client",
			IssuerURL: "http://127.0.0.1:1", // refused — nothing listens on port 1
			Scopes:    []string{"openid"},
			AdminRole: "admin",
		},
	}
	srvCfg := &config.ServerConfig{RootURL: "https://garage-ui.example"}

	_, err := NewAuthService(authCfg, srvCfg)
	if err == nil {
		t.Fatal("expected error for unreachable issuer, got nil")
	}
	if !strings.Contains(err.Error(), "failed to initialize OIDC") {
		t.Errorf("expected wrapping error, got %v", err)
	}
}

func TestGetAuthorizationURL_OIDCDisabledReturnsError(t *testing.T) {
	svc := &Service{authConfig: &config.AuthConfig{}, serverConfig: &config.ServerConfig{}}
	if _, err := svc.GetAuthorizationURL("state-x"); err == nil {
		t.Error("expected error when OIDC not initialized")
	}
}

func TestGetAuthorizationURL_OIDCEnabledIncludesState(t *testing.T) {
	disco := newDiscoveryServer(t)
	svc, err := NewAuthService(
		&config.AuthConfig{
			OIDC: config.OIDCConfig{
				Enabled:   true,
				ClientID:  "test-client",
				IssuerURL: disco.URL,
				Scopes:    []string{"openid"},
				AdminRole: "admin",
			},
		},
		&config.ServerConfig{RootURL: "https://garage-ui.example"},
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}

	url, err := svc.GetAuthorizationURL("my-state-token")
	if err != nil {
		t.Fatalf("GetAuthorizationURL: %v", err)
	}
	if !strings.Contains(url, "state=my-state-token") {
		t.Errorf("URL missing state param: %s", url)
	}
	if !strings.Contains(url, "client_id=test-client") {
		t.Errorf("URL missing client_id: %s", url)
	}
	if !strings.Contains(url, "redirect_uri=") {
		t.Errorf("URL missing redirect_uri: %s", url)
	}
}

// ---------------------------------------------------------------------------
// Task 7: ValidateSessionToken and expanded ExtractRolesFromAccessToken
// ---------------------------------------------------------------------------

// newServiceWithJWT wires a Service with a real JWTService so the session
// helpers can be exercised end-to-end. OIDC is left disabled.
func newServiceWithJWT(t *testing.T) *Service {
	t.Helper()
	jwtSvc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	return &Service{
		authConfig:   &config.AuthConfig{},
		serverConfig: &config.ServerConfig{},
		jwtService:   jwtSvc,
	}
}

func TestValidateSessionToken_HappyPath(t *testing.T) {
	svc := newServiceWithJWT(t)
	user := &UserInfo{
		Username: "alice",
		Email:    "alice@example.com",
		Name:     "Alice",
		Roles:    []string{"admin"},
	}
	tok, err := svc.GenerateSessionToken(user)
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}
	got, err := svc.ValidateSessionToken(tok)
	if err != nil {
		t.Fatalf("ValidateSessionToken: %v", err)
	}
	if got.Username != user.Username || got.Email != user.Email || got.Name != user.Name {
		t.Errorf("got %+v, want %+v", got, user)
	}
	if len(got.Roles) != 1 || got.Roles[0] != "admin" {
		t.Errorf("Roles = %v, want [admin]", got.Roles)
	}
}

func TestValidateSessionToken_Expired(t *testing.T) {
	svc := newServiceWithJWT(t)
	// Bypass GenerateSessionToken's "fall back to 24h on non-positive" guard
	// by going straight through the JWT service with a negative TTL.
	tok, err := svc.jwtService.GenerateToken(&UserInfo{Username: "a"}, -1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if _, err := svc.ValidateSessionToken(tok); err == nil {
		t.Error("expected expired-token error, got nil")
	}
}

func TestValidateSessionToken_BadSignatureRejected(t *testing.T) {
	signer := newServiceWithJWT(t)
	verifier := newServiceWithJWT(t)
	tok, err := signer.GenerateSessionToken(&UserInfo{Username: "a"})
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}
	if _, err := verifier.ValidateSessionToken(tok); err == nil {
		t.Error("expected signature-mismatch error, got nil")
	}
}

func TestValidateSessionToken_EmptyTokenRejected(t *testing.T) {
	svc := newServiceWithJWT(t)
	if _, err := svc.ValidateSessionToken(""); err == nil {
		t.Error("expected error for empty token, got nil")
	}
}

// makeAccessToken builds a JWT-shaped string with arbitrary claims. The
// signature segment is junk because ExtractRolesFromAccessToken does NOT
// verify the signature (per the doc-comment: "obtained via a verified code
// exchange, so parsing without re-verifying is safe").
func makeAccessToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(raw) + ".sig"
}

func TestExtractRolesFromAccessToken_DeeplyNestedPath(t *testing.T) {
	tok := makeAccessToken(t, map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"roles": []any{"r1", "r2", "r3"},
				},
			},
		},
	})
	svc := &Service{
		authConfig: &config.AuthConfig{
			OIDC: config.OIDCConfig{RoleAttributePath: "a.b.c.roles"},
		},
	}
	got := svc.ExtractRolesFromAccessToken(tok)
	if len(got) != 3 || got[0] != "r1" || got[2] != "r3" {
		t.Errorf("got %v, want [r1 r2 r3]", got)
	}
}

func TestExtractRolesFromAccessToken_MixedTypeArrayDropsNonStrings(t *testing.T) {
	tok := makeAccessToken(t, map[string]any{
		"roles": []any{"admin", 42, "viewer", true, nil, "writer"},
	})
	svc := &Service{
		authConfig: &config.AuthConfig{
			OIDC: config.OIDCConfig{RoleAttributePath: "roles"},
		},
	}
	got := svc.ExtractRolesFromAccessToken(tok)
	want := []string{"admin", "viewer", "writer"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractRolesFromAccessToken_EmptyPathReturnsNil(t *testing.T) {
	tok := makeAccessToken(t, map[string]any{"roles": []any{"admin"}})
	svc := &Service{
		authConfig: &config.AuthConfig{OIDC: config.OIDCConfig{RoleAttributePath: ""}},
	}
	if got := svc.ExtractRolesFromAccessToken(tok); got != nil {
		t.Errorf("expected nil for empty path, got %v", got)
	}
}

func TestExtractRolesFromAccessToken_IntermediateNodeNotMap(t *testing.T) {
	// Path tries to descend through a string — extractRoles must bail with nil.
	tok := makeAccessToken(t, map[string]any{
		"resource_access": "this-should-be-a-map",
	})
	svc := &Service{
		authConfig: &config.AuthConfig{
			OIDC: config.OIDCConfig{RoleAttributePath: "resource_access.client.roles"},
		},
	}
	if got := svc.ExtractRolesFromAccessToken(tok); got != nil {
		t.Errorf("expected nil when path traverses non-map, got %v", got)
	}
}

func TestExtractRolesFromAccessToken_ScalarStringRoleReturnsSingleElement(t *testing.T) {
	// A role_attribute_path that resolves to a scalar string (common when an IdP
	// emits a single role, e.g. "garage_role": "garage-ui-admin") must be treated
	// as a one-element role list, not silently discarded. Discarding it caused
	// admin_role (singular) + scalar claim to yield roles=[] and a spurious 403.
	tok := makeAccessToken(t, map[string]any{
		"roles": "admin",
	})
	svc := &Service{
		authConfig: &config.AuthConfig{
			OIDC: config.OIDCConfig{RoleAttributePath: "roles"},
		},
	}
	got := svc.ExtractRolesFromAccessToken(tok)
	if len(got) != 1 || got[0] != "admin" {
		t.Errorf("got %v, want [admin]", got)
	}
}

func TestExtractRolesFromAccessToken_EmptyScalarStringReturnsNil(t *testing.T) {
	// An empty scalar must not produce a [""] role, which would never match a
	// configured admin role and only muddies logs.
	tok := makeAccessToken(t, map[string]any{
		"roles": "",
	})
	svc := &Service{
		authConfig: &config.AuthConfig{
			OIDC: config.OIDCConfig{RoleAttributePath: "roles"},
		},
	}
	if got := svc.ExtractRolesFromAccessToken(tok); got != nil {
		t.Errorf("expected nil for empty scalar role, got %v", got)
	}
}

func TestExtractRolesFromAccessToken_BadBase64InPayload(t *testing.T) {
	svc := &Service{
		authConfig: &config.AuthConfig{
			OIDC: config.OIDCConfig{RoleAttributePath: "roles"},
		},
	}
	if got := svc.ExtractRolesFromAccessToken("hdr.!!!not-base64!!!.sig"); got != nil {
		t.Errorf("expected nil for bad base64, got %v", got)
	}
}

func TestExtractRolesFromAccessToken_BadJSONInPayload(t *testing.T) {
	svc := &Service{
		authConfig: &config.AuthConfig{
			OIDC: config.OIDCConfig{RoleAttributePath: "roles"},
		},
	}
	// Valid base64 of "not json"
	tok := "hdr." + base64.RawURLEncoding.EncodeToString([]byte("not json")) + ".sig"
	if got := svc.ExtractRolesFromAccessToken(tok); got != nil {
		t.Errorf("expected nil for non-JSON payload, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Task 8: IsAdmin coverage
// ---------------------------------------------------------------------------

func TestIsAdmin(t *testing.T) {
	tests := []struct {
		name       string
		adminRole  string
		adminRoles []string
		userRoles  []string
		want       bool
	}{
		{"empty admin role config returns false", "", nil, []string{"admin"}, false},
		{"user has admin role", "admin", nil, []string{"viewer", "admin"}, true},
		{"user lacks admin role", "admin", nil, []string{"viewer"}, false},
		{"user has no roles", "admin", nil, nil, false},
		{"role match is exact (case-sensitive)", "admin", nil, []string{"Admin"}, false},

		// admin_roles list
		{"user matches first entry in admin_roles", "", []string{"group1", "group2"}, []string{"group1"}, true},
		{"user matches second entry in admin_roles", "", []string{"group1", "group2"}, []string{"group2"}, true},
		{"user matches none of admin_roles", "", []string{"group1", "group2"}, []string{"group3"}, false},
		{"empty admin_roles list returns false", "", []string{}, []string{"group1"}, false},

		// admin_role + admin_roles merge
		{"matches single admin_role when admin_roles set too", "admin", []string{"ops"}, []string{"admin"}, true},
		{"matches admin_roles entry when admin_role set too", "admin", []string{"ops"}, []string{"ops"}, true},
		{"matches neither admin_role nor admin_roles", "admin", []string{"ops"}, []string{"viewer"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &Service{
				authConfig: &config.AuthConfig{
					OIDC: config.OIDCConfig{AdminRole: tc.adminRole, AdminRoles: tc.adminRoles},
				},
			}
			if got := svc.IsAdmin(&UserInfo{Roles: tc.userRoles}); got != tc.want {
				t.Errorf("IsAdmin = %v, want %v", got, tc.want)
			}
		})
	}
}
