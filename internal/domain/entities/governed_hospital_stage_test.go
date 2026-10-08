package entities

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGovernedHospitalStageJournalPinsEventClockAndACKAcrossHydration(t *testing.T) {
	state, task, now := governedJobFixture(t)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	stages := job.PendingStages()
	require.Len(t, stages, 1)
	require.EqualValues(t, 1, stages[0].Sequence)
	require.Equal(t, GovernedHospitalExecuting, stages[0].Phase)
	deadline := now.Add(20 * time.Second)
	ack := GovernedHospitalStageAck{JobID: task.JobID, EventID: stages[0].Binding.EventID, Sequence: 1, SessionEpoch: task.SessionEpoch, ExecutionDeadline: deadline, QueryDeadline: task.QueryDeadline}
	require.NoError(t, job.AcknowledgeStage(ack))
	require.Equal(t, deadline, job.Record().ExecutionDeadline, "CP deadline can tighten, never extend the local clock")
	require.Empty(t, job.PendingStages())
	revision := job.Record().Revision
	require.NoError(t, job.AcknowledgeStage(ack))
	require.Equal(t, revision, job.Record().Revision)
	wrong := ack
	wrong.ExecutionDeadline = wrong.ExecutionDeadline.Add(time.Second)
	require.Error(t, job.AcknowledgeStage(wrong))
	require.NoError(t, job.Prepare(9, state, now.Add(time.Second)))
	_, err = job.ReleaseIntent(state, now.Add(time.Second))
	require.NoError(t, err)
	stages = job.PendingStages()
	require.Len(t, stages, 1)
	require.EqualValues(t, 2, stages[0].Sequence)
	require.Equal(t, GovernedHospitalReady, stages[0].Phase)
	require.Equal(t, job.Record().Binding, stages[0].Binding)
	encoded, err := json.Marshal(job.Record())
	require.NoError(t, err)
	var record GovernedHospitalJobRecord
	require.NoError(t, json.Unmarshal(encoded, &record))
	restored, err := NewGovernedHospitalJobFromRecord(record)
	require.NoError(t, err)
	require.Equal(t, stages, restored.PendingStages())
	stage := stages[0]
	for _, change := range []func(*GovernedHospitalStageAck){
		func(a *GovernedHospitalStageAck) { a.EventID = uuid.NewString() },
		func(a *GovernedHospitalStageAck) { a.SessionEpoch = "foreign" },
		func(a *GovernedHospitalStageAck) { a.QueryDeadline = a.QueryDeadline.Add(time.Second) },
		func(a *GovernedHospitalStageAck) { a.Sequence++ },
		func(a *GovernedHospitalStageAck) { a.ExecutionDeadline = a.ExecutionDeadline.Add(time.Second) },
	} {
		invalid := GovernedHospitalStageAck{JobID: task.JobID, EventID: stage.Binding.EventID, Sequence: stage.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: deadline, QueryDeadline: task.QueryDeadline}
		change(&invalid)
		require.Error(t, restored.AcknowledgeStage(invalid))
	}
	ack.EventID, ack.Sequence = stage.Binding.EventID, stage.Sequence
	require.NoError(t, restored.AcknowledgeStage(ack))
	require.Empty(t, restored.PendingStages())
	corrupt := restored.Record()
	changedDeadline := deadline.Add(time.Second)
	corrupt.Stages[1].AcknowledgedExecutionDeadline = &changedDeadline
	_, err = NewGovernedHospitalJobFromRecord(corrupt)
	require.Error(t, err, "stage history must retain the one committed CP execution clock")
}

func TestGovernedHospitalReconnectingStageReplayDoesNotRewriteRecordedAuthority(t *testing.T) {
	state, task, now := governedJobFixture(t)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	original := job.Record().Stages[0]
	state.FenceConnection()
	snapshot := state.Record().Snapshot
	snapshot.SessionEpoch, snapshot.Revision = "epoch-reconnected", snapshot.Revision+1
	require.NoError(t, state.ApplySnapshot(snapshot, now.Add(time.Second)))
	require.NoError(t, state.Acknowledge(snapshot.SessionEpoch, snapshot.Revision, state.Record().LocalRevision))
	task.SessionEpoch = snapshot.SessionEpoch
	require.NoError(t, job.Resume(task, state, now.Add(time.Second)))
	replayed := job.PendingStages()[0]
	require.Equal(t, original.Binding.EventID, replayed.Binding.EventID)
	require.Equal(t, original.OccurredAt, replayed.OccurredAt)
	require.Equal(t, task.SessionEpoch, replayed.Binding.SessionEpoch)
	require.Equal(t, original, job.Record().Stages[0], "rebinding the transport must not rewrite historical stage data")
}

func TestGovernedHospitalManualStageUsesPersistedControlPlaneApprovalClock(t *testing.T) {
	_, job, task, now := waitingManualJob(t)
	stage := job.PendingStages()[0]
	// Transport and persistence happen after the local WAITING_APPROVAL event.
	committedAt := stage.OccurredAt.Add(time.Second)
	ack := GovernedHospitalStageAck{
		JobID: task.JobID, EventID: stage.Binding.EventID, Sequence: stage.Sequence,
		SessionEpoch: task.SessionEpoch, ExecutionDeadline: now.Add(30 * time.Second),
		ApprovalDeadline: committedAt.Add(48 * time.Hour), QueryDeadline: task.QueryDeadline,
	}
	for _, deadline := range []time.Time{{}, stage.OccurredAt, task.QueryDeadline.Add(time.Second)} {
		invalid := ack
		invalid.ApprovalDeadline = deadline
		require.ErrorIs(t, job.AcknowledgeStage(invalid), ErrGovernedHospitalJobInvalid)
	}
	require.NoError(t, job.AcknowledgeStage(ack), "CP persistence, not the Node event clock, starts the approval window")
	require.Equal(t, ack.ApprovalDeadline, job.Record().ApprovalDeadline)
	encoded, err := json.Marshal(job.Record())
	require.NoError(t, err)
	var record GovernedHospitalJobRecord
	require.NoError(t, json.Unmarshal(encoded, &record))
	restored, err := NewGovernedHospitalJobFromRecord(record)
	require.NoError(t, err)
	revision := restored.Record().Revision
	require.NoError(t, restored.AcknowledgeStage(ack), "replayed ACK does not extend the persisted deadline")
	require.Equal(t, revision, restored.Record().Revision)
	changed := ack
	changed.ApprovalDeadline = changed.ApprovalDeadline.Add(time.Second)
	require.ErrorIs(t, restored.AcknowledgeStage(changed), ErrGovernedHospitalJobInvalid)
	require.Equal(t, ack.ApprovalDeadline, restored.Record().ApprovalDeadline)
	for _, deadline := range []time.Time{stage.OccurredAt, task.QueryDeadline.Add(time.Second)} {
		corrupt := restored.Record()
		corrupt.Stages[1].AcknowledgedApprovalDeadline, corrupt.ApprovalDeadline = &deadline, deadline
		_, err = NewGovernedHospitalJobFromRecord(corrupt)
		require.ErrorIs(t, err, ErrGovernedHospitalJobInvalid)
	}
}
