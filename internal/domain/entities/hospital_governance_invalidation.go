package entities

import (
	"math"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

type HospitalInvalidationCommand struct {
	CommandID      string
	TenantID       string
	PermitID       string
	QueryID        string
	ThroughVersion int64
	Reason         string
	SessionEpoch   string
}

type HospitalPermitFence struct {
	TenantID       string
	PermitID       string
	ThroughVersion int64
}

func (g *HospitalGovernance) InvalidatePermit(command HospitalInvalidationCommand) error {
	if command.QueryID != "" || command.SessionEpoch == "" || command.SessionEpoch != g.record.Snapshot.SessionEpoch || command.ThroughVersion < 1 || !hospitalPermitIDPattern.MatchString(command.PermitID) || (command.Reason != types.GovernanceReasonPermitRevoked && command.Reason != types.GovernanceReasonPermitSuperseded) {
		return ErrHospitalGovernanceInvalid
	}
	for _, id := range []string{command.CommandID, command.TenantID} {
		if parsed, err := uuid.Parse(id); err != nil || parsed == uuid.Nil {
			return ErrHospitalGovernanceInvalid
		}
	}
	index := -1
	for i, f := range g.record.InvalidatedPermits {
		if f.TenantID == command.TenantID && f.PermitID == command.PermitID {
			if command.ThroughVersion <= f.ThroughVersion {
				return nil
			}
			index = i
			break
		}
	}
	if g.record.LocalRevision == math.MaxInt64 || (index == -1 && len(g.record.InvalidatedPermits) >= 10000) {
		return ErrHospitalGovernanceInvalid
	}
	fence := HospitalPermitFence{TenantID: command.TenantID, PermitID: command.PermitID, ThroughVersion: command.ThroughVersion}
	if index == -1 {
		g.record.InvalidatedPermits = append(g.record.InvalidatedPermits, fence)
	} else {
		g.record.InvalidatedPermits[index] = fence
	}
	g.record.LocalRevision++
	return nil
}

func (g *HospitalGovernance) PermitInvalidated(tenant, id string, version int64) bool {
	for _, f := range g.record.InvalidatedPermits {
		if f.TenantID == tenant && f.PermitID == id && version <= f.ThroughVersion {
			return true
		}
	}
	return false
}
