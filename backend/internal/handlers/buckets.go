package handlers

import (
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"

	"github.com/gofiber/fiber/v3"
)

// BucketHandler handles bucket-related HTTP requests.
type BucketHandler struct {
	adminService services.AdminService
	s3Service    services.S3Storage
}

// NewBucketHandler creates a new bucket handler.
func NewBucketHandler(adminService services.AdminService, s3Service services.S3Storage) *BucketHandler {
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

		// Get detailed bucket info from Admin API to retrieve object count and size
		detailedInfo, err := h.adminService.GetBucketInfoByAlias(ctx, bucketName)
		if err != nil {
			// If we can't get detailed info, return basic info without stats
			buckets = append(buckets, models.BucketInfo{
				Name:         bucketName,
				CreationDate: adminBucket.Created,
				Region:       "",
			})
			continue
		}

		bucketInfo := models.BucketInfo{
			Name:          bucketName,
			CreationDate:  adminBucket.Created,
			Region:        "",
			ObjectCount:   &detailedInfo.Objects,
			Size:          &detailedInfo.Bytes,
			WebsiteAccess: detailedInfo.WebsiteAccess,
			WebsiteConfig: detailedInfo.WebsiteConfig,
			Quotas:        detailedInfo.Quotas,
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
//	@Param			payload	body		models.CreateBucketRequest										true	"Bucket creation payload"
//	@Success		201		{object}	models.APIResponse{data=object{bucket=string,message=string}}	"Bucket created successfully"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}						"Invalid request body or bucket name is required"
//	@Failure		409		{object}	models.APIResponse{error=models.APIError}						"Bucket already exists"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}						"Failed to create bucket"
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

	// Create the bucket
	createBucketReq := models.CreateBucketAdminRequest{
		GlobalAlias: &req.Name,
	}

	if _, err := h.adminService.CreateBucket(ctx, createBucketReq); err != nil {
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

	// Garage's AllowBucketKey is additive — false values are no-ops, not revokes.
	// To make this endpoint a true "set permissions" operation, split into Allow
	// for the requested-true perms and Deny for the requested-false perms.
	allow := models.BucketKeyPermission{
		Read:  req.Permissions.Read,
		Write: req.Permissions.Write,
		Owner: req.Permissions.Owner,
	}
	deny := models.BucketKeyPermission{
		Read:  !req.Permissions.Read,
		Write: !req.Permissions.Write,
		Owner: !req.Permissions.Owner,
	}

	var result *models.GarageBucketInfo

	if allow.Read || allow.Write || allow.Owner {
		r, err := h.adminService.AllowBucketKey(ctx, models.BucketKeyPermRequest{
			BucketID:    bucketInfo.ID,
			AccessKeyID: req.AccessKeyID,
			Permissions: allow,
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(
				models.ErrorResponse(models.ErrCodeInternalError, "Failed to grant permissions: "+err.Error()),
			)
		}
		result = r
	}

	if deny.Read || deny.Write || deny.Owner {
		r, err := h.adminService.DenyBucketKey(ctx, models.BucketKeyPermRequest{
			BucketID:    bucketInfo.ID,
			AccessKeyID: req.AccessKeyID,
			Permissions: deny,
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(
				models.ErrorResponse(models.ErrCodeInternalError, "Failed to revoke permissions: "+err.Error()),
			)
		}
		result = r
	}

	if result == nil {
		// Caller passed all-false on a key with no existing perms — nothing to do.
		// Fetch current bucket state to return a consistent response.
		r, err := h.adminService.GetBucketInfo(ctx, bucketInfo.ID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(
				models.ErrorResponse(models.ErrCodeInternalError, "Failed to fetch bucket info: "+err.Error()),
			)
		}
		result = r
	}

	return c.JSON(models.SuccessResponse(result))
}

// UpdateBucketWebsite updates the website access configuration for a bucket
//
//	@Summary		Update bucket website configuration
//	@Description	Enables or disables static website hosting for a bucket
//	@Tags			Buckets
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string												true	"Name of the bucket"
//	@Param			request	body		models.UpdateBucketWebsiteRequest				true	"Website configuration"
//	@Success		200		{object}	models.APIResponse{data=models.GarageBucketInfo}	"Website configuration updated"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}			"Invalid request"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}			"Bucket not found"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}			"Failed to update bucket"
//	@Router			/api/v1/buckets/{name}/website [put]
func (h *BucketHandler) UpdateBucketWebsite(c fiber.Ctx) error {
	ctx := c.Context()

	bucketName := c.Params("name")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	var req models.UpdateBucketWebsiteRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body: "+err.Error()),
		)
	}

	if req.Enabled && req.IndexDocument == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "indexDocument is required when enabling website access"),
		)
	}

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

	websiteAccess := &models.UpdateBucketWebsiteAccess{
		Enabled: req.Enabled,
	}
	if req.Enabled {
		websiteAccess.IndexDocument = &req.IndexDocument
		if req.ErrorDocument != "" {
			websiteAccess.ErrorDocument = &req.ErrorDocument
		}
	}

	updateReq := models.UpdateBucketRequest{
		WebsiteAccess: websiteAccess,
	}

	result, err := h.adminService.UpdateBucket(ctx, bucketInfo.ID, updateReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to update bucket website: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(result))
}

// UpdateBucketQuotas updates the quota settings for a bucket
//
//	@Summary		Update bucket quotas
//	@Description	Sets or clears the max size (bytes) and max object count quotas for a bucket. A null field clears that quota (unlimited).
//	@Tags			Buckets
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string												true	"Name of the bucket"
//	@Param			request	body		models.UpdateBucketQuotasRequest					true	"Quota configuration"
//	@Success		200		{object}	models.APIResponse{data=models.GarageBucketInfo}	"Quotas updated"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}			"Invalid request"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}			"Bucket not found"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}			"Failed to update bucket"
//	@Router			/api/v1/buckets/{name}/quotas [put]
func (h *BucketHandler) UpdateBucketQuotas(c fiber.Ctx) error {
	ctx := c.Context()

	bucketName := c.Params("name")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	var req models.UpdateBucketQuotasRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body: "+err.Error()),
		)
	}

	if req.MaxSize != nil && *req.MaxSize <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "maxSize must be greater than 0"),
		)
	}
	if req.MaxObjects != nil && *req.MaxObjects <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "maxObjects must be greater than 0"),
		)
	}

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

	updateReq := models.UpdateBucketRequest{
		Quotas: &models.BucketQuotas{
			MaxSize:    req.MaxSize,
			MaxObjects: req.MaxObjects,
		},
	}

	result, err := h.adminService.UpdateBucket(ctx, bucketInfo.ID, updateReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to update bucket quotas: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(result))
}
