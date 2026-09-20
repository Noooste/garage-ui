package middleware

import (
	"strings"
	"time"

	logpkg "Noooste/garage-ui/pkg/logger"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
)

// LoggerLocalsKey is the fiber.Ctx.Locals key carrying the per-request logger.
const LoggerLocalsKey = "logger"

// Logging returns middleware that (1) builds a per-request zerolog.Logger
// bound with request_id/method/path/remote_ip/user_agent, (2) injects it into
// c.Context() so service layers can retrieve it via logger.FromCtx, and
// (3) emits a single access-log line after the handler runs.
//
// The base logger is the one to derive from — typically the global zerolog
// logger configured at startup. Tests pass a buffer-backed logger here.
//
// Access-log line fields: request_id, method, path, remote_ip, user_agent,
// status, duration_ms, bytes_out. Skipped for /health and OPTIONS.
// Level is chosen from status: >=500 error, >=400 warn, else info.
func Logging(base zerolog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		requestID, _ := c.Locals(RequestIDLocalsKey).(string)

		ctx := base.With().
			Str("request_id", requestID).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Str("remote_ip", c.IP()).
			Str("user_agent", c.Get("User-Agent"))

		// With base_path set, StripBasePath rewrites the path before this runs,
		// so c.Path() is not what the client asked for. Record the original too
		// when they differ, otherwise a misrouting proxy is invisible in the
		// logs. The query string is dropped: it can carry tokens.
		if original, _, _ := strings.Cut(c.OriginalURL(), "?"); original != c.Path() {
			ctx = ctx.Str("request_path", original)
		}

		reqLogger := ctx.Logger()

		c.Locals(LoggerLocalsKey, reqLogger)
		c.SetContext(logpkg.IntoCtx(c.Context(), reqLogger))

		start := time.Now()
		err := c.Next()
		duration := time.Since(start)

		if skipAccessLog(c) {
			return err
		}

		status := c.Response().StatusCode()
		bytesOut := len(c.Response().Body())

		evt := eventForStatus(&reqLogger, status)
		evt.
			Int("status", status).
			Float64("duration_ms", float64(duration.Microseconds())/1000.0).
			Int("bytes_out", bytesOut).
			Msg("http_request")

		return err
	}
}

func skipAccessLog(c fiber.Ctx) bool {
	if c.Method() == fiber.MethodOptions {
		return true
	}
	if c.Path() == "/health" {
		return true
	}
	return false
}

func eventForStatus(l *zerolog.Logger, status int) *zerolog.Event {
	switch {
	case status >= 500:
		return l.Error()
	case status >= 400:
		return l.Warn()
	default:
		return l.Info()
	}
}
