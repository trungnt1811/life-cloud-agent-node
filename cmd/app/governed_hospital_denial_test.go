package app

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
)

func TestGovernedHospitalGrantDenialAuditRollbackReplayAndRestart(t *testing.T) {
	db, u, task, _ := governedHospitalUseCaseFixture(t)
	ctx := context.Background()
	_, err := u.Begin(ctx, task)
	require.NoError(t, err)
	_, err = u.Prepare(ctx, task.JobID, task.SessionEpoch, 9)
	require.NoError(t, err)
	intent, err := u.Intent(ctx, task.JobID, task.SessionEpoch)
	require.NoError(t, err)
	denial := entities.GovernedHospitalDenial{Binding: intent.Binding, Reason: "SESSION_STALE"}
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit ADD CONSTRAINT reject_grant_denial CHECK (event_type <> 'GOVERNED_GRANT_DENIED') NOT VALID").Error)
	_, err = u.Deny(ctx, denial)
	require.Error(t, err)
	stored, err := u.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, intent, stored, "failed mandatory audit cannot consume a release intent")
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit DROP CONSTRAINT reject_grant_denial").Error)
	start := make(chan struct{})
	type response struct {
		record *entities.GovernedHospitalJobRecord
		err    error
	}
	responses := make(chan response, 2)
	for range 2 {
		go func() { <-start; r, err := u.Deny(ctx, denial); responses <- response{r, err} }()
	}
	close(start)
	a, b := <-responses, <-responses
	require.NoError(t, a.err)
	require.NoError(t, b.err)
	require.Equal(t, a.record, b.record)
	require.Nil(t, a.record.Receipt)
	require.Nil(t, a.record.Grant)
	var count int64
	require.NoError(t, db.Table("hospital_governance_audit").Where("event_type = ?", "GOVERNED_GRANT_DENIED").Count(&count).Error)
	require.EqualValues(t, 1, count)
	restarted := usecases.NewGovernedHospitalJobUseCase(usecases.GovernedHospitalJobDeps{NodeID: task.NodeID, Repository: repositories.NewGovernedHospitalJobRepository(db)})
	reloaded, err := restarted.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, a.record, reloaded)
	fresh, err := u.Intent(ctx, task.JobID, task.SessionEpoch)
	require.NoError(t, err)
	require.NotEqual(t, intent.Binding.EventID, fresh.Binding.EventID)
	replay, err := u.Deny(ctx, denial)
	require.NoError(t, err)
	require.Equal(t, fresh, replay, "a historical denial never resets the new intent")
	denial.Binding.EventID = uuid.NewString()
	_, err = u.Deny(ctx, denial)
	require.Error(t, err)
}
