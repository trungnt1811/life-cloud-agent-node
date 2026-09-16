package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/dto"
	httpresponse "github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/response"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/contracts"
	domainerrors "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// ExampleHandler handles requests related to example operations.
type ExampleHandler struct {
	exampleUseCase interfaces.ExampleUseCase
	logger         logger.Logger
}

// NewExampleHandler creates a new ExampleHandler.
func NewExampleHandler(exampleUseCase interfaces.ExampleUseCase, logger logger.Logger) *ExampleHandler {
	return &ExampleHandler{
		exampleUseCase: exampleUseCase,
		logger:         logger,
	}
}

// ListExamples handles the request to list examples.
// @Summary List examples
// @Description List examples with pagination
// @Tags examples
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} dto.ExampleListDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 500 {object} dto.ErrorDTOResponse
// @Router /api/v1/examples [get]
func (h *ExampleHandler) ListExamples(c *gin.Context) {
	result, err := h.exampleUseCase.ListExamples(c.Request.Context(), contracts.ListExamplesInput{
		Page:     c.GetInt(constants.DefaultPageText),
		PageSize: c.GetInt(constants.AltPageSizeText),
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.NewExampleListDTOFromOutput(result))
}

// GetExample handles the request to get an example by ID.
// @Summary Get an example by ID
// @Description Get an example by ID
// @Tags examples
// @Accept json
// @Produce json
// @Param id path string true "Example ID"
// @Success 200 {object} dto.ExampleDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 404 {object} dto.ErrorDTOResponse
// @Failure 500 {object} dto.ErrorDTOResponse
// @Router /api/v1/examples/{id} [get]
func (h *ExampleHandler) GetExample(c *gin.Context) {
	id, ok := h.exampleIDFromParam(c)
	if !ok {
		return
	}

	example, err := h.exampleUseCase.GetExample(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.NewExampleDTOFromOutput(example))
}

// CreateExample handles the request to create a new example.
// @Summary Create a new example
// @Description Create a new example
// @Tags examples
// @Accept json
// @Produce json
// @Param example body dto.CreateExampleRequest true "Example to create"
// @Success 201 {object} dto.ExampleDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 500 {object} dto.ErrorDTOResponse
// @Router /api/v1/examples [post]
func (h *ExampleHandler) CreateExample(c *gin.Context) {
	var req dto.CreateExampleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresponse.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), nil)
		return
	}

	example, err := h.exampleUseCase.CreateExample(c.Request.Context(), req.ToInput())
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.NewExampleDTOFromOutput(example))
}

// UpdateExample handles the request to update an example.
// @Summary Update an example
// @Description Update an example by ID
// @Tags examples
// @Accept json
// @Produce json
// @Param id path string true "Example ID"
// @Param example body dto.UpdateExampleRequest true "Example fields to update"
// @Success 200 {object} dto.ExampleDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 404 {object} dto.ErrorDTOResponse
// @Failure 500 {object} dto.ErrorDTOResponse
// @Router /api/v1/examples/{id} [put]
func (h *ExampleHandler) UpdateExample(c *gin.Context) {
	id, ok := h.exampleIDFromParam(c)
	if !ok {
		return
	}

	var req dto.UpdateExampleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresponse.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), nil)
		return
	}

	example, err := h.exampleUseCase.UpdateExample(c.Request.Context(), req.ToInput(id))
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.NewExampleDTOFromOutput(example))
}

// DeleteExample handles the request to delete an example.
// @Summary Delete an example
// @Description Delete an example by ID
// @Tags examples
// @Accept json
// @Produce json
// @Param id path string true "Example ID"
// @Success 200 {object} dto.ExampleDeletedDTO
// @Failure 400 {object} dto.ErrorDTOResponse
// @Failure 404 {object} dto.ErrorDTOResponse
// @Failure 500 {object} dto.ErrorDTOResponse
// @Router /api/v1/examples/{id} [delete]
func (h *ExampleHandler) DeleteExample(c *gin.Context) {
	id, ok := h.exampleIDFromParam(c)
	if !ok {
		return
	}

	if err := h.exampleUseCase.DeleteExample(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.NewExampleDeletedDTO(id))
}

func (h *ExampleHandler) exampleIDFromParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpresponse.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid example ID format", nil)
		return uuid.Nil, false
	}
	return id, true
}

func (h *ExampleHandler) handleError(c *gin.Context, err error) {
	var domainErr *domainerrors.DomainError
	if errors.As(err, &domainErr) {
		HandleDomainError(c, domainErr)
		return
	}

	if h.logger != nil {
		h.logger.Error("Unhandled example handler error", logger.Err(err))
	}
	httpresponse.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
}
