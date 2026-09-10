package routes

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// testIssuer is a test-only fake OIDC provider. Fields with Fn suffix are
// per-request hooks a test can swap; defaults implement a happy path.
type testIssuer struct {
	Server   *httptest.Server
	ClientID string
	Key      *rsa.PrivateKey
	KeyID    string

	mu sync.Mutex

	// TokenEndpointFn, if set, overrides the default /token handler.
	TokenEndpointFn func(w http.ResponseWriter, r *http.Request)
	// UserInfoFn, if set, overrides the default /userinfo handler.
	UserInfoFn func(w http.ResponseWriter, r *http.Request)

	// Default token response state — used by the default TokenEndpointFn.
	// Tests mutate these between requests rather than overriding the handler.
	DefaultAccessClaims map[string]any
	DefaultIDClaims     map[string]any
	// When true the default /token handler omits id_token from the response.
	OmitIDToken bool
	// When non-empty, /token returns the specified HTTP status with a JSON
	// error payload — simulating code-exchange failures.
	TokenError string
	// When true the default /token handler returns an id_token signed with a
	// rogue key, forcing ID-token verification to fail.
	SignIDTokenWithWrongKey bool
}

// newTestIssuer spins up the fake issuer and registers cleanup.
func newTestIssuer(t *testing.T) *testIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	iss := &testIssuer{
		ClientID: "test-client",
		Key:      key,
		KeyID:    "test-key-1",
	}

	mux := http.NewServeMux()
	var srv *httptest.Server

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]any{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/authorize",
			"token_endpoint":                        srv.URL + "/token",
			"userinfo_endpoint":                     srv.URL + "/userinfo",
			"jwks_uri":                              srv.URL + "/jwks",
			"end_session_endpoint":                  srv.URL + "/logout",
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(iss.jwks())
	})

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		// Tests exercise /auth/oidc/login which only reads the redirect URL;
		// no need to implement the full authorize flow here.
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if iss.TokenEndpointFn != nil {
			iss.TokenEndpointFn(w, r)
			return
		}
		iss.defaultTokenHandler(w, r)
	})

	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if iss.UserInfoFn != nil {
			iss.UserInfoFn(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sub":                "user-1",
			"preferred_username": "alice",
			"email":              "alice@example.com",
		})
	})

	srv = httptest.NewServer(mux)
	iss.Server = srv
	t.Cleanup(srv.Close)

	// Happy-path defaults; tests override fields before exercising callback.
	iss.DefaultIDClaims = map[string]any{
		"iss":                srv.URL,
		"sub":                "user-1",
		"aud":                iss.ClientID,
		"exp":                time.Now().Add(10 * time.Minute).Unix(),
		"iat":                time.Now().Unix(),
		"preferred_username": "alice",
		"email":              "alice@example.com",
	}
	iss.DefaultAccessClaims = map[string]any{
		"iss": srv.URL,
		"sub": "user-1",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
	}
	return iss
}

func (iss *testIssuer) defaultTokenHandler(w http.ResponseWriter, r *http.Request) {
	iss.mu.Lock()
	defer iss.mu.Unlock()

	if iss.TokenError != "" {
		http.Error(w, iss.TokenError, http.StatusBadRequest)
		return
	}

	access := iss.signJWT(iss.DefaultAccessClaims, iss.Key)
	resp := map[string]any{
		"access_token": access,
		"token_type":   "Bearer",
		"expires_in":   600,
	}
	if !iss.OmitIDToken {
		key := iss.Key
		if iss.SignIDTokenWithWrongKey {
			rogue, _ := rsa.GenerateKey(rand.Reader, 2048)
			key = rogue
		}
		resp["id_token"] = iss.signJWT(iss.DefaultIDClaims, key)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// signJWT signs the given claims with the given RSA key using RS256.
func (iss *testIssuer) signJWT(claims map[string]any, key *rsa.PrivateKey) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims))
	tok.Header["kid"] = iss.KeyID
	signed, err := tok.SignedString(key)
	if err != nil {
		panic(fmt.Sprintf("signJWT: %v", err))
	}
	return signed
}

// jwks returns a JWKS document exposing the issuer's RSA public key.
func (iss *testIssuer) jwks() map[string]any {
	pub := iss.Key.PublicKey
	return map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"kid": iss.KeyID,
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			},
		},
	}
}

// IDToken signs an ID token with the issuer key. Tests use this to mint
// access tokens carrying nested role claims for fallback-path assertions.
func (iss *testIssuer) IDToken(claims map[string]any) string {
	return iss.signJWT(claims, iss.Key)
}

// AccessToken signs an access token carrying Keycloak-shaped resource_access
// roles at resource_access.test-client.roles.
func (iss *testIssuer) AccessToken(roles []string) string {
	rolesAny := make([]any, 0, len(roles))
	for _, r := range roles {
		rolesAny = append(rolesAny, r)
	}
	return iss.signJWT(map[string]any{
		"iss": iss.Server.URL,
		"sub": "user-1",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
		"resource_access": map[string]any{
			"test-client": map[string]any{"roles": rolesAny},
		},
	}, iss.Key)
}
