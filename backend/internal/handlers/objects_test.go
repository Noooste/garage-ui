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

func newObjectsTestApp(t *testing.T) (*fiber.App, *mocks.S3Mock) {
	t.Helper()
	s3 := &mocks.S3Mock{}
	h := NewObjectHandler(s3)
	app := fiber.New()
	app.Get("/buckets/:bucket/objects", h.ListObjects)
	app.Post("/buckets/:bucket/objects", h.UploadObject)
	app.Post("/buckets/:bucket/directories", h.CreateDirectory)
	app.Post("/buckets/:bucket/objects/upload-multiple", h.UploadMultipleObjects)
	app.Post("/buckets/:bucket/objects/delete-multiple", h.DeleteMultipleObjects)
	// Wildcard endpoints — mount under :key for tests. Handlers prefer
	// c.Locals("objectKey") but fall back to c.Params("key"), so :key works.
	app.Get("/buckets/:bucket/objects/:key", h.GetObject)
	app.Get("/buckets/:bucket/objects/:key/metadata", h.GetObjectMetadata)
	app.Get("/buckets/:bucket/objects/:key/presigned", h.GetPresignedURL)
	app.Delete("/buckets/:bucket/objects/:key", h.DeleteObject)
	return app, s3
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
	s3.DeleteMultipleObjectsFn = func(_ context.Context, bucket string, keys []string) (int, error) {
		if bucket != "b1" || len(keys) != 3 {
			t.Errorf("args = (%q, %v)", bucket, keys)
		}
		return len(keys), nil
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

func TestDeleteMultipleObjects_Prefixes_Recursive(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.DeleteObjectsByPrefixFn = func(_ context.Context, bucket, prefix string) (int, error) {
		if bucket != "b1" || prefix != "docs/" {
			t.Errorf("args = (%q, %q)", bucket, prefix)
		}
		return 4, nil
	}
	body, _ := json.Marshal(map[string]any{"prefixes": []string{"docs/"}})
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
	if out.Data.Deleted != 4 {
		t.Errorf("Deleted = %d, want 4", out.Data.Deleted)
	}
}

func TestDeleteMultipleObjects_KeysAndPrefixes(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.DeleteMultipleObjectsFn = func(_ context.Context, _ string, keys []string) (int, error) {
		if len(keys) != 2 {
			t.Errorf("keys = %v", keys)
		}
		return len(keys), nil
	}
	s3.DeleteObjectsByPrefixFn = func(_ context.Context, _, _ string) (int, error) { return 3, nil }
	body, _ := json.Marshal(map[string]any{"keys": []string{"a", "b"}, "prefixes": []string{"docs/"}})
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
	if out.Data.Deleted != 5 {
		t.Errorf("Deleted = %d, want 5 (2 keys + 3 under prefix)", out.Data.Deleted)
	}
}

func TestDeleteMultipleObjects_PrefixError500(t *testing.T) {
	app, s3 := newObjectsTestApp(t)
	s3.DeleteObjectsByPrefixFn = func(_ context.Context, _, _ string) (int, error) {
		return 0, errors.New("boom")
	}
	body, _ := json.Marshal(map[string]any{"prefixes": []string{"docs/"}})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/delete-multiple", bytes.NewReader(body))
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

func TestDeleteMultipleObjects_BlankPrefix400(t *testing.T) {
	// A blank/whitespace-only prefix must be rejected with a 4XX before any
	// delete is attempted — it would otherwise target the whole bucket.
	for _, prefix := range []string{"", "   "} {
		app, s3 := newObjectsTestApp(t)
		s3.DeleteObjectsByPrefixFn = func(_ context.Context, _, _ string) (int, error) {
			t.Errorf("DeleteObjectsByPrefix must not be called for blank prefix %q", prefix)
			return 0, nil
		}
		body, _ := json.Marshal(map[string]any{"prefixes": []string{prefix}})
		req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/delete-multiple", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("prefix %q: status = %d, want 400", prefix, resp.StatusCode)
		}
	}
}

func TestDeleteMultipleObjects_PrefixNormalizedToTrailingSlash(t *testing.T) {
	// A prefix without a trailing slash must be normalized so it only deletes
	// its own folder ("photos/2024/"), not siblings like "photos/2024-old/".
	app, s3 := newObjectsTestApp(t)
	var gotPrefix string
	s3.DeleteObjectsByPrefixFn = func(_ context.Context, _, prefix string) (int, error) {
		gotPrefix = prefix
		return 1, nil
	}
	body, _ := json.Marshal(map[string]any{"prefixes": []string{"photos/2024"}})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/delete-multiple", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if gotPrefix != "photos/2024/" {
		t.Errorf("prefix passed to service = %q, want %q", gotPrefix, "photos/2024/")
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
	s3.DeleteMultipleObjectsFn = func(_ context.Context, _ string, _ []string) (int, error) { return 0, errors.New("boom") }
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
