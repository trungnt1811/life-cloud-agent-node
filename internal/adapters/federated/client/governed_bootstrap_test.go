package client

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestGovernedRecoveryStartsOnlyAfterSyncAndNeverRecountsTerminalQuery(t *testing.T) {
	task, state, now := runtimeGovernedTask(t)
	job, err := entities.NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.NoError(t, job.Prepare(9, state, now.Add(time.Millisecond)))
	b, err := job.ReleaseIntent(state, now.Add(time.Millisecond))
	require.NoError(t, err)
	grant := entities.GovernedHospitalGrant{ID: uuid.NewString(), Binding: b, GrantedAt: now.Add(time.Millisecond), ExpiresAt: now.Add(time.Second)}
	require.NoError(t, job.AuthorizeSend(grant, state, now.Add(2*time.Millisecond)))
	event, err := job.OutboundEvent(now.Add(2 * time.Millisecond))
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	jobs, gov := mocks.NewMockGovernedHospitalJobUseCase(ctrl), mocks.NewMockHospitalGovernanceUseCase(ctrl)
	unsynced := state.Record()
	unsynced.AcknowledgedRevision = 0
	synced := state.Record()
	gov.EXPECT().Read(gomock.Any()).Return(&unsynced, nil)
	gov.EXPECT().Read(gomock.Any()).Return(&synced, nil).MinTimes(2)
	jobs.EXPECT().Unconfirmed(gomock.Any(), "", 100).Return([]entities.GovernedHospitalOutboundEvent{event}, nil)
	jobs.EXPECT().ResumableManual(gomock.Any(), "", 100).Return([]entities.GovernedHospitalJobRecord{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	send := make(chan *nodev1.NodeToCenter, 8)
	c := NewNodeClient(Config{NodeID: task.NodeID, GovernedExecutionEnabled: true}, Dependencies{GovernedHospitalJobs: jobs, HospitalGovernance: gov, CohortCounter: mocks.NewMockCohortCountUseCase(ctrl)}, nil, nil)
	r := newGovernedRuntime(ctx, cancel, c, send, newInflightJobs(), &wg)
	t.Cleanup(func() { cancel(); wg.Wait() })
	require.NoError(t, r.startRecovery())
	require.Empty(t, send, "reconnect alone is not synchronized authority")
	require.NoError(t, r.startRecovery())
	packet := nextGovernedPacket(t, send).GetReceiptLookup()
	require.NotNil(t, packet)
	require.Equal(t, event.Grant.ID, packet.GrantId)
	require.Equal(t, b.EventID, packet.Binding.EventId)
	require.Equal(t, b.SessionEpoch, packet.Binding.SessionEpoch, "lookup preserves historical binding")
	wg.Wait()
	require.NoError(t, r.startRecovery(), "one bootstrap scan per synchronized connection")
	require.Empty(t, send)
}

func TestGovernedRecoveryFailureClosesStreamForReconnect(t *testing.T) {
	for _, phase := range []string{"receipts", "manual-jobs"} {
		t.Run(phase, func(t *testing.T) {
			task, state, _ := runtimeGovernedTask(t)
			ctrl := gomock.NewController(t)
			jobs, gov := mocks.NewMockGovernedHospitalJobUseCase(ctrl), mocks.NewMockHospitalGovernanceUseCase(ctrl)
			synced := state.Record()
			gov.EXPECT().Read(gomock.Any()).Return(&synced, nil).Times(2)
			readError := errors.New("synthetic recovery storage failure")
			if phase == "receipts" {
				jobs.EXPECT().Unconfirmed(gomock.Any(), "", 100).Return(nil, readError)
			} else {
				jobs.EXPECT().Unconfirmed(gomock.Any(), "", 100).Return(nil, nil)
				jobs.EXPECT().ResumableManual(gomock.Any(), "", 100).Return(nil, readError)
			}
			ctx, cancel := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			send := make(chan *nodev1.NodeToCenter, 8)
			c := NewNodeClient(Config{NodeID: task.NodeID, GovernedExecutionEnabled: true}, Dependencies{GovernedHospitalJobs: jobs, HospitalGovernance: gov}, nil, nil)
			r := newGovernedRuntime(ctx, cancel, c, send, newInflightJobs(), &wg)
			t.Cleanup(func() { cancel(); wg.Wait() })
			require.NoError(t, r.startRecovery())
			wg.Wait()
			require.ErrorIs(t, ctx.Err(), context.Canceled, "failed recovery must reconnect instead of silently stopping its once-only scan")
			require.Empty(t, send)
		})
	}
}
