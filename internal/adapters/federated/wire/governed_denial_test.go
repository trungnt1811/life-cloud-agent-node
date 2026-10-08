package wire

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func TestGovernedHospitalDenialAndTerminalWireRefuseFakeCommitment(t *testing.T) {
	packet, state, now := governedWireTask(t)
	task, err := GovernedHospitalTaskFromProto(packet)
	require.NoError(t, err)
	job, err := entities.NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.NoError(t, job.Prepare(9, state, now.Add(time.Second)))
	b, err := job.ReleaseIntent(state, now.Add(time.Second))
	require.NoError(t, err)
	p := &nodev1.ReleaseDenied{Binding: governedHospitalBindingToProto(b), Reason: nodev1.GovernanceReason_GOVERNANCE_REASON_PERMIT_REVOKED}
	d, err := GovernedHospitalDenialFromProto(p)
	require.NoError(t, err)
	require.NoError(t, job.DenyGrant(d, now.Add(2*time.Second)))
	r := job.Record()
	stage, err := GovernedHospitalStageToProto(r.Stages[len(r.Stages)-1])
	require.NoError(t, err)
	require.Equal(t, nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_POLICY_DENIED, stage.Phase)
	require.Equal(t, p.Reason, stage.Reason)
	_, err = GovernedHospitalResultToProto(r)
	require.Error(t, err)
	p.Reason = nodev1.GovernanceReason_GOVERNANCE_REASON_UNSPECIFIED
	_, err = GovernedHospitalDenialFromProto(p)
	require.Error(t, err)
	p.Reason = nodev1.GovernanceReason(99)
	_, err = GovernedHospitalDenialFromProto(p)
	require.Error(t, err)
	p.Reason = nodev1.GovernanceReason_GOVERNANCE_REASON_PERMIT_REVOKED
	p.ProtoReflect().SetUnknown([]byte{0x78, 1})
	_, err = GovernedHospitalDenialFromProto(p)
	require.Error(t, err)
}
