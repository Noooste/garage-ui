package routes

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Noooste/garage-ui/internal/authz"
	"Noooste/garage-ui/internal/config"
)

// Flag off (default): /metrics is not registered, and the authenticated
// /api/v1/monitoring/metrics route still rejects unauthenticated requests.
func TestRoutes_MetricsPublic_Disabled_NotRegistered(t *testing.T) {
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
		// MetricsPublic defaults to false.
	})

	expectStatus(t, f.App, httptest.NewRequest("GET", "/metrics", nil), 404)
	expectStatus(t, f.App, httptest.NewRequest("GET", "/api/v1/monitoring/metrics", nil), 401)
}

// Flag on, with admin auth enabled: /metrics serves without credentials, while
// the authenticated /api/v1/monitoring/metrics route still requires auth.
func TestRoutes_MetricsPublic_Enabled_ServesWithoutAuth(t *testing.T) {
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
		c.Auth.MetricsPublic = true
	})
	f.Admin.GetMetricsFn = func(_ context.Context) (string, error) {
		return "garage_metric 1", nil
	}

	resp := expectStatus(t, f.App, httptest.NewRequest("GET", "/metrics", nil), 200)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "garage_metric") {
		t.Errorf("GET /metrics body = %q, want it to contain the metrics text", string(body))
	}

	// The /api/v1 route stays gated; the fail-closed guarantee is intact.
	expectStatus(t, f.App, httptest.NewRequest("GET", "/api/v1/monitoring/metrics", nil), 401)
}

// Route coverage must still pass with the flag on: /metrics is outside /api/v1,
// so VerifyRouteCoverage neither requires a Require handler for it nor errors.
func TestRoutes_MetricsPublic_Enabled_RouteCoverageStillPasses(t *testing.T) {
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
		c.Auth.MetricsPublic = true
	})
	if err := authz.VerifyRouteCoverage(f.App); err != nil {
		t.Errorf("VerifyRouteCoverage returned error with metrics_public on: %v", err)
	}
}

// With the metrics flag OFF and the SPA frontend present, GET /metrics must
// return 404 (not the SPA index.html), so a misconfigured Prometheus scrape
// fails loudly instead of silently receiving HTML with a 200 status.
func TestRoutes_MetricsPublic_Disabled_WithSPA_Returns404(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Create ./frontend/dist/index.html so the SPA fallback mounts.
	if err := os.MkdirAll(filepath.Join(dir, "frontend", "dist"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend", "dist", "index.html"),
		[]byte("<!doctype html><title>spa</title>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
		// MetricsPublic defaults to false → no /metrics route registered.
	})

	// SPA fallback is mounted; /metrics must be excluded from it → 404, not
	// index.html with 200.
	expectStatus(t, f.App, httptest.NewRequest("GET", "/metrics", nil), 404)
}
