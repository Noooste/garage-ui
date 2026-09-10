//go:build smoke

package smoke_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	composeProject = "garage-ui-smoke"
	backendBaseURL = "http://127.0.0.1:18080"
	// Second instance of the same image, served from a subpath (issue #107).
	subpathBaseURL = "http://127.0.0.1:18081"
	subpathPrefix  = "/garage-ui"
	adminToken     = "smoke-admin-token-do-not-use-in-prod"
	adminUsername  = "smokeadmin"
	adminPassword  = "smokepass"
	readyTimeout   = 90 * time.Second
	readyPollEvery = 1 * time.Second
)

func composeFile(t testing.TB) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "docker-compose.test.yml")
}

func runCompose(t testing.TB, args ...string) string {
	t.Helper()
	full := append([]string{"compose", "-p", composeProject, "-f", composeFile(t)}, args...)
	cmd := exec.Command("docker", full...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("docker %s failed: %v\n%s", strings.Join(full, " "), err, out.String())
	}
	return out.String()
}

func composeExec(service string, argv ...string) (string, error) {
	full := append([]string{"compose", "-p", composeProject, "-f", mustComposeFile(), "exec", "-T", service}, argv...)
	cmd := exec.Command("docker", full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker %s: %w\nstderr:\n%s", strings.Join(full, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

func mustComposeFile() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "docker-compose.test.yml")
}

func waitForHTTP(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(readyPollEvery):
		}
	}
	return fmt.Errorf("timeout waiting for %s", url)
}

func initGarageLayout() error {
	nodeOut, err := composeExec("garage", "/garage", "node", "id", "-q")
	if err != nil {
		return fmt.Errorf("garage node id: %w", err)
	}
	nodeID := strings.TrimSpace(nodeOut)
	if i := strings.Index(nodeID, "@"); i > 0 {
		nodeID = nodeID[:i]
	}
	if nodeID == "" {
		return fmt.Errorf("empty node id from garage node id")
	}

	if _, err := composeExec("garage", "/garage", "layout", "assign", "-z", "dc1", "-c", "1G", nodeID); err != nil {
		return fmt.Errorf("garage layout assign: %w", err)
	}
	if _, err := composeExec("garage", "/garage", "layout", "apply", "--version", "1"); err != nil {
		return fmt.Errorf("garage layout apply: %w", err)
	}
	return nil
}

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Fprintln(os.Stderr, "[smoke] bringing up garage (waiting for healthcheck)...")
	runComposeNoT("up", "-d", "--wait", "garage")

	cleanup := func() {
		fmt.Fprintln(os.Stderr, "[smoke] tearing down compose...")
		_ = exec.Command("docker", "compose", "-p", composeProject, "-f", mustComposeFile(), "logs").Run()
		_ = exec.Command("docker", "compose", "-p", composeProject, "-f", mustComposeFile(), "down", "-v").Run()
	}

	fmt.Fprintln(os.Stderr, "[smoke] initializing garage cluster layout...")
	if err := initGarageLayout(); err != nil {
		fmt.Fprintf(os.Stderr, "[smoke] layout init failed: %v\n", err)
		cleanup()
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "[smoke] starting backend...")
	runComposeNoT("up", "-d", "backend", "backend-subpath")

	if err := waitForHTTP(ctx, backendBaseURL+"/health", readyTimeout); err != nil {
		fmt.Fprintf(os.Stderr, "[smoke] backend not ready: %v\n", err)
		cleanup()
		os.Exit(1)
	}

	if err := waitForHTTP(ctx, subpathBaseURL+subpathPrefix+"/api/v1/health", readyTimeout); err != nil {
		fmt.Fprintf(os.Stderr, "[smoke] subpath backend not ready: %v\n", err)
		cleanup()
		os.Exit(1)
	}

	code := m.Run()

	if code != 0 {
		fmt.Fprintln(os.Stderr, "[smoke] tests failed — dumping compose logs")
		logCmd := exec.Command("docker", "compose", "-p", composeProject, "-f", mustComposeFile(), "logs", "--no-color")
		logCmd.Stdout = os.Stderr
		logCmd.Stderr = os.Stderr
		_ = logCmd.Run()
	}

	_ = exec.Command("docker", "compose", "-p", composeProject, "-f", mustComposeFile(), "down", "-v").Run()
	os.Exit(code)
}

func runComposeNoT(args ...string) {
	full := append([]string{"compose", "-p", composeProject, "-f", mustComposeFile()}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[smoke] docker %s failed: %v\n", strings.Join(full, " "), err)
		os.Exit(1)
	}
}

type testState struct {
	client      *http.Client
	token       string
	bucketName  string
	objectKey   string
	sourceBody  []byte
	accessKeyID string
}

func (s *testState) do(t *testing.T, method, path string, body io.Reader, contentType string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, backendBaseURL+path, body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return b
}

func TestSmokeGoldenPath(t *testing.T) {
	sourcePath := filepath.Join(filepath.Dir(mustComposeFile()), "testdata", "small.bin")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	state := &testState{
		client:     &http.Client{Timeout: 30 * time.Second},
		bucketName: fmt.Sprintf("smoke-%d", time.Now().UnixNano()),
		objectKey:  "small.bin",
		sourceBody: source,
	}

	t.Run("AdminLogin", func(t *testing.T) {
		payload := fmt.Sprintf(`{"username":%q,"password":%q}`, adminUsername, adminPassword)
		resp := state.do(t, "POST", "/auth/login", strings.NewReader(payload), "application/json")
		body := readBody(t, resp)
		if resp.StatusCode != 200 {
			t.Fatalf("login status = %d, body = %s", resp.StatusCode, body)
		}
		var parsed struct {
			Success bool   `json:"success"`
			Token   string `json:"token"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("parse login body: %v, body=%s", err, body)
		}
		if !parsed.Success || parsed.Token == "" {
			t.Fatalf("login did not return token: %s", body)
		}
		state.token = parsed.Token
	})

	t.Run("CreateBucket", func(t *testing.T) {
		if state.token == "" {
			t.Skip("login did not succeed")
		}
		payload := fmt.Sprintf(`{"name":%q}`, state.bucketName)
		resp := state.do(t, "POST", "/api/v1/buckets", strings.NewReader(payload), "application/json")
		body := readBody(t, resp)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Fatalf("create bucket status = %d, body = %s", resp.StatusCode, body)
		}
	})

	t.Run("CreateKey", func(t *testing.T) {
		if state.token == "" {
			t.Skip("login did not succeed")
		}
		payload := `{"name":"smoke-key"}`
		resp := state.do(t, "POST", "/api/v1/users", strings.NewReader(payload), "application/json")
		body := readBody(t, resp)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Fatalf("create key status = %d, body = %s", resp.StatusCode, body)
		}
		var parsed struct {
			Data struct {
				AccessKeyID string `json:"accessKeyId"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("parse key body: %v, body=%s", err, body)
		}
		if parsed.Data.AccessKeyID == "" {
			t.Fatalf("empty accessKeyId: %s", body)
		}
		state.accessKeyID = parsed.Data.AccessKeyID
	})

	t.Run("GrantPermission", func(t *testing.T) {
		if state.accessKeyID == "" {
			t.Skip("key creation did not succeed")
		}
		payload := fmt.Sprintf(
			`{"accessKeyId":%q,"permissions":{"read":true,"write":true,"owner":false}}`,
			state.accessKeyID,
		)
		path := fmt.Sprintf("/api/v1/buckets/%s/permissions", state.bucketName)
		resp := state.do(t, "POST", path, strings.NewReader(payload), "application/json")
		body := readBody(t, resp)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Fatalf("grant permission status = %d, body = %s", resp.StatusCode, body)
		}
	})

	t.Run("UploadObject", func(t *testing.T) {
		if state.token == "" {
			t.Skip("login did not succeed")
		}
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, err := mw.CreateFormFile("file", filepath.Base(state.objectKey))
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := fw.Write(state.sourceBody); err != nil {
			t.Fatalf("write form file: %v", err)
		}
		if err := mw.WriteField("key", state.objectKey); err != nil {
			t.Fatalf("write key field: %v", err)
		}
		if err := mw.Close(); err != nil {
			t.Fatalf("close multipart: %v", err)
		}
		path := fmt.Sprintf("/api/v1/buckets/%s/objects/", state.bucketName)
		resp := state.do(t, "POST", path, &buf, mw.FormDataContentType())
		body := readBody(t, resp)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Fatalf("upload status = %d, body = %s", resp.StatusCode, body)
		}
	})

	t.Run("ListObjects", func(t *testing.T) {
		if state.token == "" {
			t.Skip("login did not succeed")
		}
		path := fmt.Sprintf("/api/v1/buckets/%s/objects/", state.bucketName)
		resp := state.do(t, "GET", path, nil, "")
		body := readBody(t, resp)
		if resp.StatusCode != 200 {
			t.Fatalf("list status = %d, body = %s", resp.StatusCode, body)
		}
		if !bytes.Contains(body, []byte(state.objectKey)) {
			t.Fatalf("uploaded key %q not found in list: %s", state.objectKey, body)
		}
	})

	t.Run("DownloadObject", func(t *testing.T) {
		if state.token == "" {
			t.Skip("login did not succeed")
		}
		path := fmt.Sprintf("/api/v1/buckets/%s/objects/%s", state.bucketName, state.objectKey)
		resp := state.do(t, "GET", path, nil, "")
		body := readBody(t, resp)
		if resp.StatusCode != 200 {
			t.Fatalf("download status = %d, body = %s", resp.StatusCode, body)
		}
		if !bytes.Equal(body, state.sourceBody) {
			t.Fatalf("downloaded bytes differ: got %d bytes, want %d bytes", len(body), len(state.sourceBody))
		}
	})

	t.Run("DeleteObject", func(t *testing.T) {
		if state.token == "" {
			t.Skip("login did not succeed")
		}
		path := fmt.Sprintf("/api/v1/buckets/%s/objects/%s", state.bucketName, state.objectKey)
		resp := state.do(t, "DELETE", path, nil, "")
		body := readBody(t, resp)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Fatalf("delete object status = %d, body = %s", resp.StatusCode, body)
		}
	})

	t.Run("DeleteBucket", func(t *testing.T) {
		if state.token == "" {
			t.Skip("login did not succeed")
		}
		path := fmt.Sprintf("/api/v1/buckets/%s", state.bucketName)
		resp := state.do(t, "DELETE", path, nil, "")
		body := readBody(t, resp)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Fatalf("delete bucket status = %d, body = %s", resp.StatusCode, body)
		}
	})

	t.Run("ListKeys", func(t *testing.T) {
		if state.token == "" {
			t.Skip("login did not succeed")
		}
		resp := state.do(t, "GET", "/api/v1/users", nil, "")
		body := readBody(t, resp)
		if resp.StatusCode != 200 {
			t.Fatalf("list users status = %d, body = %s", resp.StatusCode, body)
		}
	})
}
