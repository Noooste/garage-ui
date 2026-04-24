package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services"
	"Noooste/garage-ui/internal/services/mocks"

	"github.com/gofiber/fiber/v3"
)

func newClusterTestApp(t *testing.T) (*fiber.App, *mocks.AdminMock) {
	t.Helper()
	admin := &mocks.AdminMock{}
	h := NewClusterHandler(admin)
	app := fiber.New()
	app.Get("/cluster/health", h.GetHealth)
	app.Get("/cluster/status", h.GetStatus)
	app.Get("/cluster/statistics", h.GetStatistics)
	app.Get("/cluster/nodes/:node_id", h.GetNodeInfo)
	app.Get("/cluster/nodes/:node_id/statistics", h.GetNodeStatistics)
	// Extra routes without a node_id param so we can exercise the empty-id gate.
	// Fiber requires a bound param value, so we mount a path with an empty
	// trailing segment explicitly via Locals.
	app.Get("/cluster/nodes-empty", func(c fiber.Ctx) error {
		// Set empty node_id in locals via a route that doesn't capture it.
		return h.GetNodeInfo(c)
	})
	return app, admin
}

func doGet(t *testing.T, app *fiber.App, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test %s: %v", path, err)
	}
	return resp
}

func TestCluster_GetHealth_Success(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterHealthFn = func(_ context.Context) (*models.ClusterHealth, error) {
		return &models.ClusterHealth{Status: "healthy"}, nil
	}
	resp := doGet(t, app, "/cluster/health")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Success bool                 `json:"success"`
		Data    models.ClusterHealth `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if !body.Success || body.Data.Status != "healthy" {
		t.Errorf("body = %+v", body)
	}
}

func TestCluster_GetHealth_ServiceErrorReturns500(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterHealthFn = func(_ context.Context) (*models.ClusterHealth, error) {
		return nil, errors.New("upstream down")
	}
	resp := doGet(t, app, "/cluster/health")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestCluster_GetStatus_Success(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterStatusFn = func(_ context.Context) (*models.ClusterStatus, error) {
		return &models.ClusterStatus{}, nil
	}
	resp := doGet(t, app, "/cluster/status")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestCluster_GetStatus_ServiceErrorReturns500(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterStatusFn = func(_ context.Context) (*models.ClusterStatus, error) {
		return nil, errors.New("boom")
	}
	resp := doGet(t, app, "/cluster/status")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestCluster_GetStatistics_Success(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterStatisticsFn = func(_ context.Context) (*models.ClusterStatistics, error) {
		return &models.ClusterStatistics{}, nil
	}
	resp := doGet(t, app, "/cluster/statistics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestCluster_GetStatistics_ServiceErrorReturns500(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterStatisticsFn = func(_ context.Context) (*models.ClusterStatistics, error) {
		return nil, errors.New("boom")
	}
	resp := doGet(t, app, "/cluster/statistics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestCluster_GetStatistics_UnsupportedReturns501(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterStatisticsFn = func(_ context.Context) (*models.ClusterStatistics, error) {
		return nil, services.ErrUnsupported
	}
	resp := doGet(t, app, "/cluster/statistics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", resp.StatusCode)
	}
}

func TestCluster_GetNodeInfo_UnsupportedReturns501(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetNodeInfoFn = func(_ context.Context, _ string) (*models.MultiNodeResponse, error) {
		return nil, services.ErrUnsupported
	}
	resp := doGet(t, app, "/cluster/nodes/n1")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", resp.StatusCode)
	}
}

func TestCluster_GetNodeStatistics_UnsupportedReturns501(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetNodeStatisticsFn = func(_ context.Context, _ string) (*models.MultiNodeResponse, error) {
		return nil, services.ErrUnsupported
	}
	resp := doGet(t, app, "/cluster/nodes/n1/statistics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", resp.StatusCode)
	}
}

func TestCluster_GetNodeInfo_Success(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetNodeInfoFn = func(_ context.Context, nodeID string) (*models.MultiNodeResponse, error) {
		if nodeID != "node-1" {
			t.Errorf("nodeID = %q, want node-1", nodeID)
		}
		return &models.MultiNodeResponse{}, nil
	}
	resp := doGet(t, app, "/cluster/nodes/node-1")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestCluster_GetNodeInfo_ServiceErrorReturns500(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetNodeInfoFn = func(_ context.Context, _ string) (*models.MultiNodeResponse, error) {
		return nil, errors.New("boom")
	}
	resp := doGet(t, app, "/cluster/nodes/n1")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestCluster_GetNodeStatistics_Success(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetNodeStatisticsFn = func(_ context.Context, nodeID string) (*models.MultiNodeResponse, error) {
		return &models.MultiNodeResponse{}, nil
	}
	resp := doGet(t, app, "/cluster/nodes/n1/statistics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestCluster_GetNodeStatistics_ServiceErrorReturns500(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetNodeStatisticsFn = func(_ context.Context, _ string) (*models.MultiNodeResponse, error) {
		return nil, errors.New("boom")
	}
	resp := doGet(t, app, "/cluster/nodes/n1/statistics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
