package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/testsupport"
	transactionrepo "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/transaction"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
)

const (
	hospitalTestNodeID    = "node-a"
	hospitalTestAdminUser = "hospital-admin"
	hospitalTestAdminPass = "test-only"
)

func TestGovernedHospitalGovernanceHTTPPolicyAuthenticationReplayAndSafeBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testsupport.OpenPostgresDB(t)
	config := conf.DefaultConfiguration()
	config.NodeID, config.AdminBasicAuthUser, config.AdminBasicAuthPass = hospitalTestNodeID, hospitalTestAdminUser, hospitalTestAdminPass
	router := SetupRouter(context.Background(), db, &config, nil, nil)
	body := `{"expected_revision":1,"local_k":"15","release_mode":"AUTO","enabled_field_codes":["MCV"],"filter_field_codes":["MCV"],"panel_field_codes":[]}`
	call := func(method, path, body, key string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		if authenticated {
			req.SetBasicAuth(hospitalTestAdminUser, hospitalTestAdminPass)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, http.StatusUnauthorized, call("GET", "/admin/policy", "", "", false).Code)
	initial := call("GET", "/admin/policy", "", "", true)
	require.Equal(t, http.StatusOK, initial.Code, initial.Body.String())
	require.Contains(t, initial.Body.String(), `"local_k":"10"`)
	require.Contains(t, initial.Body.String(), `"cp_synchronized":false`)
	require.Equal(t, http.StatusBadRequest, call("PUT", "/admin/policy", body, "", true).Code)
	first := call("PUT", "/admin/policy", body, "policy-one", true)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	second := call("PUT", "/admin/policy", body, "policy-one", true)
	require.Equal(t, first.Body.String(), second.Body.String())
	require.Equal(t, http.StatusConflict, call("PUT", "/admin/policy", strings.Replace(body, `"15"`, `"20"`, 1), "policy-one", true).Code)
	require.Equal(t, http.StatusConflict, call("PUT", "/admin/policy", body, "stale", true).Code)
	for _, invalid := range []string{
		`{"local_k":"10","actor":"spoofed"}`,
		`{"local_k":`,
		body + `{}`,
		strings.Replace(body, `"15"`, `15`, 1),
		strings.Replace(body, `"15"`, `"9223372036854775808"`, 1),
		strings.Replace(body, `"15"`, `"5"`, 1),
		strings.Replace(body, `"AUTO"`, `"REMOTE"`, 1),
	} {
		w := call("PUT", "/admin/policy", invalid, "invalid", true)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		require.NotContains(t, w.Body.String(), "spoofed")
		require.NotContains(t, w.Body.String(), "unexpected EOF")
	}
	oversized := call("PUT", "/admin/policy", strings.Repeat(" ", 65537), "oversized", true)
	require.Equal(t, http.StatusRequestEntityTooLarge, oversized.Code)
	var audits []struct{ Actor string }
	require.NoError(t, db.Table("hospital_governance_audit").Select("actor").Find(&audits).Error)
	require.Len(t, audits, 1)
	require.Equal(t, hospitalTestAdminUser, audits[0].Actor)
	require.Equal(t, first.Body.String(), call("GET", "/admin/policy", "", "", true).Body.String())

	require.NoError(t, db.Exec(`CREATE FUNCTION reject_http_governance_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'private store failure'; END $$`).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_http_governance_audit BEFORE INSERT ON hospital_governance_audit FOR EACH ROW EXECUTE FUNCTION reject_http_governance_audit()`).Error)
	t.Cleanup(func() {
		db.Exec(`DROP TRIGGER IF EXISTS reject_http_governance_audit ON hospital_governance_audit`)
		db.Exec(`DROP FUNCTION IF EXISTS reject_http_governance_audit()`)
	})
	failed := call("PUT", "/admin/policy", strings.Replace(body, `"expected_revision":1`, `"expected_revision":2`, 1), "audit-failure", true)
	require.Equal(t, http.StatusServiceUnavailable, failed.Code, failed.Body.String())
	require.NotContains(t, failed.Body.String(), "private store failure")
	require.Equal(t, first.Body.String(), call("GET", "/admin/policy", "", "", true).Body.String())
}

func TestGovernedHospitalGovernanceHTTPAcceptanceCannotInventControlPlaneScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testsupport.OpenPostgresDB(t)
	config := conf.DefaultConfiguration()
	config.NodeID, config.AdminBasicAuthUser, config.AdminBasicAuthPass = hospitalTestNodeID, hospitalTestAdminUser, hospitalTestAdminPass
	router := SetupRouter(context.Background(), db, &config, nil, nil)
	put := func(body, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PUT", "/admin/study-permits/demo/acceptance", strings.NewReader(body))
		req.SetBasicAuth(hospitalTestAdminUser, hospitalTestAdminPass)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	p := entities.HospitalPermitRecord{
		TenantID: "53a96e1c-87aa-4def-b926-675e7e74df6c", ID: "demo", Version: 1, Status: "ACTIVE",
		HospitalPermitScope: entities.HospitalPermitScope{
			Title: "Synthetic study", PIPrincipalID: "4bf1a9a6-1a90-43bc-bded-0a4e901559fb",
			MemberPrincipalIDs: []string{"4bf1a9a6-1a90-43bc-bded-0a4e901559fb"}, Purpose: "research", JobType: "COUNT_MATCHING_COHORT", JobVersion: "v1",
			AllowedFieldCodes: []string{"MCV"}, AllowedNodeIDs: []string{hospitalTestNodeID}, EthicsReference: "synthetic", ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
		},
	}
	var err error
	p.PermitHash, err = p.Hash()
	require.NoError(t, err)
	encoded, err := json.Marshal(map[string]any{"permit_version": 1, "permit_hash": p.PermitHash, "decision": "ACCEPTED", "allowed_field_codes": []string{"MCV"}, "expires_at": p.ExpiresAt, "expected_revision": 0})
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, put(string(encoded), "accept").Code)
	repo := repositories.NewHospitalGovernanceRepository(db, nil)
	tx := transactionrepo.NewTransactionManager(db, transactionrepo.TransactionManagerDeps{HospitalGovernanceRepo: repo})
	u := usecases.NewHospitalGovernanceUseCase(usecases.HospitalGovernanceDeps{NodeID: hospitalTestNodeID, Repository: repo, Transactions: tx})
	_, err = u.ApplySnapshot(context.Background(), entities.HospitalSnapshot{Revision: 1, SessionEpoch: "epoch-1", NetworkFloor: 10, Permits: []entities.HospitalPermitRecord{p}})
	require.NoError(t, err)
	var missingRevision map[string]any
	require.NoError(t, json.Unmarshal(encoded, &missingRevision))
	delete(missingRevision, "expected_revision")
	missingBody, err := json.Marshal(missingRevision)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, put(string(missingBody), "missing-revision").Code)
	missingRevision["expected_revision"] = nil
	missingBody, err = json.Marshal(missingRevision)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, put(string(missingBody), "null-revision").Code)
	accepted := put(string(encoded), "accept")
	require.Equal(t, http.StatusOK, accepted.Code, accepted.Body.String())
	require.Contains(t, accepted.Body.String(), `"cp_synchronized":false`)
	require.Equal(t, accepted.Body.String(), put(string(encoded), "accept").Body.String())
	require.Equal(t, http.StatusBadRequest, put(strings.Replace(string(encoded), `"MCV"`, `"HB"`, 1), "widen").Code)
	require.NoError(t, u.Acknowledge(context.Background(), "epoch-1", 1, 2))
	require.NoError(t, u.FenceConnection(context.Background()))
	require.Error(t, u.Acknowledge(context.Background(), "epoch-1", 1, 2), "old ACK must not restore authority after disconnect")
	restarted := usecases.NewHospitalGovernanceUseCase(usecases.HospitalGovernanceDeps{NodeID: hospitalTestNodeID, Repository: repositories.NewHospitalGovernanceRepository(db, nil), Transactions: tx})
	r, err := restarted.Read(context.Background())
	require.NoError(t, err)
	require.False(t, entities.NewHospitalGovernanceFromRecord(*r).Synchronized())
	require.Len(t, r.Acceptances, 1)
}
