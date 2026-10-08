package entities

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGovernedHospitalGovernanceFenceRefusesStoredAcknowledgment(t *testing.T) {
	state, _, _ := syncedHospitalFixture(t)
	require.NoError(t, state.Acknowledge("epoch-a", 1, 1))
	state.FenceConnection()
	require.False(t, state.Synchronized())
	require.Error(t, state.Acknowledge("epoch-a", 1, 1))
}

func TestGovernedHospitalGovernanceSnapshotSupportsControlPlaneMemberLimit(t *testing.T) {
	state, p, now := syncedHospitalFixture(t)
	for range 100 {
		p.MemberPrincipalIDs = append(p.MemberPrincipalIDs, uuid.NewString())
	}
	var err error
	p.PermitHash, err = p.Hash()
	require.NoError(t, err)
	require.NoError(t, state.ApplySnapshot(HospitalSnapshot{Revision: 2, SessionEpoch: "epoch-b", NetworkFloor: 10, Permits: []HospitalPermitRecord{p}}, now))
}
