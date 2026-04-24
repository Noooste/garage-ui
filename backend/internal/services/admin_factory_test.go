package services

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Noooste/garage-ui/internal/config"
)

func TestErrUnsupportedIsSentinel(t *testing.T) {
	err := ErrUnsupported
	if !errors.Is(err, ErrUnsupported) {
		t.Fatal("ErrUnsupported should match itself via errors.Is")
	}
}

func TestCapabilitiesV2AllTrue(t *testing.T) {
	caps := CapabilitiesV2()
	if !caps.ClusterStatistics || !caps.NodeInfo || !caps.NodeStatistics {
		t.Fatalf("v2 capabilities should all be true, got %+v", caps)
	}
}

func TestCapabilitiesV1AllFalse(t *testing.T) {
	caps := CapabilitiesV1()
	if caps.ClusterStatistics || caps.NodeInfo || caps.NodeStatistics {
		t.Fatalf("v1 capabilities should all be false, got %+v", caps)
	}
}

func TestDetectVersion_V2(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/GetClusterHealth" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"healthy"}`))
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.GarageConfig{AdminEndpoint: srv.URL, AdminToken: "tok"}
	result, err := NewAdminService(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.APIVersion != "v2" {
		t.Fatalf("expected v2, got %s", result.APIVersion)
	}
	if !result.Capabilities.ClusterStatistics {
		t.Fatal("v2 should have ClusterStatistics capability")
	}
}

func TestDetectVersion_V1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/GetClusterHealth" {
			w.WriteHeader(404)
			return
		}
		if r.URL.Path == "/v1/health" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"healthy"}`))
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.GarageConfig{AdminEndpoint: srv.URL, AdminToken: "tok"}
	result, err := NewAdminService(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.APIVersion != "v1" {
		t.Fatalf("expected v1, got %s", result.APIVersion)
	}
	if result.Capabilities.ClusterStatistics {
		t.Fatal("v1 should not have ClusterStatistics capability")
	}
}

func TestDetectVersion_Unreachable(t *testing.T) {
	cfg := &config.GarageConfig{AdminEndpoint: "http://127.0.0.1:1", AdminToken: "tok"}
	_, err := NewAdminService(cfg, "")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
