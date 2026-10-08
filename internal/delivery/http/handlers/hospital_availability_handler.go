package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/dto"
)

// ReadAvailability reads local pause independently of Control Plane connectivity.
// @Summary Read hospital-local availability
// @Tags hospital-governance
// @Produce json
// @Security BasicAuth
// @Success 200 {object} dto.HospitalAvailabilityDTO
// @Failure 401 {object} dto.ErrorDTOResponse
// @Failure 503 {object} dto.ErrorDTOResponse
// @Router /admin/availability [get]
func (h *HospitalGovernanceHandler) ReadAvailability(c *gin.Context) {
	if !h.available(c) {
		return
	}
	state, err := h.useCase.Read(c.Request.Context())
	if err != nil {
		hospitalGovernanceHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewHospitalAvailabilityDTO(*state))
}

// ChangeAvailability requires the local service actor, revision and immutable key.
// @Summary Change hospital-local pause
// @Tags hospital-governance
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param Idempotency-Key header string true "One key per logical command"
// @Param body body dto.HospitalAvailabilityRequest true "Versioned pause command"
// @Success 200 {object} dto.HospitalAvailabilityDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 401 {object} dto.ErrorDTOResponse
// @Failure 409 {object} dto.ErrorDTOResponse
// @Failure 413 {object} dto.ErrorDTOResponse
// @Failure 503 {object} dto.ErrorDTOResponse
// @Router /admin/availability [put]
func (h *HospitalGovernanceHandler) ChangeAvailability(c *gin.Context) {
	if !h.available(c) {
		return
	}
	var request dto.HospitalAvailabilityRequest
	if !bindHospitalGovernance(c, &request) {
		return
	}
	if request.Paused == nil || request.ExpectedRevision == nil {
		invalidHospitalRequest(c)
		return
	}
	state, err := h.useCase.ChangeAvailability(c.Request.Context(), hospitalActor(c), c.GetHeader("Idempotency-Key"), *request.Paused, *request.ExpectedRevision)
	if err != nil {
		hospitalGovernanceHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewHospitalAvailabilityDTO(*state))
}
