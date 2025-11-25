package services

import (
	"context"
	"fmt"
	"io"
	"time"

	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/pkg/utils"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Service handles all S3 operations with Garage
type S3Service struct {
	client       *s3.Client
	config       *config.GarageConfig
	adminService *GarageAdminService
}

// NewS3Service creates a new S3 service instance
func NewS3Service(cfg *config.GarageConfig, adminService *GarageAdminService) *S3Service {
	// Create AWS credentials from Garage config (default/fallback credentials)
	creds := credentials.NewStaticCredentialsProvider(
		cfg.AccessKey,
		cfg.SecretKey,
		"", // session token (not used for Garage)
	)

	// Configure S3 client for Garage
	s3Config := aws.Config{
		Region:      cfg.Region,
		Credentials: creds,
	}

	// Create S3 client with custom endpoint resolver for Garage
	client := s3.NewFromConfig(s3Config, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = cfg.ForcePathStyle
	})

	return &S3Service{
		client:       client,
		config:       cfg,
		adminService: adminService,
	}
}

// getBucketCredentials retrieves credentials for a specific bucket
// It checks the cache first, then queries the Garage Admin API
func (s *S3Service) getBucketCredentials(ctx context.Context, bucketName string) (aws.CredentialsProvider, error) {
	cacheKey := fmt.Sprintf("key:%s", bucketName)
	cacheData := utils.GlobalCache.Get(cacheKey)

	if cacheData != nil {
		return cacheData.(aws.CredentialsProvider), nil
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

	// Create credentials provider
	credential := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")

	// Cache credentials for 1 hour
	utils.GlobalCache.Set(cacheKey, credential, time.Hour)

	return credential, nil
}

// getS3Client creates an S3 client for a specific bucket with dynamic credentials
func (s *S3Service) getS3Client(ctx context.Context, bucketName string) (*s3.Client, error) {
	creds, err := s.getBucketCredentials(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("cannot get credentials for bucket %s: %w", bucketName, err)
	}

	// AWS config
	awsConfig := aws.Config{
		Credentials: creds,
		Region:      s.config.Region,
	}

	// Build S3 client with BaseEndpoint for Garage
	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(s.config.Endpoint)
		o.UsePathStyle = s.config.ForcePathStyle
		o.EndpointResolver = s3.EndpointResolverFunc(func(region string, opts s3.EndpointResolverOptions) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           s.config.Endpoint,
				SigningRegion: s.config.Region,
			}, nil
		})
	})

	return client, nil
}

// ListBuckets retrieves all buckets from Garage
func (s *S3Service) ListBuckets(ctx context.Context) (*models.BucketListResponse, error) {
	// Call S3 ListBuckets API
	result, err := s.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	// Convert S3 buckets to our model
	buckets := make([]models.BucketInfo, 0, len(result.Buckets))
	for _, bucket := range result.Buckets {
		buckets = append(buckets, models.BucketInfo{
			Name:         aws.ToString(bucket.Name),
			CreationDate: aws.ToTime(bucket.CreationDate),
		})
	}

	return &models.BucketListResponse{
		Buckets: buckets,
		Count:   len(buckets),
	}, nil
}

// CreateBucket creates a new bucket in Garage
func (s *S3Service) CreateBucket(ctx context.Context, bucketName string) error {
	client, err := s.getS3Client(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to get S3 client for bucket %s: %w", bucketName, err)
	}

	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}

	// Call S3 CreateBucket API
	_, err = client.CreateBucket(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
	}

	return nil
}

// DeleteBucket deletes a bucket from Garage
func (s *S3Service) DeleteBucket(ctx context.Context, bucketName string) error {
	client, err := s.getS3Client(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to get S3 client for bucket %s: %w", bucketName, err)
	}

	// Call S3 DeleteBucket API
	_, err = client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete bucket %s: %w", bucketName, err)
	}

	return nil
}

// ListObjects lists objects in a bucket with optional prefix filter and pagination
func (s *S3Service) ListObjects(ctx context.Context, bucketName, prefix string, maxKeys int, continuationToken string) (*models.ObjectListResponse, error) {
	// Get bucket-specific S3 client
	client, err := s.getS3Client(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 client for bucket %s: %w", bucketName, err)
	}

	// Set default max keys if not specified
	if maxKeys <= 0 {
		maxKeys = 100
	}

	// Create list objects input
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucketName),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int32(int32(maxKeys)),
	}

	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}

	if continuationToken != "" {
		input.ContinuationToken = aws.String(continuationToken)
	}

	// Call S3 ListObjectsV2 API
	result, err := client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects in bucket %s: %w", bucketName, err)
	}

	// Convert S3 objects to our model
	objects := make([]models.ObjectInfo, 0, len(result.Contents))
	for _, obj := range result.Contents {
		objects = append(objects, models.ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
			ETag:         aws.ToString(obj.ETag),
			StorageClass: string(obj.StorageClass),
		})
	}

	// Extract common prefixes (folders/directories)
	prefixes := make([]string, 0, len(result.CommonPrefixes))
	for _, p := range result.CommonPrefixes {
		prefixes = append(prefixes, aws.ToString(p.Prefix))
	}

	return &models.ObjectListResponse{
		Bucket:                bucketName,
		Objects:               objects,
		Prefixes:              prefixes,
		Count:                 len(objects),
		IsTruncated:           aws.ToBool(result.IsTruncated),
		NextContinuationToken: aws.ToString(result.NextContinuationToken),
	}, nil
}

// UploadObject uploads an object to a bucket
func (s *S3Service) UploadObject(ctx context.Context, bucketName, key string, body io.Reader, contentType string) (*models.ObjectUploadResponse, error) {
	// Get bucket-specific S3 client
	client, err := s.getS3Client(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 client for bucket %s: %w", bucketName, err)
	}

	// Create put object input
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   body,
	}

	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	// Call S3 PutObject API
	result, err := client.PutObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to upload object %s to bucket %s: %w", key, bucketName, err)
	}

	// Get object metadata to return size
	headResult, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})

	var size int64
	if err == nil {
		size = aws.ToInt64(headResult.ContentLength)
	}

	return &models.ObjectUploadResponse{
		Bucket:      bucketName,
		Key:         key,
		ETag:        aws.ToString(result.ETag),
		Size:        size,
		ContentType: contentType,
	}, nil
}

// GetObject retrieves an object from a bucket
func (s *S3Service) GetObject(ctx context.Context, bucketName, key string) (io.ReadCloser, *models.ObjectInfo, error) {
	// Call S3 GetObject API
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get object %s from bucket %s: %w", key, bucketName, err)
	}

	// Create object info
	objectInfo := &models.ObjectInfo{
		Key:          key,
		Size:         aws.ToInt64(result.ContentLength),
		LastModified: aws.ToTime(result.LastModified),
		ETag:         aws.ToString(result.ETag),
		ContentType:  aws.ToString(result.ContentType),
	}

	return result.Body, objectInfo, nil
}

// DeleteObject deletes an object from a bucket
func (s *S3Service) DeleteObject(ctx context.Context, bucketName, key string) error {
	// Call S3 DeleteObject API
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object %s from bucket %s: %w", key, bucketName, err)
	}

	return nil
}

// ObjectExists checks if an object exists in a bucket
func (s *S3Service) ObjectExists(ctx context.Context, bucketName, key string) (bool, error) {
	// Get bucket-specific S3 client
	client, err := s.getS3Client(ctx, bucketName)
	if err != nil {
		return false, fmt.Errorf("failed to get S3 client for bucket %s: %w", bucketName, err)
	}

	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

// GetObjectMetadata retrieves metadata for an object without downloading it
func (s *S3Service) GetObjectMetadata(ctx context.Context, bucketName, key string) (*models.ObjectInfo, error) {
	// Get bucket-specific S3 client
	client, err := s.getS3Client(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 client for bucket %s: %w", bucketName, err)
	}

	result, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata for object %s in bucket %s: %w", key, bucketName, err)
	}

	return &models.ObjectInfo{
		Key:          key,
		Size:         aws.ToInt64(result.ContentLength),
		LastModified: aws.ToTime(result.LastModified),
		ETag:         aws.ToString(result.ETag),
		ContentType:  aws.ToString(result.ContentType),
	}, nil
}

// DeleteMultipleObjects deletes multiple objects from a bucket
func (s *S3Service) DeleteMultipleObjects(ctx context.Context, bucketName string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	// Get bucket-specific S3 client
	client, err := s.getS3Client(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to get S3 client for bucket %s: %w", bucketName, err)
	}

	// Create delete objects for batch deletion
	objects := make([]types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = types.ObjectIdentifier{
			Key: aws.String(key),
		}
	}

	// Call S3 DeleteObjects API (batch delete)
	_, err = client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &types.Delete{
			Objects: objects,
			Quiet:   aws.Bool(false),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete multiple objects from bucket %s: %w", bucketName, err)
	}

	return nil
}

// GetPresignedURL generates a pre-signed URL for temporary access to an object
// This is useful for sharing files without exposing credentials
func (s *S3Service) GetPresignedURL(ctx context.Context, bucketName, key string, expiresIn time.Duration) (string, error) {
	// Get bucket-specific S3 client
	client, err := s.getS3Client(ctx, bucketName)
	if err != nil {
		return "", fmt.Errorf("failed to get S3 client for bucket %s: %w", bucketName, err)
	}

	// Create presign client
	presignClient := s3.NewPresignClient(client)

	// Generate presigned GET request
	presignResult, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiresIn
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL for %s/%s: %w", bucketName, key, err)
	}

	return presignResult.URL, nil
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

	// Get bucket-specific S3 client once for all uploads
	client, err := s.getS3Client(ctx, bucketName)
	if err != nil {
		// If we can't get the client, all uploads fail
		for i := range files {
			results[i] = UploadResult{
				Key:     files[i].Key,
				Success: false,
				Error:   fmt.Errorf("failed to get S3 client for bucket %s: %w", bucketName, err),
			}
		}
		return results
	}

	// Upload each file
	for i, file := range files {
		// Create put object input
		input := &s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(file.Key),
			Body:   file.Body,
		}

		if file.ContentType != "" {
			input.ContentType = aws.String(file.ContentType)
		}

		// Attempt upload
		result, err := client.PutObject(ctx, input)
		if err != nil {
			results[i] = UploadResult{
				Key:         file.Key,
				Success:     false,
				Error:       fmt.Errorf("failed to upload object %s: %w", file.Key, err),
				ContentType: file.ContentType,
			}
			continue
		}

		// Get object metadata to return size
		headResult, err := client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(file.Key),
		})

		var size int64
		if err == nil {
			size = aws.ToInt64(headResult.ContentLength)
		}

		results[i] = UploadResult{
			Key:         file.Key,
			Success:     true,
			Error:       nil,
			ETag:        aws.ToString(result.ETag),
			Size:        size,
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
