package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/lifenetwork-ai/go-backend-template/internal/domain/contracts"
	domaintypes "github.com/lifenetwork-ai/go-backend-template/internal/domain/types"
	domainerrors "github.com/lifenetwork-ai/go-backend-template/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/go-backend-template/internal/mocks"
)

func TestExampleHandlerListExamplesSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	output := sampleExampleOutput()
	useCase := mocks.NewMockExampleUseCase(ctrl)
	useCase.EXPECT().
		ListExamples(gomock.Any(), contracts.ListExamplesInput{}).
		Return(&domaintypes.PaginatedResponse[*contracts.ExampleOutput]{
			Items:      []*contracts.ExampleOutput{output},
			TotalCount: 1,
			Page:       1,
			PageSize:   20,
		}, nil).
		Times(1)

	status, payload := performExampleHandlerRequest(t, NewExampleHandler(useCase, nil), http.MethodGet, "/api/v1/examples", nil)

	require.Equal(t, http.StatusOK, status)
	items := requireSlice(t, payload["items"])
	require.Len(t, items, 1)
	require.Equal(t, "Alice", requireMap(t, items[0])["name"])
	require.EqualValues(t, 1, payload["total_count"])
}

func TestExampleHandlerGetExampleSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	output := sampleExampleOutput()
	useCase := mocks.NewMockExampleUseCase(ctrl)
	useCase.EXPECT().
		GetExample(gomock.Any(), output.ID).
		Return(output, nil).
		Times(1)

	status, payload := performExampleHandlerRequest(t, NewExampleHandler(useCase, nil), http.MethodGet, "/api/v1/examples/"+output.ID.String(), nil)

	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "Alice", payload["name"])
}

func TestExampleHandlerCreateExampleSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	output := sampleExampleOutput()
	useCase := mocks.NewMockExampleUseCase(ctrl)
	useCase.EXPECT().
		CreateExample(gomock.Any(), contracts.CreateExampleInput{Name: "Alice", Description: "first"}).
		Return(output, nil).
		Times(1)

	status, payload := performExampleHandlerRequest(
		t,
		NewExampleHandler(useCase, nil),
		http.MethodPost,
		"/api/v1/examples",
		map[string]string{"name": "Alice", "description": "first"},
	)

	require.Equal(t, http.StatusCreated, status)
	require.Equal(t, "Alice", payload["name"])
}

func TestExampleHandlerUpdateExampleSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	output := sampleExampleOutput()
	useCase := mocks.NewMockExampleUseCase(ctrl)
	useCase.EXPECT().
		UpdateExample(gomock.Any(), contracts.UpdateExampleInput{
			ID:          output.ID,
			Name:        "Bob",
			Description: "updated",
		}).
		Return(&contracts.ExampleOutput{
			ID:          output.ID,
			Name:        "Bob",
			Description: "updated",
			CreatedAt:   output.CreatedAt,
			UpdatedAt:   output.UpdatedAt.Add(time.Minute),
		}, nil).
		Times(1)

	status, payload := performExampleHandlerRequest(
		t,
		NewExampleHandler(useCase, nil),
		http.MethodPut,
		"/api/v1/examples/"+output.ID.String(),
		map[string]string{"name": "Bob", "description": "updated"},
	)

	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "Bob", payload["name"])
}

func TestExampleHandlerDeleteExampleSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	id := uuid.New()
	useCase := mocks.NewMockExampleUseCase(ctrl)
	useCase.EXPECT().
		DeleteExample(gomock.Any(), id).
		Return(nil).
		Times(1)

	status, payload := performExampleHandlerRequest(t, NewExampleHandler(useCase, nil), http.MethodDelete, "/api/v1/examples/"+id.String(), nil)

	require.Equal(t, http.StatusOK, status)
	require.Equal(t, id.String(), payload["id"])
	require.Equal(t, true, payload["deleted"])
}

func TestExampleHandlerInvalidRequestUsesResponseEnvelope(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	status, payload := performExampleHandlerRequest(t, NewExampleHandler(mocks.NewMockExampleUseCase(ctrl), nil), http.MethodPost, "/api/v1/examples", map[string]string{})

	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "INVALID_REQUEST", payload["code"])
	require.NotEmpty(t, payload["message"])
}

func TestExampleHandlerInvalidIDUsesResponseEnvelope(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	status, payload := performExampleHandlerRequest(t, NewExampleHandler(mocks.NewMockExampleUseCase(ctrl), nil), http.MethodGet, "/api/v1/examples/not-a-uuid", nil)

	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "INVALID_ID", payload["code"])
}

func TestExampleHandlerDomainErrorIncludesTypedDetails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	id := uuid.New()
	useCase := mocks.NewMockExampleUseCase(ctrl)
	useCase.EXPECT().
		GetExample(gomock.Any(), id).
		Return(nil, domainerrors.NewNotFoundError("EXAMPLE_NOT_FOUND", "example not found").WithDetails(domainerrors.ErrorDetail{
			Field:   "id",
			Code:    "EXAMPLE_NOT_FOUND",
			Message: "example not found",
		})).
		Times(1)

	status, payload := performExampleHandlerRequest(t, NewExampleHandler(useCase, nil), http.MethodGet, "/api/v1/examples/"+id.String(), nil)

	require.Equal(t, http.StatusNotFound, status)
	require.Equal(t, "EXAMPLE_NOT_FOUND", payload["code"])
	details := requireSlice(t, payload["errors"])
	require.Equal(t, "id", requireMap(t, details[0])["field"])
}

func TestExampleHandlerUnexpectedErrorUsesResponseEnvelope(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	id := uuid.New()
	useCase := mocks.NewMockExampleUseCase(ctrl)
	useCase.EXPECT().
		GetExample(gomock.Any(), id).
		Return(nil, errors.New("boom")).
		Times(1)

	status, payload := performExampleHandlerRequest(t, NewExampleHandler(useCase, nil), http.MethodGet, "/api/v1/examples/"+id.String(), nil)

	require.Equal(t, http.StatusInternalServerError, status)
	require.Equal(t, "INTERNAL_ERROR", payload["code"])
	require.Equal(t, "internal server error", payload["message"])
}

func performExampleHandlerRequest(t *testing.T, handler *ExampleHandler, method, path string, body any) (int, map[string]any) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1/examples")
	api.GET("", handler.ListExamples)
	api.POST("", handler.CreateExample)
	api.GET("/:id", handler.GetExample)
	api.PUT("/:id", handler.UpdateExample)
	api.DELETE("/:id", handler.DeleteExample)

	var requestBody bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&requestBody).Encode(body))
	}
	req := httptest.NewRequest(method, path, &requestBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload), rec.Body.String())
	return rec.Code, payload
}

func sampleExampleOutput() *contracts.ExampleOutput {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	return &contracts.ExampleOutput{
		ID:          uuid.New(),
		Name:        "Alice",
		Description: "first",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func requireMap(t *testing.T, value any) map[string]any {
	t.Helper()

	mapped, ok := value.(map[string]any)
	require.Truef(t, ok, "expected map[string]any, got %T", value)
	return mapped
}

func requireSlice(t *testing.T, value any) []any {
	t.Helper()

	items, ok := value.([]any)
	require.Truef(t, ok, "expected []any, got %T", value)
	return items
}
