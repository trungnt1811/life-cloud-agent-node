package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/middleware"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestHospitalApprovalHandlerUsesBasicActorAndIdempotencyKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	jobs := mocks.NewMockGovernedHospitalJobUseCase(ctrl)
	jobID, commandID := "c2da32fe-a1d5-4c58-9708-b676bfc8f10e", "36ac1073-6f31-4588-bc5b-5894b420ab46"
	jobs.EXPECT().Approve(gomock.Any(), "admin", jobID, int64(4), commandID).Return(&entities.GovernedHospitalJobRecord{Task: entities.GovernedHospitalTask{JobID: jobID}, Phase: entities.GovernedHospitalReady, Revision: 5}, nil)

	status, payload := performHospitalApprovalRequest(t, NewHospitalGovernanceHandler(nil, jobs), http.MethodPost, "/admin/approvals/"+jobID+"/approve", map[string]any{"expected_revision": 4}, "admin", "secret", commandID)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, jobID, payload["job_id"])
	require.Equal(t, entities.GovernedHospitalReady, payload["phase"])
}

func TestHospitalApprovalHandlerRequiresBasicAuthAndValidCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	jobs := mocks.NewMockGovernedHospitalJobUseCase(ctrl)
	handler := NewHospitalGovernanceHandler(nil, jobs)
	jobID := "c2da32fe-a1d5-4c58-9708-b676bfc8f10e"
	status, _ := performHospitalApprovalRequest(t, handler, http.MethodPost, "/admin/approvals/"+jobID+"/decline", map[string]any{"expected_revision": 4}, "", "", "")
	require.Equal(t, http.StatusUnauthorized, status)

	status, payload := performHospitalApprovalRequest(t, handler, http.MethodPost, "/admin/approvals/"+jobID+"/approve", map[string]any{"expected_revision": 0}, "admin", "secret", "36ac1073-6f31-4588-bc5b-5894b420ab46")
	require.Equal(t, http.StatusBadRequest, status)
	require.NotNil(t, payload)
}

func TestHospitalApprovalHandlerListsOnlyUseCasePreviews(t *testing.T) {
	ctrl := gomock.NewController(t)
	jobs := mocks.NewMockGovernedHospitalJobUseCase(ctrl)
	jobs.EXPECT().ListApprovals(gomock.Any(), 1, 20).Return([]entities.GovernedHospitalJobRecord{}, int64(0), nil)
	status, payload := performHospitalApprovalRequest(t, NewHospitalGovernanceHandler(nil, jobs), http.MethodGet, "/admin/approvals", nil, "admin", "secret", "")
	require.Equal(t, http.StatusOK, status)
	require.EqualValues(t, 0, payload["total_count"])
}

func performHospitalApprovalRequest(t *testing.T, handler *HospitalGovernanceHandler, method, path string, body any, username, password, commandID string) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.DefaultPagination())
	admin := router.Group("/admin")
	admin.Use(middleware.HTTPBasicAuth("admin", "secret", "hospital-admin"))
	admin.GET("/approvals", handler.ListApprovals)
	admin.POST("/approvals/:job_id/approve", handler.ApproveResult)
	admin.POST("/approvals/:job_id/decline", handler.DeclineResult)
	var requestBody bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&requestBody).Encode(body))
	}
	req := httptest.NewRequest(method, path, &requestBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if commandID != "" {
		req.Header.Set("Idempotency-Key", commandID)
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
