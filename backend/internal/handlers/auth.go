package handlers

import (
	"crypto/subtle"

	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"

	"github.com/gofiber/fiber/v3"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	cfg         *config.Config
	authService *auth.Service
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(cfg *config.Config, authService *auth.Service) *AuthHandler {
	return &AuthHandler{
		cfg:         cfg,
		authService: authService,
	}
}

// GetAuthConfig returns the current authentication configuration
//
//	@Summary		Get authentication configuration
//	@Description	Returns the current auth configuration (admin and/or OIDC)
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	object{admin=object,oidc=object}	"Auth config"
//	@Router			/auth/config [get]
func (h *AuthHandler) GetAuthConfig(c fiber.Ctx) error {
	response := fiber.Map{
		"admin": fiber.Map{
			"enabled": h.cfg.Auth.Admin.Enabled,
		},
		"oidc": fiber.Map{
			"enabled": h.cfg.Auth.OIDC.Enabled,
		},
		"token": fiber.Map{
			"enabled": h.cfg.Auth.Token.Enabled,
		},
	}

	// Add provider name if OIDC is enabled
	if h.cfg.Auth.OIDC.Enabled {
		provider := h.cfg.Auth.OIDC.ProviderName
		if provider == "" {
			provider = "OIDC Provider"
		}
		response["oidc"].(fiber.Map)["provider"] = provider
	}

	return c.JSON(response)
}

// LoginBasicRequest represents the basic auth login request
type LoginBasicRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginAdmin handles admin authentication login
//
//	@Summary		Admin auth login
//	@Description	Authenticate with admin username and password, returns JWT token
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			credentials	body		LoginBasicRequest								true	"Login credentials"
//	@Success		200			{object}	object{success=bool,token=string,user=object}	"Login successful"
//	@Failure		400			{object}	models.APIResponse								"Invalid request"
//	@Failure		401			{object}	models.APIResponse								"Invalid credentials"
//	@Router			/auth/login [post]
func (h *AuthHandler) LoginAdmin(c fiber.Ctx) error {
	// Parse request body
	var req LoginBasicRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"),
		)
	}

	// Validate credentials against admin config
	if req.Username != h.cfg.Auth.Admin.Username || req.Password != h.cfg.Auth.Admin.Password {
		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid credentials"),
		)
	}

	// Create user info object
	userInfo := &auth.UserInfo{
		Username: req.Username,
	}

	// Generate JWT session token
	sessionToken, err := h.authService.GenerateSessionToken(userInfo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to create session"),
		)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"token":   sessionToken,
		"user": fiber.Map{
			"username": userInfo.Username,
		},
	})
}

// LoginTokenRequest represents the token auth login request
type LoginTokenRequest struct {
	Token string `json:"token" validate:"required"`
}

// LoginToken handles admin token authentication login
func (h *AuthHandler) LoginToken(c fiber.Ctx) error {
	var req LoginTokenRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"),
		)
	}

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(h.cfg.Garage.AdminToken), []byte(req.Token)) != 1 {
		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid admin token"),
		)
	}

	userInfo := &auth.UserInfo{
		Username: "admin-token",
	}

	sessionToken, err := h.authService.GenerateSessionToken(userInfo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to create session"),
		)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"token":   sessionToken,
		"user": fiber.Map{
			"username": userInfo.Username,
		},
	})
}

// GetMe returns the current authenticated user's information
//
//	@Summary		Get current user
//	@Description	Returns information about the currently authenticated user
//	@Tags			auth
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	object{success=bool,user=object}	"User information"
//	@Failure		401	{object}	models.APIResponse					"Not authenticated"
//	@Router			/auth/me [get]
func (h *AuthHandler) GetMe(c fiber.Ctx) error {
	// Try to get user info from OIDC context
	userInfoInterface := c.Locals("userInfo")
	if userInfoInterface != nil {
		userInfo, ok := userInfoInterface.(*auth.UserInfo)
		if ok {
			return c.JSON(fiber.Map{
				"success": true,
				"user": fiber.Map{
					"username": userInfo.Username,
					"email":    userInfo.Email,
					"name":     userInfo.Name,
				},
			})
		}
	}

	// Try to get username from basic auth context
	usernameInterface := c.Locals("username")
	if usernameInterface != nil {
		username, ok := usernameInterface.(string)
		if ok {
			return c.JSON(fiber.Map{
				"success": true,
				"user": fiber.Map{
					"username": username,
				},
			})
		}
	}

	return c.Status(fiber.StatusUnauthorized).JSON(
		models.ErrorResponse(models.ErrCodeUnauthorized, "Not authenticated"),
	)
}
