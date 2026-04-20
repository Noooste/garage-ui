package apierr

import (
	"errors"
	"testing"
)

func TestUpstreamError_Error(t *testing.T) {
	e := &UpstreamError{
		HTTPStatus: 409,
		Code:       "BucketNotEmpty",
		Message:    "Tried to delete a non-empty bucket",
		Source:     "garage",
	}
	got := e.Error()
	want := "garage BucketNotEmpty: Tried to delete a non-empty bucket"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestUpstreamError_ErrorWithoutCode(t *testing.T) {
	e := &UpstreamError{HTTPStatus: 500, Source: "garage", Message: "server went boom"}
	got := e.Error()
	want := "garage: server went boom"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestUpstreamError_ErrorsAs(t *testing.T) {
	var base error = &UpstreamError{HTTPStatus: 404, Code: "NoSuchBucket", Source: "garage"}
	var target *UpstreamError
	if !errors.As(base, &target) {
		t.Fatal("errors.As should have matched *UpstreamError")
	}
	if target.Code != "NoSuchBucket" {
		t.Fatalf("Code = %q, want NoSuchBucket", target.Code)
	}
}
