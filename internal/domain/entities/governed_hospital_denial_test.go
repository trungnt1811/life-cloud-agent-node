package entities

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGovernedHospitalGrantDenialIsNotAReceiptAndRequiresFreshIntent(t *testing.T) {
	for _, reason := range []string{"GRANT_EXPIRED", "SESSION_STALE", "LOCAL_POLICY_CHANGED", "PERMIT_REVOKED"} {
		state, task, now := governedJobFixture(t)
		job, err := NewGovernedHospitalJob(task, state, now)
		require.NoError(t, err)
		require.NoError(t, job.Prepare(9, state, now.Add(time.Second)))
		binding, err := job.ReleaseIntent(state, now.Add(time.Second))
		require.NoError(t, err)
		denial := GovernedHospitalDenial{Binding: binding, Reason: reason}
		require.NoError(t, job.DenyGrant(denial, now.Add(2*time.Second)))
		r := job.Record()
		require.Equal(t, GovernedHospitalRejected, r.DeliveryState)
		require.Nil(t, r.Grant)
		require.Nil(t, r.Receipt, "grant refusal is not proof of received egress")
		require.Equal(t, denial, *r.Denial)
		require.NoError(t, job.DenyGrant(denial, now.Add(3*time.Second)))
		require.Equal(t, r, job.Record(), "identical denial is a no-op")
		encoded, err := json.Marshal(r)
		require.NoError(t, err)
		var stored GovernedHospitalJobRecord
		require.NoError(t, json.Unmarshal(encoded, &stored))
		restored, err := NewGovernedHospitalJobFromRecord(stored)
		require.NoError(t, err)
		changed := denial
		changed.Reason = "INTERNAL_FAILURE"
		require.Error(t, restored.DenyGrant(changed, now.Add(3*time.Second)))
		if reason == "PERMIT_REVOKED" {
			require.Equal(t, GovernedHospitalPolicyDenied, r.Phase)
			_, err = restored.ReleaseIntent(state, now.Add(4*time.Second))
			require.Error(t, err)
		} else {
			require.Equal(t, GovernedHospitalReady, r.Phase)
			fresh, err := restored.ReleaseIntent(state, now.Add(4*time.Second))
			require.NoError(t, err)
			require.NotEqual(t, binding.EventID, fresh.EventID)
			require.Nil(t, restored.Record().Denial)
		}
	}
}

func TestGovernedHospitalGrantDenialCannotResetAnUncertainSend(t *testing.T) {
	state, task, now := governedJobFixture(t)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.NoError(t, job.Prepare(14, state, now.Add(time.Second)))
	binding, err := job.ReleaseIntent(state, now.Add(time.Second))
	require.NoError(t, err)
	denial := GovernedHospitalDenial{Binding: binding, Reason: "GRANT_EXPIRED"}
	bad := denial
	bad.Binding.EventID = uuid.NewString()
	require.Error(t, job.DenyGrant(bad, now.Add(2*time.Second)))
	bad = denial
	bad.Reason = "private database exception"
	require.Error(t, job.DenyGrant(bad, now.Add(2*time.Second)))
	require.NoError(t, job.AuthorizeSend(GovernedHospitalGrant{ID: uuid.NewString(), Binding: binding, GrantedAt: now.Add(time.Second), ExpiresAt: now.Add(11 * time.Second)}, state, now.Add(2*time.Second)))
	before := job.Record()
	require.Error(t, job.DenyGrant(denial, now.Add(3*time.Second)))
	require.Equal(t, before, job.Record())
}
