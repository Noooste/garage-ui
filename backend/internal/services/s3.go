package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
	logpkg "Noooste/garage-ui/pkg/logger"
	"Noooste/garage-ui/pkg/utils"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Service handles all S3 operations with Garage using MinIO SDK
type S3Service struct {
	client       *minio.Client
	config       *config.GarageConfig
	adminService AdminService
}

// NewS3Service creates a new S3 service instance using MinIO SDK
func NewS3Service(cfg *config.GarageConfig, adminService AdminService) *S3Service {
	// Create MinIO client for Garage
	// trim http or https from endpoint
	if strings.HasPrefix(cfg.Endpoint, "http://") {
		cfg.Endpoint = strings.TrimPrefix(cfg.Endpoint, "http://")
	}

	if strings.HasPrefix(cfg.Endpoint, "https://") {
		cfg.Endpoint = strings.TrimPrefix(cfg.Endpoint, "https://")
		cfg.UseSSL = true
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Secure: cfg.UseSSL,
		Region: cfg.Region,
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

// Operation is a bitmask of S3 permissions a call needs. Combine with bitwise
// OR (e.g. OpRead | OpWrite) when more than one is required.
type Operation byte

const (
	OpRead  Operation = 0x1
	OpWrite Operation = 0x2
)

// satisfies reports whether perms grants every bit set in op.
func (op Operation) satisfies(perms models.BucketKeyPermission) bool {
	if op&OpRead != 0 && !perms.Read {
		return false
	}
	if op&OpWrite != 0 && !perms.Write {
		return false
	}
	return true
}

func setKeyInCache(bucketName string, permissions models.BucketKeyPermission, creds *credentials.Credentials) {
	canWrite := permissions.Write
	canRead := permissions.Read

	if canWrite {
		key := fmt.Sprintf("key:%s:%d", bucketName, OpWrite)
		utils.GlobalCache.Set(key, creds, time.Hour)
	}

	if canRead {
		key := fmt.Sprintf("key:%s:%d", bucketName, OpRead)
		utils.GlobalCache.Set(key, creds, time.Hour)
	}

	if canRead && canWrite {
		key := fmt.Sprintf("key:%s:%d", bucketName, OpRead|OpWrite)
		utils.GlobalCache.Set(key, creds, time.Hour)
	}
}

func (s *S3Service) getBucketCredentials(ctx context.Context, bucketName string, op Operation) (*credentials.Credentials, error) {
	cacheKey := fmt.Sprintf("key:%s:%d", bucketName, op)
	if cached := utils.GlobalCache.Get(cacheKey); cached != nil {
		return cached.(*credentials.Credentials), nil
	}

	bucketInfo, err := s.adminService.GetBucketInfoByAlias(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket info: %w", err)
	}

	for _, keyInfo := range bucketInfo.Keys {
		if !op.satisfies(keyInfo.Permissions) {
			continue
		}
		keyDetails, err := s.adminService.GetKeyInfo(ctx, keyInfo.AccessKeyID, true)
		if err != nil || keyDetails.SecretAccessKey == nil {
			continue
		}
		creds := credentials.NewStaticV4(keyDetails.AccessKeyID, *keyDetails.SecretAccessKey, "")
		setKeyInCache(bucketName, keyInfo.Permissions, creds)
		return creds, nil
	}

	return nil, fmt.Errorf("no valid credentials found for bucket %s", bucketName)
}

// getMinioClient creates a MinIO client for a specific bucket with credentials
// that satisfy op.
func (s *S3Service) getMinioClient(ctx context.Context, bucketName string, op Operation) (*minio.Client, error) {
	creds, err := s.getBucketCredentials(ctx, bucketName, op)
	if err != nil {
		return nil, fmt.Errorf("cannot get credentials for bucket %s: %w", bucketName, err)
	}

	// Create MinIO client with bucket-specific credentials
	client, err := minio.New(s.config.Endpoint, &minio.Options{
		Creds:  creds,
		Secure: s.config.UseSSL,
		Region: s.config.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client for bucket %s: %w", bucketName, err)
	}

	return client, nil
}

// ListBuckets retrieves all buckets from Garage
func (s *S3Service) ListBuckets(ctx context.Context) (*models.BucketListResponse, error) {
	var bucketInfos []minio.BucketInfo

	// Call MinIO ListBuckets API with retry logic
	retryConfig := utils.DefaultRetryConfig()
	err := utils.RetryWithBackoff(ctx, retryConfig, func() error {
		var listErr error
		bucketInfos, listErr = s.client.ListBuckets(ctx)
		return listErr
	})
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
	client, err := s.getMinioClient(ctx, bucketName, OpRead|OpWrite)
	if err != nil {
		return fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	// Call MinIO MakeBucket API with retry logic
	retryConfig := utils.DefaultRetryConfig()
	err = utils.RetryWithBackoff(ctx, retryConfig, func() error {
		return client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
			Region: s.config.Region,
		})
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
	}

	return nil
}

// DeleteBucket deletes a bucket from Garage
func (s *S3Service) DeleteBucket(ctx context.Context, bucketName string) error {
	client, err := s.getMinioClient(ctx, bucketName, OpRead|OpWrite)
	if err != nil {
		return fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	// Call MinIO RemoveBucket API with retry logic
	retryConfig := utils.DefaultRetryConfig()
	err = utils.RetryWithBackoff(ctx, retryConfig, func() error {
		return client.RemoveBucket(ctx, bucketName)
	})
	if err != nil {
		return fmt.Errorf("failed to delete bucket %s: %w", bucketName, err)
	}

	return nil
}

// ListObjects lists objects in a bucket with optional prefix filter and pagination
func (s *S3Service) ListObjects(ctx context.Context, bucketName, prefix string, maxKeys int, continuationToken string) (*models.ObjectListResponse, error) {
	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName, OpRead)
	if err != nil {
		return nil, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	// Set default max keys if not specified
	if maxKeys <= 0 {
		maxKeys = 1000
	}

	// Create Core client for low-level API access
	core := &minio.Core{Client: client}

	// Use Core.ListObjectsV2 for proper pagination with continuation tokens
	result, err := core.ListObjectsV2(
		bucketName,
		prefix,            // objectPrefix
		"",                // startAfter (empty when using continuationToken)
		continuationToken, // continuationToken (proper S3 token)
		"/",               // delimiter (for folder listing)
		maxKeys,           // maxkeys
	)

	if err != nil {
		return nil, fmt.Errorf("failed to list objects in bucket %s: %w", bucketName, err)
	}

	contents := make([]minio.ObjectInfo, 0, len(result.Contents))
	markerKeys := make([]string, 0)
	for _, obj := range result.Contents {
		if strings.HasSuffix(obj.Key, "/") && obj.Size == 0 {
			if obj.Key != prefix {
				markerKeys = append(markerKeys, obj.Key)
			}
			continue
		}
		contents = append(contents, obj)
	}

	// Process objects from result.Contents
	// Note: ListObjectsV2 doesn't return ContentType, so we need to fetch it separately
	objects := make([]models.ObjectInfo, len(contents))

	// Use goroutines to fetch ContentType concurrently for better performance
	type statResult struct {
		index       int
		contentType string
		err         error
	}

	statChan := make(chan statResult, len(result.Contents))

	for i, obj := range contents {
		go func(idx int, objKey string) {
			// Fetch object metadata to get ContentType
			stat, err := client.StatObject(ctx, bucketName, objKey, minio.StatObjectOptions{})
			if err != nil {
				// If StatObject fails, we still include the object but without ContentType
				statChan <- statResult{index: idx, contentType: "", err: err}
				return
			}
			statChan <- statResult{index: idx, contentType: stat.ContentType, err: nil}
		}(i, obj.Key)

		// Initialize the object with basic info from ListObjectsV2
		objects[i] = models.ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
			StorageClass: obj.StorageClass,
		}
	}

	// Collect results from goroutines
	for range contents {
		res := <-statChan
		if res.err == nil {
			objects[res.index].ContentType = res.contentType
		}
		// If there was an error, ContentType remains empty, which is acceptable
	}
	close(statChan)

	// Process folders from result.CommonPrefixes
	prefixList := make([]string, 0, len(result.CommonPrefixes)+len(markerKeys))
	seen := make(map[string]struct{}, len(result.CommonPrefixes))
	for _, p := range result.CommonPrefixes {
		prefixList = append(prefixList, p.Prefix)
		seen[p.Prefix] = struct{}{}
	}
	// Promote filtered directory markers into Prefixes so empty folders still
	// appear in the listing.
	for _, k := range markerKeys {
		if _, ok := seen[k]; ok {
			continue
		}
		prefixList = append(prefixList, k)
		seen[k] = struct{}{}
	}

	return &models.ObjectListResponse{
		Bucket:                bucketName,
		Objects:               objects,
		Prefixes:              prefixList,
		Count:                 len(objects),
		IsTruncated:           result.IsTruncated,
		NextContinuationToken: result.NextContinuationToken,
	}, nil
}

const (
	searchMaxScan    = 10000 // stop after scanning this many objects
	searchMaxResults = 1000  // stop after collecting this many matches
	searchPageSize   = 1000  // objects requested per ListObjectsV2 page
)

func objectMatchesSearch(key string, size int64, lowerQuery string) bool {
	if strings.HasSuffix(key, "/") && size == 0 {
		return false
	}
	return strings.Contains(strings.ToLower(key), lowerQuery)
}

// SearchObjects performs a recursive, best-effort substring search over object
// keys under the given prefix.
func (s *S3Service) SearchObjects(ctx context.Context, bucketName, prefix, search string) (*models.ObjectListResponse, error) {
	client, err := s.getMinioClient(ctx, bucketName, OpRead)
	if err != nil {
		return nil, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	core := &minio.Core{Client: client}
	lowerQuery := strings.ToLower(search)

	matches := make([]models.ObjectInfo, 0, 64)
	scanned := 0
	truncated := false
	token := ""

scan:
	for {
		result, err := core.ListObjectsV2(
			bucketName,
			prefix,
			"",
			token,
			"",
			searchPageSize,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to search objects in bucket %s: %w", bucketName, err)
		}

		for _, obj := range result.Contents {
			scanned++
			if objectMatchesSearch(obj.Key, obj.Size, lowerQuery) {
				matches = append(matches, models.ObjectInfo{
					Key:          obj.Key,
					Size:         obj.Size,
					LastModified: obj.LastModified,
					ETag:         obj.ETag,
					StorageClass: obj.StorageClass,
				})
				if len(matches) >= searchMaxResults {
					truncated = true
					break scan
				}
			}
			if scanned >= searchMaxScan {
				truncated = true
				break scan
			}
		}

		if !result.IsTruncated || result.NextContinuationToken == "" {
			break
		}
		token = result.NextContinuationToken
	}

	if truncated {
		logpkg.FromCtx(ctx).Warn().
			Str("bucket", bucketName).
			Str("prefix", prefix).
			Int("scanned", scanned).
			Int("matches", len(matches)).
			Msg("search hit scan/result cap; results are partial")
	}

	return &models.ObjectListResponse{
		Bucket:      bucketName,
		Objects:     matches,
		Prefixes:    []string{},
		Count:       len(matches),
		IsTruncated: truncated,
		// Search returns all matches up to the cap in one response; there is no
		// token-based pagination for search results.
		NextContinuationToken: "",
	}, nil
}

// UploadObject uploads an object to a bucket
func (s *S3Service) UploadObject(ctx context.Context, bucketName, key string, body io.Reader, contentType string) (*models.ObjectUploadResponse, error) {
	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName, OpWrite)
	if err != nil {
		return nil, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	// Upload options
	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}

	var info minio.UploadInfo

	// Call MinIO PutObject API with retry logic
	retryConfig := utils.DefaultRetryConfig()
	err = utils.RetryWithBackoff(ctx, retryConfig, func() error {
		var uploadErr error
		info, uploadErr = client.PutObject(ctx, bucketName, key, body, -1, opts)
		return uploadErr
	})
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

// CreateDirectoryMarker creates a zero-byte object whose key ends with "/".
// Garage rejects the streaming path (size=-1) with "Empty body" because the
// MinIO client switches to multipart upload, which requires payload. Passing
// size=0 forces a single PutObject request with Content-Length: 0, which
// Garage accepts as a directory marker.
func (s *S3Service) CreateDirectoryMarker(ctx context.Context, bucketName, key string) (*models.ObjectUploadResponse, error) {
	client, err := s.getMinioClient(ctx, bucketName, OpWrite)
	if err != nil {
		return nil, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	opts := minio.PutObjectOptions{ContentType: "application/x-directory"}

	var info minio.UploadInfo
	retryConfig := utils.DefaultRetryConfig()
	err = utils.RetryWithBackoff(ctx, retryConfig, func() error {
		var uploadErr error
		info, uploadErr = client.PutObject(ctx, bucketName, key, bytes.NewReader(nil), 0, opts)
		return uploadErr
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create directory %s in bucket %s: %w", key, bucketName, err)
	}

	return &models.ObjectUploadResponse{
		Bucket:      bucketName,
		Key:         key,
		ETag:        info.ETag,
		Size:        info.Size,
		ContentType: opts.ContentType,
	}, nil
}

// GetObject retrieves an object from a bucket
func (s *S3Service) GetObject(ctx context.Context, bucketName, key string) (io.ReadCloser, *models.ObjectInfo, error) {
	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName, OpRead)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	var object *minio.Object

	// Call MinIO GetObject API with retry logic
	retryConfig := utils.DefaultRetryConfig()
	err = utils.RetryWithBackoff(ctx, retryConfig, func() error {
		var getErr error
		object, getErr = client.GetObject(ctx, bucketName, key, minio.GetObjectOptions{})
		return getErr
	})
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
	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName, OpWrite)
	if err != nil {
		return fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	// Call MinIO RemoveObject API with retry logic
	retryConfig := utils.DefaultRetryConfig()
	err = utils.RetryWithBackoff(ctx, retryConfig, func() error {
		return client.RemoveObject(ctx, bucketName, key, minio.RemoveObjectOptions{})
	})
	if err != nil {
		return fmt.Errorf("failed to delete object %s from bucket %s: %w", key, bucketName, err)
	}

	return nil
}

// ObjectExists checks if an object exists in a bucket
func (s *S3Service) ObjectExists(ctx context.Context, bucketName, key string) (bool, error) {
	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName, OpRead)
	if err != nil {
		return false, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	var statErr error

	// Call MinIO StatObject API with retry logic
	retryConfig := utils.DefaultRetryConfig()
	err = utils.RetryWithBackoff(ctx, retryConfig, func() error {
		_, statErr = client.StatObject(ctx, bucketName, key, minio.StatObjectOptions{})
		return statErr
	})

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
	client, err := s.getMinioClient(ctx, bucketName, OpRead)
	if err != nil {
		return nil, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	var stat minio.ObjectInfo

	// Call MinIO StatObject API with retry logic
	retryConfig := utils.DefaultRetryConfig()
	err = utils.RetryWithBackoff(ctx, retryConfig, func() error {
		var statErr error
		stat, statErr = client.StatObject(ctx, bucketName, key, minio.StatObjectOptions{})
		return statErr
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata for object %s in bucket %s: %w", key, bucketName, err)
	}

	return &models.ObjectInfo{
		Key:          key,
		Size:         stat.Size,
		LastModified: stat.LastModified,
		ETag:         stat.ETag,
		ContentType:  stat.ContentType,
		StorageClass: stat.StorageClass,
		Metadata:     stat.UserMetadata,
	}, nil
}

// DeleteMultipleObjects deletes multiple objects from a bucket and returns the
// number of objects that were removed (requested keys minus any that failed).
//
// Note: S3/MinIO batch delete is idempotent — removing a key that does not
// exist succeeds and is not reported on the error channel, so it counts toward
// the returned total. The count therefore reflects "keys the delete operation
// did not fail on", which is the strongest signal obtainable without a
// per-key existence check.
func (s *S3Service) DeleteMultipleObjects(ctx context.Context, bucketName string, keys []string) (int, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName, OpWrite)
	if err != nil {
		return 0, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
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

	// Call MinIO RemoveObjects API (batch delete). RemoveObjects only surfaces
	// the objects it FAILED to delete, so we drain the whole channel (which also
	// avoids leaking the sender goroutine) and count failures.
	errorCh := client.RemoveObjects(ctx, bucketName, objectsCh, minio.RemoveObjectsOptions{})

	failed := 0
	var firstErr error
	for rerr := range errorCh {
		if rerr.Err != nil {
			failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to delete object %s from bucket %s: %w", rerr.ObjectName, bucketName, rerr.Err)
			}
		}
	}

	if firstErr != nil {
		return len(keys) - failed, firstErr
	}

	return len(keys), nil
}

// DeleteObjectsByPrefix recursively deletes every object stored under the given
// prefix (i.e. a "folder"), including the directory marker itself. It returns
// the number of objects that were deleted.
func (s *S3Service) DeleteObjectsByPrefix(ctx context.Context, bucketName, prefix string) (int, error) {
	if prefix == "" {
		return 0, fmt.Errorf("prefix is required for recursive delete")
	}

	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName, OpWrite)
	if err != nil {
		return 0, fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	// List every object under the prefix recursively (no delimiter), so nested
	// folders are flattened into their concrete keys.
	keys := make([]string, 0)
	for obj := range client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return 0, fmt.Errorf("failed to list objects under prefix %s in bucket %s: %w", prefix, bucketName, obj.Err)
		}
		keys = append(keys, obj.Key)
	}

	if len(keys) == 0 {
		return 0, nil
	}

	return s.DeleteMultipleObjects(ctx, bucketName, keys)
}

// GetPresignedURL generates a pre-signed URL for temporary access to an object
// This is useful for sharing files without exposing credentials
func (s *S3Service) GetPresignedURL(ctx context.Context, bucketName, key string, expiresIn time.Duration) (string, error) {
	// Get bucket-specific MinIO client
	client, err := s.getMinioClient(ctx, bucketName, OpRead)
	if err != nil {
		return "", fmt.Errorf("failed to get MinIO client for bucket %s: %w", bucketName, err)
	}

	var presignedURL *url.URL

	// Generate presigned GET URL with retry logic
	retryConfig := utils.DefaultRetryConfig()
	err = utils.RetryWithBackoff(ctx, retryConfig, func() error {
		var presignErr error
		presignedURL, presignErr = client.PresignedGetObject(ctx, bucketName, key, expiresIn, nil)
		return presignErr
	})
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

func (s *S3Service) UploadMultipleObjects(ctx context.Context, bucketName string, files []struct {
	Key         string
	Body        io.Reader
	ContentType string
}) []UploadResult {
	results := make([]UploadResult, len(files))

	// Get bucket-specific MinIO client once for all uploads
	client, err := s.getMinioClient(ctx, bucketName, OpWrite)
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
