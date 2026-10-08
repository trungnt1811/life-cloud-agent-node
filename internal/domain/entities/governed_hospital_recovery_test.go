package entities

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGovernedHospitalPolicyChangedReceiptAllowsOnlyFreshProtectedIntent(t *testing.T) {
	state, task, now := governedJobFixture(t)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.NoError(t, job.Prepare(14, state, now.Add(time.Second)))
	binding, err := job.ReleaseIntent(state, now.Add(time.Second))
	require.NoError(t, err)
	require.NoError(t, job.AuthorizeSend(GovernedHospitalGrant{ID: uuid.NewString(), Binding: binding, GrantedAt: now.Add(time.Second), ExpiresAt: now.Add(11 * time.Second)}, state, now.Add(2*time.Second)))
	require.NoError(t, state.UpdatePolicy(HospitalPolicyRecord{LocalK: 20, ReleaseMode: "AUTO", EnabledFieldCodes: []string{"MCV"}, FilterFieldCodes: []string{"MCV"}, PanelFieldCodes: []string{}}, 2))
	require.NoError(t, state.Acknowledge("epoch-a", 1, state.Record().LocalRevision))
	receipt := GovernedHospitalReceipt{ID: uuid.NewString(), JobID: task.JobID, EventID: binding.EventID, ProtectedPayloadDigest: binding.ProtectedPayloadDigest, Outcome: GovernedHospitalRejected, Reason: "LOCAL_POLICY_CHANGED", CommittedAt: now.Add(3 * time.Second)}
	require.NoError(t, job.Receive(receipt))
	require.Equal(t, GovernedHospitalReady, job.Record().Phase, "confirmed non-commit may re-protect, never resend uncertain prior bytes")
	fresh, err := job.ReleaseIntent(state, now.Add(4*time.Second))
	require.NoError(t, err)
	require.NotEqual(t, binding.EventID, fresh.EventID)
	_, exact := job.Record().ProtectedCount.ExactValue()
	require.False(t, exact)
	require.EqualValues(t, 20, job.Record().ProtectedCount.EffectiveK())
	require.Nil(t, job.Record().Grant)
	require.Nil(t, job.Record().Receipt)
}

func TestGovernedHospitalHydrationRejectsContradictoryReleaseHistory(t *testing.T) {
	for _, corrupt := range []string{"success-without-receipt", "send-without-grant", "zero-deadline", "deadline-after-query", "receipt-before-grant"} {
		t.Run(corrupt, func(t *testing.T) {
			state, task, now := governedJobFixture(t)
			job, err := NewGovernedHospitalJob(task, state, now)
			require.NoError(t, err)
			require.NoError(t, job.Prepare(14, state, now.Add(time.Second)))
			binding, err := job.ReleaseIntent(state, now.Add(time.Second))
			require.NoError(t, err)
			require.NoError(t, job.AuthorizeSend(GovernedHospitalGrant{ID: uuid.NewString(), Binding: binding, GrantedAt: now.Add(time.Second), ExpiresAt: now.Add(11 * time.Second)}, state, now.Add(2*time.Second)))
			record := job.Record()
			switch corrupt {
			case "success-without-receipt":
				record.Phase, record.DeliveryState = "SUCCEEDED", "RECEIVED"
			case "send-without-grant":
				record.Grant = nil
			case "zero-deadline":
				record.ExecutionDeadline = time.Time{}
			case "deadline-after-query":
				record.Task.QueryDeadline = now.Add(2 * time.Second)
				record.Fingerprint, err = record.Task.Fingerprint()
				require.NoError(t, err)
			case "receipt-before-grant":
				record.Phase, record.DeliveryState = "SUCCEEDED", "RECEIVED"
				record.Receipt = &GovernedHospitalReceipt{ID: uuid.NewString(), JobID: task.JobID, EventID: binding.EventID, ProtectedPayloadDigest: binding.ProtectedPayloadDigest, Outcome: "COMMITTED", CommittedAt: now}
			}
			_, err = NewGovernedHospitalJobFromRecord(record)
			require.Error(t, err)
		})
	}
}
