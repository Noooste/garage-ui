package routes

import (
	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/authz"
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/handlers"
	"Noooste/garage-ui/internal/middleware"
	"Noooste/garage-ui/pkg/logger"
	"net/url"
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
	authService *auth.Service,
	healthHandler *handlers.HealthHandler,
	bucketHandler *handlers.BucketHandler,
	objectHandler *handlers.ObjectHandler,
	userHandler *handlers.UserHandler,
	clusterHandler *handlers.ClusterHandler,
	monitoringHandler *handlers.MonitoringHandler,
	capabilitiesHandler *handlers.CapabilitiesHandler,
	az *authz.Middleware,
) {
	// Apply CORS middleware globally
	app.Use(middleware.CORSMiddleware(&cfg.CORS))

	// Health check endpoint (no auth required)
	app.Get("/health", healthHandler.Check)
	app.Get("/api/v1/health", healthHandler.Check)

	// Swagger documentation endpoint (no auth required)
	app.Get("/docs/*", swagger.HandlerDefault)

	// Create auth handler
	authHandler := handlers.NewAuthHandler(cfg, authService)

	// Auth configuration endpoint (always accessible, no auth required)
	app.Get("/auth/config", authHandler.GetAuthConfig)

	// Public Prometheus metrics endpoint (no auth), opt-in via auth.metrics_public.
	// Registered outside /api/v1 so it bypasses the AuthMiddleware/ResolveSubject
	// cascade and the VerifyRouteCoverage fail-closed guard entirely; the
	// authenticated /api/v1/monitoring/metrics route is unaffected. Because it is
	// registered before the SPA fallback below, Fiber matches it first.
	// Protect it at the network layer (NetworkPolicy / trusted scrape network).
	if cfg.Auth.MetricsPublic {
		app.Get("/metrics", monitoringHandler.GetMetrics)
	}

	// API v1 group
	api := app.Group("/api/v1")

	// Apply authentication middleware to all API routes
	api.Use(middleware.AuthMiddleware(&cfg.Auth, authService))

	// Resolve the authz Subject once per request, right after authentication.
	api.Use(az.ResolveSubject())

	api.Get("/capabilities", capabilitiesHandler.GetCapabilities)

	// Bucket routes
	buckets := api.Group("/buckets")
	{
		buckets.Get("/", az.Require(authz.ScopeNone, authz.PermBucketList), bucketHandler.ListBuckets)                                                                        // List all buckets
		buckets.Post("/", az.Require(authz.BucketFromBody(), authz.PermBucketCreate), bucketHandler.CreateBucket)                                                             // Create a new bucket
		buckets.Get("/:name", az.Require(authz.BucketFromParam("name"), authz.PermBucketRead), bucketHandler.GetBucketInfo)                                                   // Get bucket info
		buckets.Delete("/:name", az.Require(authz.BucketFromParam("name"), authz.PermBucketDelete), bucketHandler.DeleteBucket)                                               // Delete a bucket
		buckets.Post("/:name/permissions", az.Require(authz.BucketFromParam("name"), authz.PermAllowBucketKey, authz.PermDenyBucketKey), bucketHandler.GrantBucketPermission) // Grant bucket permissions (allow+deny)
		buckets.Put("/:name/website", az.Require(authz.BucketFromParam("name"), authz.PermBucketUpdate), bucketHandler.UpdateBucketWebsite)                                   // Update bucket website configuration
		buckets.Put("/:name/quotas", az.Require(authz.BucketFromParam("name"), authz.PermBucketUpdate), bucketHandler.UpdateBucketQuotas)                                     // Update bucket quotas
	}

	// Object routes
	objects := api.Group("/buckets/:bucket/objects")
	{
		objects.Get("/", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectList), objectHandler.ListObjects)                             // List objects in bucket
		objects.Post("/", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectWrite), objectHandler.UploadObject)                          // Upload object (multipart)
		objects.Post("/upload-multiple", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectWrite), objectHandler.UploadMultipleObjects)  // Upload multiple objects
		objects.Post("/delete-multiple", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectDelete), objectHandler.DeleteMultipleObjects) // Delete multiple objects
	}

	// Directory routes (zero-byte directory markers)
	api.Post("/buckets/:bucket/directories", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectWrite), objectHandler.CreateDirectory)

	// Fiber v3 does not auto-decode wildcard params; fall back to the raw
	// value when QueryUnescape fails.
	decodeObjectKey := func(c fiber.Ctx) string {
		raw := c.Params("*")
		if decoded, err := url.QueryUnescape(raw); err == nil {
			return decoded
		}
		return raw
	}

	objectWildcardHandler := func(c fiber.Ctx) error {
		path := decodeObjectKey(c)
		switch {
		case strings.HasSuffix(path, "/metadata"):
			c.Locals("objectKey", strings.TrimSuffix(path, "/metadata"))
			return objectHandler.GetObjectMetadata(c)
		case strings.HasSuffix(path, "/presign"):
			c.Locals("objectKey", strings.TrimSuffix(path, "/presign"))
			return objectHandler.GetPresignedURL(c)
		case strings.HasSuffix(path, "/preview-url"):
			c.Locals("objectKey", strings.TrimSuffix(path, "/preview-url"))
			return objectHandler.GetPreviewURL(c)
		default:
			c.Locals("objectKey", path)
			return objectHandler.GetObject(c)
		}
	}

	objectDeleteHandler := func(c fiber.Ctx) error {
		c.Locals("objectKey", decodeObjectKey(c))
		return objectHandler.DeleteObject(c)
	}

	objectHeadHandler := func(c fiber.Ctx) error {
		c.Locals("objectKey", decodeObjectKey(c))
		return objectHandler.GetObjectMetadata(c)
	}

	// Register with auth middleware. Although these routes live on app, not
	// api, the api group's .Use() middlewares (AuthMiddleware, ResolveSubject)
	// cascade onto them by path prefix, so ResolveSubject is not repeated here
	// (TestWildcardObjectRoutes_EnforceAuthzViaGroupCascade locks that in).
	app.Get("/api/v1/buckets/:bucket/objects/*", middleware.AuthMiddleware(&cfg.Auth, authService), az.Require(authz.BucketFromParam("bucket"), authz.PermObjectRead), objectWildcardHandler)
	app.Delete("/api/v1/buckets/:bucket/objects/*", middleware.AuthMiddleware(&cfg.Auth, authService), az.Require(authz.BucketFromParam("bucket"), authz.PermObjectDelete), objectDeleteHandler)
	app.Head("/api/v1/buckets/:bucket/objects/*", middleware.AuthMiddleware(&cfg.Auth, authService), az.Require(authz.BucketFromParam("bucket"), authz.PermObjectRead), objectHeadHandler)

	// User/Key management routes
	users := api.Group("/users")
	{
		users.Get("/", az.Require(authz.ScopeNone, authz.PermKeyList), userHandler.ListUsers)                                // List all users/keys
		users.Post("/", az.Require(authz.ScopeNone, authz.PermKeyCreate), userHandler.CreateUser)                            // Create new user/key
		users.Get("/:access_key", az.Require(authz.ScopeNone, authz.PermKeyRead), userHandler.GetUser)                       // Get user info
		users.Get("/:access_key/secret", az.Require(authz.ScopeNone, authz.PermKeyReadSecret), userHandler.GetUserSecretKey) // Get user secret key
		users.Delete("/:access_key", az.Require(authz.ScopeNone, authz.PermKeyDelete), userHandler.DeleteUser)               // Delete user/key
		users.Patch("/:access_key", az.Require(authz.ScopeNone, authz.PermKeyUpdate), userHandler.UpdateUserPermissions)     // Update user permissions
	}

	// Cluster management routes
	cluster := api.Group("/cluster")
	{
		cluster.Get("/health", az.Require(authz.ScopeNone, authz.PermClusterHealth), clusterHandler.GetHealth)                             // Get cluster health
		cluster.Get("/status", az.Require(authz.ScopeNone, authz.PermClusterStatus), clusterHandler.GetStatus)                             // Get cluster status
		cluster.Get("/statistics", az.Require(authz.ScopeNone, authz.PermClusterStatistics), clusterHandler.GetStatistics)                 // Get cluster statistics
		cluster.Get("/nodes/:node_id", az.Require(authz.ScopeNone, authz.PermNodeInfo), clusterHandler.GetNodeInfo)                        // Get node info
		cluster.Get("/nodes/:node_id/statistics", az.Require(authz.ScopeNone, authz.PermNodeStatistics), clusterHandler.GetNodeStatistics) // Get node statistics
	}

	// Monitoring routes
	monitoring := api.Group("/monitoring")
	{
		monitoring.Get("/metrics", az.Require(authz.ScopeNone, authz.PermClusterStatistics), monitoringHandler.GetMetrics)            // Get Prometheus metrics
		monitoring.Get("/admin-health", az.Require(authz.ScopeNone, authz.PermClusterHealth), monitoringHandler.CheckAdminHealth)     // Check Admin API health
		monitoring.Get("/dashboard", az.Require(authz.ScopeNone, authz.PermClusterStatistics), monitoringHandler.GetDashboardMetrics) // Get dashboard metrics
	}

	// Admin auth login endpoint (only if admin is enabled)
	if cfg.Auth.Admin.Enabled {
		app.Post("/auth/login", authHandler.LoginAdmin)
	}

	// Token auth login endpoint (only if token auth is enabled)
	if cfg.Auth.Token.Enabled {
		app.Post("/auth/login-token", authHandler.LoginToken)
	}

	// Auth "me" endpoint (if any auth is enabled)
	if cfg.Auth.Admin.Enabled || cfg.Auth.OIDC.Enabled || cfg.Auth.Token.Enabled {
		app.Get("/auth/me", middleware.AuthMiddleware(&cfg.Auth, authService), authHandler.GetMe)
	}

	// OIDC authentication routes (only if OIDC is enabled)
	if cfg.Auth.OIDC.Enabled {
		oidcRoutes := app.Group("/auth/oidc")
		{
			// Login endpoint - redirects to OIDC provider
			oidcRoutes.Get("/login", func(c fiber.Ctx) error {
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
			oidcRoutes.Get("/callback", func(c fiber.Ctx) error {
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

				logger.Debug().
					Str("access_token", token.AccessToken).
					Interface("token", token).
					Msg("Exchanged authorization code for token")

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

				// With access_control configured, non-admin users may log in:
				// they get team-scoped (possibly zero) permissions and
				// default-deny protects everything else. Without it, the
				// admin role remains the only thing standing between an IdP
				// account and full cluster access, so keep the historical gate.
				adminRoles := cfg.Auth.OIDC.EffectiveAdminRoles()
				if len(adminRoles) > 0 {
					if !authService.IsAdmin(userInfo) {
						if roles := authService.ExtractRolesFromAccessToken(token.AccessToken); len(roles) > 0 {
							userInfo.Roles = roles
						}
					}
					if !authService.IsAdmin(userInfo) {
						if ui, err := authService.GetUserInfo(ctx, token); err == nil && len(ui.Roles) > 0 {
							userInfo.Roles = ui.Roles
						}
					}
					if cfg.AccessControl == nil && !authService.IsAdmin(userInfo) {
						logger.Warn().
							Str("username", userInfo.Username).
							Strs("required_roles", adminRoles).
							Strs("roles", userInfo.Roles).
							Msg("OIDC login denied: user does not have any required admin role")
						return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
							"error": "User does not have the required admin role",
						})
					}
				}

				// Teams follow the same claim-location fallbacks as roles.
				if cfg.Auth.OIDC.TeamAttributePath != "" && len(userInfo.Teams) == 0 {
					if teams := authService.ExtractTeamsFromAccessToken(token.AccessToken); len(teams) > 0 {
						userInfo.Teams = teams
					}
				}
				if cfg.Auth.OIDC.TeamAttributePath != "" && len(userInfo.Teams) == 0 {
					if ui, err := authService.GetUserInfo(ctx, token); err == nil && len(ui.Teams) > 0 {
						userInfo.Teams = ui.Teams
					}
				}
				userInfo.AuthMethod = "oidc"

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

				// Redirect to frontend with success indicator
				return c.Redirect().To("/login?login=success")
			})

			// Logout endpoint
			oidcRoutes.Post("/logout", func(c fiber.Ctx) error {
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
		}
	}

	cfg.Server.FrontendPath = "./frontend/dist"

	// Check if frontend path exists
	if _, err := os.Stat(cfg.Server.FrontendPath); err == nil {
		// SPA fallback - serve index.html for all non-API routes
		app.Use(func(c fiber.Ctx) error {
			path := c.Path()

			if strings.HasPrefix(path, "/api/") ||
				strings.HasPrefix(path, "/auth") ||
				strings.HasPrefix(path, "/health") ||
				strings.HasPrefix(path, "/docs") ||
				path == "/metrics" {
				logger.Debug().Str("path", path).Msg("API or health check route, skipping SPA fallback")
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
