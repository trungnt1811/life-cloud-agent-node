package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
)

func TestGovernedHospitalTerminalStageAuditRollbackAndRestart(t *testing.T) {
	db, u, task, _ := governedHospitalUseCaseFixture(t)
	ctx := context.Background()
	initial, err := u.Begin(ctx, task)
	require.NoError(t, err)
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit ADD CONSTRAINT reject_terminal_stage CHECK (event_type <> 'GOVERNED_LOCAL_TERMINATED') NOT VALID").Error)
	_, err = u.Terminate(ctx, task.JobID, task.SessionEpoch, "DECLINED", "HOSPITAL_PAUSED")
	require.Error(t, err)
	read, err := u.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, initial, read)
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit DROP CONSTRAINT reject_terminal_stage").Error)
	terminal, err := u.Terminate(ctx, task.JobID, task.SessionEpoch, "DECLINED", "HOSPITAL_PAUSED")
	require.NoError(t, err)
	require.Equal(t, "DECLINED", terminal.Phase)
	require.Len(t, terminal.Stages, 2)
	require.Nil(t, terminal.ProtectedCount)
	require.Nil(t, terminal.Receipt)
	replay, err := u.Terminate(ctx, task.JobID, task.SessionEpoch, "DECLINED", "HOSPITAL_PAUSED")
	require.NoError(t, err)
	require.Equal(t, terminal, replay)
	var count int64
	require.NoError(t, db.Table("hospital_governance_audit").Where("event_type = ?", "GOVERNED_LOCAL_TERMINATED").Count(&count).Error)
	require.EqualValues(t, 1, count)
	restarted := usecases.NewGovernedHospitalJobUseCase(usecases.GovernedHospitalJobDeps{NodeID: task.NodeID, Repository: repositories.NewGovernedHospitalJobRepository(db)})
	read, err = restarted.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, terminal, read)
}
