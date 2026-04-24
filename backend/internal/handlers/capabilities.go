package handlers

import (
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"

	"github.com/gofiber/fiber/v3"
)

type CapabilitiesHandler struct {
	apiVersion   string
	capabilities services.Capabilities
}

func NewCapabilitiesHandler(apiVersion string, capabilities services.Capabilities) *CapabilitiesHandler {
	return &CapabilitiesHandler{
		apiVersion:   apiVersion,
		capabilities: capabilities,
	}
}

func (h *CapabilitiesHandler) GetCapabilities(c fiber.Ctx) error {
	return c.JSON(models.SuccessResponse(fiber.Map{
		"garageApiVersion": h.apiVersion,
		"features":         h.capabilities,
	}))
}
