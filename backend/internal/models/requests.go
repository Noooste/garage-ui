package models

// CreateBucketRequest represents a request to create a new bucket
type CreateBucketRequest struct {
	Name   string `json:"name" validate:"required"`
	Region string `json:"region,omitempty"`
}

// GrantBucketPermissionRequest represents a request to grant permissions on a bucket
type GrantBucketPermissionRequest struct {
	AccessKeyID string              `json:"accessKeyId" validate:"required"`
	Permissions BucketKeyPermission `json:"permissions" validate:"required"`
}

// DeleteBucketRequest represents a request to delete a bucket
type DeleteBucketRequest struct {
	Name string `json:"name" validate:"required"`
}

// ListObjectsRequest represents a request to list objects in a bucket
type ListObjectsRequest struct {
	Bucket  string `json:"bucket" validate:"required"`
	Prefix  string `json:"prefix,omitempty"`
	MaxKeys int    `json:"max_keys,omitempty"`
	Marker  string `json:"marker,omitempty"`
}

// UploadObjectRequest represents metadata for an object upload
type UploadObjectRequest struct {
	Bucket      string `json:"bucket" validate:"required"`
	Key         string `json:"key" validate:"required"`
	ContentType string `json:"content_type,omitempty"`
}

// DeleteObjectRequest represents a request to delete an object
type DeleteObjectRequest struct {
	Bucket string `json:"bucket" validate:"required"`
	Key    string `json:"key" validate:"required"`
}

// GetObjectRequest represents a request to get/download an object
type GetObjectRequest struct {
	Bucket string `json:"bucket" validate:"required"`
	Key    string `json:"key" validate:"required"`
}

// CreateUserRequest represents a request to create a new user/key
type CreateUserRequest struct {
	Name string `json:"name,omitempty"`
}

// DeleteUserRequest represents a request to delete a user/key
type DeleteUserRequest struct {
	AccessKey string `json:"access_key" validate:"required"`
}

// UpdateUserRequest represents a request to update user permissions
type UpdateUserRequest struct {
	Status     *string `json:"status,omitempty"`     // "active" or "inactive"
	Expiration *string `json:"expiration,omitempty"` // ISO 8601 date string
}
