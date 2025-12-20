package models

import "time"

// DashboardMetrics represents aggregated metrics for the dashboard
type DashboardMetrics struct {
	TotalSize     int64         `json:"totalSize"`
	ObjectCount   int64         `json:"objectCount"`
	BucketCount   int           `json:"bucketCount"`
	UsageByBucket []BucketUsage `json:"usageByBucket"`
}

// BucketUsage represents storage usage for a single bucket
type BucketUsage struct {
	BucketName  string  `json:"bucketName"`
	Size        int64   `json:"size"`
	ObjectCount int64   `json:"objectCount"`
	Percentage  float64 `json:"percentage"`
}

// APIResponse is the standard response structure for all API endpoints
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError represents an error in the API response
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// BucketInfo represents information about a bucket
type BucketInfo struct {
	Name         string    `json:"name"`
	CreationDate time.Time `json:"creation_date"`
	ObjectCount  *int64    `json:"object_count,omitempty"`
	Size         *int64    `json:"size,omitempty"`
	Region       string    `json:"region,omitempty"`
}

// BucketListResponse represents a list of buckets
type BucketListResponse struct {
	Buckets []BucketInfo `json:"buckets"`
	Count   int          `json:"count"`
}

// ObjectInfo represents information about an object
type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag"`
	ContentType  string    `json:"content_type,omitempty"`
	StorageClass string    `json:"storage_class,omitempty"`
}

// ObjectListResponse represents a list of objects in a bucket
type ObjectListResponse struct {
	Bucket                string       `json:"bucket"`
	Objects               []ObjectInfo `json:"objects"`
	Prefixes              []string     `json:"prefixes"`
	Count                 int          `json:"count"`
	IsTruncated           bool         `json:"is_truncated"`
	NextContinuationToken string       `json:"next_continuation_token,omitempty"`
}

// ObjectUploadResponse represents the response after uploading an object
type ObjectUploadResponse struct {
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	ETag        string `json:"etag"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// ObjectUploadMultipleResponse represents the response after uploading multiple objects
type ObjectUploadMultipleResponse struct {
	Bucket       string                     `json:"bucket"`
	TotalFiles   int                        `json:"total_files"`
	SuccessCount int                        `json:"success_count"`
	FailureCount int                        `json:"failure_count"`
	SuccessFiles []ObjectUploadResult       `json:"success_files"`
	FailedFiles  []ObjectUploadFailedResult `json:"failed_files,omitempty"`
}

// ObjectUploadResult represents a successful upload result
type ObjectUploadResult struct {
	Key         string `json:"key"`
	ETag        string `json:"etag"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type,omitempty"`
}

// ObjectUploadFailedResult represents a failed upload result
type ObjectUploadFailedResult struct {
	Key         string `json:"key"`
	Error       string `json:"error"`
	ContentType string `json:"content_type,omitempty"`
}

// ObjectDeleteResponse represents the response after deleting an object
type ObjectDeleteResponse struct {
	Bucket  string `json:"bucket"`
	Key     string `json:"key"`
	Deleted bool   `json:"deleted"`
}

// UserInfo represents information about a Garage user (key pair)
type UserInfo struct {
	AccessKeyID       string             `json:"accessKeyId"`
	Name              string             `json:"name"`
	SecretKey         *string            `json:"secretKey,omitempty"`
	CreatedAt         *time.Time         `json:"createdAt,omitempty"`
	LastUsed          *time.Time         `json:"lastUsed,omitempty"`
	Status            string             `json:"status"`      // "active" or "inactive"
	BucketPermissions []BucketPermission `json:"permissions"` // Array of bucket permissions
	Expiration        *time.Time         `json:"expiration,omitempty"`
	Expired           bool               `json:"expired"`
}

// BucketPermission represents permissions for a specific bucket
type BucketPermission struct {
	BucketID   string `json:"bucketId"`
	BucketName string `json:"bucketName"`
	Read       bool   `json:"read"`
	Write      bool   `json:"write"`
	Owner      bool   `json:"owner"`
}

// Permission represents a permission entry for access control (legacy/deprecated)
type Permission struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
	Effect   string   `json:"effect"` // "Allow" or "Deny"
}

type PresignedURLResponse struct {
	URL       string `json:"url"`
	ExpiresIn int64  `json:"expires_in"` // in seconds
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
}

type ObjectDeleteMultipleResponse struct {
	Bucket  string   `json:"bucket"`
	Deleted int      `json:"deleted"`
	Keys    []string `json:"keys"`
}

// UserListResponse represents a list of users/keys
type UserListResponse struct {
	Users []UserInfo `json:"users"`
	Count int        `json:"count"`
}

// Helper functions to create standard responses

// SuccessResponse creates a successful API response
func SuccessResponse(data interface{}) APIResponse {
	return APIResponse{
		Success: true,
		Data:    data,
		Error:   nil,
	}
}

// ErrorResponse creates an error API response
func ErrorResponse(code, message string) APIResponse {
	return APIResponse{
		Success: false,
		Data:    nil,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	}
}

// Common error codes
const (
	ErrCodeBadRequest        = "BAD_REQUEST"
	ErrCodeUnauthorized      = "UNAUTHORIZED"
	ErrCodeForbidden         = "FORBIDDEN"
	ErrCodeNotFound          = "NOT_FOUND"
	ErrCodeConflict          = "CONFLICT"
	ErrCodeInternalError     = "INTERNAL_ERROR"
	ErrCodeBucketExists      = "BUCKET_ALREADY_EXISTS"
	ErrCodeBucketNotFound    = "BUCKET_NOT_FOUND"
	ErrCodeObjectNotFound    = "OBJECT_NOT_FOUND"
	ErrCodeInvalidBucketName = "INVALID_BUCKET_NAME"
	ErrCodeInvalidObjectKey  = "INVALID_OBJECT_KEY"
	ErrCodeUploadFailed      = "UPLOAD_FAILED"
	ErrCodeDeleteFailed      = "DELETE_FAILED"
	ErrCodeListFailed        = "LIST_FAILED"
)
