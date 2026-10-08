package entities

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGovernedHospitalQueryInvalidationSurvivesReconnectWithoutRevokingPermit(t *testing.T) {
	state, task, now := governedJobFixture(t)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	command := HospitalInvalidationCommand{CommandID: uuid.NewString(), TenantID: task.TenantID, QueryID: task.QueryID, PermitID: task.Permit.ID, ThroughVersion: task.Permit.Version, Reason: "QUERY_CANCELED", SessionEpoch: task.SessionEpoch}
	require.NoError(t, state.InvalidateQuery(command))
	require.True(t, state.QueryInvalidated(task.TenantID, task.QueryID))
	require.False(t, state.PermitInvalidated(task.TenantID, task.Permit.ID, task.Permit.Version))
	require.NoError(t, state.Acknowledge(task.SessionEpoch, 1, state.Record().LocalRevision))
	require.Error(t, job.Prepare(14, state, now.Add(time.Second)), "canceled query cannot prepare a result")
	_, err = NewGovernedHospitalJob(task, state, now)
	require.Error(t, err, "delayed or duplicate task cannot recreate canceled work")
	other := task
	other.QueryID, other.JobID = uuid.NewString(), uuid.NewString()
	_, err = NewGovernedHospitalJob(other, state, now)
	require.NoError(t, err, "cancel is scoped to this query, not the whole permit")
	before := state.Record()
	require.NoError(t, state.InvalidateQuery(command))
	require.Equal(t, before, state.Record())
	state.FenceConnection()
	require.Error(t, state.InvalidateQuery(command))
	copy := NewHospitalGovernanceFromRecord(state.Record())
	require.True(t, copy.QueryInvalidated(task.TenantID, task.QueryID))
}

func TestGovernedHospitalQueryInvalidationRejectsAmbiguousScopes(t *testing.T) {
	for _, edit := range []func(*HospitalInvalidationCommand){
		func(c *HospitalInvalidationCommand) { c.QueryID = "" },
		func(c *HospitalInvalidationCommand) { c.QueryID = "invalid" },
		func(c *HospitalInvalidationCommand) { c.Reason = "PERMIT_REVOKED" },
		func(c *HospitalInvalidationCommand) { c.SessionEpoch = "stale" },
		func(c *HospitalInvalidationCommand) { c.ThroughVersion = 0 },
	} {
		state, task, _ := governedJobFixture(t)
		command := HospitalInvalidationCommand{CommandID: uuid.NewString(), TenantID: task.TenantID, QueryID: task.QueryID, PermitID: task.Permit.ID, ThroughVersion: task.Permit.Version, Reason: "QUERY_CANCELED", SessionEpoch: task.SessionEpoch}
		edit(&command)
		before := state.Record()
		require.Error(t, state.InvalidateQuery(command))
		require.Equal(t, before, state.Record())
	}
}
