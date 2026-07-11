package handlers

import (
	"testing"

	"Noooste/garage-ui/internal/authz"
	"Noooste/garage-ui/internal/models"
)

func teamSubject() authz.Subject {
	return authz.Subject{
		ID: "alice",
		Bindings: []authz.Binding{{
			BucketPrefixes: []string{"backend-"},
			Permissions:    authz.PermSet{"bucket.list": {}, "bucket.read": {}, "object.read": {}},
		}},
	}
}

func TestFilterBucketsForSubject(t *testing.T) {
	buckets := []models.BucketInfo{
		{Name: "backend-api"},
		{Name: "backend-assets"},
		{Name: "data-warehouse"},
	}
	got := filterBucketsForSubject(buckets, teamSubject())
	if len(got) != 2 {
		t.Fatalf("filtered to %d buckets, want 2", len(got))
	}
	for _, b := range got {
		if b.Name == "data-warehouse" {
			t.Error("data-warehouse must be filtered out")
		}
		if len(b.EffectivePermissions) == 0 {
			t.Errorf("%s: effective_permissions must be populated", b.Name)
		}
	}
}

func TestFilterBucketsAdminSeesAll(t *testing.T) {
	buckets := []models.BucketInfo{{Name: "a"}, {Name: "b"}}
	got := filterBucketsForSubject(buckets, authz.AdminSubject("root"))
	if len(got) != 2 {
		t.Fatalf("admin sees %d buckets, want 2", len(got))
	}
}
