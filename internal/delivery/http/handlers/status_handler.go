package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/client"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/dto"
)

// StatusHandler serves node-local operational status (decision 0003).
type StatusHandler struct {
	nodeID       string
	agentVersion string
	advisories   *client.AdvisoryStore
}

// NewStatusHandler creates a StatusHandler. A nil advisories store is
// treated as "no advisory received yet".
func NewStatusHandler(nodeID, agentVersion string, advisories *client.AdvisoryStore) *StatusHandler {
	return &StatusHandler{nodeID: nodeID, agentVersion: agentVersion, advisories: advisories}
}

// GetStatus returns this node's identity and the latest update advisory, if any.
// @Summary Get node status
// @Description Node identity and the latest UpdateAdvisory received from the
// @Description control center, if any. The node only logs and exposes an
// @Description advisory; it never auto-applies an update (decision 0003).
// @Tags admin
// @Produce json
// @Security BasicAuth
// @Success 200 {object} dto.StatusDTO
// @Failure 401 {object} dto.ErrorDTOResponse
// @Router /admin/status [get]
func (h *StatusHandler) GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, dto.NewStatusDTO(h.nodeID, h.agentVersion, h.advisories))
}
