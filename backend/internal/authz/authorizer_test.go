package authz

import (
	"sort"
	"testing"
)

func testSubject() Subject {
	return Subject{
		ID: "alice@example.com",
		Bindings: []Binding{
			{BucketPrefixes: []string{"backend-"}, Permissions: PermSet{
				"bucket.list": {}, "bucket.read": {}, "bucket.create": {}, "object.read": {}, "object.write": {},
			}},
			{BucketPrefixes: []string{"shared-"}, Permissions: PermSet{
				"bucket.list": {}, "bucket.read": {}, "object.read": {},
			}},
		},
		ClusterPerms: PermSet{"cluster.status": {}},
	}
}

func TestDecidePrefixScoped(t *testing.T) {
	s := testSubject()
	cases := []struct {
		action, bucket string
		allow          bool
	}{
		{"bucket.read", "backend-api", true},
		{"bucket.read", "shared-docs", true},
		{"bucket.read", "data-warehouse", false}, // no binding matches
		{"object.write", "backend-api", true},
		{"object.write", "shared-docs", false},   // readonly binding
		{"bucket.create", "backend-new", true},   // prefix guard: new name matches
		{"bucket.create", "frontend-new", false}, // prefix guard: no match
		{"bucket.delete", "backend-api", false},  // permission not granted at all
	}
	for _, tc := range cases {
		d := Decide(s, tc.action, Resource{Bucket: tc.bucket})
		if d.Allow != tc.allow {
			t.Errorf("Decide(%s, %s) = %v (%s), want %v", tc.action, tc.bucket, d.Allow, d.Reason, tc.allow)
		}
	}
}

func TestDecideUnscopedListEndpoint(t *testing.T) {
	// GET /buckets carries no bucket name: allowed if ANY binding grants
	// bucket.list (the response is filtered per bucket afterwards).
	s := testSubject()
	if d := Decide(s, "bucket.list", Resource{}); !d.Allow {
		t.Errorf("bucket.list with empty resource should be allowed for a subject holding it in any binding: %s", d.Reason)
	}
	noList := Subject{ID: "bob", Bindings: []Binding{{BucketPrefixes: []string{"x-"}, Permissions: PermSet{"object.read": {}}}}}
	if d := Decide(noList, "bucket.list", Resource{}); d.Allow {
		t.Error("bucket.list should be denied when no binding grants it")
	}
}

func TestDecideGlobal(t *testing.T) {
	s := testSubject()
	if d := Decide(s, "cluster.status", Resource{}); !d.Allow {
		t.Errorf("cluster.status should be allowed: %s", d.Reason)
	}
	if d := Decide(s, "cluster.statistics", Resource{}); d.Allow {
		t.Error("cluster.statistics should be denied")
	}
	if d := Decide(s, "key.create", Resource{}); d.Allow {
		t.Error("admin-only key.create should be denied for a team subject")
	}
}

func TestDecideUnknownPermission(t *testing.T) {
	if d := Decide(testSubject(), "bucket.explode", Resource{}); d.Allow {
		t.Error("unknown permission must be denied")
	}
}

func TestAdminSubject(t *testing.T) {
	a := AdminSubject("root")
	if !a.IsAdmin {
		t.Error("AdminSubject must set IsAdmin")
	}
	// Admin goes through the exact same Decide path, no shortcut.
	for _, action := range []string{"bucket.delete", "object.write", "key.create", "key.read_secret", "cluster.layout.apply", "node.repair"} {
		if d := Decide(a, action, Resource{Bucket: "any-bucket-at-all"}); !d.Allow {
			t.Errorf("admin denied %s: %s", action, d.Reason)
		}
	}
}

func TestEffectivePermissions(t *testing.T) {
	s := testSubject()
	got := EffectivePermissions(s, "backend-api")
	want := []string{"bucket.create", "bucket.list", "bucket.read", "object.read", "object.write"}
	if !equalStrings(got, want) {
		t.Errorf("EffectivePermissions(backend-api) = %v, want %v", got, want)
	}
	if got := EffectivePermissions(s, "data-x"); got != nil {
		t.Errorf("EffectivePermissions(data-x) = %v, want nil", got)
	}
	admin := EffectivePermissions(AdminSubject("root"), "anything")
	if !sort.StringsAreSorted(admin) || len(admin) == 0 {
		t.Errorf("admin effective permissions should be all prefix-scoped perms, sorted; got %v", admin)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
