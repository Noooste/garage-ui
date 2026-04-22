package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTomlFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "garage.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	return path
}

const validGarageToml = `
metadata_dir = "/var/lib/garage/meta"
data_dir = "/var/lib/garage/data"
replication_factor = 1

rpc_bind_addr = "[::]:3901"
rpc_public_addr = "127.0.0.1:3901"
rpc_secret = "0000000000000000000000000000000000000000000000000000000000000000"

[s3_api]
s3_region = "garage"
api_bind_addr = "[::]:3900"
root_domain = ".s3.garage"

[s3_web]
bind_addr = "[::]:3902"
root_domain = ".web.garage"
index = "index.html"

[admin]
api_bind_addr = "[::]:3903"
admin_token = "my-secret-admin-token"
`

func TestParseGarageToml_ValidFile(t *testing.T) {
	path := writeTomlFile(t, validGarageToml)
	result, err := ParseGarageToml(path)
	if err != nil {
		t.Fatalf("ParseGarageToml: %v", err)
	}
	if result.AdminToken != "my-secret-admin-token" {
		t.Errorf("AdminToken = %q, want my-secret-admin-token", result.AdminToken)
	}
	if result.AdminEndpoint != "http://127.0.0.1:3903" {
		t.Errorf("AdminEndpoint = %q, want http://127.0.0.1:3903", result.AdminEndpoint)
	}
	if result.Endpoint != "http://127.0.0.1:3900" {
		t.Errorf("Endpoint = %q, want http://127.0.0.1:3900", result.Endpoint)
	}
	if result.Region != "garage" {
		t.Errorf("Region = %q, want garage", result.Region)
	}
}

func TestParseGarageToml_MissingAdminToken(t *testing.T) {
	toml := `
[admin]
api_bind_addr = "[::]:3903"
[s3_api]
api_bind_addr = "[::]:3900"
`
	path := writeTomlFile(t, toml)
	_, err := ParseGarageToml(path)
	if err == nil {
		t.Fatal("expected error for missing admin_token, got nil")
	}
}

func TestParseGarageToml_MissingAdminBindAddr(t *testing.T) {
	toml := `
[admin]
admin_token = "tok"
[s3_api]
api_bind_addr = "[::]:3900"
`
	path := writeTomlFile(t, toml)
	_, err := ParseGarageToml(path)
	if err == nil {
		t.Fatal("expected error for missing admin api_bind_addr, got nil")
	}
}

func TestParseGarageToml_MissingS3BindAddr(t *testing.T) {
	toml := `
[admin]
admin_token = "tok"
api_bind_addr = "[::]:3903"
[s3_api]
s3_region = "garage"
`
	path := writeTomlFile(t, toml)
	_, err := ParseGarageToml(path)
	if err == nil {
		t.Fatal("expected error for missing s3_api api_bind_addr, got nil")
	}
}

func TestParseGarageToml_DefaultRegion(t *testing.T) {
	toml := `
[admin]
admin_token = "tok"
api_bind_addr = "[::]:3903"
[s3_api]
api_bind_addr = "[::]:3900"
`
	path := writeTomlFile(t, toml)
	result, err := ParseGarageToml(path)
	if err != nil {
		t.Fatalf("ParseGarageToml: %v", err)
	}
	if result.Region != "garage" {
		t.Errorf("Region = %q, want garage (default)", result.Region)
	}
}

func TestParseGarageToml_FileNotFound(t *testing.T) {
	_, err := ParseGarageToml("/nonexistent/garage.toml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestConvertBindAddr(t *testing.T) {
	tests := []struct {
		name     string
		bindAddr string
		want     string
		wantErr  bool
	}{
		{"ipv6 wildcard", "[::]:3900", "http://127.0.0.1:3900", false},
		{"ipv4 wildcard", "0.0.0.0:3900", "http://127.0.0.1:3900", false},
		{"localhost", "127.0.0.1:3900", "http://127.0.0.1:3900", false},
		{"specific ipv4", "192.168.1.1:3900", "http://192.168.1.1:3900", false},
		{"ipv6 localhost", "[::1]:3900", "http://[::1]:3900", false},
		{"specific ipv6", "[2001:db8::1]:3900", "http://[2001:db8::1]:3900", false},
		{"empty host", ":3900", "http://127.0.0.1:3900", false},
		{"empty string", "", "", true},
		{"no port", "127.0.0.1", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := convertBindAddr(tc.bindAddr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("convertBindAddr(%q) = %q, want %q", tc.bindAddr, got, tc.want)
			}
		})
	}
}
