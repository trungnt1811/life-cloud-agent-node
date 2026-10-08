package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/testsupport"
	transactionrepo "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/transaction"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
)

func governedHospitalUseCaseFixture(t *testing.T) (*gorm.DB, interfaces.GovernedHospitalJobUseCase, entities.GovernedHospitalTask, time.Time) {
	t.Helper()
	db := testsupport.OpenPostgresDB(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	gov := repositories.NewHospitalGovernanceRepository(db, nil)
	jobs := repositories.NewGovernedHospitalJobRepository(db)
	tx := transactionrepo.NewTransactionManager(db, transactionrepo.TransactionManagerDeps{HospitalGovernanceRepo: gov, GovernedHospitalJobsRepo: jobs})
	u := usecases.NewHospitalGovernanceUseCase(usecases.HospitalGovernanceDeps{NodeID: hospitalTestNodeID, Repository: gov, Transactions: tx, Now: func() time.Time { return now }})
	permit := entities.HospitalPermitRecord{TenantID: "53a96e1c-87aa-4def-b926-675e7e74df6c", ID: "ledger-study", Version: 1, Status: "ACTIVE", HospitalPermitScope: entities.HospitalPermitScope{Title: "Synthetic ledger study", PIPrincipalID: "4bf1a9a6-1a90-43bc-bded-0a4e901559fb", MemberPrincipalIDs: []string{"4bf1a9a6-1a90-43bc-bded-0a4e901559fb"}, Purpose: "research", JobType: "COUNT_MATCHING_COHORT", JobVersion: "v1", AllowedFieldCodes: []string{"MCV"}, AllowedNodeIDs: []string{hospitalTestNodeID}, EthicsReference: "synthetic-only", ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}}
	var err error
	permit.PermitHash, err = permit.Hash()
	require.NoError(t, err)
	_, err = u.ApplySnapshot(context.Background(), entities.HospitalSnapshot{Revision: 1, SessionEpoch: "epoch", NetworkFloor: 10, Permits: []entities.HospitalPermitRecord{permit}})
	require.NoError(t, err)
	_, err = u.UpdatePolicy(context.Background(), "test-service", "policy", entities.HospitalPolicyRecord{LocalK: 10, ReleaseMode: "AUTO", EnabledFieldCodes: []string{"MCV"}, FilterFieldCodes: []string{"MCV"}, PanelFieldCodes: []string{}}, 1)
	require.NoError(t, err)
	state, err := u.ChangeAcceptance(context.Background(), "test-service", "acceptance", entities.HospitalAcceptanceChange{PermitID: permit.ID, PermitVersion: 1, PermitHash: permit.PermitHash, Decision: "ACCEPTED", AllowedFieldCodes: []string{"MCV"}, ExpiresAt: permit.ExpiresAt})
	require.NoError(t, err)
	require.NoError(t, u.Acknowledge(context.Background(), "epoch", 1, state.LocalRevision))
	criteria := types.CohortCriteria{From: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), Conditions: []types.CohortCondition{{FieldCode: "MCV", Op: types.ComparisonOpLT, NumberValue: "80"}}}
	hash, err := criteria.GovernedDefinitionHash()
	require.NoError(t, err)
	task := entities.GovernedHospitalTask{JobID: uuid.NewString(), QueryID: uuid.NewString(), NodeID: hospitalTestNodeID, TenantID: permit.TenantID, PrincipalID: permit.PIPrincipalID, Purpose: permit.Purpose, Criteria: criteria, DefinitionHash: hash, AuthorizationSnapshotHash: hash, Permit: permit, LocalPolicyVersion: state.Policy.Version, LocalPolicyHash: state.Policy.PolicyHash, AcceptanceRevision: 1, ReleaseMode: "AUTO", EffectiveK: 10, DispatchDeadline: now.Add(30 * time.Second), QueryDeadline: now.Add(90 * time.Second), SessionEpoch: "epoch"}
	return db, usecases.NewGovernedHospitalJobUseCase(usecases.GovernedHospitalJobDeps{NodeID: hospitalTestNodeID, Repository: jobs, Transactions: tx, Now: func() time.Time { return now }}), task, now
}

func TestGovernedHospitalLedgerAtomicityConcurrencyAndRestart(t *testing.T) {
	db, u, task, now := governedHospitalUseCaseFixture(t)
	ctx := context.Background()
	first, err := u.Begin(ctx, task)
	require.NoError(t, err)
	second, err := u.Begin(ctx, task)
	require.NoError(t, err)
	require.Equal(t, first, second)
	_, err = u.Prepare(ctx, task.JobID, task.SessionEpoch, 9)
	require.NoError(t, err)
	start := make(chan struct{})
	type result struct {
		record *entities.GovernedHospitalJobRecord
		err    error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			<-start
			record, err := u.Intent(ctx, task.JobID, task.SessionEpoch)
			results <- result{record, err}
		}()
	}
	close(start)
	a, b := <-results, <-results
	require.NoError(t, a.err)
	require.NoError(t, b.err)
	require.Equal(t, a.record.Binding, b.record.Binding)
	var count int64
	require.NoError(t, db.Table("governed_hospital_outbound_events").Count(&count).Error)
	require.EqualValues(t, 1, count)
	grant := entities.GovernedHospitalGrant{ID: uuid.NewString(), Binding: a.record.Binding, GrantedAt: now, ExpiresAt: now.Add(10 * time.Second)}
	sent, err := u.Send(ctx, task.JobID, grant)
	require.NoError(t, err)
	require.Equal(t, "SENT_UNCONFIRMED", sent.DeliveryState)
	repo := repositories.NewGovernedHospitalJobRepository(db)
	restored := usecases.NewGovernedHospitalJobUseCase(usecases.GovernedHospitalJobDeps{NodeID: hospitalTestNodeID, Repository: repo})
	read, err := restored.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, sent, read, "typed protected payload and uncertainty survive a new process/use-case")
	_, err = u.Intent(ctx, task.JobID, task.SessionEpoch)
	require.Error(t, err, "a new grant is forbidden until uncertain egress is reconciled")
	receipt := entities.GovernedHospitalReceipt{ID: uuid.NewString(), JobID: task.JobID, EventID: a.record.Binding.EventID, ProtectedPayloadDigest: a.record.Binding.ProtectedPayloadDigest, Outcome: "COMMITTED", CommittedAt: now.Add(time.Second)}
	committed, err := u.Receive(ctx, receipt)
	require.NoError(t, err)
	require.Equal(t, "SUCCEEDED", committed.Phase)
	_, err = u.Receive(ctx, receipt)
	require.NoError(t, err)
	require.NoError(t, db.Table("hospital_governance_audit").Where("event_type = ?", "GOVERNED_RESULT_RECEIPT").Count(&count).Error)
	require.EqualValues(t, 1, count)
	items, total, err := repo.ListEvents(ctx, hospitalTestNodeID, 20, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "RECEIVED", items[0].DeliveryState)
	_, exact := items[0].ProtectedCount.ExactValue()
	require.False(t, exact)
	gin.SetMode(gin.TestMode)
	config := conf.DefaultConfiguration()
	config.NodeID, config.AdminBasicAuthUser, config.AdminBasicAuthPass = hospitalTestNodeID, hospitalTestAdminUser, hospitalTestAdminPass
	router := SetupRouter(ctx, db, &config, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/admin/outbound-events?page=1&page_size=20", nil)
	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, request)
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)
	request.SetBasicAuth(hospitalTestAdminUser, hospitalTestAdminPass)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), `"kind":"SUPPRESSED"`)
	require.Contains(t, response.Body.String(), `"value":null`)
	require.Contains(t, response.Body.String(), `"effective_k":"10"`)
	require.NotContains(t, response.Body.String(), task.PrincipalID)
	require.NotContains(t, response.Body.String(), grant.ID)
	require.NotContains(t, response.Body.String(), "running_count")
	invalidPage := httptest.NewRequest(http.MethodGet, "/admin/outbound-events?page=0", nil)
	invalidPage.SetBasicAuth(hospitalTestAdminUser, hospitalTestAdminPass)
	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, invalidPage)
	require.Equal(t, http.StatusBadRequest, invalid.Code)
	regressed := items[0]
	regressed.DeliveryState, regressed.Receipt, regressed.Grant = "PREPARED", nil, nil
	require.Error(t, repo.SaveEvent(ctx, regressed), "a terminal ledger record must never be replaced by an unsent record")
}

func TestGovernedHospitalLedgerAuditFailureCannotAuthorizeEgress(t *testing.T) {
	db, u, task, now := governedHospitalUseCaseFixture(t)
	ctx := context.Background()
	_, err := u.Begin(ctx, task)
	require.NoError(t, err)
	_, err = u.Prepare(ctx, task.JobID, task.SessionEpoch, 14)
	require.NoError(t, err)
	intent, err := u.Intent(ctx, task.JobID, task.SessionEpoch)
	require.NoError(t, err)
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit ADD CONSTRAINT deny_egress CHECK (event_type <> 'GOVERNED_EGRESS_PREPARED') NOT VALID").Error)
	t.Cleanup(func() { db.Exec("ALTER TABLE hospital_governance_audit DROP CONSTRAINT IF EXISTS deny_egress") })
	_, err = u.Send(ctx, task.JobID, entities.GovernedHospitalGrant{ID: uuid.NewString(), Binding: intent.Binding, GrantedAt: now, ExpiresAt: now.Add(10 * time.Second)})
	require.Error(t, err)
	stored, err := u.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, "PREPARED", stored.DeliveryState)
	require.Nil(t, stored.Grant)
	event, err := repositories.NewGovernedHospitalJobRepository(db).GetEvent(ctx, hospitalTestNodeID, intent.Binding.EventID)
	require.NoError(t, err)
	require.Equal(t, "PREPARED", event.DeliveryState)
	require.Nil(t, event.Grant)
}
