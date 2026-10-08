package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
)

func TestGovernedHospitalStageACKAuditRollbackConcurrentReplayAndRestart(t *testing.T) {
	db, u, task, now := governedHospitalUseCaseFixture(t)
	ctx := context.Background()
	started, err := u.Begin(ctx, task)
	require.NoError(t, err)
	stage := started.Stages[0]
	ack := entities.GovernedHospitalStageAck{JobID: task.JobID, EventID: stage.Binding.EventID, Sequence: stage.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: now.Add(20 * time.Second), QueryDeadline: task.QueryDeadline}
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit ADD CONSTRAINT reject_stage_ack CHECK (event_type <> 'GOVERNED_STAGE_ACKNOWLEDGED') NOT VALID").Error)
	_, err = u.AcknowledgeStage(ctx, ack)
	require.Error(t, err)
	read, err := u.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, started, read, "a failed mandatory audit cannot consume the pending stage or tighten its clock")
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit DROP CONSTRAINT reject_stage_ack").Error)
	type response struct {
		record *entities.GovernedHospitalJobRecord
		err    error
	}
	start := make(chan struct{})
	responses := make(chan response, 2)
	for range 2 {
		go func() {
			<-start
			record, err := u.AcknowledgeStage(ctx, ack)
			responses <- response{record, err}
		}()
	}
	close(start)
	a, b := <-responses, <-responses
	require.NoError(t, a.err)
	require.NoError(t, b.err)
	require.Equal(t, a.record, b.record)
	require.True(t, a.record.Stages[0].Acknowledged)
	require.Equal(t, ack.ExecutionDeadline, a.record.ExecutionDeadline)
	var audited int64
	require.NoError(t, db.Table("hospital_governance_audit").Where("event_type = ?", "GOVERNED_STAGE_ACKNOWLEDGED").Count(&audited).Error)
	require.EqualValues(t, 1, audited)
	restarted := usecases.NewGovernedHospitalJobUseCase(usecases.GovernedHospitalJobDeps{NodeID: task.NodeID, Repository: repositories.NewGovernedHospitalJobRepository(db)})
	reloaded, err := restarted.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, a.record, reloaded)
	ack.ExecutionDeadline = ack.ExecutionDeadline.Add(time.Second)
	_, err = u.AcknowledgeStage(ctx, ack)
	require.Error(t, err, "replaying another CP deadline cannot extend an acknowledged clock")
	reloaded, err = restarted.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, a.record, reloaded)
}
