package middleware

import (
	"strings"

	"Noooste/garage-ui/pkg/logger"

	"github.com/gofiber/fiber/v3"
)

// basePathStrippedKey marks a request whose base path has already been
// stripped, so the handler does not run again after RestartRouting.
const basePathStrippedKey = "garageui_base_path_stripped"

// StripBasePath accepts an optional subpath prefix on incoming requests
// (issue #107). Routes stay registered at the root; a request that arrives
// carrying the configured base path is rewritten and re-routed.
//
// Reverse proxies split into two camps: some strip the mount point before
// forwarding (tailscale serve, Traefik's StripPrefix, the usual Kubernetes
// rewrite-target), others pass the full path through (nginx proxy_pass without
// a URI part). Accepting the prefix rather than requiring it means one setting
// covers both, and container probes that hit the unprefixed /health keep
// working untouched.
//
// It must be installed BEFORE the RequestID and access-log middleware:
// RestartRouting replays the whole chain, and only handlers registered after
// this one run exactly once per request (this handler stops the first pass, so
// they never run in it).
//
// An empty basePath yields a pass-through handler.
func StripBasePath(basePath string) fiber.Handler {
	if basePath == "" {
		return func(c fiber.Ctx) error { return c.Next() }
	}

	prefix := basePath + "/"
	return func(c fiber.Ctx) error {
		// RestartRouting replays this handler, and a path that repeats the
		// prefix would otherwise be stripped twice.
		if c.Locals(basePathStrippedKey) != nil {
			return c.Next()
		}

		path := c.Path()
		if path != basePath && !strings.HasPrefix(path, prefix) {
			return c.Next()
		}

		c.Locals(basePathStrippedKey, true)
		stripped := strings.TrimPrefix(path, basePath)
		if stripped == "" {
			stripped = "/"
		}
		logger.Debug().Str("path", path).Str("stripped", stripped).Msg("Stripped base path")
		c.Path(stripped)
		return c.RestartRouting()
	}
}
