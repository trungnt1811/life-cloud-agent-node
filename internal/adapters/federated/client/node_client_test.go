package client_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/test/bufconn"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/client"
	federatedwire "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/wire"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

const bufSize = 1 << 20

// recordingNodeControlServer is an in-process NodeControl server that records
// what the client sends and can push scripted messages back. It is a real
// implementation of the generated server interface driven over bufconn, not
// a hand-rolled substitute for the client under test.
type recordingNodeControlServer struct {
	nodev1.UnimplementedNodeControlServer

	registers         chan *nodev1.Register
	results           chan *nodev1.QueryResult
	governanceReports chan *nodev1.GovernanceReport
	toSend            chan *nodev1.CenterToNode
	dropCount         atomic.Int32
	connOpened        chan struct{}
}

func newRecordingServer() *recordingNodeControlServer {
	return &recordingNodeControlServer{
		registers:         make(chan *nodev1.Register, 8),
		results:           make(chan *nodev1.QueryResult, 8),
		governanceReports: make(chan *nodev1.GovernanceReport, 16),
		toSend:            make(chan *nodev1.CenterToNode, 8),
		connOpened:        make(chan struct{}, 8),
	}
}

func (s *recordingNodeControlServer) Connect(stream nodev1.NodeControl_ConnectServer) error {
	s.connOpened <- struct{}{}
	drop := s.dropCount.Load() > 0
	if drop {
		s.dropCount.Add(-1)
	}

	errCh := make(chan error, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				errCh <- err
				return
			}
			switch payload := msg.GetPayload().(type) {
			case *nodev1.NodeToCenter_Register:
				s.registers <- payload.Register
				if drop {
					errCh <- errors.New("simulated drop after register")
					return
				}
			case *nodev1.NodeToCenter_QueryResult:
				s.results <- payload.QueryResult
			case *nodev1.NodeToCenter_GovernanceReport:
				select {
				case s.governanceReports <- payload.GovernanceReport:
				case <-stream.Context().Done():
					return
				}
			case *nodev1.NodeToCenter_Heartbeat:
				// Not asserted on directly in these tests; draining keeps
				// the client's send loop from blocking.
			}
		}
	}()

	for {
		select {
		case err := <-errCh:
			return err
		case msg := <-s.toSend:
			if err := stream.Send(msg); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func startTestServer(t *testing.T, server *recordingNodeControlServer) client.Dialer {
	t.Helper()

	listener := bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()
	nodev1.RegisterNodeControlServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)

	return func(_ string, creds credentials.TransportCredentials) (*grpc.ClientConn, error) {
		return grpc.NewClient("passthrough:///bufnet",
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
			grpc.WithTransportCredentials(creds),
		)
	}
}

func testConfig() client.Config {
	return client.Config{
		Address:            "bufnet",
		NodeID:             "node-a",
		AgentVersion:       "test-version",
		QuerySchemaVersion: 1,
		TLS:                client.TLSConfig{Insecure: true},
		HeartbeatInterval:  20 * time.Millisecond,
		ConnectTimeout:     time.Second,
		InitialBackoff:     10 * time.Millisecond,
		MaxBackoff:         50 * time.Millisecond,
	}
}

func allEnabledFieldsRepoExpectation(ctrl *gomock.Controller) *mocks.MockEnabledQueryFieldRepository {
	repo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	fields := make([]*entities.EnabledQueryField, 0, 9)
	for _, code := range []string{"HB", "MCV", "MCH", "RBC", "MCHC", "RDW", "HBA0", "HBA2", "HBF"} {
		fields = append(fields, entities.NewEnabledQueryField(code, true, "test", now))
	}
	repo.EXPECT().ListAll(gomock.Any()).Return(fields, nil).AnyTimes()
	return repo
}

func TestNodeClient_RegistersOnConnect(t *testing.T) {
	server := newRecordingServer()
	dial := startTestServer(t, server)
	config := testConfig()

	ctrl := gomock.NewController(t)
	nc := client.NewNodeClient(config, client.Dependencies{
		EnabledQueryFieldRepo: allEnabledFieldsRepoExpectation(ctrl),
		CohortCounter:         mocks.NewMockCohortCountUseCase(ctrl),
	}, nil, dial)

	ctx, cancel := context.WithCancel(context.Background())
	runErrCh := make(chan error, 1)
	go func() { runErrCh <- nc.Run(ctx) }()

	select {
	case register := <-server.registers:
		require.Equal(t, "node-a", register.GetNodeId())
		require.Equal(t, "test-version", register.GetAgentVersion())
		require.Equal(t, uint32(1), register.GetQuerySchemaVersion())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Register")
	}

	cancel()
	select {
	case err := <-runErrCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx cancellation")
	}
}

func TestNodeClient_ExecutesQueryTaskAndSendsResult(t *testing.T) {
	server := newRecordingServer()
	dial := startTestServer(t, server)
	config := testConfig()

	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	task.JobId = "job-integration-1"

	ctrl := gomock.NewController(t)
	counter := mocks.NewMockCohortCountUseCase(ctrl)
	counter.EXPECT().
		CountMatchingCohort(gomock.Any(), "job-integration-1", gomock.Any()).
		Return(uint64(2), nil)

	nc := client.NewNodeClient(config, client.Dependencies{
		EnabledQueryFieldRepo: allEnabledFieldsRepoExpectation(ctrl),
		CohortCounter:         counter,
		SuppressionThreshold:  1, // hides nothing, for a deterministic assertion
	}, nil, dial)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = nc.Run(ctx) }()

	<-server.registers
	server.toSend <- &nodev1.CenterToNode{Payload: &nodev1.CenterToNode_QueryTask{QueryTask: task}}

	select {
	case result := <-server.results:
		require.Equal(t, "job-integration-1", result.GetJobId())
		require.Equal(t, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_OK, result.GetStatus())
		require.Equal(t, uint64(2), result.GetMatchingCount())
		require.False(t, result.GetSuppressed())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for QueryResult")
	}
}

func TestNodeClient_DuplicateTaskDoesNotExecuteConcurrently(t *testing.T) {
	server := newRecordingServer()
	dial := startTestServer(t, server)
	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	task.JobId = "job-duplicate"

	ctrl := gomock.NewController(t)
	counter := mocks.NewMockCohortCountUseCase(ctrl)
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	defer close(release)
	var calls atomic.Int32
	counter.EXPECT().CountMatchingCohort(gomock.Any(), "job-duplicate", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ types.CohortCriteria) (uint64, error) {
			if calls.Add(1) == 1 {
				started <- struct{}{}
				<-release
			} else {
				started <- struct{}{}
			}
			return 2, nil
		}).AnyTimes()

	nc := client.NewNodeClient(testConfig(), client.Dependencies{
		EnabledQueryFieldRepo: allEnabledFieldsRepoExpectation(ctrl),
		CohortCounter:         counter,
		SuppressionThreshold:  1,
	}, nil, dial)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = nc.Run(ctx) }()

	<-server.registers
	message := &nodev1.CenterToNode{Payload: &nodev1.CenterToNode_QueryTask{QueryTask: task}}
	server.toSend <- message
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first task did not start")
	}
	server.toSend <- message
	select {
	case <-started:
		t.Fatal("duplicate task started while first execution was in flight")
	case <-time.After(200 * time.Millisecond):
	}
	release <- struct{}{}
	select {
	case result := <-server.results:
		require.Equal(t, uint64(2), result.GetMatchingCount())
	case <-time.After(2 * time.Second):
		t.Fatal("first task did not return a result")
	}
	require.EqualValues(t, 1, calls.Load())
}

func TestNodeClient_RejectsUnknownVersionWithoutQueryingD3(t *testing.T) {
	server := newRecordingServer()
	dial := startTestServer(t, server)
	config := testConfig()

	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	task.JobId = "job-bad-version"
	task.QuerySchemaVersion = 99

	ctrl := gomock.NewController(t)
	// No CountMatchingCohort expectation: an unsupported version must never
	// reach the counter (Phase 4 decision: version gate runs first).
	counter := mocks.NewMockCohortCountUseCase(ctrl)

	nc := client.NewNodeClient(config, client.Dependencies{
		EnabledQueryFieldRepo: allEnabledFieldsRepoExpectation(ctrl),
		CohortCounter:         counter,
	}, nil, dial)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = nc.Run(ctx) }()

	<-server.registers
	server.toSend <- &nodev1.CenterToNode{Payload: &nodev1.CenterToNode_QueryTask{QueryTask: task}}

	select {
	case result := <-server.results:
		require.Equal(t, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_UNSUPPORTED_VERSION, result.GetStatus())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for QueryResult")
	}
}

func TestNodeClient_StoresUpdateAdvisory(t *testing.T) {
	server := newRecordingServer()
	dial := startTestServer(t, server)
	config := testConfig()

	ctrl := gomock.NewController(t)
	advisories := client.NewAdvisoryStore()
	nc := client.NewNodeClient(config, client.Dependencies{
		EnabledQueryFieldRepo: allEnabledFieldsRepoExpectation(ctrl),
		CohortCounter:         mocks.NewMockCohortCountUseCase(ctrl),
		Advisories:            advisories,
	}, nil, dial)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = nc.Run(ctx) }()

	<-server.registers
	server.toSend <- &nodev1.CenterToNode{Payload: &nodev1.CenterToNode_UpdateAdvisory{UpdateAdvisory: &nodev1.UpdateAdvisory{
		Version:      "v1.2.3",
		ChangelogUrl: "https://example.invalid/changelog",
		Severity:     nodev1.AdvisorySeverity_ADVISORY_SEVERITY_RECOMMENDED,
	}}}

	require.Eventually(t, func() bool {
		advisory, ok := advisories.Latest()
		return ok && advisory.Version == "v1.2.3"
	}, 2*time.Second, 10*time.Millisecond)

	advisory, ok := advisories.Latest()
	require.True(t, ok)
	require.Equal(t, "https://example.invalid/changelog", advisory.ChangelogURL)
	require.Equal(t, "ADVISORY_SEVERITY_RECOMMENDED", advisory.Severity)
}

func TestNodeClient_ReconnectsAfterStreamDrop(t *testing.T) {
	server := newRecordingServer()
	server.dropCount.Store(1) // first connection drops right after Register
	dial := startTestServer(t, server)
	config := testConfig()

	ctrl := gomock.NewController(t)
	nc := client.NewNodeClient(config, client.Dependencies{
		EnabledQueryFieldRepo: allEnabledFieldsRepoExpectation(ctrl),
		CohortCounter:         mocks.NewMockCohortCountUseCase(ctrl),
	}, nil, dial)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = nc.Run(ctx) }()

	first := requireRegister(t, server)
	second := requireRegister(t, server)
	require.Equal(t, first.GetNodeId(), second.GetNodeId(), "same node re-registers after reconnect")
}

func TestNodeClient_ReconnectsWhenExecutionIsInterruptedOnLiveStream(t *testing.T) {
	server := newRecordingServer()
	dial := startTestServer(t, server)
	config := testConfig()
	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	task.JobId = "job-interrupted"

	ctrl := gomock.NewController(t)
	counter := mocks.NewMockCohortCountUseCase(ctrl)
	gomock.InOrder(
		counter.EXPECT().CountMatchingCohort(gomock.Any(), task.JobId, gomock.Any()).Return(uint64(0), context.DeadlineExceeded),
		counter.EXPECT().CountMatchingCohort(gomock.Any(), task.JobId, gomock.Any()).Return(uint64(7), nil),
	)
	nc := client.NewNodeClient(config, client.Dependencies{
		EnabledQueryFieldRepo: allEnabledFieldsRepoExpectation(ctrl),
		CohortCounter:         counter,
		SuppressionThreshold:  1,
	}, nil, dial)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = nc.Run(ctx) }()

	requireRegister(t, server)
	server.toSend <- &nodev1.CenterToNode{Payload: &nodev1.CenterToNode_QueryTask{QueryTask: task}}
	requireRegister(t, server)
	server.toSend <- &nodev1.CenterToNode{Payload: &nodev1.CenterToNode_QueryTask{QueryTask: task}}
	select {
	case result := <-server.results:
		require.Equal(t, task.JobId, result.GetJobId())
		require.EqualValues(t, 7, result.GetMatchingCount())
	case <-time.After(2 * time.Second):
		t.Fatal("resumed job did not return a result")
	}
}

func TestNodeClient_LogsExecutionFailureLocally(t *testing.T) {
	server := newRecordingServer()
	dial := startTestServer(t, server)
	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	task.JobId = "job-failed"

	ctrl := gomock.NewController(t)
	counter := mocks.NewMockCohortCountUseCase(ctrl)
	counter.EXPECT().CountMatchingCohort(gomock.Any(), task.JobId, gomock.Any()).Return(uint64(0), errors.New("cursor did not advance"))
	log := mocks.NewMockLogger(ctrl)
	log.EXPECT().Error("Query task execution failed", gomock.Any(), gomock.Any())
	nc := client.NewNodeClient(testConfig(), client.Dependencies{
		EnabledQueryFieldRepo: allEnabledFieldsRepoExpectation(ctrl), CohortCounter: counter,
	}, log, dial)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = nc.Run(ctx) }()
	requireRegister(t, server)
	server.toSend <- &nodev1.CenterToNode{Payload: &nodev1.CenterToNode_QueryTask{QueryTask: task}}
	select {
	case result := <-server.results:
		require.Equal(t, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_ERROR, result.GetStatus())
		require.Equal(t, "query execution failed", result.GetReason())
	case <-time.After(2 * time.Second):
		t.Fatal("failed job did not return an error result")
	}
}

func TestNodeClient_ReturnsErrorWhenExecutorIsMissing(t *testing.T) {
	server := newRecordingServer()
	dial := startTestServer(t, server)
	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	task.JobId = "job-missing-executor"

	ctrl := gomock.NewController(t)
	log := mocks.NewMockLogger(ctrl)
	log.EXPECT().Error("Query task execution failed", gomock.Any(), gomock.Any())
	nc := client.NewNodeClient(testConfig(), client.Dependencies{
		EnabledQueryFieldRepo: allEnabledFieldsRepoExpectation(ctrl),
	}, log, dial)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = nc.Run(ctx) }()
	requireRegister(t, server)
	server.toSend <- &nodev1.CenterToNode{Payload: &nodev1.CenterToNode_QueryTask{QueryTask: task}}
	select {
	case result := <-server.results:
		require.Equal(t, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_ERROR, result.GetStatus())
		require.Equal(t, "query execution failed", result.GetReason())
	case <-time.After(2 * time.Second):
		t.Fatal("missing executor did not return an error result")
	}
}

func requireRegister(t *testing.T, server *recordingNodeControlServer) *nodev1.Register {
	t.Helper()
	select {
	case register := <-server.registers:
		return register
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Register")
		return nil
	}
}

func TestNodeClient_Run_FailsFastOnMisconfiguredMTLS(t *testing.T) {
	nc := client.NewNodeClient(client.Config{
		Address: "bufnet",
		NodeID:  "node-a",
		TLS:     client.TLSConfig{ClientCertFile: "cert.pem"}, // key file missing
	}, client.Dependencies{}, nil, func(string, credentials.TransportCredentials) (*grpc.ClientConn, error) {
		t.Fatal("dial must not be attempted when TLS setup fails")
		return nil, nil
	})

	err := nc.Run(context.Background())
	require.ErrorContains(t, err, "exactly one of")
}

func TestNodeClient_Run_RequiresAddressAndNodeID(t *testing.T) {
	dialNotCalled := func(string, credentials.TransportCredentials) (*grpc.ClientConn, error) {
		panic("dial must not be attempted")
	}

	err := client.NewNodeClient(client.Config{NodeID: "node-a"}, client.Dependencies{}, nil, dialNotCalled).
		Run(context.Background())
	require.ErrorContains(t, err, "address")

	err = client.NewNodeClient(client.Config{Address: "bufnet"}, client.Dependencies{}, nil, dialNotCalled).
		Run(context.Background())
	require.ErrorContains(t, err, "node_id")
}
