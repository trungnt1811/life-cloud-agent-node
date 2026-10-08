package entities

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGovernedHospitalManualPolicyPersistsProtectedPayloadWithoutRelease(t *testing.T) {
	state, job, _, now := waitingManualJob(t)

	record := job.Record()
	require.Equal(t, "WAITING_APPROVAL", record.Phase)
	require.NotNil(t, record.ProtectedCount)
	require.Empty(t, record.Binding.EventID, "held payload must not create an outbound ledger intent")
	_, err := job.ReleaseIntent(state, now.Add(2*time.Second))
	require.Error(t, err, "a held manual result cannot be released before approval")

	encoded, err := json.Marshal(record)
	require.NoError(t, err)
	var restoredRecord GovernedHospitalJobRecord
	require.NoError(t, json.Unmarshal(encoded, &restoredRecord))
	restored, err := NewGovernedHospitalJobFromRecord(restoredRecord)
	require.NoError(t, err, "held approval must survive serialization and restart")
	require.Equal(t, record, restored.Record())
}

func TestGovernedHospitalManualApprovalIsRevisionedIdempotentAndPayloadBound(t *testing.T) {
	state, job, task, now := waitingManualJob(t)
	ackWaitingStage(t, job, task, now)
	before := job.Record()
	commandID := "3e778ebb-49b4-4c68-82cc-b42fe82e0f68"

	require.NoError(t, job.Approve(before.Revision, commandID, "hospital-admin", state, now.Add(2*time.Second)))
	approved := job.Record()
	require.Equal(t, GovernedHospitalReady, approved.Phase)
	require.Equal(t, "APPROVED", approved.Approval.Decision)
	require.Equal(t, before.Revision, approved.Approval.ExpectedRevision)
	require.Equal(t, before.Revision+1, approved.Approval.Revision)
	require.NoError(t, job.Approve(before.Revision, commandID, "hospital-admin", state, now.Add(3*time.Second)), "same command is idempotent")
	require.Error(t, job.Approve(before.Revision, "fa238968-c540-4413-8a5d-d9357920e35b", "hospital-admin", state, now.Add(3*time.Second)), "a different stale command must conflict")

	binding, err := job.ReleaseIntent(state, now.Add(4*time.Second))
	require.NoError(t, err)
	require.Equal(t, approved.Approval.PayloadDigest, binding.ProtectedPayloadDigest)
	record := job.Record()
	_, err = NewGovernedHospitalJobFromRecord(record)
	require.NoError(t, err, "approval and release binding survive restart")
}

func TestGovernedHospitalManualDeclineAndApprovalExpiryAreTerminal(t *testing.T) {
	t.Run("decline", func(t *testing.T) {
		state, job, task, now := waitingManualJob(t)
		ackWaitingStage(t, job, task, now)
		revision := job.Record().Revision
		require.NoError(t, job.Decline(revision, "b226c8b9-98f8-4e43-8167-2020e80c387f", "hospital-admin", state, now.Add(2*time.Second)))
		record := job.Record()
		require.Equal(t, GovernedHospitalDeclined, record.Phase)
		require.Equal(t, "LOCAL_DECLINED", record.Reason)
		require.Equal(t, "DECLINED", record.Approval.Decision)
		_, err := job.ReleaseIntent(state, now.Add(3*time.Second))
		require.Error(t, err)
		_, err = NewGovernedHospitalJobFromRecord(record)
		require.NoError(t, err)
	})

	t.Run("deadline", func(t *testing.T) {
		state, job, task, now := waitingManualJob(t)
		deadline := ackWaitingStage(t, job, task, now)
		require.NoError(t, job.ExpireApproval(deadline))
		record := job.Record()
		require.Equal(t, GovernedHospitalApprovalExpired, record.Phase)
		require.Equal(t, "APPROVAL_DEADLINE_REACHED", record.Reason)
		require.Error(t, job.Approve(record.Revision, "420e2d55-02c2-47e0-86c9-64507765fe26", "hospital-admin", state, deadline))
		_, err := NewGovernedHospitalJobFromRecord(record)
		require.NoError(t, err)
	})
}

func waitingManualJob(t *testing.T) (*HospitalGovernance, *GovernedHospitalJob, GovernedHospitalTask, time.Time) {
	t.Helper()
	state, task, now := governedJobFixture(t)
	policy := state.Record().Policy
	policy.ReleaseMode = HospitalReleaseManual
	require.NoError(t, state.UpdatePolicy(policy, policy.Version))
	require.NoError(t, state.Acknowledge("epoch-a", 1, state.Record().LocalRevision))
	policy = state.Record().Policy
	task.ReleaseMode, task.LocalPolicyVersion, task.LocalPolicyHash = HospitalReleaseManual, policy.Version, policy.PolicyHash
	task.QueryDeadline = minHospitalDeadline(now.Add(48*time.Hour+90*time.Second), task.Permit.ExpiresAt)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.NoError(t, job.AcknowledgeStage(GovernedHospitalStageAck{JobID: task.JobID, EventID: job.PendingStages()[0].Binding.EventID, Sequence: 1, SessionEpoch: task.SessionEpoch, ExecutionDeadline: now.Add(30 * time.Second), QueryDeadline: task.QueryDeadline}))
	require.NoError(t, job.Prepare(14, state, now.Add(time.Second)))
	return state, job, task, now
}

func ackWaitingStage(t *testing.T, job *GovernedHospitalJob, task GovernedHospitalTask, now time.Time) time.Time {
	t.Helper()
	deadline := now.Add(47 * time.Hour)
	stage := job.PendingStages()[0]
	require.Equal(t, GovernedHospitalWaitingApproval, stage.Phase)
	require.NoError(t, job.AcknowledgeStage(GovernedHospitalStageAck{JobID: task.JobID, EventID: stage.Binding.EventID, Sequence: stage.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: now.Add(30 * time.Second), ApprovalDeadline: deadline, QueryDeadline: task.QueryDeadline}))
	return deadline
}
