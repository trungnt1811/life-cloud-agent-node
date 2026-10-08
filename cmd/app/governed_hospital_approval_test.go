package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	transactionrepo "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/transaction"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
)

func governedHospitalManualApprovalFixture(t *testing.T) (*gorm.DB, interfaces.GovernedHospitalJobUseCase, entities.GovernedHospitalTask, time.Time) {
	t.Helper()
	// Kept as a separate fixture because the production constructor returns the interface.
	db, jobsInterface, task, now := governedHospitalUseCaseFixture(t)
	_ = jobsInterface
	govRepo := repositories.NewHospitalGovernanceRepository(db, nil)
	jobRepo := repositories.NewGovernedHospitalJobRepository(db)
	tx := transactionrepo.NewTransactionManager(db, transactionrepo.TransactionManagerDeps{HospitalGovernanceRepo: govRepo, GovernedHospitalJobsRepo: jobRepo})
	governance := usecases.NewHospitalGovernanceUseCase(usecases.HospitalGovernanceDeps{NodeID: hospitalTestNodeID, Repository: govRepo, Transactions: tx, Now: func() time.Time { return now }})
	state, err := governance.Read(context.Background())
	require.NoError(t, err)
	policy := state.Policy
	policy.ReleaseMode = entities.HospitalReleaseManual
	state, err = governance.UpdatePolicy(context.Background(), "test-service", "manual-policy", policy, policy.Version)
	require.NoError(t, err)
	require.NoError(t, governance.Acknowledge(context.Background(), task.SessionEpoch, state.Snapshot.Revision, state.LocalRevision))
	task.ReleaseMode, task.LocalPolicyVersion, task.LocalPolicyHash = entities.HospitalReleaseManual, state.Policy.Version, state.Policy.PolicyHash
	task.QueryDeadline = task.Permit.ExpiresAt
	return db, usecases.NewGovernedHospitalJobUseCase(usecases.GovernedHospitalJobDeps{NodeID: hospitalTestNodeID, Repository: jobRepo, Transactions: tx, Now: func() time.Time { return now }}), task, now
}

func beginManualApproval(t *testing.T, u interface {
	Begin(context.Context, entities.GovernedHospitalTask) (*entities.GovernedHospitalJobRecord, error)
	AcknowledgeStage(context.Context, entities.GovernedHospitalStageAck) (*entities.GovernedHospitalJobRecord, error)
	Prepare(context.Context, string, string, uint64) (*entities.GovernedHospitalJobRecord, error)
}, task entities.GovernedHospitalTask, now time.Time,
) *entities.GovernedHospitalJobRecord {
	t.Helper()
	ctx := context.Background()
	record, err := u.Begin(ctx, task)
	require.NoError(t, err)
	stage := record.Stages[0]
	_, err = u.AcknowledgeStage(ctx, entities.GovernedHospitalStageAck{JobID: task.JobID, EventID: stage.Binding.EventID, Sequence: stage.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: now.Add(30 * time.Second), QueryDeadline: task.QueryDeadline})
	require.NoError(t, err)
	record, err = u.Prepare(ctx, task.JobID, task.SessionEpoch, 14)
	require.NoError(t, err)
	stage = record.Stages[len(record.Stages)-1]
	deadline := now.Add(time.Hour)
	record, err = u.AcknowledgeStage(ctx, entities.GovernedHospitalStageAck{JobID: task.JobID, EventID: stage.Binding.EventID, Sequence: stage.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: now.Add(30 * time.Second), ApprovalDeadline: deadline, QueryDeadline: task.QueryDeadline})
	require.NoError(t, err)
	require.Equal(t, deadline, record.ApprovalDeadline)
	return record
}

func TestGovernedManualApprovalIsLocalDurableAndAuditedBeforeRelease(t *testing.T) {
	db, u, task, now := governedHospitalManualApprovalFixture(t)
	ctx := context.Background()
	pending := beginManualApproval(t, u, task, now)
	repo := repositories.NewGovernedHospitalJobRepository(db)
	govRepo := repositories.NewHospitalGovernanceRepository(db, nil)
	tx := transactionrepo.NewTransactionManager(db, transactionrepo.TransactionManagerDeps{HospitalGovernanceRepo: govRepo, GovernedHospitalJobsRepo: repo})
	items, total, err := u.ListApprovals(ctx, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, *pending, items[0])
	var ledgerCount int64
	require.NoError(t, db.Table("governed_hospital_outbound_events").Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount, "holding for approval must not create egress ledger intent")

	// A fresh use-case/repository proves the pending decision is PostgreSQL state.
	restarted := usecases.NewGovernedHospitalJobUseCase(usecases.GovernedHospitalJobDeps{NodeID: hospitalTestNodeID, Repository: repo, Transactions: tx})
	restored, err := restarted.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, pending, restored)

	config := conf.DefaultConfiguration()
	config.NodeID, config.AdminBasicAuthUser, config.AdminBasicAuthPass = hospitalTestNodeID, hospitalTestAdminUser, hospitalTestAdminPass
	router := SetupRouter(ctx, db, &config, nil, nil)
	gin.SetMode(gin.TestMode)
	listRequest := httptest.NewRequest(http.MethodGet, "/admin/approvals?page=1&page_size=20", nil)
	listRequest.SetBasicAuth(hospitalTestAdminUser, hospitalTestAdminPass)
	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, listRequest)
	require.Equal(t, http.StatusOK, listResponse.Code, listResponse.Body.String())
	require.Contains(t, listResponse.Body.String(), `"kind":"EXACT"`)
	require.Contains(t, listResponse.Body.String(), `"value":"14"`)
	require.NotContains(t, listResponse.Body.String(), task.PrincipalID)
	require.NotContains(t, listResponse.Body.String(), "NumberValue")

	commandID := uuid.NewString()
	var body bytes.Buffer
	require.NoError(t, json.NewEncoder(&body).Encode(map[string]int64{"expected_revision": pending.Revision}))
	approveRequest := httptest.NewRequest(http.MethodPost, "/admin/approvals/"+task.JobID+"/approve", &body)
	approveRequest.Header.Set("Content-Type", "application/json")
	approveRequest.Header.Set("Idempotency-Key", commandID)
	approveRequest.SetBasicAuth(hospitalTestAdminUser, hospitalTestAdminPass)
	approveResponse := httptest.NewRecorder()
	router.ServeHTTP(approveResponse, approveRequest)
	require.Equal(t, http.StatusOK, approveResponse.Code, approveResponse.Body.String())
	require.Contains(t, approveResponse.Body.String(), `"phase":"READY_TO_RELEASE"`)

	approved, err := restarted.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, "APPROVED", approved.Approval.Decision)
	require.Equal(t, hospitalTestAdminUser, approved.Approval.Actor)
	require.NoError(t, db.Table("hospital_governance_audit").Where("event_type = ? AND actor = ?", "LOCAL_APPROVED", hospitalTestAdminUser).Count(&ledgerCount).Error)
	require.EqualValues(t, 1, ledgerCount)
	require.NoError(t, db.Table("governed_hospital_outbound_events").Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount, "approval is not itself a release grant or egress")

	_, err = restarted.Approve(ctx, hospitalTestAdminUser, task.JobID, pending.Revision, commandID)
	require.NoError(t, err, "retrying the same approval command returns the committed winner")
	require.NoError(t, db.Table("hospital_governance_audit").Where("event_type = ? AND actor = ?", "LOCAL_APPROVED", hospitalTestAdminUser).Count(&ledgerCount).Error)
	require.EqualValues(t, 1, ledgerCount, "idempotent retry does not duplicate audit")

	_, err = restarted.Intent(ctx, task.JobID, task.SessionEpoch)
	require.NoError(t, err, "only after the persisted local approval may an egress intent be created")
	require.NoError(t, db.Table("governed_hospital_outbound_events").Count(&ledgerCount).Error)
	require.EqualValues(t, 1, ledgerCount)
}

func TestGovernedManualApprovalConcurrentApproveAndDeclineHaveOneWinner(t *testing.T) {
	db, u, task, now := governedHospitalManualApprovalFixture(t)
	pending := beginManualApproval(t, u, task, now)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, decide := range []func() error{
		func() error {
			_, err := u.Approve(context.Background(), "admin-a", task.JobID, pending.Revision, uuid.NewString())
			return err
		},
		func() error {
			_, err := u.Decline(context.Background(), "admin-b", task.JobID, pending.Revision, uuid.NewString())
			return err
		},
	} {
		wg.Add(1)
		go func(decide func() error) { defer wg.Done(); <-start; results <- decide() }(decide)
	}
	close(start)
	wg.Wait()
	close(results)
	var winners, conflicts int
	for err := range results {
		if err == nil {
			winners++
		} else {
			conflicts++
		}
	}
	require.Equal(t, 1, winners)
	require.Equal(t, 1, conflicts)
	stored, err := u.Read(context.Background(), task.JobID)
	require.NoError(t, err)
	require.NotEqual(t, entities.GovernedHospitalWaitingApproval, stored.Phase)
	var decisions int64
	require.NoError(t, db.Table("hospital_governance_audit").Where("event_type IN ?", []string{"LOCAL_APPROVED", "LOCAL_DECLINED"}).Count(&decisions).Error)
	require.EqualValues(t, 1, decisions)
}

func TestGovernedManualApprovalSurvivesRestartAndAuditFailureRollsBack(t *testing.T) {
	db, u, task, now := governedHospitalManualApprovalFixture(t)
	pending := beginManualApproval(t, u, task, now)
	records, err := u.ResumableManual(context.Background(), "", 100)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, pending.Task.JobID, records[0].Task.JobID)
	require.Equal(t, entities.GovernedHospitalWaitingApproval, records[0].Phase)

	repo := repositories.NewGovernedHospitalJobRepository(db)
	govRepo := repositories.NewHospitalGovernanceRepository(db, nil)
	tx := transactionrepo.NewTransactionManager(db, transactionrepo.TransactionManagerDeps{HospitalGovernanceRepo: govRepo, GovernedHospitalJobsRepo: repo})
	restarted := usecases.NewGovernedHospitalJobUseCase(usecases.GovernedHospitalJobDeps{NodeID: hospitalTestNodeID, Repository: repo, Transactions: tx})
	commandID := uuid.NewString()
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit ADD CONSTRAINT deny_local_approval CHECK (event_type <> 'LOCAL_APPROVED') NOT VALID").Error)
	_, err = restarted.Approve(context.Background(), "hospital-admin", task.JobID, pending.Revision, commandID)
	require.Error(t, err)
	stored, err := restarted.Read(context.Background(), task.JobID)
	require.NoError(t, err)
	require.Equal(t, entities.GovernedHospitalWaitingApproval, stored.Phase, "audit failure must roll back the approval decision")
	require.Nil(t, stored.Approval)
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit DROP CONSTRAINT deny_local_approval").Error)
	approved, err := restarted.Approve(context.Background(), "hospital-admin", task.JobID, pending.Revision, commandID)
	require.NoError(t, err)
	require.Equal(t, entities.GovernedHospitalReady, approved.Phase)
}
