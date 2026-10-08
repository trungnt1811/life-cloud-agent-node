package entities

import (
	"errors"
	"slices"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

var (
	ErrHospitalGovernanceInvalid  = errors.New("invalid hospital governance scope")
	ErrHospitalGovernanceConflict = errors.New("hospital governance revision conflict")
)

const (
	HospitalPermitActive       = "ACTIVE"
	HospitalPermitRevoked      = "REVOKED"
	HospitalAcceptanceAccepted = "ACCEPTED"
	HospitalAcceptanceDeclined = "DECLINED"
	HospitalReleaseAuto        = "AUTO"
	HospitalReleaseManual      = "MANUAL"
)

type HospitalPermitScope struct {
	Title              string    `json:"title"`
	PIPrincipalID      string    `json:"pi_principal_id"`
	MemberPrincipalIDs []string  `json:"member_principal_ids"`
	Purpose            string    `json:"purpose"`
	JobType            string    `json:"job_type"`
	JobVersion         string    `json:"job_version"`
	AllowedFieldCodes  []string  `json:"allowed_field_codes"`
	AllowedNodeIDs     []string  `json:"allowed_node_ids"`
	EthicsReference    string    `json:"ethics_reference"`
	ValidFrom          time.Time `json:"valid_from"`
	ExpiresAt          time.Time `json:"expires_at"`
}

type HospitalPermitRecord struct {
	HospitalPermitScope
	TenantID   string
	ID         string
	Version    int64
	Status     string
	PermitHash string
}

func (p HospitalPermitRecord) Hash() (string, error) {
	return types.GovernanceDigest("study-permit/v1", map[string]any{
		"tenant_id": p.TenantID, "id": p.ID, "version": p.Version, "scope": p.HospitalPermitScope,
	})
}

type HospitalPolicyRecord struct {
	Version           int64
	PolicyHash        string
	LocalK            uint64
	ReleaseMode       string
	EnabledFieldCodes []string
	FilterFieldCodes  []string
	PanelFieldCodes   []string
}

type HospitalAcceptanceRecord struct {
	TenantID          string
	PermitID          string
	PermitVersion     int64
	PermitHash        string
	Revision          int64
	Decision          string
	AllowedFieldCodes []string
	ExpiresAt         time.Time
}

type HospitalSnapshot struct {
	Revision     int64
	SessionEpoch string
	NetworkFloor uint64
	Permits      []HospitalPermitRecord
}

type HospitalGovernanceRecord struct {
	NodeID               string
	LocalRevision        int64
	AcknowledgedRevision int64
	Snapshot             HospitalSnapshot
	Policy               HospitalPolicyRecord
	Acceptances          []HospitalAcceptanceRecord
	Paused               bool
	AvailabilityRevision int64
	InvalidatedPermits   []HospitalPermitFence
	InvalidatedQueries   []HospitalQueryFence
}

type HospitalGovernance struct{ record HospitalGovernanceRecord }

func NewHospitalGovernance(nodeID string) *HospitalGovernance {
	policy := HospitalPolicyRecord{Version: 1, LocalK: 10, ReleaseMode: HospitalReleaseAuto, EnabledFieldCodes: []string{}, FilterFieldCodes: []string{}, PanelFieldCodes: []string{}}
	policy.PolicyHash, _ = hospitalPolicyHash(nodeID, policy)
	return &HospitalGovernance{record: HospitalGovernanceRecord{
		NodeID: nodeID, LocalRevision: 1, AvailabilityRevision: 1, Policy: policy, Acceptances: []HospitalAcceptanceRecord{},
	}}
}

func NewHospitalGovernanceFromRecord(record HospitalGovernanceRecord) *HospitalGovernance {
	return &HospitalGovernance{record: cloneHospitalGovernance(record)}
}

func (g *HospitalGovernance) Record() HospitalGovernanceRecord {
	return cloneHospitalGovernance(g.record)
}

func (g *HospitalGovernance) Synchronized() bool {
	return g.record.Synchronized()
}

func (r HospitalGovernanceRecord) Synchronized() bool {
	return r.LocalRevision > 0 && r.Snapshot.Revision > 0 && r.Snapshot.SessionEpoch != "" && r.AcknowledgedRevision == r.LocalRevision
}

func cloneHospitalGovernance(r HospitalGovernanceRecord) HospitalGovernanceRecord {
	r.InvalidatedPermits = slices.Clone(r.InvalidatedPermits)
	r.InvalidatedQueries = slices.Clone(r.InvalidatedQueries)
	r.Policy.EnabledFieldCodes = slices.Clone(r.Policy.EnabledFieldCodes)
	r.Policy.FilterFieldCodes = slices.Clone(r.Policy.FilterFieldCodes)
	r.Policy.PanelFieldCodes = slices.Clone(r.Policy.PanelFieldCodes)
	r.Snapshot.Permits = slices.Clone(r.Snapshot.Permits)
	for i := range r.Snapshot.Permits {
		p := &r.Snapshot.Permits[i]
		p.MemberPrincipalIDs = slices.Clone(p.MemberPrincipalIDs)
		p.AllowedNodeIDs = slices.Clone(p.AllowedNodeIDs)
		p.AllowedFieldCodes = slices.Clone(p.AllowedFieldCodes)
	}
	r.Acceptances = slices.Clone(r.Acceptances)
	for i := range r.Acceptances {
		r.Acceptances[i].AllowedFieldCodes = slices.Clone(r.Acceptances[i].AllowedFieldCodes)
	}
	return r
}
