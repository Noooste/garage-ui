package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// generatePKCS8PEM produces a PEM-encoded PKCS#8 Ed25519 private key.
// This is the format `openssl genpkey -algorithm ED25519` emits and the
// format the production code documents in jwt_private_key.
func generatePKCS8PEM(t *testing.T) (string, ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return string(pemBytes), priv
}

// generateRawPEM wraps a raw 64-byte Ed25519 key in a PEM block. The
// production code accepts this as a fallback when PKCS#8 parsing fails.
func generateRawPEM(t *testing.T) (string, ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: priv})
	return string(pemBytes), priv
}

func TestParseEd25519PrivateKeyFromPEM_PKCS8(t *testing.T) {
	pemStr, want := generatePKCS8PEM(t)
	got, err := parseEd25519PrivateKeyFromPEM(pemStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("parsed key does not equal generated key")
	}
}

func TestParseEd25519PrivateKeyFromPEM_RawBytes(t *testing.T) {
	pemStr, want := generateRawPEM(t)
	got, err := parseEd25519PrivateKeyFromPEM(pemStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("parsed raw key does not equal generated key")
	}
}

func TestParseEd25519PrivateKeyFromPEM_NotPEM(t *testing.T) {
	_, err := parseEd25519PrivateKeyFromPEM("this is not a pem block")
	if err == nil {
		t.Fatal("expected error for non-PEM input, got nil")
	}
	if !strings.Contains(err.Error(), "decode PEM block") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseEd25519PrivateKeyFromPEM_PKCS8WrongKeyType(t *testing.T) {
	// Generate a non-Ed25519 PKCS#8 key (RSA would require crypto/rsa; instead
	// we craft a PKCS#8 wrapping for an ECDSA key via x509). The simplest
	// portable way is to use a known-bad DER blob: a PKCS#8 wrapping of an
	// ed25519 PUBLIC key, which ParsePKCS8PrivateKey will reject as not a
	// private key. To keep the test deterministic and dependency-free, we
	// instead build a PEM of length-mismatched bytes that's neither PKCS#8
	// nor 64 raw bytes.
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: []byte("definitely not a valid pkcs8 or raw ed25519 key"),
	})
	_, err := parseEd25519PrivateKeyFromPEM(string(pemBytes))
	if err == nil {
		t.Fatal("expected error for invalid key bytes, got nil")
	}
	if !strings.Contains(err.Error(), "invalid Ed25519 private key format") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewJWTService_AutoGeneratesKeyPair(t *testing.T) {
	svc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	if svc.privateKey == nil {
		t.Error("privateKey is nil after auto-generate")
	}
	if svc.publicKey == nil {
		t.Error("publicKey is nil after auto-generate")
	}
	if len(svc.privateKey) != ed25519.PrivateKeySize {
		t.Errorf("privateKey size = %d, want %d", len(svc.privateKey), ed25519.PrivateKeySize)
	}
	if len(svc.publicKey) != ed25519.PublicKeySize {
		t.Errorf("publicKey size = %d, want %d", len(svc.publicKey), ed25519.PublicKeySize)
	}
	if svc.stateStore == nil || svc.stateStore.states == nil {
		t.Error("stateStore not initialized")
	}
}

// The key that shipped in docker-compose.yml (issue #132) is public, so a
// token an attacker signs with it must not validate.
func TestNewJWTServiceWithKey_RejectsPublishedComposeKey(t *testing.T) {
	const published = "-----BEGIN PRIVATE KEY-----\nMC4CAQAwBQYDK2VwBCIEIH0ZHqIV7MEyVsxNYc5TA/a0qaBxgq3ntlFS3w1F03MS\n-----END PRIVATE KEY-----\n"
	svc, err := NewJWTServiceWithKey(published)
	if err != nil {
		t.Fatalf("NewJWTServiceWithKey: %v", err)
	}
	attackerKey, err := parseEd25519PrivateKeyFromPEM(published)
	if err != nil {
		t.Fatalf("parse published key: %v", err)
	}
	forged, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, SessionClaims{
		Username:   "forged-admin",
		AuthMethod: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(attackerKey)
	if err != nil {
		t.Fatalf("sign forged token: %v", err)
	}
	if _, err := svc.ValidateToken(forged); err == nil {
		t.Fatal("token signed with the published compose key was accepted")
	}
}

func TestLoadOrCreateKeyFile_GeneratesOnceThenReuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "jwt-key.pem")

	first, err := loadOrCreateKeyFile(path)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := parseEd25519PrivateKeyFromPEM(first); err != nil {
		t.Fatalf("generated key does not parse: %v", err)
	}
	second, err := loadOrCreateKeyFile(path)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if first != second {
		t.Fatal("second call generated a new key instead of reusing the saved one")
	}
}

func TestNewJWTServiceWithKey_EmptyStringAutoGenerates(t *testing.T) {
	svc, err := NewJWTServiceWithKey("")
	if err != nil {
		t.Fatalf("NewJWTServiceWithKey(\"\"): %v", err)
	}
	if svc.privateKey == nil || svc.publicKey == nil {
		t.Error("expected auto-generated keys for empty PEM input")
	}
}

func TestNewJWTServiceWithKey_PKCS8(t *testing.T) {
	pemStr, want := generatePKCS8PEM(t)
	svc, err := NewJWTServiceWithKey(pemStr)
	if err != nil {
		t.Fatalf("NewJWTServiceWithKey: %v", err)
	}
	if !svc.privateKey.Equal(want) {
		t.Error("loaded privateKey does not match input")
	}
	// Public key must match the public part of the loaded private key.
	wantPub := want.Public().(ed25519.PublicKey)
	if !svc.publicKey.Equal(wantPub) {
		t.Error("derived publicKey does not match")
	}
}

func TestNewJWTServiceWithKey_BadPEMReturnsWrappedError(t *testing.T) {
	_, err := NewJWTServiceWithKey("garbage")
	if err == nil {
		t.Fatal("expected error for bad PEM, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse Ed25519 private key") {
		t.Errorf("expected wrapping error, got %v", err)
	}
}

func newTestUserInfo() *UserInfo {
	return &UserInfo{
		Username: "alice",
		Email:    "alice@example.com",
		Name:     "Alice Example",
		Roles:    []string{"admin", "viewer"},
	}
}

func TestGenerateAndValidateToken_RoundTrip(t *testing.T) {
	svc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}

	user := newTestUserInfo()
	tok, err := svc.GenerateToken(user, 60)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if tok == "" {
		t.Fatal("GenerateToken returned empty string")
	}

	claims, err := svc.ValidateToken(tok)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.Username != user.Username {
		t.Errorf("Username = %q, want %q", claims.Username, user.Username)
	}
	if claims.Email != user.Email {
		t.Errorf("Email = %q, want %q", claims.Email, user.Email)
	}
	if claims.Name != user.Name {
		t.Errorf("Name = %q, want %q", claims.Name, user.Name)
	}
	if len(claims.Roles) != 2 || claims.Roles[0] != "admin" || claims.Roles[1] != "viewer" {
		t.Errorf("Roles = %v, want [admin viewer]", claims.Roles)
	}
	// ExpiresAt should be ~60s in the future.
	if claims.ExpiresAt == nil {
		t.Fatal("ExpiresAt nil")
	}
	if d := time.Until(claims.ExpiresAt.Time); d <= 0 || d > 61*time.Second {
		t.Errorf("ExpiresAt delta = %v, want (0,61s]", d)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	svc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	// sessionMaxAge = -1s → token is born expired.
	tok, err := svc.GenerateToken(newTestUserInfo(), -1)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	_, err = svc.ValidateToken(tok)
	if err == nil {
		t.Fatal("expected expired-token error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse token") {
		t.Errorf("unexpected error: %v", err)
	}
	// jwt/v5 surfaces ErrTokenExpired wrapped in the parse error.
	if !errors.Is(err, jwt.ErrTokenExpired) {
		t.Errorf("expected wrapped jwt.ErrTokenExpired, got %v", err)
	}
}

func TestValidateToken_SignedByDifferentKey(t *testing.T) {
	signer, err := NewJWTService()
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	verifier, err := NewJWTService()
	if err != nil {
		t.Fatalf("verifier: %v", err)
	}
	tok, err := signer.GenerateToken(newTestUserInfo(), 60)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if _, err := verifier.ValidateToken(tok); err == nil {
		t.Fatal("expected signature-mismatch error, got nil")
	}
}

func TestValidateToken_Malformed(t *testing.T) {
	svc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	cases := []string{
		"",
		"not.a.jwt",
		"only-one-segment",
		"two.segments",
		"aaaa.bbbb.cccc", // valid shape, invalid base64/JSON
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			if _, err := svc.ValidateToken(c); err == nil {
				t.Errorf("expected error for %q, got nil", c)
			}
		})
	}
}

func TestValidateToken_WrongSigningMethod(t *testing.T) {
	svc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	// Forge an HS256 token with the same claim shape; ValidateToken's
	// keyfunc must reject the alg before signature verification.
	claims := SessionClaims{
		Username: "mallory",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte("a-shared-secret"))
	if err != nil {
		t.Fatalf("sign HS256: %v", err)
	}
	_, err = svc.ValidateToken(signed)
	if err == nil {
		t.Fatal("expected error for non-EdDSA token, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected signing method") {
		t.Errorf("expected signing-method error, got %v", err)
	}
}

func TestGenerateToken_NilPrivateKeyReturnsError(t *testing.T) {
	// Construct a service with a nil key directly. This guards the explicit
	// nil-check at the top of GenerateToken.
	svc := &JWTService{}
	_, err := svc.GenerateToken(newTestUserInfo(), 60)
	if err == nil {
		t.Fatal("expected error for nil private key, got nil")
	}
	if !strings.Contains(err.Error(), "private key not initialized") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateToken_NilPublicKeyReturnsError(t *testing.T) {
	svc := &JWTService{}
	_, err := svc.ValidateToken("anything")
	if err == nil {
		t.Fatal("expected error for nil public key, got nil")
	}
	if !strings.Contains(err.Error(), "public key not initialized") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerateStateToken_ProducesUniqueValues(t *testing.T) {
	svc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	a, err := svc.GenerateStateToken()
	if err != nil {
		t.Fatalf("GenerateStateToken: %v", err)
	}
	b, err := svc.GenerateStateToken()
	if err != nil {
		t.Fatalf("GenerateStateToken: %v", err)
	}
	if a == "" || b == "" {
		t.Fatal("state token is empty")
	}
	if a == b {
		t.Errorf("state tokens collided: %q", a)
	}
}

func TestValidateAndConsumeState_HappyPath(t *testing.T) {
	svc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	tok, err := svc.GenerateStateToken()
	if err != nil {
		t.Fatalf("GenerateStateToken: %v", err)
	}
	if !svc.ValidateAndConsumeState(tok) {
		t.Error("first consume should succeed")
	}
}

func TestValidateAndConsumeState_IsSingleUse(t *testing.T) {
	svc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	tok, err := svc.GenerateStateToken()
	if err != nil {
		t.Fatalf("GenerateStateToken: %v", err)
	}
	_ = svc.ValidateAndConsumeState(tok)
	if svc.ValidateAndConsumeState(tok) {
		t.Error("second consume should fail")
	}
}

func TestValidateAndConsumeState_UnknownTokenRejected(t *testing.T) {
	svc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	if svc.ValidateAndConsumeState("never-issued") {
		t.Error("unknown token must not validate")
	}
}

func TestValidateAndConsumeState_ExpiredTokenRejected(t *testing.T) {
	svc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	// Inject an expired entry directly to avoid a real 10-minute wait.
	svc.stateStore.states["expired"] = StateData{
		Created:   time.Now().Add(-20 * time.Minute),
		ExpiresAt: time.Now().Add(-10 * time.Minute),
	}
	if svc.ValidateAndConsumeState("expired") {
		t.Error("expired token must not validate")
	}
	// And it should be deleted as a side effect of the rejection.
	if _, exists := svc.stateStore.states["expired"]; exists {
		t.Error("expired token should be removed from the store")
	}
}

func TestGetPublicKeyPEM_ParsesBackToOriginalKey(t *testing.T) {
	svc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	pemStr, err := svc.GetPublicKeyPEM()
	if err != nil {
		t.Fatalf("GetPublicKeyPEM: %v", err)
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		t.Fatalf("returned PEM did not decode: %q", pemStr)
	}
	if block.Type != "PUBLIC KEY" {
		t.Errorf("PEM type = %q, want PUBLIC KEY", block.Type)
	}
	// The implementation writes the raw 32-byte public key as the block body.
	if len(block.Bytes) != ed25519.PublicKeySize {
		t.Errorf("body length = %d, want %d", len(block.Bytes), ed25519.PublicKeySize)
	}
	if !ed25519.PublicKey(block.Bytes).Equal(svc.publicKey) {
		t.Error("decoded public key does not match service key")
	}
}

func TestGetPublicKeyBase64_RoundTripsToOriginalKey(t *testing.T) {
	svc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	b64, err := svc.GetPublicKeyBase64()
	if err != nil {
		t.Fatalf("GetPublicKeyBase64: %v", err)
	}
	if b64 == "" {
		t.Fatal("empty base64 output")
	}
	// base64.RawURLEncoding (no padding) is what the production code uses.
	// Decode and compare.
	// Use the std encoding through helper to keep the import list small.
	got, err := decodeRawURL(b64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if !ed25519.PublicKey(got).Equal(svc.publicKey) {
		t.Error("base64-decoded key does not match service key")
	}
}

func TestGetPublicKeyPEM_NilKeyReturnsError(t *testing.T) {
	svc := &JWTService{}
	if _, err := svc.GetPublicKeyPEM(); err == nil {
		t.Error("expected error for nil public key")
	}
}

func TestGetPublicKeyBase64_NilKeyReturnsError(t *testing.T) {
	svc := &JWTService{}
	if _, err := svc.GetPublicKeyBase64(); err == nil {
		t.Error("expected error for nil public key")
	}
}

// decodeRawURL is a tiny shim around encoding/base64's RawURLEncoding decoder
// so the test body stays focused on assertions, not encoding plumbing.
func decodeRawURL(s string) ([]byte, error) {
	return base64RawURLDecode(s)
}

func base64RawURLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
