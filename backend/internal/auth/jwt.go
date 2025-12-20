package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	stateStore *StateStore
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
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

func NewJWTService() (*JWTService, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	return &JWTService{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		stateStore: &StateStore{
			states: make(map[string]StateData),
		},
	}, nil
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
	now := time.Now()
	expiresAt := now.Add(time.Duration(sessionMaxAge) * time.Second)

	claims := SessionClaims{
		Username: userInfo.Username,
		Email:    userInfo.Email,
		Name:     userInfo.Name,
		Roles:    userInfo.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(j.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func (j *JWTService) ValidateToken(tokenString string) (*SessionClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &SessionClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
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
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(j.publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return string(pubKeyPEM), nil
}
