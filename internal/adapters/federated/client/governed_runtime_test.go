package client

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func runtimeGovernedTask(t *testing.T) (entities.GovernedHospitalTask, *entities.HospitalGovernance, time.Time) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	principal, tenant := uuid.NewString(), uuid.NewString()
	permit := entities.HospitalPermitRecord{ID: "runtime-study", TenantID: tenant, Version: 1, Status: "ACTIVE", HospitalPermitScope: entities.HospitalPermitScope{Title: "Synthetic runtime test", PIPrincipalID: principal, MemberPrincipalIDs: []string{principal}, Purpose: "research", JobType: "COUNT_MATCHING_COHORT", JobVersion: "v1", AllowedFieldCodes: []string{"MCV"}, AllowedNodeIDs: []string{"node-a"}, EthicsReference: "synthetic-only", ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}}
	var err error
	permit.PermitHash, err = permit.Hash()
	require.NoError(t, err)
	state := entities.NewHospitalGovernance("node-a")
	require.NoError(t, state.ApplySnapshot(entities.HospitalSnapshot{SessionEpoch: "epoch-a", Revision: 1, NetworkFloor: 10, Permits: []entities.HospitalPermitRecord{permit}}, now))
	require.NoError(t, state.UpdatePolicy(entities.HospitalPolicyRecord{LocalK: 10, ReleaseMode: "AUTO", EnabledFieldCodes: []string{"MCV"}, FilterFieldCodes: []string{"MCV"}}, 1))
	require.NoError(t, state.Accept(entities.HospitalAcceptanceChange{PermitID: permit.ID, PermitVersion: 1, PermitHash: permit.PermitHash, Decision: "ACCEPTED", AllowedFieldCodes: []string{"MCV"}, ExpiresAt: permit.ExpiresAt}, now))
	require.NoError(t, state.Acknowledge("epoch-a", 1, state.Record().LocalRevision))
	criteria := types.CohortCriteria{From: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), Conditions: []types.CohortCondition{{FieldCode: "MCV", Op: types.ComparisonOpLT, NumberValue: "80"}}}
	hash, err := criteria.GovernedDefinitionHash()
	require.NoError(t, err)
	p := state.Record().Policy
	return entities.GovernedHospitalTask{JobID: uuid.NewString(), QueryID: uuid.NewString(), NodeID: "node-a", TenantID: tenant, PrincipalID: principal, Purpose: "research", Criteria: criteria, DefinitionHash: hash, AuthorizationSnapshotHash: "sha256:" + strings.Repeat("a", 64), Permit: permit, LocalPolicyVersion: p.Version, LocalPolicyHash: p.PolicyHash, AcceptanceRevision: 1, ReleaseMode: "AUTO", EffectiveK: 10, DispatchDeadline: now.Add(30 * time.Second), QueryDeadline: now.Add(90 * time.Second), SessionEpoch: "epoch-a"}, state, now
}

func nextGovernedPacket(t *testing.T, send <-chan *nodev1.NodeToCenter) *nodev1.NodeToCenter {
	t.Helper()
	select {
	case p := <-send:
		return p
	case <-time.After(3 * time.Second):
		t.Fatal("governed packet timed out")
		return nil
	}
}

func TestGovernedRuntimeWaitsForDurableStagesAndLedgerBeforeProtectedEgress(t *testing.T) {
	task, state, now := runtimeGovernedTask(t)
	job, err := entities.NewGovernedHospitalJob(task, state, now)
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	jobs, gov, counter := mocks.NewMockGovernedHospitalJobUseCase(ctrl), mocks.NewMockHospitalGovernanceUseCase(ctrl), mocks.NewMockCohortCountUseCase(ctrl)
	gov.EXPECT().Read(gomock.Any()).DoAndReturn(func(context.Context) (*entities.HospitalGovernanceRecord, error) { r := state.Record(); return &r, nil }).AnyTimes()
	jobs.EXPECT().Read(gomock.Any(), task.JobID).Return(nil, nil)
	jobs.EXPECT().Begin(gomock.Any(), task).DoAndReturn(func(context.Context, entities.GovernedHospitalTask) (*entities.GovernedHospitalJobRecord, error) {
		r := job.Record()
		return &r, nil
	})
	counted := make(chan struct{}, 1)
	ackExecution := jobs.EXPECT().AcknowledgeStage(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, a entities.GovernedHospitalStageAck) (*entities.GovernedHospitalJobRecord, error) {
		err := job.AcknowledgeStage(a)
		r := job.Record()
		return &r, err
	})
	countCall := counter.EXPECT().CountMatchingCohort(gomock.Any(), task.JobID, task.Criteria).DoAndReturn(func(context.Context, string, types.CohortCriteria) (uint64, error) {
		counted <- struct{}{}
		return 9, nil
	}).After(ackExecution)
	prepare := jobs.EXPECT().Prepare(gomock.Any(), task.JobID, task.SessionEpoch, uint64(9)).DoAndReturn(func(context.Context, string, string, uint64) (*entities.GovernedHospitalJobRecord, error) {
		err := job.Prepare(9, state, time.Now())
		r := job.Record()
		return &r, err
	}).After(countCall)
	intent := jobs.EXPECT().Intent(gomock.Any(), task.JobID, task.SessionEpoch).DoAndReturn(func(context.Context, string, string) (*entities.GovernedHospitalJobRecord, error) {
		_, err := job.ReleaseIntent(state, time.Now())
		r := job.Record()
		return &r, err
	}).After(prepare)
	ackReady := jobs.EXPECT().AcknowledgeStage(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, a entities.GovernedHospitalStageAck) (*entities.GovernedHospitalJobRecord, error) {
		err := job.AcknowledgeStage(a)
		r := job.Record()
		return &r, err
	}).After(intent)
	sent := jobs.EXPECT().Send(gomock.Any(), task.JobID, gomock.Any()).DoAndReturn(func(_ context.Context, _ string, g entities.GovernedHospitalGrant) (*entities.GovernedHospitalJobRecord, error) {
		err := job.AuthorizeSend(g, state, time.Now())
		r := job.Record()
		return &r, err
	}).After(ackReady)
	jobs.EXPECT().Receive(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r entities.GovernedHospitalReceipt) (*entities.GovernedHospitalJobRecord, error) {
		err := job.Receive(r)
		record := job.Record()
		return &record, err
	}).After(sent)
	ctx, cancel := context.WithCancel(context.Background())
	send := make(chan *nodev1.NodeToCenter, 8)
	var wg sync.WaitGroup
	nc := NewNodeClient(Config{NodeID: "node-a", GovernedExecutionEnabled: true}, Dependencies{GovernedHospitalJobs: jobs, HospitalGovernance: gov, CohortCounter: counter}, nil, nil)
	runtime := newGovernedRuntime(ctx, cancel, nc, send, newInflightJobs(), &wg)
	t.Cleanup(func() { cancel(); wg.Wait() })
	require.NoError(t, runtime.start(task))
	stage := nextGovernedPacket(t, send).GetJobStage()
	require.NotNil(t, stage)
	select {
	case <-counted:
		t.Fatal("counter ran before durable execution ACK")
	default:
	}
	ack := &nodev1.StageAck{JobId: task.JobID, EventId: stage.Binding.EventId, Sequence: stage.Sequence, SessionEpoch: task.SessionEpoch, ExecutionDeadline: timestamppb.New(now.Add(20 * time.Second)), QueryDeadline: timestamppb.New(task.QueryDeadline)}
	_, err = runtime.handle(&nodev1.CenterToNode{Payload: &nodev1.CenterToNode_StageAck{StageAck: ack}})
	require.NoError(t, err)
	stage = nextGovernedPacket(t, send).GetJobStage()
	require.Equal(t, nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_READY_TO_RELEASE, stage.GetPhase())
	ack.EventId, ack.Sequence = stage.Binding.EventId, stage.Sequence
	_, err = runtime.handle(&nodev1.CenterToNode{Payload: &nodev1.CenterToNode_StageAck{StageAck: ack}})
	require.NoError(t, err)
	request := nextGovernedPacket(t, send).GetReleaseRequest()
	require.NotNil(t, request)
	grant := &nodev1.ReleaseGrant{GrantId: uuid.NewString(), Binding: request.Binding, GrantedAt: timestamppb.Now(), ExpiresAt: timestamppb.New(time.Now().Add(9 * time.Second))}
	_, err = runtime.handle(&nodev1.CenterToNode{Payload: &nodev1.CenterToNode_ReleaseDecision{ReleaseDecision: &nodev1.ReleaseDecision{Outcome: &nodev1.ReleaseDecision_Grant{Grant: grant}}}})
	require.NoError(t, err)
	result := nextGovernedPacket(t, send).GetGovernedQueryResult()
	require.NotNil(t, result)
	require.Equal(t, nodev1.GovernedJobPhase_GOVERNED_JOB_PHASE_READY_TO_RELEASE, stage.Phase)
	require.Equal(t, entities.GovernedHospitalSentUnconfirmed, job.Record().DeliveryState)
	require.Nil(t, job.Record().Receipt)
	require.IsType(t, &nodev1.ProtectedCohortPayload_SuppressedCount{}, result.ProtectedPayload.Count)
	require.EqualValues(t, 1, result.ProtectedPayload.GetSuppressedCount().LowerBound)
	require.EqualValues(t, 9, result.ProtectedPayload.GetSuppressedCount().UpperBound)
	receipt := &nodev1.ResultReceipt{ReceiptId: uuid.NewString(), JobId: task.JobID, EventId: result.Binding.EventId, ProtectedPayloadDigest: result.Binding.ProtectedPayloadDigest, Outcome: nodev1.ResultReceiptOutcome_RESULT_RECEIPT_OUTCOME_COMMITTED, CommittedAt: timestamppb.Now()}
	_, err = runtime.handle(&nodev1.CenterToNode{Payload: &nodev1.CenterToNode_ResultReceipt{ResultReceipt: receipt}})
	require.NoError(t, err)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("governed actor did not finish after durable receipt")
	}
	require.Equal(t, entities.GovernedHospitalSucceeded, job.Record().Phase)
}
