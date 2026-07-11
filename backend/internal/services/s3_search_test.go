package services

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// s3SearchHandler serves a two-page recursive ListObjectsV2 response and counts
// HEAD (StatObject) requests. Page selection is driven by the
// `continuation-token` query parameter, mirroring how the MinIO SDK paginates.
func s3SearchHandler(bucket string, page1, page2 []struct {
	Key          string
	Size         int64
	LastModified string
	ETag         string
}) (http.Handler, *int, *string) {
	var heads int
	var lastDelimiter string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			heads++
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusOK)
			return
		}
		lastDelimiter = r.URL.Query().Get("delimiter")
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("continuation-token") == "PAGE2" {
			_, _ = io.WriteString(w, listBucketResultXML(bucket, false, "", page2, nil))
			return
		}
		_, _ = io.WriteString(w, listBucketResultXML(bucket, true, "PAGE2", page1, nil))
	})
	return h, &heads, &lastDelimiter
}

// TestS3_SearchObjects_FindsMatchOnLaterPage reproduces issue #87: an object
// that lives on the second page of a listing must still be found by search.
// SearchObjects should page through the whole listing recursively.
func TestS3_SearchObjects_FindsMatchOnLaterPage(t *testing.T) {
	bucket := "b-TestS3_SearchObjects_FindsMatchOnLaterPage"
	page1 := []struct {
		Key          string
		Size         int64
		LastModified string
		ETag         string
	}{
		{Key: "docs/alpha.txt", Size: 10},
		{Key: "docs/beta.txt", Size: 10},
	}
	page2 := []struct {
		Key          string
		Size         int64
		LastModified string
		ETag         string
	}{
		{Key: "docs/target-report.pdf", Size: 20},
		{Key: "docs/gamma.txt", Size: 10},
	}
	h, heads, lastDelimiter := s3SearchHandler(bucket, page1, page2)
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := s3.SearchObjects(ctx, bucket, "", "target")
	if err != nil {
		t.Fatalf("SearchObjects: %v", err)
	}
	if got.Count != 1 || len(got.Objects) != 1 || got.Objects[0].Key != "docs/target-report.pdf" {
		t.Fatalf("expected to find docs/target-report.pdf on page 2, got %+v", got.Objects)
	}
	// Search must be recursive (no delimiter) so it descends into folders.
	if *lastDelimiter != "" {
		t.Errorf("delimiter = %q, want empty (recursive listing)", *lastDelimiter)
	}
	// Search must not fetch ContentType per object — that would be N stat calls.
	if *heads != 0 {
		t.Errorf("StatObject (HEAD) calls = %d, want 0 during search", *heads)
	}
}

// TestS3_SearchObjects_ExcludesDirectoryMarkersAndIsCaseInsensitive verifies
// that zero-byte directory markers never appear as matches and that matching
// is a case-insensitive substring test on the full key.
func TestS3_SearchObjects_ExcludesDirectoryMarkersAndIsCaseInsensitive(t *testing.T) {
	bucket := "b-TestS3_SearchObjects_ExcludesDirectoryMarkers"
	page1 := []struct {
		Key          string
		Size         int64
		LastModified string
		ETag         string
	}{
		{Key: "Reports/", Size: 0},              // directory marker — must be excluded
		{Key: "Reports/Q1-REPORT.csv", Size: 5}, // matches "report" case-insensitively
		{Key: "images/logo.png", Size: 5},       // no match
	}
	h, _, _ := s3SearchHandler(bucket, page1, nil)
	s3 := newS3TestService(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := s3.SearchObjects(ctx, bucket, "", "report")
	if err != nil {
		t.Fatalf("SearchObjects: %v", err)
	}
	if got.Count != 1 || got.Objects[0].Key != "Reports/Q1-REPORT.csv" {
		t.Fatalf("expected only Reports/Q1-REPORT.csv, got %+v", got.Objects)
	}
	for _, o := range got.Objects {
		if strings.HasSuffix(o.Key, "/") {
			t.Errorf("directory marker leaked into results: %q", o.Key)
		}
	}
}
