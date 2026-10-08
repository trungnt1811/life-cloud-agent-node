package repositories

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	repositorytest "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/testsupport"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

func openHospitalGovernanceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := repositorytest.OpenPostgresDB(t)
	repositorytest.RequirePostgresDialect(t, db)
	repositorytest.ResetPostgresRepositoryTables(t, db)
	return db
}

func TestGovernedHospitalGovernanceRepositoryPersistsAcrossRecreation(t *testing.T) {
	db := openHospitalGovernanceTestDB(t)
	ctx := context.Background()
	state := entities.NewHospitalGovernance("node-a")
	policy := entities.HospitalPolicyRecord{LocalK: 20, ReleaseMode: "AUTO", EnabledFieldCodes: []string{"MCV"}, FilterFieldCodes: []string{"MCV"}}
	require.NoError(t, state.UpdatePolicy(policy, 1))
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		repo := NewHospitalGovernanceRepository(tx, logger.GetLogger())
		if err := repo.Lock(ctx, "node-a"); err != nil {
			return err
		}
		if err := repo.Save(ctx, state); err != nil {
			return err
		}
		if err := repo.AppendAudit(ctx, entities.HospitalGovernanceAuditRecord{EventID: uuid.NewString(), NodeID: "node-a", Type: "POLICY_CHANGED", Actor: "basic:steward", LocalRevision: 2, OccurredAt: now}); err != nil {
			return err
		}
		return repo.SaveCommand(ctx, entities.HospitalGovernanceCommandRecord{NodeID: "node-a", Actor: "basic:steward", Key: "update-1", Fingerprint: "fingerprint", Response: state.Record(), CreatedAt: now})
	}))
	reopened := NewHospitalGovernanceRepository(db, logger.GetLogger())
	loaded, err := reopened.Get(ctx, "node-a")
	require.NoError(t, err)
	require.Equal(t, state.Record(), loaded.Record())
	command, err := reopened.GetCommand(ctx, "node-a", "basic:steward", "update-1")
	require.NoError(t, err)
	require.NotNil(t, command)
	require.Equal(t, state.Record(), command.Response)
	other, err := reopened.Get(ctx, "node-b")
	require.NoError(t, err)
	require.Nil(t, other)
	command, err = reopened.GetCommand(ctx, "node-a", "basic:other", "update-1")
	require.NoError(t, err)
	require.Nil(t, command)
}

func TestGovernedHospitalGovernanceRepositoryAuditFailureRollsBackStateAndCommand(t *testing.T) {
	db := openHospitalGovernanceTestDB(t)
	ctx := context.Background()
	repo := NewHospitalGovernanceRepository(db, logger.GetLogger())
	initial := entities.NewHospitalGovernance("node-a")
	require.NoError(t, repo.Save(ctx, initial))
	require.NoError(t, db.Exec(`CREATE FUNCTION reject_governance_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'required audit unavailable'; END $$; CREATE TRIGGER reject_governance_audit BEFORE INSERT ON hospital_governance_audit FOR EACH ROW EXECUTE FUNCTION reject_governance_audit()`).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Exec(`DROP TRIGGER IF EXISTS reject_governance_audit ON hospital_governance_audit; DROP FUNCTION IF EXISTS reject_governance_audit()`).Error)
	})
	err := db.Transaction(func(tx *gorm.DB) error {
		repo := NewHospitalGovernanceRepository(tx, logger.GetLogger())
		if err := repo.Lock(ctx, "node-a"); err != nil {
			return err
		}
		state, err := repo.Get(ctx, "node-a")
		if err != nil {
			return err
		}
		if err := state.UpdatePolicy(entities.HospitalPolicyRecord{LocalK: 15, ReleaseMode: "AUTO"}, 1); err != nil {
			return err
		}
		if err := repo.Save(ctx, state); err != nil {
			return err
		}
		if err := repo.SaveCommand(ctx, entities.HospitalGovernanceCommandRecord{NodeID: "node-a", Actor: "basic:steward", Key: "failed-command", Fingerprint: "fingerprint", Response: state.Record(), CreatedAt: time.Now().UTC()}); err != nil {
			return err
		}
		return repo.AppendAudit(ctx, entities.HospitalGovernanceAuditRecord{EventID: uuid.NewString(), NodeID: "node-a", Type: "POLICY_CHANGED", Actor: "basic:steward", LocalRevision: 2, OccurredAt: time.Now().UTC()})
	})
	require.Error(t, err)
	loaded, err := repo.Get(ctx, "node-a")
	require.NoError(t, err)
	require.Equal(t, initial.Record(), loaded.Record())
	command, err := repo.GetCommand(ctx, "node-a", "basic:steward", "failed-command")
	require.NoError(t, err)
	require.Nil(t, command)
}

func TestGovernedHospitalGovernanceRepositoryConcurrentPolicyChangesHaveOneWinner(t *testing.T) {
	db := openHospitalGovernanceTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repo := NewHospitalGovernanceRepository(db, logger.GetLogger())
	require.NoError(t, repo.Save(ctx, entities.NewHospitalGovernance("node-a")))
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, k := range []uint64{15, 20} {
		wg.Add(1)
		go func(k uint64) {
			defer wg.Done()
			<-start
			results <- db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				repo := NewHospitalGovernanceRepository(tx, logger.GetLogger())
				if err := repo.Lock(ctx, "node-a"); err != nil {
					return err
				}
				state, err := repo.Get(ctx, "node-a")
				if err != nil {
					return err
				}
				if err := state.UpdatePolicy(entities.HospitalPolicyRecord{LocalK: k, ReleaseMode: "AUTO"}, 1); err != nil {
					return err
				}
				return repo.Save(ctx, state)
			})
		}(k)
	}
	close(start)
	wg.Wait()
	close(results)
	var successes, conflicts int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, entities.ErrHospitalGovernanceConflict):
			conflicts++
		default:
			t.Fatalf("unexpected transaction error: %v", err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	loaded, err := repo.Get(ctx, "node-a")
	require.NoError(t, err)
	require.EqualValues(t, 2, loaded.Record().LocalRevision)
	require.EqualValues(t, 2, loaded.Record().Policy.Version)
}
