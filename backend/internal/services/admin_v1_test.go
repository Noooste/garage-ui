package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
)

func newV1TestServer(t *testing.T, handler http.Handler) *GarageV1AdminService {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewGarageV1AdminService(&config.GarageConfig{
		AdminEndpoint: srv.URL,
		AdminToken:    "test-token",
	}, "")
}

func TestV1_ListKeys(t *testing.T) {
	items := []models.ListKeysResponseItem{{ID: "GK1", Name: "key1"}}
	svc := newV1TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/key" || r.URL.Query().Get("list") == "" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	}))
	result, err := svc.ListKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].ID != "GK1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestV1_GetClusterHealth(t *testing.T) {
	svc := newV1TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"healthy","knownNodes":3,"connectedNodes":3,"storageNodes":3,"storageNodesOk":3,"partitions":256,"partitionsQuorum":256,"partitionsAllOk":256}`))
	}))
	health, err := svc.GetClusterHealth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if health.StorageNodesUp != 3 {
		t.Fatalf("expected StorageNodesUp=3, got %d", health.StorageNodesUp)
	}
}

func TestV1_GetClusterStatistics_Unsupported(t *testing.T) {
	svc := newV1TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make any HTTP request for unsupported operations")
	}))
	_, err := svc.GetClusterStatistics(context.Background())
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}

func TestV1_GetNodeInfo_Unsupported(t *testing.T) {
	svc := newV1TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make any HTTP request for unsupported operations")
	}))
	_, err := svc.GetNodeInfo(context.Background(), "abc")
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}

func TestV1_GetNodeStatistics_Unsupported(t *testing.T) {
	svc := newV1TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make any HTTP request for unsupported operations")
	}))
	_, err := svc.GetNodeStatistics(context.Background(), "abc")
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}
