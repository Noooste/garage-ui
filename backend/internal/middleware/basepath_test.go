package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// appWithStripper mirrors main.go's wiring: the stripper sits ahead of the
// middleware that must run exactly once per request (request id, access log),
// here stood in for by a counter.
func appWithStripper(basePath string, counter *atomic.Int32) *fiber.App {
	app := fiber.New()
	app.Use(StripBasePath(basePath))
	app.Use(func(c fiber.Ctx) error {
		counter.Add(1)
		return c.Next()
	})
	app.Get("/health", func(c fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/api/v1/buckets/:name", func(c fiber.Ctx) error { return c.SendString(c.Params("name")) })
	return app
}

func get(t *testing.T, app *fiber.App, path string) (int, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatalf("app.Test(%s): %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	return resp.StatusCode, string(buf[:n])
}

// Review #4: RestartRouting replays the whole middleware chain. Installed
// ahead of the per-request middleware, the stripper stops the first pass, so
// everything behind it still runs exactly once.
func TestStripBasePath_DownstreamMiddlewareRunsOnce(t *testing.T) {
	var counter atomic.Int32
	app := appWithStripper("/garage-ui", &counter)

	for _, path := range []string{"/health", "/garage-ui/health", "/garage-ui/"} {
		counter.Store(0)
		if _, _ = get(t, app, path); counter.Load() != 1 {
			t.Errorf("GET %s ran the downstream middleware %d times, want 1", path, counter.Load())
		}
	}
}

func TestStripBasePath_AcceptsBothSpellings(t *testing.T) {
	var counter atomic.Int32
	app := appWithStripper("/garage-ui", &counter)

	for _, path := range []string{"/health", "/garage-ui/health"} {
		if status, _ := get(t, app, path); status != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, status)
		}
	}
}

// Route parameters must come from the stripped path, not the prefixed one.
func TestStripBasePath_KeepsRouteParams(t *testing.T) {
	var counter atomic.Int32
	app := appWithStripper("/garage-ui", &counter)

	status, body := get(t, app, "/garage-ui/api/v1/buckets/my-bucket")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if body != "my-bucket" {
		t.Errorf("param = %q, want %q", body, "my-bucket")
	}
}

func TestStripBasePath_OnlyStripsOnce(t *testing.T) {
	var counter atomic.Int32
	app := appWithStripper("/ui", &counter)

	// One strip leaves "/ui/health", which is not a route. Stripping twice
	// would answer 200 for a request nobody made.
	if status, _ := get(t, app, "/ui/ui/health"); status != http.StatusNotFound {
		t.Errorf("GET /ui/ui/health = %d, want 404 — the prefix was stripped more than once", status)
	}
}

// The prefix has to match on a segment boundary.
func TestStripBasePath_DoesNotStripPartialSegment(t *testing.T) {
	var counter atomic.Int32
	app := appWithStripper("/garage-ui", &counter)

	if status, _ := get(t, app, "/garage-ui-other/health"); status != http.StatusNotFound {
		t.Errorf("GET /garage-ui-other/health = %d, want 404", status)
	}
}

// Without a base path the handler is a pass-through and nothing is rewritten.
func TestStripBasePath_EmptyIsPassThrough(t *testing.T) {
	var counter atomic.Int32
	app := appWithStripper("", &counter)

	if status, _ := get(t, app, "/health"); status != http.StatusOK {
		t.Errorf("GET /health = %d, want 200", status)
	}
	if status, _ := get(t, app, "/garage-ui/health"); status != http.StatusNotFound {
		t.Errorf("GET /garage-ui/health = %d, want 404: no base path is configured", status)
	}
	counter.Store(0)
	_, _ = get(t, app, "/health")
	if counter.Load() != 1 {
		t.Errorf("downstream middleware ran %d times, want 1", counter.Load())
	}
}
