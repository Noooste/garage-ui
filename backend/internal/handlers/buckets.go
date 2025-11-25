package handlers

import (
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"

	"github.com/gofiber/fiber/v3"
)

// BucketHandler handles bucket-related operations
type BucketHandler struct {
	adminService *services.GarageAdminService
	s3Service    *services.S3Service
}

// NewBucketHandler creates a new bucket handler
func NewBucketHandler(adminService *services.GarageAdminService, s3Service *services.S3Service) *BucketHandler {
	return &BucketHandler{
		adminService: adminService,
		s3Service:    s3Service,
	}
}

// ListBuckets lists all buckets
//
//	@Summary		List all buckets
//	@Description	Retrieves a list of all buckets in the Garage storage system with object count and size
//	@Tags			Buckets
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	models.APIResponse{data=models.BucketListResponse}	"Successfully retrieved list of buckets"
//	@Failure		500	{object}	models.APIResponse{error=models.APIError}			"Failed to list buckets"
//	@Router			/api/v1/buckets [get]
func (h *BucketHandler) ListBuckets(c fiber.Ctx) error {
	ctx := c.Context()

	// List all buckets from Garage Admin API
	adminBuckets, err := h.adminService.ListBuckets(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeListFailed, "Failed to list buckets: "+err.Error()),
		)
	}

	// Convert admin bucket response to BucketInfo
	buckets := make([]models.BucketInfo, 0, len(adminBuckets))
	for _, adminBucket := range adminBuckets {
		// Get the bucket name from global aliases
		var bucketName string
		if len(adminBucket.GlobalAliases) > 0 {
			bucketName = adminBucket.GlobalAliases[0]
		} else {
			// Skip buckets without global aliases
			continue
		}

		bucketInfo := models.BucketInfo{
			Name:         bucketName,
			CreationDate: adminBucket.Created,
			Region:       "", // Garage doesn't have regions
		}

		// Try to get bucket statistics (object count and size)
		// This is done asynchronously to avoid blocking the response
		// If it fails, we still return the bucket info without stats
		stats, err := h.s3Service.GetBucketStatistics(ctx, bucketName)
		if err == nil && stats != nil {
			bucketInfo.ObjectCount = &stats.ObjectCount
			bucketInfo.Size = &stats.TotalSize
		}

		buckets = append(buckets, bucketInfo)
	}

	response := models.BucketListResponse{
		Buckets: buckets,
		Count:   len(buckets),
	}

	return c.JSON(models.SuccessResponse(response))
}

// CreateBucket creates a new bucket
//
//	@Summary		Create a new bucket
//	@Description	Creates a new bucket in the Garage storage system
//	@Tags			Buckets
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		models.CreateBucketRequest						true	"Bucket creation payload"
//	@Success		201		{object}	models.APIResponse{data=object{bucket=string,message=string}}	"Bucket created successfully"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}				"Invalid request body or bucket name is required"
//	@Failure		409		{object}	models.APIResponse{error=models.APIError}				"Bucket already exists"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}				"Failed to create bucket"
//	@Router			/api/v1/buckets [post]
func (h *BucketHandler) CreateBucket(c fiber.Ctx) error {
	ctx := c.Context()

	// Parse request body
	var req models.CreateBucketRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body: "+err.Error()),
		)
	}

	// Validate bucket name
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	// Check if bucket already exists
	bucketInfo, err := h.adminService.GetBucketInfoByAlias(ctx, req.Name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to check bucket existence: "+err.Error()),
		)
	}

	if bucketInfo != nil {
		return c.Status(fiber.StatusConflict).JSON(
			models.ErrorResponse(models.ErrCodeBucketExists, "Bucket already exists"),
		)
	}

	// Create the bucket
	createBucketReq := models.CreateBucketAdminRequest{
		GlobalAlias: &req.Name,
	}
	if bucketInfo, err = h.adminService.CreateBucket(ctx, createBucketReq); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to create bucket: "+err.Error()),
		)
	}

	// Return success response
	response := map[string]interface{}{
		"bucket":  req.Name,
		"message": "Bucket created successfully",
	}

	return c.Status(fiber.StatusCreated).JSON(models.SuccessResponse(response))
}

// DeleteBucket deletes a bucket
//
//	@Summary		Delete a bucket
//	@Description	Deletes an existing bucket from the Garage storage system. The bucket must be empty before deletion.
//	@Tags			Buckets
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string															true	"Name of the bucket to delete"
//	@Success		200		{object}	models.APIResponse{data=object{bucket=string,message=string}}	"Bucket deleted successfully"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}						"Bucket name is required"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}						"Bucket does not exist"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}						"Failed to delete bucket"
//	@Router			/api/v1/buckets/{name} [delete]
func (h *BucketHandler) DeleteBucket(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameter
	bucketName := c.Params("name")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	// Check if bucket already exists
	bucketInfo, err := h.adminService.GetBucketInfoByAlias(ctx, bucketName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to check bucket existence: "+err.Error()),
		)
	}

	if bucketInfo == nil {
		return c.Status(fiber.StatusNotFound).JSON(
			models.ErrorResponse(models.ErrCodeBucketNotFound, "Bucket does not exist"),
		)
	}

	// Delete the bucket
	if err := h.adminService.DeleteBucket(ctx, bucketInfo.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeDeleteFailed, "Failed to delete bucket: "+err.Error()),
		)
	}

	// Return success response
	response := map[string]interface{}{
		"bucket":  bucketName,
		"message": "Bucket deleted successfully",
	}

	return c.JSON(models.SuccessResponse(response))
}

// GetBucketInfo returns information about a specific bucket
//
//	@Summary		Get bucket information
//	@Description	Retrieves detailed information about a specific bucket including creation date and region
//	@Tags			Buckets
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string										true	"Name of the bucket to retrieve information for"
//	@Success		200		{object}	models.APIResponse{data=models.BucketInfo}	"Successfully retrieved bucket information"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}	"Bucket name is required"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}	"Bucket does not exist"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}	"Failed to retrieve bucket information"
//	@Router			/api/v1/buckets/{name} [get]
func (h *BucketHandler) GetBucketInfo(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameter
	bucketName := c.Params("name")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	// Check if bucket already exists
	bucketInfo, err := h.adminService.GetBucketInfoByAlias(ctx, bucketName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to check bucket existence: "+err.Error()),
		)
	}

	if bucketInfo == nil {
		return c.Status(fiber.StatusNotFound).JSON(
			models.ErrorResponse(models.ErrCodeBucketNotFound, "Bucket does not exist"),
		)
	}

	return c.JSON(models.SuccessResponse(bucketInfo))
}

// GrantBucketPermission grants permissions for an access key on a bucket
//
//	@Summary		Grant bucket permissions
//	@Description	Grants read/write/owner permissions for an access key on a specific bucket
//	@Tags			Buckets
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string												true	"Name of the bucket"
//	@Param			request	body		models.GrantBucketPermissionRequest					true	"Permission grant request"
//	@Success		200		{object}	models.APIResponse{data=models.GarageBucketInfo}	"Permissions granted successfully"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}			"Invalid request"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}			"Bucket not found"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}			"Failed to grant permissions"
//	@Router			/api/v1/buckets/{name}/permissions [post]
func (h *BucketHandler) GrantBucketPermission(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameter
	bucketName := c.Params("name")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	// Parse request body
	var req models.GrantBucketPermissionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body: "+err.Error()),
		)
	}

	// Validate access key ID
	if req.AccessKeyID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Access key ID is required"),
		)
	}

	// Get bucket info to retrieve bucket ID
	bucketInfo, err := h.adminService.GetBucketInfoByAlias(ctx, bucketName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get bucket info: "+err.Error()),
		)
	}

	if bucketInfo == nil {
		return c.Status(fiber.StatusNotFound).JSON(
			models.ErrorResponse(models.ErrCodeBucketNotFound, "Bucket does not exist"),
		)
	}

	// Build the permission request for Garage Admin API
	permRequest := models.BucketKeyPermRequest{
		BucketID:    bucketInfo.ID,
		AccessKeyID: req.AccessKeyID,
		Permissions: models.BucketKeyPermission{
			Read:  req.Permissions.Read,
			Write: req.Permissions.Write,
			Owner: req.Permissions.Owner,
		},
	}

	// Grant permissions using Garage Admin API
	result, err := h.adminService.AllowBucketKey(ctx, permRequest)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to grant permissions: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(result))
}
