package authz

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"Noooste/garage-ui/internal/auth"
	"Noooste/garage-ui/internal/models"
	logpkg "Noooste/garage-ui/pkg/logger"

	"github.com/gofiber/fiber/v3"
)

// SubjectLocalsKey is the fiber.Ctx.Locals key carrying the resolved Subject.
const SubjectLocalsKey = "authzSubject"

// Middleware wires the policy into Fiber. When the policy is disabled
// (access_control absent) every handler is a passthrough no-op.
type Middleware struct {
	enabled    bool
	resolver   TeamResolver
	authorizer Authorizer
}

func NewMiddleware(policy *Policy, resolver TeamResolver, authorizer Authorizer) *Middleware {
	return &Middleware{enabled: policy.Enabled, resolver: resolver, authorizer: authorizer}
}

// Enabled reports whether access control is active.
func (m *Middleware) Enabled() bool { return m.enabled }

// ResolveSubject computes the request's Subject once, right after
// authentication. Handlers and the capabilities endpoint read the same
// struct, so enforcement and UI can never disagree.
func (m *Middleware) ResolveSubject() fiber.Handler {
	return func(c fiber.Ctx) error {
		if !m.enabled {
			return c.Next()
		}
		userInfo, ok := c.Locals("userInfo").(*auth.UserInfo)
		if !ok || userInfo == nil {
			return c.Next() // no identity (auth disabled); Require will deny
		}
		c.Locals(SubjectLocalsKey, m.resolver.Resolve(userInfo))
		return c.Next()
	}
}

// SubjectFrom returns the Subject resolved for this request, if any.
func SubjectFrom(c fiber.Ctx) (Subject, bool) {
	subj, ok := c.Locals(SubjectLocalsKey).(Subject)
	return subj, ok
}

// ScopeResolver extracts the target Resource from the request.
type ScopeResolver func(c fiber.Ctx) Resource

// ScopeNone is for global permissions and unscoped list endpoints.
func ScopeNone(fiber.Ctx) Resource { return Resource{} }

// BucketFromParam reads the bucket name from a URL parameter.
func BucketFromParam(param string) ScopeResolver {
	return func(c fiber.Ctx) Resource {
		return Resource{Bucket: c.Params(param)}
	}
}

// BucketFromBody reads the bucket name from a JSON body {"name": "..."}.
// Fiber buffers the body, so the handler can bind it again afterwards.
func BucketFromBody() ScopeResolver {
	return func(c fiber.Ctx) Resource {
		var req struct {
			Name string `json:"name"`
		}
		if err := c.Bind().JSON(&req); err != nil {
			return Resource{}
		}
		return Resource{Bucket: req.Name}
	}
}

// Require gates a route on the caller holding ALL of perms for the resolved
// resource. One structured decision log line is emitted per check: denies at
// warn, allows at debug.
//
// The closure's function name is the marker VerifyRouteCoverage looks for.
// Do not wrap it in another anonymous function.
func (m *Middleware) Require(scope ScopeResolver, perms ...string) fiber.Handler {
	if len(perms) == 0 {
		// A no-perms Require would silently no-op (allow everything) yet still
		// satisfy VerifyRouteCoverage, defeating the fail-closed guarantee.
		panic("authz.Require: at least one permission required")
	}
	for _, p := range perms {
		if !IsValidPermission(p) {
			panic(fmt.Sprintf("authz.Require: unknown permission %q", p)) // programmer error, fail at wiring time
		}
	}
	return func(c fiber.Ctx) error {
		if !m.enabled {
			return c.Next()
		}
		subj, ok := SubjectFrom(c)
		if !ok {
			logDecision(c, "", strings.Join(perms, ","), "", false, "no_subject")
			return forbidden(c, perms[0])
		}
		res := scope(c)
		for _, perm := range perms {
			d := m.authorizer.Decide(subj, perm, res)
			logDecision(c, subj.ID, perm, res.Bucket, d.Allow, d.Reason)
			if !d.Allow {
				return forbidden(c, perm)
			}
		}
		return c.Next()
	}
}

func forbidden(c fiber.Ctx, perm string) error {
	return c.Status(fiber.StatusForbidden).JSON(
		models.ErrorResponse(models.ErrCodeForbidden, "Missing permission: "+perm),
	)
}

func logDecision(c fiber.Ctx, subject, action, resource string, allow bool, reason string) {
	l := logpkg.FromCtx(c.Context())
	evt := l.Debug()
	if !allow {
		evt = l.Warn()
	}
	decision := "allow"
	if !allow {
		decision = "deny"
	}
	evt.Str("subject", subject).
		Str("action", action).
		Str("resource", resource).
		Str("decision", decision).
		Str("reason", reason).
		Msg("authz_decision")
}

// coverageExemptPaths are /api/v1 routes that intentionally carry no Require:
// health is unauthenticated, capabilities is the frontend's fail-closed
// source and returns only the caller's own permissions.
var coverageExemptPaths = map[string]struct{}{
	"/api/v1/health":       {},
	"/api/v1/capabilities": {},
}

// VerifyRouteCoverage walks the app's route table and errors if any /api/v1
// route lacks a Require handler. Called at startup (and from tests): a new
// endpoint registered without declaring its permission prevents boot instead
// of silently failing open.
//
// .Use()-registered routes need special handling. Group-level middleware
// (api.Use(handler) on the "/api/v1" group) produces synthetic per-method
// bookkeeping entries at exactly the bare prefix. Those aren't endpoints and
// are exempt. But a use-route at any deeper path (api.Use("/sneaky",
// terminalHandler)) IS a reachable endpoint and gets the same fail-closed
// treatment as normal routes.
func VerifyRouteCoverage(app *fiber.App) error {
	// fiber.Route doesn't export whether a route was .Use()-registered, so
	// classify by diffing GetRoutes() (everything) against GetRoutes(true)
	// (use-routes filtered out): entries unmatched in the filtered multiset
	// are use-registered.
	nonUse := map[string]int{}
	for _, r := range app.GetRoutes(true) {
		nonUse[routeKey(r)]++
	}

	var naked []string
	for _, route := range app.GetRoutes() {
		// Consume the multiset for every route, before any skip, so
		// classification stays consistent across the whole table.
		isUse := true
		if k := routeKey(route); nonUse[k] > 0 {
			nonUse[k]--
			isUse = false
		}
		if !strings.HasPrefix(route.Path, "/api/v1") {
			continue
		}
		if isUse && route.Path == "/api/v1" {
			continue // group-middleware bookkeeping at the bare prefix
		}
		if _, exempt := coverageExemptPaths[route.Path]; exempt {
			continue
		}
		if route.Method == fiber.MethodHead && hasRequireForPath(app, fiber.MethodGet, route.Path) {
			continue // Fiber auto-registers HEAD mirroring GET
		}
		if !routeHasRequire(route.Handlers) {
			naked = append(naked, route.Method+" "+route.Path)
		}
	}
	if len(naked) > 0 {
		return fmt.Errorf("authz: routes without Require permission declaration: %s", strings.Join(naked, ", "))
	}
	return nil
}

// routeKey identifies a route for the use-route diff. Method+Path+handler
// count is robust enough: two routes sharing all three are interchangeable
// for coverage purposes, and the multiset keeps counts honest.
func routeKey(r fiber.Route) string {
	return r.Method + " " + r.Path + " " + strconv.Itoa(len(r.Handlers))
}

func hasRequireForPath(app *fiber.App, method, path string) bool {
	for _, route := range app.GetRoutes(true) {
		if route.Method == method && route.Path == path {
			return routeHasRequire(route.Handlers)
		}
	}
	return false
}

func routeHasRequire(handlers []fiber.Handler) bool {
	for _, h := range handlers {
		fn := runtime.FuncForPC(fiberHandlerPC(h))
		if fn != nil && strings.Contains(fn.Name(), "authz.(*Middleware).Require") {
			return true
		}
	}
	return false
}

func fiberHandlerPC(h fiber.Handler) uintptr {
	return reflect.ValueOf(h).Pointer()
}
