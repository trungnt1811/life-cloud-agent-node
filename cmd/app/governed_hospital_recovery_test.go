package app

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func TestGovernedHospitalRecoveryReadsOnlyUnconfirmedEventsWithStableCursor(t *testing.T) {
	_, u, task, now := governedHospitalUseCaseFixture(t)
	ctx := context.Background()
	_, err := u.Begin(ctx, task)
	require.NoError(t, err)
	_, err = u.Prepare(ctx, task.JobID, task.SessionEpoch, 9)
	require.NoError(t, err)
	prepared, err := u.Intent(ctx, task.JobID, task.SessionEpoch)
	require.NoError(t, err)
	items, err := u.Unconfirmed(ctx, "", 100)
	require.NoError(t, err)
	require.Empty(t, items, "PREPARED is not uncertain egress")
	grant := entities.GovernedHospitalGrant{ID: uuid.NewString(), Binding: prepared.Binding, GrantedAt: now, ExpiresAt: now.Add(10 * time.Second)}
	_, err = u.Send(ctx, task.JobID, grant)
	require.NoError(t, err)
	items, err = u.Unconfirmed(ctx, "", 1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, prepared.Binding, items[0].Binding)
	items, err = u.Unconfirmed(ctx, prepared.Binding.EventID, 1)
	require.NoError(t, err)
	require.Empty(t, items)
	_, err = u.Unconfirmed(ctx, "invalid", 100)
	require.Error(t, err)
	_, err = u.Unconfirmed(ctx, "", 101)
	require.Error(t, err)
	_, err = u.Receive(ctx, entities.GovernedHospitalReceipt{ID: uuid.NewString(), JobID: task.JobID, EventID: prepared.Binding.EventID, ProtectedPayloadDigest: prepared.Binding.ProtectedPayloadDigest, Outcome: entities.GovernedHospitalCommitted, CommittedAt: now.Add(time.Second)})
	require.NoError(t, err)
	items, err = u.Unconfirmed(ctx, "", 100)
	require.NoError(t, err)
	require.Empty(t, items, "committed history is not resent during bootstrap")
}
