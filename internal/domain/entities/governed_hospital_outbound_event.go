package entities

import (
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

// Event records contain only disclosed output. A later intent never overwrites
// the old event binding or its protected preview.
type GovernedHospitalOutboundEvent struct {
	Binding              GovernedHospitalBinding
	ProtectedCount       types.GovernedProtectedCount
	DeliveryState        string
	Grant                *GovernedHospitalGrant
	Receipt              *GovernedHospitalReceipt
	Denial               *GovernedHospitalDenial
	CreatedAt, UpdatedAt time.Time
}

func (e GovernedHospitalOutboundEvent) MatchesReceipt(receipt GovernedHospitalReceipt) bool {
	if e.Validate() != nil || e.Receipt == nil {
		return false
	}
	prior := *e.Receipt
	prior.Outcome = normalizedHospitalReceiptOutcome(prior.Outcome)
	receipt.Outcome = normalizedHospitalReceiptOutcome(receipt.Outcome)
	return prior == receipt
}

func (e GovernedHospitalOutboundEvent) Validate() error {
	b := e.Binding
	digest, err := e.ProtectedCount.Digest()
	if err != nil || digest != b.ProtectedPayloadDigest || !validGovernedUUID(b.TenantID) || !validGovernedUUID(b.QueryID) || !validGovernedUUID(b.JobID) || !validGovernedUUID(b.EventID) || b.NodeID == "" || !hospitalHash(b.DefinitionHash) || !hospitalHash(b.PermitHash) || !hospitalHash(b.LocalPolicyHash) || !hospitalHash(b.AuthorizationSnapshotHash) || b.PermitID == "" || b.PermitVersion < 1 || b.LocalPolicyVersion < 1 || b.AcceptanceRevision < 1 || b.SessionEpoch == "" || e.CreatedAt.IsZero() || e.UpdatedAt.Before(e.CreatedAt) {
		return ErrGovernedHospitalJobInvalid
	}
	if e.DeliveryState == GovernedHospitalPrepared {
		if e.Grant != nil || e.Receipt != nil || e.Denial != nil {
			return ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	if e.Denial != nil {
		if e.DeliveryState != GovernedHospitalRejected || e.Grant != nil || e.Receipt != nil || e.Denial.Validate() != nil || e.Denial.Binding != b {
			return ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	if e.Grant == nil || !validGovernedUUID(e.Grant.ID) || e.Grant.Binding != b || e.Grant.GrantedAt.IsZero() || !e.Grant.ExpiresAt.After(e.Grant.GrantedAt) || e.Grant.ExpiresAt.After(e.Grant.GrantedAt.Add(10*time.Second)) {
		return ErrGovernedHospitalJobInvalid
	}
	if e.DeliveryState == GovernedHospitalSentUnconfirmed {
		if e.Receipt != nil {
			return ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	r := e.Receipt
	if r == nil || !validGovernedUUID(r.ID) || r.JobID != b.JobID || r.EventID != b.EventID || r.ProtectedPayloadDigest != digest || r.CommittedAt.IsZero() || !validHospitalReceiptOutcome(*r) || !validGovernedHospitalReceiptTime(*r, *e.Grant) {
		return ErrGovernedHospitalJobInvalid
	}
	if e.DeliveryState == GovernedHospitalReceived && normalizedHospitalReceiptOutcome(r.Outcome) == GovernedHospitalCommitted {
		return nil
	}
	if e.DeliveryState == GovernedHospitalRejected && r.Outcome == GovernedHospitalRejected {
		return nil
	}
	return ErrGovernedHospitalJobInvalid
}

func (e GovernedHospitalOutboundEvent) CanReplace(prior GovernedHospitalOutboundEvent) bool {
	if e.Binding != prior.Binding || e.ProtectedCount != prior.ProtectedCount || e.UpdatedAt.Before(prior.UpdatedAt) {
		return false
	}
	if prior.DeliveryState == GovernedHospitalPrepared {
		return e.DeliveryState == GovernedHospitalPrepared || e.DeliveryState == GovernedHospitalSentUnconfirmed || (e.DeliveryState == GovernedHospitalRejected && e.Denial != nil)
	}
	if prior.Denial != nil {
		return e.DeliveryState == prior.DeliveryState && e.Denial != nil && *e.Denial == *prior.Denial
	}
	if prior.Grant == nil || e.Grant == nil || *prior.Grant != *e.Grant {
		return false
	}
	if prior.DeliveryState == GovernedHospitalSentUnconfirmed {
		return e.DeliveryState != GovernedHospitalPrepared
	}
	return e.DeliveryState == prior.DeliveryState && prior.Receipt != nil && e.Receipt != nil && *prior.Receipt == *e.Receipt
}

func (j *GovernedHospitalJob) OutboundEvent(now time.Time) (GovernedHospitalOutboundEvent, error) {
	r := j.Record()
	if r.Binding.EventID == "" || r.ProtectedCount == nil {
		return GovernedHospitalOutboundEvent{}, ErrGovernedHospitalJobInvalid
	}
	return GovernedHospitalOutboundEvent{Binding: r.Binding, ProtectedCount: *r.ProtectedCount, DeliveryState: r.DeliveryState, Grant: r.Grant, Receipt: r.Receipt, Denial: r.Denial, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}
