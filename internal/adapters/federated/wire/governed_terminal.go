package wire

import (
	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func governedHospitalTerminalStageEnums(s entities.GovernedHospitalStage) (nodev1.GovernedJobPhase, nodev1.GovernanceReason, bool) {
	var phase nodev1.GovernedJobPhase
	switch s.Phase {
	case entities.GovernedHospitalDeclined:
		phase = nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_DECLINED
	case entities.GovernedHospitalApprovalExpired:
		if s.Reason != "APPROVAL_DEADLINE_REACHED" {
			return 0, 0, false
		}
		phase = nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_APPROVAL_EXPIRED
	case entities.GovernedHospitalPolicyDenied:
		phase = nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_POLICY_DENIED
	case entities.GovernedHospitalTimedOut:
		phase = nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_TIMED_OUT
	case entities.GovernedHospitalFailed:
		phase = nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_FAILED
	case entities.GovernedHospitalCanceled:
		phase = nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_CANCELED
	default:
		return 0, 0, false
	}
	r, valid := nodev1.GovernanceReason_value["GOVERNANCE_REASON_"+s.Reason]
	if s.Phase == entities.GovernedHospitalDeclined && s.Reason == types.GovernanceReasonLocalDeclined {
		// Keep the persisted local reason while using the shared protocol enum.
		r, valid = int32(nodev1.GovernanceReason_GOVERNANCE_REASON_APPROVAL_DECLINED), true
	}
	validSequence := s.Sequence >= 2 || (s.Sequence == 1 && s.ExecutionDurationMilliseconds == 0 && s.Binding.ProtectedPayloadDigest == "")
	valid = valid && r > 0 && entities.ValidHospitalGovernanceReason(s.Reason) && validSequence && s.Binding.Validate(s.Binding.ProtectedPayloadDigest != "") == nil
	return phase, nodev1.GovernanceReason(r), valid
}
