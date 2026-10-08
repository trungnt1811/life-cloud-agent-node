package client

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestGovernedActorWaitsForPersistedApprovalThenContinues(t *testing.T) {
	task, state, now, waiting := runtimeManualWaitingJob(t)
	job, err := entities.NewGovernedHospitalJobFromRecord(*waiting)
	require.NoError(t, err)
	require.NoError(t, job.Approve(waiting.Revision, "36ac1073-6f31-4588-bc5b-5894b420ab46", "hospital-admin", state, now.Add(2*time.Millisecond)))
	ready := job.Record()

	ctrl := gomock.NewController(t)
	jobs, governance := mocks.NewMockGovernedHospitalJobUseCase(ctrl), mocks.NewMockHospitalGovernanceUseCase(ctrl)
	jobs.EXPECT().Read(gomock.Any(), task.JobID).Return(waiting, nil)
	jobs.EXPECT().Read(gomock.Any(), task.JobID).Return(&ready, nil)
	stateRecord := state.Record()
	governance.EXPECT().Read(gomock.Any()).Return(&stateRecord, nil).Times(2)
	nc := NewNodeClient(Config{NodeID: task.NodeID, ApprovalPollInterval: time.Millisecond}, Dependencies{GovernedHospitalJobs: jobs, HospitalGovernance: governance, Now: func() time.Time { return now.Add(3 * time.Millisecond) }}, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	a := &governedActor{runtime: &governedRuntime{client: nc}, task: task}
	result, err := a.waitForApproval(ctx, waiting)
	require.NoError(t, err)
	require.Equal(t, entities.GovernedHospitalReady, result.Phase)
	require.Equal(t, ready.Approval, result.Approval)
}

func TestGovernedActorExpiresApprovalWithoutReleasingAndDeclinesOnPause(t *testing.T) {
	t.Run("approval deadline", func(t *testing.T) {
		task, _, _, waiting := runtimeManualWaitingJob(t)
		expired := *waiting
		expired.Phase, expired.Reason = entities.GovernedHospitalApprovalExpired, "APPROVAL_DEADLINE_REACHED"
		ctrl := gomock.NewController(t)
		jobs := mocks.NewMockGovernedHospitalJobUseCase(ctrl)
		jobs.EXPECT().ExpireApproval(gomock.Any(), task.JobID).Return(&expired, nil)
		nc := NewNodeClient(Config{NodeID: task.NodeID}, Dependencies{GovernedHospitalJobs: jobs, Now: func() time.Time { return waiting.ApprovalDeadline.Add(time.Second) }}, nil, nil)
		a := &governedActor{runtime: &governedRuntime{client: nc}, task: task}
		result, err := a.waitForApproval(context.Background(), waiting)
		require.NoError(t, err)
		require.Equal(t, entities.GovernedHospitalApprovalExpired, result.Phase)
	})

	t.Run("local pause", func(t *testing.T) {
		task, state, now, waiting := runtimeManualWaitingJob(t)
		paused := state.Record()
		paused.Paused = true
		terminal := *waiting
		terminal.Phase, terminal.Reason = entities.GovernedHospitalDeclined, "HOSPITAL_PAUSED"
		ctrl := gomock.NewController(t)
		jobs, governance := mocks.NewMockGovernedHospitalJobUseCase(ctrl), mocks.NewMockHospitalGovernanceUseCase(ctrl)
		governance.EXPECT().Read(gomock.Any()).Return(&paused, nil)
		jobs.EXPECT().Terminate(gomock.Any(), task.JobID, task.SessionEpoch, entities.GovernedHospitalDeclined, "HOSPITAL_PAUSED").Return(&terminal, nil)
		nc := NewNodeClient(Config{NodeID: task.NodeID}, Dependencies{GovernedHospitalJobs: jobs, HospitalGovernance: governance, Now: func() time.Time { return now.Add(time.Second) }}, nil, nil)
		a := &governedActor{runtime: &governedRuntime{client: nc}, task: task}
		result, err := a.waitForApproval(context.Background(), waiting)
		require.NoError(t, err)
		require.Equal(t, entities.GovernedHospitalDeclined, result.Phase)
	})
}

func TestGovernedRegistrationAdvertisesManualOnlyWhenExecutionIsEnabled(t *testing.T) {
	off := registerMessage(Config{NodeID: "node-a", GovernanceSyncEnabled: true}).GetRegister()
	require.Empty(t, off.SupportedReleaseModes)
	active := registerMessage(Config{NodeID: "node-a", GovernanceSyncEnabled: true, GovernedExecutionEnabled: true}).GetRegister()
	require.Equal(t, []nodev1.ReleaseMode{nodev1.ReleaseMode_RELEASE_MODE_AUTO, nodev1.ReleaseMode_RELEASE_MODE_MANUAL}, active.SupportedReleaseModes)
}

func TestGovernedActorStopsHeldApprovalOnCommittedInvalidation(t *testing.T) {
	for _, operation := range []string{"cancel", "revoke"} {
		t.Run(operation, func(t *testing.T) {
			task, state, now, waiting := runtimeManualWaitingJob(t)
			fenced := state.Record()
			phase, reason := entities.GovernedHospitalCanceled, "QUERY_CANCELED"
			if operation == "cancel" {
				fenced.InvalidatedQueries = []entities.HospitalQueryFence{{TenantID: task.TenantID, QueryID: task.QueryID, PermitID: task.Permit.ID, PermitVersion: task.Permit.Version, Reason: reason}}
			} else {
				phase, reason = entities.GovernedHospitalPolicyDenied, "PERMIT_REVOKED"
				fenced.InvalidatedPermits = []entities.HospitalPermitFence{{TenantID: task.TenantID, PermitID: task.Permit.ID, ThroughVersion: task.Permit.Version}}
			}
			terminal := *waiting
			terminal.Phase, terminal.Reason = phase, reason
			ctrl := gomock.NewController(t)
			jobs, governance := mocks.NewMockGovernedHospitalJobUseCase(ctrl), mocks.NewMockHospitalGovernanceUseCase(ctrl)
			governance.EXPECT().Read(gomock.Any()).Return(&fenced, nil)
			jobs.EXPECT().Terminate(gomock.Any(), task.JobID, task.SessionEpoch, phase, reason).Return(&terminal, nil)
			nc := NewNodeClient(Config{NodeID: task.NodeID, ApprovalPollInterval: time.Hour}, Dependencies{GovernedHospitalJobs: jobs, HospitalGovernance: governance, Now: func() time.Time { return now.Add(time.Second) }}, nil, nil)
			a := &governedActor{runtime: &governedRuntime{client: nc}, task: task}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			result, err := a.waitForApproval(ctx, waiting)
			require.NoError(t, err, "a committed fence must terminate before the next approval poll")
			require.Equal(t, phase, result.Phase)
		})
	}
}

func runtimeManualWaitingJob(t *testing.T) (entities.GovernedHospitalTask, *entities.HospitalGovernance, time.Time, *entities.GovernedHospitalJobRecord) {
	t.Helper()
	task, state, now := runtimeGovernedTask(t)
	policy := state.Record().Policy
	policy.ReleaseMode = entities.HospitalReleaseManual
	require.NoError(t, state.UpdatePolicy(policy, policy.Version))
	require.NoError(t, state.Acknowledge(task.SessionEpoch, state.Record().Snapshot.Revision, state.Record().LocalRevision))
	policy = state.Record().Policy
	task.ReleaseMode, task.LocalPolicyVersion, task.LocalPolicyHash = entities.HospitalReleaseManual, policy.Version, policy.PolicyHash
	task.QueryDeadline = task.Permit.ExpiresAt
	job, err := entities.NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	first := job.PendingStages()[0]
	require.NoError(t, job.AcknowledgeStage(entities.GovernedHospitalStageAck{JobID: task.JobID, EventID: first.Binding.EventID, Sequence: first.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: now.Add(30 * time.Second), QueryDeadline: task.QueryDeadline}))
	require.NoError(t, job.Prepare(14, state, now.Add(time.Millisecond)))
	waitingStage := job.PendingStages()[0]
	deadline := now.Add(time.Hour)
	require.NoError(t, job.AcknowledgeStage(entities.GovernedHospitalStageAck{JobID: task.JobID, EventID: waitingStage.Binding.EventID, Sequence: waitingStage.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: now.Add(30 * time.Second), ApprovalDeadline: deadline, QueryDeadline: task.QueryDeadline}))
	record := job.Record()
	return task, state, now, &record
}
