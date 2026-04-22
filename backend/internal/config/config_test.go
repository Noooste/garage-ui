package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// writeConfigFile writes yaml content to a temp path and returns it.
func writeConfigFile(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// resetViper clears all global viper state between tests.
func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
}

// minimalValidYAML is the smallest configuration that passes Validate.
const minimalValidYAML = `
server:
  host: "0.0.0.0"
  port: 8080
  environment: development
garage:
  endpoint: http://garage:3900
  admin_endpoint: http://garage:3903
  admin_token: supersecret
`

func TestLoad_YAMLOnly(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, minimalValidYAML)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want 0.0.0.0", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.Environment != "development" {
		t.Errorf("Server.Environment = %q, want development", cfg.Server.Environment)
	}
	if cfg.Garage.Endpoint != "http://garage:3900" {
		t.Errorf("Garage.Endpoint = %q", cfg.Garage.Endpoint)
	}
	if cfg.Garage.AdminToken != "supersecret" {
		t.Errorf("Garage.AdminToken = %q", cfg.Garage.AdminToken)
	}
}

func TestLoad_EnvOnly_MissingFile(t *testing.T) {
	resetViper(t)
	// Point at a path that definitely does not exist. Load tolerates missing
	// files and falls back to env + viper defaults.
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	// Every required field provided via env.
	t.Setenv("GARAGE_UI_SERVER_PORT", "9090")
	t.Setenv("GARAGE_UI_GARAGE_ENDPOINT", "http://g:3900")
	t.Setenv("GARAGE_UI_GARAGE_ADMIN_ENDPOINT", "http://g:3903")
	t.Setenv("GARAGE_UI_GARAGE_ADMIN_TOKEN", "env-token")

	cfg, err := Load(missing)
	if err != nil {
		t.Fatalf("Load with env-only: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090 (from env)", cfg.Server.Port)
	}
	if cfg.Garage.AdminToken != "env-token" {
		t.Errorf("Garage.AdminToken = %q, want env-token", cfg.Garage.AdminToken)
	}
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, minimalValidYAML)

	// YAML has port=8080; env should win.
	t.Setenv("GARAGE_UI_SERVER_PORT", "9090")
	t.Setenv("GARAGE_UI_GARAGE_ADMIN_TOKEN", "env-wins")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090 (env override)", cfg.Server.Port)
	}
	if cfg.Garage.AdminToken != "env-wins" {
		t.Errorf("Garage.AdminToken = %q, want env-wins", cfg.Garage.AdminToken)
	}
	// Host was not overridden; YAML value should persist.
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want 0.0.0.0 (from YAML)", cfg.Server.Host)
	}
}

func TestLoad_MalformedYAMLReturnsError(t *testing.T) {
	resetViper(t)
	// Deliberately broken YAML: unindented key after a mapping start.
	path := writeConfigFile(t, "server:\n  port: 8080\n:: not: valid ::\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
	if !strings.Contains(err.Error(), "error reading config file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_ValidationFailurePropagates(t *testing.T) {
	resetViper(t)
	// Valid YAML syntax but Garage.Endpoint is blank → Validate must fail.
	path := writeConfigFile(t, `
server:
  port: 8080
garage:
  endpoint: ""
  admin_endpoint: http://g:3903
  admin_token: t
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid configuration") {
		t.Errorf("expected wrapped invalid-config error, got %v", err)
	}
	if !strings.Contains(err.Error(), "garage endpoint is required") {
		t.Errorf("expected endpoint-required message, got %v", err)
	}
}

// validBaseConfig returns a deep copy of a minimal Config that passes Validate.
func validBaseConfig() Config {
	return Config{
		Server: ServerConfig{Port: 8080},
		Garage: GarageConfig{
			Endpoint:      "http://g:3900",
			AdminEndpoint: "http://g:3903",
			AdminToken:    "t",
		},
	}
}

// applyValidOIDC fills OIDC with all required fields.
func applyValidOIDC(c *Config) {
	c.Auth.OIDC.Enabled = true
	c.Auth.OIDC.ClientID = "client-xyz"
	c.Auth.OIDC.IssuerURL = "https://idp.example/realms/test"
	c.Auth.OIDC.Scopes = []string{"openid"}
	c.Auth.OIDC.AdminRole = "admin"
	c.Server.RootURL = "https://garage-ui.example"
}

// Note on spec coverage: spec/2026-04-17-backend-test-suite-design.md lists
// "invalid log level/format" as a Validate case, but the current Validate does
// not check Logging.Level or Logging.Format. That's a code-vs-spec gap to
// resolve in a follow-up plan; Stage 2 tests the current behavior only.
func TestValidate(t *testing.T) {
	tests := []struct {
		name            string
		mutate          func(*Config)
		wantErrContains string // empty = expect no error
	}{
		{
			name:   "valid minimal config",
			mutate: func(c *Config) {},
		},
		{
			name:            "port zero is invalid",
			mutate:          func(c *Config) { c.Server.Port = 0 },
			wantErrContains: "invalid server port",
		},
		{
			name:            "port negative is invalid",
			mutate:          func(c *Config) { c.Server.Port = -1 },
			wantErrContains: "invalid server port",
		},
		{
			name:            "port above 65535 is invalid",
			mutate:          func(c *Config) { c.Server.Port = 70000 },
			wantErrContains: "invalid server port",
		},
		{
			name:            "port at 65535 is valid",
			mutate:          func(c *Config) { c.Server.Port = 65535 },
			wantErrContains: "",
		},
		{
			name:            "missing garage endpoint",
			mutate:          func(c *Config) { c.Garage.Endpoint = "" },
			wantErrContains: "garage endpoint is required",
		},
		{
			name:            "missing garage admin_endpoint",
			mutate:          func(c *Config) { c.Garage.AdminEndpoint = "" },
			wantErrContains: "admin_endpoint is required",
		},
		{
			name:            "missing garage admin_token",
			mutate:          func(c *Config) { c.Garage.AdminToken = "" },
			wantErrContains: "admin_token is required",
		},
		{
			name: "admin auth enabled without username",
			mutate: func(c *Config) {
				c.Auth.Admin.Enabled = true
				c.Auth.Admin.Password = "p"
			},
			wantErrContains: "admin auth username and password are required",
		},
		{
			name: "admin auth enabled without password",
			mutate: func(c *Config) {
				c.Auth.Admin.Enabled = true
				c.Auth.Admin.Username = "u"
			},
			wantErrContains: "admin auth username and password are required",
		},
		{
			name: "admin auth enabled with both set is valid",
			mutate: func(c *Config) {
				c.Auth.Admin.Enabled = true
				c.Auth.Admin.Username = "u"
				c.Auth.Admin.Password = "p"
			},
			wantErrContains: "",
		},
		{
			name: "admin auth disabled ignores missing credentials",
			mutate: func(c *Config) {
				c.Auth.Admin.Enabled = false
				c.Auth.Admin.Username = ""
				c.Auth.Admin.Password = ""
			},
			wantErrContains: "",
		},
		{
			name: "oidc enabled without client_id",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Auth.OIDC.ClientID = ""
			},
			wantErrContains: "oidc client_id is required",
		},
		{
			name: "oidc enabled without issuer_url",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Auth.OIDC.IssuerURL = ""
			},
			wantErrContains: "oidc issuer_url is required",
		},
		{
			name: "oidc enabled without server.root_url",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Server.RootURL = ""
			},
			wantErrContains: "server.root_url is required",
		},
		{
			name: "oidc enabled without scopes",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Auth.OIDC.Scopes = nil
			},
			wantErrContains: "oidc scopes are required",
		},
		{
			name: "oidc enabled without admin_role rejected for safety",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Auth.OIDC.AdminRole = ""
			},
			wantErrContains: "oidc admin_role is required",
		},
		{
			name:            "oidc fully configured is valid",
			mutate:          applyValidOIDC,
			wantErrContains: "",
		},
		{
			name: "oidc disabled ignores missing client_id",
			mutate: func(c *Config) {
				c.Auth.OIDC.Enabled = false
				c.Auth.OIDC.ClientID = ""
			},
			wantErrContains: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBaseConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()

			if tc.wantErrContains == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErrContains)
			}
			if !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrContains)
			}
		})
	}
}

func TestGetAddress(t *testing.T) {
	tests := []struct {
		host string
		port int
		want string
	}{
		{"localhost", 8080, "localhost:8080"},
		{"0.0.0.0", 80, "0.0.0.0:80"},
		{"", 443, ":443"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			cfg := &Config{Server: ServerConfig{Host: tc.host, Port: tc.port}}
			if got := cfg.GetAddress(); got != tc.want {
				t.Errorf("GetAddress() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsDevelopment(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"development", true},
		{"production", false},
		{"", false},
		// Case-sensitive per current impl; lock in that behavior.
		{"Development", false},
		{"DEV", false},
	}
	for _, tc := range tests {
		t.Run(tc.env, func(t *testing.T) {
			cfg := &Config{Server: ServerConfig{Environment: tc.env}}
			if got := cfg.IsDevelopment(); got != tc.want {
				t.Errorf("IsDevelopment(%q) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

func writeToml(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "garage.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	return path
}

const testGarageToml = `
[admin]
api_bind_addr = "[::]:3903"
admin_token = "toml-token"

[s3_api]
api_bind_addr = "[::]:3900"
s3_region = "garage"
`

func TestLoad_GarageTomlOnly(t *testing.T) {
	resetViper(t)
	tomlPath := writeToml(t, testGarageToml)
	missingYaml := filepath.Join(t.TempDir(), "nope.yaml")

	cfg, err := Load(missingYaml, WithGarageToml(tomlPath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Garage.AdminToken != "toml-token" {
		t.Errorf("AdminToken = %q, want toml-token", cfg.Garage.AdminToken)
	}
	if cfg.Garage.Endpoint != "http://127.0.0.1:3900" {
		t.Errorf("Endpoint = %q, want http://127.0.0.1:3900", cfg.Garage.Endpoint)
	}
	if cfg.Garage.AdminEndpoint != "http://127.0.0.1:3903" {
		t.Errorf("AdminEndpoint = %q, want http://127.0.0.1:3903", cfg.Garage.AdminEndpoint)
	}
	if cfg.Garage.Region != "garage" {
		t.Errorf("Region = %q, want garage", cfg.Garage.Region)
	}
}

func TestLoad_YAMLOverridesToml(t *testing.T) {
	resetViper(t)
	tomlPath := writeToml(t, testGarageToml)
	yaml := `
server:
  host: "0.0.0.0"
  port: 8080
garage:
  endpoint: http://custom:3900
  admin_endpoint: http://custom:3903
  admin_token: yaml-wins
`
	yamlPath := writeConfigFile(t, yaml)

	cfg, err := Load(yamlPath, WithGarageToml(tomlPath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Garage.AdminToken != "yaml-wins" {
		t.Errorf("AdminToken = %q, want yaml-wins (yaml overrides toml)", cfg.Garage.AdminToken)
	}
	if cfg.Garage.Endpoint != "http://custom:3900" {
		t.Errorf("Endpoint = %q, want http://custom:3900", cfg.Garage.Endpoint)
	}
}

func TestLoad_EnvOverridesToml(t *testing.T) {
	resetViper(t)
	tomlPath := writeToml(t, testGarageToml)
	missingYaml := filepath.Join(t.TempDir(), "nope.yaml")
	t.Setenv("GARAGE_UI_GARAGE_ADMIN_TOKEN", "env-wins")

	cfg, err := Load(missingYaml, WithGarageToml(tomlPath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Garage.AdminToken != "env-wins" {
		t.Errorf("AdminToken = %q, want env-wins (env overrides toml)", cfg.Garage.AdminToken)
	}
}

func TestValidate_TokenAuthAutoEnabled(t *testing.T) {
	cfg := validBaseConfig()
	cfg.ResolveTokenAuth()
	if !cfg.Auth.Token.Enabled {
		t.Error("expected token auth to be auto-enabled when no other auth is configured")
	}
}

func TestValidate_TokenAuthNotAutoEnabledWhenAdminEnabled(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Auth.Admin.Enabled = true
	cfg.Auth.Admin.Username = "u"
	cfg.Auth.Admin.Password = "p"
	cfg.ResolveTokenAuth()
	if cfg.Auth.Token.Enabled {
		t.Error("expected token auth to stay disabled when admin auth is configured")
	}
}

func TestValidate_TokenAuthNotAutoEnabledWhenOIDCEnabled(t *testing.T) {
	cfg := validBaseConfig()
	applyValidOIDC(&cfg)
	cfg.ResolveTokenAuth()
	if cfg.Auth.Token.Enabled {
		t.Error("expected token auth to stay disabled when OIDC is configured")
	}
}

func TestValidate_TokenAuthExplicitlyEnabled(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Auth.Admin.Enabled = true
	cfg.Auth.Admin.Username = "u"
	cfg.Auth.Admin.Password = "p"
	cfg.Auth.Token.Enabled = true
	cfg.ResolveTokenAuth()
	if !cfg.Auth.Token.Enabled {
		t.Error("expected token auth to stay enabled when explicitly set")
	}
}

func TestIsProduction(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"production", true},
		{"development", false},
		{"", false},
		{"Production", false},
		{"PROD", false},
	}
	for _, tc := range tests {
		t.Run(tc.env, func(t *testing.T) {
			cfg := &Config{Server: ServerConfig{Environment: tc.env}}
			if got := cfg.IsProduction(); got != tc.want {
				t.Errorf("IsProduction(%q) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}
