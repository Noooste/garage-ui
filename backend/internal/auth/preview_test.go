package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func newPreviewTestService(t *testing.T) *Service {
	t.Helper()
	jwtSvc, err := NewJWTService()
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	return &Service{jwtService: jwtSvc}
}

func TestPreviewToken_RoundTrip(t *testing.T) {
	svc := newPreviewTestService(t)
	token, expiresAt, err := svc.MintPreviewToken("b1", "dir/clip.mp4", time.Hour)
	if err != nil {
		t.Fatalf("MintPreviewToken: %v", err)
	}
	if remaining := time.Until(expiresAt); remaining < 59*time.Minute || remaining > time.Hour {
		t.Errorf("expiresAt %v is not about an hour away", expiresAt)
	}
	if err := svc.ValidatePreviewToken(token, "b1", "dir/clip.mp4"); err != nil {
		t.Errorf("ValidatePreviewToken: %v", err)
	}
}

func TestPreviewToken_Expired(t *testing.T) {
	svc := newPreviewTestService(t)
	token, _, err := svc.MintPreviewToken("b1", "k", -time.Minute)
	if err != nil {
		t.Fatalf("MintPreviewToken: %v", err)
	}
	if err := svc.ValidatePreviewToken(token, "b1", "k"); err == nil {
		t.Error("expected expired token to be rejected")
	}
}

func TestPreviewToken_WrongObject(t *testing.T) {
	svc := newPreviewTestService(t)
	token, _, err := svc.MintPreviewToken("b1", "k1", time.Hour)
	if err != nil {
		t.Fatalf("MintPreviewToken: %v", err)
	}
	if err := svc.ValidatePreviewToken(token, "b2", "k1"); err == nil {
		t.Error("expected wrong bucket to be rejected")
	}
	if err := svc.ValidatePreviewToken(token, "b1", "k2"); err == nil {
		t.Error("expected wrong key to be rejected")
	}
}

func TestPreviewToken_Tampered(t *testing.T) {
	svc := newPreviewTestService(t)
	token, _, err := svc.MintPreviewToken("b1", "k", time.Hour)
	if err != nil {
		t.Fatalf("MintPreviewToken: %v", err)
	}
	payload, sig, _ := strings.Cut(token, ".")
	flipped := "A" + payload[1:]
	if flipped == payload {
		flipped = "B" + payload[1:]
	}
	if err := svc.ValidatePreviewToken(flipped+"."+sig, "b1", "k"); err == nil {
		t.Error("expected tampered payload to be rejected")
	}
	if err := svc.ValidatePreviewToken(payload+".AAAA", "b1", "k"); err == nil {
		t.Error("expected tampered signature to be rejected")
	}
}

func TestPreviewToken_Malformed(t *testing.T) {
	svc := newPreviewTestService(t)
	for _, tok := range []string{"", "nodot", "a.b.c", "!!!.???", "bm90anNvbg.sig"} {
		if err := svc.ValidatePreviewToken(tok, "b1", "k"); err == nil {
			t.Errorf("expected malformed token %q to be rejected", tok)
		}
	}
}

func TestPreviewToken_DifferentServicesRejectEachOther(t *testing.T) {
	a := newPreviewTestService(t)
	b := newPreviewTestService(t)
	token, _, err := a.MintPreviewToken("b1", "k", time.Hour)
	if err != nil {
		t.Fatalf("MintPreviewToken: %v", err)
	}
	if err := b.ValidatePreviewToken(token, "b1", "k"); err == nil {
		t.Error("expected a token from another key to be rejected")
	}
}

// TestPreviewToken_ValidSignatureMalformedBase64Payload signs a payload that
// is not valid RawURLEncoding, so the signature check passes but the base64
// decode fails. This exercises the decode error branch in verifyPreviewToken.
func TestPreviewToken_ValidSignatureMalformedBase64Payload(t *testing.T) {
	svc := newPreviewTestService(t)
	secret := svc.jwtService.previewSecret()
	enc := "!!not-base64!!"
	token := enc + "." + signPreview(secret, enc)
	if err := svc.ValidatePreviewToken(token, "b1", "k"); err == nil {
		t.Error("expected a validly signed but non-base64 payload to be rejected")
	}
}

// TestPreviewToken_ValidSignatureNonJSONPayload signs a valid base64 payload
// whose bytes are not JSON, so the signature and decode both pass but the
// unmarshal fails. This exercises the JSON error branch in verifyPreviewToken.
func TestPreviewToken_ValidSignatureNonJSONPayload(t *testing.T) {
	svc := newPreviewTestService(t)
	secret := svc.jwtService.previewSecret()
	enc := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	token := enc + "." + signPreview(secret, enc)
	if err := svc.ValidatePreviewToken(token, "b1", "k"); err == nil {
		t.Error("expected a validly signed but non-JSON payload to be rejected")
	}
}
