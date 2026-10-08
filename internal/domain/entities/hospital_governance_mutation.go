package entities

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

type HospitalAcceptanceChange struct {
	PermitID          string
	PermitVersion     int64
	PermitHash        string
	ExpectedRevision  int64
	Decision          string
	AllowedFieldCodes []string
	ExpiresAt         time.Time
}

func NormalizeHospitalAcceptanceChange(change HospitalAcceptanceChange) (HospitalAcceptanceChange, error) {
	if change.ExpectedRevision < 0 || change.ExpectedRevision == math.MaxInt64 || !hospitalPermitIDPattern.MatchString(change.PermitID) || change.PermitVersion < 1 || (change.Decision != HospitalAcceptanceAccepted && change.Decision != HospitalAcceptanceDeclined) || change.ExpiresAt.IsZero() {
		return change, ErrHospitalGovernanceInvalid
	}
	var err error
	change.AllowedFieldCodes, err = hospitalFields(change.AllowedFieldCodes)
	if err != nil || (change.Decision == HospitalAcceptanceAccepted && len(change.AllowedFieldCodes) == 0) {
		return change, ErrHospitalGovernanceInvalid
	}
	change.ExpiresAt = change.ExpiresAt.UTC().Truncate(time.Microsecond)
	return change, nil
}

func (g *HospitalGovernance) Accept(change HospitalAcceptanceChange, now time.Time) error {
	if change.ExpectedRevision < 0 || change.ExpectedRevision == math.MaxInt64 || g.record.LocalRevision == math.MaxInt64 || (change.Decision != HospitalAcceptanceAccepted && change.Decision != HospitalAcceptanceDeclined) {
		return ErrHospitalGovernanceInvalid
	}
	var permit *HospitalPermitRecord
	for _, p := range g.record.Snapshot.Permits {
		if p.ID == change.PermitID && p.Version == change.PermitVersion && p.PermitHash == change.PermitHash {
			copyPermit := p
			permit = &copyPermit
			break
		}
	}
	if permit == nil || g.PermitInvalidated(permit.TenantID, permit.ID, permit.Version) || permit.Status != HospitalPermitActive || !permit.ExpiresAt.After(now) || !change.ExpiresAt.After(now) || change.ExpiresAt.After(permit.ExpiresAt) {
		return ErrHospitalGovernanceInvalid
	}
	fields, err := hospitalFields(change.AllowedFieldCodes)
	if err != nil || (change.Decision == HospitalAcceptanceAccepted && len(fields) == 0) || !hospitalFieldSubset(fields, permit.AllowedFieldCodes) {
		return ErrHospitalGovernanceInvalid
	}
	index := -1
	var revision int64
	for i, prior := range g.record.Acceptances {
		if prior.TenantID == permit.TenantID && prior.PermitID == permit.ID && prior.PermitVersion == permit.Version {
			index, revision = i, prior.Revision
		}
	}
	if revision != change.ExpectedRevision {
		return ErrHospitalGovernanceConflict
	}
	acceptance := HospitalAcceptanceRecord{
		TenantID: permit.TenantID, PermitID: permit.ID, PermitVersion: permit.Version, PermitHash: permit.PermitHash,
		Revision: revision + 1, Decision: change.Decision, AllowedFieldCodes: fields, ExpiresAt: change.ExpiresAt.UTC(),
	}
	if index == -1 {
		g.record.Acceptances = append(g.record.Acceptances, acceptance)
	} else {
		g.record.Acceptances[index] = acceptance
	}
	g.record.LocalRevision++
	return nil
}

func (g *HospitalGovernance) CurrentAcceptance(tenant, id string, version int64, now time.Time) *HospitalAcceptanceRecord {
	if g.PermitInvalidated(tenant, id, version) {
		return nil
	}
	for _, p := range g.record.Snapshot.Permits {
		if p.TenantID != tenant || p.ID != id || p.Version != version || p.Status != HospitalPermitActive || now.Before(p.ValidFrom) || !now.Before(p.ExpiresAt) {
			continue
		}
		for _, a := range g.record.Acceptances {
			if a.TenantID == tenant && a.PermitID == id && a.PermitVersion == version && a.PermitHash == p.PermitHash && a.Decision == HospitalAcceptanceAccepted && now.Before(a.ExpiresAt) {
				a.AllowedFieldCodes = slices.Clone(a.AllowedFieldCodes)
				return &a
			}
		}
	}
	return nil
}

func (g *HospitalGovernance) UpdatePolicy(policy HospitalPolicyRecord, expectedVersion int64) error {
	if expectedVersion < 1 || expectedVersion != g.record.Policy.Version {
		return ErrHospitalGovernanceConflict
	}
	if expectedVersion == math.MaxInt64 || g.record.LocalRevision == math.MaxInt64 || policy.LocalK < 10 || policy.LocalK > math.MaxInt64 || !validHospitalReleaseMode(policy.ReleaseMode) {
		return ErrHospitalGovernanceInvalid
	}
	var err error
	policy.EnabledFieldCodes, err = hospitalFields(policy.EnabledFieldCodes)
	if err != nil {
		return err
	}
	policy.FilterFieldCodes, err = hospitalFields(policy.FilterFieldCodes)
	if err != nil {
		return err
	}
	policy.PanelFieldCodes, err = hospitalFields(policy.PanelFieldCodes)
	if err != nil || !hospitalFieldSubset(policy.FilterFieldCodes, policy.EnabledFieldCodes) || !hospitalFieldSubset(policy.PanelFieldCodes, policy.EnabledFieldCodes) {
		return ErrHospitalGovernanceInvalid
	}
	policy.Version = expectedVersion + 1
	policy.PolicyHash, err = hospitalPolicyHash(g.record.NodeID, policy)
	if err != nil {
		return err
	}
	g.record.Policy = policy
	g.record.LocalRevision++
	return nil
}

func validHospitalReleaseMode(mode string) bool {
	return mode == HospitalReleaseAuto || mode == HospitalReleaseManual
}

func hospitalPolicyHash(nodeID string, p HospitalPolicyRecord) (string, error) {
	return types.GovernanceDigest("local-policy/v1", map[string]any{
		"node_id": nodeID, "version": p.Version, "local_k": p.LocalK, "release_mode": p.ReleaseMode,
		"enabled_field_codes": p.EnabledFieldCodes, "filter_field_codes": p.FilterFieldCodes, "panel_field_codes": p.PanelFieldCodes,
	})
}

func (g *HospitalGovernance) ValidatePolicy() error {
	p := g.record.Policy
	if p.Version < 1 {
		return ErrHospitalGovernanceInvalid
	}
	preview := NewHospitalGovernance(g.record.NodeID)
	if err := preview.UpdatePolicy(p, 1); err != nil {
		return err
	}
	normalized := preview.record.Policy
	if !slices.Equal(normalized.EnabledFieldCodes, p.EnabledFieldCodes) || !slices.Equal(normalized.FilterFieldCodes, p.FilterFieldCodes) || !slices.Equal(normalized.PanelFieldCodes, p.PanelFieldCodes) {
		return ErrHospitalGovernanceInvalid
	}
	hash, err := hospitalPolicyHash(g.record.NodeID, p)
	if err != nil || hash != p.PolicyHash {
		return ErrHospitalGovernanceInvalid
	}
	return nil
}

func hospitalFields(raw []string) ([]string, error) {
	if len(raw) > len(queryfields.SchemaV1FieldCodes)*2 {
		return nil, ErrHospitalGovernanceInvalid
	}
	fields := make([]string, 0, len(raw))
	for _, code := range raw {
		code = strings.ToUpper(strings.TrimSpace(code))
		if !queryfields.IsSchemaV1FieldCode(code) {
			return nil, ErrHospitalGovernanceInvalid
		}
		fields = append(fields, code)
	}
	slices.Sort(fields)
	return slices.Compact(fields), nil
}

func hospitalFieldSubset(fields, scope []string) bool {
	for _, field := range fields {
		if !slices.Contains(scope, field) {
			return false
		}
	}
	return true
}
