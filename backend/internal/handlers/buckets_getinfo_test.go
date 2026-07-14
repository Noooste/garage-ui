package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Noooste/garage-ui/internal/authz"
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services/mocks"

	"github.com/gofiber/fiber/v3"
)

func TestGetBucketInfoPopulatesEffectivePermissions(t *testing.T) {
	admin := &mocks.AdminMock{}
	admin.GetBucketInfoByAliasFn = func(_ context.Context, _ string) (*models.GarageBucketInfo, error) {
		return &models.GarageBucketInfo{ID: "id-1"}, nil
	}
	h := NewBucketHandler(admin, nil)

	app := fiber.New()
	app.Get("/buckets/:name", func(c fiber.Ctx) error {
		c.Locals(authz.SubjectLocalsKey, teamSubject())
		return h.GetBucketInfo(c)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/buckets/backend-api", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Data models.GarageBucketInfo `json:"data"`
	}
	decodeJSON(t, resp.Body, &body)
	if len(body.Data.EffectivePermissions) == 0 {
		t.Error("effective_permissions must be populated for a subject in scope of the bucket")
	}
}
