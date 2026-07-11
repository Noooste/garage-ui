package auth

import (
	"testing"

	"Noooste/garage-ui/internal/config"
)

func TestSessionTokenRoundTripsTeamsAndMethod(t *testing.T) {
	svc, err := NewAuthService(&config.AuthConfig{}, &config.ServerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	in := &UserInfo{
		Username:   "alice",
		Email:      "alice@example.com",
		Teams:      []string{"garage-team-backend", "garage-team-data"},
		AuthMethod: "oidc",
	}
	token, err := svc.GenerateSessionToken(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.ValidateSessionToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Teams) != 2 || out.Teams[0] != "garage-team-backend" {
		t.Errorf("Teams = %v, want round-trip", out.Teams)
	}
	if out.AuthMethod != "oidc" {
		t.Errorf("AuthMethod = %q, want oidc", out.AuthMethod)
	}
}

func TestExtractTeamsFromAccessToken(t *testing.T) {
	svc, err := NewAuthService(&config.AuthConfig{
		OIDC: config.OIDCConfig{TeamAttributePath: "groups"},
	}, &config.ServerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	// Unsigned JWT with {"groups":["team-a","team-b"]} payload. Extraction
	// parses claims without verifying (token came from a verified exchange).
	// header {"alg":"none"} / payload base64url of {"groups":["team-a","team-b"]}
	tok := "eyJhbGciOiJub25lIn0.eyJncm91cHMiOlsidGVhbS1hIiwidGVhbS1iIl19.x"
	got := svc.ExtractTeamsFromAccessToken(tok)
	if len(got) != 2 || got[0] != "team-a" || got[1] != "team-b" {
		t.Errorf("ExtractTeamsFromAccessToken = %v, want [team-a team-b]", got)
	}
	if got := svc.ExtractTeamsFromAccessToken(""); got != nil {
		t.Errorf("empty token should return nil, got %v", got)
	}
}

func TestExtractTeamsFromAccessToken_Malformed(t *testing.T) {
	svc, err := NewAuthService(&config.AuthConfig{
		OIDC: config.OIDCConfig{TeamAttributePath: "groups"},
	}, &config.ServerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	// Fewer than two dot-separated segments.
	if got := svc.ExtractTeamsFromAccessToken("single-segment"); got != nil {
		t.Errorf("one-segment token = %v, want nil", got)
	}
	// Correct shape but the payload segment is not valid base64url.
	if got := svc.ExtractTeamsFromAccessToken("hdr.!!!not-base64!!!.sig"); got != nil {
		t.Errorf("bad base64 payload = %v, want nil", got)
	}
	// Valid base64url ("bm90anNvbg" -> "notjson") but not JSON.
	if got := svc.ExtractTeamsFromAccessToken("hdr.bm90anNvbg.sig"); got != nil {
		t.Errorf("non-JSON payload = %v, want nil", got)
	}
}
