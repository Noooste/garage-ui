package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// reservedFirstSegments are the path roots the server itself serves. A base
// path starting with one of them would shadow it: the prefix is stripped
// before routing, so with base_path=/api a request for /api/v1/health arrives
// as /v1/health, and with base_path=/health the container probe on /health
// arrives as /. Fiber routes case-insensitively by default, so the comparison
// is folded. "login" and "assets" belong to the SPA rather than the API, and
// break in the same way.
var reservedFirstSegments = map[string]struct{}{
	"api":     {},
	"assets":  {},
	"auth":    {},
	"docs":    {},
	"health":  {},
	"login":   {},
	"metrics": {},
}

// segmentPattern is deliberately narrower than RFC 3986 allows. The base path
// is interpolated into HTML attributes and into URLs; unreserved characters
// plus percent-encoding cover every realistic mount point, and everything that
// could break out of an attribute is rejected outright. Escaping at the
// injection site still happens - this is the outer of two layers.
var segmentPattern = regexp.MustCompile(`^[A-Za-z0-9._~%+-]+$`)

func reservedSegmentList() string {
	names := make([]string, 0, len(reservedFirstSegments))
	for name := range reservedFirstSegments {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// NormalizeBasePath canonicalises a configured subpath so every consumer
// (route registration, OIDC redirect URIs, SPA fallback, asset base href) can
// concatenate it without re-checking slashes.
//
// The canonical form is either "" (served from the root, the default) or a
// path that starts with "/" and does not end with one, e.g. "/garage-ui".
// Empty segments are collapsed rather than rejected so that "//garage//ui/"
// from a templated Helm value still works.
func NormalizeBasePath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	if strings.ContainsAny(trimmed, "?#\\") {
		return "", fmt.Errorf("server.base_path %q must not contain %q, %q or %q", raw, "?", "#", "\\")
	}
	if strings.Contains(trimmed, "://") {
		return "", fmt.Errorf("server.base_path %q must be a path, not a URL", raw)
	}

	segments := strings.Split(trimmed, "/")
	kept := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		if segment == "." || segment == ".." {
			return "", fmt.Errorf("server.base_path %q must not contain %q segments", raw, segment)
		}
		if !segmentPattern.MatchString(segment) {
			return "", fmt.Errorf(
				"server.base_path %q: segment %q may only contain letters, digits and %q",
				raw, segment, "._~%+-")
		}
		kept = append(kept, segment)
	}
	if len(kept) == 0 {
		return "", nil
	}

	if _, reserved := reservedFirstSegments[strings.ToLower(kept[0])]; reserved {
		return "", fmt.Errorf(
			"server.base_path %q: %q is a route the server already serves, and mounting under it would shadow that route "+
				"(reserved first segments: %s)",
			raw, kept[0], reservedSegmentList())
	}

	return "/" + strings.Join(kept, "/"), nil
}

// NormalizedBasePath returns the canonical base path. Config.Load rejects
// invalid values up front, so callers that receive an already-validated config
// can use this without handling an error; a value that never went through
// Load falls back to the root path.
func (s ServerConfig) NormalizedBasePath() string {
	normalized, err := NormalizeBasePath(s.BasePath)
	if err != nil {
		return ""
	}
	return normalized
}

// PrefixPath prefixes an absolute in-app path with the configured base path.
// PrefixPath("/") returns the base path itself ("" when serving from root) so
// that callers can register or link the app root.
func (s ServerConfig) PrefixPath(path string) string {
	return JoinBasePath(s.NormalizedBasePath(), path)
}

// JoinBasePath concatenates an already-normalized base path with an absolute
// in-app path, keeping exactly one slash between them.
func JoinBasePath(basePath, path string) string {
	if path == "" || path == "/" {
		if basePath == "" {
			return "/"
		}
		return basePath
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return basePath + path
}

// ExternalURL builds an absolute URL for an in-app path from root_url and the
// base path, e.g. https://host + /garage-ui + /auth/oidc/callback.
func (s ServerConfig) ExternalURL(path string) string {
	root := strings.TrimRight(strings.TrimSpace(s.RootURL), "/")
	return root + s.PrefixPath(path)
}
