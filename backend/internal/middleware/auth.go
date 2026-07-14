package middleware

import (
	"net/url"
	"strings"

	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/config"
	"Noooste/garage-ui/internal/models"
	logpkg "Noooste/garage-ui/pkg/logger"

	"github.com/gofiber/fiber/v3"
)

// AuthMiddleware supports admin and OIDC authentication. On success it
// enriches the per-request logger (stored in c.Context() by the Logging
// middleware) with user_id and auth_method so downstream service log lines
// carry user identity. On failure it emits a warn log with the auth_method
// tried and a reason — never the token value.
func AuthMiddleware(cfg *config.AuthConfig, authService *auth.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		// If no auth is enabled, allow all requests.
		if !cfg.Admin.Enabled && !cfg.OIDC.Enabled && !cfg.Token.Enabled {
			return c.Next()
		}

		// Preview tokens authenticate object GETs from media elements, which
		// cannot send an Authorization header. The token was minted behind an
		// object.read check, names one exact object, and expires on its own.
		//
		// Bucket and key come from the raw request path (previewRouteParts),
		// not from c.Params("bucket")/c.Params("*"). routes.go registers this
		// AuthMiddleware twice for the object GET route: once cascaded from
		// the /api/v1 group's Use middleware (which runs before Fiber has
		// matched the specific wildcard route, so its params are not bound
		// yet) and once more directly on the route itself (params bound). The
		// group-cascaded pass would otherwise dead-end here on empty params
		// and fall through to a 401 before the bound-params pass ever runs.
		// Parsing the static path shape gives the same, correct answer in
		// both positions without weakening the contract: it only ever
		// resolves the exact bucket and key named in the URL.
		if pt := c.Query("pt"); pt != "" && c.Method() == fiber.MethodGet {
			bucket, _ := previewRouteParts(c)
			key := previewObjectKey(c)
			if bucket != "" && key != "" && authService.ValidatePreviewToken(pt, bucket, key) == nil {
				c.Locals(auth.PreviewTokenLocalsKey, &auth.PreviewClaims{Bucket: bucket, Key: key})
				enrichRequestLogger(c, "preview-token", "preview_token")
				return c.Next()
			}
		}

		authHeader := c.Get("Authorization")

		// Try bearer token auth (works for admin, token, or any JWT session)
		if (cfg.Admin.Enabled || cfg.Token.Enabled) && authHeader != "" {
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				token := authHeader[7:]
				userInfo, err := authService.ValidateSessionToken(token)
				if err == nil {
					c.Locals("userInfo", userInfo)
					c.Locals("username", userInfo.Username)
					if userInfo.Email != "" {
						c.Locals("email", userInfo.Email)
					}
					enrichRequestLogger(c, userInfo.Username, "admin")
					return c.Next()
				}
			}
		}

		// Try OIDC auth if enabled.
		if cfg.OIDC.Enabled {
			sessionCookie := c.Cookies(cfg.OIDC.CookieName)
			if sessionCookie != "" {
				userInfo, err := authService.ValidateSessionToken(sessionCookie)
				if err == nil {
					c.Locals("userInfo", userInfo)
					c.Locals("username", userInfo.Username)
					c.Locals("email", userInfo.Email)
					enrichRequestLogger(c, userInfo.Username, "oidc")
					return c.Next()
				}
			}
		}

		// Auth failed — log at warn without exposing token material.
		logpkg.FromCtx(c.Context()).Warn().
			Str("auth_method", authMethodsEnabled(cfg)).
			Str("reason", "no_valid_credentials").
			Msg("authentication_failed")

		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "Authentication required"),
		)
	}
}

// enrichRequestLogger rebinds the per-request logger in c.Context() with
// user_id and auth_method. Subsequent logpkg.FromCtx(c.Context()) calls
// return the enriched logger.
func enrichRequestLogger(c fiber.Ctx, userID, authMethod string) {
	l := logpkg.FromCtx(c.Context()).With().
		Str("user_id", userID).
		Str("auth_method", authMethod).
		Logger()
	c.Locals(LoggerLocalsKey, l)
	c.SetContext(logpkg.IntoCtx(c.Context(), l))
}

func authMethodsEnabled(cfg *config.AuthConfig) string {
	methods := []string{}
	if cfg.Admin.Enabled {
		methods = append(methods, "admin")
	}
	if cfg.OIDC.Enabled {
		methods = append(methods, "oidc")
	}
	if cfg.Token.Enabled {
		methods = append(methods, "token")
	}
	if len(methods) == 0 {
		return "none"
	}
	return strings.Join(methods, "+")
}

// previewRouteParts extracts the bucket and raw (still percent-encoded)
// object key from a request path shaped like
// "/api/v1/buckets/<bucket>/objects/<key>", the only shape the object GET
// route matches. It parses c.Path() directly rather than reading Fiber's
// bound :bucket/* route params, because those params are unset whenever this
// runs ahead of the specific route match (see the AuthMiddleware comment
// above). c.Path() reflects the incoming request path from the start of
// request handling, independent of routing state, so this returns the same
// answer no matter where in the chain it runs. Any path not matching the
// shape returns ("", "").
func previewRouteParts(c fiber.Ctx) (bucket, rawKey string) {
	const prefix = "/api/v1/buckets/"
	path := c.Path()
	if !strings.HasPrefix(path, prefix) {
		return "", ""
	}
	rest := path[len(prefix):]
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return "", ""
	}
	bucket = rest[:slash]
	rest = rest[slash+1:]
	const objectsPrefix = "objects/"
	if !strings.HasPrefix(rest, objectsPrefix) {
		return "", ""
	}
	return bucket, rest[len(objectsPrefix):]
}

// previewObjectKey decodes the wildcard object key the same way the routes
// layer does. Requests targeting the JSON subroutes return "" because a
// preview token only ever grants the plain byte download.
func previewObjectKey(c fiber.Ctx) string {
	_, raw := previewRouteParts(c)
	// A raw trailing slash is the one case where c.Path() (used here) and the
	// served c.Params("*") diverge: Fiber trims the trailing slash from the
	// bound wildcard, so validating against the un-trimmed key could authorize
	// a token for "dir/" to serve "dir", or let "x/metadata/" reach the
	// /metadata subroute. The SPA always percent-encodes keys as one segment,
	// so a legitimate key ending in "/" arrives as "...%2F", never a raw
	// trailing slash. Refuse the raw-trailing-slash form so the validated key
	// can never diverge from the served key.
	if raw == "" || strings.HasSuffix(raw, "/") {
		return ""
	}
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		decoded = raw
	}
	for _, suffix := range []string{"/metadata", "/presign", "/preview-url"} {
		if strings.HasSuffix(decoded, suffix) {
			return ""
		}
	}
	return decoded
}
