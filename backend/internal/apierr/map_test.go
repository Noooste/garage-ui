package apierr

import (
	"errors"
	"testing"

	"Noooste/garage-ui/internal/models"
)

func TestMap_TableDriven(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{
			name:       "BucketNotEmpty",
			err:        &UpstreamError{HTTPStatus: 409, Code: "BucketNotEmpty", Message: "Tried to delete a non-empty bucket", Source: "garage"},
			wantStatus: 409,
			wantCode:   models.ErrCodeBucketNotEmpty,
			wantMsg:    "Tried to delete a non-empty bucket",
		},
		{
			name:       "NoSuchBucket",
			err:        &UpstreamError{HTTPStatus: 404, Code: "NoSuchBucket", Message: "missing", Source: "s3"},
			wantStatus: 404,
			wantCode:   models.ErrCodeBucketNotFound,
			wantMsg:    "missing",
		},
		{
			name:       "BucketAlreadyExists",
			err:        &UpstreamError{HTTPStatus: 409, Code: "BucketAlreadyExists", Message: "dup", Source: "s3"},
			wantStatus: 409,
			wantCode:   models.ErrCodeBucketExists,
			wantMsg:    "dup",
		},
		{
			name:       "BucketAlreadyOwnedByYou",
			err:        &UpstreamError{HTTPStatus: 409, Code: "BucketAlreadyOwnedByYou", Message: "yours", Source: "s3"},
			wantStatus: 409,
			wantCode:   models.ErrCodeBucketExists,
			wantMsg:    "yours",
		},
		{
			name:       "NoSuchKey",
			err:        &UpstreamError{HTTPStatus: 404, Code: "NoSuchKey", Message: "gone", Source: "s3"},
			wantStatus: 404,
			wantCode:   models.ErrCodeObjectNotFound,
			wantMsg:    "gone",
		},
		{
			name:       "InvalidBucketName",
			err:        &UpstreamError{HTTPStatus: 400, Code: "InvalidBucketName", Message: "bad", Source: "s3"},
			wantStatus: 400,
			wantCode:   models.ErrCodeInvalidBucketName,
			wantMsg:    "bad",
		},
		{
			name:       "AccessDenied",
			err:        &UpstreamError{HTTPStatus: 403, Code: "AccessDenied", Message: "nope", Source: "garage"},
			wantStatus: 403,
			wantCode:   models.ErrCodeForbidden,
			wantMsg:    "nope",
		},
		{
			name:       "UnknownCodeWithStatus",
			err:        &UpstreamError{HTTPStatus: 503, Code: "SomethingWeird", Message: "weird", Source: "garage"},
			wantStatus: 503,
			wantCode:   models.ErrCodeInternalError,
			wantMsg:    "weird",
		},
		{
			name:       "UnknownCodeNoStatus",
			err:        &UpstreamError{HTTPStatus: 0, Code: "", Message: "boom", Source: "garage"},
			wantStatus: 500,
			wantCode:   models.ErrCodeInternalError,
			wantMsg:    "boom",
		},
		{
			name:       "NonUpstreamError",
			err:        errors.New("plain"),
			wantStatus: 500,
			wantCode:   models.ErrCodeInternalError,
			wantMsg:    "plain",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, code, msg := Map(tc.err)
			if status != tc.wantStatus {
				t.Errorf("status = %d, want %d", status, tc.wantStatus)
			}
			if code != tc.wantCode {
				t.Errorf("code = %q, want %q", code, tc.wantCode)
			}
			if msg != tc.wantMsg {
				t.Errorf("msg = %q, want %q", msg, tc.wantMsg)
			}
		})
	}
}
