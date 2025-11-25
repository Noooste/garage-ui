package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/handlers"
	"Noooste/garage-ui/internal/routes"
	"Noooste/garage-ui/internal/services"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

//	@title			Garage UI API
//	@version		0.1.0
//	@description	REST API for managing Garage distributed object storage system
//	@description	This API provides endpoints for managing buckets, objects, users, and cluster operations.
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.email	support@garage-ui.io

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

const version = "0.1.0"

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	log.Printf("Loading configuration from: %s", *configPath)
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting Garage UI Backend v%s in %s mode", version, cfg.Server.Environment)

	// Initialize services
	log.Println("Initializing Garage Admin service...")
	adminService := services.NewGarageAdminService(&cfg.Garage)

	log.Println("Initializing S3 service...")
	s3Service := services.NewS3Service(&cfg.Garage, adminService)

	log.Printf("Initializing authentication service (mode: %s)...", cfg.Auth.Mode)
	authService, err := auth.NewAuthService(&cfg.Auth)
	if err != nil {
		log.Fatalf("Failed to initialize auth service: %v", err)
	}

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(version)
	bucketHandler := handlers.NewBucketHandler(adminService, s3Service)
	objectHandler := handlers.NewObjectHandler(s3Service)
	userHandler := handlers.NewUserHandler(adminService)
	clusterHandler := handlers.NewClusterHandler(adminService)
	monitoringHandler := handlers.NewMonitoringHandler(adminService, s3Service)

	// Create Fiber app with configuration
	app := fiber.New(fiber.Config{
		AppName: "Garage UI Backend v" + version,
		//DisableStartupMessage: false,
		//EnablePrintRoutes:     cfg.IsDevelopment(),
		ErrorHandler: customErrorHandler,
	})

	// Apply global middleware
	app.Use(recover.New()) // Panic recovery
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} (${latency})\n",
	}))

	// Setup routes
	log.Println("Setting up routes...")
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
	)

	// Start server in a goroutine
	go func() {
		addr := cfg.GetAddress()
		log.Printf("Server listening on %s", addr)
		log.Printf("Health check available at: http://%s/health", addr)
		log.Printf("API documentation: http://%s/api/v1/", addr)

		if err := app.Listen(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	if err := app.Shutdown(); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server stopped gracefully")
}

// customErrorHandler handles errors globally
func customErrorHandler(c fiber.Ctx, err error) error {
	// Default to 500 Internal Server Error
	code := fiber.StatusInternalServerError

	// Check if it's a Fiber error
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	// Log the error
	log.Printf("Error: %v", err)

	// Return JSON error response
	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":    fmt.Sprintf("ERROR_%d", code),
			"message": err.Error(),
		},
	})
}
