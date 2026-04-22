package services

import (
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/pkg/utils"
	logpkg "Noooste/garage-ui/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Noooste/azuretls-client"
)

// GarageV2AdminService handles interactions with the Garage Admin API
type GarageV2AdminService struct {
	baseURL    string
	token      string
	httpClient *azuretls.Session
}

// NewGarageV2AdminService creates a new Garage Admin API service
func NewGarageV2AdminService(cfg *config.GarageConfig, logLevel string) *GarageV2AdminService {
	session := azuretls.NewSession()

	if logLevel == "debug" {
		session.Log()
	}

	return &GarageV2AdminService{
		baseURL:    cfg.AdminEndpoint,
		token:      cfg.AdminToken,
		httpClient: session,
	}
}

// doRequest performs an HTTP request to the Admin API with retry logic for connection refused errors
func (s *GarageV2AdminService) doRequest(ctx context.Context, method, path string, body interface{}) (*azuretls.Response, error) {
	var resp *azuretls.Response

	retryConfig := utils.DefaultRetryConfig()
	err := utils.RetryWithBackoff(ctx, retryConfig, func() error {
		var reqErr error
		resp, reqErr = s.httpClient.Do(&azuretls.Request{
			Method:     method,
			Url:        s.baseURL + path,
			Body:       body,
			IgnoreBody: true, // decodeResponse will handle body reading
			OrderedHeaders: azuretls.OrderedHeaders{
				{"Authorization", fmt.Sprintf("Bearer %s", s.token)},
			},
		}, ctx)
		return reqErr
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// decodeResponse decodes a JSON response into the target structure
func decodeResponse(resp *azuretls.Response, target interface{}) error {
	defer resp.RawBody.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.RawBody)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if target != nil {
		if err := json.NewDecoder(resp.RawBody).Decode(target); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// ListKeys returns all access keys in the cluster
func (s *GarageV2AdminService) ListKeys(ctx context.Context) ([]models.ListKeysResponseItem, error) {
	resp, err := s.doRequest(ctx, http.MethodGet, "/v2/ListKeys", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result []models.ListKeysResponseItem
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// CreateKey creates a new API access key
func (s *GarageV2AdminService) CreateKey(ctx context.Context, req models.CreateKeyRequest) (*models.GarageKeyInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPost, "/v2/CreateKey", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageKeyInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetKeyInfo returns information about a specific access key
func (s *GarageV2AdminService) GetKeyInfo(ctx context.Context, keyID string, showSecret bool) (*models.GarageKeyInfo, error) {
	path := fmt.Sprintf("/v2/GetKeyInfo?id=%s", keyID)
	if showSecret {
		path += "&showSecretKey=true"
	}

	resp, err := s.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageKeyInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateKey updates information about an access key
func (s *GarageV2AdminService) UpdateKey(ctx context.Context, keyID string, req models.UpdateKeyRequest) (*models.GarageKeyInfo, error) {
	path := fmt.Sprintf("/v2/UpdateKey?id=%s", keyID)

	resp, err := s.doRequest(ctx, http.MethodPost, path, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageKeyInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteKey deletes an access key from the cluster
func (s *GarageV2AdminService) DeleteKey(ctx context.Context, keyID string) error {
	path := fmt.Sprintf("/v2/DeleteKey?id=%s", keyID)

	resp, err := s.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if err := decodeResponse(resp, nil); err != nil {
		return fmt.Errorf("failed to process response: %w", err)
	}

	return nil
}

// ImportKey imports an existing API access key
func (s *GarageV2AdminService) ImportKey(ctx context.Context, req models.ImportKeyRequest) (*models.GarageKeyInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPost, "/v2/ImportKey", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageKeyInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListBuckets returns all buckets in the cluster.
func (s *GarageV2AdminService) ListBuckets(ctx context.Context) ([]models.ListBucketsResponseItem, error) {
	log := logpkg.FromCtx(ctx).With().
		Str("component", "admin").
		Str("operation", "list_buckets").
		Logger()

	log.Debug().Msg("listing buckets")
	start := time.Now()

	resp, err := s.doRequest(ctx, http.MethodGet, "/v2/ListBuckets", nil)
	if err != nil {
		log.Error().Err(err).
			Float64("duration_ms", msSince(start)).
			Str("outcome", "failure").
			Msg("garage list_buckets request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result []models.ListBucketsResponseItem
	if err := decodeResponse(resp, &result); err != nil {
		log.Error().Err(err).
			Float64("duration_ms", msSince(start)).
			Str("outcome", "failure").
			Msg("garage list_buckets decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Debug().
		Float64("duration_ms", msSince(start)).
		Str("outcome", "success").
		Int("count", len(result)).
		Msg("listed buckets")
	return result, nil
}

// GetBucketInfo returns detailed information about a bucket by ID.
func (s *GarageV2AdminService) GetBucketInfo(ctx context.Context, bucketID string) (*models.GarageBucketInfo, error) {
	log := logpkg.FromCtx(ctx).With().
		Str("component", "admin").
		Str("operation", "get_bucket_info").
		Str("bucket_id", bucketID).
		Logger()

	log.Debug().Msg("getting bucket info")
	start := time.Now()

	resp, err := s.doRequest(ctx, http.MethodGet, fmt.Sprintf("/v2/GetBucketInfo?id=%s", bucketID), nil)
	if err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage get_bucket_info request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage get_bucket_info decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Debug().Float64("duration_ms", msSince(start)).Str("outcome", "success").Msg("got bucket info")
	return &result, nil
}

// GetBucketInfoByAlias returns detailed information about a bucket by its global alias.
func (s *GarageV2AdminService) GetBucketInfoByAlias(ctx context.Context, globalAlias string) (*models.GarageBucketInfo, error) {
	log := logpkg.FromCtx(ctx).With().
		Str("component", "admin").
		Str("operation", "get_bucket_info_by_alias").
		Str("bucket", globalAlias).
		Logger()

	log.Debug().Msg("getting bucket info by alias")
	start := time.Now()

	resp, err := s.doRequest(ctx, http.MethodGet, fmt.Sprintf("/v2/GetBucketInfo?globalAlias=%s", globalAlias), nil)
	if err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage get_bucket_info_by_alias request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err = decodeResponse(resp, &result); err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage get_bucket_info_by_alias decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Debug().Float64("duration_ms", msSince(start)).Str("outcome", "success").Str("bucket_id", result.ID).Msg("got bucket info by alias")
	return &result, nil
}

// CreateBucket creates a new bucket via the Admin API.
func (s *GarageV2AdminService) CreateBucket(ctx context.Context, req models.CreateBucketAdminRequest) (*models.GarageBucketInfo, error) {
	var alias string
	if req.GlobalAlias != nil {
		alias = *req.GlobalAlias
	}
	log := logpkg.FromCtx(ctx).With().
		Str("component", "admin").
		Str("operation", "create_bucket").
		Str("bucket", alias).
		Logger()

	log.Info().Msg("creating bucket")
	start := time.Now()

	resp, err := s.doRequest(ctx, http.MethodPost, "/v2/CreateBucket", req)
	if err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage create_bucket request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage create_bucket decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Info().Float64("duration_ms", msSince(start)).Str("outcome", "success").Str("bucket_id", result.ID).Msg("bucket created")
	return &result, nil
}

// UpdateBucket updates bucket settings.
func (s *GarageV2AdminService) UpdateBucket(ctx context.Context, bucketID string, req models.UpdateBucketRequest) (*models.GarageBucketInfo, error) {
	log := logpkg.FromCtx(ctx).With().
		Str("component", "admin").
		Str("operation", "update_bucket").
		Str("bucket_id", bucketID).
		Logger()

	log.Info().Msg("updating bucket")
	start := time.Now()

	resp, err := s.doRequest(ctx, http.MethodPost, fmt.Sprintf("/v2/UpdateBucket?id=%s", bucketID), req)
	if err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage update_bucket request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage update_bucket decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Info().Float64("duration_ms", msSince(start)).Str("outcome", "success").Msg("bucket updated")
	return &result, nil
}

// DeleteBucket deletes a bucket.
func (s *GarageV2AdminService) DeleteBucket(ctx context.Context, bucketID string) error {
	log := logpkg.FromCtx(ctx).With().
		Str("component", "admin").
		Str("operation", "delete_bucket").
		Str("bucket_id", bucketID).
		Logger()

	log.Info().Msg("deleting bucket")
	start := time.Now()

	resp, err := s.doRequest(ctx, http.MethodPost, fmt.Sprintf("/v2/DeleteBucket?id=%s", bucketID), nil)
	if err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage delete_bucket request failed")
		return fmt.Errorf("request failed: %w", err)
	}

	if err := decodeResponse(resp, nil); err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage delete_bucket decode failed")
		return fmt.Errorf("failed to process response: %w", err)
	}

	log.Info().Float64("duration_ms", msSince(start)).Str("outcome", "success").Msg("bucket deleted")
	return nil
}

// AddBucketAlias adds an alias to a bucket
func (s *GarageV2AdminService) AddBucketAlias(ctx context.Context, req models.AddBucketAliasRequest) (*models.GarageBucketInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPost, "/v2/AddBucketAlias", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// RemoveBucketAlias removes an alias from a bucket
func (s *GarageV2AdminService) RemoveBucketAlias(ctx context.Context, req models.RemoveBucketAliasRequest) (*models.GarageBucketInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPost, "/v2/RemoveBucketAlias", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// AllowBucketKey grants permissions for a key on a bucket.
func (s *GarageV2AdminService) AllowBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error) {
	log := logpkg.FromCtx(ctx).With().
		Str("component", "admin").
		Str("operation", "allow_bucket_key").
		Str("bucket_id", req.BucketID).
		Str("access_key_id", logpkg.RedactKey(req.AccessKeyID)).
		Bool("perm_read", req.Permissions.Read).
		Bool("perm_write", req.Permissions.Write).
		Bool("perm_owner", req.Permissions.Owner).
		Logger()

	log.Info().Msg("granting bucket key permissions")
	start := time.Now()

	resp, err := s.doRequest(ctx, http.MethodPost, "/v2/AllowBucketKey", req)
	if err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage allow_bucket_key request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage allow_bucket_key decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Info().Float64("duration_ms", msSince(start)).Str("outcome", "success").Msg("bucket key permissions granted")
	return &result, nil
}

// DenyBucketKey revokes permissions for a key on a bucket
func (s *GarageV2AdminService) DenyBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPost, "/v2/DenyBucketKey", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetClusterHealth returns the health status of the cluster
func (s *GarageV2AdminService) GetClusterHealth(ctx context.Context) (*models.ClusterHealth, error) {
	resp, err := s.doRequest(ctx, http.MethodGet, "/v2/GetClusterHealth", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.ClusterHealth
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetClusterStatus returns the current status of the cluster
func (s *GarageV2AdminService) GetClusterStatus(ctx context.Context) (*models.ClusterStatus, error) {
	resp, err := s.doRequest(ctx, http.MethodGet, "/v2/GetClusterStatus", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.ClusterStatus
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetClusterStatistics returns global cluster statistics
func (s *GarageV2AdminService) GetClusterStatistics(ctx context.Context) (*models.ClusterStatistics, error) {
	resp, err := s.doRequest(ctx, http.MethodGet, "/v2/GetClusterStatistics", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.ClusterStatistics
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetNodeInfo returns information about a specific node
func (s *GarageV2AdminService) GetNodeInfo(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error) {
	path := fmt.Sprintf("/v2/GetNodeInfo?node=%s", nodeID)

	resp, err := s.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.MultiNodeResponse
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetNodeStatistics returns statistics for a specific node
func (s *GarageV2AdminService) GetNodeStatistics(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error) {
	path := fmt.Sprintf("/v2/GetNodeStatistics?node=%s", nodeID)

	resp, err := s.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.MultiNodeResponse
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// HealthCheck checks if the Admin API is reachable
func (s *GarageV2AdminService) HealthCheck(ctx context.Context) error {
	resp, err := s.doRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if err := decodeResponse(resp, nil); err != nil {
		return fmt.Errorf("health check returned error: %w", err)
	}

	return nil
}

// msSince returns duration since t in milliseconds as a float64.
func msSince(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000.0
}

// GetMetrics returns Prometheus metrics from the Admin API
func (s *GarageV2AdminService) GetMetrics(ctx context.Context) (string, error) {
	resp, err := s.doRequest(ctx, http.MethodGet, "/metrics", nil)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.RawBody.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.RawBody)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.RawBody)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(bodyBytes), nil
}
