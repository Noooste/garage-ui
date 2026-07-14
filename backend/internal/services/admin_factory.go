package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/pkg/logger"

	"github.com/Noooste/azuretls-client"
)

var ErrUnsupported = errors.New("operation not supported by this Garage version")

type Capabilities struct {
	ClusterStatistics bool `json:"clusterStatistics"`
	NodeInfo          bool `json:"nodeInfo"`
	NodeStatistics    bool `json:"nodeStatistics"`
}

func CapabilitiesV2() Capabilities {
	return Capabilities{
		ClusterStatistics: true,
		NodeInfo:          true,
		NodeStatistics:    true,
	}
}

func CapabilitiesV1() Capabilities {
	return Capabilities{}
}

type AdminServiceResult struct {
	Service      AdminService
	Capabilities Capabilities
	APIVersion   string
}

// errProbeNotFound means the probed route returned 404. Garage v2.x also serves
// /v1/health, so a 404 on /v2 is the only reliable "this is a v1 server" signal.
var errProbeNotFound = errors.New("probe endpoint not found")

func NewAdminService(cfg *config.GarageConfig, logLevel string) (*AdminServiceResult, error) {
	// retry so a startup fails doesn't lock us to a v1 client.
	err := probeEndpointWithRetry(cfg, "/v2/GetClusterHealth")
	if err == nil {
		logger.Info().Str("api_version", "v2").Msg("Detected Garage admin API v2")
		svc := NewGarageV2AdminService(cfg, logLevel)
		return &AdminServiceResult{
			Service:      svc,
			Capabilities: CapabilitiesV2(),
			APIVersion:   "v2",
		}, nil
	}

	// only fall back to v1 on a real 404
	// other errors mean the server is up but the probe failed transiently; picking v1 against
	// a v2.x server breaks /v1/status with "v1/ endpoint is no longer supported" (issue #78).
	if !errors.Is(err, errProbeNotFound) {
		return nil, fmt.Errorf(
			"could not detect Garage admin API version at %s: %w. Ensure Garage v1.1+ is running and the admin API is reachable",
			cfg.AdminEndpoint, err,
		)
	}

	if err := probeEndpointWithRetry(cfg, "/v1/health"); err == nil {
		logger.Info().
			Str("api_version", "v1").
			Msg("Detected Garage admin API v1 — cluster statistics and per-node details will be unavailable")
		svc := NewGarageV1AdminService(cfg, logLevel)
		return &AdminServiceResult{
			Service:      svc,
			Capabilities: CapabilitiesV1(),
			APIVersion:   "v1",
		}, nil
	}

	return nil, fmt.Errorf(
		"could not connect to Garage admin API at %s. Ensure Garage v1.1+ is running and the admin API is enabled",
		cfg.AdminEndpoint,
	)
}

const probeAttempts = 4

// probeEndpointWithRetry retries transient failures with backoff, but returns a
// 404 immediately, a missing route won't appear on a retry.
func probeEndpointWithRetry(cfg *config.GarageConfig, path string) error {
	var err error
	backoff := 250 * time.Millisecond
	for attempt := range probeAttempts {
		err = probeEndpoint(cfg, path)
		if err == nil || errors.Is(err, errProbeNotFound) {
			return err
		}
		if attempt < probeAttempts-1 {
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	return err
}

func probeEndpoint(cfg *config.GarageConfig, path string) error {
	session := azuretls.NewSession()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := session.Do(&azuretls.Request{
		Method:     http.MethodGet,
		Url:        cfg.AdminEndpoint + path,
		IgnoreBody: true,
		OrderedHeaders: azuretls.OrderedHeaders{
			{"Authorization", fmt.Sprintf("Bearer %s", cfg.AdminToken)},
		},
	}, ctx)
	if err != nil {
		return err
	}
	defer resp.RawBody.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("probe %s returned status 404: %w", path, errProbeNotFound)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("probe %s returned status %d", path, resp.StatusCode)
	}
	return nil
}
