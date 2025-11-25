package middleware

import (
	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"

	"github.com/gofiber/fiber/v3"
)

// AuthMiddleware returns a Fiber middleware for authentication
// It handles different auth modes: none, basic, and OIDC
func AuthMiddleware(cfg *config.AuthConfig, authService *auth.AuthService) fiber.Handler {
	return func(c fiber.Ctx) error {
		// If auth mode is "none", allow all requests
		if cfg.Mode == "none" {
			return c.Next()
		}

		// Handle basic authentication
		if cfg.Mode == "basic" {
			return handleBasicAuth(c, authService)
		}

		// Handle OIDC authentication
		if cfg.Mode == "oidc" {
			return handleOIDCAuth(c, authService, &cfg.OIDC)
		}

		// Unknown auth mode - deny access
		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid authentication mode"),
		)
	}
}

// handleBasicAuth validates basic authentication credentials
func handleBasicAuth(c fiber.Ctx, authService *auth.AuthService) error {
	// Get Authorization header
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		c.Set("WWW-Authenticate", `Basic realm="Restricted"`)
		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "Authorization header required"),
		)
	}

	// Parse basic auth credentials
	username, password, ok := auth.ParseBasicAuth(authHeader)
	if !ok {
		c.Set("WWW-Authenticate", `Basic realm="Restricted"`)
		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid Authorization header format"),
		)
	}

	// Validate credentials
	if !authService.ValidateBasicAuth(username, password) {
		c.Set("WWW-Authenticate", `Basic realm="Restricted"`)
		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid credentials"),
		)
	}

	// Store username in context for later use
	c.Locals("username", username)

	return c.Next()
}

// handleOIDCAuth validates OIDC session/token
func handleOIDCAuth(c fiber.Ctx, authService *auth.AuthService, oidcCfg *config.OIDCConfig) error {
	// Get session cookie
	sessionCookie := c.Cookies(oidcCfg.CookieName)
	if sessionCookie == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "Authentication required"),
		)
	}

	// In a production implementation, you would:
	// 1. Validate the session token from the cookie
	// 2. Look up the session in a session store (Redis, memory, etc.)
	// 3. Verify the session hasn't expired
	// 4. Extract user information from the session
	//
	// For now, we'll implement a basic token verification
	// You should extend this based on your session management strategy

	// Verify ID token if it's stored in the session
	ctx := c.Context()
	userInfo, err := authService.VerifyIDToken(ctx, sessionCookie)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid or expired session"),
		)
	}

	// Store user info in context for handlers to use
	c.Locals("userInfo", userInfo)
	c.Locals("username", userInfo.Username)
	c.Locals("email", userInfo.Email)

	return c.Next()
}

// RequireAuth is a simpler middleware that just checks if auth is enabled
// Use this for routes that should only be accessible when any auth is active
func RequireAuth(cfg *config.AuthConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		if cfg.Mode == "none" {
			return c.Status(fiber.StatusForbidden).JSON(
				models.ErrorResponse(models.ErrCodeForbidden, "Authentication is required but not configured"),
			)
		}
		return c.Next()
	}
}

// RequireAdmin is middleware that checks if the user has admin role (OIDC only)
func RequireAdmin(authService *auth.AuthService) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get user info from context (set by AuthMiddleware)
		userInfoInterface := c.Locals("userInfo")
		if userInfoInterface == nil {
			return c.Status(fiber.StatusForbidden).JSON(
				models.ErrorResponse(models.ErrCodeForbidden, "Admin access required"),
			)
		}

		userInfo, ok := userInfoInterface.(*auth.UserInfo)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(
				models.ErrorResponse(models.ErrCodeForbidden, "Admin access required"),
			)
		}

		// Check if user has admin role
		if !authService.IsAdmin(userInfo) {
			return c.Status(fiber.StatusForbidden).JSON(
				models.ErrorResponse(models.ErrCodeForbidden, "Admin role required"),
			)
		}

		return c.Next()
	}
}
