package mocks

import (
	"context"
	"fmt"
	"io"
	"time"

	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"
)

// s3NotConfigured mirrors errNotConfigured but keys messages on S3Mock so
// failures from unset S3 methods are distinguishable from unset admin methods.
func s3NotConfigured(method string) error {
	return fmt.Errorf("S3Mock.%s: not configured by test", method)
}

// Compile-time guarantee that S3Mock satisfies services.S3Storage.
var _ services.S3Storage = (*S3Mock)(nil)

// S3Mock is a hand-rolled mock of services.S3Storage. Tests assign the
// per-method function fields they care about; unset methods return
// s3NotConfigured.
type S3Mock struct {
	ListObjectsFn           func(ctx context.Context, bucketName, prefix string, maxKeys int, continuationToken string) (*models.ObjectListResponse, error)
	SearchObjectsFn         func(ctx context.Context, bucketName, prefix, search string) (*models.ObjectListResponse, error)
	UploadObjectFn          func(ctx context.Context, bucketName, key string, body io.Reader, contentType string) (*models.ObjectUploadResponse, error)
	CreateDirectoryMarkerFn func(ctx context.Context, bucketName, key string) (*models.ObjectUploadResponse, error)
	GetObjectFn             func(ctx context.Context, bucketName, key string) (io.ReadCloser, *models.ObjectInfo, error)
	ObjectExistsFn          func(ctx context.Context, bucketName, key string) (bool, error)
	DeleteObjectFn          func(ctx context.Context, bucketName, key string) error
	GetObjectMetadataFn     func(ctx context.Context, bucketName, key string) (*models.ObjectInfo, error)
	GetPresignedURLFn       func(ctx context.Context, bucketName, key string, expiresIn time.Duration) (string, error)
	DeleteMultipleObjectsFn func(ctx context.Context, bucketName string, keys []string) (int, error)
	DeleteObjectsByPrefixFn func(ctx context.Context, bucketName, prefix string) (int, error)
	UploadMultipleObjectsFn func(ctx context.Context, bucketName string, files []struct {
		Key         string
		Body        io.Reader
		ContentType string
	}) []services.UploadResult

	// Calls records every invocation in order. Shares the Call type with
	// AdminMock via the same package.
	Calls []Call
}

func (m *S3Mock) record(method string, args ...any) {
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

func (m *S3Mock) ListObjects(ctx context.Context, bucketName, prefix string, maxKeys int, continuationToken string) (*models.ObjectListResponse, error) {
	m.record("ListObjects", bucketName, prefix, maxKeys, continuationToken)
	if m.ListObjectsFn == nil {
		return nil, s3NotConfigured("ListObjects")
	}
	return m.ListObjectsFn(ctx, bucketName, prefix, maxKeys, continuationToken)
}

func (m *S3Mock) SearchObjects(ctx context.Context, bucketName, prefix, search string) (*models.ObjectListResponse, error) {
	m.record("SearchObjects", bucketName, prefix, search)
	if m.SearchObjectsFn == nil {
		return nil, s3NotConfigured("SearchObjects")
	}
	return m.SearchObjectsFn(ctx, bucketName, prefix, search)
}

func (m *S3Mock) UploadObject(ctx context.Context, bucketName, key string, body io.Reader, contentType string) (*models.ObjectUploadResponse, error) {
	m.record("UploadObject", bucketName, key, contentType)
	if m.UploadObjectFn == nil {
		return nil, s3NotConfigured("UploadObject")
	}
	return m.UploadObjectFn(ctx, bucketName, key, body, contentType)
}

func (m *S3Mock) CreateDirectoryMarker(ctx context.Context, bucketName, key string) (*models.ObjectUploadResponse, error) {
	m.record("CreateDirectoryMarker", bucketName, key)
	if m.CreateDirectoryMarkerFn == nil {
		return nil, s3NotConfigured("CreateDirectoryMarker")
	}
	return m.CreateDirectoryMarkerFn(ctx, bucketName, key)
}

func (m *S3Mock) GetObject(ctx context.Context, bucketName, key string) (io.ReadCloser, *models.ObjectInfo, error) {
	m.record("GetObject", bucketName, key)
	if m.GetObjectFn == nil {
		return nil, nil, s3NotConfigured("GetObject")
	}
	return m.GetObjectFn(ctx, bucketName, key)
}

func (m *S3Mock) ObjectExists(ctx context.Context, bucketName, key string) (bool, error) {
	m.record("ObjectExists", bucketName, key)
	if m.ObjectExistsFn == nil {
		return false, s3NotConfigured("ObjectExists")
	}
	return m.ObjectExistsFn(ctx, bucketName, key)
}

func (m *S3Mock) DeleteObject(ctx context.Context, bucketName, key string) error {
	m.record("DeleteObject", bucketName, key)
	if m.DeleteObjectFn == nil {
		return s3NotConfigured("DeleteObject")
	}
	return m.DeleteObjectFn(ctx, bucketName, key)
}

func (m *S3Mock) GetObjectMetadata(ctx context.Context, bucketName, key string) (*models.ObjectInfo, error) {
	m.record("GetObjectMetadata", bucketName, key)
	if m.GetObjectMetadataFn == nil {
		return nil, s3NotConfigured("GetObjectMetadata")
	}
	return m.GetObjectMetadataFn(ctx, bucketName, key)
}

func (m *S3Mock) GetPresignedURL(ctx context.Context, bucketName, key string, expiresIn time.Duration) (string, error) {
	m.record("GetPresignedURL", bucketName, key, expiresIn)
	if m.GetPresignedURLFn == nil {
		return "", s3NotConfigured("GetPresignedURL")
	}
	return m.GetPresignedURLFn(ctx, bucketName, key, expiresIn)
}

func (m *S3Mock) DeleteMultipleObjects(ctx context.Context, bucketName string, keys []string) (int, error) {
	m.record("DeleteMultipleObjects", bucketName, keys)
	if m.DeleteMultipleObjectsFn == nil {
		return 0, s3NotConfigured("DeleteMultipleObjects")
	}
	return m.DeleteMultipleObjectsFn(ctx, bucketName, keys)
}

func (m *S3Mock) DeleteObjectsByPrefix(ctx context.Context, bucketName, prefix string) (int, error) {
	m.record("DeleteObjectsByPrefix", bucketName, prefix)
	if m.DeleteObjectsByPrefixFn == nil {
		return 0, s3NotConfigured("DeleteObjectsByPrefix")
	}
	return m.DeleteObjectsByPrefixFn(ctx, bucketName, prefix)
}

func (m *S3Mock) UploadMultipleObjects(ctx context.Context, bucketName string, files []struct {
	Key         string
	Body        io.Reader
	ContentType string
}) []services.UploadResult {
	m.record("UploadMultipleObjects", bucketName, len(files))
	if m.UploadMultipleObjectsFn == nil {
		// Can't return an error here — the interface has no error channel.
		// Return a single synthetic failure for every file so tests that
		// forget to configure this method see a clear red flag.
		results := make([]services.UploadResult, len(files))
		for i, f := range files {
			results[i] = services.UploadResult{
				Key:     f.Key,
				Success: false,
				Error:   s3NotConfigured("UploadMultipleObjects"),
			}
		}
		return results
	}
	return m.UploadMultipleObjectsFn(ctx, bucketName, files)
}
