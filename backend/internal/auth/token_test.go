package auth

import (
	"crypto/subtle"
	"testing"
)

func TestValidateAdminToken(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		provided   string
		want       bool
	}{
		{"correct token", "my-secret-token", "my-secret-token", true},
		{"wrong token", "my-secret-token", "wrong-token", false},
		{"empty provided", "my-secret-token", "", false},
		{"empty configured", "", "any-token", false},
		{"both empty", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := subtle.ConstantTimeCompare([]byte(tc.configured), []byte(tc.provided)) == 1
			if got != tc.want {
				t.Errorf("ValidateAdminToken(%q, %q) = %v, want %v", tc.configured, tc.provided, got, tc.want)
			}
		})
	}
}
