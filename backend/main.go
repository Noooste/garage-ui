package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/authz"
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/handlers"
	appmw "Noooste/garage-ui/internal/middleware"
	"Noooste/garage-ui/internal/routes"
	"Noooste/garage-ui/internal/services"
	"Noooste/garage-ui/pkg/logger"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/rs/zerolog/log"
)

//	@title			Garage UI API
//	@version		0.1.0
//	@description	REST API for managing Garage distributed object storage system
//	@description	This API provides endpoints for managing buckets, objects, users, and cluster operations.
//	@termsOfService	http://swagger.io/terms/

//	@license.name	MIT
//	@license.url	https://opensource.org/licenses/MIT

//	@host		localhost:8080
//	@BasePath	/
//	@schemes	http https

//	@tag.name			Health
//	@tag.description	Health check endpoints

//	@tag.name			Buckets
//	@tag.description	Bucket management operations

//	@tag.name			Objects
//	@tag.description	Object storage and retrieval operations

//	@tag.name			Users
//	@tag.description	User and access key management

//	@tag.name			Cluster
//	@tag.description	Cluster status and node management

//	@tag.name			Monitoring
//	@tag.description	Monitoring and metrics endpoints

//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer" followed by a space and JWT token.

var version = "dev"

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	garageTomlPath := flag.String("garage-toml", "", "Path to garage.toml file (extracts Garage connection values)")
	flag.Parse()

	// Env var fallback for --garage-toml
	if *garageTomlPath == "" {
		if envPath := os.Getenv("GARAGE_UI_GARAGE_TOML"); envPath != "" {
			*garageTomlPath = envPath
		}
	}

	// Build load options
	var loadOpts []config.LoadOption
	if *garageTomlPath != "" {
		loadOpts = append(loadOpts, config.WithGarageToml(*garageTomlPath))
	}

	// Load configuration first (before initializing logger)
	cfg, err := config.Load(*configPath, loadOpts...)
	if err != nil {
		// If config fails to load, use default logger to report the error
		logger.Get().Fatal().Err(err).Str("config_path", *configPath).Msg("Failed to load configuration")
	}

	// Initialize logger with configuration from config file
	logger.Init(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})

	// Now log with the properly configured logger
	logger.Info().
		Str("config_path", *configPath).
		Str("version", version).
		Str("go_version", runtime.Version()).
		Str("environment", cfg.Server.Environment).
		Msg("Starting Garage UI Backend")

	if *garageTomlPath != "" {
		logger.Warn().
			Str("s3_endpoint", cfg.Garage.Endpoint).
			Str("admin_endpoint", cfg.Garage.AdminEndpoint).
			Msg("Endpoints inferred from garage.toml bind addresses — override with GARAGE_UI_GARAGE_ENDPOINT / GARAGE_UI_GARAGE_ADMIN_ENDPOINT for remote/container setups")
	}

	// Initialize services
	logger.Info().Msg("Detecting Garage API version")
	adminResult, err := services.NewAdminService(&cfg.Garage, cfg.Logging.Level)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Garage admin API")
	}
	adminService := adminResult.Service
	capabilitiesHandler := handlers.NewCapabilitiesHandler(adminResult.APIVersion, adminResult.Capabilities, cfg.AccessControl != nil)

	logger.Info().Msg("Initializing S3 service")
	s3Service := services.NewS3Service(&cfg.Garage, adminService)

	// Determine enabled auth methods for logging
	authMethods := []string{}
	if cfg.Auth.Admin.Enabled {
		authMethods = append(authMethods, "admin")
	}
	if cfg.Auth.OIDC.Enabled {
		authMethods = append(authMethods, "oidc")
	}
	if cfg.Auth.Token.Enabled {
		authMethods = append(authMethods, "token")
	}
	if len(authMethods) == 0 {
		authMethods = append(authMethods, "none")
	}
	logger.Info().Strs("enabled_methods", authMethods).Msg("Initializing authentication service")
	authService, err := auth.NewAuthService(&cfg.Auth, &cfg.Server)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize auth service")
	}

	policy, err := authz.CompilePolicy(cfg.AccessControl)
	if err != nil {
		logger.Fatal().Err(err).Msg("Invalid access_control configuration")
	}
	if cfg.AccessControl != nil && !cfg.Auth.OIDC.Enabled {
		logger.Warn().Msg("access_control is configured but OIDC is disabled: admin and token logins are always full-admin in v1, so the policy currently gates nothing")
	}
	azMiddleware := authz.NewMiddleware(policy, authz.NewTeamResolver(policy, cfg.Auth.OIDC.EffectiveAdminRoles()), authz.NewAuthorizer())

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(version)
	bucketHandler := handlers.NewBucketHandler(adminService, s3Service)
	objectHandler := handlers.NewObjectHandler(s3Service, authService)
	userHandler := handlers.NewUserHandler(adminService)
	clusterHandler := handlers.NewClusterHandler(adminService)
	monitoringHandler := handlers.NewMonitoringHandler(adminService, s3Service)

	// Set default values for buffer sizes if not configured
	maxBodySize := cfg.Server.MaxBodySize
	if maxBodySize == 0 {
		maxBodySize = 300 * 1024 * 1024 // 300MB default
	}
	maxHeaderSize := cfg.Server.MaxHeaderSize
	if maxHeaderSize == 0 {
		maxHeaderSize = 1 * 1024 * 1024 // 1MB default
	}
	readBufferSize := cfg.Server.ReadBufferSize
	if readBufferSize == 0 {
		readBufferSize = 4096 // 4KB default
	}
	writeBufferSize := cfg.Server.WriteBufferSize
	if writeBufferSize == 0 {
		writeBufferSize = 4096 // 4KB default
	}

	logger.Info().
		Int64("max_body_bytes", maxBodySize).
		Float64("max_body_mb", float64(maxBodySize)/(1024*1024)).
		Int("max_header_bytes", maxHeaderSize).
		Float64("max_header_kb", float64(maxHeaderSize)/1024).
		Msg("Server request limits configured")

	// Create Fiber app with configuration
	app := fiber.New(fiber.Config{
		AppName:         "Garage UI Backend | Version: " + version,
		BodyLimit:       int(maxBodySize),
		ReadBufferSize:  readBufferSize,
		WriteBufferSize: writeBufferSize,
		ErrorHandler:    customErrorHandler,
	})

	// Apply global middleware (order matters):
	//   1. recover — must be outermost so panics become 500s.
	//   2. RequestID — mints/reads X-Request-ID before any logger needs it.
	//   3. Logging — builds per-request zerolog logger + emits access log.
	// Auth middleware is installed per-route inside routes.SetupRoutes.
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, e interface{}) {
			logger.FromCtx(c.Context()).Error().
				Interface("panic", e).
				Bytes("stack", debug.Stack()).
				Msg("panic_recovered")
		},
	}))
	app.Use(appmw.RequestID())
	app.Use(appmw.Logging(log.Logger))

	// Setup routes
	logger.Info().Msg("Setting up routes")
	routes.SetupRoutes(
		app,
		cfg,
		authService,
		healthHandler,
		bucketHandler,
		objectHandler,
		userHandler,
		clusterHandler,
		monitoringHandler,
		capabilitiesHandler,
		azMiddleware,
	)

	if err := authz.VerifyRouteCoverage(app); err != nil {
		logger.Fatal().Err(err).Msg("authz route coverage check failed")
	}

	// Start server in a goroutine
	go func() {
		addr := cfg.GetAddress()
		logger.Info().
			Str("address", addr).
			Str("network", fiber.NetworkTCP).
			Str("health_endpoint", fmt.Sprintf("http://%s/health", addr)).
			Str("api_docs", fmt.Sprintf("http://%s/api/v1/", addr)).
			Msg("Server starting")

		if err := app.Listen(addr, fiber.ListenConfig{ListenerNetwork: fiber.NetworkTCP}); err != nil {
			logger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit

	logger.Info().Str("signal", sig.String()).Msg("Shutting down server")
	shutdownStart := time.Now()
	if err := app.Shutdown(); err != nil {
		logger.Fatal().Err(err).Msg("Server shutdown failed")
	}

	logger.Info().
		Dur("shutdown_duration", time.Since(shutdownStart)).
		Msg("Server stopped gracefully")
}

// customErrorHandler handles errors globally. It uses the per-request logger
// from c.Context() so request_id / user_id attach automatically, and it
// demotes expected 4xx responses to warn (5xx stays at error).
func customErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	l := logger.FromCtx(c.Context())
	evt := l.Error()
	if code >= 400 && code < 500 {
		evt = l.Warn()
	}
	evt.Err(err).Int("status_code", code).Msg("request_error")

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":    fmt.Sprintf("ERROR_%d", code),
			"message": err.Error(),
		},
	})
}
