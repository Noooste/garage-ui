package authz

import (
	"fmt"
	"strings"

	"Noooste/garage-ui/internal/config"
)

// PermSet is a set of concrete permission names.
type PermSet map[string]struct{}

// Binding pairs bucket-name prefixes with the prefix-scoped permissions that
// apply to buckets matching them.
type Binding struct {
	BucketPrefixes []string
	Permissions    PermSet
}

// TeamPolicy is a compiled team: presets resolved, globs expanded, validated.
type TeamPolicy struct {
	Name         string
	ClaimValues  []string
	Bindings     []Binding
	ClusterPerms PermSet
}

// Policy is the compiled access-control policy. Enabled=false (access_control
// absent) means "behave exactly as before this feature existed".
type Policy struct {
	Enabled bool
	Teams   []TeamPolicy
	byClaim map[string][]int // claim value -> indexes into Teams
}

const presetPrefix = "preset:"

// CompilePolicy validates and compiles the access_control config section.
// Any error here must abort startup; a half-valid policy is worse than none.
func CompilePolicy(cfg *config.AccessControlConfig) (*Policy, error) {
	if cfg == nil {
		return &Policy{Enabled: false}, nil
	}

	resolvedPresets := make(map[string]PermSet, len(cfg.Presets))
	for name := range cfg.Presets {
		perms, err := resolvePreset(cfg.Presets, name, nil)
		if err != nil {
			return nil, err
		}
		resolvedPresets[name] = perms
	}

	p := &Policy{Enabled: true, byClaim: map[string][]int{}}
	seenNames := map[string]struct{}{}
	for ti, team := range cfg.Teams {
		if team.Name == "" {
			return nil, fmt.Errorf("access_control: team %d has no name", ti)
		}
		if _, dup := seenNames[team.Name]; dup {
			return nil, fmt.Errorf("access_control: duplicate team name %q", team.Name)
		}
		seenNames[team.Name] = struct{}{}
		if len(team.ClaimValues) == 0 {
			return nil, fmt.Errorf("access_control: team %q has empty claim_values", team.Name)
		}
		if len(team.Bindings) == 0 && len(team.ClusterPermissions) == 0 {
			return nil, fmt.Errorf("access_control: team %q must have at least one binding or cluster permission", team.Name)
		}

		tp := TeamPolicy{Name: team.Name, ClaimValues: team.ClaimValues, ClusterPerms: PermSet{}}

		for bi, b := range team.Bindings {
			if len(b.BucketPrefixes) == 0 {
				return nil, fmt.Errorf("access_control: team %q binding %d has no bucket_prefixes", team.Name, bi)
			}
			perms, err := resolvePermList(b.Permissions, resolvedPresets, team.Name)
			if err != nil {
				return nil, err
			}
			if len(perms) == 0 {
				return nil, fmt.Errorf("access_control: team %q binding %d has no permissions", team.Name, bi)
			}
			for perm := range perms {
				if Vocabulary[perm].Scope != ScopePrefix {
					return nil, fmt.Errorf("access_control: team %q binding %d: %q is a global permission, put it under cluster_permissions", team.Name, bi, perm)
				}
			}
			tp.Bindings = append(tp.Bindings, Binding{BucketPrefixes: b.BucketPrefixes, Permissions: perms})
		}

		clusterPerms, err := resolvePermList(team.ClusterPermissions, resolvedPresets, team.Name)
		if err != nil {
			return nil, err
		}
		for perm := range clusterPerms {
			if Vocabulary[perm].Scope != ScopeGlobal {
				return nil, fmt.Errorf("access_control: team %q cluster_permissions: %q is prefix-scoped, put it in a binding", team.Name, perm)
			}
		}
		tp.ClusterPerms = clusterPerms

		p.Teams = append(p.Teams, tp)
		idx := len(p.Teams) - 1
		for _, cv := range team.ClaimValues {
			p.byClaim[cv] = append(p.byClaim[cv], idx)
		}
	}
	return p, nil
}

// resolvePermList turns a raw permission list (concrete names, preset refs,
// globs) into a validated PermSet.
func resolvePermList(raw []string, presets map[string]PermSet, teamName string) (PermSet, error) {
	out := PermSet{}
	for _, entry := range raw {
		switch {
		case strings.HasPrefix(entry, presetPrefix):
			name := strings.TrimPrefix(entry, presetPrefix)
			perms, ok := presets[name]
			if !ok {
				return nil, fmt.Errorf("access_control: team %q references unknown preset %q", teamName, name)
			}
			for perm := range perms {
				out[perm] = struct{}{}
			}
		case strings.HasSuffix(entry, "*"):
			expanded := ExpandGlob(entry)
			if expanded == nil {
				return nil, fmt.Errorf("access_control: team %q: glob %q matches no permission (unknown permission pattern)", teamName, entry)
			}
			for _, perm := range expanded {
				out[perm] = struct{}{}
			}
		default:
			spec, ok := Vocabulary[entry]
			if !ok {
				return nil, fmt.Errorf("access_control: team %q: unknown permission %q", teamName, entry)
			}
			if spec.AdminOnly {
				return nil, fmt.Errorf("access_control: team %q: %q is admin-only in v1 and cannot be granted to a team", teamName, entry)
			}
			out[entry] = struct{}{}
		}
	}
	return out, nil
}

// resolvePreset resolves one preset, following preset:… references with cycle
// detection. path carries the current resolution chain.
func resolvePreset(presets map[string][]string, name string, path []string) (PermSet, error) {
	for _, seen := range path {
		if seen == name {
			return nil, fmt.Errorf("access_control: preset cycle detected: %s -> %s", strings.Join(path, " -> "), name)
		}
	}
	entries, ok := presets[name]
	if !ok {
		return nil, fmt.Errorf("access_control: unknown preset %q", name)
	}
	out := PermSet{}
	for _, entry := range entries {
		switch {
		case strings.HasPrefix(entry, presetPrefix):
			sub, err := resolvePreset(presets, strings.TrimPrefix(entry, presetPrefix), append(path, name))
			if err != nil {
				return nil, err
			}
			for perm := range sub {
				out[perm] = struct{}{}
			}
		case strings.HasSuffix(entry, "*"):
			expanded := ExpandGlob(entry)
			if expanded == nil {
				return nil, fmt.Errorf("access_control: preset %q: glob %q matches no permission", name, entry)
			}
			for _, perm := range expanded {
				out[perm] = struct{}{}
			}
		default:
			spec, ok := Vocabulary[entry]
			if !ok {
				return nil, fmt.Errorf("access_control: preset %q: unknown permission %q", name, entry)
			}
			if spec.AdminOnly {
				return nil, fmt.Errorf("access_control: preset %q: %q is admin-only in v1 and cannot be granted to a team", name, entry)
			}
			out[entry] = struct{}{}
		}
	}
	return out, nil
}

// TeamsForClaims returns every team whose claim_values intersect claims.
// Multiple teams sharing a claim value are all returned (union semantics).
func (p *Policy) TeamsForClaims(claims []string) []*TeamPolicy {
	if !p.Enabled {
		return nil
	}
	seen := map[int]struct{}{}
	var out []*TeamPolicy
	for _, c := range claims {
		for _, idx := range p.byClaim[c] {
			if _, dup := seen[idx]; dup {
				continue
			}
			seen[idx] = struct{}{}
			out = append(out, &p.Teams[idx])
		}
	}
	return out
}
