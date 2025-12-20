package services

import (
	"context"
	"fmt"
	"io"
	"time"

	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/pkg/utils"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Service handles all S3 operations with Garage using MinIO SDK
type S3Service struct {
	client       *minio.Client
	config       *config.GarageConfig
	adminService *GarageAdminService
}

// NewS3Service creates a new S3 service instance using MinIO SDK
func NewS3Service(cfg *config.GarageConfig, adminService *GarageAdminService) *S3Service {
	// Create MinIO client for Garage
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		//Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		panic(fmt.Errorf("failed to create MinIO client: %w", err))
	}

	return &S3Service{
		client:       client,
		config:       cfg,
		adminService: adminService,
	}
}

// getBucketCredentials retrieves credentials for a specific bucket
// It checks the cache first, then queries the Garage Admin API
func (s *S3Service) getBucketCredentials(ctx context.Context, bucketName string) (*credentials.Credentials, error) {
	cacheKey := fmt.Sprintf("key:%s", bucketName)
	cacheData := utils.GlobalCache.Get(cacheKey)

	if cacheData != nil {
		return cacheData.(*credentials.Credentials), nil
	}

	// Get bucket info from Garage Admin API
	bucketInfo, err := s.adminService.GetBucketInfoByAlias(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket info: %w", err)
	}

	// Find a key with read and write permissions
	var accessKeyID, secretAccessKey string
	for _, keyInfo := range bucketInfo.Keys {
		if !keyInfo.Permissions.Read || !keyInfo.Permissions.Write {
			continue
		}

		// Get key details with secret
		keyDetails, err := s.adminService.GetKeyInfo(ctx, keyInfo.AccessKeyID, true)
		if err != nil {
			return nil, fmt.Errorf("failed to get key info: %w", err)
		}

		if keyDetails.SecretAccessKey != nil {
			accessKeyID = keyDetails.AccessKeyID
			secretAccessKey = *keyDetails.SecretAccessKey
			break
		}
	}

	if accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("no valid credentials found for bucket %s", bucketName)
	}

	// Create credentials
	creds := credentials.NewStaticV4(accessKeyID, secretAccessKey, "")

	// Cache credentials for 1 hour
	utils.GlobalCache.Set(cacheKey, creds, time.Hour)

	return creds, nil
}

// getMinioClient creates a MinIO client for a specific bucket with dynamic credentials
func (s *S3Service) getMinioClient(ctx context.Context, bucketName string) (*minio.Client, error) {
	creds, err := s.getBucketCredentials(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("cannot get credentials for bucket %s: %w", bucketName, err)
	}

	// Create MinIO client with bucket-specific credentials
	client, err := minio.New(s.config.Endpoint, &minio.Options{
		Creds:  creds,
		Secure: s.config.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client for bucket %s: %w", bucketName, err)
	}

	return client, nil
}

// ListBuckets retrieves all buckets from Garage
func (s *S3Service) ListBuckets(ctx context.Context) (*models.BucketListResponse, error) {
	// Call MinIO ListBuckets API
	bucketInfos, err := s.client.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	// Convert MinIO buckets to our model
	buckets := make([]models.BucketInfo, 0, len(bucketInfos))
	for _, bucket := range bucketInfos {
		buckets = append(buckets, models.BucketInfo{
			Name:         bucket.Name,
			CreationDate: bucket.CreationDate,
		})
	}

	return &models.BucketListResponse{
		Buckets: buckets,
		Count:   len(buckets),
	}, nil
}

// CreateBucket creates a new bucket in Garage
func (s *S3Service) CreateBucket(ctx context.Context, bucketName string) error {
	client, err := s.getMinioClient(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	// Call MinIO MakeBucket API
	err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
		Region: s.config.Region,
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
	}

	return nil
}

// DeleteBucket deletes a bucket from Garage
func (s *S3Service) DeleteBucket(ctx context.Context, bucketName string) error {
	client, err := s.getMinioClient(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	// Call MinIO RemoveBucket API
	err = client.RemoveBucket(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to delete bucket %s: %w", bucketName, err)
	}

	return nil
}

// ListObjects lists objects in a bucket with optional prefix filter and pagination
func (s *S3Service) ListObjects(ctx context.Context, bucketName, prefix string, maxKeys int, continuationToken string) (*models.ObjectListResponse, error) {
	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	// Set default max keys if not specified
	if maxKeys <= 0 {
		maxKeys = 1000
	}

	// Use ListObjectsV2 for proper pagination support
	opts := minio.ListObjectsOptions{
		Prefix:     prefix,
		Recursive:  false,
		MaxKeys:    maxKeys,
		StartAfter: continuationToken,
		UseV1:      false,
	}

	objects := make([]models.ObjectInfo, 0)
	prefixesMap := make(map[string]bool)

	var lastKey string
	isTruncated := false
	itemCount := 0

	// List objects using the channel-based API
	for object := range client.ListObjects(ctx, bucketName, opts) {
		if object.Err != nil {
			return nil, fmt.Errorf("failed to list objects in bucket %s: %w", bucketName, object.Err)
		}

		// Check if this is a prefix (directory)
		if len(object.Key) > 0 && object.Key[len(object.Key)-1:] == "/" && object.Size == 0 {
			prefixesMap[object.Key] = true
			continue
		}

		// Track the last key for pagination
		lastKey = object.Key

		// Add to objects list
		objects = append(objects, models.ObjectInfo{
			Key:          object.Key,
			Size:         object.Size,
			LastModified: object.LastModified,
			ETag:         object.ETag,
			ContentType:  object.ContentType,
			StorageClass: object.StorageClass,
		})

		itemCount++
		if itemCount >= maxKeys {
			isTruncated = true
			break
		}
	}

	// Convert prefixes map to slice
	prefixList := make([]string, 0, len(prefixesMap))
	for p := range prefixesMap {
		prefixList = append(prefixList, p)
	}

	// Prepare next continuation token
	var nextToken string
	if isTruncated && lastKey != "" {
		nextToken = lastKey
	}

	return &models.ObjectListResponse{
		Bucket:                bucketName,
		Objects:               objects,
		Prefixes:              prefixList,
		Count:                 len(objects),
		IsTruncated:           isTruncated,
		NextContinuationToken: nextToken,
	}, nil
}

// UploadObject uploads an object to a bucket
func (s *S3Service) UploadObject(ctx context.Context, bucketName, key string, body io.Reader, contentType string) (*models.ObjectUploadResponse, error) {
	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	// Upload options
	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}

	// Call MinIO PutObject API
	info, err := client.PutObject(ctx, bucketName, key, body, -1, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to upload object %s to bucket %s: %w", key, bucketName, err)
	}

	return &models.ObjectUploadResponse{
		Bucket:      bucketName,
		Key:         key,
		ETag:        info.ETag,
		Size:        info.Size,
		ContentType: contentType,
	}, nil
}

// GetObject retrieves an object from a bucket
func (s *S3Service) GetObject(ctx context.Context, bucketName, key string) (io.ReadCloser, *models.ObjectInfo, error) {
	// Call MinIO GetObject API
	object, err := s.client.GetObject(ctx, bucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get object %s from bucket %s: %w", key, bucketName, err)
	}

	// Get object info
	stat, err := object.Stat()
	if err != nil {
		object.Close()
		return nil, nil, fmt.Errorf("failed to get object info for %s in bucket %s: %w", key, bucketName, err)
	}

	// Create object info
	objectInfo := &models.ObjectInfo{
		Key:          key,
		Size:         stat.Size,
		LastModified: stat.LastModified,
		ETag:         stat.ETag,
		ContentType:  stat.ContentType,
	}

	return object, objectInfo, nil
}

// DeleteObject deletes an object from a bucket
func (s *S3Service) DeleteObject(ctx context.Context, bucketName, key string) error {
	// Call MinIO RemoveObject API
	err := s.client.RemoveObject(ctx, bucketName, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object %s from bucket %s: %w", key, bucketName, err)
	}

	return nil
}

// ObjectExists checks if an object exists in a bucket
func (s *S3Service) ObjectExists(ctx context.Context, bucketName, key string) (bool, error) {
	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName)
	if err != nil {
		return false, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	_, err = client.StatObject(ctx, bucketName, key, minio.StatObjectOptions{})
	if err != nil {
		// Check if error is "object not found"
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if object exists: %w", err)
	}
	return true, nil
}

// GetObjectMetadata retrieves metadata for an object without downloading it
func (s *S3Service) GetObjectMetadata(ctx context.Context, bucketName, key string) (*models.ObjectInfo, error) {
	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	stat, err := client.StatObject(ctx, bucketName, key, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata for object %s in bucket %s: %w", key, bucketName, err)
	}

	return &models.ObjectInfo{
		Key:          key,
		Size:         stat.Size,
		LastModified: stat.LastModified,
		ETag:         stat.ETag,
		ContentType:  stat.ContentType,
	}, nil
}

// DeleteMultipleObjects deletes multiple objects from a bucket
func (s *S3Service) DeleteMultipleObjects(ctx context.Context, bucketName string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	// Create channel for objects to delete
	objectsCh := make(chan minio.ObjectInfo)

	// Send objects to delete in a goroutine
	go func() {
		defer close(objectsCh)
		for _, key := range keys {
			objectsCh <- minio.ObjectInfo{
				Key: key,
			}
		}
	}()

	// Call MinIO RemoveObjects API (batch delete)
	errorCh := client.RemoveObjects(ctx, bucketName, objectsCh, minio.RemoveObjectsOptions{})

	// Check for errors
	for err := range errorCh {
		if err.Err != nil {
			return fmt.Errorf("failed to delete object %s from bucket %s: %w", err.ObjectName, bucketName, err.Err)
		}
	}

	return nil
}

// GetPresignedURL generates a pre-signed URL for temporary access to an object
// This is useful for sharing files without exposing credentials
func (s *S3Service) GetPresignedURL(ctx context.Context, bucketName, key string, expiresIn time.Duration) (string, error) {
	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName)
	if err != nil {
		return "", fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	// Generate presigned GET URL
	presignedURL, err := client.PresignedGetObject(ctx, bucketName, key, expiresIn, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL for %s/%s: %w", bucketName, key, err)
	}

	return presignedURL.String(), nil
}

// UploadResult represents the result of a single file upload
type UploadResult struct {
	Key         string
	Success     bool
	Error       error
	ETag        string
	Size        int64
	ContentType string
}

// UploadMultipleObjects uploads multiple objects to a bucket
// It handles uploads in batches to respect any S3/Garage limits
// Returns results for each file, including both successes and failures
func (s *S3Service) UploadMultipleObjects(ctx context.Context, bucketName string, files []struct {
	Key         string
	Body        io.Reader
	ContentType string
}) []UploadResult {
	results := make([]UploadResult, len(files))

	// Get bucket-specific MinIO client once for all uploads
	client, err := s.getMinioClient(ctx, bucketName)
	if err != nil {
		// If we can't get the client, all uploads fail
		for i := range files {
			results[i] = UploadResult{
				Key:     files[i].Key,
				Success: false,
				Error:   fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err),
			}
		}
		return results
	}

	// Upload each file
	for i, file := range files {
		// Upload options
		opts := minio.PutObjectOptions{
			ContentType: file.ContentType,
		}

		// Attempt upload
		info, err := client.PutObject(ctx, bucketName, file.Key, file.Body, -1, opts)
		if err != nil {
			results[i] = UploadResult{
				Key:         file.Key,
				Success:     false,
				Error:       fmt.Errorf("failed to upload object %s: %w", file.Key, err),
				ContentType: file.ContentType,
			}
			continue
		}

		results[i] = UploadResult{
			Key:         file.Key,
			Success:     true,
			Error:       nil,
			ETag:        info.ETag,
			Size:        info.Size,
			ContentType: file.ContentType,
		}
	}

	return results
}

// BucketStatistics holds statistical information about a bucket
type BucketStatistics struct {
	ObjectCount int64
	TotalSize   int64
}

// GetBucketStatistics retrieves bucket statistics from Garage Admin API
// This is much more efficient than iterating through all objects
func (s *S3Service) GetBucketStatistics(ctx context.Context, bucketName string) (*BucketStatistics, error) {
	// Get bucket info from Garage Admin API which includes object count and size
	bucketInfo, err := s.adminService.GetBucketInfoByAlias(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket info for %s: %w", bucketName, err)
	}

	// Return statistics from Admin API
	return &BucketStatistics{
		ObjectCount: bucketInfo.Objects,
		TotalSize:   bucketInfo.Bytes,
	}, nil
}
