package wire

import (
	"slices"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

// Mapping is not permission: the client still refuses governed messages while OFF.
func HospitalSnapshotFromProto(nodeID string, packet *nodev1.GovernanceSnapshot) (entities.HospitalSnapshot, error) {
	if packet == nil || len(packet.ProtoReflect().GetUnknown()) != 0 || packet.Profile != types.GovernedCohortProfile || packet.MessageSchemaVersion != 1 || len(packet.Permits) > 1000 {
		return entities.HospitalSnapshot{}, entities.ErrHospitalGovernanceInvalid
	}
	snapshot := entities.HospitalSnapshot{Revision: packet.SnapshotRevision, SessionEpoch: packet.SessionEpoch, NetworkFloor: packet.NetworkFloor, Permits: make([]entities.HospitalPermitRecord, 0, len(packet.Permits))}
	for _, p := range packet.Permits {
		permit, err := hospitalPermitFromProto(p)
		if err != nil {
			return entities.HospitalSnapshot{}, err
		}
		snapshot.Permits = append(snapshot.Permits, permit)
	}
	state := entities.NewHospitalGovernance(nodeID)
	if err := state.ApplySnapshot(snapshot, time.Time{}); err != nil {
		return entities.HospitalSnapshot{}, err
	}
	return state.Record().Snapshot, nil
}

func hospitalPermitFromProto(p *nodev1.PermitScope) (entities.HospitalPermitRecord, error) {
	if p == nil || len(p.ProtoReflect().GetUnknown()) != 0 || !validHospitalTimestamp(p.ValidFrom) || !validHospitalTimestamp(p.ExpiresAt) {
		return entities.HospitalPermitRecord{}, entities.ErrHospitalGovernanceInvalid
	}
	var status string
	switch p.Status {
	case nodev1.PermitStatus_PERMIT_STATUS_ACTIVE:
		status = entities.HospitalPermitActive
	case nodev1.PermitStatus_PERMIT_STATUS_REVOKED:
		status = entities.HospitalPermitRevoked
	default:
		return entities.HospitalPermitRecord{}, entities.ErrHospitalGovernanceInvalid
	}
	return entities.HospitalPermitRecord{
		TenantID: p.TenantId, ID: p.PermitId, Version: p.Version, PermitHash: p.PermitHash, Status: status,
		HospitalPermitScope: entities.HospitalPermitScope{
			Title: p.Title, PIPrincipalID: p.PiPrincipalId, MemberPrincipalIDs: slices.Clone(p.MemberPrincipalIds),
			Purpose: p.Purpose, JobType: p.JobType, JobVersion: p.JobVersion, AllowedFieldCodes: slices.Clone(p.AllowedFieldCodes), AllowedNodeIDs: slices.Clone(p.AllowedNodeIds),
			EthicsReference: p.EthicsReference, ValidFrom: p.ValidFrom.AsTime(), ExpiresAt: p.ExpiresAt.AsTime(),
		},
	}, nil
}

func HospitalReportToProto(r entities.HospitalGovernanceRecord) (*nodev1.GovernanceReport, error) {
	state := entities.NewHospitalGovernance(r.NodeID)
	if err := state.ApplySnapshot(r.Snapshot, time.Time{}); err != nil {
		return nil, err
	}
	if r.LocalRevision < 1 || entities.NewHospitalGovernanceFromRecord(r).ValidatePolicy() != nil {
		return nil, entities.ErrHospitalGovernanceInvalid
	}
	p := r.Policy
	var releaseMode nodev1.ReleaseMode
	switch p.ReleaseMode {
	case entities.HospitalReleaseAuto:
		releaseMode = nodev1.ReleaseMode_RELEASE_MODE_AUTO
	case entities.HospitalReleaseManual:
		releaseMode = nodev1.ReleaseMode_RELEASE_MODE_MANUAL
	default:
		return nil, entities.ErrHospitalGovernanceInvalid
	}
	packet := &nodev1.GovernanceReport{
		Profile: types.GovernedCohortProfile, MessageSchemaVersion: 1, SessionEpoch: r.Snapshot.SessionEpoch,
		AppliedSnapshotRevision: r.Snapshot.Revision, LocalRevision: r.LocalRevision, Paused: r.Paused,
		Policy: &nodev1.LocalPolicy{
			Version: p.Version, PolicyHash: p.PolicyHash, LocalK: p.LocalK, ReleaseMode: releaseMode,
			EnabledFieldCodes: slices.Clone(p.EnabledFieldCodes), FilterFieldCodes: slices.Clone(p.FilterFieldCodes), PanelFieldCodes: slices.Clone(p.PanelFieldCodes),
		},
	}
	// Historical decisions remain local; a report describes current CP scopes only.
	current := make(map[string]string, len(r.Snapshot.Permits))
	for _, permit := range r.Snapshot.Permits {
		current[permit.TenantID+"/"+permit.ID] = permit.PermitHash
	}
	for _, a := range r.Acceptances {
		if current[a.TenantID+"/"+a.PermitID] != a.PermitHash {
			continue
		}
		if a.Revision < 1 {
			return nil, entities.ErrHospitalGovernanceInvalid
		}
		var decision nodev1.AcceptanceDecision
		switch a.Decision {
		case entities.HospitalAcceptanceAccepted:
			decision = nodev1.AcceptanceDecision_ACCEPTANCE_DECISION_ACCEPTED
		case entities.HospitalAcceptanceDeclined:
			decision = nodev1.AcceptanceDecision_ACCEPTANCE_DECISION_DECLINED
		default:
			return nil, entities.ErrHospitalGovernanceInvalid
		}
		expiry := timestamppb.New(a.ExpiresAt)
		if !validHospitalTimestamp(expiry) {
			return nil, entities.ErrHospitalGovernanceInvalid
		}
		packet.Acceptances = append(packet.Acceptances, &nodev1.PermitAcceptance{
			TenantId: a.TenantID, PermitId: a.PermitID, PermitVersion: a.PermitVersion, PermitHash: a.PermitHash, Revision: a.Revision,
			Decision: decision, AllowedFieldCodes: slices.Clone(a.AllowedFieldCodes), ExpiresAt: expiry,
		})
	}
	return packet, nil
}

func validHospitalTimestamp(value *timestamppb.Timestamp) bool {
	return value != nil && len(value.ProtoReflect().GetUnknown()) == 0 && value.CheckValid() == nil
}
