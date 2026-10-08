package wire

import (
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func GovernedHospitalTaskFromProto(p *nodev1.GovernedQueryTask) (entities.GovernedHospitalTask, error) {
	invalid := entities.ErrGovernedHospitalJobInvalid
	if p == nil || governedPacketUnknown(p.ProtoReflect()) || p.Profile != types.GovernedCohortProfile || p.MessageSchemaVersion != 1 || p.Criteria == nil || p.Authorization == nil || p.Permit == nil || !validHospitalTimestamp(p.DispatchDeadline) || !validHospitalTimestamp(p.QueryDeadline) || (p.ReleaseMode != nodev1.ReleaseMode_RELEASE_MODE_AUTO && p.ReleaseMode != nodev1.ReleaseMode_RELEASE_MODE_MANUAL) {
		return entities.GovernedHospitalTask{}, invalid
	}
	a := p.Authorization
	if a.TenantId != p.Permit.TenantId || a.Purpose != p.Permit.Purpose || a.JobType != p.Permit.JobType || a.JobVersion != p.Permit.JobVersion || strings.TrimSpace(a.PolicyVersion) == "" || len(a.PolicyVersion) > 255 || !governedPacketHash(a.PolicyHash) {
		return entities.GovernedHospitalTask{}, invalid
	}
	for _, validate := range []func(*nodev1.QueryTask) error{ValidateQueryTaskVersionV1, ValidateQueryTaskStructureV1, ValidateQueryTaskSemanticV1} {
		if validate(p.Criteria) != nil {
			return entities.GovernedHospitalTask{}, invalid
		}
	}
	criteria, err := CohortCriteriaFromQueryTaskV1(p.Criteria)
	if err != nil {
		return entities.GovernedHospitalTask{}, invalid
	}
	permit, err := hospitalPermitFromProto(p.Permit)
	if err != nil {
		return entities.GovernedHospitalTask{}, invalid
	}
	releaseMode := entities.HospitalReleaseAuto
	if p.ReleaseMode == nodev1.ReleaseMode_RELEASE_MODE_MANUAL {
		releaseMode = entities.HospitalReleaseManual
	}
	task := entities.GovernedHospitalTask{JobID: p.Criteria.JobId, QueryID: p.QueryId, NodeID: p.NodeId, TenantID: a.TenantId, PrincipalID: a.PrincipalId, Purpose: a.Purpose, Criteria: criteria, DefinitionHash: p.DefinitionHash, AuthorizationSnapshotHash: a.AuthorizationSnapshotHash, Permit: permit, LocalPolicyVersion: p.LocalPolicyVersion, LocalPolicyHash: p.LocalPolicyHash, AcceptanceRevision: p.AcceptanceRevision, ReleaseMode: releaseMode, EffectiveK: p.EffectiveK, DispatchDeadline: p.DispatchDeadline.AsTime(), QueryDeadline: p.QueryDeadline.AsTime(), SessionEpoch: p.SessionEpoch}
	if err := task.ValidateEnvelope(); err != nil {
		return entities.GovernedHospitalTask{}, err
	}
	return task, nil
}

func governedPacketHash(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, c := range value[7:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func governedPacketUnknown(message protoreflect.Message) bool {
	if !message.IsValid() || len(message.GetUnknown()) != 0 {
		return true
	}
	unknown := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		switch {
		case field.IsMap():
			if field.MapValue().Kind() == protoreflect.MessageKind {
				value.Map().Range(func(_ protoreflect.MapKey, v protoreflect.Value) bool {
					unknown = governedPacketUnknown(v.Message())
					return !unknown
				})
			}
		case field.Kind() != protoreflect.MessageKind:
			return true
		case field.IsList():
			list := value.List()
			for i := 0; i < list.Len(); i++ {
				if governedPacketUnknown(list.Get(i).Message()) {
					unknown = true
					break
				}
			}
		default:
			unknown = governedPacketUnknown(value.Message())
		}
		return !unknown
	})
	return unknown
}
