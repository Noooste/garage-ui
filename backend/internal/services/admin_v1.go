package services

import (
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/pkg/utils"
	logpkg "Noooste/garage-ui/pkg/logger"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Noooste/azuretls-client"
)

type GarageV1AdminService struct {
	baseURL    string
	token      string
	httpClient *azuretls.Session
}

func NewGarageV1AdminService(cfg *config.GarageConfig, logLevel string) *GarageV1AdminService {
	session := azuretls.NewSession()
	if logLevel == "debug" {
		session.Log()
	}
	return &GarageV1AdminService{
		baseURL:    cfg.AdminEndpoint,
		token:      cfg.AdminToken,
		httpClient: session,
	}
}

func (s *GarageV1AdminService) doRequest(ctx context.Context, method, path string, body interface{}) (*azuretls.Response, error) {
	var resp *azuretls.Response
	retryConfig := utils.DefaultRetryConfig()
	err := utils.RetryWithBackoff(ctx, retryConfig, func() error {
		var reqErr error
		resp, reqErr = s.httpClient.Do(&azuretls.Request{
			Method:     method,
			Url:        s.baseURL + path,
			Body:       body,
			IgnoreBody: true,
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

func (s *GarageV1AdminService) ListKeys(ctx context.Context) ([]models.ListKeysResponseItem, error) {
	resp, err := s.doRequest(ctx, http.MethodGet, "/v1/key?list=true", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result []models.ListKeysResponseItem
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

func (s *GarageV1AdminService) CreateKey(ctx context.Context, req models.CreateKeyRequest) (*models.GarageKeyInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPost, "/v1/key?list", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result models.GarageKeyInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (s *GarageV1AdminService) GetKeyInfo(ctx context.Context, keyID string, showSecret bool) (*models.GarageKeyInfo, error) {
	path := fmt.Sprintf("/v1/key?id=%s", keyID)
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

func (s *GarageV1AdminService) UpdateKey(ctx context.Context, keyID string, req models.UpdateKeyRequest) (*models.GarageKeyInfo, error) {
	path := fmt.Sprintf("/v1/key?id=%s", keyID)
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

func (s *GarageV1AdminService) DeleteKey(ctx context.Context, keyID string) error {
	path := fmt.Sprintf("/v1/key?id=%s", keyID)
	resp, err := s.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	if err := decodeResponse(resp, nil); err != nil {
		return fmt.Errorf("failed to process response: %w", err)
	}
	return nil
}

func (s *GarageV1AdminService) ImportKey(ctx context.Context, req models.ImportKeyRequest) (*models.GarageKeyInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPost, "/v1/key/import", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result models.GarageKeyInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (s *GarageV1AdminService) ListBuckets(ctx context.Context) ([]models.ListBucketsResponseItem, error) {
	log := logpkg.FromCtx(ctx).With().Str("component", "admin-v1").Str("operation", "list_buckets").Logger()
	log.Debug().Msg("listing buckets")
	start := time.Now()
	resp, err := s.doRequest(ctx, http.MethodGet, "/v1/bucket?list", nil)
	if err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Msg("list_buckets request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result []models.ListBucketsResponseItem
	if err := decodeResponse(resp, &result); err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Msg("list_buckets decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	log.Debug().Float64("duration_ms", msSince(start)).Int("count", len(result)).Msg("listed buckets")
	return result, nil
}

func (s *GarageV1AdminService) GetBucketInfo(ctx context.Context, bucketID string) (*models.GarageBucketInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/bucket?id=%s", bucketID), nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (s *GarageV1AdminService) GetBucketInfoByAlias(ctx context.Context, globalAlias string) (*models.GarageBucketInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/bucket?globalAlias=%s", globalAlias), nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (s *GarageV1AdminService) CreateBucket(ctx context.Context, req models.CreateBucketAdminRequest) (*models.GarageBucketInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPost, "/v1/bucket", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (s *GarageV1AdminService) UpdateBucket(ctx context.Context, bucketID string, req models.UpdateBucketRequest) (*models.GarageBucketInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPut, fmt.Sprintf("/v1/bucket?id=%s", bucketID), req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (s *GarageV1AdminService) DeleteBucket(ctx context.Context, bucketID string) error {
	resp, err := s.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/v1/bucket?id=%s", bucketID), nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	if err := decodeResponse(resp, nil); err != nil {
		return fmt.Errorf("failed to process response: %w", err)
	}
	return nil
}

func (s *GarageV1AdminService) AllowBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPost, "/v1/bucket/allow", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (s *GarageV1AdminService) DenyBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error) {
	resp, err := s.doRequest(ctx, http.MethodPost, "/v1/bucket/deny", req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (s *GarageV1AdminService) AddBucketAlias(ctx context.Context, req models.AddBucketAliasRequest) (*models.GarageBucketInfo, error) {
	var path string
	if req.GlobalAlias != nil {
		path = fmt.Sprintf("/v1/bucket/alias/global?id=%s&alias=%s", req.BucketID, *req.GlobalAlias)
	} else if req.LocalAlias != nil && req.AccessKeyID != nil {
		path = fmt.Sprintf("/v1/bucket/alias/local?id=%s&accessKeyId=%s&alias=%s", req.BucketID, *req.AccessKeyID, *req.LocalAlias)
	} else {
		return nil, fmt.Errorf("AddBucketAlias requires either globalAlias or localAlias+accessKeyId")
	}
	resp, err := s.doRequest(ctx, http.MethodPut, path, nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (s *GarageV1AdminService) RemoveBucketAlias(ctx context.Context, req models.RemoveBucketAliasRequest) (*models.GarageBucketInfo, error) {
	var path string
	if req.GlobalAlias != nil {
		path = fmt.Sprintf("/v1/bucket/alias/global?id=%s&alias=%s", req.BucketID, *req.GlobalAlias)
	} else if req.LocalAlias != nil && req.AccessKeyID != nil {
		path = fmt.Sprintf("/v1/bucket/alias/local?id=%s&accessKeyId=%s&alias=%s", req.BucketID, *req.AccessKeyID, *req.LocalAlias)
	} else {
		return nil, fmt.Errorf("RemoveBucketAlias requires either globalAlias or localAlias+accessKeyId")
	}
	resp, err := s.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (s *GarageV1AdminService) GetClusterHealth(ctx context.Context) (*models.ClusterHealth, error) {
	resp, err := s.doRequest(ctx, http.MethodGet, "/v1/health", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result models.ClusterHealth
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

type v1StatusResponse struct {
	Node          string        `json:"node"`
	GarageVersion string        `json:"garageVersion"`
	KnownNodes    []v1KnownNode `json:"knownNodes"`
	Layout        *v1Layout     `json:"layout"`
}

type v1KnownNode struct {
	ID              string `json:"id"`
	Addr            string `json:"addr"`
	IsUp            bool   `json:"isUp"`
	LastSeenSecsAgo *int64 `json:"lastSeenSecsAgo"`
	Hostname        string `json:"hostname"`
}

type v1Layout struct {
	Version int `json:"version"`
}

func (s *GarageV1AdminService) GetClusterStatus(ctx context.Context) (*models.ClusterStatus, error) {
	resp, err := s.doRequest(ctx, http.MethodGet, "/v1/status", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var raw v1StatusResponse
	if err := decodeResponse(resp, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	nodes := make([]models.NodeInfo, len(raw.KnownNodes))
	for i, n := range raw.KnownNodes {
		addr := n.Addr
		hostname := n.Hostname
		nodes[i] = models.NodeInfo{
			ID:              n.ID,
			IsUp:            n.IsUp,
			LastSeenSecsAgo: n.LastSeenSecsAgo,
			Hostname:        &hostname,
			Addr:            &addr,
		}
	}
	layoutVersion := 0
	if raw.Layout != nil {
		layoutVersion = raw.Layout.Version
	}
	return &models.ClusterStatus{
		LayoutVersion: layoutVersion,
		Nodes:         nodes,
	}, nil
}

func (s *GarageV1AdminService) GetClusterStatistics(ctx context.Context) (*models.ClusterStatistics, error) {
	return nil, ErrUnsupported
}

func (s *GarageV1AdminService) GetNodeInfo(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error) {
	return nil, ErrUnsupported
}

func (s *GarageV1AdminService) GetNodeStatistics(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error) {
	return nil, ErrUnsupported
}

func (s *GarageV1AdminService) HealthCheck(ctx context.Context) error {
	resp, err := s.doRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	if err := decodeResponse(resp, nil); err != nil {
		return fmt.Errorf("health check returned error: %w", err)
	}
	return nil
}

func (s *GarageV1AdminService) GetMetrics(ctx context.Context) (string, error) {
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
