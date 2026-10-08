package entities

import (
	"math"
	"time"
)

func (d GovernedHospitalDenial) Validate() error {
	if d.Binding.Validate(true) != nil || d.Reason == "" || !ValidHospitalGovernanceReason(d.Reason) {
		return ErrGovernedHospitalJobInvalid
	}
	return nil
}

func (j *GovernedHospitalJob) DenyGrant(d GovernedHospitalDenial, now time.Time) error {
	r := j.record
	if d.Validate() != nil || d.Binding != r.Binding || now.IsZero() {
		return ErrGovernedHospitalJobInvalid
	}
	if r.Denial != nil {
		if *r.Denial != d {
			return ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	// A grant refusal is not a result receipt and cannot resolve prior egress.
	if r.Phase != GovernedHospitalReady || r.DeliveryState != GovernedHospitalPrepared || r.Grant != nil || r.Receipt != nil || r.Revision == math.MaxInt64 || now.Before(r.ExecutionStartedAt) || (!retryableGovernedHospitalReceipt(d.Reason) && len(r.Stages) >= 100) {
		return ErrGovernedHospitalJobInvalid
	}
	r.Denial = &d
	r.DeliveryState, r.Reason = GovernedHospitalRejected, d.Reason
	if !retryableGovernedHospitalReceipt(d.Reason) {
		r.Phase = GovernedHospitalPolicyDenied
	}
	r.Revision++
	j.record = cloneGovernedHospitalJob(r)
	if !retryableGovernedHospitalReceipt(d.Reason) {
		j.appendTerminalStage(now)
	}
	return nil
}

func (e GovernedHospitalOutboundEvent) MatchesDenial(d GovernedHospitalDenial) bool {
	return e.Validate() == nil && e.Denial != nil && *e.Denial == d
}
