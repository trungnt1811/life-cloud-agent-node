package entities

import (
	"math"
	"time"

	"github.com/google/uuid"
)

func (j *GovernedHospitalJob) Terminate(phase, reason string, now time.Time) error {
	r := j.record
	if !governedHospitalTerminalPhase(phase) || phase == GovernedHospitalSucceeded || reason == "" || !ValidHospitalGovernanceReason(reason) || now.IsZero() || now.Before(r.ExecutionStartedAt) {
		return ErrGovernedHospitalJobInvalid
	}
	if governedHospitalTerminalPhase(r.Phase) {
		if r.Phase != phase || r.Reason != reason {
			return ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	if (r.Phase != GovernedHospitalExecuting && r.Phase != GovernedHospitalReady && r.Phase != GovernedHospitalWaitingApproval) || r.DeliveryState == GovernedHospitalSentUnconfirmed || r.Grant != nil || r.Receipt != nil || r.Denial != nil || r.Revision == math.MaxInt64 || len(r.Stages) >= 100 {
		return ErrGovernedHospitalJobInvalid
	}
	r.Phase, r.Reason = phase, reason
	r.Revision++
	j.record = cloneGovernedHospitalJob(r)
	j.appendTerminalStage(now)
	return nil
}

func (j *GovernedHospitalJob) appendTerminalStage(now time.Time) {
	r := &j.record
	last := r.Stages[len(r.Stages)-1]
	binding := last.Binding
	binding.EventID, binding.SessionEpoch = uuid.NewString(), r.Task.SessionEpoch
	r.Stages = append(r.Stages, GovernedHospitalStage{Binding: binding, Sequence: int64(len(r.Stages) + 1), Phase: r.Phase, Reason: r.Reason, OccurredAt: now.UTC(), ExecutionDurationMilliseconds: r.ExecutionDurationMilliseconds})
}

func governedHospitalTerminalPhase(phase string) bool {
	switch phase {
	case GovernedHospitalSucceeded, GovernedHospitalPolicyDenied, GovernedHospitalDeclined, GovernedHospitalApprovalExpired, GovernedHospitalTimedOut, GovernedHospitalFailed, GovernedHospitalCanceled:
		return true
	default:
		return false
	}
}

func validGovernedHospitalLocalTermination(r GovernedHospitalJobRecord) bool {
	if !governedHospitalTerminalPhase(r.Phase) || r.Phase == GovernedHospitalSucceeded || r.Receipt != nil || r.Denial != nil || r.Grant != nil || r.Reason == "" || !ValidHospitalGovernanceReason(r.Reason) || len(r.Stages) < 2 {
		return false
	}
	last := r.Stages[len(r.Stages)-1]
	return last.Phase == r.Phase && last.Reason == r.Reason
}
