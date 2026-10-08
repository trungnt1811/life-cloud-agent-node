package entities

import (
	"math"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// A stored ACK is not authority for a new process or transport connection.
func (g *HospitalGovernance) FenceConnection() {
	g.record.AcknowledgedRevision = 0
	g.record.Snapshot.SessionEpoch = ""
}

var hospitalPermitIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func (g *HospitalGovernance) ApplySnapshot(snapshot HospitalSnapshot, _ time.Time) error {
	if g.record.NodeID == "" || snapshot.Revision < 1 || snapshot.NetworkFloor < 10 || snapshot.NetworkFloor > math.MaxInt64 || strings.TrimSpace(snapshot.SessionEpoch) == "" || len(snapshot.SessionEpoch) > 128 || len(snapshot.Permits) > 1000 {
		return ErrHospitalGovernanceInvalid
	}
	prior := g.record.Snapshot
	if snapshot.Revision < prior.Revision || (snapshot.Revision == prior.Revision && !reflect.DeepEqual(snapshot, prior)) {
		return ErrHospitalGovernanceConflict
	}
	seen := make(map[string]bool, len(snapshot.Permits))
	for _, permit := range snapshot.Permits {
		key := permit.TenantID + "/" + permit.ID
		if seen[key] || !validHospitalPermit(permit, g.record.NodeID) {
			return ErrHospitalGovernanceInvalid
		}
		seen[key] = true
	}
	if reflect.DeepEqual(snapshot, prior) {
		return nil
	}
	g.record.Snapshot = snapshot
	g.record.AcknowledgedRevision = 0
	g.record = cloneHospitalGovernance(g.record)
	return nil
}

func (g *HospitalGovernance) Acknowledge(epoch string, snapshotRevision, localRevision int64) error {
	if epoch == "" || epoch != g.record.Snapshot.SessionEpoch || snapshotRevision != g.record.Snapshot.Revision || localRevision < 1 || localRevision > g.record.LocalRevision {
		return ErrHospitalGovernanceConflict
	}
	if localRevision > g.record.AcknowledgedRevision {
		g.record.AcknowledgedRevision = localRevision
	}
	return nil
}

func validHospitalPermit(p HospitalPermitRecord, nodeID string) bool {
	if parsed, err := uuid.Parse(p.TenantID); err != nil || parsed == uuid.Nil {
		return false
	}
	if !hospitalPermitIDPattern.MatchString(p.ID) || p.Version < 1 || (p.Status != HospitalPermitActive && p.Status != HospitalPermitRevoked) || p.JobType != "COUNT_MATCHING_COHORT" || p.JobVersion != "v1" || p.Purpose != "research" || p.ValidFrom.IsZero() || !p.ExpiresAt.After(p.ValidFrom) || !slices.Contains(p.AllowedNodeIDs, nodeID) || len(p.MemberPrincipalIDs) < 1 || len(p.MemberPrincipalIDs) > 1000 || len(p.AllowedNodeIDs) > 100 || strings.TrimSpace(p.Title) == "" || len(p.Title) > 255 || strings.TrimSpace(p.EthicsReference) == "" || len(p.EthicsReference) > 255 {
		return false
	}
	fields, err := hospitalFields(p.AllowedFieldCodes)
	if err != nil || len(fields) == 0 || !slices.Equal(fields, p.AllowedFieldCodes) {
		return false
	}
	for _, id := range append(slices.Clone(p.MemberPrincipalIDs), p.PIPrincipalID) {
		if parsed, err := uuid.Parse(id); err != nil || parsed == uuid.Nil {
			return false
		}
	}
	hash, err := p.Hash()
	return err == nil && hash == p.PermitHash
}
