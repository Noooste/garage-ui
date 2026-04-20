package apierr

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/Noooste/azuretls-client"
	"github.com/minio/minio-go/v7"
)

// garageErrBody mirrors Garage's JSON error envelope.
type garageErrBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Region  string `json:"region"`
	Path    string `json:"path"`
}

// ParseGarage returns nil for 2xx responses. For non-2xx it reads the body
// once, JSON-decodes the structured Garage error envelope, and returns a
// *UpstreamError. Malformed bodies are preserved verbatim in Message.
//
// ParseGarage DOES NOT close resp.RawBody on the success path — callers that
// decode the success body (decodeResponse in services/admin.go) still need
// access and are responsible for closing. On the error path the body is fully
// consumed before return.
func ParseGarage(resp *azuretls.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.RawBody)

	ue := &UpstreamError{
		HTTPStatus: resp.StatusCode,
		Source:     "garage",
	}

	var parsed garageErrBody
	if len(bodyBytes) > 0 && json.Unmarshal(bodyBytes, &parsed) == nil && parsed.Code != "" {
		ue.Code = parsed.Code
		ue.Message = parsed.Message
		ue.Details = map[string]string{}
		if parsed.Region != "" {
			ue.Details["region"] = parsed.Region
		}
		if parsed.Path != "" {
			ue.Details["path"] = parsed.Path
		}
	} else {
		ue.Message = string(bodyBytes)
	}
	return ue
}

// FromMinio converts a MinIO/S3 error into an *UpstreamError. Returns nil when
// err is nil. If err is not a minio.ErrorResponse (e.g. a network error), a
// generic 500 *UpstreamError is returned with the raw error string as Message.
func FromMinio(err error) *UpstreamError {
	if err == nil {
		return nil
	}

	var mer minio.ErrorResponse
	if errors.As(err, &mer) {
		details := map[string]string{}
		if mer.BucketName != "" {
			details["bucket"] = mer.BucketName
		}
		if mer.Key != "" {
			details["key"] = mer.Key
		}
		status := mer.StatusCode
		if status == 0 {
			status = 500
		}
		return &UpstreamError{
			HTTPStatus: status,
			Code:       mer.Code,
			Message:    mer.Message,
			Source:     "s3",
			Details:    details,
		}
	}

	return &UpstreamError{
		HTTPStatus: 500,
		Source:     "s3",
		Message:    err.Error(),
	}
}
