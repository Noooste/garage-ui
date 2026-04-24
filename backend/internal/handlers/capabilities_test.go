package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Noooste/garage-ui/internal/services"

	"github.com/gofiber/fiber/v3"
)

func TestCapabilities_V2(t *testing.T) {
	app := fiber.New()
	h := NewCapabilitiesHandler("v2", services.CapabilitiesV2())
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
			GarageApiVersion string              `json:"garageApiVersion"`
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
	h := NewCapabilitiesHandler("v1", services.CapabilitiesV1())
	app.Get("/capabilities", h.GetCapabilities)

	req := httptest.NewRequest(http.MethodGet, "/capabilities", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var body struct {
		Data struct {
			GarageApiVersion string              `json:"garageApiVersion"`
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
