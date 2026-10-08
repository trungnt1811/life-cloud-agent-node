package entities

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func TestGovernedHospitalInitialRefusalOnlyReportsCurrentLocalGuards(t *testing.T) {
	for _, scenario := range []string{"pause", "acceptance", "scope", "cancel", "revoke"} {
		t.Run(scenario, func(t *testing.T) {
			state, task, now := governedJobFixture(t)
			phase, reason := GovernedHospitalPolicyDenied, "SCOPE_DENIED"
			switch scenario {
			case "pause":
				require.NoError(t, state.ChangeAvailability(true, 1))
				phase, reason = GovernedHospitalDeclined, "HOSPITAL_PAUSED"
			case "acceptance":
				r := state.Record()
				r.Acceptances = nil
				state = NewHospitalGovernanceFromRecord(r)
				reason = "ACCEPTANCE_REQUIRED"
			case "scope":
				task.LocalPolicyVersion++
			case "cancel":
				require.NoError(t, state.InvalidateQuery(HospitalInvalidationCommand{CommandID: uuid.NewString(), TenantID: task.TenantID, QueryID: task.QueryID, PermitID: task.Permit.ID, ThroughVersion: task.Permit.Version, SessionEpoch: task.SessionEpoch, Reason: types.GovernanceReasonQueryCanceled}))
				phase, reason = GovernedHospitalCanceled, types.GovernanceReasonQueryCanceled
			case "revoke":
				require.NoError(t, state.InvalidatePermit(HospitalInvalidationCommand{CommandID: uuid.NewString(), TenantID: task.TenantID, PermitID: task.Permit.ID, ThroughVersion: task.Permit.Version, SessionEpoch: task.SessionEpoch, Reason: types.GovernanceReasonPermitRevoked}))
				reason = types.GovernanceReasonPermitRevoked
			}
			job, err := NewGovernedHospitalRefusal(task, state, now)
			require.NoError(t, err)
			r := job.Record()
			require.Equal(t, phase, r.Phase)
			require.Equal(t, reason, r.Reason)
			require.True(t, r.ExecutionStartedAt.IsZero())
			require.True(t, r.ExecutionDeadline.IsZero())
			require.Nil(t, r.ProtectedCount)
			for _, change := range []func(*GovernedHospitalJobRecord){
				func(r *GovernedHospitalJobRecord) { r.ExecutionStartedAt = now },
				func(r *GovernedHospitalJobRecord) { r.ExecutionDeadline = now.Add(time.Second) },
				func(r *GovernedHospitalJobRecord) { r.Stages[0].Sequence = 2 },
				func(r *GovernedHospitalJobRecord) { r.Stages[0].ExecutionDurationMilliseconds = 1 },
				func(r *GovernedHospitalJobRecord) { r.Stages[0].Binding.EventID = uuid.Nil.String() },
			} {
				corrupt := job.Record()
				change(&corrupt)
				_, err := NewGovernedHospitalJobFromRecord(corrupt)
				require.Error(t, err)
			}
			ack := GovernedHospitalStageAck{JobID: task.JobID, EventID: r.Stages[0].Binding.EventID, Sequence: 1, SessionEpoch: task.SessionEpoch, QueryDeadline: task.QueryDeadline, ExecutionDeadline: now.Add(time.Second)}
			require.Error(t, job.AcknowledgeStage(ack), "refusal cannot acquire an invented execution deadline")
			ack.ExecutionDeadline = time.Time{}
			require.NoError(t, job.AcknowledgeStage(ack))
			restored, err := NewGovernedHospitalJobFromRecord(job.Record())
			require.NoError(t, err)
			require.Empty(t, restored.PendingStages())
		})
	}
}

func TestGovernedHospitalInitialRefusalCannotAcceptMalformedOrUnfencedWork(t *testing.T) {
	state, task, now := governedJobFixture(t)
	_, err := NewGovernedHospitalRefusal(task, state, now)
	require.Error(t, err, "authorized work cannot be falsely labeled refused")
	require.NoError(t, state.ChangeAvailability(true, 1))
	for _, change := range []func(*GovernedHospitalTask){
		func(t *GovernedHospitalTask) { t.NodeID = "foreign" },
		func(t *GovernedHospitalTask) { t.SessionEpoch = "old" },
		func(t *GovernedHospitalTask) { t.DefinitionHash = "invalid" },
		func(t *GovernedHospitalTask) { t.EffectiveK = 1 },
	} {
		invalid := task.Clone()
		change(&invalid)
		_, err := NewGovernedHospitalRefusal(invalid, state, now)
		require.Error(t, err)
	}
	state.FenceConnection()
	_, err = NewGovernedHospitalRefusal(task, state, now)
	require.Error(t, err, "a closed stream cannot create a new refusal proof")
	validState, validTask, _ := governedJobFixture(t)
	active, err := NewGovernedHospitalJob(validTask, validState, now)
	require.NoError(t, err)
	stage := active.PendingStages()[0]
	require.Error(t, active.AcknowledgeStage(GovernedHospitalStageAck{JobID: validTask.JobID, EventID: stage.Binding.EventID, Sequence: 1, SessionEpoch: validTask.SessionEpoch, QueryDeadline: validTask.QueryDeadline}), "execution still requires its committed CP clock")
}
