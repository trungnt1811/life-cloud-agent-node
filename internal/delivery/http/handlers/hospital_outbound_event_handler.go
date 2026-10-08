package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/dto"
	httpresponse "github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/response"
)

// ListOutboundEvents never exposes local running-count checkpoints.
// @Summary List hospital-local protected outbound events
// @Tags hospital-governance
// @Produce json
// @Security BasicAuth
// @Param page query int false "Page, minimum 1" default(1)
// @Param page_size query int false "Page size, maximum 100" default(20)
// @Success 200 {object} dto.HospitalOutboundEventListDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 401 {object} dto.ErrorDTOResponse
// @Failure 503 {object} dto.ErrorDTOResponse
// @Router /admin/outbound-events [get]
func (h *HospitalGovernanceHandler) ListOutboundEvents(c *gin.Context) {
	if h.jobs == nil {
		httpresponse.Error(c, http.StatusServiceUnavailable, "GOVERNANCE_STORE_UNAVAILABLE", "hospital governance store is unavailable", nil)
		return
	}
	page, size := c.GetInt(constants.DefaultPageText), c.GetInt(constants.AltPageSizeText)
	items, total, err := h.jobs.ListEvents(c.Request.Context(), page, size)
	if err != nil {
		hospitalGovernanceHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewHospitalOutboundEventListDTO(items, total, page, size))
}
