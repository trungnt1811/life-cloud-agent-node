package client

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestGovernedRuntimeInitialLocalRefusalDoesNotExecuteOrDisconnect(t *testing.T) {
	task, state, now := runtimeGovernedTask(t)
	require.NoError(t, state.ChangeAvailability(true, 1))
	job, err := entities.NewGovernedHospitalRefusal(task, state, now)
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	jobs := mocks.NewMockGovernedHospitalJobUseCase(ctrl)
	counter := mocks.NewMockCohortCountUseCase(ctrl)
	jobs.EXPECT().Read(gomock.Any(), task.JobID).Return(nil, nil)
	jobs.EXPECT().Begin(gomock.Any(), task).DoAndReturn(func(context.Context, entities.GovernedHospitalTask) (*entities.GovernedHospitalJobRecord, error) {
		r := job.Record()
		return &r, nil
	})
	jobs.EXPECT().AcknowledgeStage(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, ack entities.GovernedHospitalStageAck) (*entities.GovernedHospitalJobRecord, error) {
		err := job.AcknowledgeStage(ack)
		r := job.Record()
		return &r, err
	})
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	send := make(chan *nodev1.NodeToCenter, 4)
	r := newGovernedRuntime(ctx, cancel, NewNodeClient(Config{NodeID: task.NodeID}, Dependencies{GovernedHospitalJobs: jobs, CohortCounter: counter}, nil, nil), send, newInflightJobs(), &wg)
	t.Cleanup(func() { cancel(); wg.Wait() })
	require.NoError(t, r.start(task))
	stage := nextGovernedPacket(t, send).GetJobStage()
	require.NotNil(t, stage)
	require.Equal(t, int64(1), stage.Sequence)
	require.Equal(t, nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_DECLINED, stage.Phase)
	require.Equal(t, nodev1.GovernanceReason_GOVERNANCE_REASON_HOSPITAL_PAUSED, stage.Reason)
	require.Empty(t, stage.Binding.ProtectedPayloadDigest)
	_, err = r.handle(&nodev1.CenterToNode{Payload: &nodev1.CenterToNode_StageAck{StageAck: &nodev1.StageAck{JobId: task.JobID, EventId: stage.Binding.EventId, Sequence: stage.Sequence, SessionEpoch: task.SessionEpoch, QueryDeadline: timestamppb.New(task.QueryDeadline)}}})
	require.NoError(t, err)
	wg.Wait()
	require.Empty(t, send, "no execution stage, grant request or count-bearing result")
	require.NoError(t, ctx.Err(), "honest local refusal keeps a healthy mTLS stream open")
}
