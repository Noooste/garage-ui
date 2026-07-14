package authz

import (
	"sort"
	"testing"
)

func TestExpandGlobRejectsNonGlob(t *testing.T) {
	// A pattern without a trailing star is not a glob and expands to nothing.
	if got := ExpandGlob("bucket.read"); got != nil {
		t.Errorf("ExpandGlob(%q) = %v, want nil", "bucket.read", got)
	}
}

func TestVocabularyContainsRatifiedPermissions(t *testing.T) {
	// Spot-check one permission per family plus scope/admin flags.
	cases := []struct {
		perm      string
		scope     ScopeKind
		adminOnly bool
	}{
		{"bucket.list", ScopePrefix, false},
		{"bucket.read", ScopePrefix, false},
		{"bucket.create", ScopePrefix, false},
		{"bucket_alias.add", ScopePrefix, false},
		{"object.list", ScopePrefix, false},
		{"object.read", ScopePrefix, false},
		{"object.write", ScopePrefix, false},
		{"object.delete", ScopePrefix, false},
		{"permission.allow_bucket_key", ScopePrefix, false},
		{"key.list", ScopeGlobal, false},
		{"key.read", ScopeGlobal, false},
		{"key.read_secret", ScopeGlobal, true},
		{"key.create", ScopeGlobal, true},
		{"key.import", ScopeGlobal, true},
		{"key.update", ScopeGlobal, true},
		{"key.delete", ScopeGlobal, true},
		{"cluster.status", ScopeGlobal, false},
		{"cluster.layout.apply", ScopeGlobal, false},
		{"node.repair", ScopeGlobal, false},
		{"worker.set_variable", ScopeGlobal, false},
		{"block.info", ScopeGlobal, false},
	}
	for _, tc := range cases {
		spec, ok := Vocabulary[tc.perm]
		if !ok {
			t.Errorf("missing permission %q", tc.perm)
			continue
		}
		if spec.Scope != tc.scope {
			t.Errorf("%s: scope = %v, want %v", tc.perm, spec.Scope, tc.scope)
		}
		if spec.AdminOnly != tc.adminOnly {
			t.Errorf("%s: adminOnly = %v, want %v", tc.perm, spec.AdminOnly, tc.adminOnly)
		}
	}
	if _, ok := Vocabulary["admin_token.list"]; ok {
		t.Error("admin_token.* must not be in the v1 vocabulary")
	}
	if len(Vocabulary) != 40 {
		t.Errorf("vocabulary size = %d, want 40", len(Vocabulary))
	}
}

func TestIsValidPermission(t *testing.T) {
	if !IsValidPermission("bucket.read") {
		t.Error("bucket.read should be valid")
	}
	if IsValidPermission("bucket.explode") {
		t.Error("bucket.explode should be invalid")
	}
	if IsValidPermission("bucket.*") {
		t.Error("globs are not permissions; IsValidPermission must reject them")
	}
}

func TestExpandGlob(t *testing.T) {
	got := ExpandGlob("object.*")
	sort.Strings(got)
	want := []string{"object.delete", "object.list", "object.read", "object.write"}
	if len(got) != len(want) {
		t.Fatalf("object.* expanded to %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("object.* expanded to %v, want %v", got, want)
		}
	}

	// key.* must NOT include admin-only permissions.
	for _, p := range ExpandGlob("key.*") {
		if Vocabulary[p].AdminOnly {
			t.Errorf("glob expansion included admin-only permission %q", p)
		}
	}

	// cluster.* includes cluster.layout.* (prefix match on the dotted name).
	found := false
	for _, p := range ExpandGlob("cluster.*") {
		if p == "cluster.layout.apply" {
			found = true
		}
	}
	if !found {
		t.Error("cluster.* should include cluster.layout.apply")
	}

	// Bare * expands to every non-admin-only permission.
	star := ExpandGlob("*")
	if len(star) == 0 {
		t.Fatal("* expanded to nothing")
	}
	for _, p := range star {
		if Vocabulary[p].AdminOnly {
			t.Errorf("* expansion included admin-only permission %q", p)
		}
	}

	if got := ExpandGlob("nonexistent.*"); got != nil {
		t.Errorf("nonexistent.* should expand to nil, got %v", got)
	}
}
