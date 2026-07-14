package services

import (
	"context"
	"io"
	"time"

	"Noooste/garage-ui/internal/models"
)

// AdminService is the set of Garage Admin API operations used by HTTP handlers.
// It is implemented by *GarageV2AdminService in admin_v2.go. Kept narrow so that
// hand-rolled mocks in tests don't need to cover admin methods the handlers
// never call.
type AdminService interface {
	// Access keys
	ListKeys(ctx context.Context) ([]models.ListKeysResponseItem, error)
	CreateKey(ctx context.Context, req models.CreateKeyRequest) (*models.GarageKeyInfo, error)
	GetKeyInfo(ctx context.Context, keyID string, showSecret bool) (*models.GarageKeyInfo, error)
	UpdateKey(ctx context.Context, keyID string, req models.UpdateKeyRequest) (*models.GarageKeyInfo, error)
	DeleteKey(ctx context.Context, keyID string) error

	// Buckets
	ListBuckets(ctx context.Context) ([]models.ListBucketsResponseItem, error)
	GetBucketInfo(ctx context.Context, bucketID string) (*models.GarageBucketInfo, error)
	GetBucketInfoByAlias(ctx context.Context, globalAlias string) (*models.GarageBucketInfo, error)
	CreateBucket(ctx context.Context, req models.CreateBucketAdminRequest) (*models.GarageBucketInfo, error)
	UpdateBucket(ctx context.Context, bucketID string, req models.UpdateBucketRequest) (*models.GarageBucketInfo, error)
	DeleteBucket(ctx context.Context, bucketID string) error
	AllowBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error)
	DenyBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error)

	// Cluster
	GetClusterHealth(ctx context.Context) (*models.ClusterHealth, error)
	GetClusterStatus(ctx context.Context) (*models.ClusterStatus, error)
	GetClusterStatistics(ctx context.Context) (*models.ClusterStatistics, error)
	GetNodeInfo(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error)
	GetNodeStatistics(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error)

	// Monitoring
	HealthCheck(ctx context.Context) error
	GetMetrics(ctx context.Context) (string, error)
}

// S3Storage is the set of S3 operations used by HTTP handlers. It is
// implemented by *S3Service in s3.go. Methods on *S3Service that are not
// called by handlers (ListBuckets, CreateBucket, DeleteBucket,
// GetBucketStatistics) are intentionally excluded.
type S3Storage interface {
	ListObjects(ctx context.Context, bucketName, prefix string, maxKeys int, continuationToken string) (*models.ObjectListResponse, error)
	SearchObjects(ctx context.Context, bucketName, prefix, search string) (*models.ObjectListResponse, error)
	UploadObject(ctx context.Context, bucketName, key string, body io.Reader, contentType string) (*models.ObjectUploadResponse, error)
	CreateDirectoryMarker(ctx context.Context, bucketName, key string) (*models.ObjectUploadResponse, error)
	GetObject(ctx context.Context, bucketName, key string) (io.ReadCloser, *models.ObjectInfo, error)
	ObjectExists(ctx context.Context, bucketName, key string) (bool, error)
	DeleteObject(ctx context.Context, bucketName, key string) error
	GetObjectMetadata(ctx context.Context, bucketName, key string) (*models.ObjectInfo, error)
	GetPresignedURL(ctx context.Context, bucketName, key string, expiresIn time.Duration) (string, error)
	DeleteMultipleObjects(ctx context.Context, bucketName string, keys []string) (int, error)
	DeleteObjectsByPrefix(ctx context.Context, bucketName, prefix string) (int, error)
	UploadMultipleObjects(ctx context.Context, bucketName string, files []struct {
		Key         string
		Body        io.Reader
		ContentType string
	}) []UploadResult
}

// Compile-time guarantees that the concrete services implement the interfaces.
var (
	_ AdminService = (*GarageV2AdminService)(nil)
	_ AdminService = (*GarageV1AdminService)(nil)
	_ S3Storage    = (*S3Service)(nil)
)
