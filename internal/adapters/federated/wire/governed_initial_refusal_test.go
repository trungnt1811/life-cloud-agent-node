package wire

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func TestGovernedHospitalInitialRefusalWireHasNoExecutionClockOrCount(t *testing.T) {
	packet, state, now := governedWireTask(t)
	task, err := GovernedHospitalTaskFromProto(packet)
	require.NoError(t, err)
	job, err := entities.NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	stage := job.PendingStages()[0]
	stage.Phase, stage.Reason = entities.GovernedHospitalDeclined, "HOSPITAL_PAUSED"
	refused, err := GovernedHospitalStageToProto(stage)
	require.NoError(t, err)
	require.Empty(t, refused.Binding.ProtectedPayloadDigest)
	require.Zero(t, refused.ExecutionDurationMilliseconds)
	ack := &nodev1.StageAck{JobId: task.JobID, EventId: stage.Binding.EventID, Sequence: 1, SessionEpoch: task.SessionEpoch, QueryDeadline: timestamppb.New(task.QueryDeadline)}
	decoded, err := GovernedHospitalStageAckFromProto(ack)
	require.NoError(t, err)
	require.True(t, decoded.ExecutionDeadline.IsZero())
	ack.Sequence = 2
	_, err = GovernedHospitalStageAckFromProto(ack)
	require.Error(t, err, "later stages must retain the committed execution clock")
	ack.Sequence, ack.EventId = 1, uuid.NewString()
	ack.QueryDeadline.ProtoReflect().SetUnknown([]byte{0x78, 1})
	_, err = GovernedHospitalStageAckFromProto(ack)
	require.Error(t, err, "optional execution clock must not relax timestamp validation")
}
