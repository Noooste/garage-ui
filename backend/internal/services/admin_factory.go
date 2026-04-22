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

func NewAdminService(cfg *config.GarageConfig, logLevel string) (*AdminServiceResult, error) {
	if err := probeEndpoint(cfg, "/v2/GetClusterHealth"); err == nil {
		logger.Info().Str("api_version", "v2").Msg("Detected Garage admin API v2")
		svc := NewGarageV2AdminService(cfg, logLevel)
		return &AdminServiceResult{
			Service:      svc,
			Capabilities: CapabilitiesV2(),
			APIVersion:   "v2",
		}, nil
	}

	if err := probeEndpoint(cfg, "/v1/health"); err == nil {
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("probe %s returned status %d", path, resp.StatusCode)
	}
	return nil
}
