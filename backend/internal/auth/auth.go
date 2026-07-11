package auth

import (
	"Noooste/garage-ui/pkg/logger"
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"Noooste/garage-ui/internal/config"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Service handles authentication operations
type Service struct {
	authConfig   *config.AuthConfig
	serverConfig *config.ServerConfig
	oidcProvider *oidc.Provider
	oidcVerifier *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	oidcClient   *http.Client
	jwtService   *JWTService
}

// UserInfo represents authenticated user information
type UserInfo struct {
	Username   string
	Email      string
	Name       string
	Roles      []string
	Teams      []string // raw team claim values (team_attribute_path), OIDC only
	AuthMethod string   // "oidc" | "admin" | "token"; "" on legacy sessions
}

// NewAuthService creates a new authentication service
func NewAuthService(authCfg *config.AuthConfig, serverCfg *config.ServerConfig) (*Service, error) {
	jwtService, err := NewJWTServiceWithKey(authCfg.JWTPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize JWT service: %w", err)
	}

	service := &Service{
		authConfig:   authCfg,
		serverConfig: serverCfg,
		jwtService:   jwtService,
	}

	// Initialize OIDC if enabled
	if authCfg.OIDC.Enabled {
		if err := service.initOIDC(); err != nil {
			return nil, fmt.Errorf("failed to initialize OIDC: %w", err)
		}
	}

	return service, nil
}

// initOIDC initializes the OIDC provider and configuration
func (a *Service) initOIDC() error {
	if a.authConfig.OIDC.TLSSkipVerify {
		a.oidcClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}

	ctx := a.oidcContext(context.Background())

	// Create OIDC provider
	provider, err := oidc.NewProvider(ctx, a.authConfig.OIDC.IssuerURL)
	if err != nil {
		return fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	a.oidcProvider = provider

	// Create ID token verifier
	verifierConfig := &oidc.Config{
		ClientID:        a.authConfig.OIDC.ClientID,
		SkipIssuerCheck: a.authConfig.OIDC.SkipIssuerCheck,
		SkipExpiryCheck: a.authConfig.OIDC.SkipExpiryCheck,
	}
	a.oidcVerifier = provider.Verifier(verifierConfig)

	// Construct redirect URL from server config
	// Use root_url if set, otherwise construct from protocol/domain
	redirectURL := a.serverConfig.RootURL + "/auth/oidc/callback"

	// Create OAuth2 config
	a.oauth2Config = &oauth2.Config{
		ClientID:     a.authConfig.OIDC.ClientID,
		ClientSecret: a.authConfig.OIDC.ClientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       a.authConfig.OIDC.Scopes,
	}

	return nil
}

func (a *Service) oidcContext(ctx context.Context) context.Context {
	if a.oidcClient == nil {
		return ctx
	}
	return oidc.ClientContext(ctx, a.oidcClient)
}

// ValidateBasicAuth validates basic authentication credentials
func (a *Service) ValidateBasicAuth(username, password string) bool {
	// Use constant-time comparison to prevent timing attacks
	usernameMatch := subtle.ConstantTimeCompare(
		[]byte(username),
		[]byte(a.authConfig.Admin.Username),
	) == 1

	passwordMatch := subtle.ConstantTimeCompare(
		[]byte(password),
		[]byte(a.authConfig.Admin.Password),
	) == 1

	return usernameMatch && passwordMatch
}

// GetAuthorizationURL returns the OIDC authorization URL for login
func (a *Service) GetAuthorizationURL(state string) (string, error) {
	if a.oauth2Config == nil {
		return "", fmt.Errorf("OIDC not initialized")
	}

	return a.oauth2Config.AuthCodeURL(state), nil
}

// ExchangeCode exchanges an authorization code for tokens
func (a *Service) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	if a.oauth2Config == nil {
		return nil, fmt.Errorf("OIDC not initialized")
	}

	token, err := a.oauth2Config.Exchange(a.oidcContext(ctx), code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	return token, nil
}

// VerifyIDToken verifies an OIDC ID token and extracts user info
func (a *Service) VerifyIDToken(ctx context.Context, rawIDToken string) (*UserInfo, error) {
	if a.oidcVerifier == nil {
		return nil, fmt.Errorf("OIDC not initialized")
	}

	// Verify the ID token
	idToken, err := a.oidcVerifier.Verify(a.oidcContext(ctx), rawIDToken)
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
		Username: extractClaim(claims, a.authConfig.OIDC.UsernameAttribute),
		Email:    extractClaim(claims, a.authConfig.OIDC.EmailAttribute),
		Name:     extractClaim(claims, a.authConfig.OIDC.NameAttribute),
	}

	// Extract roles if configured
	if a.authConfig.OIDC.RoleAttributePath != "" {
		userInfo.Roles = extractRoles(claims, a.authConfig.OIDC.RoleAttributePath)
	}

	if a.authConfig.OIDC.TeamAttributePath != "" {
		userInfo.Teams = extractRoles(claims, a.authConfig.OIDC.TeamAttributePath)
	}

	return userInfo, nil
}

// GetUserInfo retrieves user information from the OIDC provider
func (a *Service) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	if a.oidcProvider == nil {
		return nil, fmt.Errorf("OIDC not initialized")
	}

	ctx = a.oidcContext(ctx)

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

	logger.Debug().Interface("claims", claims).Msg("Extracted user info claims")

	// Build user info
	userInfo := &UserInfo{
		Username: extractClaim(claims, a.authConfig.OIDC.UsernameAttribute),
		Email:    extractClaim(claims, a.authConfig.OIDC.EmailAttribute),
		Name:     extractClaim(claims, a.authConfig.OIDC.NameAttribute),
	}

	// Extract roles if configured
	if a.authConfig.OIDC.RoleAttributePath != "" {
		userInfo.Roles = extractRoles(claims, a.authConfig.OIDC.RoleAttributePath)
	}

	if a.authConfig.OIDC.TeamAttributePath != "" {
		userInfo.Teams = extractRoles(claims, a.authConfig.OIDC.TeamAttributePath)
	}

	return userInfo, nil
}

// ExtractRolesFromAccessToken parses the access token JWT payload and extracts
// roles using the configured role_attribute_path. Keycloak emits resource_access
// claims only in the access token by default, so this is required to support
// the common Keycloak client-role setup without extra mapper configuration.
//
// The access token was obtained via a verified code exchange with the provider,
// so parsing its claims without re-verifying the signature is safe here.
func (a *Service) ExtractRolesFromAccessToken(accessToken string) []string {
	if accessToken == "" || a.authConfig.OIDC.RoleAttributePath == "" {
		return nil
	}

	parts := strings.Split(accessToken, ".")
	if len(parts) < 2 {
		return nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}

	return extractRoles(claims, a.authConfig.OIDC.RoleAttributePath)
}

// ExtractTeamsFromAccessToken parses the access token JWT payload and extracts
// team claim values using the configured team_attribute_path. Same rationale
// as ExtractRolesFromAccessToken: Keycloak-style IdPs often emit group claims
// only in the access token, which came from a verified code exchange.
func (a *Service) ExtractTeamsFromAccessToken(accessToken string) []string {
	if accessToken == "" || a.authConfig.OIDC.TeamAttributePath == "" {
		return nil
	}

	parts := strings.Split(accessToken, ".")
	if len(parts) < 2 {
		return nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}

	return extractRoles(claims, a.authConfig.OIDC.TeamAttributePath)
}

// IsAdmin checks if the user has any of the configured admin roles.
func (a *Service) IsAdmin(userInfo *UserInfo) bool {
	adminRoles := a.authConfig.OIDC.EffectiveAdminRoles()
	if len(adminRoles) == 0 {
		return false
	}

	for _, role := range userInfo.Roles {
		for _, adminRole := range adminRoles {
			if role == adminRole {
				return true
			}
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

// extractStringArray converts an interface{} to []string if possible.
//
// A scalar string is treated as a single-element list: IdPs commonly emit a
// single role as a bare string (e.g. "garage_role": "garage-ui-admin") rather
// than a one-element array, and discarding it would make admin_role checks
// fail with a spurious 403, see https://github.com/Noooste/garage-ui/issues/75
func extractStringArray(value interface{}) []string {
	// Try a scalar string (single role emitted as a bare value)
	if str, ok := value.(string); ok {
		if str == "" {
			return nil
		}
		return []string{str}
	}

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
func (a *Service) GenerateStateToken() (string, error) {
	return a.jwtService.GenerateStateToken()
}

// ValidateAndConsumeState validates and consumes a CSRF state token
func (a *Service) ValidateAndConsumeState(token string) bool {
	return a.jwtService.ValidateAndConsumeState(token)
}

// GenerateSessionToken generates a JWT session token for the user.
//
// SessionMaxAge lives under the OIDC config block, but the JWT is shared with
// admin-only deployments that never set it. Treat a non-positive value as
// "not configured" and fall back to 24h so the token isn't issued already
// expired (which would make every subsequent request fail with 401).
func (a *Service) GenerateSessionToken(userInfo *UserInfo) (string, error) {
	maxAge := a.authConfig.OIDC.SessionMaxAge
	if maxAge <= 0 {
		maxAge = 86400
	}
	return a.jwtService.GenerateToken(userInfo, maxAge)
}

// ValidateSessionToken validates a JWT session token and returns user info
func (a *Service) ValidateSessionToken(tokenString string) (*UserInfo, error) {
	claims, err := a.jwtService.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	return &UserInfo{
		Username:   claims.Username,
		Email:      claims.Email,
		Name:       claims.Name,
		Roles:      claims.Roles,
		Teams:      claims.Teams,
		AuthMethod: claims.AuthMethod,
	}, nil
}
