package mocks

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"
)

// The mocks are exercised across the handlers test suite, but Go's per-
// package coverage counts only statements executed from tests in THIS
// package. This test calls every mock method with no Fn configured, which
// covers the default "not configured" error path (and the record() helpers
// that build the call log).

func TestAdminMock_UnconfiguredMethodsReturnSentinel(t *testing.T) {
	ctx := context.Background()
	m := &AdminMock{}

	type call struct {
		name string
		fn   func() error
	}
	calls := []call{
		{"ListKeys", func() error { _, e := m.ListKeys(ctx); return e }},
		{"CreateKey", func() error { _, e := m.CreateKey(ctx, models.CreateKeyRequest{}); return e }},
		{"GetKeyInfo", func() error { _, e := m.GetKeyInfo(ctx, "k", false); return e }},
		{"UpdateKey", func() error { _, e := m.UpdateKey(ctx, "k", models.UpdateKeyRequest{}); return e }},
		{"DeleteKey", func() error { return m.DeleteKey(ctx, "k") }},
		{"ListBuckets", func() error { _, e := m.ListBuckets(ctx); return e }},
		{"GetBucketInfo", func() error { _, e := m.GetBucketInfo(ctx, "b"); return e }},
		{"GetBucketInfoByAlias", func() error { _, e := m.GetBucketInfoByAlias(ctx, "a"); return e }},
		{"CreateBucket", func() error { _, e := m.CreateBucket(ctx, models.CreateBucketAdminRequest{}); return e }},
		{"UpdateBucket", func() error { _, e := m.UpdateBucket(ctx, "b", models.UpdateBucketRequest{}); return e }},
		{"DeleteBucket", func() error { return m.DeleteBucket(ctx, "b") }},
		{"AllowBucketKey", func() error { _, e := m.AllowBucketKey(ctx, models.BucketKeyPermRequest{}); return e }},
		{"DenyBucketKey", func() error { _, e := m.DenyBucketKey(ctx, models.BucketKeyPermRequest{}); return e }},
		{"GetClusterHealth", func() error { _, e := m.GetClusterHealth(ctx); return e }},
		{"GetClusterStatus", func() error { _, e := m.GetClusterStatus(ctx); return e }},
		{"GetClusterStatistics", func() error { _, e := m.GetClusterStatistics(ctx); return e }},
		{"GetNodeInfo", func() error { _, e := m.GetNodeInfo(ctx, "n"); return e }},
		{"GetNodeStatistics", func() error { _, e := m.GetNodeStatistics(ctx, "n"); return e }},
		{"HealthCheck", func() error { return m.HealthCheck(ctx) }},
		{"GetMetrics", func() error { _, e := m.GetMetrics(ctx); return e }},
	}

	for _, c := range calls {
		err := c.fn()
		if err == nil {
			t.Errorf("%s: expected error from unconfigured mock, got nil", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.name) {
			t.Errorf("%s: error %q should mention method name", c.name, err.Error())
		}
	}
	if len(m.Calls) != len(calls) {
		t.Errorf("Calls recorded = %d, want %d", len(m.Calls), len(calls))
	}
}

func TestAdminMock_ConfiguredFnsAreInvoked(t *testing.T) {
	ctx := context.Background()
	m := &AdminMock{
		ListKeysFn: func(ctx context.Context) ([]models.ListKeysResponseItem, error) {
			return []models.ListKeysResponseItem{{ID: "k1"}}, nil
		},
		DeleteKeyFn:  func(ctx context.Context, id string) error { return nil },
		HealthCheckFn: func(ctx context.Context) error { return nil },
		GetMetricsFn: func(ctx context.Context) (string, error) { return "metric 1", nil },
	}
	if got, err := m.ListKeys(ctx); err != nil || len(got) != 1 {
		t.Errorf("ListKeys = (%v, %v), want one item", got, err)
	}
	if err := m.DeleteKey(ctx, "k"); err != nil {
		t.Errorf("DeleteKey: %v", err)
	}
	if err := m.HealthCheck(ctx); err != nil {
		t.Errorf("HealthCheck: %v", err)
	}
	if got, _ := m.GetMetrics(ctx); got != "metric 1" {
		t.Errorf("GetMetrics = %q", got)
	}
}

func TestS3Mock_UnconfiguredMethodsReturnSentinel(t *testing.T) {
	ctx := context.Background()
	m := &S3Mock{}

	if _, err := m.ListObjects(ctx, "b", "", 0, ""); err == nil {
		t.Error("ListObjects: want error")
	}
	if _, err := m.UploadObject(ctx, "b", "k", strings.NewReader(""), ""); err == nil {
		t.Error("UploadObject: want error")
	}
	if _, err := m.CreateDirectoryMarker(ctx, "b", "k/"); err == nil {
		t.Error("CreateDirectoryMarker: want error")
	}
	if _, _, err := m.GetObject(ctx, "b", "k"); err == nil {
		t.Error("GetObject: want error")
	}
	if _, err := m.ObjectExists(ctx, "b", "k"); err == nil {
		t.Error("ObjectExists: want error")
	}
	if err := m.DeleteObject(ctx, "b", "k"); err == nil {
		t.Error("DeleteObject: want error")
	}
	if _, err := m.GetObjectMetadata(ctx, "b", "k"); err == nil {
		t.Error("GetObjectMetadata: want error")
	}
	if _, err := m.GetPresignedURL(ctx, "b", "k", time.Minute); err == nil {
		t.Error("GetPresignedURL: want error")
	}
	if _, err := m.DeleteMultipleObjects(ctx, "b", []string{"k"}); err == nil {
		t.Error("DeleteMultipleObjects: want error")
	}
	// UploadMultipleObjects has no error channel; it must return a result slice
	// with one failed entry per input file.
	results := m.UploadMultipleObjects(ctx, "b", []struct {
		Key         string
		Body        io.Reader
		ContentType string
	}{
		{Key: "a"}, {Key: "b"},
	})
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for _, r := range results {
		if r.Success {
			t.Errorf("result[%s] should not be successful with no Fn set", r.Key)
		}
	}
	if len(m.Calls) < 10 {
		t.Errorf("expected Calls to capture each invocation, got %d entries", len(m.Calls))
	}
}

func TestS3Mock_ConfiguredFnsAreInvoked(t *testing.T) {
	ctx := context.Background()
	m := &S3Mock{
		ListObjectsFn: func(_ context.Context, _, _ string, _ int, _ string) (*models.ObjectListResponse, error) {
			return &models.ObjectListResponse{Count: 1}, nil
		},
		UploadObjectFn: func(_ context.Context, _, _ string, _ io.Reader, _ string) (*models.ObjectUploadResponse, error) {
			return &models.ObjectUploadResponse{}, nil
		},
		CreateDirectoryMarkerFn: func(_ context.Context, _, _ string) (*models.ObjectUploadResponse, error) {
			return &models.ObjectUploadResponse{}, nil
		},
		GetObjectFn: func(_ context.Context, _, _ string) (io.ReadCloser, *models.ObjectInfo, error) {
			return io.NopCloser(strings.NewReader("x")), &models.ObjectInfo{}, nil
		},
		ObjectExistsFn: func(_ context.Context, _, _ string) (bool, error) { return true, nil },
		DeleteObjectFn: func(_ context.Context, _, _ string) error { return nil },
		GetObjectMetadataFn: func(_ context.Context, _, _ string) (*models.ObjectInfo, error) {
			return &models.ObjectInfo{}, nil
		},
		GetPresignedURLFn: func(_ context.Context, _, _ string, _ time.Duration) (string, error) {
			return "http://x", nil
		},
		DeleteMultipleObjectsFn: func(_ context.Context, _ string, keys []string) (int, error) { return len(keys), nil },
		UploadMultipleObjectsFn: func(_ context.Context, _ string, files []struct {
			Key         string
			Body        io.Reader
			ContentType string
		}) []services.UploadResult {
			out := make([]services.UploadResult, len(files))
			for i, f := range files {
				out[i] = services.UploadResult{Key: f.Key, Success: true}
			}
			return out
		},
	}

	if r, err := m.ListObjects(ctx, "b", "", 0, ""); err != nil || r.Count != 1 {
		t.Errorf("ListObjects = (%+v, %v)", r, err)
	}
	if _, err := m.UploadObject(ctx, "b", "k", strings.NewReader(""), ""); err != nil {
		t.Errorf("UploadObject: %v", err)
	}
	if _, err := m.CreateDirectoryMarker(ctx, "b", "k/"); err != nil {
		t.Errorf("CreateDirectoryMarker: %v", err)
	}
	if _, _, err := m.GetObject(ctx, "b", "k"); err != nil {
		t.Errorf("GetObject: %v", err)
	}
	if ok, err := m.ObjectExists(ctx, "b", "k"); err != nil || !ok {
		t.Errorf("ObjectExists = (%v, %v)", ok, err)
	}
	if err := m.DeleteObject(ctx, "b", "k"); err != nil {
		t.Errorf("DeleteObject: %v", err)
	}
	if _, err := m.GetObjectMetadata(ctx, "b", "k"); err != nil {
		t.Errorf("GetObjectMetadata: %v", err)
	}
	if u, err := m.GetPresignedURL(ctx, "b", "k", time.Minute); err != nil || u == "" {
		t.Errorf("GetPresignedURL = (%q, %v)", u, err)
	}
	if n, err := m.DeleteMultipleObjects(ctx, "b", []string{"k"}); err != nil || n != 1 {
		t.Errorf("DeleteMultipleObjects = (%d, %v)", n, err)
	}
	results := m.UploadMultipleObjects(ctx, "b", []struct {
		Key         string
		Body        io.Reader
		ContentType string
	}{{Key: "a"}})
	if len(results) != 1 || !results[0].Success {
		t.Errorf("UploadMultipleObjects results = %+v", results)
	}
}
