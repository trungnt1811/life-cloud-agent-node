package entities

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func governedJobFixture(t *testing.T) (*HospitalGovernance, GovernedHospitalTask, time.Time) {
	t.Helper()
	state, permit, now := syncedHospitalFixture(t)
	require.NoError(t, state.UpdatePolicy(HospitalPolicyRecord{LocalK: 10, ReleaseMode: "AUTO", EnabledFieldCodes: []string{"MCV"}, FilterFieldCodes: []string{"MCV"}, PanelFieldCodes: []string{}}, 1))
	require.NoError(t, state.Accept(HospitalAcceptanceChange{PermitID: permit.ID, PermitVersion: permit.Version, PermitHash: permit.PermitHash, Decision: "ACCEPTED", AllowedFieldCodes: []string{"MCV"}, ExpiresAt: permit.ExpiresAt}, now))
	require.NoError(t, state.Acknowledge("epoch-a", 1, state.Record().LocalRevision))
	criteria := types.CohortCriteria{From: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), Conditions: []types.CohortCondition{{FieldCode: "MCV", Op: types.ComparisonOpLT, NumberValue: "80"}}}
	hash, err := criteria.GovernedDefinitionHash()
	require.NoError(t, err)
	p := state.Record().Policy
	return state, GovernedHospitalTask{JobID: uuid.NewString(), QueryID: uuid.NewString(), NodeID: "node-a", TenantID: permit.TenantID, PrincipalID: permit.PIPrincipalID, Purpose: "research", Criteria: criteria, DefinitionHash: hash, AuthorizationSnapshotHash: "sha256:" + strings.Repeat("a", 64), Permit: permit, LocalPolicyVersion: p.Version, LocalPolicyHash: p.PolicyHash, AcceptanceRevision: 1, ReleaseMode: "AUTO", EffectiveK: 10, DispatchDeadline: now.Add(30 * time.Second), QueryDeadline: now.Add(90 * time.Second), SessionEpoch: "epoch-a"}, now
}

func TestGovernedHospitalJobProtectsBeforeLedgerAndDistinguishesSendFromReceipt(t *testing.T) {
	for _, raw := range []uint64{0, 1, 9, 10, 14} {
		state, task, now := governedJobFixture(t)
		job, err := NewGovernedHospitalJob(task, state, now)
		require.NoError(t, err)
		require.Equal(t, "EXECUTING", job.Record().Phase)
		require.NoError(t, job.Prepare(raw, state, now.Add(time.Second)))
		r := job.Record()
		require.Equal(t, "READY_TO_RELEASE", r.Phase)
		require.Equal(t, "PREPARED", r.DeliveryState)
		require.NotNil(t, r.ProtectedCount)
		value, exact := r.ProtectedCount.ExactValue()
		require.Equal(t, raw == 0 || raw >= 10, exact)
		if exact {
			require.Equal(t, raw, value)
		}
		encoded, err := json.Marshal(r)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "raw_count")
		require.NotContains(t, string(encoded), "RunningCount")
		binding, err := job.ReleaseIntent(state, now.Add(time.Second))
		require.NoError(t, err)
		grant := GovernedHospitalGrant{ID: uuid.NewString(), Binding: binding, GrantedAt: now.Add(time.Second), ExpiresAt: now.Add(11 * time.Second)}
		require.NoError(t, job.AuthorizeSend(grant, state, now.Add(2*time.Second)))
		require.Equal(t, "SENT_UNCONFIRMED", job.Record().DeliveryState)
		require.Equal(t, "READY_TO_RELEASE", job.Record().Phase, "transport success is not result commitment")
		receipt := GovernedHospitalReceipt{ID: uuid.NewString(), JobID: task.JobID, EventID: binding.EventID, ProtectedPayloadDigest: binding.ProtectedPayloadDigest, Outcome: "COMMITTED", CommittedAt: now.Add(3 * time.Second)}
		require.NoError(t, job.Receive(receipt))
		require.Equal(t, "RECEIVED", job.Record().DeliveryState)
		require.Equal(t, "SUCCEEDED", job.Record().Phase)
		require.NoError(t, job.Receive(receipt), "identical receipt is idempotent")
		receipt.ProtectedPayloadDigest = "sha256:" + strings.Repeat("b", 64)
		require.Error(t, job.Receive(receipt))
	}
}

func TestGovernedHospitalJobRejectsUnauthorizedOrStaleTaskBeforeExecution(t *testing.T) {
	for _, edit := range []func(*GovernedHospitalTask){
		func(t *GovernedHospitalTask) { t.NodeID = "node-b" },
		func(t *GovernedHospitalTask) { t.SessionEpoch = "stale" },
		func(t *GovernedHospitalTask) { t.PrincipalID = uuid.NewString() },
		func(t *GovernedHospitalTask) { t.TenantID = uuid.NewString() },
		func(t *GovernedHospitalTask) { t.EffectiveK = 5 },
		func(t *GovernedHospitalTask) { t.LocalPolicyVersion++ },
		func(t *GovernedHospitalTask) { t.DefinitionHash = "sha256:" + strings.Repeat("b", 64) },
		func(t *GovernedHospitalTask) { t.ReleaseMode = "MANUAL" },
		func(t *GovernedHospitalTask) { t.DispatchDeadline = t.DispatchDeadline.Add(-time.Minute) },
		func(t *GovernedHospitalTask) { t.Criteria.Conditions[0].FieldCode = "HB" },
	} {
		state, task, now := governedJobFixture(t)
		edit(&task)
		job, err := NewGovernedHospitalJob(task, state, now)
		require.Error(t, err)
		require.Nil(t, job)
	}
}

func TestGovernedHospitalAutoTaskAcceptsSharedManualQueryLifetime(t *testing.T) {
	state, _, task, now := waitingManualJob(t)
	policy := state.Record().Policy
	policy.ReleaseMode = HospitalReleaseAuto
	require.NoError(t, state.UpdatePolicy(policy, policy.Version))
	require.NoError(t, state.Acknowledge(task.SessionEpoch, state.Record().Snapshot.Revision, state.Record().LocalRevision))
	policy = state.Record().Policy
	task.ReleaseMode, task.LocalPolicyVersion, task.LocalPolicyHash = policy.ReleaseMode, policy.Version, policy.PolicyHash
	require.Greater(t, task.QueryDeadline.Sub(now), 90*time.Second)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err, "another MANUAL target extends the shared query lifetime, not this AUTO job's execution budget")
	require.Equal(t, now.Add(30*time.Second), job.Record().ExecutionDeadline)
	require.NoError(t, job.Prepare(14, state, now.Add(time.Second)))
	require.Equal(t, GovernedHospitalReady, job.Record().Phase)
	require.Nil(t, job.Record().Approval)
	job, err = NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.Error(t, job.Prepare(14, state, now.Add(30*time.Second)), "shared approval lifetime cannot extend local execution")
}

func TestGovernedHospitalGrantAndPolicyTighteningCannotReleaseOldIntent(t *testing.T) {
	state, task, now := governedJobFixture(t)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.NoError(t, job.Prepare(14, state, now.Add(time.Second)))
	binding, err := job.ReleaseIntent(state, now.Add(time.Second))
	require.NoError(t, err)
	grant := GovernedHospitalGrant{ID: uuid.NewString(), Binding: binding, GrantedAt: now.Add(time.Second), ExpiresAt: now.Add(12 * time.Second)}
	require.Error(t, job.AuthorizeSend(grant, state, now.Add(2*time.Second)), "grant window cannot exceed 10 seconds")
	grant.ExpiresAt = now.Add(11 * time.Second)
	require.Error(t, job.AuthorizeSend(grant, state, grant.ExpiresAt), "expiry is exclusive")
	require.NoError(t, state.UpdatePolicy(HospitalPolicyRecord{LocalK: 20, ReleaseMode: "AUTO", EnabledFieldCodes: []string{"MCV"}, FilterFieldCodes: []string{"MCV"}, PanelFieldCodes: []string{}}, 2))
	require.Error(t, job.AuthorizeSend(grant, state, now.Add(2*time.Second)))
	require.NoError(t, state.Acknowledge("epoch-a", 1, state.Record().LocalRevision))
	updated, err := job.ReleaseIntent(state, now.Add(2*time.Second))
	require.NoError(t, err)
	require.NotEqual(t, binding.EventID, updated.EventID)
	_, exact := job.Record().ProtectedCount.ExactValue()
	require.False(t, exact, "old visible 14 must be suppressed under k=20")
	require.NoError(t, state.UpdatePolicy(HospitalPolicyRecord{LocalK: 10, ReleaseMode: "AUTO", EnabledFieldCodes: []string{"MCV"}, FilterFieldCodes: []string{"MCV"}, PanelFieldCodes: []string{}}, 3))
	require.NoError(t, state.Acknowledge("epoch-a", 1, state.Record().LocalRevision))
	_, err = job.ReleaseIntent(state, now.Add(3*time.Second))
	require.NoError(t, err)
	_, exact = job.Record().ProtectedCount.ExactValue()
	require.False(t, exact, "lowering k cannot reconstruct a hidden raw value")
}

func TestGovernedHospitalUncertainSendRequiresReceiptReconciliationAndSurvivesHydration(t *testing.T) {
	state, task, now := governedJobFixture(t)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.NoError(t, job.Prepare(9, state, now.Add(time.Second)))
	binding, err := job.ReleaseIntent(state, now.Add(time.Second))
	require.NoError(t, err)
	require.NoError(t, job.AuthorizeSend(GovernedHospitalGrant{ID: uuid.NewString(), Binding: binding, GrantedAt: now.Add(time.Second), ExpiresAt: now.Add(11 * time.Second)}, state, now.Add(2*time.Second)))
	bytes, err := json.Marshal(job.Record())
	require.NoError(t, err)
	var stored GovernedHospitalJobRecord
	require.NoError(t, json.Unmarshal(bytes, &stored))
	restored, err := NewGovernedHospitalJobFromRecord(stored)
	require.NoError(t, err)
	_, err = restored.ReleaseIntent(state, now.Add(3*time.Second))
	require.Error(t, err, "uncertain egress cannot request a new event/grant before reconciliation")
	require.Equal(t, binding, restored.Record().Binding)
	require.Equal(t, job.Record().ProtectedCount, restored.Record().ProtectedCount)
}

func TestGovernedHospitalExpiredGrantReconciliationPermitsOnlyAFreshIntent(t *testing.T) {
	state, task, now := governedJobFixture(t)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.NoError(t, job.Prepare(9, state, now.Add(time.Second)))
	binding, err := job.ReleaseIntent(state, now.Add(time.Second))
	require.NoError(t, err)
	grant := GovernedHospitalGrant{ID: uuid.NewString(), Binding: binding, GrantedAt: now.Add(time.Second), ExpiresAt: now.Add(11 * time.Second)}
	require.NoError(t, job.AuthorizeSend(grant, state, now.Add(2*time.Second)))
	require.NoError(t, job.Receive(GovernedHospitalReceipt{ID: uuid.NewString(), JobID: task.JobID, EventID: binding.EventID, ProtectedPayloadDigest: binding.ProtectedPayloadDigest, Outcome: "REJECTED", Reason: "GRANT_EXPIRED", CommittedAt: now.Add(12 * time.Second)}))
	fresh, err := job.ReleaseIntent(state, now.Add(13*time.Second))
	require.NoError(t, err, "confirmed non-commit may request new authority, not reuse the expired grant")
	require.NotEqual(t, binding.EventID, fresh.EventID)
	require.Nil(t, job.Record().Grant)
	require.Nil(t, job.Record().Receipt)
	require.Equal(t, "PREPARED", job.Record().DeliveryState)
	require.Error(t, job.AuthorizeSend(grant, state, now.Add(13*time.Second)))
}

func TestGovernedHospitalReconnectDoesNotExtendExecutionDeadline(t *testing.T) {
	const newEpoch = "epoch-b"
	state, task, now := governedJobFixture(t)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	deadline := job.Record().ExecutionDeadline
	state.FenceConnection()
	snapshot := state.Record().Snapshot
	snapshot.SessionEpoch, snapshot.Revision = newEpoch, snapshot.Revision+1
	require.NoError(t, state.ApplySnapshot(snapshot, now.Add(10*time.Second)))
	require.NoError(t, state.Acknowledge(newEpoch, snapshot.Revision, state.Record().LocalRevision))
	task.SessionEpoch = newEpoch
	require.NoError(t, job.Resume(task, state, now.Add(10*time.Second)))
	require.Equal(t, deadline, job.Record().ExecutionDeadline)
	require.NoError(t, job.Prepare(14, state, now.Add(11*time.Second)))
	_, err = job.ReleaseIntent(state, now.Add(12*time.Second))
	require.NoError(t, err)
	require.Equal(t, newEpoch, job.Record().Binding.SessionEpoch)
}

func TestGovernedHospitalPausePreventsFreshExecutionAndUnusedGrant(t *testing.T) {
	state, task, now := governedJobFixture(t)
	job, err := NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.NoError(t, job.Prepare(14, state, now.Add(time.Second)))
	binding, err := job.ReleaseIntent(state, now.Add(time.Second))
	require.NoError(t, err)
	require.NoError(t, state.ChangeAvailability(true, 1))
	require.NoError(t, state.Acknowledge("epoch-a", 1, state.Record().LocalRevision))
	_, err = NewGovernedHospitalJob(task, state, now.Add(2*time.Second))
	require.Error(t, err)
	grant := GovernedHospitalGrant{ID: uuid.NewString(), Binding: binding, GrantedAt: now.Add(time.Second), ExpiresAt: now.Add(11 * time.Second)}
	require.Error(t, job.AuthorizeSend(grant, state, now.Add(2*time.Second)), "a pause fences an already issued unused grant")
}
