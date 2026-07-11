package authz

import (
	"Noooste/garage-ui/internal/auth"
)

// TeamResolver maps an authenticated identity to an authorization Subject.
// It is an interface so non-OIDC identity sources (deferred in v1) can plug
// in later without touching middleware or handlers.
type TeamResolver interface {
	Resolve(userInfo *auth.UserInfo) Subject
}

type configTeamResolver struct {
	policy     *Policy
	adminRoles []string
}

// NewTeamResolver builds the v1 resolver: OIDC identities resolve through the
// compiled policy; admin/token logins resolve to the synthetic admin subject
// (non-OIDC team mapping is deferred).
func NewTeamResolver(policy *Policy, adminRoles []string) TeamResolver {
	return &configTeamResolver{policy: policy, adminRoles: adminRoles}
}

func (r *configTeamResolver) Resolve(userInfo *auth.UserInfo) Subject {
	id := userInfo.Email
	if id == "" {
		id = userInfo.Username
	}

	// Trust only signed claims, never the transport channel: the auth method
	// is a JWT claim stamped at login. Legacy sessions ("") resolve like OIDC
	// so a replayed cookie can never escalate.
	if userInfo.AuthMethod == "admin" || userInfo.AuthMethod == "token" {
		return AdminSubject(id)
	}

	for _, role := range userInfo.Roles {
		for _, adminRole := range r.adminRoles {
			if role == adminRole {
				return AdminSubject(id)
			}
		}
	}

	subj := Subject{ID: id, ClusterPerms: PermSet{}}
	for _, team := range r.policy.TeamsForClaims(userInfo.Teams) {
		subj.Bindings = append(subj.Bindings, team.Bindings...)
		for perm := range team.ClusterPerms {
			subj.ClusterPerms[perm] = struct{}{}
		}
	}
	return subj
}
