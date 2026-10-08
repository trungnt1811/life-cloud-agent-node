package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/testsupport"
)

func TestGovernedHospitalAvailabilityPrivateRevisionReplayAndAuditAtomicity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testsupport.OpenPostgresDB(t)
	config := conf.DefaultConfiguration()
	config.NodeID, config.AdminBasicAuthUser, config.AdminBasicAuthPass = hospitalTestNodeID, hospitalTestAdminUser, hospitalTestAdminPass
	router := SetupRouter(context.Background(), db, &config, nil, nil)
	call := func(method, body, key string, auth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/admin/availability", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		if auth {
			req.SetBasicAuth(hospitalTestAdminUser, hospitalTestAdminPass)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, http.StatusUnauthorized, call("GET", "", "", false).Code)
	initial := call("GET", "", "", true)
	require.Equal(t, http.StatusOK, initial.Code, initial.Body.String())
	require.Contains(t, initial.Body.String(), `"paused":false`)
	require.Contains(t, initial.Body.String(), `"version":1`)
	for _, body := range []string{`{"expected_revision":1}`, `{"paused":null,"expected_revision":1}`, `{"paused":true}`, `{"paused":true,"expected_revision":1,"actor":"spoofed"}`} {
		require.Equal(t, http.StatusBadRequest, call("PUT", body, "invalid", true).Code)
	}
	body := `{"paused":true,"expected_revision":1}`
	paused := call("PUT", body, "pause", true)
	require.Equal(t, http.StatusOK, paused.Code, paused.Body.String())
	require.Contains(t, paused.Body.String(), `"paused":true`)
	require.Contains(t, paused.Body.String(), `"cp_synchronized":false`)
	require.Equal(t, paused.Body.String(), call("PUT", body, "pause", true).Body.String())
	require.Equal(t, http.StatusConflict, call("PUT", body, "stale", true).Code)
	require.Equal(t, http.StatusConflict, call("PUT", strings.Replace(body, "true", "false", 1), "pause", true).Code)
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit ADD CONSTRAINT deny_availability CHECK (event_type <> 'HOSPITAL_AVAILABILITY_CHANGED') NOT VALID").Error)
	t.Cleanup(func() { db.Exec("ALTER TABLE hospital_governance_audit DROP CONSTRAINT IF EXISTS deny_availability") })
	failed := call("PUT", `{"paused":false,"expected_revision":2}`, "resume", true)
	require.Equal(t, http.StatusServiceUnavailable, failed.Code, failed.Body.String())
	require.Equal(t, paused.Body.String(), call("GET", "", "", true).Body.String())
	state, err := repositories.NewHospitalGovernanceRepository(db, nil).Get(context.Background(), hospitalTestNodeID)
	require.NoError(t, err)
	require.True(t, state.Record().Paused)
	var count int64
	require.NoError(t, db.Table("hospital_governance_commands").Count(&count).Error)
	require.EqualValues(t, 1, count)
}
