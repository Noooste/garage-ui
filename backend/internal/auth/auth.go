package auth

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"Noooste/garage-ui/internal/config"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// AuthService handles authentication operations
type AuthService struct {
	config       *config.AuthConfig
	oidcProvider *oidc.Provider
	oidcVerifier *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	jwtService   *JWTService
}

// UserInfo represents authenticated user information
type UserInfo struct {
	Username string
	Email    string
	Name     string
	Roles    []string
}

// NewAuthService creates a new authentication service
func NewAuthService(cfg *config.AuthConfig) (*AuthService, error) {
	jwtService, err := NewJWTService()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize JWT service: %w", err)
	}

	service := &AuthService{
		config:     cfg,
		jwtService: jwtService,
	}

	// Initialize OIDC if enabled
	if cfg.Mode == "oidc" && cfg.OIDC.Enabled {
		if err := service.initOIDC(); err != nil {
			return nil, fmt.Errorf("failed to initialize OIDC: %w", err)
		}
	}

	return service, nil
}

// initOIDC initializes the OIDC provider and configuration
func (a *AuthService) initOIDC() error {
	ctx := context.Background()

	// Create OIDC provider
	provider, err := oidc.NewProvider(ctx, a.config.OIDC.IssuerURL)
	if err != nil {
		return fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	a.oidcProvider = provider

	// Create ID token verifier
	verifierConfig := &oidc.Config{
		ClientID:        a.config.OIDC.ClientID,
		SkipIssuerCheck: a.config.OIDC.SkipIssuerCheck,
		SkipExpiryCheck: a.config.OIDC.SkipExpiryCheck,
	}
	a.oidcVerifier = provider.Verifier(verifierConfig)

	// Create OAuth2 config
	a.oauth2Config = &oauth2.Config{
		ClientID:     a.config.OIDC.ClientID,
		ClientSecret: a.config.OIDC.ClientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       a.config.OIDC.Scopes,
	}

	return nil
}

// ValidateBasicAuth validates basic authentication credentials
func (a *AuthService) ValidateBasicAuth(username, password string) bool {
	// Use constant-time comparison to prevent timing attacks
	usernameMatch := subtle.ConstantTimeCompare(
		[]byte(username),
		[]byte(a.config.Basic.Username),
	) == 1

	passwordMatch := subtle.ConstantTimeCompare(
		[]byte(password),
		[]byte(a.config.Basic.Password),
	) == 1

	return usernameMatch && passwordMatch
}

// ParseBasicAuth parses the Authorization header for basic auth
func ParseBasicAuth(authHeader string) (username, password string, ok bool) {
	if authHeader == "" {
		return "", "", false
	}

	// Check if it's a Basic auth header
	const prefix = "Basic "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", "", false
	}

	// Decode base64 credentials
	encoded := authHeader[len(prefix):]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", false
	}

	// Split username:password
	credentials := string(decoded)
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// GetAuthorizationURL returns the OIDC authorization URL for login
func (a *AuthService) GetAuthorizationURL(state string) (string, error) {
	if a.oauth2Config == nil {
		return "", fmt.Errorf("OIDC not initialized")
	}

	return a.oauth2Config.AuthCodeURL(state), nil
}

// ExchangeCode exchanges an authorization code for tokens
func (a *AuthService) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	if a.oauth2Config == nil {
		return nil, fmt.Errorf("OIDC not initialized")
	}

	token, err := a.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	return token, nil
}

// VerifyIDToken verifies an OIDC ID token and extracts user info
func (a *AuthService) VerifyIDToken(ctx context.Context, rawIDToken string) (*UserInfo, error) {
	if a.oidcVerifier == nil {
		return nil, fmt.Errorf("OIDC not initialized")
	}

	// Verify the ID token
	idToken, err := a.oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract claims
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	// Extract user information using configured attributes
	userInfo := &UserInfo{
		Username: extractClaim(claims, a.config.OIDC.UsernameAttribute),
		Email:    extractClaim(claims, a.config.OIDC.EmailAttribute),
		Name:     extractClaim(claims, a.config.OIDC.NameAttribute),
	}

	// Extract roles if configured
	if a.config.OIDC.RoleAttributePath != "" {
		userInfo.Roles = extractRoles(claims, a.config.OIDC.RoleAttributePath)
	}

	return userInfo, nil
}

// GetUserInfo retrieves user information from the OIDC provider
func (a *AuthService) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	if a.oidcProvider == nil {
		return nil, fmt.Errorf("OIDC not initialized")
	}

	// Create OAuth2 token source
	tokenSource := a.oauth2Config.TokenSource(ctx, token)

	// Get user info from the provider
	userInfoEndpoint, err := a.oidcProvider.UserInfo(ctx, tokenSource)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	// Extract claims
	var claims map[string]interface{}
	if err := userInfoEndpoint.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse user info claims: %w", err)
	}

	// Build user info
	userInfo := &UserInfo{
		Username: extractClaim(claims, a.config.OIDC.UsernameAttribute),
		Email:    extractClaim(claims, a.config.OIDC.EmailAttribute),
		Name:     extractClaim(claims, a.config.OIDC.NameAttribute),
	}

	// Extract roles if configured
	if a.config.OIDC.RoleAttributePath != "" {
		userInfo.Roles = extractRoles(claims, a.config.OIDC.RoleAttributePath)
	}

	return userInfo, nil
}

// IsAdmin checks if the user has admin role
func (a *AuthService) IsAdmin(userInfo *UserInfo) bool {
	if a.config.OIDC.AdminRole == "" {
		return false
	}

	for _, role := range userInfo.Roles {
		if role == a.config.OIDC.AdminRole {
			return true
		}
	}

	return false
}

// Helper functions

// extractClaim extracts a string claim from the claims map
func extractClaim(claims map[string]interface{}, key string) string {
	if key == "" {
		return ""
	}

	value, ok := claims[key]
	if !ok {
		return ""
	}

	str, ok := value.(string)
	if !ok {
		return ""
	}

	return str
}

// extractRoles extracts roles from nested claim path (e.g., "resource_access.garage-ui.roles")
func extractRoles(claims map[string]interface{}, path string) []string {
	if path == "" {
		return nil
	}

	// Split the path by dots to navigate nested claims
	parts := strings.Split(path, ".")

	current := claims
	for i, part := range parts {
		value, ok := current[part]
		if !ok {
			return nil
		}

		if i == len(parts)-1 {
			return extractStringArray(value)
		}

		// Navigate to next level
		next, ok := value.(map[string]interface{})
		if !ok {
			return nil
		}
		current = next
	}

	return nil
}

// extractStringArray converts an interface{} to []string if possible
func extractStringArray(value interface{}) []string {
	// Try direct string array
	if strArray, ok := value.([]string); ok {
		return strArray
	}

	// Try interface array and convert to strings
	if array, ok := value.([]interface{}); ok {
		result := make([]string, 0, len(array))
		for _, item := range array {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}

	return nil
}

// GenerateStateToken generates a secure CSRF state token
func (a *AuthService) GenerateStateToken() (string, error) {
	return a.jwtService.GenerateStateToken()
}

// ValidateAndConsumeState validates and consumes a CSRF state token
func (a *AuthService) ValidateAndConsumeState(token string) bool {
	return a.jwtService.ValidateAndConsumeState(token)
}

// GenerateSessionToken generates a JWT session token for the user
func (a *AuthService) GenerateSessionToken(userInfo *UserInfo) (string, error) {
	return a.jwtService.GenerateToken(userInfo, a.config.OIDC.SessionMaxAge)
}

// ValidateSessionToken validates a JWT session token and returns user info
func (a *AuthService) ValidateSessionToken(tokenString string) (*UserInfo, error) {
	claims, err := a.jwtService.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	return &UserInfo{
		Username: claims.Username,
		Email:    claims.Email,
		Name:     claims.Name,
		Roles:    claims.Roles,
	}, nil
}
