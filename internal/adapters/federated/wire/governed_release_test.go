package wire

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func TestGovernedHospitalReleaseWireUsesOnlyJournalAndProtectedLedger(t *testing.T) {
	packet, state, now := governedWireTask(t)
	task, err := GovernedHospitalTaskFromProto(packet)
	require.NoError(t, err)
	job, err := entities.NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	stage, err := GovernedHospitalStageToProto(job.PendingStages()[0])
	require.NoError(t, err)
	require.Equal(t, nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_EXECUTING, stage.Phase)
	require.Empty(t, stage.Binding.ProtectedPayloadDigest)
	ackPacket := &nodev1.StageAck{JobId: task.JobID, EventId: stage.Binding.EventId, Sequence: stage.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: timestamppb.New(now.Add(30 * time.Second)), QueryDeadline: packet.QueryDeadline}
	ack, err := GovernedHospitalStageAckFromProto(ackPacket)
	require.NoError(t, err)
	require.NoError(t, job.AcknowledgeStage(ack))
	require.NoError(t, job.Prepare(9, state, now.Add(time.Second)))
	binding, err := job.ReleaseIntent(state, now.Add(time.Second))
	require.NoError(t, err)
	ready, err := GovernedHospitalStageToProto(job.PendingStages()[0])
	require.NoError(t, err)
	require.Equal(t, nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_READY_TO_RELEASE, ready.Phase)
	request, err := GovernedHospitalReleaseRequestToProto(binding)
	require.NoError(t, err)
	require.Equal(t, ready.Binding, request.Binding)
	grantPacket := &nodev1.ReleaseGrant{GrantId: uuid.NewString(), Binding: request.Binding, GrantedAt: timestamppb.New(now.Add(time.Second)), ExpiresAt: timestamppb.New(now.Add(11 * time.Second))}
	grant, err := GovernedHospitalGrantFromProto(grantPacket)
	require.NoError(t, err)
	_, err = GovernedHospitalResultToProto(job.Record())
	require.Error(t, err, "prepared ledger without committed send intent is not egress permission")
	require.NoError(t, job.AuthorizeSend(grant, state, now.Add(2*time.Second)))
	result, err := GovernedHospitalResultToProto(job.Record())
	require.NoError(t, err)
	require.Equal(t, grant.ID, result.GrantId)
	require.Equal(t, binding.ProtectedPayloadDigest, result.Binding.ProtectedPayloadDigest)
	require.IsType(t, &nodev1.ProtectedCohortPayload_SuppressedCount{}, result.ProtectedPayload.Count)
	require.EqualValues(t, 1, result.ProtectedPayload.GetSuppressedCount().LowerBound)
	require.EqualValues(t, 9, result.ProtectedPayload.GetSuppressedCount().UpperBound)
	receipt, err := GovernedHospitalReceiptFromProto(&nodev1.ResultReceipt{ReceiptId: uuid.NewString(), JobId: task.JobID, EventId: binding.EventID, ProtectedPayloadDigest: binding.ProtectedPayloadDigest, Outcome: nodev1.ResultReceiptOutcome_RESULT_RECEIPT_OUTCOME_COMMITTED, CommittedAt: timestamppb.New(now.Add(3 * time.Second))})
	require.NoError(t, err)
	require.NoError(t, job.Receive(receipt))
	_, err = GovernedHospitalResultToProto(job.Record())
	require.Error(t, err, "a historical committed receipt is not fresh send permission")
}

func TestGovernedHospitalReleaseWireRejectsMissingOrUnknownProof(t *testing.T) {
	for _, packet := range []*nodev1.ResultReceipt{
		nil,
		{ReceiptId: uuid.NewString()},
		{ReceiptId: uuid.NewString(), JobId: uuid.NewString(), EventId: uuid.NewString(), Outcome: nodev1.ResultReceiptOutcome(99), CommittedAt: timestamppb.Now()},
	} {
		_, err := GovernedHospitalReceiptFromProto(packet)
		require.Error(t, err)
	}
	_, err := GovernedHospitalGrantFromProto(nil)
	require.Error(t, err)
	_, err = GovernedHospitalStageAckFromProto(nil)
	require.Error(t, err)
	packet, state, now := governedWireTask(t)
	task, err := GovernedHospitalTaskFromProto(packet)
	require.NoError(t, err)
	job, err := entities.NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	stage := job.PendingStages()[0]
	bad := stage
	bad.Phase = entities.GovernedHospitalSucceeded
	_, err = GovernedHospitalStageToProto(bad)
	require.Error(t, err, "Node cannot declare CP result commitment")
	bad = stage
	bad.Binding.EventID = "malformed"
	_, err = GovernedHospitalStageToProto(bad)
	require.Error(t, err)
	ack := &nodev1.StageAck{JobId: task.JobID, EventId: stage.Binding.EventID, Sequence: 1, SessionEpoch: task.SessionEpoch, ExecutionDeadline: timestamppb.New(now.Add(30 * time.Second)), QueryDeadline: packet.QueryDeadline, ApprovalDeadline: timestamppb.New(now.Add(time.Hour))}
	decoded, err := GovernedHospitalStageAckFromProto(ack)
	require.NoError(t, err, "stage authority is checked against the persisted stage by the entity")
	require.Error(t, job.AcknowledgeStage(decoded), "an AUTO execution stage cannot receive an approval deadline")
	ack.ApprovalDeadline = nil
	ack.ExecutionDeadline.ProtoReflect().SetUnknown([]byte{0x78, 1})
	_, err = GovernedHospitalStageAckFromProto(ack)
	require.Error(t, err)
}

func TestGovernedHospitalLocalDeclineHasCanonicalWireReason(t *testing.T) {
	packet, state, now := governedWireTask(t)
	policy := state.Record().Policy
	policy.ReleaseMode = entities.HospitalReleaseManual
	require.NoError(t, state.UpdatePolicy(policy, policy.Version))
	require.NoError(t, state.Acknowledge(packet.SessionEpoch, state.Record().Snapshot.Revision, state.Record().LocalRevision))
	policy = state.Record().Policy
	packet.ReleaseMode = nodev1.ReleaseMode_RELEASE_MODE_MANUAL
	packet.LocalPolicyVersion, packet.LocalPolicyHash = policy.Version, policy.PolicyHash
	task, err := GovernedHospitalTaskFromProto(packet)
	require.NoError(t, err)
	job, err := entities.NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	first := job.PendingStages()[0]
	ack := entities.GovernedHospitalStageAck{JobID: task.JobID, EventID: first.Binding.EventID, Sequence: first.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: now.Add(30 * time.Second), QueryDeadline: task.QueryDeadline}
	require.NoError(t, job.AcknowledgeStage(ack))
	require.NoError(t, job.Prepare(9, state, now.Add(time.Second)))
	waiting := job.PendingStages()[0]
	ack.EventID, ack.Sequence, ack.ApprovalDeadline = waiting.Binding.EventID, waiting.Sequence, now.Add(60*time.Second)
	require.NoError(t, job.AcknowledgeStage(ack))
	require.NoError(t, job.Decline(job.Record().Revision, uuid.NewString(), "hospital-admin", state, now.Add(2*time.Second)))
	stage, err := GovernedHospitalStageToProto(job.PendingStages()[0])
	require.NoError(t, err, "a durable human decline must be publishable without reconnecting forever")
	require.Equal(t, nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_DECLINED, stage.Phase)
	require.Equal(t, nodev1.GovernanceReason_GOVERNANCE_REASON_APPROVAL_DECLINED, stage.Reason)
	require.EqualValues(t, 3, stage.Sequence)
	require.Nil(t, job.Record().Grant)
	require.Nil(t, job.Record().Receipt)
}
