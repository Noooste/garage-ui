package handlers

import (
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"

	"github.com/gofiber/fiber/v3"
)

// MonitoringHandler handles monitoring operations
type MonitoringHandler struct {
	adminService *services.GarageAdminService
	s3Service    *services.S3Service
}

// NewMonitoringHandler creates a new monitoring handler
func NewMonitoringHandler(adminService *services.GarageAdminService, s3Service *services.S3Service) *MonitoringHandler {
	return &MonitoringHandler{
		adminService: adminService,
		s3Service:    s3Service,
	}
}

// GetMetrics retrieves system metrics from the Admin API
//
//	@Summary		Get system metrics
//	@Description	Retrieves system metrics from the Garage Admin API for monitoring purposes
//	@Tags			Monitoring
//	@Accept			json
//	@Produce		text/plain
//	@Success		200	{string}	string	"System metrics in plain text format"
//	@Failure		500	{object}	models.APIResponse{error=models.APIError}	"Failed to retrieve metrics"
//	@Router			/api/v1/monitoring/metrics [get]
func (h *MonitoringHandler) GetMetrics(c fiber.Ctx) error {
	ctx := c.Context()

	metrics, err := h.adminService.GetMetrics(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get metrics: "+err.Error()),
		)
	}

	// Return metrics as plain text
	c.Set("Content-Type", "text/plain; charset=utf-8")
	return c.SendString(metrics)
}

// CheckAdminHealth checks if the Admin API is reachable
//
//	@Summary		Check Admin API health
//	@Description	Performs a health check on the Garage Admin API to verify connectivity and availability
//	@Tags			Monitoring
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	models.APIResponse{data=object{status=string,message=string}}	"Admin API is healthy"
//	@Failure		503	{object}	models.APIResponse{error=models.APIError}						"Admin API health check failed"
//	@Router			/api/v1/monitoring/admin-health [get]
func (h *MonitoringHandler) CheckAdminHealth(c fiber.Ctx) error {
	ctx := c.Context()

	err := h.adminService.HealthCheck(ctx)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Admin API health check failed: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(map[string]interface{}{
		"status":  "healthy",
		"message": "Admin API is reachable",
	}))
}

// GetDashboardMetrics retrieves aggregated dashboard metrics
//
//	@Summary		Get dashboard metrics
//	@Description	Retrieves aggregated metrics for the dashboard including storage, buckets, and request metrics
//	@Tags			Monitoring
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	models.APIResponse{data=models.DashboardMetrics}	"Successfully retrieved dashboard metrics"
//	@Failure		500	{object}	models.APIResponse{error=models.APIError}			"Failed to get dashboard metrics"
//	@Router			/api/v1/monitoring/dashboard [get]
func (h *MonitoringHandler) GetDashboardMetrics(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket list
	buckets, err := h.adminService.ListBuckets(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get buckets: "+err.Error()),
		)
	}

	// Calculate aggregated metrics
	var totalSize int64
	var totalObjects int64
	usageByBucket := make([]models.BucketUsage, 0)

	for _, bucket := range buckets {
		// Get bucket info to calculate size and object count
		bucketInfo, err := h.adminService.GetBucketInfo(ctx, bucket.ID)
		if err != nil {
			continue // Skip buckets we can't access
		}

		// Get size and object count from bucket info
		bucketSize := bucketInfo.Bytes
		objectCount := bucketInfo.Objects

		totalSize += bucketSize
		totalObjects += objectCount

		// Get bucket name from aliases
		bucketName := bucket.ID
		if len(bucket.LocalAliases) > 0 {
			bucketName = bucket.LocalAliases[0].Alias
		} else if len(bucket.GlobalAliases) > 0 {
			bucketName = bucket.GlobalAliases[0]
		}

		usageByBucket = append(usageByBucket, models.BucketUsage{
			BucketName:  bucketName,
			Size:        bucketSize,
			ObjectCount: objectCount,
		})
	}

	// Calculate percentages
	for i := range usageByBucket {
		if totalSize > 0 {
			usageByBucket[i].Percentage = float64(usageByBucket[i].Size) / float64(totalSize) * 100
		}
	}

	dashboardMetrics := models.DashboardMetrics{
		TotalSize:     totalSize,
		ObjectCount:   totalObjects,
		BucketCount:   len(buckets),
		UsageByBucket: usageByBucket,
	}

	return c.JSON(models.SuccessResponse(dashboardMetrics))
}
