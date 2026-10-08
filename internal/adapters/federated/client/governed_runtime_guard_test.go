package client

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func runtimeStageACK(t *testing.T, r *governedRuntime, task entities.GovernedHospitalTask, stage *nodev1.JobStage, deadline time.Time) {
	t.Helper()
	require.NotNil(t, stage)
	_, err := r.handle(&nodev1.CenterToNode{Payload: &nodev1.CenterToNode_StageAck{StageAck: &nodev1.StageAck{JobId: task.JobID, EventId: stage.Binding.EventId, Sequence: stage.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: timestamppb.New(deadline), QueryDeadline: timestamppb.New(task.QueryDeadline)}}})
	require.NoError(t, err)
}

func TestGovernedRuntimeStopsActiveCounterOnLocalPauseWithoutEgress(t *testing.T) {
	task, state, now := runtimeGovernedTask(t)
	job, err := entities.NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	jobs, gov, counter := mocks.NewMockGovernedHospitalJobUseCase(ctrl), mocks.NewMockHospitalGovernanceUseCase(ctrl), mocks.NewMockCohortCountUseCase(ctrl)
	var current atomic.Pointer[entities.HospitalGovernanceRecord]
	initial := state.Record()
	current.Store(&initial)
	gov.EXPECT().Read(gomock.Any()).DoAndReturn(func(context.Context) (*entities.HospitalGovernanceRecord, error) { return current.Load(), nil }).AnyTimes()
	jobs.EXPECT().Read(gomock.Any(), task.JobID).Return(nil, nil)
	jobs.EXPECT().Begin(gomock.Any(), task).DoAndReturn(func(context.Context, entities.GovernedHospitalTask) (*entities.GovernedHospitalJobRecord, error) {
		r := job.Record()
		return &r, nil
	})
	jobs.EXPECT().AcknowledgeStage(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, a entities.GovernedHospitalStageAck) (*entities.GovernedHospitalJobRecord, error) {
		err := job.AcknowledgeStage(a)
		r := job.Record()
		return &r, err
	}).Times(2)
	counting := make(chan struct{})
	counter.EXPECT().CountMatchingCohort(gomock.Any(), task.JobID, task.Criteria).DoAndReturn(func(ctx context.Context, _ string, _ types.CohortCriteria) (uint64, error) {
		close(counting)
		<-ctx.Done()
		return 9, ctx.Err()
	})
	jobs.EXPECT().Terminate(gomock.Any(), task.JobID, task.SessionEpoch, "DECLINED", "HOSPITAL_PAUSED").DoAndReturn(func(context.Context, string, string, string, string) (*entities.GovernedHospitalJobRecord, error) {
		err := job.Terminate("DECLINED", "HOSPITAL_PAUSED", time.Now())
		r := job.Record()
		return &r, err
	})
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	send := make(chan *nodev1.NodeToCenter, 8)
	r := newGovernedRuntime(ctx, cancel, NewNodeClient(Config{NodeID: task.NodeID}, Dependencies{GovernedHospitalJobs: jobs, HospitalGovernance: gov, CohortCounter: counter}, nil, nil), send, newInflightJobs(), &wg)
	t.Cleanup(func() { cancel(); wg.Wait() })
	require.NoError(t, r.start(task))
	runtimeStageACK(t, r, task, nextGovernedPacket(t, send).GetJobStage(), now.Add(20*time.Second))
	select {
	case <-counting:
	case <-time.After(3 * time.Second):
		t.Fatal("counter did not start")
	}
	paused := initial
	paused.Paused = true
	current.Store(&paused)
	terminal := nextGovernedPacket(t, send).GetJobStage()
	require.Equal(t, nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_DECLINED, terminal.Phase)
	require.Equal(t, nodev1.GovernanceReason_GOVERNANCE_REASON_HOSPITAL_PAUSED, terminal.Reason)
	require.Empty(t, terminal.Binding.ProtectedPayloadDigest)
	runtimeStageACK(t, r, task, terminal, now.Add(20*time.Second))
	wg.Wait()
	require.Empty(t, send, "pause must not produce a release request or result")
	require.Nil(t, job.Record().ProtectedCount, "aborted raw totals must not become a protected preview")
	require.NoError(t, ctx.Err(), "local pause does not disconnect an otherwise healthy transport")
}

func TestGovernedRuntimeSendAuditFailureCannotEnqueueProtectedResult(t *testing.T) {
	task, state, now := runtimeGovernedTask(t)
	job, err := entities.NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	first := job.PendingStages()[0]
	require.NoError(t, job.AcknowledgeStage(entities.GovernedHospitalStageAck{JobID: task.JobID, EventID: first.Binding.EventID, Sequence: 1, SessionEpoch: task.SessionEpoch, ExecutionDeadline: now.Add(20 * time.Second), QueryDeadline: task.QueryDeadline}))
	require.NoError(t, job.Prepare(9, state, now.Add(time.Millisecond)))
	stored := job.Record()
	ctrl := gomock.NewController(t)
	jobs := mocks.NewMockGovernedHospitalJobUseCase(ctrl)
	jobs.EXPECT().Read(gomock.Any(), task.JobID).Return(&stored, nil)
	jobs.EXPECT().Begin(gomock.Any(), task).Return(&stored, nil)
	jobs.EXPECT().Intent(gomock.Any(), task.JobID, task.SessionEpoch).DoAndReturn(func(context.Context, string, string) (*entities.GovernedHospitalJobRecord, error) {
		_, err := job.ReleaseIntent(state, time.Now())
		r := job.Record()
		return &r, err
	})
	jobs.EXPECT().AcknowledgeStage(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, a entities.GovernedHospitalStageAck) (*entities.GovernedHospitalJobRecord, error) {
		err := job.AcknowledgeStage(a)
		r := job.Record()
		return &r, err
	})
	jobs.EXPECT().Send(gomock.Any(), task.JobID, gomock.Any()).Return(nil, errors.New("mandatory audit unavailable"))
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	send := make(chan *nodev1.NodeToCenter, 8)
	r := newGovernedRuntime(ctx, cancel, NewNodeClient(Config{NodeID: task.NodeID}, Dependencies{GovernedHospitalJobs: jobs, CohortCounter: mocks.NewMockCohortCountUseCase(ctrl)}, nil, nil), send, newInflightJobs(), &wg)
	t.Cleanup(func() { cancel(); wg.Wait() })
	require.NoError(t, r.start(task))
	runtimeStageACK(t, r, task, nextGovernedPacket(t, send).GetJobStage(), now.Add(20*time.Second))
	request := nextGovernedPacket(t, send).GetReleaseRequest()
	require.NotNil(t, request)
	_, err = r.handle(&nodev1.CenterToNode{Payload: &nodev1.CenterToNode_ReleaseDecision{ReleaseDecision: &nodev1.ReleaseDecision{Outcome: &nodev1.ReleaseDecision_Grant{Grant: &nodev1.ReleaseGrant{GrantId: uuid.NewString(), Binding: request.Binding, GrantedAt: timestamppb.Now(), ExpiresAt: timestamppb.New(time.Now().Add(9 * time.Second))}}}}})
	require.NoError(t, err)
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("failed egress audit did not close the stream")
	}
	wg.Wait()
	require.Empty(t, send)
	require.Equal(t, entities.GovernedHospitalPrepared, job.Record().DeliveryState)
}
