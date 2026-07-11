package authz

import (
	"testing"

	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/config"
)

func resolverFixture(t *testing.T) TeamResolver {
	t.Helper()
	policy, err := CompilePolicy(&config.AccessControlConfig{
		Teams: []config.TeamConfig{
			{
				Name:        "backend",
				ClaimValues: []string{"g-backend"},
				Bindings: []config.BindingConfig{
					{BucketPrefixes: []string{"backend-"}, Permissions: []string{"bucket.list", "bucket.read"}},
				},
				ClusterPermissions: []string{"cluster.status"},
			},
			{
				Name:               "observers",
				ClaimValues:        []string{"g-backend", "g-obs"},
				ClusterPermissions: []string{"cluster.statistics"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewTeamResolver(policy, []string{"garage-admin"})
}

func TestResolveOIDCTeamUser(t *testing.T) {
	r := resolverFixture(t)
	s := r.Resolve(&auth.UserInfo{Email: "a@x.com", AuthMethod: "oidc", Teams: []string{"g-backend"}})
	if s.IsAdmin {
		t.Error("team user must not be admin")
	}
	if s.ID != "a@x.com" {
		t.Errorf("ID = %q", s.ID)
	}
	if len(s.Bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(s.Bindings))
	}
	// Union across the two teams matched by g-backend.
	if _, ok := s.ClusterPerms["cluster.status"]; !ok {
		t.Error("missing cluster.status")
	}
	if _, ok := s.ClusterPerms["cluster.statistics"]; !ok {
		t.Error("missing cluster.statistics from second team sharing the claim")
	}
}

func TestResolveOIDCAdminRole(t *testing.T) {
	r := resolverFixture(t)
	s := r.Resolve(&auth.UserInfo{Username: "root", AuthMethod: "oidc", Roles: []string{"garage-admin"}})
	if !s.IsAdmin {
		t.Error("admin-role user must resolve to admin subject")
	}
}

func TestResolveNonOIDCIsAdmin(t *testing.T) {
	r := resolverFixture(t)
	for _, method := range []string{"admin", "token"} {
		s := r.Resolve(&auth.UserInfo{Username: "op", AuthMethod: method})
		if !s.IsAdmin {
			t.Errorf("method %q must resolve to admin (deferred: non-OIDC team mapping)", method)
		}
	}
}

func TestResolveLegacySessionFailsClosed(t *testing.T) {
	// Pre-upgrade JWTs have no auth_method claim. Resolve them like OIDC:
	// roles can still grant admin, but there is no channel-based trust.
	r := resolverFixture(t)
	s := r.Resolve(&auth.UserInfo{Username: "old", AuthMethod: ""})
	if s.IsAdmin {
		t.Error("legacy session without admin role must not be admin")
	}
	if len(s.Bindings) != 0 || len(s.ClusterPerms) != 0 {
		t.Error("legacy session with no teams must have zero permissions")
	}
}

func TestResolveZeroTeamUser(t *testing.T) {
	r := resolverFixture(t)
	s := r.Resolve(&auth.UserInfo{Email: "b@x.com", AuthMethod: "oidc", Teams: []string{"unmatched"}})
	if s.IsAdmin || len(s.Bindings) != 0 || len(s.ClusterPerms) != 0 {
		t.Errorf("zero-team subject must have nothing: %+v", s)
	}
}
