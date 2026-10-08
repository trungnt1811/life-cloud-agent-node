package dto

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

type HospitalPolicyRequest struct {
	ExpectedRevision  *int64   `json:"expected_revision" binding:"required"`
	LocalK            string   `json:"local_k" binding:"required"`
	ReleaseMode       string   `json:"release_mode" binding:"required" enums:"AUTO,MANUAL"`
	EnabledFieldCodes []string `json:"enabled_field_codes"`
	FilterFieldCodes  []string `json:"filter_field_codes"`
	PanelFieldCodes   []string `json:"panel_field_codes"`
}

func (r HospitalPolicyRequest) Policy() (entities.HospitalPolicyRecord, error) {
	k, err := strconv.ParseUint(r.LocalK, 10, 64)
	if err != nil || k > math.MaxInt64 || strconv.FormatUint(k, 10) != r.LocalK {
		return entities.HospitalPolicyRecord{}, errors.New("invalid local_k decimal string")
	}
	return entities.HospitalPolicyRecord{
		LocalK: k, ReleaseMode: r.ReleaseMode, EnabledFieldCodes: r.EnabledFieldCodes,
		FilterFieldCodes: r.FilterFieldCodes, PanelFieldCodes: r.PanelFieldCodes,
	}, nil
}

type HospitalAcceptanceRequest struct {
	PermitVersion     int64     `json:"permit_version" binding:"required"`
	PermitHash        string    `json:"permit_hash" binding:"required"`
	Decision          string    `json:"decision" binding:"required" enums:"ACCEPTED,DECLINED"`
	AllowedFieldCodes []string  `json:"allowed_field_codes"`
	ExpiresAt         time.Time `json:"expires_at" binding:"required"`
	ExpectedRevision  *int64    `json:"expected_revision" binding:"required"`
}

func (r HospitalAcceptanceRequest) Change(id string) entities.HospitalAcceptanceChange {
	var expected int64
	if r.ExpectedRevision != nil {
		expected = *r.ExpectedRevision
	}
	return entities.HospitalAcceptanceChange{
		PermitID: id, PermitVersion: r.PermitVersion, PermitHash: r.PermitHash,
		Decision: r.Decision, AllowedFieldCodes: r.AllowedFieldCodes, ExpiresAt: r.ExpiresAt, ExpectedRevision: expected,
	}
}

type HospitalSyncDTO struct {
	LocalRevision        int64 `json:"local_revision"`
	AcknowledgedRevision int64 `json:"acknowledged_revision"`
	SnapshotRevision     int64 `json:"snapshot_revision"`
	CPSynchronized       bool  `json:"cp_synchronized"`
}

func hospitalSyncDTO(r entities.HospitalGovernanceRecord) HospitalSyncDTO {
	return HospitalSyncDTO{
		LocalRevision: r.LocalRevision, AcknowledgedRevision: r.AcknowledgedRevision,
		SnapshotRevision: r.Snapshot.Revision, CPSynchronized: r.Synchronized(),
	}
}

type HospitalPolicyDTO struct {
	HospitalSyncDTO
	Version           int64    `json:"version"`
	PolicyHash        string   `json:"policy_hash"`
	LocalK            string   `json:"local_k"`
	ReleaseMode       string   `json:"release_mode"`
	EnabledFieldCodes []string `json:"enabled_field_codes"`
	FilterFieldCodes  []string `json:"filter_field_codes"`
	PanelFieldCodes   []string `json:"panel_field_codes"`
}

func NewHospitalPolicyDTO(r entities.HospitalGovernanceRecord) HospitalPolicyDTO {
	p := r.Policy
	return HospitalPolicyDTO{
		HospitalSyncDTO: hospitalSyncDTO(r), Version: p.Version, PolicyHash: p.PolicyHash,
		LocalK: strconv.FormatUint(p.LocalK, 10), ReleaseMode: p.ReleaseMode,
		EnabledFieldCodes: p.EnabledFieldCodes, FilterFieldCodes: p.FilterFieldCodes, PanelFieldCodes: p.PanelFieldCodes,
	}
}

type HospitalAcceptanceDTO struct {
	HospitalSyncDTO
	TenantID          string    `json:"tenant_id"`
	PermitID          string    `json:"permit_id"`
	PermitVersion     int64     `json:"permit_version"`
	PermitHash        string    `json:"permit_hash"`
	Revision          int64     `json:"revision"`
	Decision          string    `json:"decision"`
	AllowedFieldCodes []string  `json:"allowed_field_codes"`
	ExpiresAt         time.Time `json:"expires_at"`
}

func NewHospitalAcceptanceDTO(r entities.HospitalGovernanceRecord, id string, version int64, permitHash string) *HospitalAcceptanceDTO {
	for _, a := range r.Acceptances {
		if a.PermitID == id && a.PermitVersion == version && a.PermitHash == permitHash {
			return &HospitalAcceptanceDTO{
				HospitalSyncDTO: hospitalSyncDTO(r), TenantID: a.TenantID, PermitID: a.PermitID,
				PermitVersion: a.PermitVersion, PermitHash: a.PermitHash, Revision: a.Revision, Decision: a.Decision,
				AllowedFieldCodes: a.AllowedFieldCodes, ExpiresAt: a.ExpiresAt,
			}
		}
	}
	return nil
}

type HospitalPermitDTO struct {
	TenantID           string                 `json:"tenant_id"`
	ID                 string                 `json:"id"`
	Version            int64                  `json:"version"`
	PermitHash         string                 `json:"permit_hash"`
	Status             string                 `json:"status"`
	Title              string                 `json:"title"`
	PIPrincipalID      string                 `json:"pi_principal_id"`
	MemberPrincipalIDs []string               `json:"member_principal_ids"`
	Purpose            string                 `json:"purpose"`
	JobType            string                 `json:"job_type"`
	JobVersion         string                 `json:"job_version"`
	AllowedFieldCodes  []string               `json:"allowed_field_codes"`
	AllowedNodeIDs     []string               `json:"allowed_node_ids"`
	EthicsReference    string                 `json:"ethics_reference"`
	ValidFrom          time.Time              `json:"valid_from"`
	ExpiresAt          time.Time              `json:"expires_at"`
	Acceptance         *HospitalAcceptanceDTO `json:"acceptance"`
}

type HospitalPermitListDTO struct {
	HospitalSyncDTO
	Items      []HospitalPermitDTO `json:"items"`
	TotalCount int                 `json:"total_count"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
}

func NewHospitalPermitListDTO(r entities.HospitalGovernanceRecord, page, size int) HospitalPermitListDTO {
	items := make([]HospitalPermitDTO, 0)
	if page < 1 || size < 1 {
		return HospitalPermitListDTO{HospitalSyncDTO: hospitalSyncDTO(r), Items: items}
	}
	// Bound before multiplication so an extreme page cannot overflow an offset.
	total := len(r.Snapshot.Permits)
	if page <= total/size+1 {
		start := (page - 1) * size
		end := min(start+size, total)
		for _, p := range r.Snapshot.Permits[start:end] {
			items = append(items, HospitalPermitDTO{
				TenantID: p.TenantID, ID: p.ID, Version: p.Version, PermitHash: p.PermitHash, Status: p.Status, Title: p.Title,
				PIPrincipalID: p.PIPrincipalID, MemberPrincipalIDs: p.MemberPrincipalIDs, Purpose: p.Purpose, JobType: p.JobType, JobVersion: p.JobVersion,
				AllowedFieldCodes: p.AllowedFieldCodes, AllowedNodeIDs: p.AllowedNodeIDs, EthicsReference: p.EthicsReference,
				ValidFrom: p.ValidFrom, ExpiresAt: p.ExpiresAt, Acceptance: NewHospitalAcceptanceDTO(r, p.ID, p.Version, p.PermitHash),
			})
		}
	}
	return HospitalPermitListDTO{HospitalSyncDTO: hospitalSyncDTO(r), Items: items, TotalCount: total, Page: page, PageSize: size}
}
