package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	stateStore *StateStore
	mu         sync.RWMutex
}

type StateStore struct {
	mu     sync.RWMutex
	states map[string]StateData
}

type StateData struct {
	Created   time.Time
	ExpiresAt time.Time
}

type SessionClaims struct {
	Username   string   `json:"username"`
	Email      string   `json:"email"`
	Name       string   `json:"name"`
	Roles      []string `json:"roles"`
	Teams      []string `json:"teams,omitempty"`
	AuthMethod string   `json:"auth_method,omitempty"`
	jwt.RegisteredClaims
}

func NewJWTService() (*JWTService, error) {
	return NewJWTServiceWithKey("")
}

func NewJWTServiceWithKey(privateKeyPEM string) (*JWTService, error) {
	var privateKey ed25519.PrivateKey
	var publicKey ed25519.PublicKey
	var err error

	if privateKeyPEM != "" {
		// Parse the provided PEM-encoded private key
		privateKey, err = parseEd25519PrivateKeyFromPEM(privateKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Ed25519 private key: %w", err)
		}
		publicKey = privateKey.Public().(ed25519.PublicKey)
	} else {
		// Generate a new Ed25519 key pair if no key is provided
		publicKey, privateKey, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate Ed25519 key: %w", err)
		}
	}

	return &JWTService{
		privateKey: privateKey,
		publicKey:  publicKey,
		stateStore: &StateStore{
			states: make(map[string]StateData),
		},
	}, nil
}

func parseEd25519PrivateKeyFromPEM(privateKeyPEM string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try to parse as PKCS#8 format (standard format from openssl genpkey -algorithm ED25519)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		// Successfully parsed as PKCS#8, check if it's an Ed25519 key
		if ed25519Key, ok := key.(ed25519.PrivateKey); ok {
			return ed25519Key, nil
		}
		return nil, fmt.Errorf("PKCS#8 key is not an Ed25519 key")
	}

	// Fallback: Check if it's raw Ed25519 private key bytes (64 bytes)
	if len(block.Bytes) == ed25519.PrivateKeySize {
		return block.Bytes, nil
	}

	return nil, fmt.Errorf("invalid Ed25519 private key format: not PKCS#8 and not raw %d bytes (got %d bytes)",
		ed25519.PrivateKeySize, len(block.Bytes))
}

func (j *JWTService) GenerateStateToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate state token: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(tokenBytes)

	j.stateStore.mu.Lock()
	defer j.stateStore.mu.Unlock()

	now := time.Now()
	j.stateStore.states[token] = StateData{
		Created:   now,
		ExpiresAt: now.Add(10 * time.Minute),
	}

	go j.cleanupExpiredStates()

	return token, nil
}

func (j *JWTService) ValidateAndConsumeState(token string) bool {
	j.stateStore.mu.Lock()
	defer j.stateStore.mu.Unlock()

	state, exists := j.stateStore.states[token]
	if !exists {
		return false
	}

	if time.Now().After(state.ExpiresAt) {
		delete(j.stateStore.states, token)
		return false
	}

	delete(j.stateStore.states, token)
	return true
}

func (j *JWTService) cleanupExpiredStates() {
	j.stateStore.mu.Lock()
	defer j.stateStore.mu.Unlock()

	now := time.Now()
	for token, state := range j.stateStore.states {
		if now.After(state.ExpiresAt) {
			delete(j.stateStore.states, token)
		}
	}
}

func (j *JWTService) GenerateToken(userInfo *UserInfo, sessionMaxAge int) (string, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if j.privateKey == nil {
		return "", fmt.Errorf("private key not initialized")
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(sessionMaxAge) * time.Second)

	claims := SessionClaims{
		Username:   userInfo.Username,
		Email:      userInfo.Email,
		Name:       userInfo.Name,
		Roles:      userInfo.Roles,
		Teams:      userInfo.Teams,
		AuthMethod: userInfo.AuthMethod,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tokenString, err := token.SignedString(j.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func (j *JWTService) ValidateToken(tokenString string) (*SessionClaims, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if j.publicKey == nil {
		return nil, fmt.Errorf("public key not initialized")
	}

	token, err := jwt.ParseWithClaims(tokenString, &SessionClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*SessionClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func (j *JWTService) GetPublicKeyPEM() (string, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if j.publicKey == nil {
		return "", fmt.Errorf("public key not initialized")
	}

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: []byte(j.publicKey),
	})

	return string(pubKeyPEM), nil
}

// GetPublicKeyBase64 returns the base64url-encoded public key for JWKS
func (j *JWTService) GetPublicKeyBase64() (string, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if j.publicKey == nil {
		return "", fmt.Errorf("public key not initialized")
	}

	return base64.RawURLEncoding.EncodeToString(j.publicKey), nil
}
