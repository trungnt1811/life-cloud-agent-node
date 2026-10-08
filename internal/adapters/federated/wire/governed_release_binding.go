package wire

import (
	"strings"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func governedHospitalBindingFromProto(p *nodev1.ReleaseBinding) (entities.GovernedHospitalBinding, error) {
	if p == nil || governedPacketUnknown(p.ProtoReflect()) {
		return entities.GovernedHospitalBinding{}, entities.ErrGovernedHospitalJobInvalid
	}
	b := entities.GovernedHospitalBinding{TenantID: p.TenantId, NodeID: p.NodeId, QueryID: p.QueryId, JobID: p.JobId, EventID: p.EventId, ProtectedPayloadDigest: p.ProtectedPayloadDigest, DefinitionHash: p.DefinitionHash, PermitID: p.PermitId, PermitVersion: p.PermitVersion, PermitHash: p.PermitHash, LocalPolicyVersion: p.LocalPolicyVersion, LocalPolicyHash: p.LocalPolicyHash, AcceptanceRevision: p.AcceptanceRevision, AuthorizationSnapshotHash: p.AuthorizationSnapshotHash, SessionEpoch: p.SessionEpoch}
	return b, b.Validate(true)
}

func governedHospitalBindingToProto(b entities.GovernedHospitalBinding) *nodev1.ReleaseBinding {
	return &nodev1.ReleaseBinding{TenantId: b.TenantID, NodeId: b.NodeID, QueryId: b.QueryID, JobId: b.JobID, EventId: b.EventID, ProtectedPayloadDigest: b.ProtectedPayloadDigest, DefinitionHash: b.DefinitionHash, PermitId: b.PermitID, PermitVersion: b.PermitVersion, PermitHash: b.PermitHash, LocalPolicyVersion: b.LocalPolicyVersion, LocalPolicyHash: b.LocalPolicyHash, AcceptanceRevision: b.AcceptanceRevision, AuthorizationSnapshotHash: b.AuthorizationSnapshotHash, SessionEpoch: b.SessionEpoch}
}

func governedHospitalReasonFromProto(value nodev1.GovernanceReason) (string, bool) {
	if value == nodev1.GovernanceReason_GOVERNANCE_REASON_UNSPECIFIED {
		return "", true
	}
	name, exists := nodev1.GovernanceReason_name[int32(value)]
	return strings.TrimPrefix(name, "GOVERNANCE_REASON_"), exists
}
