package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGovernedHospitalPermitInvalidationIsMonotonicAndSessionBound(t *testing.T) {
	state := NewHospitalGovernance("node-a")
	r := state.Record()
	r.Snapshot.SessionEpoch = "current"
	state = NewHospitalGovernanceFromRecord(r)
	command := HospitalInvalidationCommand{CommandID: "6c990c65-61b0-4311-86f9-6e4b6beb53cc", TenantID: "53a96e1c-87aa-4def-b926-675e7e74df6c", PermitID: "demo", ThroughVersion: 2, Reason: "PERMIT_SUPERSEDED", SessionEpoch: "current"}
	require.NoError(t, state.InvalidatePermit(command))
	require.True(t, state.PermitInvalidated(command.TenantID, command.PermitID, 1))
	require.True(t, state.PermitInvalidated(command.TenantID, command.PermitID, 2))
	require.False(t, state.PermitInvalidated(command.TenantID, command.PermitID, 3))
	require.False(t, state.PermitInvalidated("another-tenant", command.PermitID, 1))
	before := state.Record()
	require.NoError(t, state.InvalidatePermit(command))
	require.Equal(t, before, state.Record())
	command.ThroughVersion = 1
	require.NoError(t, state.InvalidatePermit(command))
	require.Equal(t, before, state.Record(), "old invalidation cannot remove a newer fence")
	command.SessionEpoch = "old"
	require.Error(t, state.InvalidatePermit(command))
	require.Equal(t, before, state.Record())
}
