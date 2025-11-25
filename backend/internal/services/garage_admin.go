package services

import (
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Noooste/azuretls-client"
)

// GarageAdminService handles interactions with the Garage Admin API
type GarageAdminService struct {
	baseURL    string
	token      string
	httpClient *azuretls.Session
}

// NewGarageAdminService creates a new Garage Admin API service
func NewGarageAdminService(cfg *config.GarageConfig) *GarageAdminService {
	session := azuretls.NewSession()
	session.Log()

	return &GarageAdminService{
		baseURL:    cfg.AdminEndpoint,
		token:      cfg.AdminToken,
		httpClient: session,
	}
}

// doRequest performs an HTTP request to the Admin API
func (s *GarageAdminService) doRequest(ctx context.Context, method, path string, body interface{}) (*azuretls.Response, error) {
	return s.httpClient.Do(&azuretls.Request{
		Method:     method,
		Url:        s.baseURL + path,
		Body:       body,
		IgnoreBody: true, // decodeResponse will handle body reading
		OrderedHeaders: azuretls.OrderedHeaders{
			{"Authorization", fmt.Sprintf("Bearer %s", s.token)},
		},
	}, ctx)
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

// ====================================
// Access Key Operations
// ====================================

// ListKeys returns all access keys in the cluster
func (s *GarageAdminService) ListKeys(ctx context.Context) ([]models.ListKeysResponseItem, error) {
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
func (s *GarageAdminService) CreateKey(ctx context.Context, req models.CreateKeyRequest) (*models.GarageKeyInfo, error) {
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
func (s *GarageAdminService) GetKeyInfo(ctx context.Context, keyID string, showSecret bool) (*models.GarageKeyInfo, error) {
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
func (s *GarageAdminService) UpdateKey(ctx context.Context, keyID string, req models.UpdateKeyRequest) (*models.GarageKeyInfo, error) {
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
func (s *GarageAdminService) DeleteKey(ctx context.Context, keyID string) error {
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
func (s *GarageAdminService) ImportKey(ctx context.Context, req models.ImportKeyRequest) (*models.GarageKeyInfo, error) {
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

// ====================================
// Bucket Operations (Admin API)
// ====================================

// ListBuckets returns all buckets in the cluster
func (s *GarageAdminService) ListBuckets(ctx context.Context) ([]models.ListBucketsResponseItem, error) {
	resp, err := s.doRequest(ctx, http.MethodGet, "/v2/ListBuckets", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result []models.ListBucketsResponseItem
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// GetBucketInfo returns detailed information about a bucket by ID
func (s *GarageAdminService) GetBucketInfo(ctx context.Context, bucketID string) (*models.GarageBucketInfo, error) {
	path := fmt.Sprintf("/v2/GetBucketInfo?id=%s", bucketID)

	resp, err := s.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetBucketInfoByAlias returns detailed information about a bucket by its global alias
func (s *GarageAdminService) GetBucketInfoByAlias(ctx context.Context, globalAlias string) (*models.GarageBucketInfo, error) {
	path := fmt.Sprintf("/v2/GetBucketInfo?globalAlias=%s", globalAlias)

	resp, err := s.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CreateBucket creates a new bucket via the Admin API
func (s *GarageAdminService) CreateBucket(ctx context.Context, req models.CreateBucketAdminRequest) (*models.GarageBucketInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPost, "/v2/CreateBucket", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateBucket updates bucket settings
func (s *GarageAdminService) UpdateBucket(ctx context.Context, bucketID string, req models.UpdateBucketRequest) (*models.GarageBucketInfo, error) {
	path := fmt.Sprintf("/v2/UpdateBucket?id=%s", bucketID)

	resp, err := s.doRequest(ctx, http.MethodPost, path, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteBucket deletes a bucket
func (s *GarageAdminService) DeleteBucket(ctx context.Context, bucketID string) error {
	path := fmt.Sprintf("/v2/DeleteBucket?id=%s", bucketID)

	resp, err := s.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if err := decodeResponse(resp, nil); err != nil {
		return fmt.Errorf("failed to process response: %w", err)
	}

	return nil
}

// ====================================
// Bucket Alias Operations
// ====================================

// AddBucketAlias adds an alias to a bucket
func (s *GarageAdminService) AddBucketAlias(ctx context.Context, req models.AddBucketAliasRequest) (*models.GarageBucketInfo, error) {
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
func (s *GarageAdminService) RemoveBucketAlias(ctx context.Context, req models.RemoveBucketAliasRequest) (*models.GarageBucketInfo, error) {
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

// ====================================
// Permission Operations
// ====================================

// AllowBucketKey grants permissions for a key on a bucket
func (s *GarageAdminService) AllowBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPost, "/v2/AllowBucketKey", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DenyBucketKey revokes permissions for a key on a bucket
func (s *GarageAdminService) DenyBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error) {
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

// ====================================
// Cluster Operations
// ====================================

// GetClusterHealth returns the health status of the cluster
func (s *GarageAdminService) GetClusterHealth(ctx context.Context) (*models.ClusterHealth, error) {
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
func (s *GarageAdminService) GetClusterStatus(ctx context.Context) (*models.ClusterStatus, error) {
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
func (s *GarageAdminService) GetClusterStatistics(ctx context.Context) (*models.ClusterStatistics, error) {
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

// ====================================
// Node Operations
// ====================================

// GetNodeInfo returns information about a specific node
func (s *GarageAdminService) GetNodeInfo(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error) {
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
func (s *GarageAdminService) GetNodeStatistics(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error) {
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

// ====================================
// Monitoring Operations
// ====================================

// HealthCheck checks if the Admin API is reachable
func (s *GarageAdminService) HealthCheck(ctx context.Context) error {
	resp, err := s.doRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if err := decodeResponse(resp, nil); err != nil {
		return fmt.Errorf("health check returned error: %w", err)
	}

	return nil
}

// GetMetrics returns Prometheus metrics from the Admin API
func (s *GarageAdminService) GetMetrics(ctx context.Context) (string, error) {
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
