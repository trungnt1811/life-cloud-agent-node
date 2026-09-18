package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/middleware"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/contracts"
	domainerrors "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestEnabledQueryFieldHandlerListSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	useCase := mocks.NewMockEnabledQueryFieldUseCase(ctrl)
	useCase.EXPECT().
		ListQueryFields(gomock.Any()).
		Return([]*contracts.EnabledQueryFieldOutput{{
			FieldCode: "HB",
			Enabled:   true,
			UpdatedAt: &now,
			UpdatedBy: "alice",
		}}, nil)

	status, payload := performEnabledQueryFieldRequest(
		t,
		NewEnabledQueryFieldHandler(useCase, nil),
		http.MethodGet,
		"/admin/query-fields",
		nil,
		"admin",
		"secret",
	)
	require.Equal(t, http.StatusOK, status)
	items := requireSlice(t, payload["items"])
	require.Len(t, items, 1)
	require.Equal(t, "HB", requireMap(t, items[0])["field_code"])
}

func TestEnabledQueryFieldHandlerUpdateSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	useCase := mocks.NewMockEnabledQueryFieldUseCase(ctrl)
	useCase.EXPECT().
		// updated_by ("admin") is the Basic Auth username performEnabledQueryFieldRequest
		// authenticates with below, not the request body - proving the
		// handler derives it from the authenticated identity, not the client.
		UpdateQueryField(gomock.Any(), contracts.UpdateEnabledQueryFieldInput{
			FieldCode: "HB",
			Enabled:   true,
			UpdatedBy: "admin",
		}).
		Return(&contracts.EnabledQueryFieldOutput{
			FieldCode: "HB",
			Enabled:   true,
			UpdatedAt: &now,
			UpdatedBy: "admin",
		}, nil)

	status, payload := performEnabledQueryFieldRequest(
		t,
		NewEnabledQueryFieldHandler(useCase, nil),
		http.MethodPut,
		"/admin/query-fields/HB",
		map[string]any{"enabled": true},
		"admin",
		"secret",
	)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "HB", payload["field_code"])
	require.Equal(t, true, payload["enabled"])
	require.Equal(t, "admin", payload["updated_by"])
}

func TestEnabledQueryFieldHandlerUnauthorizedWithoutCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	status, _ := performEnabledQueryFieldRequest(
		t,
		NewEnabledQueryFieldHandler(mocks.NewMockEnabledQueryFieldUseCase(ctrl), nil),
		http.MethodGet,
		"/admin/query-fields",
		nil,
		"",
		"",
	)
	require.Equal(t, http.StatusUnauthorized, status)
}

func TestEnabledQueryFieldHandlerDomainError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	useCase := mocks.NewMockEnabledQueryFieldUseCase(ctrl)
	useCase.EXPECT().
		UpdateQueryField(gomock.Any(), gomock.Any()).
		Return(nil, domainerrors.NewValidationError("UNKNOWN_FIELD_CODE", "field_code is not in schema v1"))

	status, payload := performEnabledQueryFieldRequest(
		t,
		NewEnabledQueryFieldHandler(useCase, nil),
		http.MethodPut,
		"/admin/query-fields/ZZZ",
		map[string]any{"enabled": true},
		"admin",
		"secret",
	)
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "UNKNOWN_FIELD_CODE", payload["code"])
}

func TestEnabledQueryFieldHandlerRejectsOmittedEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	status, payload := performEnabledQueryFieldRequest(
		t,
		NewEnabledQueryFieldHandler(mocks.NewMockEnabledQueryFieldUseCase(ctrl), nil),
		http.MethodPut,
		"/admin/query-fields/HB",
		map[string]any{},
		"admin",
		"secret",
	)
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "INVALID_REQUEST", payload["code"])
}

func performEnabledQueryFieldRequest(
	t *testing.T,
	handler *EnabledQueryFieldHandler,
	method, path string,
	body any,
	username, password string,
) (int, map[string]any) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := router.Group("/admin/query-fields")
	admin.Use(middleware.HTTPBasicAuth("admin", "secret", "admin"))
	admin.GET("", handler.ListQueryFields)
	admin.PUT("/:field_code", handler.UpdateQueryField)

	var requestBody bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&requestBody).Encode(body))
	}
	req := httptest.NewRequest(method, path, &requestBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Body.Len() == 0 {
		return rec.Code, nil
	}
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload), rec.Body.String())
	return rec.Code, payload
}
