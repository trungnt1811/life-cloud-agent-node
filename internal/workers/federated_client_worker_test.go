package workers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/client"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/workers"
)

func TestFederatedClientWorker_Name(t *testing.T) {
	nodeClient := client.NewNodeClient(client.Config{}, client.Dependencies{}, nil, nil)
	worker := workers.NewFederatedClientWorker(nodeClient, nil)
	require.Equal(t, "federated-client", worker.Name())
}

// Start must return (not block or panic) when the underlying client fails
// fast on a startup misconfiguration (here: missing address/node_id), since
// that path only logs and never propagates - Worker.Start has no error return.
func TestFederatedClientWorker_StartReturnsOnFatalClientError(t *testing.T) {
	nodeClient := client.NewNodeClient(client.Config{}, client.Dependencies{}, nil, nil)
	worker := workers.NewFederatedClientWorker(nodeClient, nil)

	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Start(context.Background())
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after a fatal client configuration error")
	}
}
