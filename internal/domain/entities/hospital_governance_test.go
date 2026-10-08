package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func hospitalPermitFixture(t *testing.T) HospitalPermitRecord {
	t.Helper()
	p := HospitalPermitRecord{
		TenantID: "53a96e1c-87aa-4def-b926-675e7e74df6c", ID: "demo", Version: 1, Status: "ACTIVE",
		HospitalPermitScope: HospitalPermitScope{
			Title: "Synthetic study", PIPrincipalID: "4bf1a9a6-1a90-43bc-bded-0a4e901559fb",
			MemberPrincipalIDs: []string{"4bf1a9a6-1a90-43bc-bded-0a4e901559fb"}, Purpose: "research",
			JobType: "COUNT_MATCHING_COHORT", JobVersion: "v1", AllowedFieldCodes: []string{"MCH", "MCV"},
			AllowedNodeIDs: []string{"node-a"}, EthicsReference: "synthetic-only",
			ValidFrom: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	var err error
	p.PermitHash, err = p.Hash()
	require.NoError(t, err)
	return p
}

func syncedHospitalFixture(t *testing.T) (*HospitalGovernance, HospitalPermitRecord, time.Time) {
	t.Helper()
	p := hospitalPermitFixture(t)
	now := p.ValidFrom.Add(24 * time.Hour)
	state := NewHospitalGovernance("node-a")
	require.NoError(t, state.ApplySnapshot(HospitalSnapshot{
		Revision: 1, SessionEpoch: "epoch-a", NetworkFloor: 10, Permits: []HospitalPermitRecord{p},
	}, now))
	return state, p, now
}

func TestGovernedHospitalSnapshotRejectsNilTenantEvenWithValidHash(t *testing.T) {
	p := hospitalPermitFixture(t)
	p.TenantID = "00000000-0000-0000-0000-000000000000"
	p.PermitHash, _ = p.Hash()
	state := NewHospitalGovernance("node-a")
	err := state.ApplySnapshot(HospitalSnapshot{Revision: 1, SessionEpoch: "epoch", NetworkFloor: 10, Permits: []HospitalPermitRecord{p}}, p.ValidFrom)
	require.Error(t, err)
}

func TestGovernedHospitalGovernanceAcceptanceAndAcknowledgment(t *testing.T) {
	state, permit, now := syncedHospitalFixture(t)
	change := HospitalAcceptanceChange{
		PermitID: permit.ID, PermitVersion: 1, PermitHash: permit.PermitHash,
		Decision: "ACCEPTED", AllowedFieldCodes: []string{"MCV"}, ExpiresAt: permit.ExpiresAt,
	}
	require.NoError(t, state.Accept(change, now))
	record := state.Record()
	require.EqualValues(t, 2, record.LocalRevision)
	require.Len(t, record.Acceptances, 1)
	require.Equal(t, permit.TenantID, record.Acceptances[0].TenantID)
	require.False(t, state.Synchronized())
	require.NoError(t, state.Acknowledge("epoch-a", 1, 2))
	require.True(t, state.Synchronized())

	change.ExpectedRevision = 1
	change.Decision = "DECLINED"
	require.NoError(t, state.Accept(change, now))
	require.NoError(t, state.Acknowledge("epoch-a", 1, 2))
	require.False(t, state.Synchronized(), "an old ACK cannot acknowledge a new decision")
	require.ErrorIs(t, state.Accept(change, now), ErrHospitalGovernanceConflict)

	record.Acceptances[0].AllowedFieldCodes[0] = "HB"
	require.Equal(t, []string{"MCV"}, state.Record().Acceptances[0].AllowedFieldCodes, "records must not mutate the entity")
}

func TestGovernedHospitalGovernanceAcceptanceDeniesUnknownOrWidenedScope(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*HospitalAcceptanceChange)
	}{
		{"unknown permit", func(c *HospitalAcceptanceChange) { c.PermitID = "unknown" }},
		{"wrong version", func(c *HospitalAcceptanceChange) { c.PermitVersion = 2 }},
		{"wrong hash", func(c *HospitalAcceptanceChange) { c.PermitHash = "sha256:bad" }},
		{"wider fields", func(c *HospitalAcceptanceChange) { c.AllowedFieldCodes = []string{"HB"} }},
		{"wider validity", func(c *HospitalAcceptanceChange) { c.ExpiresAt = c.ExpiresAt.Add(time.Hour) }},
		{"expired acceptance", func(c *HospitalAcceptanceChange) { c.ExpiresAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }},
		{"unknown decision", func(c *HospitalAcceptanceChange) { c.Decision = "ALLOW_ALL" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, permit, now := syncedHospitalFixture(t)
			before := state.Record()
			change := HospitalAcceptanceChange{PermitID: permit.ID, PermitVersion: 1, PermitHash: permit.PermitHash, Decision: "ACCEPTED", AllowedFieldCodes: []string{"MCV"}, ExpiresAt: permit.ExpiresAt}
			tc.edit(&change)
			require.Error(t, state.Accept(change, now))
			require.Equal(t, before, state.Record(), "a rejected change cannot mutate authority")
		})
	}
}

func TestGovernedHospitalGovernanceSnapshotFencesReconnectAndSupersededAcceptance(t *testing.T) {
	state, permit, now := syncedHospitalFixture(t)
	require.NoError(t, state.Accept(HospitalAcceptanceChange{PermitID: permit.ID, PermitVersion: 1, PermitHash: permit.PermitHash, Decision: "ACCEPTED", AllowedFieldCodes: []string{"MCV"}, ExpiresAt: permit.ExpiresAt}, now))
	require.NoError(t, state.Acknowledge("epoch-a", 1, 2))
	permit.Version = 2
	var err error
	permit.PermitHash, err = permit.Hash()
	require.NoError(t, err)
	require.NoError(t, state.ApplySnapshot(HospitalSnapshot{Revision: 2, SessionEpoch: "epoch-b", NetworkFloor: 10, Permits: []HospitalPermitRecord{permit}}, now))
	require.False(t, state.Synchronized())
	require.Error(t, state.Acknowledge("epoch-a", 1, 2))
	require.Nil(t, state.CurrentAcceptance(permit.TenantID, permit.ID, 2, now))
	require.Error(t, state.ApplySnapshot(HospitalSnapshot{Revision: 1, SessionEpoch: "epoch-b", NetworkFloor: 10}, now))
}

func TestGovernedHospitalGovernanceSnapshotValidatesHashNodeAndFloor(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*HospitalSnapshot)
	}{
		{"lower floor", func(s *HospitalSnapshot) { s.NetworkFloor = 5 }},
		{"missing epoch", func(s *HospitalSnapshot) { s.SessionEpoch = "" }},
		{"wrong hash", func(s *HospitalSnapshot) { s.Permits[0].Title = "modified" }},
		{"wrong node", func(s *HospitalSnapshot) { s.Permits[0].AllowedNodeIDs = []string{"node-b"} }},
		{"duplicates", func(s *HospitalSnapshot) { s.Permits = append(s.Permits, s.Permits[0]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := NewHospitalGovernance("node-a")
			p := hospitalPermitFixture(t)
			snapshot := HospitalSnapshot{Revision: 1, SessionEpoch: "epoch-a", NetworkFloor: 10, Permits: []HospitalPermitRecord{p}}
			tc.edit(&snapshot)
			before := state.Record()
			require.Error(t, state.ApplySnapshot(snapshot, p.ValidFrom))
			require.Equal(t, before, state.Record())
		})
	}
}

func TestGovernedHospitalGovernancePolicyOnlyAllowsBoundedAutomaticScope(t *testing.T) {
	state, _, _ := syncedHospitalFixture(t)
	policy := HospitalPolicyRecord{LocalK: 15, ReleaseMode: "AUTO", EnabledFieldCodes: []string{"MCV", "MCH", "MCV"}, FilterFieldCodes: []string{"MCV"}, PanelFieldCodes: []string{"MCH"}}
	require.NoError(t, state.UpdatePolicy(policy, 1))
	record := state.Record()
	require.EqualValues(t, 2, record.Policy.Version)
	require.Equal(t, []string{"MCH", "MCV"}, record.Policy.EnabledFieldCodes)
	require.NotEmpty(t, record.Policy.PolicyHash)
	require.ErrorIs(t, state.UpdatePolicy(policy, 1), ErrHospitalGovernanceConflict)
	for _, edit := range []func(*HospitalPolicyRecord){
		func(p *HospitalPolicyRecord) { p.LocalK = 5 },
		func(p *HospitalPolicyRecord) { p.ReleaseMode = "REMOTE" },
		func(p *HospitalPolicyRecord) { p.FilterFieldCodes = []string{"HB"} },
		func(p *HospitalPolicyRecord) { p.PanelFieldCodes = []string{"SQL"} },
	} {
		before := state.Record()
		invalid := policy
		edit(&invalid)
		require.Error(t, state.UpdatePolicy(invalid, 2))
		require.Equal(t, before, state.Record())
	}
}
