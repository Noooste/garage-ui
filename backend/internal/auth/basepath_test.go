package auth

import (
	"net/url"
	"strings"
	"testing"

	"Noooste/garage-ui/internal/config"
)

// Issue #107: behind a path-routing reverse proxy the OIDC redirect URI must
// be {root_url}{base_path}/auth/oidc/callback, because that is what the IdP
// redirects the browser to and what has to be registered with the provider.
func TestNewAuthService_OIDCRedirectURLIncludesBasePath(t *testing.T) {
	tests := []struct {
		name     string
		rootURL  string
		basePath string
		want     string
	}{
		{
			name:    "root deployment is unchanged",
			rootURL: "https://garage-ui.example",
			want:    "https://garage-ui.example/auth/oidc/callback",
		},
		{
			name:     "subpath deployment",
			rootURL:  "https://host.ts.net",
			basePath: "/garage-ui",
			want:     "https://host.ts.net/garage-ui/auth/oidc/callback",
		},
		{
			name:     "unnormalized base path is canonicalised",
			rootURL:  "https://host.ts.net/",
			basePath: "garage-ui/",
			want:     "https://host.ts.net/garage-ui/auth/oidc/callback",
		},
		{
			name:     "nested subpath",
			rootURL:  "https://host.ts.net",
			basePath: "/admin/garage-ui",
			want:     "https://host.ts.net/admin/garage-ui/auth/oidc/callback",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			disco := newDiscoveryServer(t)

			svc, err := NewAuthService(&config.AuthConfig{
				OIDC: config.OIDCConfig{
					Enabled:   true,
					ClientID:  "test-client",
					IssuerURL: disco.URL,
					Scopes:    []string{"openid", "profile"},
					AdminRole: "admin",
				},
			}, &config.ServerConfig{
				RootURL:  tc.rootURL,
				BasePath: tc.basePath,
			})
			if err != nil {
				t.Fatalf("NewAuthService: %v", err)
			}
			if svc.oauth2Config == nil {
				t.Fatal("oauth2Config not initialized")
			}
			if svc.oauth2Config.RedirectURL != tc.want {
				t.Errorf("RedirectURL = %q, want %q", svc.oauth2Config.RedirectURL, tc.want)
			}
		})
	}
}

// The authorization URL handed to the browser carries the same redirect_uri,
// so a wrong base path surfaces as an IdP-side redirect_uri mismatch.
func TestGetAuthorizationURL_CarriesBasePathRedirectURI(t *testing.T) {
	disco := newDiscoveryServer(t)

	svc, err := NewAuthService(&config.AuthConfig{
		OIDC: config.OIDCConfig{
			Enabled:   true,
			ClientID:  "test-client",
			IssuerURL: disco.URL,
			Scopes:    []string{"openid"},
			AdminRole: "admin",
		},
	}, &config.ServerConfig{
		RootURL:  "https://host.ts.net",
		BasePath: "/garage-ui",
	})
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}

	authURL, err := svc.GetAuthorizationURL("state-token")
	if err != nil {
		t.Fatalf("GetAuthorizationURL: %v", err)
	}

	const want = "redirect_uri=https%3A%2F%2Fhost.ts.net%2Fgarage-ui%2Fauth%2Foidc%2Fcallback"
	if !strings.Contains(authURL, want) {
		t.Errorf("authorization URL %q does not carry %q", authURL, want)
	}
}

// Issue #107: the callback URI was prefixed but post_logout_redirect_uri was
// still built from root_url alone. Behind a path-routing proxy that sends the
// browser to a path this app does not own, and providers that validate the
// value against their registered set reject the logout request outright.
func TestPostLogoutRedirectURL_CarriesBasePath(t *testing.T) {
	newService := func(basePath string) *Service {
		return &Service{
			authConfig:         &config.AuthConfig{OIDC: config.OIDCConfig{ClientID: "garage-ui"}},
			serverConfig:       &config.ServerConfig{RootURL: "https://host.ts.net", BasePath: basePath},
			endSessionEndpoint: "https://sso.example.com/logout",
		}
	}

	for _, tt := range []struct{ basePath, want string }{
		{"", "https://host.ts.net/login"},
		{"/garage-ui", "https://host.ts.net/garage-ui/login"},
		{"/admin/garage-ui", "https://host.ts.net/admin/garage-ui/login"},
	} {
		if got := newService(tt.basePath).postLogoutRedirectURL(); got != tt.want {
			t.Errorf("base_path %q: postLogoutRedirectURL = %q, want %q", tt.basePath, got, tt.want)
		}
	}

	// It also has to reach the IdP, not just be computed correctly.
	logoutURL := newService("/garage-ui").LogoutURL("id-token")
	if !strings.Contains(logoutURL, url.QueryEscape("https://host.ts.net/garage-ui/login")) {
		t.Errorf("LogoutURL = %q, want it to carry the prefixed post_logout_redirect_uri", logoutURL)
	}

	// An explicitly configured value still wins.
	svc := newService("/garage-ui")
	svc.authConfig.OIDC.PostLogoutRedirectURL = "https://elsewhere.example/bye"
	if got := svc.postLogoutRedirectURL(); got != "https://elsewhere.example/bye" {
		t.Errorf("configured post_logout_redirect_url = %q, want it used untouched", got)
	}
}
