package handlers

import (
	"time"

	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"

	"github.com/gofiber/fiber/v3"
)

// UserHandler handles user/key management operations using Garage Admin API
type UserHandler struct {
	adminService *services.GarageAdminService
}

// NewUserHandler creates a new user handler
func NewUserHandler(adminService *services.GarageAdminService) *UserHandler {
	return &UserHandler{
		adminService: adminService,
	}
}

// ListUsers lists all users/access keys
//
//	@Summary		List all users
//	@Description	Retrieves a list of all users/access keys
//	@Tags			Users
//	@Produce		json
//	@Success		200	{object}	models.APIResponse{data=models.UserListResponse}	"List of users retrieved successfully"
//	@Failure		500	{object}	models.APIResponse{error=models.APIError}			"Failed to list users"
//	@Router			/api/v1/users [get]
func (h *UserHandler) ListUsers(c fiber.Ctx) error {
	ctx := c.Context()

	keys, err := h.adminService.ListKeys(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to list users: "+err.Error()),
		)
	}

	// Convert to UserInfo format
	users := make([]models.UserInfo, 0, len(keys))
	for _, key := range keys {
		// Get full key info to retrieve bucket permissions
		keyInfo, err := h.adminService.GetKeyInfo(ctx, key.ID, false)
		if err != nil {
			// If we can't get full info, skip this key or use basic info
			continue
		}

		// Convert bucket permissions to frontend format
		bucketPermissions := convertBucketPermissionsToBucketPermissions(keyInfo.Buckets)

		// Determine status based on expiration
		status := "active"
		if keyInfo.Expired {
			status = "inactive"
		}

		users = append(users, models.UserInfo{
			AccessKeyID:       keyInfo.AccessKeyID,
			Name:              keyInfo.Name,
			CreatedAt:         keyInfo.Created,
			Status:            status,
			BucketPermissions: bucketPermissions,
			Expiration:        keyInfo.Expiration,
			Expired:           keyInfo.Expired,
		})
	}

	return c.JSON(models.SuccessResponse(models.UserListResponse{
		Users: users,
		Count: len(users),
	}))
}

// convertBucketPermissionsToBucketPermissions converts Garage bucket permissions to frontend BucketPermission format
func convertBucketPermissionsToBucketPermissions(buckets []models.KeyBucketInfo) []models.BucketPermission {
	permissions := make([]models.BucketPermission, 0, len(buckets))

	for _, bucket := range buckets {
		// Get bucket name from aliases
		var bucketName string
		if len(bucket.GlobalAliases) > 0 {
			bucketName = bucket.GlobalAliases[0]
		} else if len(bucket.LocalAliases) > 0 {
			bucketName = bucket.LocalAliases[0]
		} else {
			bucketName = bucket.ID
		}

		// Create bucket permission with simple read/write/owner flags
		permissions = append(permissions, models.BucketPermission{
			BucketID:   bucket.ID,
			BucketName: bucketName,
			Read:       bucket.Permissions.Read,
			Write:      bucket.Permissions.Write,
			Owner:      bucket.Permissions.Owner,
		})
	}

	return permissions
}

// CreateUser creates a new user/access key
//
//	@Summary		Create a new user
//	@Description	Creates a new user/access key with optional name
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			request	body		models.CreateUserRequest					true	"User creation request"
//	@Success		201		{object}	models.APIResponse{data=models.UserInfo}	"User created successfully"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}	"Invalid request body"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}	"Failed to create user"
//	@Router			/api/v1/users [post]
func (h *UserHandler) CreateUser(c fiber.Ctx) error {
	ctx := c.Context()

	var req models.CreateUserRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body: "+err.Error()),
		)
	}

	// Prepare create key request
	createReq := models.CreateKeyRequest{}
	if req.Name != "" {
		createReq.Name = &req.Name
	}

	// Create the key
	keyInfo, err := h.adminService.CreateKey(ctx, createReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to create user: "+err.Error()),
		)
	}

	// Convert bucket permissions to frontend format
	bucketPermissions := convertBucketPermissionsToBucketPermissions(keyInfo.Buckets)

	// Determine status
	status := "active"
	if keyInfo.Expired {
		status = "inactive"
	}

	// Convert to UserInfo format
	userInfo := models.UserInfo{
		AccessKeyID:       keyInfo.AccessKeyID,
		SecretKey:         keyInfo.SecretAccessKey,
		Name:              keyInfo.Name,
		CreatedAt:         keyInfo.Created,
		Status:            status,
		BucketPermissions: bucketPermissions,
		Expiration:        keyInfo.Expiration,
		Expired:           keyInfo.Expired,
	}

	return c.Status(fiber.StatusCreated).JSON(models.SuccessResponse(userInfo))
}

// DeleteUser deletes a user/access key
//
//	@Summary		Delete a user
//	@Description	Deletes a specific user/access key
//	@Tags			Users
//	@Produce		json
//	@Param			access_key	path		string										true	"Access key of the user to delete"
//	@Success		200			{object}	models.APIResponse{data=map[string]interface{}}	"User deleted successfully"
//	@Failure		400			{object}	models.APIResponse{error=models.APIError}			"Access key is required"
//	@Failure		500			{object}	models.APIResponse{error=models.APIError}			"Failed to delete user"
//	@Router			/api/v1/users/{access_key} [delete]
func (h *UserHandler) DeleteUser(c fiber.Ctx) error {
	ctx := c.Context()
	accessKey := c.Params("access_key")

	if accessKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Access key is required"),
		)
	}

	// Delete the key
	err := h.adminService.DeleteKey(ctx, accessKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to delete user: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(map[string]interface{}{
		"access_key": accessKey,
		"deleted":    true,
	}))
}

// GetUser retrieves information about a specific user/access key
//
//	@Summary		Get user information
//	@Description	Retrieves information about a specific user/access key
//	@Tags			Users
//	@Produce		json
//	@Param			access_key	path		string										true	"Access key of the user to retrieve"
//	@Success		200			{object}	models.APIResponse{data=models.UserInfo}	"User information retrieved successfully"
//	@Failure		400			{object}	models.APIResponse{error=models.APIError}	"Access key is required"
//	@Failure		500			{object}	models.APIResponse{error=models.APIError}	"Failed to get user info"
//	@Router			/api/v1/users/{access_key} [get]
func (h *UserHandler) GetUser(c fiber.Ctx) error {
	ctx := c.Context()
	accessKey := c.Params("access_key")

	if accessKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Access key is required"),
		)
	}

	// Get key information (without secret key)
	keyInfo, err := h.adminService.GetKeyInfo(ctx, accessKey, false)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get user info: "+err.Error()),
		)
	}

	// Convert bucket permissions to frontend format
	bucketPermissions := convertBucketPermissionsToBucketPermissions(keyInfo.Buckets)

	// Determine status
	status := "active"
	if keyInfo.Expired {
		status = "inactive"
	}

	// Convert to UserInfo format
	userInfo := models.UserInfo{
		AccessKeyID:       keyInfo.AccessKeyID,
		Name:              keyInfo.Name,
		CreatedAt:         keyInfo.Created,
		Status:            status,
		BucketPermissions: bucketPermissions,
		Expiration:        keyInfo.Expiration,
		Expired:           keyInfo.Expired,
	}

	return c.JSON(models.SuccessResponse(userInfo))
}

// UpdateUserPermissions updates user permissions
//
//	@Summary		Update user permissions
//	@Description	Updates the permissions and settings for a specific user/access key
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			access_key	path		string										true	"Access key of the user to update"
//	@Param			request		body		models.UpdateUserRequest					true	"User update request with new permissions"
//	@Success		200			{object}	models.APIResponse{data=models.UserInfo}	"User updated successfully"
//	@Failure		400			{object}	models.APIResponse{error=models.APIError}	"Access key is required or invalid request body"
//	@Failure		500			{object}	models.APIResponse{error=models.APIError}	"Failed to update user"
//	@Router			/api/v1/users/{access_key} [patch]
func (h *UserHandler) UpdateUserPermissions(c fiber.Ctx) error {
	ctx := c.Context()
	accessKey := c.Params("access_key")

	if accessKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Access key is required"),
		)
	}

	var req models.UpdateUserRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body: "+err.Error()),
		)
	}

	// Prepare update request
	updateReq := models.UpdateKeyRequest{}

	// Handle status change (activate/deactivate)
	if req.Status != nil {
		if *req.Status == "inactive" {
			// Deactivate by setting expiration to the past
			pastTime := time.Now().Add(-24 * time.Hour)
			updateReq.Expiration = &pastTime
			updateReq.NeverExpires = false
		} else if *req.Status == "active" {
			// Activate by removing expiration (set to never expire)
			updateReq.NeverExpires = true
		}
	}

	// Handle explicit expiration date setting
	if req.Expiration != nil && *req.Expiration != "" {
		expirationTime, err := time.Parse(time.RFC3339, *req.Expiration)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(
				models.ErrorResponse(models.ErrCodeBadRequest, "Invalid expiration date format: "+err.Error()),
			)
		}
		updateReq.Expiration = &expirationTime
		updateReq.NeverExpires = false
	}

	// Update the key
	keyInfo, err := h.adminService.UpdateKey(ctx, accessKey, updateReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to update user: "+err.Error()),
		)
	}

	// Convert bucket permissions to frontend format
	bucketPermissions := convertBucketPermissionsToBucketPermissions(keyInfo.Buckets)

	// Determine status
	status := "active"
	if keyInfo.Expired {
		status = "inactive"
	}

	// Convert to UserInfo format
	userInfo := models.UserInfo{
		AccessKeyID:       keyInfo.AccessKeyID,
		Name:              keyInfo.Name,
		CreatedAt:         keyInfo.Created,
		Status:            status,
		BucketPermissions: bucketPermissions,
		Expiration:        keyInfo.Expiration,
		Expired:           keyInfo.Expired,
	}

	return c.JSON(models.SuccessResponse(userInfo))
}
