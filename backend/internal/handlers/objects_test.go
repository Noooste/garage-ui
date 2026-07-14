package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"
	"Noooste/garage-ui/internal/services/mocks"

	"github.com/gofiber/fiber/v3"
)

// mintStub satisfies PreviewTokenMinter for handler tests.
type mintStub struct {
	fn func(bucket, key string, ttl time.Duration) (string, time.Time, error)
}

func (m *mintStub) MintPreviewToken(bucket, key string, ttl time.Duration) (string, time.Time, error) {
	if m.fn == nil {
		return "test-token", time.Now().Add(ttl), nil
	}
	return m.fn(bucket, key, ttl)
}

func newObjectsTestApp(t *testing.T) (*fiber.App, *mocks.S3Mock) {
	app, s3, _ := newObjectsTestAppWithMinter(t)
	return app, s3
}

func newObjectsTestAppWithMinter(t *testing.T) (*fiber.App, *mocks.S3Mock, *mintStub) {
	t.Helper()
	s3 := &mocks.S3Mock{}
	minter := &mintStub{}
	h := NewObjectHandler(s3, minter)
	app := fiber.New()
	app.Get("/buckets/:bucket/objects", h.ListObjects)
	app.Post("/buckets/:bucket/objects", h.UploadObject)
	app.Post("/buckets/:bucket/directories", h.CreateDirectory)
	app.Post("/buckets/:bucket/objects/upload-multiple", h.UploadMultipleObjects)
	app.Post("/buckets/:bucket/objects/delete-multiple", h.DeleteMultipleObjects)
	// Wildcard endpoints. Mount under :key for tests. Handlers prefer
	// c.Locals("objectKey") but fall back to c.Params("key"), so :key works.
	app.Get("/buckets/:bucket/objects/:key", h.GetObject)
	app.Get("/buckets/:bucket/objects/:key/metadata", h.GetObjectMetadata)
	app.Get("/buckets/:bucket/objects/:key/presigned", h.GetPresignedURL)
	app.Get("/buckets/:bucket/objects/:key/preview-url", h.GetPreviewURL)
	app.Delete("/buckets/:bucket/objects/:key", h.DeleteObject)
	return app, s3, minter
}

// --- ListObjects ---

func TestListObjects_Success(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.ListObjectsFn = func(_ context.Context, bucket, prefix string, max int, tok string) (*models.ObjectListResponse, error) {
		if bucket != "b1" || prefix != "p/" || max != 50 || tok != "T" {
			t.Errorf("args = (%q, %q, %d, %q)", bucket, prefix, max, tok)
		}
		return &models.ObjectListResponse{
			Bucket: bucket, Count: 1,
			Objects: []models.ObjectInfo{{Key: "k1", Size: 1}},
		}, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/buckets/b1/objects?prefix=p/&max_keys=50&continuation_token=T", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Data models.ObjectListResponse `json:"data"`
	}
	decodeJSON(t, resp.Body, &body)
	if body.Data.Count != 1 {
		t.Errorf("count = %d", body.Data.Count)
	}
}

func TestListObjects_DefaultMaxKeys(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.ListObjectsFn = func(_ context.Context, _, _ string, max int, _ string) (*models.ObjectListResponse, error) {
		if max != 100 {
			t.Errorf("max = %d, want default 100", max)
		}
		return &models.ObjectListResponse{}, nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestListObjects_InvalidMaxKeys400(t *testing.T) {
	app, _ := newObjectsTestApp(t)
	cases := []string{"0", "-1", "abc"}
	for _, mk := range cases {
		t.Run(mk, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects?max_keys="+mk, nil))
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestListObjects_ServiceError500(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.ListObjectsFn = func(_ context.Context, _, _ string, _ int, _ string) (*models.ObjectListResponse, error) {
		return nil, errors.New("boom")
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestListObjects_SearchRoutesToSearchObjects(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	// Intentionally leave ListObjectsFn unset: if the handler wrongly falls
	// through to a normal listing, the mock returns an error and this fails.
	s3.SearchObjectsFn = func(_ context.Context, bucket, prefix, search string) (*models.ObjectListResponse, error) {
		if bucket != "b1" || prefix != "docs/" || search != "target" {
			t.Errorf("args = (%q, %q, %q)", bucket, prefix, search)
		}
		return &models.ObjectListResponse{
			Bucket: bucket, Count: 1,
			Objects: []models.ObjectInfo{{Key: "docs/target.pdf", Size: 20}},
		}, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/buckets/b1/objects?prefix=docs/&search=target", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Data models.ObjectListResponse `json:"data"`
	}
	decodeJSON(t, resp.Body, &body)
	if body.Data.Count != 1 || len(body.Data.Objects) != 1 || body.Data.Objects[0].Key != "docs/target.pdf" {
		t.Errorf("unexpected search results: %+v", body.Data)
	}
}

func TestListObjects_SearchError500(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.SearchObjectsFn = func(_ context.Context, _, _, _ string) (*models.ObjectListResponse, error) {
		return nil, errors.New("boom")
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects?search=target", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

// --- GetObjectMetadata ---

func TestGetObjectMetadata_Success(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.GetObjectMetadataFn = func(_ context.Context, b, k string) (*models.ObjectInfo, error) {
		return &models.ObjectInfo{Key: k, Size: 42, ContentType: "image/png"}, nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/k1/metadata", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Data models.ObjectInfo `json:"data"`
	}
	decodeJSON(t, resp.Body, &body)
	if body.Data.Size != 42 {
		t.Errorf("size = %d", body.Data.Size)
	}
}

func TestGetObjectMetadata_NotFound404(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.GetObjectMetadataFn = func(_ context.Context, _, _ string) (*models.ObjectInfo, error) {
		return nil, errors.New("not found")
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/nope/metadata", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- DeleteObject ---

func TestDeleteObject_Success(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.ObjectExistsFn = func(_ context.Context, _, _ string) (bool, error) { return true, nil }
	s3.DeleteObjectFn = func(_ context.Context, b, k string) error {
		if b != "b1" || k != "k1" {
			t.Errorf("args = (%q, %q)", b, k)
		}
		return nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodDelete, "/buckets/b1/objects/k1", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestDeleteObject_NotExists404(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.ObjectExistsFn = func(_ context.Context, _, _ string) (bool, error) { return false, nil }
	resp, err := app.Test(httptest.NewRequest(http.MethodDelete, "/buckets/b1/objects/nope", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestDeleteObject_ExistsCheckError500(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.ObjectExistsFn = func(_ context.Context, _, _ string) (bool, error) { return false, errors.New("boom") }
	resp, err := app.Test(httptest.NewRequest(http.MethodDelete, "/buckets/b1/objects/k1", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestDeleteObject_DeleteError500(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.ObjectExistsFn = func(_ context.Context, _, _ string) (bool, error) { return true, nil }
	s3.DeleteObjectFn = func(_ context.Context, _, _ string) error { return errors.New("boom") }
	resp, err := app.Test(httptest.NewRequest(http.MethodDelete, "/buckets/b1/objects/k1", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

// --- GetPresignedURL ---

func TestGetPresignedURL_DefaultExpiration(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.ObjectExistsFn = func(_ context.Context, _, _ string) (bool, error) { return true, nil }
	s3.GetPresignedURLFn = func(_ context.Context, b, k string, exp time.Duration) (string, error) {
		if exp != 3600*time.Second {
			t.Errorf("exp = %v, want 1h", exp)
		}
		return "https://example/signed", nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/k1/presigned", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Data models.PresignedURLResponse `json:"data"`
	}
	decodeJSON(t, resp.Body, &body)
	if body.Data.URL != "https://example/signed" || body.Data.ExpiresIn != 3600 {
		t.Errorf("body = %+v", body.Data)
	}
}

func TestGetPresignedURL_CustomExpirationWithinRange(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.ObjectExistsFn = func(_ context.Context, _, _ string) (bool, error) { return true, nil }
	s3.GetPresignedURLFn = func(_ context.Context, _, _ string, exp time.Duration) (string, error) {
		if exp != 60*time.Second {
			t.Errorf("exp = %v, want 60s", exp)
		}
		return "u", nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/k1/presigned?expires_in=60", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestGetPresignedURL_InvalidExpiration400(t *testing.T) {
	app, _ := newObjectsTestApp(t)
	cases := []string{"0", "-1", "604801", "abc"}
	for _, val := range cases {
		t.Run(val, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/k1/presigned?expires_in="+val, nil))
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestGetPresignedURL_ObjectMissing404(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.ObjectExistsFn = func(_ context.Context, _, _ string) (bool, error) { return false, nil }
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/nope/presigned", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- GetPreviewURL ---

func TestGetPreviewURL_Success(t *testing.T) {
	app, _, minter := newObjectsTestAppWithMinter(t)
	fixed := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	minter.fn = func(bucket, key string, ttl time.Duration) (string, time.Time, error) {
		if bucket != "b1" || key != "clip.mp4" {
			t.Errorf("mint args = (%q, %q)", bucket, key)
		}
		if ttl != time.Hour {
			t.Errorf("ttl = %v, want 1h", ttl)
		}
		return "tok123", fixed, nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/clip.mp4/preview-url", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Data models.PreviewURLResponse `json:"data"`
	}
	decodeJSON(t, resp.Body, &body)
	if body.Data.URL != "/api/v1/buckets/b1/objects/clip.mp4?pt=tok123" {
		t.Errorf("url = %q", body.Data.URL)
	}
	if body.Data.ExpiresAt != "2026-07-11T12:00:00Z" {
		t.Errorf("expires_at = %q", body.Data.ExpiresAt)
	}
}

func TestGetPreviewURL_EscapesKeyInURL(t *testing.T) {
	// Production sets the decoded key in locals via the wildcard dispatcher,
	// so mirror that here instead of relying on :key param decoding.
	s3 := &mocks.S3Mock{}
	minter := &mintStub{}
	minter.fn = func(_, key string, _ time.Duration) (string, time.Time, error) {
		if key != "dir/my file.mp4" {
			t.Errorf("key = %q", key)
		}
		return "tok", time.Now().Add(time.Hour), nil
	}
	h := NewObjectHandler(s3, minter)
	app := fiber.New()
	app.Get("/buckets/:bucket/preview-url", func(c fiber.Ctx) error {
		c.Locals("objectKey", "dir/my file.mp4")
		return h.GetPreviewURL(c)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/preview-url", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Data models.PreviewURLResponse `json:"data"`
	}
	decodeJSON(t, resp.Body, &body)
	if !strings.HasPrefix(body.Data.URL, "/api/v1/buckets/b1/objects/dir%2Fmy%20file.mp4?pt=") {
		t.Errorf("url = %q, want the key percent-encoded whole", body.Data.URL)
	}
}

func TestGetPreviewURL_MissingBucketAndKey400(t *testing.T) {
	// Mount on a route with no :bucket param and no objectKey local, so both
	// bucket and key are empty and the handler short-circuits with 400.
	s3 := &mocks.S3Mock{}
	minter := &mintStub{}
	h := NewObjectHandler(s3, minter)
	app := fiber.New()
	app.Get("/preview-url-nobucket", h.GetPreviewURL)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/preview-url-nobucket", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestGetPreviewURL_MintError500(t *testing.T) {
	app, _, minter := newObjectsTestAppWithMinter(t)
	minter.fn = func(_, _ string, _ time.Duration) (string, time.Time, error) {
		return "", time.Time{}, errors.New("boom")
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/f/preview-url", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

// --- GetObject ---

func TestGetObject_Success_StreamsBodyAndHeaders(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	content := []byte("hello world")
	s3.GetObjectFn = func(_ context.Context, b, k string) (io.ReadCloser, *models.ObjectInfo, error) {
		return io.NopCloser(bytes.NewReader(content)), &models.ObjectInfo{
			Key: k, Size: int64(len(content)), ETag: `"etag-1"`,
			ContentType: "image/png", LastModified: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		}, nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/k1", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	if !strings.Contains(resp.Header.Get("Content-Disposition"), `filename="k1"`) {
		t.Errorf("Content-Disposition = %q", resp.Header.Get("Content-Disposition"))
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, content) {
		t.Errorf("body = %q, want %q", body, content)
	}
}

func TestGetObject_RewritesExecutableContentType(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.GetObjectFn = func(_ context.Context, _, _ string) (io.ReadCloser, *models.ObjectInfo, error) {
		return io.NopCloser(strings.NewReader("<script>alert(1)</script>")),
			&models.ObjectInfo{Key: "evil.html", Size: 25, ContentType: "text/html"},
			nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/evil.html", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", got)
	}
}

func TestGetObject_DownloadQuerySetsAttachment(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.GetObjectFn = func(_ context.Context, _, _ string) (io.ReadCloser, *models.ObjectInfo, error) {
		return io.NopCloser(strings.NewReader("x")), &models.ObjectInfo{Key: "file.txt", Size: 1}, nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/file.txt?download=true", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment") {
		t.Errorf("Content-Disposition = %q, want attachment", resp.Header.Get("Content-Disposition"))
	}
}

func TestGetObject_ServiceErrorReturns404(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.GetObjectFn = func(_ context.Context, _, _ string) (io.ReadCloser, *models.ObjectInfo, error) {
		return nil, nil, errors.New("not found")
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/nope", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// buildMultipart builds a multipart body with a single file field plus
// optional additional form fields. Returns body bytes and the Content-Type
// header value (which includes the boundary).
func buildMultipart(t *testing.T, fields map[string]string, files map[string]struct {
	Filename    string
	Content     []byte
	ContentType string
}) ([]byte, string) {
	t.Helper()
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	for name, f := range files {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{
			`form-data; name="` + name + `"; filename="` + f.Filename + `"`,
		}
		ct := f.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		h["Content-Type"] = []string{ct}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("CreatePart: %v", err)
		}
		if _, err := part.Write(f.Content); err != nil {
			t.Fatalf("part.Write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

func TestUploadObject_Success(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.UploadObjectFn = func(_ context.Context, bucket, key string, body io.Reader, ct string) (*models.ObjectUploadResponse, error) {
		if bucket != "b1" || key != "myfile.bin" {
			t.Errorf("args = (%q, %q)", bucket, key)
		}
		if ct != "application/octet-stream" {
			t.Errorf("contentType = %q", ct)
		}
		b, _ := io.ReadAll(body)
		if string(b) != "payload" {
			t.Errorf("body = %q, want 'payload'", b)
		}
		return &models.ObjectUploadResponse{Bucket: bucket, Key: key, Size: int64(len(b))}, nil
	}
	body, ct := buildMultipart(t, nil, map[string]struct {
		Filename    string
		Content     []byte
		ContentType string
	}{
		"file": {Filename: "myfile.bin", Content: []byte("payload")},
	})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 201\nbody: %s", resp.StatusCode, raw)
	}
}

func TestUploadObject_ExplicitKeyOverridesFilename(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.UploadObjectFn = func(_ context.Context, _, key string, _ io.Reader, _ string) (*models.ObjectUploadResponse, error) {
		if key != "custom/key.txt" {
			t.Errorf("key = %q, want custom/key.txt", key)
		}
		return &models.ObjectUploadResponse{Key: key}, nil
	}
	body, ct := buildMultipart(t, map[string]string{"key": "custom/key.txt"}, map[string]struct {
		Filename    string
		Content     []byte
		ContentType string
	}{
		"file": {Filename: "whatever.txt", Content: []byte("x")},
	})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestUploadObject_MissingFileReturns400(t *testing.T) {
	app, _ := newObjectsTestApp(t)
	body, ct := buildMultipart(t, map[string]string{"key": "k"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUploadObject_ServiceError500(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.UploadObjectFn = func(_ context.Context, _, _ string, _ io.Reader, _ string) (*models.ObjectUploadResponse, error) {
		return nil, errors.New("boom")
	}
	body, ct := buildMultipart(t, nil, map[string]struct {
		Filename    string
		Content     []byte
		ContentType string
	}{"file": {Filename: "f.bin", Content: []byte("x")}})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestDeleteMultipleObjects_Success(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.DeleteMultipleObjectsFn = func(_ context.Context, bucket string, keys []string) error {
		if bucket != "b1" || len(keys) != 3 {
			t.Errorf("args = (%q, %v)", bucket, keys)
		}
		return nil
	}
	body, _ := json.Marshal(map[string]any{"keys": []string{"a", "b", "c"}})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/delete-multiple", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var out struct {
		Data models.ObjectDeleteMultipleResponse `json:"data"`
	}
	decodeJSON(t, resp.Body, &out)
	if out.Data.Deleted != 3 {
		t.Errorf("Deleted = %d, want 3", out.Data.Deleted)
	}
}

func TestDeleteMultipleObjects_EmptyKeys400(t *testing.T) {
	app, _ := newObjectsTestApp(t)
	body, _ := json.Marshal(map[string]any{"keys": []string{}})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/delete-multiple", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDeleteMultipleObjects_MalformedJSON400(t *testing.T) {
	app, _ := newObjectsTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/delete-multiple", strings.NewReader("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestDeleteMultipleObjects_ServiceError500(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.DeleteMultipleObjectsFn = func(_ context.Context, _ string, _ []string) error { return errors.New("boom") }
	body, _ := json.Marshal(map[string]any{"keys": []string{"a"}})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/delete-multiple", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

// buildMultipartMulti builds a body with N "files" parts.
func buildMultipartMulti(t *testing.T, files []struct {
	Filename, ContentType string
	Content               []byte
}) ([]byte, string) {
	t.Helper()
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for _, f := range files {
		h := map[string][]string{
			"Content-Disposition": {`form-data; name="files"; filename="` + f.Filename + `"`},
			"Content-Type":        {f.ContentType},
		}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("CreatePart: %v", err)
		}
		_, _ = part.Write(f.Content)
	}
	_ = w.Close()
	return buf.Bytes(), w.FormDataContentType()
}

func TestUploadMultiple_AllSuccess201(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.UploadMultipleObjectsFn = func(_ context.Context, bucket string, files []struct {
		Key         string
		Body        io.Reader
		ContentType string
	}) []services.UploadResult {
		if bucket != "b1" || len(files) != 2 {
			t.Errorf("got bucket=%q files=%d", bucket, len(files))
		}
		out := make([]services.UploadResult, len(files))
		for i, f := range files {
			out[i] = services.UploadResult{Key: f.Key, Success: true, ContentType: f.ContentType, Size: 1}
		}
		return out
	}
	body, ct := buildMultipartMulti(t, []struct {
		Filename, ContentType string
		Content               []byte
	}{
		{"a.txt", "text/plain", []byte("a")},
		{"b.txt", "text/plain", []byte("b")},
	})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/upload-multiple", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 201\nbody: %s", resp.StatusCode, raw)
	}
	var out struct {
		Data models.ObjectUploadMultipleResponse `json:"data"`
	}
	decodeJSON(t, resp.Body, &out)
	if out.Data.SuccessCount != 2 || out.Data.FailureCount != 0 {
		t.Errorf("counts = (%d, %d)", out.Data.SuccessCount, out.Data.FailureCount)
	}
}

func TestUploadMultiple_PartialReturns207(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.UploadMultipleObjectsFn = func(_ context.Context, _ string, files []struct {
		Key         string
		Body        io.Reader
		ContentType string
	}) []services.UploadResult {
		return []services.UploadResult{
			{Key: files[0].Key, Success: true, Size: 1, ContentType: files[0].ContentType},
			{Key: files[1].Key, Success: false, Error: errors.New("upload failed"), ContentType: files[1].ContentType},
		}
	}
	body, ct := buildMultipartMulti(t, []struct {
		Filename, ContentType string
		Content               []byte
	}{
		{"a.txt", "text/plain", []byte("a")},
		{"b.txt", "text/plain", []byte("b")},
	})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/upload-multiple", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207", resp.StatusCode)
	}
}

func TestUploadMultiple_AllFailReturns500(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.UploadMultipleObjectsFn = func(_ context.Context, _ string, files []struct {
		Key         string
		Body        io.Reader
		ContentType string
	}) []services.UploadResult {
		out := make([]services.UploadResult, len(files))
		for i, f := range files {
			out[i] = services.UploadResult{Key: f.Key, Success: false, Error: errors.New("boom")}
		}
		return out
	}
	body, ct := buildMultipartMulti(t, []struct {
		Filename, ContentType string
		Content               []byte
	}{{"a.txt", "text/plain", []byte("a")}})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/upload-multiple", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestUploadMultiple_NoFiles400(t *testing.T) {
	app, _ := newObjectsTestApp(t)
	body, ct := buildMultipartMulti(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/upload-multiple", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUploadMultiple_DefaultsContentType(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.UploadMultipleObjectsFn = func(_ context.Context, _ string, files []struct {
		Key         string
		Body        io.Reader
		ContentType string
	}) []services.UploadResult {
		if files[0].ContentType != "application/octet-stream" {
			t.Errorf("ContentType = %q, want default application/octet-stream", files[0].ContentType)
		}
		return []services.UploadResult{{Key: files[0].Key, Success: true}}
	}
	// Write a part with an empty Content-Type header explicitly.
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	part, err := w.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="files"; filename="a.txt"`},
		"Content-Type":        {""},
	})
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	_, _ = part.Write([]byte("x"))
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/upload-multiple", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
}

// --- CreateDirectory ---

func TestCreateDirectory_Success_AppendsTrailingSlash(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	var gotKey string
	s3.CreateDirectoryMarkerFn = func(_ context.Context, bucket, key string) (*models.ObjectUploadResponse, error) {
		gotKey = key
		return &models.ObjectUploadResponse{Bucket: bucket, Key: key, Size: 0, ContentType: "application/x-directory"}, nil
	}

	body := bytes.NewBufferString(`{"key": "photos/2024"}`)
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/directories", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if gotKey != "photos/2024/" {
		t.Errorf("service key = %q, want trailing slash appended", gotKey)
	}
}

func TestCreateDirectory_StripsLeadingSlashes(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	var gotKey string
	s3.CreateDirectoryMarkerFn = func(_ context.Context, _, key string) (*models.ObjectUploadResponse, error) {
		gotKey = key
		return &models.ObjectUploadResponse{Key: key}, nil
	}

	body := bytes.NewBufferString(`{"key": "///already/"}`)
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/directories", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if gotKey != "already/" {
		t.Errorf("service key = %q, want 'already/'", gotKey)
	}
}

func TestCreateDirectory_MissingKey400(t *testing.T) {
	app, _ := newObjectsTestApp(t)
	body := bytes.NewBufferString(`{"key": ""}`)
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/directories", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateDirectory_MalformedJSON400(t *testing.T) {
	app, _ := newObjectsTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/directories", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateDirectory_ServiceError500(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.CreateDirectoryMarkerFn = func(_ context.Context, _, _ string) (*models.ObjectUploadResponse, error) {
		return nil, errors.New("boom")
	}
	body := bytes.NewBufferString(`{"key": "x/"}`)
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/directories", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

// --- GetObject Range support ---

func TestGetObject_NoRangeHeaderAdvertisesAcceptRanges(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.GetObjectFn = func(_ context.Context, _, key string) (io.ReadCloser, *models.ObjectInfo, error) {
		return io.NopCloser(strings.NewReader("0123456789")), &models.ObjectInfo{Key: key, Size: 10, ContentType: "text/plain", LastModified: time.Now()}, nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/f.txt", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("Accept-Ranges = %q, want %q", got, "bytes")
	}
}

func TestGetObject_RangeRequestServes206(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	now := time.Now()
	s3.GetObjectMetadataFn = func(_ context.Context, _, key string) (*models.ObjectInfo, error) {
		return &models.ObjectInfo{Key: key, Size: 10, ContentType: "video/mp4", ETag: "e1", LastModified: now}, nil
	}
	s3.GetObjectRangeFn = func(_ context.Context, _, _ string, start, end int64) (io.ReadCloser, error) {
		if start != 2 || end != 6 {
			t.Errorf("range = %d-%d, want 2-6", start, end)
		}
		return io.NopCloser(strings.NewReader("23456")), nil
	}
	req := httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/clip.mp4", nil)
	req.Header.Set("Range", "bytes=2-6")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Range"); got != "bytes 2-6/10" {
		t.Errorf("Content-Range = %q, want %q", got, "bytes 2-6/10")
	}
	if got := resp.Header.Get("Content-Length"); got != "5" {
		t.Errorf("Content-Length = %q, want %q", got, "5")
	}
	if got := resp.Header.Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("Accept-Ranges = %q, want %q", got, "bytes")
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "23456" {
		t.Errorf("body = %q, want %q", body, "23456")
	}
}

func TestGetObject_UnsatisfiableRangeServes416(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.GetObjectMetadataFn = func(_ context.Context, _, key string) (*models.ObjectInfo, error) {
		return &models.ObjectInfo{Key: key, Size: 10}, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/f.bin", nil)
	req.Header.Set("Range", "bytes=50-")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("status = %d, want 416", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Range"); got != "bytes */10" {
		t.Errorf("Content-Range = %q, want %q", got, "bytes */10")
	}
}

func TestGetObject_MultiRangeFallsBackToFullResponse(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.GetObjectMetadataFn = func(_ context.Context, _, key string) (*models.ObjectInfo, error) {
		return &models.ObjectInfo{Key: key, Size: 10}, nil
	}
	s3.GetObjectFn = func(_ context.Context, _, key string) (io.ReadCloser, *models.ObjectInfo, error) {
		return io.NopCloser(strings.NewReader("0123456789")), &models.ObjectInfo{Key: key, Size: 10, LastModified: time.Now()}, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/f.bin", nil)
	req.Header.Set("Range", "bytes=0-1,3-4")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "0123456789" {
		t.Errorf("body = %q, want the full object", body)
	}
}

func TestGetObject_RangeForMissingObjectIs404(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.GetObjectMetadataFn = func(_ context.Context, _, _ string) (*models.ObjectInfo, error) {
		return nil, errors.New("no such key")
	}
	req := httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/gone.bin", nil)
	req.Header.Set("Range", "bytes=0-5")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// A ranged read whose metadata resolves but whose byte fetch fails, for example
// when the object is deleted between the two calls, returns 404.
func TestGetObject_RangeReadErrorIs404(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.GetObjectMetadataFn = func(_ context.Context, _, key string) (*models.ObjectInfo, error) {
		return &models.ObjectInfo{Key: key, Size: 10}, nil
	}
	s3.GetObjectRangeFn = func(_ context.Context, _, _ string, _, _ int64) (io.ReadCloser, error) {
		return nil, errors.New("read failed")
	}
	req := httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/clip.mp4", nil)
	req.Header.Set("Range", "bytes=0-5")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// A ranged request with download=true still streams 206 but marks the body as
// an attachment instead of inline.
func TestGetObject_RangeWithDownloadSetsAttachment(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.GetObjectMetadataFn = func(_ context.Context, _, key string) (*models.ObjectInfo, error) {
		return &models.ObjectInfo{Key: key, Size: 10, ContentType: "video/mp4"}, nil
	}
	s3.GetObjectRangeFn = func(_ context.Context, _, _ string, _, _ int64) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("01234")), nil
	}
	req := httptest.NewRequest(http.MethodGet, "/buckets/b1/objects/clip.mp4?download=true", nil)
	req.Header.Set("Range", "bytes=0-4")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
		t.Errorf("Content-Disposition = %q, want attachment", got)
	}
}

// GetObject rejects a request that resolves to an empty object key with 400.
// This guards the wildcard dispatch path where the key comes from locals.
func TestGetObject_EmptyKeyIsBadRequest(t *testing.T) {
	s3 := &mocks.S3Mock{}
	h := NewObjectHandler(s3, &mintStub{})
	app := fiber.New()
	// Mounted without a :key param so the handler resolves an empty key.
	app.Get("/buckets/:bucket/object", h.GetObject)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/b1/object", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}
