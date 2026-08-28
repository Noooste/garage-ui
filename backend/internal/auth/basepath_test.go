package auth

import (
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
