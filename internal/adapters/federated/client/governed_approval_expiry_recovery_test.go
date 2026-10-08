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

func TestGovernedActorReplaysExpiredApprovalPastQueryDeadlineWithoutCounting(t *testing.T) {
	task, _, _, waiting := runtimeManualWaitingJob(t)
	// This fixture's immutable task clock is in the past, not a production override.
	task.QueryDeadline = time.Now().UTC().Add(-time.Hour)
	waiting.Task.QueryDeadline = task.QueryDeadline
	waiting.ApprovalDeadline = task.QueryDeadline.Add(-time.Hour)
	waiting.Stages[1].AcknowledgedApprovalDeadline = &waiting.ApprovalDeadline
	waiting.Stages[0].OccurredAt = task.QueryDeadline.Add(-3 * time.Hour)
	waiting.ExecutionStartedAt = waiting.Stages[0].OccurredAt
	waiting.ExecutionDeadline = waiting.ExecutionStartedAt.Add(30 * time.Second)
	for i := range waiting.Stages {
		waiting.Stages[i].AcknowledgedExecutionDeadline = &waiting.ExecutionDeadline
	}
	waiting.Stages[1].OccurredAt = waiting.ExecutionStartedAt.Add(time.Millisecond)
	task.DispatchDeadline = waiting.ExecutionStartedAt.Add(30 * time.Second)
	waiting.Task.DispatchDeadline = task.DispatchDeadline
	fingerprint, err := task.Fingerprint()
	require.NoError(t, err)
	waiting.Fingerprint = fingerprint
	job, err := entities.NewGovernedHospitalJobFromRecord(*waiting)
	require.NoError(t, err)
	require.NoError(t, job.ExpireApproval(waiting.ApprovalDeadline))
	expired := job.Record()
	stage := job.PendingStages()[0]
	ack := entities.GovernedHospitalStageAck{JobID: task.JobID, EventID: stage.Binding.EventID, Sequence: stage.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: expired.ExecutionDeadline, QueryDeadline: task.QueryDeadline}
	require.NoError(t, job.AcknowledgeStage(ack))
	acked := job.Record()
	ctrl := gomock.NewController(t)
	jobs := mocks.NewMockGovernedHospitalJobUseCase(ctrl)
	jobs.EXPECT().Read(gomock.Any(), task.JobID).Return(&expired, nil)
	jobs.EXPECT().Begin(gomock.Any(), task).DoAndReturn(func(ctx context.Context, _ entities.GovernedHospitalTask) (*entities.GovernedHospitalJobRecord, error) {
		require.NoError(t, ctx.Err())
		return &expired, nil
	})
	nc := NewNodeClient(Config{NodeID: task.NodeID}, Dependencies{GovernedHospitalJobs: jobs, Now: time.Now}, nil, nil)
	send := make(chan *nodev1.NodeToCenter, 1)
	replies := make(chan governedReply, 1)
	replies <- governedReply{kind: governedReplyStage, eventID: stage.Binding.EventID, sequence: stage.Sequence, record: &acked}
	a := &governedActor{runtime: &governedRuntime{ctx: context.Background(), client: nc, send: send}, task: task, fingerprint: fingerprint, replies: replies}
	require.NoError(t, a.execute())
	packet := (<-send).GetJobStage()
	require.NotNil(t, packet)
	require.Equal(t, nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_APPROVAL_EXPIRED, packet.Phase)
	require.Empty(t, send, "no query task, release request or protected result may be emitted")
}
