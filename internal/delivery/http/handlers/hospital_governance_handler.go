package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/dto"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/middleware"
	httpresponse "github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/response"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	domainerrors "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
)

type HospitalGovernanceHandler struct {
	useCase interfaces.HospitalGovernanceUseCase
	jobs    interfaces.GovernedHospitalJobUseCase
}

func NewHospitalGovernanceHandler(useCase interfaces.HospitalGovernanceUseCase, jobs interfaces.GovernedHospitalJobUseCase) *HospitalGovernanceHandler {
	return &HospitalGovernanceHandler{useCase: useCase, jobs: jobs}
}

// ReadPolicy returns locally stored policy, not proof of active governed execution.
// @Summary Read hospital-local governance policy
// @Tags hospital-governance
// @Produce json
// @Security BasicAuth
// @Success 200 {object} dto.HospitalPolicyDTO
// @Failure 401 {object} dto.ErrorDTOResponse
// @Failure 503 {object} dto.ErrorDTOResponse
// @Router /admin/policy [get]
func (h *HospitalGovernanceHandler) ReadPolicy(c *gin.Context) {
	if !h.available(c) {
		return
	}
	r, err := h.useCase.Read(c.Request.Context())
	if err != nil {
		hospitalGovernanceHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewHospitalPolicyDTO(*r))
}

// UpdatePolicy changes independent local policy atomically with its audit/command.
// @Summary Update hospital-local governance policy
// @Tags hospital-governance
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param Idempotency-Key header string true "One key per logical command"
// @Param body body dto.HospitalPolicyRequest true "Versioned local policy; AUTO or MANUAL"
// @Success 200 {object} dto.HospitalPolicyDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 401 {object} dto.ErrorDTOResponse
// @Failure 409 {object} dto.ErrorDTOResponse
// @Failure 413 {object} dto.ErrorDTOResponse
// @Failure 503 {object} dto.ErrorDTOResponse
// @Router /admin/policy [put]
func (h *HospitalGovernanceHandler) UpdatePolicy(c *gin.Context) {
	if !h.available(c) {
		return
	}
	var request dto.HospitalPolicyRequest
	if !bindHospitalGovernance(c, &request) {
		return
	}
	if request.ExpectedRevision == nil {
		invalidHospitalRequest(c)
		return
	}
	policy, err := request.Policy()
	if err != nil {
		invalidHospitalRequest(c)
		return
	}
	r, err := h.useCase.UpdatePolicy(c.Request.Context(), hospitalActor(c), c.GetHeader("Idempotency-Key"), policy, *request.ExpectedRevision)
	if err != nil {
		hospitalGovernanceHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewHospitalPolicyDTO(*r))
}

// ListPermits reads authenticated CP scope with any local decision for that version.
// @Summary List hospital-local study permit snapshots
// @Tags hospital-governance
// @Produce json
// @Security BasicAuth
// @Param page query int false "Page, minimum 1" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} dto.HospitalPermitListDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 401 {object} dto.ErrorDTOResponse
// @Failure 503 {object} dto.ErrorDTOResponse
// @Router /admin/study-permits [get]
func (h *HospitalGovernanceHandler) ListPermits(c *gin.Context) {
	if !h.available(c) {
		return
	}
	page, size := c.GetInt(constants.DefaultPageText), c.GetInt(constants.AltPageSizeText)
	if page < 1 || size < 1 {
		invalidHospitalRequest(c)
		return
	}
	r, err := h.useCase.Read(c.Request.Context())
	if err != nil {
		hospitalGovernanceHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewHospitalPermitListDTO(*r, page, size))
}

// ChangeAcceptance cannot create or expand CP scope or impersonate a human approver.
// @Summary Accept or decline a hospital-local permit version
// @Tags hospital-governance
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param id path string true "Known permit ID"
// @Param Idempotency-Key header string true "One key per logical command"
// @Param body body dto.HospitalAcceptanceRequest true "Restricted local scope"
// @Success 200 {object} dto.HospitalAcceptanceDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 401 {object} dto.ErrorDTOResponse
// @Failure 409 {object} dto.ErrorDTOResponse
// @Failure 413 {object} dto.ErrorDTOResponse
// @Failure 503 {object} dto.ErrorDTOResponse
// @Router /admin/study-permits/{id}/acceptance [put]
func (h *HospitalGovernanceHandler) ChangeAcceptance(c *gin.Context) {
	if !h.available(c) {
		return
	}
	var request dto.HospitalAcceptanceRequest
	if !bindHospitalGovernance(c, &request) {
		return
	}
	if request.ExpectedRevision == nil {
		invalidHospitalRequest(c)
		return
	}
	r, err := h.useCase.ChangeAcceptance(c.Request.Context(), hospitalActor(c), c.GetHeader("Idempotency-Key"), request.Change(c.Param("id")))
	if err != nil {
		hospitalGovernanceHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewHospitalAcceptanceDTO(*r, c.Param("id"), request.PermitVersion, request.PermitHash))
}

// ListApprovals exposes only locally protected previews awaiting a hospital decision.
// @Summary List pending hospital-local result approvals
// @Tags hospital-governance
// @Produce json
// @Security BasicAuth
// @Param page query int false "Page, minimum 1" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} dto.HospitalApprovalListDTO
// @Failure 401 {object} dto.ErrorDTOResponse
// @Failure 503 {object} dto.ErrorDTOResponse
// @Router /admin/approvals [get]
func (h *HospitalGovernanceHandler) ListApprovals(c *gin.Context) {
	if !h.jobsAvailable(c) {
		return
	}
	page, size := c.GetInt(constants.DefaultPageText), c.GetInt(constants.AltPageSizeText)
	if page < 1 || size < 1 {
		invalidHospitalRequest(c)
		return
	}
	items, total, err := h.jobs.ListApprovals(c.Request.Context(), page, size)
	if err != nil {
		hospitalGovernanceHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewHospitalApprovalListDTO(items, total, page, size))
}

// ApproveResult applies an optimistic, idempotent hospital-local approval.
// @Summary Approve a protected governed result
// @Tags hospital-governance
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param job_id path string true "Governed job ID"
// @Param Idempotency-Key header string true "UUID stable across retries"
// @Param body body dto.HospitalApprovalRequest true "Expected current approval revision"
// @Success 200 {object} dto.HospitalApprovalDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 401 {object} dto.ErrorDTOResponse
// @Failure 409 {object} dto.ErrorDTOResponse
// @Failure 503 {object} dto.ErrorDTOResponse
// @Router /admin/approvals/{job_id}/approve [post]
func (h *HospitalGovernanceHandler) ApproveResult(c *gin.Context) {
	h.decideApproval(c, true)
}

// DeclineResult permanently declines a held result.
// @Summary Decline a protected governed result
// @Tags hospital-governance
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param job_id path string true "Governed job ID"
// @Param Idempotency-Key header string true "UUID stable across retries"
// @Param body body dto.HospitalApprovalRequest true "Expected current approval revision"
// @Success 200 {object} dto.HospitalApprovalDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 401 {object} dto.ErrorDTOResponse
// @Failure 409 {object} dto.ErrorDTOResponse
// @Failure 503 {object} dto.ErrorDTOResponse
// @Router /admin/approvals/{job_id}/decline [post]
func (h *HospitalGovernanceHandler) DeclineResult(c *gin.Context) {
	h.decideApproval(c, false)
}

func (h *HospitalGovernanceHandler) decideApproval(c *gin.Context, approve bool) {
	if !h.jobsAvailable(c) {
		return
	}
	var request dto.HospitalApprovalRequest
	if !bindHospitalGovernance(c, &request) {
		return
	}
	if request.ExpectedRevision < 1 {
		invalidHospitalRequest(c)
		return
	}
	commandID := c.GetHeader("Idempotency-Key")
	parsedCommandID, parseErr := uuid.Parse(commandID)
	if parseErr != nil || parsedCommandID == uuid.Nil || parsedCommandID.String() != commandID {
		invalidHospitalRequest(c)
		return
	}
	var record *entities.GovernedHospitalJobRecord
	var err error
	if approve {
		record, err = h.jobs.Approve(c.Request.Context(), hospitalActor(c), c.Param("job_id"), request.ExpectedRevision, commandID)
	} else {
		record, err = h.jobs.Decline(c.Request.Context(), hospitalActor(c), c.Param("job_id"), request.ExpectedRevision, commandID)
	}
	if err != nil {
		hospitalGovernanceHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewHospitalApprovalDTO(*record))
}

func (h *HospitalGovernanceHandler) jobsAvailable(c *gin.Context) bool {
	if h.jobs != nil {
		return true
	}
	httpresponse.Error(c, http.StatusServiceUnavailable, "GOVERNANCE_STORE_UNAVAILABLE", "hospital governance store is unavailable", nil)
	return false
}

func (h *HospitalGovernanceHandler) available(c *gin.Context) bool {
	if h.useCase != nil {
		return true
	}
	httpresponse.Error(c, http.StatusServiceUnavailable, "GOVERNANCE_STORE_UNAVAILABLE", "hospital governance store is unavailable", nil)
	return false
}

func hospitalActor(c *gin.Context) string {
	actor, _ := c.Get(middleware.AuthenticatedUserContextKey)
	value, _ := actor.(string)
	return value
}

func bindHospitalGovernance(c *gin.Context, out any) bool {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		invalidHospitalRequest(c)
		return false
	}
	const limit = 64 * 1024
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(out)
	if err == nil {
		var extra any
		err = decoder.Decode(&extra)
		if errors.Is(err, io.EOF) {
			return true
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		httpresponse.Error(c, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body exceeds the endpoint limit", nil)
		return false
	}
	invalidHospitalRequest(c)
	return false
}

func invalidHospitalRequest(c *gin.Context) {
	httpresponse.Error(c, http.StatusBadRequest, "INVALID_GOVERNANCE_REQUEST", "invalid hospital governance request", nil)
}

func hospitalGovernanceHTTPError(c *gin.Context, err error) {
	var domainErr *domainerrors.DomainError
	if errors.As(err, &domainErr) {
		HandleDomainError(c, domainErr)
		return
	}
	httpresponse.Error(c, http.StatusServiceUnavailable, "GOVERNANCE_STORE_UNAVAILABLE", "hospital governance store is unavailable", nil)
}
