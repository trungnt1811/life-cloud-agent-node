package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/dto"
	httpresponse "github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/response"
	domainerrors "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// EnabledQueryFieldHandler handles local admin whitelist endpoints.
type EnabledQueryFieldHandler struct {
	useCase interfaces.EnabledQueryFieldUseCase
	logger  logger.Logger
}

// NewEnabledQueryFieldHandler creates a new EnabledQueryFieldHandler.
func NewEnabledQueryFieldHandler(useCase interfaces.EnabledQueryFieldUseCase, logger logger.Logger) *EnabledQueryFieldHandler {
	return &EnabledQueryFieldHandler{
		useCase: useCase,
		logger:  logger,
	}
}

// ListQueryFields lists all schema v1 field codes and this node's enablement state.
// @Summary List enabled query fields
// @Description List all schema v1 field codes with this node's local whitelist state
// @Tags admin
// @Produce json
// @Success 200 {object} dto.EnabledQueryFieldListDTO
// @Failure 401 {object} dto.ErrorDTOResponse
// @Failure 500 {object} dto.ErrorDTOResponse
// @Router /admin/query-fields [get]
func (h *EnabledQueryFieldHandler) ListQueryFields(c *gin.Context) {
	result, err := h.useCase.ListQueryFields(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewEnabledQueryFieldListDTOFromOutputs(result))
}

// UpdateQueryField enables or disables one schema v1 field on this node.
// @Summary Update an enabled query field
// @Description Set enabled and updated_by for one schema v1 field_code
// @Tags admin
// @Accept json
// @Produce json
// @Param field_code path string true "Schema v1 field code"
// @Param body body dto.UpdateEnabledQueryFieldRequest true "Enablement update"
// @Success 200 {object} dto.EnabledQueryFieldDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 401 {object} dto.ErrorDTOResponse
// @Failure 500 {object} dto.ErrorDTOResponse
// @Router /admin/query-fields/{field_code} [put]
func (h *EnabledQueryFieldHandler) UpdateQueryField(c *gin.Context) {
	fieldCode := strings.TrimSpace(c.Param("field_code"))
	if fieldCode == "" {
		httpresponse.Error(c, http.StatusBadRequest, "INVALID_FIELD_CODE", "field_code is required", nil)
		return
	}

	var req dto.UpdateEnabledQueryFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresponse.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), nil)
		return
	}

	result, err := h.useCase.UpdateQueryField(c.Request.Context(), req.ToInput(fieldCode))
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewEnabledQueryFieldDTOFromOutput(result))
}

func (h *EnabledQueryFieldHandler) handleError(c *gin.Context, err error) {
	var domainErr *domainerrors.DomainError
	if errors.As(err, &domainErr) {
		HandleDomainError(c, domainErr)
		return
	}

	if h.logger != nil {
		h.logger.Error("Unhandled enabled query field handler error", logger.Err(err))
	}
	httpresponse.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
}
