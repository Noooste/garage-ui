package services

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
	logpkg "Noooste/garage-ui/pkg/logger"

	"github.com/rs/zerolog"
)

// newAdminWithServer creates a GarageV2AdminService pointed at a test server
// and returns it with a cleanup hook.
func newAdminWithServer(t *testing.T, handler http.HandlerFunc) *GarageV2AdminService {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg := &config.GarageConfig{
		AdminEndpoint: srv.URL,
		AdminToken:    "test-token",
	}
	return NewGarageV2AdminService(cfg, "info")
}

// ctxWithBufferLogger attaches a zerolog.Logger writing to buf onto ctx.
func ctxWithBufferLogger(buf *bytes.Buffer) context.Context {
	l := zerolog.New(buf)
	return logpkg.IntoCtx(context.Background(), l)
}

func parseJSONLines(t *testing.T, s string) []map[string]any {
	t.Helper()
	out := []map[string]any{}
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("not JSON: %v — %s", err, line)
		}
		out = append(out, m)
	}
	return out
}

func TestCreateBucket_SuccessLogsInfoWithFields(t *testing.T) {
	admin := newAdminWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models.GarageBucketInfo{ID: "bkt-123"})
	})

	var buf bytes.Buffer
	ctx := ctxWithBufferLogger(&buf)

	alias := "my-bucket"
	if _, err := admin.CreateBucket(ctx, models.CreateBucketAdminRequest{GlobalAlias: &alias}); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	lines := parseJSONLines(t, buf.String())
	if len(lines) < 2 {
		t.Fatalf("expected >=2 lines (start + success), got %d: %s", len(lines), buf.String())
	}

	final := lines[len(lines)-1]
	if final["component"] != "admin" {
		t.Errorf("component = %v, want admin", final["component"])
	}
	if final["operation"] != "create_bucket" {
		t.Errorf("operation = %v, want create_bucket", final["operation"])
	}
	if final["outcome"] != "success" {
		t.Errorf("outcome = %v, want success", final["outcome"])
	}
	if final["level"] != "info" {
		t.Errorf("level = %v, want info", final["level"])
	}
	if _, ok := final["duration_ms"].(float64); !ok {
		t.Errorf("missing duration_ms: %v", final)
	}
	if final["bucket_id"] != "bkt-123" {
		t.Errorf("bucket_id = %v, want bkt-123", final["bucket_id"])
	}
}

func TestCreateBucket_FailureLogsError(t *testing.T) {
	admin := newAdminWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"already exists"}`))
	})

	var buf bytes.Buffer
	ctx := ctxWithBufferLogger(&buf)

	alias := "duplicate"
	_, err := admin.CreateBucket(ctx, models.CreateBucketAdminRequest{GlobalAlias: &alias})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	lines := parseJSONLines(t, buf.String())
	var failure map[string]any
	for _, line := range lines {
		if line["level"] == "error" {
			failure = line
			break
		}
	}
	if failure == nil {
		t.Fatalf("no error-level log line: %s", buf.String())
	}

	if failure["outcome"] != "failure" {
		t.Errorf("outcome = %v, want failure", failure["outcome"])
	}
	if failure["operation"] != "create_bucket" {
		t.Errorf("operation = %v, want create_bucket", failure["operation"])
	}
	if _, ok := failure["error"].(string); !ok {
		t.Errorf("missing error field: %v", failure)
	}
	if _, ok := failure["duration_ms"].(float64); !ok {
		t.Errorf("missing duration_ms: %v", failure)
	}
}
