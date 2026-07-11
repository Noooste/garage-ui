package authz

import (
	"strings"
	"testing"

	"Noooste/garage-ui/internal/config"
)

func validAC() *config.AccessControlConfig {
	return &config.AccessControlConfig{
		Presets: map[string][]string{
			"bucket_readonly": {"bucket.list", "bucket.read", "object.list", "object.read"},
			"bucket_owner":    {"preset:bucket_readonly", "bucket.create", "bucket.update", "bucket.delete", "object.write", "object.delete"},
		},
		Teams: []config.TeamConfig{
			{
				Name:        "backend",
				ClaimValues: []string{"garage-team-backend"},
				Bindings: []config.BindingConfig{
					{BucketPrefixes: []string{"backend-"}, Permissions: []string{"preset:bucket_owner"}},
					{BucketPrefixes: []string{"shared-"}, Permissions: []string{"preset:bucket_readonly"}},
				},
				ClusterPermissions: []string{"cluster.status", "cluster.health"},
			},
		},
	}
}

func TestCompilePolicyNilConfig(t *testing.T) {
	p, err := CompilePolicy(nil)
	if err != nil {
		t.Fatalf("CompilePolicy(nil): %v", err)
	}
	if p.Enabled {
		t.Error("nil config must compile to disabled policy")
	}
}

func TestCompilePolicyResolvesPresetsAndGlobs(t *testing.T) {
	cfg := validAC()
	cfg.Teams[0].Bindings[0].Permissions = append(cfg.Teams[0].Bindings[0].Permissions, "bucket_alias.*")
	p, err := CompilePolicy(cfg)
	if err != nil {
		t.Fatalf("CompilePolicy: %v", err)
	}
	if !p.Enabled {
		t.Fatal("policy should be enabled")
	}
	b0 := p.Teams[0].Bindings[0].Permissions
	for _, want := range []string{"bucket.list", "bucket.read", "bucket.create", "object.delete", "bucket_alias.add", "bucket_alias.remove"} {
		if _, ok := b0[want]; !ok {
			t.Errorf("binding 0 missing %q after preset/glob resolution: %v", want, b0)
		}
	}
	if _, ok := p.Teams[0].Bindings[1].Permissions["bucket.create"]; ok {
		t.Error("readonly binding must not gain owner permissions")
	}
	if _, ok := p.Teams[0].ClusterPerms["cluster.status"]; !ok {
		t.Error("cluster_permissions not compiled")
	}
}

func TestCompilePolicyValidationErrors(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*config.AccessControlConfig)
		errPart string
	}{
		{"unknown permission", func(c *config.AccessControlConfig) {
			c.Teams[0].Bindings[0].Permissions = []string{"bucket.explode"}
		}, "unknown permission"},
		{"unknown preset", func(c *config.AccessControlConfig) {
			c.Teams[0].Bindings[0].Permissions = []string{"preset:nope"}
		}, "unknown preset"},
		{"preset cycle", func(c *config.AccessControlConfig) {
			c.Presets["a"] = []string{"preset:b"}
			c.Presets["b"] = []string{"preset:a"}
			c.Teams[0].Bindings[0].Permissions = []string{"preset:a"}
		}, "cycle"},
		{"admin-only to team", func(c *config.AccessControlConfig) {
			c.Teams[0].ClusterPermissions = []string{"key.create"}
		}, "admin-only"},
		{"duplicate team name", func(c *config.AccessControlConfig) {
			c.Teams = append(c.Teams, c.Teams[0])
		}, "duplicate team"},
		{"empty claim_values", func(c *config.AccessControlConfig) {
			c.Teams[0].ClaimValues = nil
		}, "claim_values"},
		{"no bindings or cluster perms", func(c *config.AccessControlConfig) {
			c.Teams[0].Bindings = nil
			c.Teams[0].ClusterPermissions = nil
		}, "at least one"},
		{"prefix-scoped perm in cluster_permissions", func(c *config.AccessControlConfig) {
			c.Teams[0].ClusterPermissions = []string{"bucket.read"}
		}, "prefix-scoped"},
		{"global perm in binding", func(c *config.AccessControlConfig) {
			c.Teams[0].Bindings[0].Permissions = []string{"cluster.status"}
		}, "global"},
		{"binding without prefixes", func(c *config.AccessControlConfig) {
			c.Teams[0].Bindings[0].BucketPrefixes = nil
		}, "bucket_prefixes"},
		{"binding with empty permissions", func(c *config.AccessControlConfig) {
			c.Teams[0].Bindings[0].Permissions = nil
		}, "has no permissions"},
		{"empty team name", func(c *config.AccessControlConfig) {
			c.Teams[0].Name = ""
		}, "has no name"},
		{"glob matches nothing in binding", func(c *config.AccessControlConfig) {
			c.Teams[0].Bindings[0].Permissions = []string{"nonexistent.*"}
		}, "matches no permission"},
		{"preset references unknown preset", func(c *config.AccessControlConfig) {
			c.Presets["broken"] = []string{"preset:ghost"}
		}, "unknown preset"},
		{"glob matches nothing in preset", func(c *config.AccessControlConfig) {
			c.Presets["globby"] = []string{"nonexistent.*"}
		}, "matches no permission"},
		{"unknown permission in preset", func(c *config.AccessControlConfig) {
			c.Presets["badperm"] = []string{"bucket.explode"}
		}, "unknown permission"},
		{"admin-only permission in preset", func(c *config.AccessControlConfig) {
			c.Presets["adminy"] = []string{"key.create"}
		}, "admin-only"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validAC()
			tc.mutate(cfg)
			_, err := CompilePolicy(cfg)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errPart)
			}
			if !strings.Contains(err.Error(), tc.errPart) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.errPart)
			}
		})
	}
}

func TestCompilePolicyPresetWithGlob(t *testing.T) {
	// A preset may itself contain a trailing-star glob; it must expand at
	// compile time just like a glob written directly in a binding.
	cfg := validAC()
	cfg.Presets["aliasops"] = []string{"bucket_alias.*"}
	cfg.Teams[0].Bindings[0].Permissions = []string{"preset:aliasops"}
	p, err := CompilePolicy(cfg)
	if err != nil {
		t.Fatalf("CompilePolicy: %v", err)
	}
	b0 := p.Teams[0].Bindings[0].Permissions
	for _, want := range []string{"bucket_alias.add", "bucket_alias.remove"} {
		if _, ok := b0[want]; !ok {
			t.Errorf("preset glob did not expand %q: %v", want, b0)
		}
	}
}

func TestTeamsForClaimsDisabledReturnsNil(t *testing.T) {
	p, err := CompilePolicy(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.TeamsForClaims([]string{"anything"}); got != nil {
		t.Errorf("disabled policy matched %v, want nil", got)
	}
}

func TestTeamsForClaimsDeduplicatesSameTeam(t *testing.T) {
	// One team reachable via two claim values: presenting both must not
	// return the team twice.
	cfg := validAC()
	cfg.Teams[0].ClaimValues = []string{"g-a", "g-b"}
	p, err := CompilePolicy(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.TeamsForClaims([]string{"g-a", "g-b"}); len(got) != 1 {
		t.Fatalf("TeamsForClaims returned %d teams, want 1 (deduped)", len(got))
	}
}

func TestTeamsForClaims(t *testing.T) {
	cfg := validAC()
	// Second team sharing a claim value with the first: union case from the issue.
	cfg.Teams = append(cfg.Teams, config.TeamConfig{
		Name:               "backend-observers",
		ClaimValues:        []string{"garage-team-backend"},
		ClusterPermissions: []string{"cluster.statistics"},
	})
	p, err := CompilePolicy(cfg)
	if err != nil {
		t.Fatalf("CompilePolicy: %v", err)
	}
	got := p.TeamsForClaims([]string{"garage-team-backend"})
	if len(got) != 2 {
		t.Fatalf("TeamsForClaims matched %d teams, want 2 (shared claim value)", len(got))
	}
	if got := p.TeamsForClaims([]string{"unrelated"}); len(got) != 0 {
		t.Fatalf("unrelated claim matched %d teams, want 0", len(got))
	}
}
