package middleware

import (
	"strings"

	"Noooste/garage-ui/internal/config"

	"github.com/gofiber/fiber/v3"
)

// CORSMiddleware creates a CORS middleware from configuration
func CORSMiddleware(cfg *config.CORSConfig) fiber.Handler {
	// If CORS is disabled, return a no-op middleware
	if !cfg.Enabled {
		return func(c fiber.Ctx) error {
			return c.Next()
		}
	}

	return func(c fiber.Ctx) error {
		origin := c.Get("Origin")

		// Check if origin is allowed
		if origin != "" && isAllowedOrigin(origin, cfg.AllowedOrigins) {
			// Set CORS headers
			c.Set("Access-Control-Allow-Origin", origin)

			if cfg.AllowCredentials {
				c.Set("Access-Control-Allow-Credentials", "true")
			}

			// Set allowed methods
			if len(cfg.AllowedMethods) > 0 {
				c.Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
			}

			// Set allowed headers
			if len(cfg.AllowedHeaders) > 0 {
				c.Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
			}

			// Set max age for preflight cache
			if cfg.MaxAge > 0 {
				c.Set("Access-Control-Max-Age", string(rune(cfg.MaxAge)))
			}
		}

		// Handle preflight requests
		if c.Method() == "OPTIONS" {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

// isAllowedOrigin checks if an origin is in the allowed list
func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}
