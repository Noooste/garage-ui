package routes

import (
	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/handlers"
	"Noooste/garage-ui/internal/middleware"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	// Swagger imports
	_ "Noooste/garage-ui/docs"

	"github.com/Noooste/swagger"
)

// SetupRoutes configures all API routes
func SetupRoutes(
	app *fiber.App,
	cfg *config.Config,
	authService *auth.AuthService,
	healthHandler *handlers.HealthHandler,
	bucketHandler *handlers.BucketHandler,
	objectHandler *handlers.ObjectHandler,
	userHandler *handlers.UserHandler,
	clusterHandler *handlers.ClusterHandler,
	monitoringHandler *handlers.MonitoringHandler,
) {
	// Apply CORS middleware globally
	app.Use(middleware.CORSMiddleware(&cfg.CORS))

	// Health check endpoint (no auth required)
	app.Get("/health", healthHandler.Check)
	app.Get("/api/v1/health", healthHandler.Check)

	// Swagger documentation endpoint (no auth required)
	app.Get("/docs/*", swagger.HandlerDefault)

	// API v1 group
	api := app.Group("/api/v1")

	// Apply authentication middleware to all API routes
	api.Use(middleware.AuthMiddleware(&cfg.Auth, authService))

	// Bucket routes
	buckets := api.Group("/buckets")
	{
		buckets.Get("/", bucketHandler.ListBuckets)                             // List all buckets
		buckets.Post("/", bucketHandler.CreateBucket)                           // Create a new bucket
		buckets.Get("/:name", bucketHandler.GetBucketInfo)                      // Get bucket info
		buckets.Delete("/:name", bucketHandler.DeleteBucket)                    // Delete a bucket
		buckets.Post("/:name/permissions", bucketHandler.GrantBucketPermission) // Grant bucket permissions
	}

	// Object routes
	objects := api.Group("/buckets/:bucket/objects")
	{
		objects.Get("/", objectHandler.ListObjects)                           // List objects in bucket
		objects.Post("/", objectHandler.UploadObject)                         // Upload object (multipart)
		objects.Post("/upload-multiple", objectHandler.UploadMultipleObjects) // Upload multiple objects
		objects.Post("/delete-multiple", objectHandler.DeleteMultipleObjects) // Delete multiple objects
		objects.Get("/:key", objectHandler.GetObject)                         // Download object
		objects.Delete("/:key", objectHandler.DeleteObject)                   // Delete object
		objects.Head("/:key", objectHandler.GetObjectMetadata)                // Get object metadata
		objects.Post("/:key/presign", objectHandler.GetPresignedURL)          // Generate pre-signed URL
	}

	// User/Key management routes
	users := api.Group("/users")
	{
		users.Get("/", userHandler.ListUsers)                          // List all users/keys
		users.Post("/", userHandler.CreateUser)                        // Create new user/key
		users.Get("/:access_key", userHandler.GetUser)                 // Get user info
		users.Delete("/:access_key", userHandler.DeleteUser)           // Delete user/key
		users.Patch("/:access_key", userHandler.UpdateUserPermissions) // Update user permissions
	}

	// Cluster management routes
	cluster := api.Group("/cluster")
	{
		cluster.Get("/health", clusterHandler.GetHealth)                            // Get cluster health
		cluster.Get("/status", clusterHandler.GetStatus)                            // Get cluster status
		cluster.Get("/statistics", clusterHandler.GetStatistics)                    // Get cluster statistics
		cluster.Get("/nodes/:node_id", clusterHandler.GetNodeInfo)                  // Get node info
		cluster.Get("/nodes/:node_id/statistics", clusterHandler.GetNodeStatistics) // Get node statistics
	}

	// Monitoring routes
	monitoring := api.Group("/monitoring")
	{
		monitoring.Get("/metrics", monitoringHandler.GetMetrics)            // Get Prometheus metrics
		monitoring.Get("/admin-health", monitoringHandler.CheckAdminHealth) // Check Admin API health
		monitoring.Get("/dashboard", monitoringHandler.GetDashboardMetrics) // Get dashboard metrics
	}

	// OIDC authentication routes (only if OIDC is enabled)
	if cfg.Auth.Mode == "oidc" && cfg.Auth.OIDC.Enabled {
		authRoutes := app.Group("/auth")
		{
			// Login endpoint - redirects to OIDC provider
			authRoutes.Get("/login", func(c fiber.Ctx) error {
				state, err := authService.GenerateStateToken()
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": "Failed to generate state token",
					})
				}

				authURL, err := authService.GetAuthorizationURL(state)
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": "Failed to generate login URL",
					})
				}
				return c.Redirect().To(authURL)
			})

			// Callback endpoint - handles OIDC redirect after login
			authRoutes.Get("/callback", func(c fiber.Ctx) error {
				// Get and validate state token
				state := c.Query("state")
				if !authService.ValidateAndConsumeState(state) {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Invalid or expired state token",
					})
				}

				// Get authorization code from query
				code := c.Query("code")
				if code == "" {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Authorization code is required",
					})
				}

				// Exchange code for tokens
				ctx := c.Context()
				token, err := authService.ExchangeCode(ctx, code)
				if err != nil {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "Failed to exchange authorization code",
					})
				}

				// Extract ID token from OAuth2 token
				rawIDToken, ok := token.Extra("id_token").(string)
				if !ok {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "No ID token in response",
					})
				}

				// Verify ID token and get user info
				userInfo, err := authService.VerifyIDToken(ctx, rawIDToken)
				if err != nil {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "Invalid ID token",
					})
				}

				// Generate JWT session token
				sessionToken, err := authService.GenerateSessionToken(userInfo)
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": "Failed to create session",
					})
				}

				// Set JWT session token as secure cookie
				c.Cookie(&fiber.Cookie{
					Name:     cfg.Auth.OIDC.CookieName,
					Value:    sessionToken,
					MaxAge:   cfg.Auth.OIDC.SessionMaxAge,
					Secure:   cfg.Auth.OIDC.CookieSecure,
					HTTPOnly: cfg.Auth.OIDC.CookieHTTPOnly,
					SameSite: cfg.Auth.OIDC.CookieSameSite,
				})

				return c.JSON(fiber.Map{
					"success": true,
					"user":    userInfo,
				})
			})

			// Logout endpoint
			authRoutes.Post("/logout", func(c fiber.Ctx) error {
				// Clear session cookie
				c.Cookie(&fiber.Cookie{
					Name:   cfg.Auth.OIDC.CookieName,
					Value:  "",
					MaxAge: -1,
				})

				return c.JSON(fiber.Map{
					"success": true,
					"message": "Logged out successfully",
				})
			})

			// User info endpoint
			authRoutes.Get("/me", middleware.AuthMiddleware(&cfg.Auth, authService), func(c fiber.Ctx) error {
				userInfo := c.Locals("userInfo")
				if userInfo == nil {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "Not authenticated",
					})
				}

				return c.JSON(fiber.Map{
					"success": true,
					"user":    userInfo,
				})
			})
		}
	}

	// Check if frontend path exists
	if _, err := os.Stat(cfg.Server.FrontendPath); err == nil {
		fmt.Println("Serving frontend from:", cfg.Server.FrontendPath)

		// SPA fallback - serve index.html for all non-API routes
		app.Use(func(c fiber.Ctx) error {
			path := c.Path()

			if strings.HasPrefix(path, "/api/") ||
				strings.HasPrefix(path, "/health") ||
				strings.HasPrefix(path, "/docs") {
				fmt.Println("API or health check route, skipping SPA fallback:", path)
				return c.Next()
			}

			// Try to serve static files first
			filePath := filepath.Join(cfg.Server.FrontendPath, path)
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				return c.SendFile(filePath)
			}

			// If no static file exists, serve index.html for SPA routing
			indexPath := filepath.Join(cfg.Server.FrontendPath, "index.html")
			return c.SendFile(indexPath)
		})
	}
}
