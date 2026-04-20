package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Noooste/garage-ui/internal/apierr"
	"Noooste/garage-ui/internal/models"
	"Noooste/garage-ui/internal/services/mocks"

	"github.com/gofiber/fiber/v3"
)

func newUsersTestApp(t *testing.T) (*fiber.App, *mocks.AdminMock) {
	t.Helper()
	admin := &mocks.AdminMock{}
	h := NewUserHandler(admin)
	app := fiber.New()
	app.Get("/users", h.ListUsers)
	app.Post("/users", h.CreateUser)
	app.Get("/users/:access_key", h.GetUser)
	app.Get("/users/:access_key/secret", h.GetUserSecretKey)
	app.Delete("/users/:access_key", h.DeleteUser)
	app.Patch("/users/:access_key", h.UpdateUserPermissions)
	return app, admin
}

// --- ListUsers ---

func TestListUsers_MapsAndSkipsFailed(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.ListKeysFn = func(_ context.Context) ([]models.ListKeysResponseItem, error) {
		return []models.ListKeysResponseItem{
			{ID: "AKIA-1", Name: "one"},
			{ID: "AKIA-2", Name: "bad-detail"},
			{ID: "AKIA-3", Name: "three"},
		}, nil
	}
	admin.GetKeyInfoFn = func(_ context.Context, id string, showSecret bool) (*models.GarageKeyInfo, error) {
		if showSecret {
			t.Error("ListUsers must not request secret")
		}
		if id == "AKIA-2" {
			return nil, errors.New("forbidden")
		}
		return &models.GarageKeyInfo{AccessKeyID: id, Name: id, Expired: false}, nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/users", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Data models.UserListResponse `json:"data"`
	}
	decodeJSON(t, resp.Body, &body)
	if body.Data.Count != 2 {
		t.Errorf("count = %d, want 2 (failed detail skipped)", body.Data.Count)
	}
}

func TestListUsers_ListError500(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.ListKeysFn = func(_ context.Context) ([]models.ListKeysResponseItem, error) {
		return nil, errors.New("boom")
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/users", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

// --- CreateUser ---

func TestCreateUser_Success201(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.CreateKeyFn = func(_ context.Context, req models.CreateKeyRequest) (*models.GarageKeyInfo, error) {
		if req.Name == nil || *req.Name != "alice" {
			t.Errorf("Name = %v", req.Name)
		}
		sk := "secret-xyz"
		return &models.GarageKeyInfo{AccessKeyID: "AKIA-1", Name: "alice", SecretAccessKey: &sk}, nil
	}
	body, _ := json.Marshal(models.CreateUserRequest{Name: "alice"})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var decoded struct {
		Data models.UserInfo `json:"data"`
	}
	decodeJSON(t, resp.Body, &decoded)
	if decoded.Data.SecretKey == nil || *decoded.Data.SecretKey != "secret-xyz" {
		t.Errorf("SecretKey = %v, want secret-xyz", decoded.Data.SecretKey)
	}
}

func TestCreateUser_MalformedJSON400(t *testing.T) {
	app, _ := newUsersTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateUser_AdminError500(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.CreateKeyFn = func(_ context.Context, _ models.CreateKeyRequest) (*models.GarageKeyInfo, error) {
		return nil, errors.New("boom")
	}
	body, _ := json.Marshal(models.CreateUserRequest{})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

// --- GetUser ---

func TestGetUser_Success(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.GetKeyInfoFn = func(_ context.Context, id string, showSecret bool) (*models.GarageKeyInfo, error) {
		if showSecret {
			t.Error("GetUser must not request secret")
		}
		return &models.GarageKeyInfo{AccessKeyID: id, Name: "alice"}, nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/users/AKIA-1", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestGetUser_ServiceError500(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.GetKeyInfoFn = func(_ context.Context, _ string, _ bool) (*models.GarageKeyInfo, error) {
		return nil, errors.New("boom")
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/users/AKIA-1", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

// --- GetUserSecretKey ---

func TestGetUserSecretKey_Success(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.GetKeyInfoFn = func(_ context.Context, id string, showSecret bool) (*models.GarageKeyInfo, error) {
		if !showSecret {
			t.Error("GetUserSecretKey must request secret")
		}
		sk := "s3cr3t"
		return &models.GarageKeyInfo{AccessKeyID: id, SecretAccessKey: &sk}, nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/users/AKIA-1/secret", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Data map[string]string `json:"data"`
	}
	decodeJSON(t, resp.Body, &body)
	if body.Data["secretKey"] != "s3cr3t" {
		t.Errorf("secretKey = %q", body.Data["secretKey"])
	}
}

// --- DeleteUser ---

func TestDeleteUser_Success(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.DeleteKeyFn = func(_ context.Context, id string) error {
		if id != "AKIA-1" {
			t.Errorf("id = %q", id)
		}
		return nil
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodDelete, "/users/AKIA-1", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestDeleteUser_ServiceError500(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.DeleteKeyFn = func(_ context.Context, _ string) error { return errors.New("boom") }
	resp, err := app.Test(httptest.NewRequest(http.MethodDelete, "/users/AKIA-1", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

// --- UpdateUserPermissions ---

func TestUpdateUser_StatusInactiveSetsPastExpiration(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.UpdateKeyFn = func(_ context.Context, _ string, req models.UpdateKeyRequest) (*models.GarageKeyInfo, error) {
		if req.NeverExpires {
			t.Error("NeverExpires should be false when deactivating")
		}
		if req.Expiration == nil || !req.Expiration.Before(time.Now()) {
			t.Errorf("Expiration = %v, want past time", req.Expiration)
		}
		return &models.GarageKeyInfo{}, nil
	}
	status := "inactive"
	body, _ := json.Marshal(models.UpdateUserRequest{Status: &status})
	req := httptest.NewRequest(http.MethodPatch, "/users/AKIA-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestUpdateUser_StatusActiveSetsNeverExpires(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.UpdateKeyFn = func(_ context.Context, _ string, req models.UpdateKeyRequest) (*models.GarageKeyInfo, error) {
		if !req.NeverExpires {
			t.Error("NeverExpires should be true when activating")
		}
		return &models.GarageKeyInfo{}, nil
	}
	status := "active"
	body, _ := json.Marshal(models.UpdateUserRequest{Status: &status})
	req := httptest.NewRequest(http.MethodPatch, "/users/AKIA-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestUpdateUser_ExplicitExpiration(t *testing.T) {
	app, admin := newUsersTestApp(t)
	wantTime, _ := time.Parse(time.RFC3339, "2030-01-02T03:04:05Z")
	admin.UpdateKeyFn = func(_ context.Context, _ string, req models.UpdateKeyRequest) (*models.GarageKeyInfo, error) {
		if req.Expiration == nil || !req.Expiration.Equal(wantTime) {
			t.Errorf("Expiration = %v, want %v", req.Expiration, wantTime)
		}
		if req.NeverExpires {
			t.Error("NeverExpires should be false with explicit expiration")
		}
		return &models.GarageKeyInfo{}, nil
	}
	exp := "2030-01-02T03:04:05Z"
	body, _ := json.Marshal(models.UpdateUserRequest{Expiration: &exp})
	req := httptest.NewRequest(http.MethodPatch, "/users/AKIA-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestUpdateUser_BadExpirationFormat400(t *testing.T) {
	app, _ := newUsersTestApp(t)
	exp := "not-a-date"
	body, _ := json.Marshal(models.UpdateUserRequest{Expiration: &exp})
	req := httptest.NewRequest(http.MethodPatch, "/users/AKIA-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUpdateUser_MalformedJSON400(t *testing.T) {
	app, _ := newUsersTestApp(t)
	req := httptest.NewRequest(http.MethodPatch, "/users/AKIA-1", strings.NewReader("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUpdateUser_AdminError500(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.UpdateKeyFn = func(_ context.Context, _ string, _ models.UpdateKeyRequest) (*models.GarageKeyInfo, error) {
		return nil, errors.New("boom")
	}
	status := "active"
	body, _ := json.Marshal(models.UpdateUserRequest{Status: &status})
	req := httptest.NewRequest(http.MethodPatch, "/users/AKIA-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestDeleteUser_UpstreamAccessDenied(t *testing.T) {
	app, admin := newUsersTestApp(t)
	admin.DeleteKeyFn = func(_ context.Context, _ string) error {
		return &apierr.UpstreamError{
			HTTPStatus: 403,
			Code:       "AccessDenied",
			Message:    "permission denied",
			Source:     "garage",
		}
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodDelete, "/users/abc", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	var body models.APIResponse
	decodeJSON(t, resp.Body, &body)
	if body.Error == nil || body.Error.Code != models.ErrCodeForbidden {
		t.Fatalf("error = %+v, want code %s", body.Error, models.ErrCodeForbidden)
	}
	if body.Error.Message != "permission denied" {
		t.Errorf("message = %q", body.Error.Message)
	}
}
