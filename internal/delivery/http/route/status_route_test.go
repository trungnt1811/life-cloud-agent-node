package route

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/client"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/handlers"
)

func TestSetupStatusRoutesRequiresCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := handlers.NewStatusHandler("node-a", "v1.0.0", client.NewAdvisoryStore())

	t.Run("missing credentials skips registration", func(t *testing.T) {
		router := gin.New()
		SetupStatusRoutes(router, handler, AdminAuthOptions{}, nil)

		req := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("partial credentials panics instead of silently 404ing forever", func(t *testing.T) {
		router := gin.New()
		require.Panics(t, func() {
			SetupStatusRoutes(router, handler, AdminAuthOptions{Username: "admin"}, nil)
		})
	})

	t.Run("credentials register protected route and report identity", func(t *testing.T) {
		router := gin.New()
		SetupStatusRoutes(router, handler, AdminAuthOptions{Username: "admin", Password: "secret"}, nil)

		unauth := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
		unauthRec := httptest.NewRecorder()
		router.ServeHTTP(unauthRec, unauth)
		require.Equal(t, http.StatusUnauthorized, unauthRec.Code)

		auth := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
		auth.SetBasicAuth("admin", "secret")
		authRec := httptest.NewRecorder()
		router.ServeHTTP(authRec, auth)
		require.Equal(t, http.StatusOK, authRec.Code)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(authRec.Body.Bytes(), &payload))
		require.Equal(t, "node-a", payload["node_id"])
		require.Equal(t, "v1.0.0", payload["agent_version"])
		require.NotContains(t, payload, "advisory", "no advisory received yet")
	})
}
