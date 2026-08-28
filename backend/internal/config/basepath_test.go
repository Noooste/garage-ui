package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeBasePath(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"empty means root", "", ""},
		{"single slash means root", "/", ""},
		{"repeated slashes mean root", "///", ""},
		{"whitespace only means root", "   ", ""},
		{"leading slash added", "garage-ui", "/garage-ui"},
		{"already canonical", "/garage-ui", "/garage-ui"},
		{"trailing slash stripped", "/garage-ui/", "/garage-ui"},
		{"multiple trailing slashes stripped", "/garage-ui//", "/garage-ui"},
		{"nested path", "/admin/garage-ui", "/admin/garage-ui"},
		{"nested path normalized", "admin/garage-ui/", "/admin/garage-ui"},
		{"inner empty segments collapsed", "//admin///garage-ui//", "/admin/garage-ui"},
		{"surrounding whitespace trimmed", "  /garage-ui  ", "/garage-ui"},
		{"unreserved characters kept", "/garage_ui.v2-final~1", "/garage_ui.v2-final~1"},
		{"percent encoding kept", "/garage%20ui", "/garage%20ui"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeBasePath(tc.raw)
			if err != nil {
				t.Fatalf("NormalizeBasePath(%q) returned error: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("NormalizeBasePath(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNormalizeBasePath_Idempotent(t *testing.T) {
	for _, raw := range []string{"", "/", "garage-ui/", "//a//b//"} {
		once, err := NormalizeBasePath(raw)
		if err != nil {
			t.Fatalf("NormalizeBasePath(%q): %v", raw, err)
		}
		twice, err := NormalizeBasePath(once)
		if err != nil {
			t.Fatalf("NormalizeBasePath(%q): %v", once, err)
		}
		if once != twice {
			t.Errorf("not idempotent for %q: %q then %q", raw, once, twice)
		}
	}
}

func TestNormalizeBasePath_Rejects(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"query string", "/garage-ui?foo=bar"},
		{"fragment", "/garage-ui#top"},
		{"backslash", "\\garage-ui"},
		{"absolute URL", "https://garage.example/garage-ui"},
		{"dot segment", "/garage-ui/."},
		{"parent segment", "/garage-ui/../admin"},
		{"inner whitespace", "/garage ui"},
		{"tab", "/garage\tui"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeBasePath(tc.raw)
			if err == nil {
				t.Fatalf("NormalizeBasePath(%q) = %q, want error", tc.raw, got)
			}
		})
	}
}

func TestServerConfigPrefixPath(t *testing.T) {
	tests := []struct {
		basePath string
		path     string
		want     string
	}{
		{"", "/api/v1/health", "/api/v1/health"},
		{"", "/", "/"},
		{"/garage-ui", "/api/v1/health", "/garage-ui/api/v1/health"},
		{"/garage-ui", "/", "/garage-ui"},
		{"/garage-ui", "", "/garage-ui"},
		{"garage-ui/", "/auth/oidc/callback", "/garage-ui/auth/oidc/callback"},
		{"/garage-ui", "login", "/garage-ui/login"},
		// An invalid value never reaches a running server (Load rejects it);
		// the accessor must not produce a half-applied prefix either.
		{"https://evil.example", "/login", "/login"},
	}

	for _, tc := range tests {
		s := ServerConfig{BasePath: tc.basePath}
		if got := s.PrefixPath(tc.path); got != tc.want {
			t.Errorf("ServerConfig{BasePath:%q}.PrefixPath(%q) = %q, want %q", tc.basePath, tc.path, got, tc.want)
		}
	}
}

func TestServerConfigExternalURL(t *testing.T) {
	tests := []struct {
		name     string
		rootURL  string
		basePath string
		want     string
	}{
		{
			name:    "root deployment keeps historical shape",
			rootURL: "https://garage-ui.example",
			want:    "https://garage-ui.example/auth/oidc/callback",
		},
		{
			name:     "subpath deployment",
			rootURL:  "https://tailnet-host.ts.net",
			basePath: "/garage-ui",
			want:     "https://tailnet-host.ts.net/garage-ui/auth/oidc/callback",
		},
		{
			name:     "trailing slash on root_url does not double up",
			rootURL:  "https://tailnet-host.ts.net/",
			basePath: "/garage-ui",
			want:     "https://tailnet-host.ts.net/garage-ui/auth/oidc/callback",
		},
		{
			name:    "trailing slash on root_url without base path",
			rootURL: "https://garage-ui.example/",
			want:    "https://garage-ui.example/auth/oidc/callback",
		},
		{
			name:     "nested base path",
			rootURL:  "https://host.example",
			basePath: "admin/garage-ui/",
			want:     "https://host.example/admin/garage-ui/auth/oidc/callback",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := ServerConfig{RootURL: tc.rootURL, BasePath: tc.basePath}
			if got := s.ExternalURL("/auth/oidc/callback"); got != tc.want {
				t.Errorf("ExternalURL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJoinBasePath(t *testing.T) {
	tests := []struct {
		base string
		path string
		want string
	}{
		{"", "/login?login=success", "/login?login=success"},
		{"/garage-ui", "/login?login=success", "/garage-ui/login?login=success"},
		{"", "/", "/"},
		{"/garage-ui", "/", "/garage-ui"},
	}
	for _, tc := range tests {
		if got := JoinBasePath(tc.base, tc.path); got != tc.want {
			t.Errorf("JoinBasePath(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
	}
}

// The env var named in issue #107 must reach server.base_path, and Load must
// hand back the canonical form so no consumer has to normalize again.
func TestLoad_BasePathFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		want    string
		wantErr bool
	}{
		{name: "unset means root", env: "", want: ""},
		{name: "normalized on the way in", env: "garage-ui/", want: "/garage-ui"},
		{name: "canonical passes through", env: "/garage-ui", want: "/garage-ui"},
		{name: "invalid value fails the boot", env: "/garage-ui?x=1", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv("GARAGE_UI_SERVER_BASE_PATH", tc.env)
			}
			t.Setenv("GARAGE_UI_GARAGE_ENDPOINT", "localhost:3900")
			t.Setenv("GARAGE_UI_GARAGE_ADMIN_ENDPOINT", "http://localhost:3903")
			t.Setenv("GARAGE_UI_GARAGE_ADMIN_TOKEN", "token")

			cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Load() succeeded with base path %q, want error", tc.env)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load(): %v", err)
			}
			if cfg.Server.BasePath != tc.want {
				t.Errorf("cfg.Server.BasePath = %q, want %q", cfg.Server.BasePath, tc.want)
			}
		})
	}
}

func TestLoad_BasePathFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := `
server:
  port: 8080
  base_path: /garage-ui/
garage:
  endpoint: localhost:3900
  admin_endpoint: http://localhost:3903
  admin_token: token
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.Server.BasePath != "/garage-ui" {
		t.Errorf("cfg.Server.BasePath = %q, want %q", cfg.Server.BasePath, "/garage-ui")
	}
}

func TestValidate_RejectsInvalidBasePath(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080, BasePath: "/garage-ui?x=1"},
		Garage: GarageConfig{Endpoint: "localhost:3900", AdminEndpoint: "http://localhost:3903", AdminToken: "t"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() accepted a base path containing a query string")
	}
}
