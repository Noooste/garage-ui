package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/pkg/utils"
)

// s3ErrorXML writes an S3-style error response that the MinIO SDK parses.
func s3ErrorXML(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error><Code>%s</Code><Message>%s</Message><Resource>/</Resource><RequestId>x</RequestId></Error>`, code, msg)
}

// newS3TestService builds an S3Service whose Endpoint points at a single
// httptest.Server handling BOTH Garage admin calls (for credential lookup)
// and S3 data-plane requests. Admin requests are routed by path prefix
// `/v2/` and `/health`; everything else is treated as an S3 call and
// dispatched to s3Handler.
func newS3TestService(t *testing.T, s3Handler http.Handler) *S3Service {
	t.Helper()

	secret := "s3-test-secret"
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/v2/GetBucketInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageBucketInfo{
			ID: "bid",
			Keys: []models.BucketKeyInfo{
				{AccessKeyID: "TESTAK", Permissions: models.BucketKeyPermission{Read: true, Write: true}},
			},
		})
	})
	adminMux.HandleFunc("/v2/GetKeyInfo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&models.GarageKeyInfo{
			AccessKeyID:     "TESTAK",
			SecretAccessKey: &secret,
		})
	})

	combined := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v2/") || r.URL.Path == "/health" {
			adminMux.ServeHTTP(w, r)
			return
		}
		s3Handler.ServeHTTP(w, r)
	})

	srv := httptest.NewServer(combined)
	t.Cleanup(srv.Close)

	admin := NewGarageV2AdminService(&config.GarageConfig{
		AdminEndpoint: srv.URL,
		AdminToken:    "test",
	}, "")

	// strip scheme for S3 endpoint (NewS3Service does this itself if http:// prefix)
	s3 := NewS3Service(&config.GarageConfig{
		Endpoint: srv.URL, // http://127.0.0.1:NNNN
		Region:   "garage",
	}, admin)
	return s3
}

// uniqueBucket2 returns a per-test bucket name so GlobalCache doesn't leak
// credentials between tests.
func uniqueBucket2(t *testing.T) string {
	t.Helper()
	name := "b-" + strings.ReplaceAll(t.Name(), "/", "-")
	t.Cleanup(func() { utils.GlobalCache.Delete("key:" + name) })
	return name
}

// fixedRequestCounter returns an http.Handler that always replies with the
// given S3 error, and counts requests.
func errS3Handler(status int, code string) (http.Handler, *int) {
	var count int
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		s3ErrorXML(w, status, code, code)
	}), &count
}

func TestS3_ListBuckets_ServerError(t *testing.T) {
	h, _ := errS3Handler(http.StatusInternalServerError, "InternalError")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := s3.ListBuckets(ctx)
	if err == nil {
		t.Fatal("expected error from ListBuckets, got nil")
	}
	if !strings.Contains(err.Error(), "failed to list buckets") {
		t.Errorf("error %v should wrap 'failed to list buckets'", err)
	}
}

func TestS3_CreateBucket_ServerError(t *testing.T) {
	h, _ := errS3Handler(http.StatusConflict, "BucketAlreadyExists")
	s3 := newS3TestService(t, h)
	_ = uniqueBucket2(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := s3.CreateBucket(ctx, "b-TestS3_CreateBucket_ServerError")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create bucket") {
		t.Errorf("error = %v, want wrap 'failed to create bucket'", err)
	}
}

func TestS3_DeleteBucket_ServerError(t *testing.T) {
	h, _ := errS3Handler(http.StatusNotFound, "NoSuchBucket")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := s3.DeleteBucket(ctx, "b-TestS3_DeleteBucket_ServerError")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete bucket") {
		t.Errorf("error = %v", err)
	}
}

func TestS3_ListObjects_ServerError(t *testing.T) {
	h, _ := errS3Handler(http.StatusForbidden, "AccessDenied")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := s3.ListObjects(ctx, "b-TestS3_ListObjects_ServerError", "", 0, "")
	if err == nil {
		t.Fatal("expected error from ListObjects, got nil")
	}
	if !strings.Contains(err.Error(), "failed to list objects") {
		t.Errorf("error = %v", err)
	}
}

func TestS3_UploadObject_ServerError(t *testing.T) {
	h, _ := errS3Handler(http.StatusForbidden, "AccessDenied")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := s3.UploadObject(ctx, "b-TestS3_UploadObject_ServerError", "k", bytes.NewReader([]byte("hi")), "text/plain")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to upload object") {
		t.Errorf("error = %v", err)
	}
}

func TestS3_CreateDirectoryMarker_ServerError(t *testing.T) {
	h, _ := errS3Handler(http.StatusForbidden, "AccessDenied")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := s3.CreateDirectoryMarker(ctx, "b-TestS3_CreateDirectoryMarker_ServerError", "folder/")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create directory") {
		t.Errorf("error = %v", err)
	}
}

func TestS3_GetObject_ServerError(t *testing.T) {
	h, _ := errS3Handler(http.StatusNotFound, "NoSuchKey")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, _, err := s3.GetObject(ctx, "b-TestS3_GetObject_ServerError", "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestS3_DeleteObject_ServerError(t *testing.T) {
	h, _ := errS3Handler(http.StatusForbidden, "AccessDenied")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := s3.DeleteObject(ctx, "b-TestS3_DeleteObject_ServerError", "k")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete object") {
		t.Errorf("error = %v", err)
	}
}

func TestS3_ObjectExists_NoSuchKeyReturnsFalseNil(t *testing.T) {
	h, _ := errS3Handler(http.StatusNotFound, "NoSuchKey")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	exists, err := s3.ObjectExists(ctx, "b-TestS3_ObjectExists_NoSuchKeyReturnsFalseNil", "k")
	if err != nil {
		t.Fatalf("ObjectExists returned error for NoSuchKey: %v", err)
	}
	if exists {
		t.Error("exists should be false for NoSuchKey")
	}
}

func TestS3_ObjectExists_OtherErrorPropagates(t *testing.T) {
	h, _ := errS3Handler(http.StatusForbidden, "AccessDenied")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := s3.ObjectExists(ctx, "b-TestS3_ObjectExists_OtherErrorPropagates", "k")
	if err == nil {
		t.Fatal("expected error for AccessDenied, got nil")
	}
}

func TestS3_GetObjectMetadata_ServerError(t *testing.T) {
	h, _ := errS3Handler(http.StatusNotFound, "NoSuchKey")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := s3.GetObjectMetadata(ctx, "b-TestS3_GetObjectMetadata_ServerError", "k")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get metadata") {
		t.Errorf("error = %v", err)
	}
}

func TestS3_DeleteMultipleObjects_EmptyKeysIsNoop(t *testing.T) {
	// No S3 handler should be called; use a handler that fails if invoked.
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		s3ErrorXML(w, http.StatusInternalServerError, "ShouldNotHappen", "")
	})
	s3 := newS3TestService(t, h)

	if n, err := s3.DeleteMultipleObjects(context.Background(), "whatever", nil); err != nil || n != 0 {
		t.Fatalf("empty keys should return (0, nil), got (%d, %v)", n, err)
	}
	if called {
		t.Error("S3 handler was invoked for empty-keys call")
	}
}

func TestS3_DeleteMultipleObjects_ServerErrorPropagates(t *testing.T) {
	h, _ := errS3Handler(http.StatusForbidden, "AccessDenied")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := s3.DeleteMultipleObjects(ctx, "b-TestS3_DeleteMultipleObjects_ServerErrorPropagates", []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// s3PrefixDeleteHandler serves a ListObjectsV2 response (GET) from listBody and
// a successful multi-object DeleteResult (POST /{bucket}?delete), recording how
// many batch-delete requests were made.
func s3PrefixDeleteHandler(listBody string, deletePosts *int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			*deletePosts++
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></DeleteResult>`)
			return
		}
		// Any GET is treated as a ListObjectsV2 request.
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, listBody)
	})
}

func TestS3_DeleteObjectsByPrefix_EmptyPrefixIsError(t *testing.T) {
	// A blank prefix must be rejected before any network call — it would
	// otherwise match (and delete) every object in the bucket.
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		s3ErrorXML(w, http.StatusInternalServerError, "ShouldNotHappen", "")
	})
	s3 := newS3TestService(t, h)

	n, err := s3.DeleteObjectsByPrefix(context.Background(), "b-TestS3_DeleteObjectsByPrefix_EmptyPrefixIsError", "")
	if err == nil {
		t.Fatal("empty prefix should return an error")
	}
	if n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
	if called {
		t.Error("no S3 request should be made for an empty prefix")
	}
}

func TestS3_DeleteObjectsByPrefix_ListsThenDeletes(t *testing.T) {
	contents := []struct {
		Key          string
		Size         int64
		LastModified string
		ETag         string
	}{
		{Key: "docs/a"}, {Key: "docs/b"}, {Key: "docs/sub/c"},
	}
	listBody := listBucketResultXML("b", false, "", contents, nil)
	deletePosts := 0
	s3 := newS3TestService(t, s3PrefixDeleteHandler(listBody, &deletePosts))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	n, err := s3.DeleteObjectsByPrefix(ctx, "b-TestS3_DeleteObjectsByPrefix_ListsThenDeletes", "docs/")
	if err != nil {
		t.Fatalf("DeleteObjectsByPrefix: %v", err)
	}
	if n != 3 {
		t.Errorf("deleted = %d, want 3 (all objects listed under the prefix)", n)
	}
	if deletePosts == 0 {
		t.Error("expected a batch-delete request to be made")
	}
}

func TestS3_DeleteObjectsByPrefix_NoObjectsReturnsZero(t *testing.T) {
	listBody := listBucketResultXML("b", false, "", nil, nil)
	deletePosts := 0
	s3 := newS3TestService(t, s3PrefixDeleteHandler(listBody, &deletePosts))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	n, err := s3.DeleteObjectsByPrefix(ctx, "b-TestS3_DeleteObjectsByPrefix_NoObjectsReturnsZero", "empty/")
	if err != nil {
		t.Fatalf("DeleteObjectsByPrefix: %v", err)
	}
	if n != 0 {
		t.Errorf("deleted = %d, want 0", n)
	}
	if deletePosts != 0 {
		t.Errorf("no batch-delete should be made when nothing matches, got %d", deletePosts)
	}
}

func TestS3_DeleteObjectsByPrefix_ListErrorPropagates(t *testing.T) {
	h, _ := errS3Handler(http.StatusForbidden, "AccessDenied")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := s3.DeleteObjectsByPrefix(ctx, "b-TestS3_DeleteObjectsByPrefix_ListErrorPropagates", "docs/")
	if err == nil {
		t.Fatal("expected the list error to propagate, got nil")
	}
}

func TestS3_GetPresignedURL_ReturnsURLWithoutServerCall(t *testing.T) {
	// Presign is purely local (no network round-trip). Any handler suffices.
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	got, err := s3.GetPresignedURL(ctx, "b-TestS3_GetPresignedURL_ReturnsURLWithoutServerCall", "k", 10*time.Minute)
	if err != nil {
		t.Fatalf("GetPresignedURL: %v", err)
	}
	u, perr := url.Parse(got)
	if perr != nil {
		t.Fatalf("returned URL is not parseable: %v", perr)
	}
	if u.Scheme == "" || u.Host == "" {
		t.Errorf("presigned URL missing scheme/host: %q", got)
	}
	if !strings.Contains(u.RawQuery, "X-Amz-Signature") {
		t.Errorf("presigned URL should contain X-Amz-Signature, got %q", got)
	}
	if called {
		t.Error("presign should not make a network call")
	}
}

// listBucketResultXML produces a ListBucketResult XML body that MinIO
// parses. ListObjectsV2 is keyed on the `list-type=2` query parameter.
func listBucketResultXML(bucket string, isTruncated bool, nextToken string, contents []struct {
	Key          string
	Size         int64
	LastModified string
	ETag         string
}, commonPrefixes []string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`)
	fmt.Fprintf(&b, `<Name>%s</Name>`, bucket)
	b.WriteString(`<Prefix></Prefix>`)
	fmt.Fprintf(&b, `<KeyCount>%d</KeyCount>`, len(contents))
	b.WriteString(`<MaxKeys>1000</MaxKeys>`)
	fmt.Fprintf(&b, `<IsTruncated>%t</IsTruncated>`, isTruncated)
	if nextToken != "" {
		fmt.Fprintf(&b, `<NextContinuationToken>%s</NextContinuationToken>`, nextToken)
	}
	for _, c := range contents {
		lm := c.LastModified
		if lm == "" {
			lm = "2024-01-01T00:00:00.000Z"
		}
		etag := c.ETag
		if etag == "" {
			etag = "d41d8cd98f00b204e9800998ecf8427e"
		}
		fmt.Fprintf(&b, `<Contents><Key>%s</Key><LastModified>%s</LastModified><ETag>"%s"</ETag><Size>%d</Size><StorageClass>STANDARD</StorageClass></Contents>`,
			c.Key, lm, etag, c.Size)
	}
	for _, p := range commonPrefixes {
		fmt.Fprintf(&b, `<CommonPrefixes><Prefix>%s</Prefix></CommonPrefixes>`, p)
	}
	b.WriteString(`</ListBucketResult>`)
	return b.String()
}

// s3ListHandler routes ListObjectsV2 (GET with list-type=2) to listBody
// and StatObject (HEAD) to a 200 response whose headers reflect statHeaders.
func s3ListHandler(listBody string, statHeaders map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			for k, v := range statHeaders {
				w.Header().Set(k, v)
			}
			if _, ok := statHeaders["Content-Length"]; !ok {
				w.Header().Set("Content-Length", "0")
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		// Treat any GET as a ListObjectsV2 request.
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, listBody)
	})
}

func TestS3_ListObjects_EmptyResult(t *testing.T) {
	xml := listBucketResultXML("b", false, "", nil, nil)
	s3 := newS3TestService(t, s3ListHandler(xml, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	got, err := s3.ListObjects(ctx, "b-TestS3_ListObjects_EmptyResult", "", 0, "")
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if got.Count != 0 || len(got.Objects) != 0 || len(got.Prefixes) != 0 {
		t.Errorf("expected empty listing, got %+v", got)
	}
}

func TestS3_ListObjects_ObjectsAndPrefixes(t *testing.T) {
	contents := []struct {
		Key          string
		Size         int64
		LastModified string
		ETag         string
	}{
		{Key: "file.txt", Size: 10},
	}
	xml := listBucketResultXML("b", false, "", contents, []string{"folder/"})
	s3 := newS3TestService(t, s3ListHandler(xml, map[string]string{
		"Content-Type":   "text/plain",
		"Content-Length": "10",
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	got, err := s3.ListObjects(ctx, "b-TestS3_ListObjects_ObjectsAndPrefixes", "", 0, "")
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if got.Count != 1 || got.Objects[0].Key != "file.txt" {
		t.Errorf("objects = %+v", got.Objects)
	}
	if len(got.Prefixes) != 1 || got.Prefixes[0] != "folder/" {
		t.Errorf("prefixes = %+v", got.Prefixes)
	}
	// ContentType from StatObject may or may not round-trip depending on
	// signature validation in the MinIO client; don't assert on it here.
}

func TestS3_ListObjects_DirectoryMarkerPromotedToPrefix(t *testing.T) {
	// A zero-byte key ending in "/" in Contents must be dropped from
	// Objects and promoted to Prefixes (unless already covered).
	contents := []struct {
		Key          string
		Size         int64
		LastModified string
		ETag         string
	}{
		{Key: "empty-folder/", Size: 0},
		{Key: "already/", Size: 0}, // duplicate of CommonPrefix — must not duplicate
		{Key: "real.txt", Size: 5},
	}
	xml := listBucketResultXML("b", true, "tokenXYZ", contents, []string{"already/"})
	s3 := newS3TestService(t, s3ListHandler(xml, map[string]string{"Content-Length": "5"}))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	got, err := s3.ListObjects(ctx, "b-TestS3_ListObjects_DirectoryMarkerPromotedToPrefix", "", 0, "")
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if got.Count != 1 || got.Objects[0].Key != "real.txt" {
		t.Errorf("Objects should contain only real.txt, got %+v", got.Objects)
	}
	// Prefixes should contain "already/" (from CommonPrefix) and "empty-folder/"
	// (promoted from Contents). "already/" must not appear twice.
	count := map[string]int{}
	for _, p := range got.Prefixes {
		count[p]++
	}
	if count["already/"] != 1 {
		t.Errorf("Prefixes contains 'already/' %d times, want 1: %v", count["already/"], got.Prefixes)
	}
	if count["empty-folder/"] != 1 {
		t.Errorf("Prefixes missing 'empty-folder/': %v", got.Prefixes)
	}
	if !got.IsTruncated || got.NextContinuationToken != "tokenXYZ" {
		t.Errorf("pagination fields not propagated: IsTruncated=%v Token=%q", got.IsTruncated, got.NextContinuationToken)
	}
}

func TestS3_ListObjects_MarkerMatchingPrefixIsDropped(t *testing.T) {
	// When listing a specific prefix, a marker whose key == the listing
	// prefix is the folder itself — it must not render as a child of itself.
	contents := []struct {
		Key          string
		Size         int64
		LastModified string
		ETag         string
	}{
		{Key: "mydir/", Size: 0}, // matches prefix — must be dropped entirely
	}
	xml := listBucketResultXML("b", false, "", contents, nil)
	s3 := newS3TestService(t, s3ListHandler(xml, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	got, err := s3.ListObjects(ctx, "b-TestS3_ListObjects_MarkerMatchingPrefixIsDropped", "mydir/", 0, "")
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(got.Objects) != 0 || len(got.Prefixes) != 0 {
		t.Errorf("marker equal to prefix should be dropped entirely, got %+v", got)
	}
}

func TestS3_ListObjects_StatObjectFailureLeavesContentTypeEmpty(t *testing.T) {
	contents := []struct {
		Key          string
		Size         int64
		LastModified string
		ETag         string
	}{
		{Key: "f.bin", Size: 100},
	}
	xml := listBucketResultXML("b", false, "", contents, nil)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			// StatObject fails — return 403.
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, xml)
	})
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := s3.ListObjects(ctx, "b-TestS3_ListObjects_StatObjectFailureLeavesContentTypeEmpty", "", 0, "")
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if got.Count != 1 || got.Objects[0].Key != "f.bin" {
		t.Fatalf("unexpected object list %+v", got.Objects)
	}
	if got.Objects[0].ContentType != "" {
		t.Errorf("ContentType = %q, want empty on StatObject failure", got.Objects[0].ContentType)
	}
}

func TestS3_UploadMultipleObjects_PerFileFailuresRecorded(t *testing.T) {
	h, _ := errS3Handler(http.StatusForbidden, "AccessDenied")
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results := s3.UploadMultipleObjects(ctx, "b-TestS3_UploadMultipleObjects_PerFileFailuresRecorded", []struct {
		Key         string
		Body        io.Reader
		ContentType string
	}{
		{Key: "a", Body: bytes.NewReader([]byte("hello")), ContentType: "text/plain"},
		{Key: "b", Body: bytes.NewReader([]byte("world")), ContentType: "text/plain"},
	})

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for i, r := range results {
		if r.Success {
			t.Errorf("result[%d] should not be successful: %+v", i, r)
		}
		if r.Error == nil {
			t.Errorf("result[%d] should have Error set", i)
		}
	}
}
