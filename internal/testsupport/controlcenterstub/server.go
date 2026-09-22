// Package controlcenterstub is a minimal, real NodeControl gRPC server used
// to exercise the Phase 7 node client end-to-end in tests, since no real
// control center exists yet (decision 0001). Test support only: it must
// never be imported from cmd/ or any other production code path - enforced
// by TestCmdProductionCodeDoesNotImportTestSupport
// (internal/domain/architecture_boundary_test.go), not just this comment.
package controlcenterstub

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"

	"google.golang.org/grpc"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
)

const messageBufferSize = 32

// Server accepts NodeControl.Connect, records what the node sends, and lets
// a test push QueryTask/UpdateAdvisory messages to whichever node is
// currently connected and force-drop that connection to exercise the
// client's reconnect path. It only supports one connected node at a time,
// which is all Phase 9's end-to-end test needs.
type Server struct {
	nodev1.UnimplementedNodeControlServer

	registers  chan *nodev1.Register
	heartbeats chan *nodev1.Heartbeat
	results    chan *nodev1.QueryResult

	mu         sync.Mutex
	conn       *activeConnection
	connSignal chan struct{} // closed and replaced whenever conn changes
}

type activeConnection struct {
	send chan *nodev1.CenterToNode
	stop chan struct{} // closed by DropConnection to simulate a network drop
}

// New creates a Server with no connection yet. Call Start to begin serving.
func New() *Server {
	return &Server{
		registers:  make(chan *nodev1.Register, messageBufferSize),
		heartbeats: make(chan *nodev1.Heartbeat, messageBufferSize),
		results:    make(chan *nodev1.QueryResult, messageBufferSize),
		connSignal: make(chan struct{}),
	}
}

// Start serves Server on a loopback TCP port and returns its address
// ("127.0.0.1:PORT"), suitable for client.Config.Address. The server stops
// when the test ends.
func (s *Server) Start(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("controlcenterstub: listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	nodev1.RegisterNodeControlServer(grpcServer, s)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)

	return listener.Addr().String()
}

// Registers observes Register messages the node has sent, one per
// (re)connect.
func (s *Server) Registers() <-chan *nodev1.Register {
	return s.registers
}

// Heartbeats observes Heartbeat messages the node has sent.
func (s *Server) Heartbeats() <-chan *nodev1.Heartbeat {
	return s.heartbeats
}

// QueryResults observes QueryResult messages the node has sent.
func (s *Server) QueryResults() <-chan *nodev1.QueryResult {
	return s.results
}

// SendQueryTask delivers task to the currently connected node, waiting for a
// connection until ctx is done.
func (s *Server) SendQueryTask(ctx context.Context, task *nodev1.QueryTask) error {
	return s.send(ctx, &nodev1.CenterToNode{Payload: &nodev1.CenterToNode_QueryTask{QueryTask: task}})
}

// SendUpdateAdvisory delivers advisory to the currently connected node,
// waiting for a connection until ctx is done.
func (s *Server) SendUpdateAdvisory(ctx context.Context, advisory *nodev1.UpdateAdvisory) error {
	return s.send(ctx, &nodev1.CenterToNode{Payload: &nodev1.CenterToNode_UpdateAdvisory{UpdateAdvisory: advisory}})
}

// DropConnection force-closes the current connection, if any, simulating a
// network drop so the node's reconnect logic can be exercised. It is a no-op
// when nothing is connected.
func (s *Server) DropConnection() {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn != nil {
		close(conn.stop)
	}
}

func (s *Server) send(ctx context.Context, msg *nodev1.CenterToNode) error {
	conn, err := s.waitForConnection(ctx)
	if err != nil {
		return fmt.Errorf("controlcenterstub: no connected node: %w", err)
	}
	select {
	case conn.send <- msg:
		return nil
	case <-conn.stop:
		return errors.New("controlcenterstub: connection was dropped before the message was sent")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) waitForConnection(ctx context.Context) (*activeConnection, error) {
	for {
		s.mu.Lock()
		conn := s.conn
		signal := s.connSignal
		s.mu.Unlock()
		if conn != nil {
			return conn, nil
		}
		select {
		case <-signal:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (s *Server) setConnection(conn *activeConnection) {
	s.mu.Lock()
	s.conn = conn
	previousSignal := s.connSignal
	s.connSignal = make(chan struct{})
	s.mu.Unlock()
	close(previousSignal)
}

func (s *Server) clearConnection(conn *activeConnection) {
	s.mu.Lock()
	if s.conn == conn {
		s.conn = nil
	}
	s.mu.Unlock()
}

// Connect implements nodev1.NodeControlServer. One goroutine drains
// stream.Recv() into the Registers/Heartbeats/QueryResults channels; the
// caller goroutine serializes stream.Send() calls, both required by the gRPC
// stream API's single-reader/single-writer rule.
func (s *Server) Connect(stream nodev1.NodeControl_ConnectServer) error {
	conn := &activeConnection{
		send: make(chan *nodev1.CenterToNode, messageBufferSize),
		stop: make(chan struct{}),
	}
	s.setConnection(conn)
	defer s.clearConnection(conn)

	recvErr := make(chan error, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				recvErr <- err
				return
			}
			switch payload := msg.GetPayload().(type) {
			case *nodev1.NodeToCenter_Register:
				s.registers <- payload.Register
			case *nodev1.NodeToCenter_Heartbeat:
				s.heartbeats <- payload.Heartbeat
			case *nodev1.NodeToCenter_QueryResult:
				s.results <- payload.QueryResult
			}
		}
	}()

	for {
		select {
		case err := <-recvErr:
			return err
		case <-conn.stop:
			return errors.New("controlcenterstub: connection dropped by test")
		case msg := <-conn.send:
			if err := stream.Send(msg); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}
