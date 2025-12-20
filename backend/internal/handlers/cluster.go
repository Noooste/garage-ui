package handlers

import (
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"

	"github.com/gofiber/fiber/v3"
)

// ClusterHandler handles cluster management operations
type ClusterHandler struct {
	adminService *services.GarageAdminService
}

// NewClusterHandler creates a new cluster handler
func NewClusterHandler(adminService *services.GarageAdminService) *ClusterHandler {
	return &ClusterHandler{
		adminService: adminService,
	}
}

// GetHealth returns the health status of the cluster
//
//	@Summary		Get cluster health
//	@Description	Retrieves the overall health status of the Garage storage cluster
//	@Tags			Cluster
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	models.APIResponse{data=object}				"Successfully retrieved cluster health"
//	@Failure		500	{object}	models.APIResponse{error=models.APIError}	"Failed to get cluster health"
//	@Router			/api/v1/cluster/health [get]
func (h *ClusterHandler) GetHealth(c fiber.Ctx) error {
	ctx := c.Context()

	health, err := h.adminService.GetClusterHealth(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get cluster health: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(health))
}

// GetStatus returns the status of the cluster
//
//	@Summary		Get cluster status
//	@Description	Retrieves the current status of the Garage storage cluster
//	@Tags			Cluster
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	models.APIResponse{data=object}				"Successfully retrieved cluster status"
//	@Failure		500	{object}	models.APIResponse{error=models.APIError}	"Failed to get cluster status"
//	@Router			/api/v1/cluster/status [get]
func (h *ClusterHandler) GetStatus(c fiber.Ctx) error {
	ctx := c.Context()

	status, err := h.adminService.GetClusterStatus(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get cluster status: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(status))
}

// GetStatistics returns global cluster statistics
// GET /api/v1/cluster/statistics
func (h *ClusterHandler) GetStatistics(c fiber.Ctx) error {
	ctx := c.Context()

	stats, err := h.adminService.GetClusterStatistics(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get cluster statistics: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(stats))
}

// GetNodeInfo returns information about a specific node
//
//	@Summary		Get node information
//	@Description	Retrieves detailed information about a specific node in the Garage storage cluster
//	@Tags			Cluster
//	@Accept			json
//	@Produce		json
//	@Param			node_id	path		string										true	"ID of the node to retrieve information for"
//	@Success		200		{object}	models.APIResponse{data=object}				"Successfully retrieved node information"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}	"Node ID is required"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}	"Failed to get node information"
//	@Router			/api/v1/cluster/nodes/{node_id} [get]
func (h *ClusterHandler) GetNodeInfo(c fiber.Ctx) error {
	ctx := c.Context()
	nodeID := c.Params("node_id")

	if nodeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Node ID is required"),
		)
	}

	info, err := h.adminService.GetNodeInfo(ctx, nodeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get node info: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(info))
}

// GetNodeStatistics returns statistics for a specific node
//
//	@Summary		Get node statistics
//	@Description	Retrieves performance statistics and metrics for a specific node in the Garage storage cluster
//	@Tags			Cluster
//	@Accept			json
//	@Produce		json
//	@Param			node_id	path		string										true	"ID of the node to retrieve statistics for"
//	@Success		200		{object}	models.APIResponse{data=object}				"Successfully retrieved node statistics"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}	"Node ID is required"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}	"Failed to get node statistics"
//	@Router			/api/v1/cluster/nodes/{node_id}/statistics [get]
func (h *ClusterHandler) GetNodeStatistics(c fiber.Ctx) error {
	ctx := c.Context()
	nodeID := c.Params("node_id")

	if nodeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Node ID is required"),
		)
	}

	stats, err := h.adminService.GetNodeStatistics(ctx, nodeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get node statistics: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(stats))
}
