package app

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	transactionrepo "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/transaction"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
)

func TestGovernedApprovalExpiryRecoversOnlyUnacknowledgedTerminalProof(t *testing.T) {
	db, u, task, now := governedHospitalManualApprovalFixture(t)
	ctx := context.Background()
	pending := beginManualApproval(t, u, task, now)
	repo := repositories.NewGovernedHospitalJobRepository(db)
	gov := repositories.NewHospitalGovernanceRepository(db, nil)
	tx := transactionrepo.NewTransactionManager(db, transactionrepo.TransactionManagerDeps{HospitalGovernanceRepo: gov, GovernedHospitalJobsRepo: repo})
	clock := pending.ApprovalDeadline
	restarted := usecases.NewGovernedHospitalJobUseCase(usecases.GovernedHospitalJobDeps{NodeID: hospitalTestNodeID, Repository: repo, Transactions: tx, Now: func() time.Time { return clock }})
	results := make(chan *entities.GovernedHospitalJobRecord, 2)
	errors := make(chan error, 2)
	for _, approve := range []bool{false, true} {
		go func(approve bool) {
			var record *entities.GovernedHospitalJobRecord
			var err error
			if approve {
				record, err = restarted.Approve(ctx, "hospital-admin", task.JobID, pending.Revision, uuid.NewString())
			} else {
				record, err = restarted.ExpireApproval(ctx, task.JobID)
			}
			results <- record
			errors <- err
		}(approve)
	}
	for range 2 {
		<-results
		<-errors
	}
	expired, err := restarted.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, entities.GovernedHospitalApprovalExpired, expired.Phase)
	require.Nil(t, expired.Approval)
	clock = task.QueryDeadline.Add(time.Hour)
	items, err := restarted.ResumableManual(ctx, "", 100)
	require.NoError(t, err)
	require.Len(t, items, 1, "CP no longer dispatches terminal jobs; reconnect must recover pending expiry journal")
	require.Equal(t, expired, &items[0])
	last := expired.Stages[len(expired.Stages)-1]
	ack := entities.GovernedHospitalStageAck{JobID: task.JobID, EventID: last.Binding.EventID, Sequence: last.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: *expired.Stages[0].AcknowledgedExecutionDeadline, QueryDeadline: task.QueryDeadline}
	_, err = restarted.AcknowledgeStage(ctx, ack)
	require.NoError(t, err)
	items, err = restarted.ResumableManual(ctx, "", 100)
	require.NoError(t, err)
	require.Empty(t, items, "ACKed expiry must not rescan forever")
	var count int64
	require.NoError(t, db.Table("hospital_governance_audit").Where("query_id = ? AND event_type = 'APPROVAL_EXPIRED'", task.QueryID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Table("governed_hospital_outbound_events").Count(&count).Error)
	require.Zero(t, count)
}
