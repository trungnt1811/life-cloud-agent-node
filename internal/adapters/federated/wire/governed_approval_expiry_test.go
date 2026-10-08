package wire

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func TestGovernedHospitalApprovalExpiryHasCanonicalCountFreeWireProof(t *testing.T) {
	packet, state, now := governedWireTask(t)
	policy := state.Record().Policy
	policy.ReleaseMode = entities.HospitalReleaseManual
	require.NoError(t, state.UpdatePolicy(policy, policy.Version))
	require.NoError(t, state.Acknowledge(packet.SessionEpoch, state.Record().Snapshot.Revision, state.Record().LocalRevision))
	packet.ReleaseMode = nodev1.ReleaseMode_RELEASE_MODE_MANUAL
	packet.LocalPolicyVersion, packet.LocalPolicyHash = state.Record().Policy.Version, state.Record().Policy.PolicyHash
	task, err := GovernedHospitalTaskFromProto(packet)
	require.NoError(t, err)
	job, err := entities.NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	first := job.PendingStages()[0]
	ack := entities.GovernedHospitalStageAck{JobID: task.JobID, EventID: first.Binding.EventID, Sequence: 1, SessionEpoch: task.SessionEpoch, ExecutionDeadline: now.Add(30 * time.Second), QueryDeadline: task.QueryDeadline}
	require.NoError(t, job.AcknowledgeStage(ack))
	require.NoError(t, job.Prepare(9, state, now.Add(time.Second)))
	waiting := job.PendingStages()[0]
	ack.EventID, ack.Sequence, ack.ApprovalDeadline = waiting.Binding.EventID, 2, now.Add(60*time.Second)
	require.NoError(t, job.AcknowledgeStage(ack))
	require.NoError(t, job.ExpireApproval(ack.ApprovalDeadline))
	stage, err := GovernedHospitalStageToProto(job.PendingStages()[0])
	require.NoError(t, err)
	require.Equal(t, nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_APPROVAL_EXPIRED, stage.Phase)
	require.EqualValues(t, 17, stage.Reason, "append the shared reason without renumbering existing wire enums")
	encoded, err := proto.Marshal(stage)
	require.NoError(t, err)
	var decoded nodev1.JobStage
	require.NoError(t, proto.Unmarshal(encoded, &decoded))
	require.True(t, proto.Equal(stage, &decoded))
	require.EqualValues(t, 3, decoded.Sequence)
	require.Equal(t, waiting.Binding.ProtectedPayloadDigest, decoded.Binding.ProtectedPayloadDigest)
	require.Nil(t, job.Record().Grant)
	require.Nil(t, job.Record().Receipt)
}
