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

// Garage v2.x serves /v1/health too, so a transient failure of the /v2 probe
// must not cause a permanent downgrade to the (broken on v2.x) v1 client.
// Regression test for https://github.com/Noooste/garage-ui/issues/78
func TestDetectVersion_V2_TransientProbeFailure(t *testing.T) {
	var v2Hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/GetClusterHealth":
			v2Hits++
			if v2Hits < 3 { // fail the first two attempts, then recover
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"healthy"}`))
		case "/v1/health": // v2.x still answers this
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"healthy"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.GarageConfig{AdminEndpoint: srv.URL, AdminToken: "tok"}
	result, err := NewAdminService(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.APIVersion != "v2" {
		t.Fatalf("expected v2 after transient probe failure, got %s", result.APIVersion)
	}
}

// A server that answers /v1/health but returns a server error (not 404) for
// /v2/GetClusterHealth must NOT be detected as v1, because v2.x servers also
// answer /v1/health. Falling through to v1 here is the issue #78 misdetection.
func TestDetectVersion_DoesNotDowngradeOnV2ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/GetClusterHealth":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"healthy"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.GarageConfig{AdminEndpoint: srv.URL, AdminToken: "tok"}
	result, err := NewAdminService(cfg, "")
	if err == nil && result.APIVersion == "v1" {
		t.Fatal("must not downgrade to v1 when /v2 returns a server error; v2.x also serves /v1/health")
	}
}
