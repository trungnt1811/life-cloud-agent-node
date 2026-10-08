package app

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/testsupport"
	transactionrepo "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/transaction"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
)

func TestGovernedHospitalQueryInvalidationIsDurableAtomicAndReplaySafe(t *testing.T) {
	db := testsupport.OpenPostgresDB(t)
	repo := repositories.NewHospitalGovernanceRepository(db, nil)
	manager := transactionrepo.NewTransactionManager(db, transactionrepo.TransactionManagerDeps{HospitalGovernanceRepo: repo})
	deps := usecases.HospitalGovernanceDeps{NodeID: hospitalTestNodeID, Repository: repo, Transactions: manager}
	u := usecases.NewHospitalGovernanceUseCase(deps)
	ctx := context.Background()
	_, err := u.ApplySnapshot(ctx, entities.HospitalSnapshot{Revision: 1, NetworkFloor: 10, SessionEpoch: "current", Permits: []entities.HospitalPermitRecord{}})
	require.NoError(t, err)
	command := entities.HospitalInvalidationCommand{CommandID: uuid.NewString(), TenantID: "53a96e1c-87aa-4def-b926-675e7e74df6c", QueryID: uuid.NewString(), PermitID: "demo", ThroughVersion: 2, Reason: "QUERY_CANCELED", SessionEpoch: "current"}
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); failures <- u.Invalidate(ctx, command) }()
	}
	wg.Wait()
	require.NoError(t, <-failures)
	require.NoError(t, <-failures)
	state, err := repo.Get(ctx, hospitalTestNodeID)
	require.NoError(t, err)
	require.True(t, state.QueryInvalidated(command.TenantID, command.QueryID))
	require.False(t, state.PermitInvalidated(command.TenantID, command.PermitID, 2))
	var count int64
	require.NoError(t, db.Table("hospital_governance_audit").Where("event_type = 'QUERY_INVALIDATED' AND query_id = ? AND command_id = ?", command.QueryID, command.CommandID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	restarted := usecases.NewHospitalGovernanceUseCase(deps)
	require.NoError(t, restarted.FenceConnection(ctx))
	require.Error(t, restarted.Invalidate(ctx, command))
	_, err = restarted.ApplySnapshot(ctx, entities.HospitalSnapshot{Revision: 2, NetworkFloor: 10, SessionEpoch: "new", Permits: []entities.HospitalPermitRecord{}})
	require.NoError(t, err)
	command.SessionEpoch = "new"
	require.NoError(t, restarted.Invalidate(ctx, command))
	changed := command
	changed.QueryID = uuid.NewString()
	require.Error(t, restarted.Invalidate(ctx, changed))
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit ADD CONSTRAINT reject_query_invalidation CHECK (event_type <> 'QUERY_INVALIDATED') NOT VALID").Error)
	changed.CommandID = uuid.NewString()
	require.Error(t, restarted.Invalidate(ctx, changed))
	state, err = repo.Get(ctx, hospitalTestNodeID)
	require.NoError(t, err)
	require.True(t, state.QueryInvalidated(command.TenantID, command.QueryID))
	require.False(t, state.QueryInvalidated(changed.TenantID, changed.QueryID))
	require.Equal(t, "new", state.Record().Snapshot.SessionEpoch)
	require.NoError(t, db.Table("hospital_governance_commands").Count(&count).Error)
	require.EqualValues(t, 1, count)
}
