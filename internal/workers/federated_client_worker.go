package workers

import (
	"context"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/client"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/workers/types"
)

// FederatedClientWorker runs the gRPC control-center client (decision 0001)
// for the lifetime of the process, reconnecting on its own until ctx is done.
type FederatedClientWorker struct {
	nodeClient *client.NodeClient
	logger     logger.Logger
}

// NewFederatedClientWorker creates the worker. nodeClient must not be nil.
func NewFederatedClientWorker(nodeClient *client.NodeClient, log logger.Logger) *FederatedClientWorker {
	if log == nil {
		log = logger.GetLogger()
	}
	return &FederatedClientWorker{nodeClient: nodeClient, logger: log}
}

func (w *FederatedClientWorker) Name() string {
	return "federated-client"
}

// Start runs until ctx is done. Run only returns a non-nil error for a fatal
// startup misconfiguration (decision 0006); transient connection failures are
// retried internally and never surface here.
func (w *FederatedClientWorker) Start(ctx context.Context) {
	if err := w.Run(ctx); err != nil {
		w.logger.Error("Federated client worker stopped", logger.Err(err))
	}
}

// Run returns fatal client errors to callers that own the application lifecycle.
func (w *FederatedClientWorker) Run(ctx context.Context) error {
	return w.nodeClient.Run(ctx)
}

var _ types.Worker = (*FederatedClientWorker)(nil)
