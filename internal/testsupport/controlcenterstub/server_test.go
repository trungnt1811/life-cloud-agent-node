package controlcenterstub_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/testsupport/controlcenterstub"
)

// rawClient dials Server with the generated client stub directly (not
// client.NodeClient), so these tests exercise the stub in isolation, ahead
// of Phase 9's end-to-end test which drives it through the real node client.
func rawClient(t *testing.T, addr string) nodev1.NodeControl_ConnectClient {
	t.Helper()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	stream, err := nodev1.NewNodeControlClient(conn).Connect(context.Background())
	require.NoError(t, err)
	return stream
}

func withTimeout(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 2*time.Second)
}

func TestServer_RecordsRegisterAndHeartbeat(t *testing.T) {
	server := controlcenterstub.New()
	addr := server.Start(t)
	stream := rawClient(t, addr)

	require.NoError(t, stream.Send(&nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_Register{
		Register: &nodev1.Register{NodeId: "node-a", AgentVersion: "v1", QuerySchemaVersion: 1},
	}}))
	require.NoError(t, stream.Send(&nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_Heartbeat{
		Heartbeat: &nodev1.Heartbeat{InflightJobIds: []string{"job-1"}},
	}}))

	select {
	case register := <-server.Registers():
		require.Equal(t, "node-a", register.GetNodeId())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Register")
	}

	select {
	case heartbeat := <-server.Heartbeats():
		require.Equal(t, []string{"job-1"}, heartbeat.GetInflightJobIds())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Heartbeat")
	}
}

func TestServer_SendQueryTaskDeliversToConnectedClientAndRecordsResult(t *testing.T) {
	server := controlcenterstub.New()
	addr := server.Start(t)
	stream := rawClient(t, addr)
	require.NoError(t, stream.Send(&nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_Register{
		Register: &nodev1.Register{NodeId: "node-a"},
	}}))
	<-server.Registers()

	ctx, cancel := withTimeout(t)
	defer cancel()
	require.NoError(t, server.SendQueryTask(ctx, &nodev1.QueryTask{JobId: "job-1"}))

	received, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, "job-1", received.GetQueryTask().GetJobId())

	require.NoError(t, stream.Send(&nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_QueryResult{
		QueryResult: &nodev1.QueryResult{JobId: "job-1", Status: nodev1.QueryResultStatus_QUERY_RESULT_STATUS_OK, MatchingCount: 3},
	}}))

	select {
	case result := <-server.QueryResults():
		require.Equal(t, "job-1", result.GetJobId())
		require.Equal(t, uint64(3), result.GetMatchingCount())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for QueryResult")
	}
}

func TestServer_SendUpdateAdvisoryDeliversToConnectedClient(t *testing.T) {
	server := controlcenterstub.New()
	addr := server.Start(t)
	stream := rawClient(t, addr)
	require.NoError(t, stream.Send(&nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_Register{
		Register: &nodev1.Register{NodeId: "node-a"},
	}}))
	<-server.Registers()

	ctx, cancel := withTimeout(t)
	defer cancel()
	require.NoError(t, server.SendUpdateAdvisory(ctx, &nodev1.UpdateAdvisory{Version: "v2.0.0"}))

	received, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, "v2.0.0", received.GetUpdateAdvisory().GetVersion())
}

func TestServer_SendQueryTaskErrorsWithoutAConnection(t *testing.T) {
	server := controlcenterstub.New()
	server.Start(t)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := server.SendQueryTask(ctx, &nodev1.QueryTask{JobId: "job-1"})
	require.ErrorContains(t, err, "no connected node")
}

func TestServer_DropConnectionEndsTheStream(t *testing.T) {
	server := controlcenterstub.New()
	addr := server.Start(t)
	stream := rawClient(t, addr)
	require.NoError(t, stream.Send(&nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_Register{
		Register: &nodev1.Register{NodeId: "node-a"},
	}}))
	<-server.Registers()

	server.DropConnection()

	_, err := stream.Recv()
	require.Error(t, err, "the stream must end once the stub drops the connection")
}

func TestServer_SecondConnectionReplacesTheFirst(t *testing.T) {
	server := controlcenterstub.New()
	addr := server.Start(t)

	first := rawClient(t, addr)
	require.NoError(t, first.Send(&nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_Register{
		Register: &nodev1.Register{NodeId: "node-a"},
	}}))
	<-server.Registers()

	second := rawClient(t, addr)
	require.NoError(t, second.Send(&nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_Register{
		Register: &nodev1.Register{NodeId: "node-a"},
	}}))
	<-server.Registers()

	ctx, cancel := withTimeout(t)
	defer cancel()
	require.NoError(t, server.SendQueryTask(ctx, &nodev1.QueryTask{JobId: "job-on-latest"}))

	received, err := second.Recv()
	require.NoError(t, err)
	require.Equal(t, "job-on-latest", received.GetQueryTask().GetJobId())
}
