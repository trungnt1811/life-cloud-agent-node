package wire

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func governedWireTask(t *testing.T) (*nodev1.GovernedQueryTask, *entities.HospitalGovernance, time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	packet := hospitalWireSnapshot(t)
	snapshot, err := HospitalSnapshotFromProto("node-a", packet)
	require.NoError(t, err)
	state := entities.NewHospitalGovernance("node-a")
	require.NoError(t, state.ApplySnapshot(snapshot, now))
	require.NoError(t, state.UpdatePolicy(entities.HospitalPolicyRecord{LocalK: 10, ReleaseMode: "AUTO", EnabledFieldCodes: []string{"MCV"}, FilterFieldCodes: []string{"MCV"}}, 1))
	p := snapshot.Permits[0]
	require.NoError(t, state.Accept(entities.HospitalAcceptanceChange{PermitID: p.ID, PermitVersion: p.Version, PermitHash: p.PermitHash, Decision: "ACCEPTED", AllowedFieldCodes: []string{"MCV"}, ExpiresAt: p.ExpiresAt}, now))
	require.NoError(t, state.Acknowledge(snapshot.SessionEpoch, snapshot.Revision, state.Record().LocalRevision))
	criteria := &nodev1.QueryTask{JobId: uuid.NewString(), QuerySchemaVersion: 1, TimeRange: &nodev1.DateRange{From: "2024-01-01", To: "2024-12-31"}, SpecimenPolicy: nodev1.SpecimenPolicy_SPECIMEN_POLICY_LATEST_IN_RANGE, Conditions: []*nodev1.QueryCondition{{FieldCode: "MCV", Op: nodev1.ComparisonOperator_COMPARISON_OPERATOR_LT, Value: &nodev1.ConditionValue{Kind: &nodev1.ConditionValue_NumberValue{NumberValue: "80"}}}}}
	domainCriteria, err := CohortCriteriaFromQueryTaskV1(criteria)
	require.NoError(t, err)
	hash, err := domainCriteria.GovernedDefinitionHash()
	require.NoError(t, err)
	policy := state.Record().Policy
	return &nodev1.GovernedQueryTask{Profile: types.GovernedCohortProfile, MessageSchemaVersion: 1, Criteria: criteria, QueryId: uuid.NewString(), NodeId: "node-a", DefinitionHash: hash, Permit: packet.Permits[0], Authorization: &nodev1.AuthorizationContext{TenantId: p.TenantID, PrincipalId: p.PIPrincipalID, Purpose: p.Purpose, JobType: p.JobType, JobVersion: p.JobVersion, AuthorizationSnapshotHash: "sha256:" + strings.Repeat("a", 64), PolicyVersion: "local-v1", PolicyHash: "sha256:" + strings.Repeat("b", 64)}, LocalPolicyVersion: policy.Version, LocalPolicyHash: policy.PolicyHash, AcceptanceRevision: 1, ReleaseMode: nodev1.ReleaseMode_RELEASE_MODE_AUTO, EffectiveK: 10, DispatchDeadline: timestamppb.New(now.Add(30 * time.Second)), QueryDeadline: timestamppb.New(now.Add(90 * time.Second)), SessionEpoch: snapshot.SessionEpoch}, state, now
}

func TestGovernedTaskWireMappingIsNotExecutionAuthority(t *testing.T) {
	packet, state, now := governedWireTask(t)
	task, err := GovernedHospitalTaskFromProto(packet)
	require.NoError(t, err)
	require.Equal(t, packet.Criteria.JobId, task.JobID)
	require.Equal(t, packet.Authorization.AuthorizationSnapshotHash, task.AuthorizationSnapshotHash)
	require.NoError(t, task.Validate(state, now, true))
	state.FenceConnection()
	require.Error(t, task.Validate(state, now, true), "well-formed wire data does not supply local permission")
	packet.Permit.MemberPrincipalIds[0] = uuid.NewString()
	require.NotEqual(t, packet.Permit.MemberPrincipalIds, task.Permit.MemberPrincipalIDs, "mapping must not alias the packet")
}

func TestGovernedTaskWireRejectsAmbiguityBeforeCounter(t *testing.T) {
	for _, change := range []func(*nodev1.GovernedQueryTask){
		func(p *nodev1.GovernedQueryTask) { p.Profile = "legacy-cohort/v1" },
		func(p *nodev1.GovernedQueryTask) { p.MessageSchemaVersion++ },
		func(p *nodev1.GovernedQueryTask) { p.Criteria = nil },
		func(p *nodev1.GovernedQueryTask) { p.Permit = nil },
		func(p *nodev1.GovernedQueryTask) { p.Authorization = nil },
		func(p *nodev1.GovernedQueryTask) { p.Authorization.JobType = "EXPORT_PATIENT_RECORDS" },
		func(p *nodev1.GovernedQueryTask) { p.Authorization.TenantId = uuid.NewString() },
		func(p *nodev1.GovernedQueryTask) { p.Authorization.PolicyHash = "unreviewed" },
		func(p *nodev1.GovernedQueryTask) { p.Authorization.PrincipalId = uuid.NewString() },
		func(p *nodev1.GovernedQueryTask) { p.LocalPolicyHash = "unreviewed" },
		func(p *nodev1.GovernedQueryTask) { p.DefinitionHash = "sha256:" + strings.Repeat("b", 64) },
		func(p *nodev1.GovernedQueryTask) { p.Permit.Title = "tampered" },
		func(p *nodev1.GovernedQueryTask) { p.Permit.Status = nodev1.PermitStatus_PERMIT_STATUS_REVOKED },
		func(p *nodev1.GovernedQueryTask) { p.Criteria.QuerySchemaVersion++ },
		func(p *nodev1.GovernedQueryTask) { p.Criteria.JobId = "not-a-job" },
		func(p *nodev1.GovernedQueryTask) { p.EffectiveK = 5 },
		func(p *nodev1.GovernedQueryTask) { p.DispatchDeadline = nil },
		func(p *nodev1.GovernedQueryTask) {
			p.QueryDeadline = proto.Clone(p.DispatchDeadline).(*timestamppb.Timestamp)
			p.QueryDeadline.Seconds--
		},
		func(p *nodev1.GovernedQueryTask) {
			p.Criteria.Conditions[0].Value.ProtoReflect().SetUnknown([]byte{0x78, 1})
		},
		func(p *nodev1.GovernedQueryTask) { p.QueryDeadline.ProtoReflect().SetUnknown([]byte{0x78, 1}) },
	} {
		packet, _, _ := governedWireTask(t)
		change(packet)
		_, err := GovernedHospitalTaskFromProto(packet)
		require.Error(t, err)
	}
	_, err := GovernedHospitalTaskFromProto(nil)
	require.Error(t, err)
}

func TestGovernedTaskWireAcceptsManualOnlyWhenLocalPolicyMatches(t *testing.T) {
	packet, state, now := governedWireTask(t)
	policy := state.Record().Policy
	policy.ReleaseMode = entities.HospitalReleaseManual
	require.NoError(t, state.UpdatePolicy(policy, policy.Version))
	require.NoError(t, state.Acknowledge(packet.SessionEpoch, state.Record().Snapshot.Revision, state.Record().LocalRevision))
	policy = state.Record().Policy
	packet.ReleaseMode = nodev1.ReleaseMode_RELEASE_MODE_MANUAL
	packet.LocalPolicyVersion, packet.LocalPolicyHash = policy.Version, policy.PolicyHash
	packet.QueryDeadline = timestamppb.New(now.Add(48*time.Hour + 90*time.Second))
	if packet.QueryDeadline.AsTime().After(packet.Permit.ExpiresAt.AsTime()) {
		packet.QueryDeadline = packet.Permit.ExpiresAt
	}
	task, err := GovernedHospitalTaskFromProto(packet)
	require.NoError(t, err)
	require.Equal(t, entities.HospitalReleaseManual, task.ReleaseMode)
	require.NoError(t, task.Validate(state, now, true))

	packet.ReleaseMode = nodev1.ReleaseMode_RELEASE_MODE_AUTO
	wrongMode, err := GovernedHospitalTaskFromProto(packet)
	require.NoError(t, err, "task shape is valid but current hospital policy must reject mismatched execution")
	require.Error(t, wrongMode.Validate(state, now, true))
}
