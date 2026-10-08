package entities

import (
	"math"
	"slices"
	"strings"
)

const HospitalPurposeResearch = "research"

// Shape validation supplies no authority. Validate still checks current local state.
func (t GovernedHospitalTask) ValidateEnvelope() error {
	if !validGovernedUUID(t.JobID) || !validGovernedUUID(t.QueryID) || !validGovernedUUID(t.PrincipalID) || !validGovernedUUID(t.TenantID) || !governedHospitalIdentifier(t.NodeID) || !governedHospitalIdentifier(t.SessionEpoch) || t.TenantID != t.Permit.TenantID || t.Permit.Status != HospitalPermitActive || t.Purpose != HospitalPurposeResearch || t.Purpose != t.Permit.Purpose || !slices.Contains(t.Permit.MemberPrincipalIDs, t.PrincipalID) || !validHospitalPermit(t.Permit, t.NodeID) || !hospitalHash(t.AuthorizationSnapshotHash) || !hospitalHash(t.LocalPolicyHash) || t.LocalPolicyVersion < 1 || t.AcceptanceRevision < 1 || !validHospitalReleaseMode(t.ReleaseMode) || t.EffectiveK < 10 || t.EffectiveK > math.MaxInt64 || t.DispatchDeadline.IsZero() || t.QueryDeadline.Before(t.DispatchDeadline) || t.QueryDeadline.After(t.Permit.ExpiresAt) {
		return ErrGovernedHospitalJobInvalid
	}
	hash, err := t.Criteria.GovernedDefinitionHash()
	if err != nil || hash != t.DefinitionHash {
		return ErrGovernedHospitalJobInvalid
	}
	return nil
}

func governedHospitalIdentifier(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, c := range value {
		if c < 32 || c == 127 {
			return false
		}
	}
	return true
}
