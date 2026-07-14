package authz

import (
	"sort"
	"strings"
)

// Subject is who is asking: resolved once per request by the TeamResolver.
type Subject struct {
	ID           string
	IsAdmin      bool
	Bindings     []Binding
	ClusterPerms PermSet
}

// Resource is what is being acted on. Empty Bucket means the action is
// global or unscoped (e.g. the ListBuckets endpoint itself).
type Resource struct {
	Bucket string
}

// Decision is the outcome of an authorization check.
type Decision struct {
	Allow  bool
	Reason string
}

// Authorizer decides whether a subject may perform an action on a resource.
// It is an interface so enforcement can later move to an external PDP or
// Garage-side scoped tokens without touching handlers.
type Authorizer interface {
	Decide(subj Subject, action string, res Resource) Decision
}

type policyAuthorizer struct{}

// NewAuthorizer returns the built-in policy evaluator.
func NewAuthorizer() Authorizer { return policyAuthorizer{} }

func (policyAuthorizer) Decide(subj Subject, action string, res Resource) Decision {
	return Decide(subj, action, res)
}

// Decide is the pure decision function. The synthetic admin subject flows
// through the same logic as any team, with no IsAdmin shortcut.
func Decide(subj Subject, action string, res Resource) Decision {
	spec, ok := Vocabulary[action]
	if !ok {
		return Decision{Allow: false, Reason: "unknown_permission"}
	}

	if spec.Scope == ScopeGlobal {
		if _, ok := subj.ClusterPerms[action]; ok {
			return Decision{Allow: true, Reason: "cluster_permission"}
		}
		return Decision{Allow: false, Reason: "no_cluster_permission"}
	}

	// Prefix-scoped.
	for _, b := range subj.Bindings {
		if _, ok := b.Permissions[action]; !ok {
			continue
		}
		// Unscoped call (list endpoint): any binding holding the permission
		// suffices; per-bucket filtering happens on the response.
		if res.Bucket == "" {
			return Decision{Allow: true, Reason: "any_binding"}
		}
		if prefixesMatch(b.BucketPrefixes, res.Bucket) {
			return Decision{Allow: true, Reason: "binding_match"}
		}
	}
	return Decision{Allow: false, Reason: "no_matching_binding"}
}

func prefixesMatch(prefixes []string, bucket string) bool {
	for _, p := range prefixes {
		if p == "*" || strings.HasPrefix(bucket, p) {
			return true
		}
	}
	return false
}

// AdminSubject builds the synthetic admin team: one wildcard binding holding
// every prefix-scoped permission (admin-only included) plus every global
// permission. Same code path as any team.
func AdminSubject(id string) Subject {
	prefixPerms := PermSet{}
	clusterPerms := PermSet{}
	for name, spec := range Vocabulary {
		if spec.Scope == ScopePrefix {
			prefixPerms[name] = struct{}{}
		} else {
			clusterPerms[name] = struct{}{}
		}
	}
	return Subject{
		ID:           id,
		IsAdmin:      true,
		Bindings:     []Binding{{BucketPrefixes: []string{"*"}, Permissions: prefixPerms}},
		ClusterPerms: clusterPerms,
	}
}

// EffectivePermissions returns the sorted union of prefix-scoped permissions
// the subject holds on the named bucket. This is the value served in API responses
// so the frontend never does prefix matching. Returns nil when nothing
// matches.
func EffectivePermissions(subj Subject, bucket string) []string {
	set := PermSet{}
	for _, b := range subj.Bindings {
		if !prefixesMatch(b.BucketPrefixes, bucket) {
			continue
		}
		for perm := range b.Permissions {
			set[perm] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for perm := range set {
		out = append(out, perm)
	}
	sort.Strings(out)
	return out
}
