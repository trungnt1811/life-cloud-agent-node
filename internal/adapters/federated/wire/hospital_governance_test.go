package wire

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func hospitalWireSnapshot(t *testing.T) *nodev1.GovernanceSnapshot {
	t.Helper()
	p := entities.HospitalPermitRecord{
		TenantID: "53a96e1c-87aa-4def-b926-675e7e74df6c", ID: "demo", Version: 1, Status: "ACTIVE",
		HospitalPermitScope: entities.HospitalPermitScope{Title: "Synthetic study", PIPrincipalID: "4bf1a9a6-1a90-43bc-bded-0a4e901559fb", MemberPrincipalIDs: []string{"4bf1a9a6-1a90-43bc-bded-0a4e901559fb"}, Purpose: "research", JobType: "COUNT_MATCHING_COHORT", JobVersion: "v1", AllowedFieldCodes: []string{"MCV"}, AllowedNodeIDs: []string{"node-a"}, EthicsReference: "synthetic", ValidFrom: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)},
	}
	var err error
	p.PermitHash, err = p.Hash()
	require.NoError(t, err)
	return &nodev1.GovernanceSnapshot{Profile: "governed-cohort/v1", MessageSchemaVersion: 1, SnapshotRevision: 1, SessionEpoch: "epoch-a", NetworkFloor: 10, Permits: []*nodev1.PermitScope{{
		TenantId: p.TenantID, PermitId: p.ID, Version: p.Version, PermitHash: p.PermitHash, Title: p.Title, PiPrincipalId: p.PIPrincipalID, MemberPrincipalIds: p.MemberPrincipalIDs,
		Purpose: p.Purpose, JobType: p.JobType, JobVersion: p.JobVersion, AllowedFieldCodes: p.AllowedFieldCodes, AllowedNodeIds: p.AllowedNodeIDs, EthicsReference: p.EthicsReference, ValidFrom: timestamppb.New(p.ValidFrom), ExpiresAt: timestamppb.New(p.ExpiresAt), Status: nodev1.PermitStatus_PERMIT_STATUS_ACTIVE,
	}}}
}

func TestGovernedHospitalGovernanceWireSnapshotAndReport(t *testing.T) {
	packet := hospitalWireSnapshot(t)
	snapshot, err := HospitalSnapshotFromProto("node-a", packet)
	require.NoError(t, err)
	state := entities.NewHospitalGovernance("node-a")
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	require.NoError(t, state.ApplySnapshot(snapshot, now))
	p := snapshot.Permits[0]
	require.NoError(t, state.Accept(entities.HospitalAcceptanceChange{PermitID: p.ID, PermitVersion: p.Version, PermitHash: p.PermitHash, Decision: "ACCEPTED", AllowedFieldCodes: []string{"MCV"}, ExpiresAt: p.ExpiresAt}, now))
	report, err := HospitalReportToProto(state.Record())
	require.NoError(t, err)
	require.Equal(t, packet.SessionEpoch, report.SessionEpoch)
	require.EqualValues(t, 2, report.LocalRevision)
	require.Equal(t, packet.SnapshotRevision, report.AppliedSnapshotRevision)
	require.Equal(t, nodev1.ReleaseMode_RELEASE_MODE_AUTO, report.Policy.ReleaseMode)
	require.EqualValues(t, 10, report.Policy.LocalK)
	require.Len(t, report.Acceptances, 1)
	require.Equal(t, p.TenantID, report.Acceptances[0].TenantId)
	require.Equal(t, p.PermitHash, report.Acceptances[0].PermitHash)
	require.Equal(t, nodev1.AcceptanceDecision_ACCEPTANCE_DECISION_ACCEPTED, report.Acceptances[0].Decision)
	report.Acceptances[0].AllowedFieldCodes[0] = "HB"
	require.Equal(t, []string{"MCV"}, state.Record().Acceptances[0].AllowedFieldCodes)
	state.FenceConnection()
	_, err = HospitalReportToProto(state.Record())
	require.Error(t, err)
}

func TestGovernedHospitalGovernanceReportCarriesManualLocalMode(t *testing.T) {
	packet := hospitalWireSnapshot(t)
	snapshot, err := HospitalSnapshotFromProto("node-a", packet)
	require.NoError(t, err)
	state := entities.NewHospitalGovernance("node-a")
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	require.NoError(t, state.ApplySnapshot(snapshot, now))
	policy := state.Record().Policy
	policy.ReleaseMode = entities.HospitalReleaseManual
	require.NoError(t, state.UpdatePolicy(policy, policy.Version))
	report, err := HospitalReportToProto(state.Record())
	require.NoError(t, err)
	require.Equal(t, nodev1.ReleaseMode_RELEASE_MODE_MANUAL, report.Policy.ReleaseMode)
}

func TestGovernedHospitalGovernanceWireRejectsAmbiguousScope(t *testing.T) {
	for _, edit := range []func(*nodev1.GovernanceSnapshot){
		func(p *nodev1.GovernanceSnapshot) { p.Profile = "legacy-cohort/v1" },
		func(p *nodev1.GovernanceSnapshot) { p.MessageSchemaVersion = 2 },
		func(p *nodev1.GovernanceSnapshot) { p.ProtoReflect().SetUnknown([]byte{0x78, 1}) },
		func(p *nodev1.GovernanceSnapshot) { p.Permits[0].ProtoReflect().SetUnknown([]byte{0x88, 2, 1}) },
		func(p *nodev1.GovernanceSnapshot) { p.Permits[0].ValidFrom = nil },
		func(p *nodev1.GovernanceSnapshot) {
			p.Permits[0].ExpiresAt = &timestamppb.Timestamp{Seconds: 253402300800}
		},
		func(p *nodev1.GovernanceSnapshot) { p.Permits[0].Status = nodev1.PermitStatus(99) },
		func(p *nodev1.GovernanceSnapshot) { p.Permits[0].Title = "tampered" },
		func(p *nodev1.GovernanceSnapshot) { p.NetworkFloor = 5 },
		func(p *nodev1.GovernanceSnapshot) { p.Permits[0] = nil },
	} {
		packet := hospitalWireSnapshot(t)
		edit(packet)
		_, err := HospitalSnapshotFromProto("node-a", packet)
		require.Error(t, err)
	}
	_, err := HospitalSnapshotFromProto("node-a", nil)
	require.Error(t, err)
}
