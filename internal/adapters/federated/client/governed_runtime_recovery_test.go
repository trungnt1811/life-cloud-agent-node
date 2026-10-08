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
	"google.golang.org/protobuf/types/known/timestamppb"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestGovernedRuntimeReconcilesUncertainEgressWithoutReplayingCount(t *testing.T) {
	task, state, now := runtimeGovernedTask(t)
	job, err := entities.NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	require.NoError(t, job.Prepare(9, state, now.Add(time.Millisecond)))
	b, err := job.ReleaseIntent(state, now.Add(time.Millisecond))
	require.NoError(t, err)
	require.NoError(t, job.AuthorizeSend(entities.GovernedHospitalGrant{ID: uuid.NewString(), Binding: b, GrantedAt: now.Add(time.Millisecond), ExpiresAt: now.Add(time.Second)}, state, now.Add(2*time.Millisecond)))
	stored := job.Record()
	ctrl := gomock.NewController(t)
	jobs := mocks.NewMockGovernedHospitalJobUseCase(ctrl)
	jobs.EXPECT().Read(gomock.Any(), task.JobID).Return(&stored, nil)
	jobs.EXPECT().Receive(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, proof entities.GovernedHospitalReceipt) (*entities.GovernedHospitalJobRecord, error) {
		err := job.Receive(proof)
		r := job.Record()
		return &r, err
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	send := make(chan *nodev1.NodeToCenter, 8)
	nc := NewNodeClient(Config{NodeID: task.NodeID, GovernedExecutionEnabled: true}, Dependencies{GovernedHospitalJobs: jobs, CohortCounter: mocks.NewMockCohortCountUseCase(ctrl)}, nil, nil)
	r := newGovernedRuntime(ctx, cancel, nc, send, newInflightJobs(), &wg)
	t.Cleanup(func() { cancel(); wg.Wait() })
	require.NoError(t, r.start(task))
	packet := nextGovernedPacket(t, send).GetReceiptLookup()
	require.NotNil(t, packet)
	require.Equal(t, stored.Grant.ID, packet.GrantId)
	require.Equal(t, stored.Binding.EventID, packet.Binding.EventId)
	require.Equal(t, stored.Binding.ProtectedPayloadDigest, packet.Binding.ProtectedPayloadDigest)
	proof := &nodev1.ResultReceipt{ReceiptId: uuid.NewString(), JobId: task.JobID, EventId: stored.Binding.EventID, ProtectedPayloadDigest: stored.Binding.ProtectedPayloadDigest, Outcome: nodev1.ResultReceiptOutcome_RESULT_RECEIPT_OUTCOME_COMMITTED, CommittedAt: timestamppb.New(now.Add(3 * time.Millisecond))}
	_, err = r.handle(&nodev1.CenterToNode{Payload: &nodev1.CenterToNode_ResultReceipt{ResultReceipt: proof}})
	require.NoError(t, err)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("receipt reconciliation did not complete")
	}
	require.Empty(t, send, "a prior send intent is not permission to replay bytes after expiry")
	require.Equal(t, entities.GovernedHospitalSucceeded, job.Record().Phase)
}

func TestGovernedRuntimeOwnsTaskDataAndRefusesChangedDuplicate(t *testing.T) {
	task, _, _ := runtimeGovernedTask(t)
	ctrl := gomock.NewController(t)
	jobs := mocks.NewMockGovernedHospitalJobUseCase(ctrl)
	entered, proceed := make(chan struct{}), make(chan struct{})
	jobs.EXPECT().Read(gomock.Any(), task.JobID).DoAndReturn(func(context.Context, string) (*entities.GovernedHospitalJobRecord, error) {
		close(entered)
		<-proceed
		return nil, nil
	})
	got := make(chan entities.GovernedHospitalTask, 1)
	jobs.EXPECT().Begin(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, in entities.GovernedHospitalTask) (*entities.GovernedHospitalJobRecord, error) {
		got <- in
		return nil, errors.New("isolated test stop")
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	r := newGovernedRuntime(ctx, cancel, NewNodeClient(Config{NodeID: task.NodeID}, Dependencies{GovernedHospitalJobs: jobs}, nil, nil), make(chan *nodev1.NodeToCenter, 8), newInflightJobs(), &wg)
	require.NoError(t, r.start(task))
	<-entered
	require.NoError(t, r.start(task), "identical inflight delivery does not spawn another counter")
	task.Criteria.Conditions[0].NumberValue = "70"
	hash, err := task.Criteria.GovernedDefinitionHash()
	require.NoError(t, err)
	task.DefinitionHash = hash
	require.Error(t, r.start(task), "changed work under one job ID must not be silently ignored")
	close(proceed)
	received := <-got
	require.Equal(t, "80", received.Criteria.Conditions[0].NumberValue, "actor must own a defensive copy of caller-owned criteria")
	wg.Wait()
}

func TestGovernedExecutionConfigRefusesMissingDurabilityAndPlaintext(t *testing.T) {
	c := Config{Address: "cp:9090", NodeID: "node-a", GovernanceSyncEnabled: true, GovernedExecutionEnabled: true, TLS: TLSConfig{Insecure: true}}
	require.ErrorContains(t, c.Validate(), "mutual TLS")
	nc := NewNodeClient(c, Dependencies{}, nil, nil)
	require.ErrorContains(t, nc.Run(context.Background()), "durable governed jobs")
}
