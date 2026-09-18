package route

import (
	"github.com/gin-gonic/gin"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/handlers"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/middleware"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// AdminAuthOptions configures HTTP Basic Auth for local admin routes.
// Credentials are ADMIN_BASIC_AUTH_USER/PASS, deliberately separate from
// SWAGGER_BASIC_AUTH_*: this gates a state-mutating endpoint (which lab
// fields a hospital exposes to federated queries), not read-only API docs,
// so rotating one credential must never also rotate the other.
type AdminAuthOptions struct {
	Username string
	Password string
}

// SetupEnabledQueryFieldRoutes registers D5 admin whitelist routes.
//
// Both credentials empty is treated as "admin routes intentionally not
// configured" (e.g. local dev) and only logs a warning. Exactly one set is
// always an operator error (typo, partial env rollout) - unlike Swagger,
// this endpoint is required functionality per the implementation plan, so
// a partially-configured admin surface fails loudly at startup instead of
// silently 404ing forever.
func SetupEnabledQueryFieldRoutes(
	r *gin.Engine,
	handler *handlers.EnabledQueryFieldHandler,
	auth AdminAuthOptions,
	log logger.Logger,
) {
	if r == nil || handler == nil {
		return
	}

	hasUser := auth.Username != ""
	hasPass := auth.Password != ""

	switch {
	case hasUser && hasPass:
		admin := r.Group("/admin/query-fields")
		admin.Use(middleware.HTTPBasicAuth(auth.Username, auth.Password, "admin"))
		admin.GET("", handler.ListQueryFields)
		admin.PUT("/:field_code", handler.UpdateQueryField)
	case !hasUser && !hasPass:
		if log != nil {
			log.Warn("Admin query-fields routes not registered; ADMIN_BASIC_AUTH_USER and ADMIN_BASIC_AUTH_PASS are not set")
		}
	default:
		const msg = "admin query-fields misconfigured: exactly one of ADMIN_BASIC_AUTH_USER/ADMIN_BASIC_AUTH_PASS is set; set both or neither"
		if log != nil {
			log.Panic(msg)
		}
		panic(msg)
	}
}
