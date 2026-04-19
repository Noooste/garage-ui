package apierr

import (
	"errors"

	"Noooste/garage-ui/internal/models"

	"github.com/gofiber/fiber/v3"
)

// codeEntry defines how an upstream code translates to the API surface.
type codeEntry struct {
	HTTPStatus int
	APICode    string
}

// upstreamCodeTable maps upstream codes (Garage + S3 share the same naming for
// most codes) to (httpStatus, apiCode). Extend here as new upstream codes are
// discovered.
var upstreamCodeTable = map[string]codeEntry{
	"BucketNotEmpty":          {409, models.ErrCodeBucketNotEmpty},
	"NoSuchBucket":            {404, models.ErrCodeBucketNotFound},
	"BucketAlreadyExists":     {409, models.ErrCodeBucketExists},
	"BucketAlreadyOwnedByYou": {409, models.ErrCodeBucketExists},
	"NoSuchKey":               {404, models.ErrCodeObjectNotFound},
	"InvalidBucketName":       {400, models.ErrCodeInvalidBucketName},
	"AccessDenied":            {403, models.ErrCodeForbidden},
}

// Map translates an error into (httpStatus, apiCode, message) for the API
// response. Falls back to 500 / INTERNAL_ERROR for non-UpstreamError values.
func Map(err error) (int, string, string) {
	var ue *UpstreamError
	if !errors.As(err, &ue) {
		return 500, models.ErrCodeInternalError, err.Error()
	}

	if entry, ok := upstreamCodeTable[ue.Code]; ok {
		return entry.HTTPStatus, entry.APICode, ue.Message
	}

	status := ue.HTTPStatus
	if status == 0 {
		status = 500
	}
	msg := ue.Message
	if msg == "" {
		msg = ue.Error()
	}
	return status, models.ErrCodeInternalError, msg
}

// Respond writes the mapped error as a Fiber JSON response using the standard
// APIResponse envelope. Handlers should call this from their err != nil
// branches for all upstream failures.
func Respond(c fiber.Ctx, err error) error {
	status, code, msg := Map(err)
	return c.Status(status).JSON(models.ErrorResponse(code, msg))
}
