package route

import (
	"github.com/gin-gonic/gin"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/handlers"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/middleware"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// SetupStatusRoutes registers GET /admin/status (decision 0003), gated by the
// same admin credentials and partial-config fail-fast rule as
// SetupEnabledQueryFieldRoutes - see AdminAuthOptions's doc comment.
func SetupStatusRoutes(r *gin.Engine, handler *handlers.StatusHandler, auth AdminAuthOptions, log logger.Logger) {
	if r == nil || handler == nil {
		return
	}

	hasUser := auth.Username != ""
	hasPass := auth.Password != ""

	switch {
	case hasUser && hasPass:
		admin := r.Group("/admin/status")
		admin.Use(middleware.HTTPBasicAuth(auth.Username, auth.Password, "admin"))
		admin.GET("", handler.GetStatus)
	case !hasUser && !hasPass:
		if log != nil {
			log.Warn("Admin status route not registered; ADMIN_BASIC_AUTH_USER and ADMIN_BASIC_AUTH_PASS are not set")
		}
	default:
		const msg = "admin status misconfigured: exactly one of ADMIN_BASIC_AUTH_USER/ADMIN_BASIC_AUTH_PASS is set; set both or neither"
		if log != nil {
			log.Panic(msg)
		}
		panic(msg)
	}
}
