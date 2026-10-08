package entities

import (
	"math"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

type HospitalQueryFence struct {
	TenantID      string
	QueryID       string
	PermitID      string
	PermitVersion int64
	Reason        string
}

func (g *HospitalGovernance) InvalidateQuery(command HospitalInvalidationCommand) error {
	if command.SessionEpoch == "" || command.SessionEpoch != g.record.Snapshot.SessionEpoch || !validGovernedUUID(command.CommandID) || !validGovernedUUID(command.TenantID) || !validGovernedUUID(command.QueryID) || command.ThroughVersion < 1 || !hospitalPermitIDPattern.MatchString(command.PermitID) || (command.Reason != types.GovernanceReasonQueryCanceled && command.Reason != types.GovernanceReasonQueryExpired) {
		return ErrHospitalGovernanceInvalid
	}
	fence := HospitalQueryFence{TenantID: command.TenantID, QueryID: command.QueryID, PermitID: command.PermitID, PermitVersion: command.ThroughVersion, Reason: command.Reason}
	for _, prior := range g.record.InvalidatedQueries {
		if prior.TenantID == fence.TenantID && prior.QueryID == fence.QueryID {
			if prior != fence {
				return ErrHospitalGovernanceConflict
			}
			return nil
		}
	}
	if g.record.LocalRevision == math.MaxInt64 || len(g.record.InvalidatedQueries) >= 10000 {
		return ErrHospitalGovernanceInvalid
	}
	g.record.InvalidatedQueries = append(g.record.InvalidatedQueries, fence)
	g.record.LocalRevision++
	return nil
}

func (g *HospitalGovernance) QueryInvalidated(tenant, queryID string) bool {
	for _, fence := range g.record.InvalidatedQueries {
		if fence.TenantID == tenant && fence.QueryID == queryID {
			return true
		}
	}
	return false
}
