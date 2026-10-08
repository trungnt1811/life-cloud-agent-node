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

func TestGovernedHospitalInitialPauseRefusalIsDurableWithoutExecutionOrEgress(t *testing.T) {
	db, u, task, _ := governedHospitalUseCaseFixture(t)
	ctx := context.Background()
	gov := repositories.NewHospitalGovernanceRepository(db, nil)
	state, err := gov.Get(ctx, task.NodeID)
	require.NoError(t, err)
	require.NoError(t, state.ChangeAvailability(true, state.Record().AvailabilityRevision))
	require.NoError(t, gov.Save(ctx, state))
	first, err := u.Begin(ctx, task)
	require.NoError(t, err, "a local pause must persist refusal, not fail the transport")
	require.Equal(t, entities.GovernedHospitalDeclined, first.Phase)
	require.Equal(t, "HOSPITAL_PAUSED", first.Reason)
	require.True(t, first.ExecutionStartedAt.IsZero())
	require.True(t, first.ExecutionDeadline.IsZero())
	require.Nil(t, first.ProtectedCount)
	require.Empty(t, first.DeliveryState)
	require.Len(t, first.Stages, 1)
	require.Equal(t, int64(1), first.Stages[0].Sequence)
	require.Equal(t, first.Phase, first.Stages[0].Phase)
	replay, err := u.Begin(ctx, task)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	ack := entities.GovernedHospitalStageAck{JobID: task.JobID, EventID: first.Stages[0].Binding.EventID, Sequence: 1, SessionEpoch: task.SessionEpoch, QueryDeadline: task.QueryDeadline}
	acknowledged, err := u.AcknowledgeStage(ctx, ack)
	require.NoError(t, err, "a refusal ACK has no execution clock")
	require.True(t, acknowledged.Stages[0].Acknowledged)
	require.Nil(t, acknowledged.Stages[0].AcknowledgedExecutionDeadline)
	second, err := u.AcknowledgeStage(ctx, ack)
	require.NoError(t, err)
	require.Equal(t, acknowledged, second)
	restarted := usecases.NewGovernedHospitalJobUseCase(usecases.GovernedHospitalJobDeps{NodeID: task.NodeID, Repository: repositories.NewGovernedHospitalJobRepository(db)})
	read, err := restarted.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Equal(t, acknowledged, read)
	var count int64
	require.NoError(t, db.Table("governed_hospital_outbound_events").Count(&count).Error)
	require.Zero(t, count, "a refusal must never create count-bearing egress intent")
	require.NoError(t, db.Table("hospital_governance_audit").Where("event_type = ?", "GOVERNED_LOCAL_REFUSED").Count(&count).Error)
	require.EqualValues(t, 1, count)
	foreign := task
	foreign.JobID, foreign.NodeID = uuid.NewString(), "foreign-node"
	_, err = u.Begin(ctx, foreign)
	require.Error(t, err, "a pause is not authority to accept a foreign task")
}

func TestGovernedHospitalInitialRefusalAuditFailureRollsBack(t *testing.T) {
	db, u, task, _ := governedHospitalUseCaseFixture(t)
	ctx := context.Background()
	gov := repositories.NewHospitalGovernanceRepository(db, nil)
	state, err := gov.Get(ctx, task.NodeID)
	require.NoError(t, err)
	require.NoError(t, state.ChangeAvailability(true, state.Record().AvailabilityRevision))
	require.NoError(t, gov.Save(ctx, state))
	require.NoError(t, db.Exec("ALTER TABLE hospital_governance_audit ADD CONSTRAINT deny_refusal CHECK (event_type <> 'GOVERNED_LOCAL_REFUSED') NOT VALID").Error)
	t.Cleanup(func() { db.Exec("ALTER TABLE hospital_governance_audit DROP CONSTRAINT IF EXISTS deny_refusal") })
	_, err = u.Begin(ctx, task)
	require.Error(t, err)
	stored, err := u.Read(ctx, task.JobID)
	require.NoError(t, err)
	require.Nil(t, stored, "required refusal audit and local job must commit together")
}
