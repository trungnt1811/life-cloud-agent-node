package route

import (
	"github.com/gin-gonic/gin"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/handlers"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/middleware"
)

func SetupHospitalGovernanceRoutes(r *gin.Engine, h *handlers.HospitalGovernanceHandler, auth AdminAuthOptions) {
	if r == nil || h == nil || auth.Username == "" || auth.Password == "" {
		return
	}
	admin := r.Group("/admin")
	admin.Use(middleware.HTTPBasicAuth(auth.Username, auth.Password, "hospital-admin"))
	admin.GET("/policy", h.ReadPolicy)
	admin.PUT("/policy", h.UpdatePolicy)
	admin.GET("/availability", h.ReadAvailability)
	admin.PUT("/availability", h.ChangeAvailability)
	admin.GET("/outbound-events", h.ListOutboundEvents)
	admin.GET("/approvals", h.ListApprovals)
	admin.POST("/approvals/:job_id/approve", h.ApproveResult)
	admin.POST("/approvals/:job_id/decline", h.DeclineResult)
	admin.GET("/study-permits", h.ListPermits)
	admin.PUT("/study-permits/:id/acceptance", h.ChangeAcceptance)
}
