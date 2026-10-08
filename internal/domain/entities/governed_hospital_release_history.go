package entities

func retryableGovernedHospitalReceipt(reason string) bool {
	return reason == "GRANT_EXPIRED" || reason == "SESSION_STALE" || reason == "LOCAL_POLICY_CHANGED"
}

func validGovernedHospitalReceiptTime(receipt GovernedHospitalReceipt, grant GovernedHospitalGrant) bool {
	return !receipt.CommittedAt.Before(grant.GrantedAt) && (receipt.Outcome == GovernedHospitalRejected || receipt.CommittedAt.Before(grant.ExpiresAt))
}

func validateGovernedHospitalReleaseHistory(r GovernedHospitalJobRecord) error {
	if validGovernedHospitalInitialRefusal(r) {
		return nil
	}
	if validGovernedHospitalLocalTermination(r) {
		if r.Binding.EventID != "" {
			if r.ProtectedCount == nil || r.DeliveryState != GovernedHospitalPrepared {
				return ErrGovernedHospitalJobInvalid
			}
			return (GovernedHospitalOutboundEvent{Binding: r.Binding, ProtectedCount: *r.ProtectedCount, DeliveryState: r.DeliveryState, CreatedAt: r.ExecutionStartedAt, UpdatedAt: r.ExecutionStartedAt}).Validate()
		}
		if r.Binding != (GovernedHospitalBinding{}) || (r.ProtectedCount == nil && r.DeliveryState != "") || (r.ProtectedCount != nil && r.DeliveryState != GovernedHospitalPrepared) {
			return ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	if r.Phase == GovernedHospitalExecuting {
		if r.ProtectedCount != nil || r.Binding != (GovernedHospitalBinding{}) || r.Grant != nil || r.Receipt != nil || r.Denial != nil || r.DeliveryState != "" || r.Reason != "" {
			return ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	if r.ProtectedCount == nil {
		return ErrGovernedHospitalJobInvalid
	}
	if r.Binding.EventID == "" {
		if r.Binding != (GovernedHospitalBinding{}) || (r.Phase != GovernedHospitalReady && r.Phase != GovernedHospitalWaitingApproval) || r.DeliveryState != GovernedHospitalPrepared || r.Reason != "" || r.Grant != nil || r.Receipt != nil || r.Denial != nil {
			return ErrGovernedHospitalJobInvalid
		}
		if r.Phase == GovernedHospitalWaitingApproval && (r.Task.ReleaseMode != HospitalReleaseManual || r.Approval != nil) {
			return ErrGovernedHospitalJobInvalid
		}
		if r.Phase == GovernedHospitalReady && r.Task.ReleaseMode == HospitalReleaseManual && (r.Approval == nil || r.Approval.Decision != "APPROVED") {
			return ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	event := GovernedHospitalOutboundEvent{Binding: r.Binding, ProtectedCount: *r.ProtectedCount, DeliveryState: r.DeliveryState, Grant: r.Grant, Receipt: r.Receipt, Denial: r.Denial, CreatedAt: r.ExecutionStartedAt, UpdatedAt: r.ExecutionStartedAt}
	if err := event.Validate(); err != nil {
		return err
	}
	switch r.DeliveryState {
	case GovernedHospitalPrepared, GovernedHospitalSentUnconfirmed:
		if r.Phase != GovernedHospitalReady || r.Reason != "" {
			return ErrGovernedHospitalJobInvalid
		}
		if r.Task.ReleaseMode == HospitalReleaseManual && (r.Approval == nil || r.Approval.Decision != "APPROVED") {
			return ErrGovernedHospitalJobInvalid
		}
	case GovernedHospitalReceived:
		if r.Phase != GovernedHospitalSucceeded || r.Reason != "" {
			return ErrGovernedHospitalJobInvalid
		}
	case GovernedHospitalRejected:
		expected := GovernedHospitalPolicyDenied
		reason := ""
		if r.Receipt != nil {
			reason = r.Receipt.Reason
		} else if r.Denial != nil {
			reason = r.Denial.Reason
		}
		if retryableGovernedHospitalReceipt(reason) {
			expected = GovernedHospitalReady
		}
		if r.Phase != expected || r.Reason != reason {
			return ErrGovernedHospitalJobInvalid
		}
	default:
		return ErrGovernedHospitalJobInvalid
	}
	return nil
}
