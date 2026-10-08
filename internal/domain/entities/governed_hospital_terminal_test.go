package entities

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func TestGovernedHospitalTerminalStageIsDurableAndDoesNotClaimRelease(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		state, task, now := governedJobFixture(t)
		job, err := NewGovernedHospitalJob(task, state, now)
		require.NoError(t, err)
		if prepared {
			require.NoError(t, job.Prepare(9, state, now.Add(time.Second)))
			_, err := job.ReleaseIntent(state, now.Add(time.Second))
			require.NoError(t, err)
		}
		require.NoError(t, job.Terminate("DECLINED", "HOSPITAL_PAUSED", now.Add(2*time.Second)))
		r := job.Record()
		require.Equal(t, "DECLINED", r.Phase)
		require.Nil(t, r.Grant)
		require.Nil(t, r.Receipt)
		last := r.Stages[len(r.Stages)-1]
		require.Equal(t, "DECLINED", last.Phase)
		require.Equal(t, "HOSPITAL_PAUSED", last.Reason)
		require.NotEqual(t, r.Binding.EventID, last.Binding.EventID)
		require.NoError(t, job.Terminate("DECLINED", "HOSPITAL_PAUSED", now.Add(3*time.Second)))
		require.Equal(t, r, job.Record())
		encoded, err := json.Marshal(r)
		require.NoError(t, err)
		var stored GovernedHospitalJobRecord
		require.NoError(t, json.Unmarshal(encoded, &stored))
		restored, err := NewGovernedHospitalJobFromRecord(stored)
		require.NoError(t, err)
		require.Error(t, restored.Terminate("FAILED", "INTERNAL_FAILURE", now.Add(4*time.Second)))
		require.Error(t, restored.Prepare(14, state, now.Add(4*time.Second)))
		_, err = restored.ReleaseIntent(state, now.Add(4*time.Second))
		require.Error(t, err)
	}
}

func TestGovernedHospitalTerminalStageCannotResolveUncertainEgress(t *testing.T) {
	state, task, now := governedJobFixture(t)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.Error(t, job.Terminate(GovernedHospitalSucceeded, "INTERNAL_FAILURE", now))
	require.Error(t, job.Terminate("FAILED", "database stack trace", now))
	require.Error(t, job.Terminate("FAILED", "INTERNAL_FAILURE", now.Add(-time.Second)))
	require.NoError(t, job.Prepare(9, state, now.Add(time.Second)))
	binding, err := job.ReleaseIntent(state, now.Add(time.Second))
	require.NoError(t, err)
	require.NoError(t, job.AuthorizeSend(GovernedHospitalGrant{ID: uuid.NewString(), Binding: binding, GrantedAt: now.Add(time.Second), ExpiresAt: now.Add(11 * time.Second)}, state, now.Add(2*time.Second)))
	before := job.Record()
	require.Error(t, job.Terminate("CANCELED", "QUERY_CANCELED", now.Add(3*time.Second)))
	require.Equal(t, before, job.Record())
	before.Phase, before.Reason = "CANCELED", "QUERY_CANCELED"
	_, err = NewGovernedHospitalJobFromRecord(before)
	require.Error(t, err, "storage cannot fabricate a locally canceled receipt outcome")
}

func TestGovernedHospitalTerminalJournalRebindsTransportWithoutRenewingPermission(t *testing.T) {
	state, task, now := governedJobFixture(t)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.NoError(t, job.Terminate("DECLINED", "HOSPITAL_PAUSED", now.Add(time.Second)))
	before := job.Record()
	state.FenceConnection()
	snapshot := state.Record().Snapshot
	snapshot.SessionEpoch, snapshot.Revision = "epoch-b", snapshot.Revision+1
	require.NoError(t, state.ApplySnapshot(snapshot, now.Add(2*time.Second)))
	require.NoError(t, state.ChangeAvailability(true, 1))
	require.NoError(t, state.Acknowledge("epoch-b", snapshot.Revision, state.Record().LocalRevision))
	task.SessionEpoch = "epoch-b"
	require.NoError(t, job.Resume(task, state, now.Add(2*time.Second)))
	require.Equal(t, "epoch-b", job.Record().Task.SessionEpoch)
	require.Equal(t, before.Stages, job.Record().Stages, "recorded stage authority remains historical")
	require.Equal(t, "epoch-b", job.PendingStages()[1].Binding.SessionEpoch)
	require.Equal(t, before.ExecutionDeadline, job.Record().ExecutionDeadline)
	_, err = job.ReleaseIntent(state, now.Add(2*time.Second))
	require.Error(t, err, "rebinding a terminal report cannot create execution or disclosure authority")
}

func TestGovernedHospitalResumeRefusesInvalidatedManualWorkWithoutRecount(t *testing.T) {
	for _, operation := range []string{"cancel", "revoke", "pause"} {
		for _, approved := range []bool{false, true} {
			t.Run(operation+map[bool]string{false: "-held", true: "-approved"}[approved], func(t *testing.T) {
				state, job, task, now := waitingManualJob(t)
				ackWaitingStage(t, job, task, now)
				if approved {
					require.NoError(t, job.Approve(job.Record().Revision, uuid.NewString(), "hospital-admin", state, now.Add(2*time.Second)))
				}
				before := job.Record()
				phase, reason := GovernedHospitalCanceled, "QUERY_CANCELED"
				command := HospitalInvalidationCommand{CommandID: uuid.NewString(), TenantID: task.TenantID, PermitID: task.Permit.ID, ThroughVersion: task.Permit.Version, SessionEpoch: task.SessionEpoch}
				switch operation {
				case "cancel":
					command.QueryID, command.Reason = task.QueryID, reason
					require.NoError(t, state.InvalidateQuery(command))
				case "revoke":
					phase, reason = GovernedHospitalPolicyDenied, types.GovernanceReasonPermitRevoked
					command.Reason = reason
					require.NoError(t, state.InvalidatePermit(command))
				case "pause":
					phase, reason = GovernedHospitalDeclined, types.GovernanceReasonHospitalPaused
					require.NoError(t, state.ChangeAvailability(true, 1))
				}
				state.FenceConnection()
				snapshot := state.Record().Snapshot
				snapshot.SessionEpoch, snapshot.Revision = "epoch-restarted", snapshot.Revision+1
				require.NoError(t, state.ApplySnapshot(snapshot, now.Add(3*time.Second)))
				require.NoError(t, state.Acknowledge(snapshot.SessionEpoch, snapshot.Revision, state.Record().LocalRevision))
				task.SessionEpoch = snapshot.SessionEpoch
				foreign := task
				foreign.SessionEpoch = "foreign-epoch"
				require.Error(t, job.Resume(foreign, state, now.Add(3*time.Second)))
				require.Equal(t, before, job.Record())
				require.NoError(t, job.Resume(task, state, now.Add(3*time.Second)), "recovery records a refusal instead of retrying a revoked held job forever")
				record := job.Record()
				require.Equal(t, phase, record.Phase)
				require.Equal(t, reason, record.Reason)
				require.Equal(t, task.SessionEpoch, record.Task.SessionEpoch)
				require.Equal(t, before.ExecutionStartedAt, record.ExecutionStartedAt)
				require.Equal(t, before.ExecutionDeadline, record.ExecutionDeadline)
				require.Equal(t, before.ApprovalDeadline, record.ApprovalDeadline)
				require.Equal(t, before.ProtectedCount, record.ProtectedCount)
				require.Nil(t, record.Grant)
				require.Nil(t, record.Receipt)
				_, err := NewGovernedHospitalJobFromRecord(record)
				require.NoError(t, err)
				_, err = job.ReleaseIntent(state, now.Add(4*time.Second))
				require.Error(t, err)
			})
		}
	}
}
