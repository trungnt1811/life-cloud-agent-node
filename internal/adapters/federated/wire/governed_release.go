package wire

import (
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func GovernedHospitalStageToProto(s entities.GovernedHospitalStage) (*nodev1.JobStage, error) {
	invalid := entities.ErrGovernedHospitalJobInvalid
	var phase nodev1.GovernedJobPhase
	reason := nodev1.GovernanceReason_GOVERNANCE_REASON_UNSPECIFIED
	switch s.Phase {
	case entities.GovernedHospitalExecuting:
		if s.Sequence != 1 || s.ExecutionDurationMilliseconds != 0 || s.Binding.Validate(false) != nil {
			return nil, invalid
		}
		phase = nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_EXECUTING
	case entities.GovernedHospitalReady:
		if s.Sequence < 2 || s.Binding.Validate(true) != nil {
			return nil, invalid
		}
		phase = nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_READY_TO_RELEASE
	case entities.GovernedHospitalWaitingApproval:
		if s.Sequence < 2 || s.Binding.Validate(true) != nil {
			return nil, invalid
		}
		phase = nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_WAITING_APPROVAL
	default:
		var valid bool
		phase, reason, valid = governedHospitalTerminalStageEnums(s)
		if !valid {
			return nil, invalid
		}
	}
	at := timestamppb.New(s.OccurredAt)
	if (reason == nodev1.GovernanceReason_GOVERNANCE_REASON_UNSPECIFIED && s.Reason != "") || s.OccurredAt.IsZero() || !validHospitalTimestamp(at) || s.ExecutionDurationMilliseconds > 30000 {
		return nil, invalid
	}
	return &nodev1.JobStage{Binding: governedHospitalBindingToProto(s.Binding), Sequence: s.Sequence, Phase: phase, Reason: reason, OccurredAt: at, ExecutionDurationMilliseconds: s.ExecutionDurationMilliseconds}, nil
}

func GovernedHospitalStageAckFromProto(p *nodev1.StageAck) (entities.GovernedHospitalStageAck, error) {
	if p == nil || governedPacketUnknown(p.ProtoReflect()) || !governedPacketUUID(p.JobId) || !governedPacketUUID(p.EventId) || p.Sequence < 1 || p.SessionEpoch == "" || len(p.SessionEpoch) > 128 || !validHospitalTimestamp(p.QueryDeadline) {
		return entities.GovernedHospitalStageAck{}, entities.ErrGovernedHospitalJobInvalid
	}
	ack := entities.GovernedHospitalStageAck{JobID: p.JobId, EventID: p.EventId, Sequence: p.Sequence, SessionEpoch: p.SessionEpoch, QueryDeadline: p.QueryDeadline.AsTime()}
	if p.ApprovalDeadline != nil {
		if !validHospitalTimestamp(p.ApprovalDeadline) {
			return entities.GovernedHospitalStageAck{}, entities.ErrGovernedHospitalJobInvalid
		}
		ack.ApprovalDeadline = p.ApprovalDeadline.AsTime()
	}
	if p.ExecutionDeadline == nil {
		if p.Sequence != 1 {
			return entities.GovernedHospitalStageAck{}, entities.ErrGovernedHospitalJobInvalid
		}
		return ack, nil
	}
	if !validHospitalTimestamp(p.ExecutionDeadline) || p.ExecutionDeadline.AsTime().After(ack.QueryDeadline) {
		return entities.GovernedHospitalStageAck{}, entities.ErrGovernedHospitalJobInvalid
	}
	ack.ExecutionDeadline = p.ExecutionDeadline.AsTime()
	return ack, nil
}

func GovernedHospitalReleaseRequestToProto(b entities.GovernedHospitalBinding) (*nodev1.ReleaseRequest, error) {
	if err := b.Validate(true); err != nil {
		return nil, err
	}
	return &nodev1.ReleaseRequest{Binding: governedHospitalBindingToProto(b)}, nil
}

func GovernedHospitalReceiptLookupToProto(r entities.GovernedHospitalJobRecord) (*nodev1.ReceiptLookup, error) {
	if _, err := entities.NewGovernedHospitalJobFromRecord(r); err != nil {
		return nil, err
	}
	if r.DeliveryState != entities.GovernedHospitalSentUnconfirmed || r.Grant == nil || r.Grant.Binding != r.Binding {
		return nil, entities.ErrGovernedHospitalJobInvalid
	}
	return &nodev1.ReceiptLookup{GrantId: r.Grant.ID, Binding: governedHospitalBindingToProto(r.Binding)}, nil
}

func GovernedHospitalOutboundLookupToProto(e entities.GovernedHospitalOutboundEvent) (*nodev1.ReceiptLookup, error) {
	if e.Validate() != nil || e.DeliveryState != entities.GovernedHospitalSentUnconfirmed || e.Grant == nil {
		return nil, entities.ErrGovernedHospitalJobInvalid
	}
	return &nodev1.ReceiptLookup{GrantId: e.Grant.ID, Binding: governedHospitalBindingToProto(e.Binding)}, nil
}

func GovernedHospitalGrantFromProto(p *nodev1.ReleaseGrant) (entities.GovernedHospitalGrant, error) {
	if p == nil || governedPacketUnknown(p.ProtoReflect()) || !governedPacketUUID(p.GrantId) || !validHospitalTimestamp(p.GrantedAt) || !validHospitalTimestamp(p.ExpiresAt) || !p.ExpiresAt.AsTime().After(p.GrantedAt.AsTime()) || p.ExpiresAt.AsTime().After(p.GrantedAt.AsTime().Add(10*time.Second)) {
		return entities.GovernedHospitalGrant{}, entities.ErrGovernedHospitalJobInvalid
	}
	binding, err := governedHospitalBindingFromProto(p.Binding)
	if err != nil {
		return entities.GovernedHospitalGrant{}, err
	}
	return entities.GovernedHospitalGrant{ID: p.GrantId, Binding: binding, GrantedAt: p.GrantedAt.AsTime(), ExpiresAt: p.ExpiresAt.AsTime()}, nil
}

func GovernedHospitalReceiptFromProto(p *nodev1.ResultReceipt) (entities.GovernedHospitalReceipt, error) {
	if p == nil || governedPacketUnknown(p.ProtoReflect()) || !validHospitalTimestamp(p.CommittedAt) {
		return entities.GovernedHospitalReceipt{}, entities.ErrGovernedHospitalJobInvalid
	}
	reason, valid := governedHospitalReasonFromProto(p.Reason)
	if !valid {
		return entities.GovernedHospitalReceipt{}, entities.ErrGovernedHospitalJobInvalid
	}
	var outcome string
	switch p.Outcome {
	case nodev1.ResultReceiptOutcome_RESULT_RECEIPT_OUTCOME_COMMITTED:
		outcome = entities.GovernedHospitalCommitted
	case nodev1.ResultReceiptOutcome_RESULT_RECEIPT_OUTCOME_DUPLICATE_COMMITTED:
		outcome = "DUPLICATE_COMMITTED"
	case nodev1.ResultReceiptOutcome_RESULT_RECEIPT_OUTCOME_REJECTED:
		outcome = entities.GovernedHospitalRejected
	default:
		return entities.GovernedHospitalReceipt{}, entities.ErrGovernedHospitalJobInvalid
	}
	r := entities.GovernedHospitalReceipt{ID: p.ReceiptId, JobID: p.JobId, EventID: p.EventId, ProtectedPayloadDigest: p.ProtectedPayloadDigest, Outcome: outcome, Reason: reason, CommittedAt: p.CommittedAt.AsTime()}
	return r, r.ValidateProof()
}

func GovernedHospitalResultToProto(r entities.GovernedHospitalJobRecord) (*nodev1.GovernedQueryResult, error) {
	if _, err := entities.NewGovernedHospitalJobFromRecord(r); err != nil {
		return nil, err
	}
	if r.DeliveryState != entities.GovernedHospitalSentUnconfirmed || r.Grant == nil || r.ProtectedCount == nil {
		return nil, entities.ErrGovernedHospitalJobInvalid
	}
	count := *r.ProtectedCount
	payload := &nodev1.ProtectedCohortPayload{EffectiveK: count.EffectiveK()}
	if value, exact := count.ExactValue(); exact {
		payload.Count = &nodev1.ProtectedCohortPayload_ExactCount{ExactCount: value}
	} else {
		lower, upper, _ := count.Bounds()
		payload.Count = &nodev1.ProtectedCohortPayload_SuppressedCount{SuppressedCount: &nodev1.PositiveSuppressedCount{LowerBound: lower, UpperBound: upper}}
	}
	return &nodev1.GovernedQueryResult{Profile: types.GovernedCohortProfile, MessageSchemaVersion: 1, GrantId: r.Grant.ID, Binding: governedHospitalBindingToProto(r.Binding), ProtectedPayload: payload}, nil
}

func governedPacketUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value
}
