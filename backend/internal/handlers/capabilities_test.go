package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"Noooste/garage-ui/internal/authz"
	"Noooste/garage-ui/internal/services"

	"github.com/gofiber/fiber/v3"
)

func TestCapabilities_V2(t *testing.T) {
	app := fiber.New()
	h := NewCapabilitiesHandler("v2", services.CapabilitiesV2(), false)
	app.Get("/capabilities", h.GetCapabilities)

	req := httptest.NewRequest(http.MethodGet, "/capabilities", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			GarageApiVersion string                `json:"garageApiVersion"`
			Features         services.Capabilities `json:"features"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Success {
		t.Fatal("expected success=true")
	}
	if body.Data.GarageApiVersion != "v2" {
		t.Errorf("garageApiVersion = %q, want v2", body.Data.GarageApiVersion)
	}
	if !body.Data.Features.ClusterStatistics || !body.Data.Features.NodeInfo || !body.Data.Features.NodeStatistics {
		t.Errorf("features = %+v, want all true", body.Data.Features)
	}
}

func TestCapabilities_V1(t *testing.T) {
	app := fiber.New()
	h := NewCapabilitiesHandler("v1", services.CapabilitiesV1(), false)
	app.Get("/capabilities", h.GetCapabilities)

	req := httptest.NewRequest(http.MethodGet, "/capabilities", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var body struct {
		Data struct {
			GarageApiVersion string                `json:"garageApiVersion"`
			Features         services.Capabilities `json:"features"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Data.GarageApiVersion != "v1" {
		t.Errorf("garageApiVersion = %q, want v1", body.Data.GarageApiVersion)
	}
	if body.Data.Features.ClusterStatistics || body.Data.Features.NodeInfo || body.Data.Features.NodeStatistics {
		t.Errorf("features = %+v, want all false", body.Data.Features)
	}
}

func TestSortedPermsEmptyReturnsNil(t *testing.T) {
	if got := sortedPerms(authz.PermSet{}); got != nil {
		t.Errorf("sortedPerms(empty) = %v, want nil", got)
	}
	if got := sortedPerms(nil); got != nil {
		t.Errorf("sortedPerms(nil) = %v, want nil", got)
	}
}

func TestGetCapabilitiesAccessControlDisabled(t *testing.T) {
	h := NewCapabilitiesHandler("v2", services.CapabilitiesV2(), false)
	app := fiber.New()
	app.Get("/capabilities", h.GetCapabilities)
	resp, err := app.Test(httptest.NewRequest("GET", "/capabilities", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Data struct {
			AccessControl struct {
				Enabled bool `json:"enabled"`
			} `json:"access_control"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.AccessControl.Enabled {
		t.Error("enabled should be false when access control is off")
	}
}

func TestGetCapabilitiesAccessControlSubject(t *testing.T) {
	h := NewCapabilitiesHandler("v2", services.CapabilitiesV2(), true)
	app := fiber.New()
	app.Get("/capabilities", func(c fiber.Ctx) error {
		c.Locals(authz.SubjectLocalsKey, authz.Subject{
			ID: "alice@example.com",
			Bindings: []authz.Binding{{
				BucketPrefixes: []string{"backend-"},
				Permissions:    authz.PermSet{"bucket.list": {}, "bucket.read": {}},
			}},
			ClusterPerms: authz.PermSet{"cluster.status": {}},
		})
		return h.GetCapabilities(c)
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/capabilities", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Data struct {
			AccessControl struct {
				Enabled            bool     `json:"enabled"`
				Subject            string   `json:"subject"`
				IsAdmin            bool     `json:"is_admin"`
				ClusterPermissions []string `json:"cluster_permissions"`
				Bindings           []struct {
					BucketPrefixes []string `json:"bucket_prefixes"`
					Permissions    []string `json:"permissions"`
				} `json:"bindings"`
			} `json:"access_control"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	ac := envelope.Data.AccessControl
	if !ac.Enabled || ac.Subject != "alice@example.com" || ac.IsAdmin {
		t.Errorf("unexpected access_control header fields: %+v", ac)
	}
	if len(ac.Bindings) != 1 || len(ac.Bindings[0].Permissions) != 2 {
		t.Errorf("bindings not mirrored unflattened: %+v", ac.Bindings)
	}
	if len(ac.ClusterPermissions) != 1 || ac.ClusterPermissions[0] != "cluster.status" {
		t.Errorf("cluster_permissions = %v", ac.ClusterPermissions)
	}
}
