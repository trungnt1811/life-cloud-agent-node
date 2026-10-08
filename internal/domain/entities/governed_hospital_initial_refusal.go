package entities

import (
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

// Refusal records prove a local guard decision, never execution or egress.
func NewGovernedHospitalRefusal(task GovernedHospitalTask, state *HospitalGovernance, now time.Time) (*GovernedHospitalJob, error) {
	if task.ValidateEnvelope() != nil || state == nil || !governedHospitalCurrentConnection(task, state) || state.ValidatePolicy() != nil || now.IsZero() || !now.Before(task.QueryDeadline) {
		return nil, ErrGovernedHospitalJobInvalid
	}
	phase, reason := initialGovernedHospitalRefusal(task, state, now)
	if phase == "" {
		return nil, ErrGovernedHospitalJobInvalid
	}
	fingerprint, err := task.Fingerprint()
	if err != nil {
		return nil, err
	}
	stage := initialGovernedHospitalStage(task, now)
	stage.Phase, stage.Reason = phase, reason
	return NewGovernedHospitalJobFromRecord(GovernedHospitalJobRecord{Task: task, Fingerprint: fingerprint, Revision: 1, Phase: phase, Reason: reason, Stages: []GovernedHospitalStage{stage}})
}

func governedHospitalCurrentConnection(task GovernedHospitalTask, state *HospitalGovernance) bool {
	return state != nil && state.record.NodeID == task.NodeID && state.record.Snapshot.Revision > 0 && state.record.Snapshot.SessionEpoch == task.SessionEpoch && state.record.AcknowledgedRevision > 0
}

func initialGovernedHospitalRefusal(task GovernedHospitalTask, state *HospitalGovernance, now time.Time) (string, string) {
	if phase, reason := governedHospitalLocalInvalidation(task, state); phase != "" {
		return phase, reason
	}
	if !state.Synchronized() || !now.Before(task.DispatchDeadline) {
		return "", ""
	}
	if state.CurrentAcceptance(task.TenantID, task.Permit.ID, task.Permit.Version, now) == nil {
		return GovernedHospitalPolicyDenied, types.GovernanceReasonAcceptanceRequired
	}
	if task.Validate(state, now, true) != nil {
		return GovernedHospitalPolicyDenied, types.GovernanceReasonScopeDenied
	}
	return "", ""
}

func governedHospitalLocalInvalidation(task GovernedHospitalTask, state *HospitalGovernance) (string, string) {
	for _, fence := range state.record.InvalidatedQueries {
		if fence.TenantID == task.TenantID && fence.QueryID == task.QueryID {
			if fence.Reason == types.GovernanceReasonQueryExpired {
				return GovernedHospitalTimedOut, fence.Reason
			}
			return GovernedHospitalCanceled, fence.Reason
		}
	}
	if state.record.Paused {
		return GovernedHospitalDeclined, types.GovernanceReasonHospitalPaused
	}
	if state.PermitInvalidated(task.TenantID, task.Permit.ID, task.Permit.Version) {
		reason := types.GovernanceReasonPermitRevoked
		for _, permit := range state.record.Snapshot.Permits {
			if permit.TenantID == task.TenantID && permit.ID == task.Permit.ID && permit.Version > task.Permit.Version {
				reason = types.GovernanceReasonPermitSuperseded
			}
		}
		return GovernedHospitalPolicyDenied, reason
	}
	return "", ""
}

func validGovernedHospitalInitialRefusal(r GovernedHospitalJobRecord) bool {
	if r.Task.ValidateEnvelope() != nil || !r.ExecutionStartedAt.IsZero() || !r.ExecutionDeadline.IsZero() || r.ExecutionDurationMilliseconds != 0 || r.ProtectedCount != nil || r.Binding != (GovernedHospitalBinding{}) || r.Grant != nil || r.Receipt != nil || r.Denial != nil || r.DeliveryState != "" || len(r.Stages) != 1 {
		return false
	}
	stage := r.Stages[0]
	if stage.Phase != r.Phase || stage.Reason != r.Reason || stage.ExecutionDurationMilliseconds != 0 || stage.Binding.ProtectedPayloadDigest != "" || stage.AcknowledgedExecutionDeadline != nil {
		return false
	}
	switch r.Phase {
	case GovernedHospitalDeclined:
		return r.Reason == types.GovernanceReasonHospitalPaused
	case GovernedHospitalCanceled:
		return r.Reason == types.GovernanceReasonQueryCanceled
	case GovernedHospitalTimedOut:
		return r.Reason == types.GovernanceReasonQueryExpired
	case GovernedHospitalPolicyDenied:
		return r.Reason == types.GovernanceReasonPermitRevoked || r.Reason == types.GovernanceReasonPermitSuperseded || r.Reason == types.GovernanceReasonAcceptanceRequired || r.Reason == types.GovernanceReasonScopeDenied
	default:
		return false
	}
}
