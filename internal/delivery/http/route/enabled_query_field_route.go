package route

import (
	"github.com/gin-gonic/gin"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/handlers"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/middleware"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// AdminAuthOptions configures HTTP Basic Auth for local admin routes.
// Credentials intentionally reuse SWAGGER_BASIC_AUTH_* for Phase 3; see the
// active plan decision if a dedicated admin credential set is needed later.
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

	admin := r.Group("/admin/query-fields")
	switch {
	case auth.Username == "" && auth.Password == "":
		if log != nil {
			log.Warn("Admin query-fields routes registered without basic auth credentials")
		}
	case auth.Username != "" && auth.Password != "":
		admin.Use(middleware.HTTPBasicAuth(auth.Username, auth.Password, "admin"))
	default:
		if log != nil {
			log.Warn("Admin query-fields basic auth is misconfigured; routes will not be registered")
		}
		return
	}

	admin.GET("", handler.ListQueryFields)
	admin.PUT("/:field_code", handler.UpdateQueryField)
}
