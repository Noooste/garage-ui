package apierr

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Noooste/azuretls-client"
	"github.com/minio/minio-go/v7"
)

func fakeResp(status int, body string) *azuretls.Response {
	return &azuretls.Response{
		StatusCode: status,
		RawBody:    io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestParseGarage_Success(t *testing.T) {
	resp := fakeResp(200, `{"foo":"bar"}`)
	if err := ParseGarage(resp); err != nil {
		t.Fatalf("ParseGarage(2xx) returned %v, want nil", err)
	}
}

func TestParseGarage_StructuredError(t *testing.T) {
	body := `{"code":"BucketNotEmpty","message":"Tried to delete a non-empty bucket","region":"eu-west-1","path":"/v2/DeleteBucket"}`
	resp := fakeResp(409, body)

	err := ParseGarage(resp)
	ue, ok := err.(*UpstreamError)
	if !ok {
		t.Fatalf("ParseGarage returned %T, want *UpstreamError", err)
	}
	if ue.HTTPStatus != 409 {
		t.Errorf("HTTPStatus = %d, want 409", ue.HTTPStatus)
	}
	if ue.Code != "BucketNotEmpty" {
		t.Errorf("Code = %q, want BucketNotEmpty", ue.Code)
	}
	if ue.Message != "Tried to delete a non-empty bucket" {
		t.Errorf("Message = %q", ue.Message)
	}
	if ue.Source != "garage" {
		t.Errorf("Source = %q, want garage", ue.Source)
	}
	if ue.Details["region"] != "eu-west-1" || ue.Details["path"] != "/v2/DeleteBucket" {
		t.Errorf("Details = %v", ue.Details)
	}
}

func TestParseGarage_MalformedBody(t *testing.T) {
	resp := fakeResp(500, "not json at all")

	err := ParseGarage(resp)
	ue, ok := err.(*UpstreamError)
	if !ok {
		t.Fatalf("ParseGarage returned %T, want *UpstreamError", err)
	}
	if ue.HTTPStatus != 500 || ue.Code != "" {
		t.Errorf("HTTPStatus=%d Code=%q, want 500 and empty code", ue.HTTPStatus, ue.Code)
	}
	if !strings.Contains(ue.Message, "not json at all") {
		t.Errorf("Message = %q, expected raw body", ue.Message)
	}
}

func TestParseGarage_EmptyBody(t *testing.T) {
	resp := fakeResp(502, "")

	err := ParseGarage(resp)
	ue, ok := err.(*UpstreamError)
	if !ok {
		t.Fatalf("ParseGarage returned %T, want *UpstreamError", err)
	}
	if ue.HTTPStatus != 502 {
		t.Errorf("HTTPStatus = %d, want 502", ue.HTTPStatus)
	}
}

func TestFromMinio_NilInput(t *testing.T) {
	if got := FromMinio(nil); got != nil {
		t.Fatalf("FromMinio(nil) = %v, want nil", got)
	}
}

func TestFromMinio_MinioErrorResponse(t *testing.T) {
	in := minio.ErrorResponse{
		StatusCode: 404,
		Code:       "NoSuchBucket",
		Message:    "The specified bucket does not exist",
		BucketName: "missing",
	}
	got := FromMinio(in)
	if got == nil {
		t.Fatal("FromMinio returned nil")
	}
	if got.HTTPStatus != 404 {
		t.Errorf("HTTPStatus = %d, want 404", got.HTTPStatus)
	}
	if got.Code != "NoSuchBucket" {
		t.Errorf("Code = %q, want NoSuchBucket", got.Code)
	}
	if got.Message != "The specified bucket does not exist" {
		t.Errorf("Message = %q", got.Message)
	}
	if got.Source != "s3" {
		t.Errorf("Source = %q, want s3", got.Source)
	}
	if got.Details["bucket"] != "missing" {
		t.Errorf("Details[bucket] = %q, want missing", got.Details["bucket"])
	}
}

func TestFromMinio_NonMinioError(t *testing.T) {
	got := FromMinio(errors.New("connection refused"))
	if got == nil {
		t.Fatal("FromMinio returned nil")
	}
	if got.HTTPStatus != 500 {
		t.Errorf("HTTPStatus = %d, want 500", got.HTTPStatus)
	}
	if got.Code != "" {
		t.Errorf("Code = %q, want empty", got.Code)
	}
	if !strings.Contains(got.Message, "connection refused") {
		t.Errorf("Message = %q", got.Message)
	}
	if got.Source != "s3" {
		t.Errorf("Source = %q, want s3", got.Source)
	}
}
