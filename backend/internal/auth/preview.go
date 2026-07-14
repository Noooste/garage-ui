package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PreviewTokenLocalsKey is the fiber Locals key the auth middleware sets
// after validating a preview token. The authz middleware reads it to
// authorize the request without a subject.
const PreviewTokenLocalsKey = "previewTokenClaims"

// PreviewClaims identify the single object a preview token can read.
type PreviewClaims struct {
	Bucket    string `json:"b"`
	Key       string `json:"k"`
	ExpiresAt int64  `json:"exp"`
}

// previewSecret derives the HMAC key from the JWT signing key. A configured
// session key therefore keeps preview URLs valid across restarts, and a
// generated key invalidates them on restart, which the frontend recovers
// from by minting a fresh URL.
func (j *JWTService) previewSecret() []byte {
	j.mu.RLock()
	defer j.mu.RUnlock()
	sum := sha256.Sum256(append([]byte("garage-ui-preview-token:"), j.privateKey...))
	return sum[:]
}

// MintPreviewToken signs a token granting read access to one exact object
// until the TTL elapses.
func (a *Service) MintPreviewToken(bucket, key string, ttl time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(ttl)
	token, err := mintPreviewToken(a.jwtService.previewSecret(), bucket, key, expiresAt)
	return token, expiresAt, err
}

// ValidatePreviewToken checks the signature, expiry, and exact object match.
func (a *Service) ValidatePreviewToken(token, bucket, key string) error {
	return verifyPreviewToken(a.jwtService.previewSecret(), token, bucket, key, time.Now())
}

func mintPreviewToken(secret []byte, bucket, key string, expiresAt time.Time) (string, error) {
	payload, err := json.Marshal(PreviewClaims{Bucket: bucket, Key: key, ExpiresAt: expiresAt.Unix()})
	// Defensive and unreachable: marshaling a struct of two strings and an int64
	// cannot fail, so this branch stays uncovered by design.
	if err != nil {
		return "", fmt.Errorf("failed to encode preview claims: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + signPreview(secret, encoded), nil
}

func verifyPreviewToken(secret []byte, token, bucket, key string, now time.Time) error {
	encoded, sig, ok := strings.Cut(token, ".")
	if !ok {
		return fmt.Errorf("malformed preview token")
	}
	if !hmac.Equal([]byte(sig), []byte(signPreview(secret, encoded))) {
		return fmt.Errorf("invalid preview token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("malformed preview token payload")
	}
	var claims PreviewClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return fmt.Errorf("malformed preview token claims")
	}
	if now.Unix() > claims.ExpiresAt {
		return fmt.Errorf("preview token expired")
	}
	if claims.Bucket != bucket || claims.Key != key {
		return fmt.Errorf("preview token does not match the requested object")
	}
	return nil
}

func signPreview(secret []byte, encoded string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(encoded))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
