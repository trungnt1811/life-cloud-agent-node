package client_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/client"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestGovernedHospitalClientPersistsSnapshotBeforeReportAndAppliesBoundACK(t *testing.T) {
	server := newRecordingServer()
	ctrl := gomock.NewController(t)
	governance := mocks.NewMockHospitalGovernanceUseCase(ctrl)
	var stored atomic.Pointer[entities.HospitalGovernanceRecord]
	initial := entities.NewHospitalGovernance("node-a").Record()
	stored.Store(&initial)
	fences := make(chan struct{}, 8)
	governance.EXPECT().FenceConnection(gomock.Any()).DoAndReturn(func(context.Context) error {
		state := entities.NewHospitalGovernanceFromRecord(*stored.Load())
		state.FenceConnection()
		r := state.Record()
		stored.Store(&r)
		fences <- struct{}{}
		return nil
	}).AnyTimes()
	governance.EXPECT().Read(gomock.Any()).DoAndReturn(func(context.Context) (*entities.HospitalGovernanceRecord, error) { return stored.Load(), nil }).AnyTimes()
	governance.EXPECT().ApplySnapshot(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s entities.HospitalSnapshot) (*entities.HospitalGovernanceRecord, error) {
		state := entities.NewHospitalGovernanceFromRecord(*stored.Load())
		if err := state.ApplySnapshot(s, time.Now()); err != nil {
			return nil, err
		}
		r := state.Record()
		stored.Store(&r)
		return &r, nil
	})
	acked := make(chan struct{}, 1)
	governance.EXPECT().Acknowledge(gomock.Any(), "current-epoch", int64(1), int64(1)).DoAndReturn(func(_ context.Context, epoch string, snapshot, local int64) error {
		state := entities.NewHospitalGovernanceFromRecord(*stored.Load())
		if err := state.Acknowledge(epoch, snapshot, local); err != nil {
			return err
		}
		r := state.Record()
		stored.Store(&r)
		acked <- struct{}{}
		return nil
	})
	config := testConfig()
	config.GovernanceSyncEnabled = true
	nc := client.NewNodeClient(config, client.Dependencies{HospitalGovernance: governance}, nil, startTestServer(t, server))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- nc.Run(ctx) }()
	t.Cleanup(func() { cancel(); require.NoError(t, <-done) })
	select {
	case register := <-server.registers:
		require.NotEmpty(t, fences, "durable fence precedes registration")
		require.Equal(t, []string{types.GovernedCohortProfile}, register.SupportedGovernanceProfiles)
		require.Empty(t, register.SupportedReleaseModes, "metadata synchronization does not advertise execution capability")
	case <-time.After(2 * time.Second):
		t.Fatal("register timed out")
	}
	server.toSend <- &nodev1.CenterToNode{Payload: &nodev1.CenterToNode_GovernanceSnapshot{GovernanceSnapshot: &nodev1.GovernanceSnapshot{
		Profile: types.GovernedCohortProfile, MessageSchemaVersion: 1, SnapshotRevision: 1, SessionEpoch: "current-epoch", NetworkFloor: 10,
	}}}
	select {
	case report := <-server.governanceReports:
		require.Equal(t, "current-epoch", report.SessionEpoch)
		require.EqualValues(t, 1, report.AppliedSnapshotRevision)
		require.False(t, stored.Load().Synchronized(), "network send is not an ACK")
	case <-time.After(2 * time.Second):
		t.Fatal("report timed out")
	}
	server.toSend <- &nodev1.CenterToNode{Payload: &nodev1.CenterToNode_GovernanceAck{GovernanceAck: &nodev1.GovernanceAck{SessionEpoch: "current-epoch", SnapshotRevision: 1, LocalRevision: 1}}}
	select {
	case <-acked:
		require.True(t, stored.Load().Synchronized())
	case <-time.After(2 * time.Second):
		t.Fatal("ACK timed out")
	}
}

func TestGovernedHospitalClientDoesNotDialWithoutDurableGovernance(t *testing.T) {
	config := testConfig()
	config.GovernanceSyncEnabled = true
	nc := client.NewNodeClient(config, client.Dependencies{}, nil, nil)
	err := nc.Run(context.Background())
	require.ErrorContains(t, err, "governance")
}
