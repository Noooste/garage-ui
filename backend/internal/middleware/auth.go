package middleware

import (
	"strings"

	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
	logpkg "Noooste/garage-ui/pkg/logger"

	"github.com/gofiber/fiber/v3"
)

// AuthMiddleware supports admin and OIDC authentication. On success it
// enriches the per-request logger (stored in c.Context() by the Logging
// middleware) with user_id and auth_method so downstream service log lines
// carry user identity. On failure it emits a warn log with the auth_method
// tried and a reason — never the token value.
func AuthMiddleware(cfg *config.AuthConfig, authService *auth.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		// If no auth is enabled, allow all requests.
		if !cfg.Admin.Enabled && !cfg.OIDC.Enabled && !cfg.Token.Enabled {
			return c.Next()
		}

		authHeader := c.Get("Authorization")

		// Try bearer token auth (works for admin, token, or any JWT session)
		if (cfg.Admin.Enabled || cfg.Token.Enabled) && authHeader != "" {
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				token := authHeader[7:]
				userInfo, err := authService.ValidateSessionToken(token)
				if err == nil {
					c.Locals("userInfo", userInfo)
					c.Locals("username", userInfo.Username)
					if userInfo.Email != "" {
						c.Locals("email", userInfo.Email)
					}
					enrichRequestLogger(c, userInfo.Username, "admin")
					return c.Next()
				}
			}
		}

		// Try OIDC auth if enabled.
		if cfg.OIDC.Enabled {
			sessionCookie := c.Cookies(cfg.OIDC.CookieName)
			if sessionCookie != "" {
				userInfo, err := authService.ValidateSessionToken(sessionCookie)
				if err == nil {
					c.Locals("userInfo", userInfo)
					c.Locals("username", userInfo.Username)
					c.Locals("email", userInfo.Email)
					enrichRequestLogger(c, userInfo.Username, "oidc")
					return c.Next()
				}
			}
		}

		// Auth failed — log at warn without exposing token material.
		logpkg.FromCtx(c.Context()).Warn().
			Str("auth_method", authMethodsEnabled(cfg)).
			Str("reason", "no_valid_credentials").
			Msg("authentication_failed")

		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "Authentication required"),
		)
	}
}

// enrichRequestLogger rebinds the per-request logger in c.Context() with
// user_id and auth_method. Subsequent logpkg.FromCtx(c.Context()) calls
// return the enriched logger.
func enrichRequestLogger(c fiber.Ctx, userID, authMethod string) {
	l := logpkg.FromCtx(c.Context()).With().
		Str("user_id", userID).
		Str("auth_method", authMethod).
		Logger()
	c.Locals(LoggerLocalsKey, l)
	c.SetContext(logpkg.IntoCtx(c.Context(), l))
}

func authMethodsEnabled(cfg *config.AuthConfig) string {
	methods := []string{}
	if cfg.Admin.Enabled {
		methods = append(methods, "admin")
	}
	if cfg.OIDC.Enabled {
		methods = append(methods, "oidc")
	}
	if cfg.Token.Enabled {
		methods = append(methods, "token")
	}
	if len(methods) == 0 {
		return "none"
	}
	return strings.Join(methods, "+")
}
