package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"Noooste/garage-ui/internal/config"
)

// newOIDCTeamFixture builds an OIDC-enabled fixture with team_attribute_path
// set and no admin-role gate, so a non-admin user can complete the callback
// and have their teams resolved.
func newOIDCTeamFixture(t *testing.T, teamPath string) (*routeFixture, *testIssuer) {
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
			AdminRole:         "", // no role gate: non-admin OIDC users may log in
			UsernameAttribute: "preferred_username",
			EmailAttribute:    "email",
			NameAttribute:     "name",
			RoleAttributePath: "resource_access.test-client.roles",
			TeamAttributePath: teamPath,
			CookieName:        "session",
			CookieHTTPOnly:    true,
			CookieSameSite:    "Lax",
			SessionMaxAge:     3600,
		}
	})
	return f, iss
}

// runCallback drives the OIDC callback happy path and returns the resolved
// session's user info (decoded from the session cookie).
func runCallbackTeams(t *testing.T, f *routeFixture) []string {
	t.Helper()
	state := oidcState(t, f)
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state="+state+"&code=c", nil)
	resp, err := f.App.Test(req)
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	if resp.StatusCode != 303 {
		t.Fatalf("callback status = %d, want 303", resp.StatusCode)
	}
	var sess *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session" && c.Value != "" {
			sess = c
		}
	}
	if sess == nil {
		t.Fatal("no session cookie in callback response")
	}
	info, err := f.Auth.ValidateSessionToken(sess.Value)
	if err != nil {
		t.Fatalf("ValidateSessionToken: %v", err)
	}
	return info.Teams
}

func TestRoutes_OIDCCallback_TeamsFromIDToken(t *testing.T) {
	f, iss := newOIDCTeamFixture(t, "groups")
	// The verified ID token carries the team claim directly.
	iss.DefaultIDClaims["groups"] = []any{"garage-team-backend"}

	teams := runCallbackTeams(t, f)
	if len(teams) != 1 || teams[0] != "garage-team-backend" {
		t.Errorf("Teams = %v, want [garage-team-backend] from the ID token", teams)
	}
}

func TestRoutes_OIDCCallback_TeamsFromAccessTokenFallback(t *testing.T) {
	f, iss := newOIDCTeamFixture(t, "groups")
	// ID token carries no team claim; the access token does.
	iss.DefaultAccessClaims["groups"] = []any{"garage-team-data"}

	teams := runCallbackTeams(t, f)
	if len(teams) != 1 || teams[0] != "garage-team-data" {
		t.Errorf("Teams = %v, want [garage-team-data] from the access-token fallback", teams)
	}
}

func TestRoutes_OIDCCallback_TeamsFromUserInfoFallback(t *testing.T) {
	f, iss := newOIDCTeamFixture(t, "groups")
	// Neither token carries the team claim; only the userinfo endpoint does.
	iss.UserInfoFn = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sub":"user-1","preferred_username":"alice","email":"alice@example.com","groups":["garage-team-ops"]}`))
	}

	teams := runCallbackTeams(t, f)
	if len(teams) != 1 || teams[0] != "garage-team-ops" {
		t.Errorf("Teams = %v, want [garage-team-ops] from the userinfo fallback", teams)
	}
}
