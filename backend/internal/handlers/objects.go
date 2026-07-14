package handlers

import (
	"bufio"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"

	"github.com/gofiber/fiber/v3"
)

// unsafeInlineContentTypes are MIME types that a browser can execute as
// JavaScript in the response's origin when rendered inline. Since the SPA is
// served from the same origin as the API, any uploader could otherwise plant
// stored XSS by uploading a file with one of these Content-Types.
var unsafeInlineContentTypes = map[string]struct{}{
	"text/html":              {},
	"application/xhtml+xml":  {},
	"image/svg+xml":          {},
	"application/xml":        {},
	"text/xml":               {},
	"application/javascript": {},
	"text/javascript":        {},
}

// safeContentType rewrites Content-Types that the browser would treat as
// executable to application/octet-stream.
func safeContentType(ct string) string {
	base := strings.TrimSpace(strings.ToLower(ct))
	if i := strings.IndexByte(base, ';'); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}
	if _, bad := unsafeInlineContentTypes[base]; bad {
		return "application/octet-stream"
	}
	return ct
}

// contentDispositionHeader builds an RFC 6266 / RFC 5987 Content-Disposition
// header value with the user-controlled object key safely encoded. Strips
// path components and control characters before emitting the ASCII fallback,
// then appends the percent-encoded UTF-8 filename*= for full fidelity.
func contentDispositionHeader(disposition, key string) string {
	name := path.Base(key)
	if name == "." || name == "/" || name == "" {
		name = "download"
	}
	// ASCII-safe fallback: drop anything that could break the quoted value.
	var asciiFallback strings.Builder
	for _, r := range name {
		if r < 0x20 || r == 0x7f || r == '"' || r == '\\' || r > 0x7e {
			asciiFallback.WriteByte('_')
			continue
		}
		asciiFallback.WriteRune(r)
	}
	fallback := asciiFallback.String()
	if fallback == "" {
		fallback = "download"
	}
	encoded := url.PathEscape(name)
	return disposition + "; filename=\"" + fallback + "\"; filename*=UTF-8''" + encoded
}

// ObjectHandler handles object-related HTTP requests.
type ObjectHandler struct {
	s3Service services.S3Storage
}

// NewObjectHandler creates a new object handler.
func NewObjectHandler(s3Service services.S3Storage) *ObjectHandler {
	return &ObjectHandler{
		s3Service: s3Service,
	}
}

// ListObjects lists objects in a bucket with optional filtering and pagination
//
//	@Summary		List objects in a bucket
//	@Description	Retrieves a list of objects and prefixes (folders) stored in the specified bucket, with optional filtering by prefix, pagination support, and max keys
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket				path		string												true	"Name of the bucket to list objects from"
//	@Param			prefix				query		string												false	"Filter objects by prefix"
//	@Param			search				query		string												false	"Recursively search object keys under prefix by case-insensitive substring (best-effort; bypasses max_keys and continuation_token)"
//	@Param			max_keys			query		int													false	"Maximum number of objects to return (default: 100)"
//	@Param			continuation_token	query		string												false	"Token for pagination to retrieve next page of results"
//	@Success		200					{object}	models.APIResponse{data=models.ObjectListResponse}	"Successfully retrieved list of objects and prefixes"
//	@Failure		400					{object}	models.APIResponse{error=models.APIError}			"Invalid request parameters"
//	@Failure		404					{object}	models.APIResponse{error=models.APIError}			"Bucket not found"
//	@Failure		500					{object}	models.APIResponse{error=models.APIError}			"Failed to list objects"
//	@Router			/api/v1/buckets/{bucket}/objects [get]
func (h *ObjectHandler) ListObjects(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameter
	bucketName := c.Params("bucket")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	// Get query parameters for filtering and pagination
	prefix := c.Query("prefix", "")

	// Search mode: a recursive, best-effort substring search across the whole
	// subtree under prefix. S3/Garage has no server-side substring search, so
	// the backend scans and filters. This bypasses page-token pagination and
	// max_keys, see S3Service.SearchObjects -> pagination is handled frontend side.
	if search := c.Query("search", ""); search != "" {
		results, err := h.s3Service.SearchObjects(ctx, bucketName, prefix, search)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(
				models.ErrorResponse(models.ErrCodeListFailed, "Failed to search objects: "+err.Error()),
			)
		}
		return c.JSON(models.SuccessResponse(results))
	}

	continuationToken := c.Query("continuation_token", "")

	maxKeysStr := c.Query("max_keys", "100")
	maxKeys, err := strconv.Atoi(maxKeysStr)
	if err != nil || maxKeys <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid max_keys parameter"),
		)
	}

	// List objects in the bucket
	objects, err := h.s3Service.ListObjects(ctx, bucketName, prefix, maxKeys, continuationToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeListFailed, "Failed to list objects: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(objects))
}

// UploadObject uploads an object to a bucket
//
//	@Summary		Upload object to bucket
//	@Description	Uploads an object to the specified bucket using multipart/form-data
//	@Tags			Objects
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			bucket	path		string													true	"Name of the bucket to upload the object to"
//	@Param			file	formData	file													true	"File to upload"
//	@Param			key		formData	string													false	"Object key (path in bucket). If not provided, the filename will be used"
//	@Success		201		{object}	models.APIResponse{data=models.ObjectUploadResponse}	"Object uploaded successfully"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}				"Invalid request parameters"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}				"Bucket not found"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}				"Failed to upload object"
//	@Router			/api/v1/buckets/{bucket}/objects [post]
func (h *ObjectHandler) UploadObject(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameter
	bucketName := c.Params("bucket")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	// Get file from multipart form
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "File is required: "+err.Error()),
		)
	}

	// Get object key (path in bucket)
	key := c.FormValue("key")
	if key == "" {
		// Use filename as key if not provided
		key = file.Filename
	}

	// Open the uploaded file
	fileHandle, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeUploadFailed, "Failed to open uploaded file: "+err.Error()),
		)
	}
	defer fileHandle.Close()

	// Get content type
	contentType := file.Header.Get("Content-Type")

	// Upload to Garage
	uploadResult, err := h.s3Service.UploadObject(ctx, bucketName, key, fileHandle, contentType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeUploadFailed, "Failed to upload object: "+err.Error()),
		)
	}

	return c.Status(fiber.StatusCreated).JSON(models.SuccessResponse(uploadResult))
}

// CreateDirectory creates an empty directory marker in a bucket.
//
//	@Summary		Create directory in bucket
//	@Description	Creates a zero-byte object whose key ends with "/" so that S3 clients display it as an empty folder.
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket	path		string												true	"Name of the bucket"
//	@Param			request	body		object{key=string}									true	"Directory key (must end with '/')"
//	@Success		201		{object}	models.APIResponse{data=models.ObjectUploadResponse}	"Directory created"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}			"Invalid request parameters"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}			"Failed to create directory"
//	@Router			/api/v1/buckets/{bucket}/directories [post]
func (h *ObjectHandler) CreateDirectory(c fiber.Ctx) error {
	ctx := c.Context()

	bucketName := c.Params("bucket")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	var req struct {
		Key string `json:"key"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body: "+err.Error()),
		)
	}

	key := strings.TrimLeft(req.Key, "/")
	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Directory key is required"),
		)
	}
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}

	result, err := h.s3Service.CreateDirectoryMarker(ctx, bucketName, key)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeUploadFailed, "Failed to create directory: "+err.Error()),
		)
	}

	return c.Status(fiber.StatusCreated).JSON(models.SuccessResponse(result))
}

// GetObject retrieves an object from a bucket
//
//	@Summary		Get object from bucket
//	@Description	Retrieves an object stored in the specified bucket
//	@Tags			Objects
//	@Accept			json
//	@Produce		application/octet-stream
//	@Param			bucket		path		string										true	"Name of the bucket containing the object"
//	@Param			key			path		string										true	"Key (path) of the object"
//	@Param			download	query		bool										false	"Set to true to download the object as an attachment"
//	@Success		200			{file}		binary										"Successfully retrieved the object"
//	@Failure		400			{object}	models.APIResponse{error=models.APIError}	"Bucket name and object key are required"
//	@Failure		404			{object}	models.APIResponse{error=models.APIError}	"Object not found"
//	@Router			/api/v1/buckets/{bucket}/objects/{key} [get]
func (h *ObjectHandler) GetObject(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameters
	bucketName := c.Params("bucket")

	// Get object key from locals (set by route handler) or from params
	key, ok := c.Locals("objectKey").(string)
	if !ok || key == "" {
		key = c.Params("key")
	}

	if bucketName == "" || key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name and object key are required"),
		)
	}

	// Get object from Garage
	body, objectInfo, err := h.s3Service.GetObject(ctx, bucketName, key)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(
			models.ErrorResponse(models.ErrCodeObjectNotFound, "Object not found: "+err.Error()),
		)
	}

	// The uploader controls Content-Type. Rewrite executable MIME types to
	// application/octet-stream and always disable sniffing so stored HTML/SVG
	// cannot run as XSS in the SPA origin when fetched inline.
	c.Set("Content-Type", safeContentType(objectInfo.ContentType))
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Content-Length", strconv.FormatInt(objectInfo.Size, 10))
	c.Set("ETag", objectInfo.ETag)
	c.Set("Last-Modified", objectInfo.LastModified.Format(time.RFC1123))

	// The object key is attacker-controlled — build the header via the safe
	// RFC 6266 helper to avoid quote/semicolon injection into filename=.
	disposition := "inline"
	if c.Query("download") == "true" {
		disposition = "attachment"
	}
	c.Set("Content-Disposition", contentDispositionHeader(disposition, key))

	// Stream the object body to the client without buffering the entire file
	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer body.Close()
		io.Copy(w, body)
	})
}

// DeleteObject deletes an object from a bucket
//
//	@Summary		Delete object from bucket
//	@Description	Deletes an object stored in the specified bucket
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket	path		string													true	"Name of the bucket containing the object"
//	@Param			key		path		string													true	"Key (path) of the object"
//	@Success		200		{object}	models.APIResponse{data=models.ObjectDeleteResponse}	"Successfully deleted the object"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}				"Bucket name and object key are required"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}				"Object not found"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}				"Failed to delete object"
//	@Router			/api/v1/buckets/{bucket}/objects/{key} [delete]
func (h *ObjectHandler) DeleteObject(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameters
	bucketName := c.Params("bucket")

	// Get object key from locals (set by route handler) or from params
	key, ok := c.Locals("objectKey").(string)
	if !ok || key == "" {
		key = c.Params("key")
	}

	if bucketName == "" || key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name and object key are required"),
		)
	}

	// Check if object exists
	exists, err := h.s3Service.ObjectExists(ctx, bucketName, key)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to check object existence: "+err.Error()),
		)
	}

	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(
			models.ErrorResponse(models.ErrCodeObjectNotFound, "Object not found"),
		)
	}

	// Delete the object
	if err := h.s3Service.DeleteObject(ctx, bucketName, key); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeDeleteFailed, "Failed to delete object: "+err.Error()),
		)
	}

	// Return success response
	response := models.ObjectDeleteResponse{
		Bucket:  bucketName,
		Key:     key,
		Deleted: true,
	}

	return c.JSON(models.SuccessResponse(response))
}

// GetObjectMetadata returns metadata for an object without downloading it
//
//	@Summary		Get object metadata
//	@Description	Retrieves metadata information about an object without downloading the actual content
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket	path		string										true	"Name of the bucket containing the object"
//	@Param			key		path		string										true	"Key (path) of the object"
//	@Success		200		{object}	models.APIResponse{data=models.ObjectInfo}	"Successfully retrieved object metadata"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}	"Bucket name and object key are required"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}	"Object not found"
//	@Router			/api/v1/buckets/{bucket}/objects/{key}/metadata [get]
func (h *ObjectHandler) GetObjectMetadata(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameters
	bucketName := c.Params("bucket")

	// Get object key from locals (set by route handler) or from params
	key, ok := c.Locals("objectKey").(string)
	if !ok || key == "" {
		key = c.Params("key")
	}

	if bucketName == "" || key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name and object key are required"),
		)
	}

	// Get object metadata
	metadata, err := h.s3Service.GetObjectMetadata(ctx, bucketName, key)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(
			models.ErrorResponse(models.ErrCodeObjectNotFound, "Object not found: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(metadata))
}

// GetPresignedURL generates a pre-signed URL for accessing an object
//
//	@Summary		Get pre-signed URL for object
//	@Description	Generates a pre-signed URL that allows temporary access to the specified object
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket		path		string													true	"Name of the bucket containing the object"
//	@Param			key			path		string													true	"Key (path) of the object"
//	@Param			expires_in	query		int														false	"Expiration time in seconds for the pre-signed URL (default: 3600 seconds)"
//	@Success		200			{object}	models.APIResponse{data=models.PresignedURLResponse}	"Successfully generated pre-signed URL"
//	@Failure		400			{object}	models.APIResponse{error=models.APIError}				"Invalid request parameters"
//	@Failure		404			{object}	models.APIResponse{error=models.APIError}				"Object not found"
//	@Failure		500			{object}	models.APIResponse{error=models.APIError}				"Failed to generate pre-signed URL"
//	@Router			/api/v1/buckets/{bucket}/objects/{key}/presigned-url [get]
func (h *ObjectHandler) GetPresignedURL(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameters
	bucketName := c.Params("bucket")

	// Get object key from locals (set by route handler) or from params
	key, ok := c.Locals("objectKey").(string)
	if !ok || key == "" {
		key = c.Params("key")
	}

	if bucketName == "" || key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name and object key are required"),
		)
	}

	// Get expiration time from query parameter (default: 1 hour)
	expiresInStr := c.Query("expires_in", "3600")
	expiresIn, err := strconv.ParseInt(expiresInStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid expiration time: "+err.Error()),
		)
	}

	// Validate expiration time (1 second to 7 days)
	if expiresIn <= 0 || expiresIn > 604800 { // Max 7 days
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid expiration time (must be between 1 and 604800 seconds)"),
		)
	}

	// Check if object exists
	exists, err := h.s3Service.ObjectExists(ctx, bucketName, key)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to check object existence: "+err.Error()),
		)
	}

	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(
			models.ErrorResponse(models.ErrCodeObjectNotFound, "Object not found"),
		)
	}

	// Generate pre-signed URL
	url, err := h.s3Service.GetPresignedURL(ctx, bucketName, key, time.Duration(expiresIn)*time.Second)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to generate pre-signed URL: "+err.Error()),
		)
	}

	response := models.PresignedURLResponse{
		URL:       url,
		ExpiresIn: expiresIn,
		Bucket:    bucketName,
		Key:       key,
	}

	return c.JSON(models.SuccessResponse(response))
}

// DeleteMultipleObjects deletes multiple objects from a bucket
//
//	@Summary		Delete multiple objects from bucket
//	@Description	Deletes multiple objects stored in the specified bucket
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket	path		string																true	"Name of the bucket containing the objects"
//	@Param			request	body		object{keys=[]string,prefixes=[]string}								true	"Object keys to delete and/or folder prefixes to delete recursively"
//	@Success		200		{object}	models.APIResponse{data=models.ObjectDeleteMultipleResponse}	"Successfully deleted the objects"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}						"Invalid request parameters"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}						"Bucket not found"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}						"Failed to delete objects"
//	@Router			/api/v1/buckets/{bucket}/objects/delete-multiple [post]
func (h *ObjectHandler) DeleteMultipleObjects(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameter
	bucketName := c.Params("bucket")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	// Parse request body. "keys" are concrete objects to delete; "prefixes" are
	// folders to delete recursively (every object stored under the prefix).
	var req struct {
		Keys     []string `json:"keys"`
		Prefixes []string `json:"prefixes,omitempty"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body: "+err.Error()),
		)
	}

	if len(req.Keys) == 0 && len(req.Prefixes) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "At least one key or prefix is required"),
		)
	}

	// Validate and normalize folder prefixes before running an irreversible
	// recursive delete on a public endpoint. A blank prefix would match the
	// entire bucket, and a prefix without a trailing slash (e.g. "photos/2024")
	// would also match sibling keys such as "photos/2024-old/...". Reject blanks
	// with a 4XX and force a trailing slash so a prefix only ever deletes the
	// objects inside its own folder.
	prefixes := make([]string, 0, len(req.Prefixes))
	for _, p := range req.Prefixes {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			return c.Status(fiber.StatusBadRequest).JSON(
				models.ErrorResponse(models.ErrCodeBadRequest, "Prefix must not be blank"),
			)
		}
		if !strings.HasSuffix(trimmed, "/") {
			trimmed += "/"
		}
		prefixes = append(prefixes, trimmed)
	}

	deleted := 0

	// Delete the individually selected objects in a single batch call.
	if len(req.Keys) > 0 {
		n, err := h.s3Service.DeleteMultipleObjects(ctx, bucketName, req.Keys)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(
				models.ErrorResponse(models.ErrCodeDeleteFailed, "Failed to delete objects: "+err.Error()),
			)
		}
		deleted += n
	}

	// Recursively delete every object under each selected folder prefix.
	for _, prefix := range prefixes {
		n, err := h.s3Service.DeleteObjectsByPrefix(ctx, bucketName, prefix)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(
				models.ErrorResponse(models.ErrCodeDeleteFailed, "Failed to delete folder "+prefix+": "+err.Error()),
			)
		}
		deleted += n
	}

	response := models.ObjectDeleteMultipleResponse{
		Bucket:   bucketName,
		Deleted:  deleted,
		Keys:     req.Keys,
		Prefixes: prefixes,
	}

	return c.JSON(models.SuccessResponse(response))
}

// UploadMultipleObjects uploads multiple objects to a bucket
//
//	@Summary		Upload multiple objects to bucket
//	@Description	Uploads multiple objects to the specified bucket using multipart/form-data. Accepts unlimited number of files and handles them in a loop.
//	@Tags			Objects
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			bucket	path		string															true	"Name of the bucket to upload the objects to"
//	@Param			files	formData	file															true	"Files to upload (can be multiple)"
//	@Success		201		{object}	models.APIResponse{data=models.ObjectUploadMultipleResponse}	"Objects uploaded successfully (including partial failures)"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}						"Invalid request parameters"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}						"Bucket not found"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}						"Failed to upload objects"
//	@Router			/api/v1/buckets/{bucket}/objects/upload-multiple [post]
func (h *ObjectHandler) UploadMultipleObjects(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameter
	bucketName := c.Params("bucket")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	// Parse multipart form to get all files
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Failed to parse multipart form: "+err.Error()),
		)
	}

	files := form.File["files"]
	if len(files) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "At least one file is required"),
		)
	}

	// Prepare upload data structure
	uploadFiles := make([]struct {
		Key         string
		Body        io.Reader
		ContentType string
	}, len(files))

	// Open all files and prepare for upload
	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(
				models.ErrorResponse(models.ErrCodeUploadFailed, "Failed to open file "+fileHeader.Filename+": "+err.Error()),
			)
		}
		defer file.Close()

		// Use filename as the key
		key := fileHeader.Filename
		contentType := fileHeader.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		uploadFiles[i] = struct {
			Key         string
			Body        io.Reader
			ContentType string
		}{
			Key:         key,
			Body:        file,
			ContentType: contentType,
		}
	}

	// Upload all files using the service method
	results := h.s3Service.UploadMultipleObjects(ctx, bucketName, uploadFiles)

	// Process results and categorize successes and failures
	var successFiles []models.ObjectUploadResult
	var failedFiles []models.ObjectUploadFailedResult
	successCount := 0
	failureCount := 0

	for _, result := range results {
		if result.Success {
			successCount++
			successFiles = append(successFiles, models.ObjectUploadResult{
				Key:         result.Key,
				ETag:        result.ETag,
				Size:        result.Size,
				ContentType: result.ContentType,
			})
		} else {
			failureCount++
			failedFiles = append(failedFiles, models.ObjectUploadFailedResult{
				Key:         result.Key,
				Error:       result.Error.Error(),
				ContentType: result.ContentType,
			})
		}
	}

	response := models.ObjectUploadMultipleResponse{
		Bucket:       bucketName,
		TotalFiles:   len(files),
		SuccessCount: successCount,
		FailureCount: failureCount,
		SuccessFiles: successFiles,
		FailedFiles:  failedFiles,
	}

	// Return 201 if all succeeded, 207 (Multi-Status) if partial success, 500 if all failed
	statusCode := fiber.StatusCreated
	if failureCount > 0 && successCount > 0 {
		statusCode = fiber.StatusMultiStatus // 207
	} else if failureCount > 0 && successCount == 0 {
		statusCode = fiber.StatusInternalServerError
	}

	return c.Status(statusCode).JSON(models.SuccessResponse(response))
}
