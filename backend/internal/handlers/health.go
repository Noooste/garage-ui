package handlers

import (
	"time"

	"Noooste/garage-ui/internal/models"

	"github.com/gofiber/fiber/v3"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	version string
}

// NewHealthHandler creates a new health check handler
func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{
		version: version,
	}
}

// Check returns the health status of the service
//
//	@Summary		Health check
//	@Description	Returns the health status of the API service along with version information
//	@Tags			Health
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	models.APIResponse{data=models.HealthResponse}	"Service is healthy"
//	@Router			/api/v1/health [get]
func (h *HealthHandler) Check(c fiber.Ctx) error {
	response := models.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   h.version,
	}

	return c.JSON(models.SuccessResponse(response))
}
