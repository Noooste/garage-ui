package handlers

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"
	"Noooste/garage-ui/internal/services/mocks"

	"github.com/gofiber/fiber/v3"
)

func newStripTestApp(t *testing.T, strip bool) (*fiber.App, *mocks.S3Mock) {
	t.Helper()
	s3 := &mocks.S3Mock{}
	h := NewObjectHandler(s3, &mintStub{}, WithStripImageMetadata(strip))
	app := fiber.New()
	app.Post("/buckets/:bucket/objects", h.UploadObject)
	app.Post("/buckets/:bucket/objects/upload-multiple", h.UploadMultipleObjects)
	return app, s3
}

// jpegWithGPSExif returns a JPEG carrying an EXIF block (orientation 6 plus
// trailing "SECRET-GPS" bytes) and an XMP block.
func jpegWithGPSExif(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	base := buf.Bytes()
	tiff := []byte{
		'M', 'M', 0, 0x2A, 0, 0, 0, 8, 0, 1,
		0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0,
		0, 0, 0, 0,
	}
	exif := append(append([]byte("Exif\x00\x00"), tiff...), "SECRET-GPS"...)
	xmp := append([]byte("http://ns.adobe.com/xap/1.0/\x00"), "SECRET-XMP"...)
	seg := func(payload []byte) []byte {
		l := len(payload) + 2
		return append([]byte{0xFF, 0xE1, byte(l >> 8), byte(l)}, payload...)
	}
	out := append([]byte{}, base[:2]...)
	out = append(out, seg(exif)...)
	out = append(out, seg(xmp)...)
	return append(out, base[2:]...)
}

func postSingle(t *testing.T, app *fiber.App, filename, contentType string, content []byte) *http.Response {
	t.Helper()
	body, ct := buildMultipart(t, nil, map[string]struct {
		Filename    string
		Content     []byte
		ContentType string
	}{
		"file": {Filename: filename, Content: content, ContentType: contentType},
	})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return resp
}

func captureUpload(s3 *mocks.S3Mock) *[]byte {
	var got []byte
	s3.UploadObjectFn = func(_ context.Context, bucket, key string, body io.Reader, _ string) (*models.ObjectUploadResponse, error) {
		got, _ = io.ReadAll(body)
		return &models.ObjectUploadResponse{Bucket: bucket, Key: key, Size: int64(len(got))}, nil
	}
	return &got
}

func TestUploadObject_StripEnabled_RemovesMetadataKeepsOrientation(t *testing.T) {
	app, s3 := newStripTestApp(t, true)
	got := captureUpload(s3)
	orig := jpegWithGPSExif(t)

	// Wrong filename and Content-Type: detection must use the bytes.
	resp := postSingle(t, app, "photo.txt", "text/plain", orig)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d\n%s", resp.StatusCode, raw)
	}
	if bytes.Contains(*got, []byte("SECRET")) {
		t.Error("metadata was stored")
	}
	if !bytes.Contains(*got, []byte("Exif\x00\x00MM\x00\x2A")) || !bytes.Contains(*got, []byte{0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6}) {
		t.Error("orientation was not preserved")
	}
	if _, err := jpeg.Decode(bytes.NewReader(*got)); err != nil {
		t.Errorf("stored image does not decode: %v", err)
	}
}

func TestUploadObject_StripDisabled_ByteIdentical(t *testing.T) {
	app, s3 := newStripTestApp(t, false)
	got := captureUpload(s3)
	orig := jpegWithGPSExif(t)

	resp := postSingle(t, app, "photo.jpg", "image/jpeg", orig)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !bytes.Equal(*got, orig) {
		t.Error("upload was modified with stripping disabled")
	}
}

func TestUploadObject_StripEnabled_NonImageUntouched(t *testing.T) {
	app, s3 := newStripTestApp(t, true)
	got := captureUpload(s3)
	content := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><metadata>keep</metadata></svg>`)

	resp := postSingle(t, app, "a.svg", "image/svg+xml", content)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || !bytes.Equal(*got, content) {
		t.Errorf("status=%d identical=%v", resp.StatusCode, bytes.Equal(*got, content))
	}
}

func TestUploadObject_StripEnabled_CorruptImageRejected(t *testing.T) {
	app, s3 := newStripTestApp(t, true)
	called := false
	s3.UploadObjectFn = func(context.Context, string, string, io.Reader, string) (*models.ObjectUploadResponse, error) {
		called = true
		return &models.ObjectUploadResponse{}, nil
	}
	orig := jpegWithGPSExif(t)

	for name, data := range map[string][]byte{
		"truncated jpeg": orig[:len(orig)-20],
		"truncated png":  truncatedPNG(t),
	} {
		resp := postSingle(t, app, "x.bin", "application/octet-stream", data)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, resp.StatusCode)
		}
	}
	if called {
		t.Error("corrupt image was passed to storage (must fail closed)")
	}
}

func truncatedPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()[:buf.Len()-14]
}

func TestUploadMultiple_StripEnabled_CorruptFileReportedOthersUploaded(t *testing.T) {
	app, s3 := newStripTestApp(t, true)
	stored := map[string][]byte{}
	s3.UploadMultipleObjectsFn = func(_ context.Context, _ string, files []struct {
		Key         string
		Body        io.Reader
		ContentType string
	}) []services.UploadResult {
		out := make([]services.UploadResult, len(files))
		for i, f := range files {
			b, _ := io.ReadAll(f.Body)
			stored[f.Key] = b
			out[i] = services.UploadResult{Key: f.Key, Success: true, ContentType: f.ContentType, Size: int64(len(b))}
		}
		return out
	}
	body, ct := buildMultipartMulti(t, []struct {
		Filename, ContentType string
		Content               []byte
	}{
		{"good.jpg", "image/jpeg", jpegWithGPSExif(t)},
		{"bad.png", "image/png", truncatedPNG(t)},
		{"notes.txt", "text/plain", []byte("hello")},
	})
	req := httptest.NewRequest(http.MethodPost, "/buckets/b1/objects/upload-multiple", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMultiStatus {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 207\n%s", resp.StatusCode, raw)
	}
	var out struct {
		Data models.ObjectUploadMultipleResponse `json:"data"`
	}
	decodeJSON(t, resp.Body, &out)
	if out.Data.SuccessCount != 2 || out.Data.FailureCount != 1 || out.Data.FailedFiles[0].Key != "bad.png" {
		t.Errorf("response = %+v", out.Data)
	}
	if _, ok := stored["bad.png"]; ok {
		t.Error("corrupt image was stored")
	}
	if bytes.Contains(stored["good.jpg"], []byte("SECRET")) {
		t.Error("metadata stored for good.jpg")
	}
	if string(stored["notes.txt"]) != "hello" {
		t.Error("text file modified")
	}
}
