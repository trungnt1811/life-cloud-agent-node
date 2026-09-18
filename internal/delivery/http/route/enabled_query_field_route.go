package route

import (
	"github.com/gin-gonic/gin"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/handlers"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/middleware"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// AdminAuthOptions configures HTTP Basic Auth for local admin routes.
// Credentials intentionally reuse SWAGGER_BASIC_AUTH_* for Phase 3.
// Unlike Swagger (which may stay open in non-prod when empty), admin
// routes are never registered without both username and password.
type AdminAuthOptions struct {
	Username string
	Password string
}

// SetupEnabledQueryFieldRoutes registers D5 admin whitelist routes.
func SetupEnabledQueryFieldRoutes(
	r *gin.Engine,
	handler *handlers.EnabledQueryFieldHandler,
	auth AdminAuthOptions,
	log logger.Logger,
) {
	if r == nil || handler == nil {
		return
	}

	if auth.Username == "" || auth.Password == "" {
		if log != nil {
			log.Warn("Admin query-fields routes not registered; both SWAGGER_BASIC_AUTH_USER and SWAGGER_BASIC_AUTH_PASS are required")
		}
		return
	}

	admin := r.Group("/admin/query-fields")
	admin.Use(middleware.HTTPBasicAuth(auth.Username, auth.Password, "admin"))
	admin.GET("", handler.ListQueryFields)
	admin.PUT("/:field_code", handler.UpdateQueryField)
}
