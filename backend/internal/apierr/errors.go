// Package apierr translates upstream (Garage Admin API, S3/MinIO) errors into
// backend API error responses with correct HTTP status codes and stable codes.
package apierr

import "fmt"

// UpstreamError is a typed error describing a failure reported by an upstream
// service. Returned from the services layer and consumed by handlers via Map
// or Respond.
type UpstreamError struct {
	HTTPStatus int
	Code       string            // upstream code, e.g. "BucketNotEmpty", "NoSuchKey"
	Message    string            // human-readable upstream message
	Source     string            // "garage" or "s3"
	Details    map[string]string // optional: region, path, bucket, key
}

func (e *UpstreamError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("%s: %s", e.Source, e.Message)
	}
	return fmt.Sprintf("%s %s: %s", e.Source, e.Code, e.Message)
}
