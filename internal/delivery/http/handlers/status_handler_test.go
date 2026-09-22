package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/client"
)

func TestStatusHandlerGetStatus_NoAdvisoryYet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewStatusHandler("node-a", "v1.0.0", client.NewAdvisoryStore())

	router := gin.New()
	router.GET("/admin/status", handler.GetStatus)

	req := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, "node-a", payload["node_id"])
	require.Equal(t, "v1.0.0", payload["agent_version"])
	require.NotContains(t, payload, "advisory")
}

func TestStatusHandlerGetStatus_ReportsLatestAdvisory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	advisories := client.NewAdvisoryStore()
	advisories.SetFromWire(&nodev1.UpdateAdvisory{
		Version:      "v2.0.0",
		ChangelogUrl: "https://example.invalid/changelog",
		Severity:     nodev1.AdvisorySeverity_ADVISORY_SEVERITY_CRITICAL,
	})
	handler := NewStatusHandler("node-a", "v1.0.0", advisories)

	router := gin.New()
	router.GET("/admin/status", handler.GetStatus)

	req := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	advisory := requireMap(t, payload["advisory"])
	require.Equal(t, "v2.0.0", advisory["version"])
	require.Equal(t, "https://example.invalid/changelog", advisory["changelog_url"])
	require.Equal(t, "ADVISORY_SEVERITY_CRITICAL", advisory["severity"])
	require.NotEmpty(t, advisory["received_at"])
}

func TestStatusHandlerGetStatus_NilAdvisoryStoreIsSafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewStatusHandler("node-a", "v1.0.0", nil)

	router := gin.New()
	router.GET("/admin/status", handler.GetStatus)

	req := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}
