package dto

import (
	"strconv"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

type HospitalProtectedCountDTO struct {
	Kind       string  `json:"kind" enums:"EXACT,SUPPRESSED"`
	Value      *string `json:"value"`
	LowerBound *string `json:"lower_bound"`
	UpperBound *string `json:"upper_bound"`
}

type HospitalProtectedPayloadDTO struct {
	Count      HospitalProtectedCountDTO `json:"count"`
	EffectiveK string                    `json:"effective_k"`
}

type HospitalOutboundEventDTO struct {
	EventID                string                      `json:"event_id"`
	JobID                  string                      `json:"job_id"`
	QueryID                string                      `json:"query_id"`
	TenantID               string                      `json:"tenant_id"`
	PermitID               string                      `json:"permit_id"`
	PermitVersion          int64                       `json:"permit_version"`
	LocalPolicyVersion     int64                       `json:"local_policy_version"`
	DeliveryState          string                      `json:"delivery_state"`
	ProtectedPayload       HospitalProtectedPayloadDTO `json:"protected_payload"`
	ProtectedPayloadDigest string                      `json:"protected_payload_digest"`
	ReceiptID              *string                     `json:"receipt_id"`
	CommittedAt            *time.Time                  `json:"committed_at"`
	CreatedAt              time.Time                   `json:"created_at"`
	UpdatedAt              time.Time                   `json:"updated_at"`
}

type HospitalOutboundEventListDTO struct {
	Items      []HospitalOutboundEventDTO `json:"items"`
	TotalCount int64                      `json:"total_count"`
	Page       int                        `json:"page"`
	PageSize   int                        `json:"page_size"`
}

func NewHospitalOutboundEventListDTO(events []entities.GovernedHospitalOutboundEvent, total int64, page, size int) HospitalOutboundEventListDTO {
	items := make([]HospitalOutboundEventDTO, 0, len(events))
	for _, event := range events {
		b := event.Binding
		payload := HospitalProtectedPayloadDTO{EffectiveK: strconv.FormatUint(event.ProtectedCount.EffectiveK(), 10), Count: HospitalProtectedCountDTO{Kind: "SUPPRESSED"}}
		if value, exact := event.ProtectedCount.ExactValue(); exact {
			v := strconv.FormatUint(value, 10)
			payload.Count.Kind, payload.Count.Value = "EXACT", &v
		} else {
			lower, upper, _ := event.ProtectedCount.Bounds()
			l, u := strconv.FormatUint(lower, 10), strconv.FormatUint(upper, 10)
			payload.Count.LowerBound, payload.Count.UpperBound = &l, &u
		}
		dto := HospitalOutboundEventDTO{EventID: b.EventID, JobID: b.JobID, QueryID: b.QueryID, TenantID: b.TenantID, PermitID: b.PermitID, PermitVersion: b.PermitVersion, LocalPolicyVersion: b.LocalPolicyVersion, DeliveryState: event.DeliveryState, ProtectedPayload: payload, ProtectedPayloadDigest: b.ProtectedPayloadDigest, CreatedAt: event.CreatedAt, UpdatedAt: event.UpdatedAt}
		if event.Receipt != nil {
			id, at := event.Receipt.ID, event.Receipt.CommittedAt
			dto.ReceiptID, dto.CommittedAt = &id, &at
		}
		items = append(items, dto)
	}
	return HospitalOutboundEventListDTO{Items: items, TotalCount: total, Page: page, PageSize: size}
}

type HospitalApprovalDTO struct {
	JobID             string                      `json:"job_id"`
	QueryID           string                      `json:"query_id"`
	DefinitionHash    string                      `json:"definition_hash"`
	PermitID          string                      `json:"permit_id"`
	PermitVersion     int64                       `json:"permit_version"`
	PolicyVersion     int64                       `json:"local_policy_version"`
	PolicyHash        string                      `json:"local_policy_hash"`
	ProtectedPayload  HospitalProtectedPayloadDTO `json:"protected_payload"`
	ProtectedDigest   string                      `json:"protected_payload_digest"`
	ApprovalDeadline  time.Time                   `json:"approval_deadline"`
	Revision          int64                       `json:"revision"`
	Phase             string                      `json:"phase"`
	Decision          *string                     `json:"decision,omitempty"`
	DecisionActor     *string                     `json:"decision_actor,omitempty"`
	DecisionTimestamp *time.Time                  `json:"decision_at,omitempty"`
}

type HospitalApprovalListDTO struct {
	Items      []HospitalApprovalDTO `json:"items"`
	TotalCount int64                 `json:"total_count"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
}

type HospitalApprovalRequest struct {
	ExpectedRevision int64 `json:"expected_revision" binding:"required,min=1"`
}

func NewHospitalApprovalDTO(record entities.GovernedHospitalJobRecord) HospitalApprovalDTO {
	payload := HospitalProtectedPayloadDTO{Count: HospitalProtectedCountDTO{Kind: "UNAVAILABLE"}}
	digest := ""
	if record.ProtectedCount != nil {
		count := *record.ProtectedCount
		payload.EffectiveK = strconv.FormatUint(count.EffectiveK(), 10)
		if value, exact := count.ExactValue(); exact {
			v := strconv.FormatUint(value, 10)
			payload.Count.Kind, payload.Count.Value = "EXACT", &v
		} else {
			lower, upper, _ := count.Bounds()
			l, u := strconv.FormatUint(lower, 10), strconv.FormatUint(upper, 10)
			payload.Count = HospitalProtectedCountDTO{Kind: "SUPPRESSED", LowerBound: &l, UpperBound: &u}
		}
		digest, _ = count.Digest()
	}
	dto := HospitalApprovalDTO{JobID: record.Task.JobID, QueryID: record.Task.QueryID, DefinitionHash: record.Task.DefinitionHash, PermitID: record.Task.Permit.ID, PermitVersion: record.Task.Permit.Version, PolicyVersion: record.Task.LocalPolicyVersion, PolicyHash: record.Task.LocalPolicyHash, ProtectedPayload: payload, ProtectedDigest: digest, ApprovalDeadline: record.ApprovalDeadline, Revision: record.Revision, Phase: record.Phase}
	if record.Approval != nil {
		decision, actor, at := record.Approval.Decision, record.Approval.Actor, record.Approval.DecidedAt
		dto.Decision, dto.DecisionActor, dto.DecisionTimestamp = &decision, &actor, &at
	}
	return dto
}

func NewHospitalApprovalListDTO(records []entities.GovernedHospitalJobRecord, total int64, page, size int) HospitalApprovalListDTO {
	items := make([]HospitalApprovalDTO, 0, len(records))
	for _, record := range records {
		items = append(items, NewHospitalApprovalDTO(record))
	}
	return HospitalApprovalListDTO{Items: items, TotalCount: total, Page: page, PageSize: size}
}
