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

func TestGovernedHospitalHistoricReceiptReplayCannotRewriteFreshEvent(t *testing.T) {
	db, u, task, now := governedHospitalUseCaseFixture(t)
	ctx := context.Background()
	_, err := u.Begin(ctx, task)
	require.NoError(t, err)
	_, err = u.Prepare(ctx, task.JobID, task.SessionEpoch, 14)
	require.NoError(t, err)
	first, err := u.Intent(ctx, task.JobID, task.SessionEpoch)
	require.NoError(t, err)
	_, err = u.Send(ctx, task.JobID, entities.GovernedHospitalGrant{ID: uuid.NewString(), Binding: first.Binding, GrantedAt: now, ExpiresAt: now.Add(10 * time.Second)})
	require.NoError(t, err)
	receipt := entities.GovernedHospitalReceipt{ID: uuid.NewString(), JobID: task.JobID, EventID: first.Binding.EventID, ProtectedPayloadDigest: first.Binding.ProtectedPayloadDigest, Outcome: entities.GovernedHospitalRejected, Reason: "SESSION_STALE", CommittedAt: now.Add(time.Second)}
	_, err = u.Receive(ctx, receipt)
	require.NoError(t, err)
	fresh, err := u.Intent(ctx, task.JobID, task.SessionEpoch)
	require.NoError(t, err)
	require.NotEqual(t, first.Binding.EventID, fresh.Binding.EventID)
	gov, jobs := repositories.NewHospitalGovernanceRepository(db, nil), repositories.NewGovernedHospitalJobRepository(db)
	tx := transactionrepo.NewTransactionManager(db, transactionrepo.TransactionManagerDeps{HospitalGovernanceRepo: gov, GovernedHospitalJobsRepo: jobs})
	restarted := usecases.NewGovernedHospitalJobUseCase(usecases.GovernedHospitalJobDeps{NodeID: hospitalTestNodeID, Repository: jobs, Transactions: tx})
	replayed, err := restarted.Receive(ctx, receipt)
	require.NoError(t, err, "historical receipt must reconcile its recorded event without reauthorizing egress")
	require.Equal(t, fresh, replayed)
	var audits int64
	require.NoError(t, db.Table("hospital_governance_audit").Where("event_type = ?", "GOVERNED_RESULT_RECEIPT").Count(&audits).Error)
	require.EqualValues(t, 1, audits)
	for _, mutate := range []func(*entities.GovernedHospitalReceipt){
		func(r *entities.GovernedHospitalReceipt) { r.ID = uuid.NewString() },
		func(r *entities.GovernedHospitalReceipt) { r.Outcome, r.Reason = "COMMITTED", "" },
		func(r *entities.GovernedHospitalReceipt) { r.JobID = uuid.NewString() },
		func(r *entities.GovernedHospitalReceipt) { r.CommittedAt = r.CommittedAt.Add(time.Microsecond) },
	} {
		invalid := receipt
		mutate(&invalid)
		_, err = restarted.Receive(ctx, invalid)
		require.Error(t, err)
	}
	stored, err := restarted.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, fresh, stored)
}
