package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/testsupport"
	transactionrepo "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/transaction"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
)

func TestGovernedHospitalInvalidationCommitReplayFencingAndAuditRollback(t *testing.T) {
	db := testsupport.OpenPostgresDB(t)
	repo := repositories.NewHospitalGovernanceRepository(db, nil)
	manager := transactionrepo.NewTransactionManager(db, transactionrepo.TransactionManagerDeps{HospitalGovernanceRepo: repo})
	u := usecases.NewHospitalGovernanceUseCase(usecases.HospitalGovernanceDeps{NodeID: hospitalTestNodeID, Repository: repo, Transactions: manager})
	ctx := context.Background()
	_, err := u.ApplySnapshot(ctx, entities.HospitalSnapshot{Revision: 1, NetworkFloor: 10, SessionEpoch: "current", Permits: []entities.HospitalPermitRecord{}})
	require.NoError(t, err)
	command := entities.HospitalInvalidationCommand{CommandID: "6c990c65-61b0-4311-86f9-6e4b6beb53cc", TenantID: "53a96e1c-87aa-4def-b926-675e7e74df6c", PermitID: "demo", ThroughVersion: 2, Reason: "PERMIT_SUPERSEDED", SessionEpoch: "current"}
	require.NoError(t, u.Invalidate(ctx, command))
	require.NoError(t, u.Invalidate(ctx, command))
	state, err := repo.Get(ctx, hospitalTestNodeID)
	require.NoError(t, err)
	require.True(t, state.PermitInvalidated(command.TenantID, command.PermitID, 2))
	require.EqualValues(t, 2, state.Record().LocalRevision)
	changed := command
	changed.ThroughVersion++
	require.Error(t, u.Invalidate(ctx, changed), "changed command ID reuse must not expand the recorded intent")
	require.NoError(t, u.FenceConnection(ctx))
	require.Error(t, u.Invalidate(ctx, command), "a stale session cannot ACK even an identical command")
	_, err = u.ApplySnapshot(ctx, entities.HospitalSnapshot{Revision: 2, NetworkFloor: 10, SessionEpoch: "new", Permits: []entities.HospitalPermitRecord{}})
	require.NoError(t, err)
	command.SessionEpoch = "new"
	require.NoError(t, u.Invalidate(ctx, command), "same committed intent is reconciled on the new connection")
	state, err = repo.Get(ctx, hospitalTestNodeID)
	require.NoError(t, err)
	require.Equal(t, "new", state.Record().Snapshot.SessionEpoch, "immutable command response must not overwrite current authority")
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit ADD CONSTRAINT reject_invalidation CHECK (event_type <> 'PERMIT_INVALIDATED') NOT VALID").Error)
	t.Cleanup(func() {
		require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit DROP CONSTRAINT IF EXISTS reject_invalidation").Error)
	})
	command.CommandID, command.ThroughVersion = "3d6e20f2-5e7e-4e57-b5bc-e80321119c31", 3
	require.Error(t, u.Invalidate(ctx, command))
	state, err = repo.Get(ctx, hospitalTestNodeID)
	require.NoError(t, err)
	require.False(t, state.PermitInvalidated(command.TenantID, command.PermitID, 3))
	var count int64
	require.NoError(t, db.Table("hospital_governance_commands").Count(&count).Error)
	require.EqualValues(t, 1, count, "failed audit cannot leave an ACK-able command receipt")
}
