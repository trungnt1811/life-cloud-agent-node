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
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/testsupport"
)

func TestGovernedHospitalGovernanceHTTPConcurrentCommandCommitsOneAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testsupport.OpenPostgresDB(t)
	config := conf.DefaultConfiguration()
	config.NodeID, config.AdminBasicAuthUser, config.AdminBasicAuthPass = hospitalTestNodeID, hospitalTestAdminUser, hospitalTestAdminPass
	router := SetupRouter(context.Background(), db, &config, nil, nil)
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		go func() {
			<-start
			req := httptest.NewRequest(http.MethodPut, "/admin/policy", strings.NewReader(`{"expected_revision":1,"local_k":"15","release_mode":"AUTO","enabled_field_codes":["MCV"],"filter_field_codes":["MCV"]}`))
			req.SetBasicAuth(hospitalTestAdminUser, hospitalTestAdminPass)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "concurrent-command")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			results <- w
		}()
	}
	close(start)
	a, b := <-results, <-results
	require.Equal(t, http.StatusOK, a.Code, a.Body.String())
	require.Equal(t, http.StatusOK, b.Code, b.Body.String())
	require.Equal(t, a.Body.String(), b.Body.String())
	var count int64
	require.NoError(t, db.Table("hospital_governance_commands").Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Table("hospital_governance_audit").Count(&count).Error)
	require.EqualValues(t, 1, count)
}
