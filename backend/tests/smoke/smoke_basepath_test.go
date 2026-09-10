//go:build smoke

package smoke_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Issue #107: the published image must be able to serve from a subpath, so
// this runs a second instance of the very same image with
// GARAGE_UI_SERVER_BASE_PATH=/garage-ui and drives it end to end — real
// binary, real frontend build, real Garage cluster.
//
// The contract is "accept the prefix, don't require it", so both spellings are
// exercised: the prefixed one for proxies that forward the mount point, the
// unprefixed one for those that strip it (tailscale serve, Traefik
// StripPrefix, k8s rewrite-target) and for the image's own HEALTHCHECK.

// noRedirectClient keeps 3xx responses intact so redirect targets can be
// asserted.
func noRedirectClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func subpathGet(t *testing.T, client *http.Client, path string, token string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, subpathBaseURL+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp, readBody(t, resp)
}

func TestSmokeSubpathDeployment(t *testing.T) {
	client := noRedirectClient()

	t.Run("HealthUnderPrefix", func(t *testing.T) {
		resp, body := subpathGet(t, client, subpathPrefix+"/api/v1/health", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
		}
	})

	// The image's HEALTHCHECK and the Helm probes hit the unprefixed path.
	t.Run("ProbeHealthStaysAtRoot", func(t *testing.T) {
		resp, body := subpathGet(t, client, "/health", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
		}
	})

	// What a stripping proxy forwards: the same request without the prefix.
	t.Run("APIAnswersStripped", func(t *testing.T) {
		resp, body := subpathGet(t, client, "/api/v1/health", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/v1/health = %d, body = %s — a stripping proxy would break", resp.StatusCode, body)
		}
	})

	t.Run("BarePrefixServesTheShell", func(t *testing.T) {
		resp, body := subpathGet(t, client, subpathPrefix, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d", subpathPrefix, resp.StatusCode)
		}
		// The injected absolute base href anchors the relative asset URLs even
		// without a trailing slash, so no redirect is needed here.
		if !strings.Contains(string(body), `<base href="`+subpathPrefix+`/">`) {
			t.Errorf("bare prefix shell missing the base href:\n%s", body)
		}
	})

	var assetURL string

	t.Run("SPAShellCarriesInjectedBasePath", func(t *testing.T) {
		resp, body := subpathGet(t, client, subpathPrefix+"/", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
		}
		html := string(body)
		if !strings.Contains(html, `<base href="`+subpathPrefix+`/">`) {
			t.Errorf("index.html was not rewritten with the base href:\n%s", html)
		}
		if !strings.Contains(html, `<meta name="garage-ui-base-path" content="`+subpathPrefix+`">`) {
			t.Errorf("index.html does not expose the base path to the SPA:\n%s", html)
		}

		// Assets must be referenced relatively so the base href anchors them.
		asset := regexp.MustCompile(`src="(\./assets/[^"]+\.js)"`).FindStringSubmatch(html)
		if asset == nil {
			t.Fatalf("no relative asset reference found in index.html:\n%s", html)
		}
		assetURL = subpathPrefix + "/" + strings.TrimPrefix(asset[1], "./")
	})

	t.Run("AssetsResolveUnderPrefix", func(t *testing.T) {
		if assetURL == "" {
			t.Skip("no asset URL captured")
		}
		resp, body := subpathGet(t, client, assetURL, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d", assetURL, resp.StatusCode)
		}
		// And stripped, the way tailscale serve would forward it.
		strippedResp, _ := subpathGet(t, client, strings.TrimPrefix(assetURL, subpathPrefix), "")
		if strippedResp.StatusCode != http.StatusOK {
			t.Errorf("stripped asset request = %d, want 200", strippedResp.StatusCode)
		}
		if len(body) == 0 {
			t.Error("asset body is empty")
		}
		if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
			t.Errorf("Content-Type = %q, want a javascript type", ct)
		}
	})

	// A deep client-side route is served the same shell; the relative asset
	// URLs still resolve because of the injected base href. Both spellings,
	// because a stripping proxy delivers the second one.
	t.Run("DeepClientRouteServesShell", func(t *testing.T) {
		for _, path := range []string{
			subpathPrefix + "/buckets/some-bucket/objects/a/b.txt",
			"/buckets/some-bucket/objects/a/b.txt",
		} {
			resp, body := subpathGet(t, client, path, "")
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s = %d, body = %s", path, resp.StatusCode, body)
			}
			if !strings.Contains(string(body), `<base href="`+subpathPrefix+`/">`) {
				t.Errorf("GET %s: shell missing the base href:\n%s", path, body)
			}
		}
	})

	t.Run("UnauthenticatedAPIIs401NotHTML", func(t *testing.T) {
		resp, body := subpathGet(t, client, subpathPrefix+"/api/v1/buckets", "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401, body = %s", resp.StatusCode, body)
		}
		if strings.Contains(string(body), "<div id=\"root\">") {
			t.Errorf("API error was answered with the SPA shell: %s", body)
		}
	})

	t.Run("UnauthenticatedAPIIs401Stripped", func(t *testing.T) {
		resp, _ := subpathGet(t, client, "/api/v1/buckets", "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("stripped API status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("LoginAndListBucketsThroughPrefix", func(t *testing.T) {
		payload := fmt.Sprintf(`{"username":%q,"password":%q}`, adminUsername, adminPassword)
		req, err := http.NewRequest(http.MethodPost, subpathBaseURL+subpathPrefix+"/auth/login", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("login: %v", err)
		}
		body := readBody(t, resp)
		if resp.StatusCode != http.StatusOK {
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
			t.Fatalf("login did not return a token: %s", body)
		}

		listResp, listBody := subpathGet(t, client, subpathPrefix+"/api/v1/buckets", parsed.Token)
		if listResp.StatusCode != http.StatusOK {
			t.Fatalf("list buckets status = %d, body = %s", listResp.StatusCode, listBody)
		}
	})
}
