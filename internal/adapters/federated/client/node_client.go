package client

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/timestamppb"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	federatedwire "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/wire"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// Defaults for Config fields left unset. Tunable operational parameters, not
// wire contract (decision 0004's Heartbeat interval is a documented default,
// not a wire field); overriding them does not need a decision record.
const (
	DefaultHeartbeatInterval  = 30 * time.Second
	DefaultConnectTimeout     = 10 * time.Second
	DefaultInitialBackoff     = time.Second
	DefaultMaxBackoff         = 30 * time.Second
	defaultQuerySchemaVersion = 1
	sendQueueSize             = 8
)

// Dependencies are what the client needs to validate and execute QueryTasks
// it receives (Phases 4-6).
type Dependencies struct {
	EnabledQueryFieldRepo repositories.EnabledQueryFieldRepository
	CohortCounter         interfaces.CohortCountUseCase
	SuppressionThreshold  uint64
	Advisories            *AdvisoryStore
	HospitalGovernance    interfaces.HospitalGovernanceUseCase
	GovernedHospitalJobs  interfaces.GovernedHospitalJobUseCase
	Now                   func() time.Time
}

// Config configures one NodeClient.
type Config struct {
	Address                  string
	NodeID                   string
	AgentVersion             string
	QuerySchemaVersion       uint32
	TLS                      TLSConfig
	HeartbeatInterval        time.Duration
	ConnectTimeout           time.Duration
	InitialBackoff           time.Duration
	MaxBackoff               time.Duration
	GovernanceSyncEnabled    bool
	GovernedExecutionEnabled bool
	ApprovalPollInterval     time.Duration
}

func (c Config) normalized() Config {
	if c.QuerySchemaVersion == 0 {
		c.QuerySchemaVersion = defaultQuerySchemaVersion
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = DefaultHeartbeatInterval
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = DefaultConnectTimeout
	}
	if c.InitialBackoff <= 0 {
		c.InitialBackoff = DefaultInitialBackoff
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = DefaultMaxBackoff
	}
	if c.MaxBackoff < c.InitialBackoff {
		c.MaxBackoff = c.InitialBackoff
	}
	if c.ApprovalPollInterval <= 0 {
		c.ApprovalPollInterval = time.Second
	}
	return c
}

// Validate checks fatal client configuration before the application starts
// serving HTTP. Reachability is retried by Run and is not checked here.
func (c Config) Validate() error {
	_, err := c.transportCredentials()
	return err
}

func (c Config) transportCredentials() (credentials.TransportCredentials, error) {
	if c.GovernedExecutionEnabled && (!c.GovernanceSyncEnabled || c.TLS.Insecure || strings.TrimSpace(c.TLS.ClientCertFile) == "" || strings.TrimSpace(c.TLS.ClientKeyFile) == "") {
		return nil, errors.New("governed execution requires synchronized governance and verified mutual TLS")
	}
	if strings.TrimSpace(c.Address) == "" {
		return nil, errors.New("control center address is required")
	}
	if strings.TrimSpace(c.NodeID) == "" {
		return nil, errors.New("node_id is required")
	}
	creds, err := c.TLS.Credentials()
	if err != nil {
		return nil, fmt.Errorf("control center transport credentials: %w", err)
	}
	return creds, nil
}

// Dialer creates a ClientConn for address. Overridable in tests (bufconn).
type Dialer func(address string, creds credentials.TransportCredentials) (*grpc.ClientConn, error)

func defaultDialer(address string, creds credentials.TransportCredentials) (*grpc.ClientConn, error) {
	return grpc.NewClient(address, grpc.WithTransportCredentials(creds))
}

// NodeClient holds the long-lived, node-initiated stream to the control
// center (decision 0001): dial-out, Register/Heartbeat, receive QueryTask and
// UpdateAdvisory, reconnect with backoff on stream loss (Fig. 4).
type NodeClient struct {
	config           Config
	deps             Dependencies
	logger           logger.Logger
	dial             Dialer
	governanceSendMu sync.Mutex
}

// NewNodeClient creates a NodeClient. dial is optional; nil uses the real
// gRPC dialer.
func NewNodeClient(config Config, deps Dependencies, log logger.Logger, dial Dialer) *NodeClient {
	if log == nil {
		log = logger.GetLogger()
	}
	if dial == nil {
		dial = defaultDialer
	}
	if deps.Advisories == nil {
		deps.Advisories = NewAdvisoryStore()
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &NodeClient{
		config: config.normalized(),
		deps:   deps,
		logger: log,
		dial:   dial,
	}
}

// Run dials, registers, and serves the control stream until ctx is done,
// reconnecting with backoff on any stream loss. It returns nil on a clean
// ctx-driven shutdown, and a non-nil error only for a fatal, non-retryable
// misconfiguration (decision 0006: bad TLS setup fails loudly at start,
// never falls back to plaintext).
func (c *NodeClient) Run(ctx context.Context) error {
	if c.config.GovernedExecutionEnabled && (c.deps.GovernedHospitalJobs == nil || c.deps.HospitalGovernance == nil || c.deps.CohortCounter == nil) {
		return errors.New("durable governed jobs, governance and counter are required for governed execution")
	}
	if c.config.GovernanceSyncEnabled && c.deps.HospitalGovernance == nil {
		return errors.New("durable hospital governance is required for synchronization")
	}
	creds, err := c.config.transportCredentials()
	if err != nil {
		return err
	}

	backoff := c.config.InitialBackoff
	for ctx.Err() == nil {
		if c.attemptConnection(ctx, creds) {
			backoff = c.config.InitialBackoff
		} else {
			backoff = nextBackoff(backoff, c.config.MaxBackoff)
		}

		select {
		case <-time.After(jitter(backoff)):
		case <-ctx.Done():
			return nil
		}
	}
	return nil
}

// attemptConnection runs one dial-connect-serve cycle. It returns true when
// the stream was established and served for a while (so the backoff resets),
// false when the attempt never got off the ground.
func (c *NodeClient) attemptConnection(ctx context.Context, creds credentials.TransportCredentials) bool {
	conn, err := c.dial(c.config.Address, creds)
	if err != nil {
		c.logger.Warn("failed to create control center connection", logger.Err(err))
		return false
	}
	defer func() { _ = conn.Close() }()

	if err := waitForReady(ctx, conn, c.config.ConnectTimeout); err != nil {
		if ctx.Err() == nil {
			c.logger.Warn("control center not reachable", logger.Err(err))
		}
		return false
	}

	if err := c.runConnection(ctx, conn); err != nil && ctx.Err() == nil {
		c.logger.Warn("control center stream ended; reconnecting", logger.Err(err))
	}
	return true
}

// runConnection opens the bidi stream, registers, and serves it until the
// stream errors, ctx is done, or a send fails.
func (c *NodeClient) runConnection(ctx context.Context, conn *grpc.ClientConn) error {
	connCtx, cancel := context.WithCancel(ctx)
	var connWG, taskWG sync.WaitGroup
	defer func() {
		cancel()
		taskWG.Wait()
		connWG.Wait()
		if c.config.GovernanceSyncEnabled {
			cleanupCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer stop()
			if err := c.deps.HospitalGovernance.FenceConnection(cleanupCtx); err != nil {
				c.logger.Error("Failed to persist disconnected governance fence", logger.Err(err))
			}
		}
	}()
	if c.config.GovernanceSyncEnabled {
		if err := c.deps.HospitalGovernance.FenceConnection(connCtx); err != nil {
			return fmt.Errorf("fence hospital governance session: %w", err)
		}
	}

	stub := nodev1.NewNodeControlClient(conn)
	stream, err := stub.Connect(connCtx)
	if err != nil {
		return fmt.Errorf("open control stream: %w", err)
	}

	sendCh := make(chan *nodev1.NodeToCenter, sendQueueSize)
	connWG.Add(1)
	go func() {
		defer connWG.Done()
		c.sendLoop(connCtx, cancel, stream, sendCh)
	}()

	select {
	case sendCh <- registerMessage(c.config):
	case <-connCtx.Done():
		return connCtx.Err()
	}

	inflight := newInflightJobs()
	var governed *governedRuntime
	if c.config.GovernedExecutionEnabled {
		governed = newGovernedRuntime(connCtx, cancel, c, sendCh, inflight, &taskWG)
	}
	connWG.Add(1)
	go func() {
		defer connWG.Done()
		c.heartbeatLoop(connCtx, sendCh, inflight)
	}()
	if c.config.GovernanceSyncEnabled {
		connWG.Add(1)
		go func() {
			defer connWG.Done()
			c.governanceReportLoop(connCtx, cancel, sendCh)
		}()
	}

	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		c.handleCenterMessage(connCtx, cancel, msg, sendCh, inflight, &taskWG, governed)
		if err := connCtx.Err(); err != nil {
			return err
		}
	}
}

func (c *NodeClient) handleCenterMessage(
	connCtx context.Context,
	cancel context.CancelFunc,
	msg *nodev1.CenterToNode,
	sendCh chan<- *nodev1.NodeToCenter,
	inflight *inflightJobs,
	taskWG *sync.WaitGroup,
	governed *governedRuntime,
) {
	if connCtx.Err() != nil {
		return
	}
	if msg == nil || len(msg.ProtoReflect().GetUnknown()) > 0 {
		c.logger.Warn("Unsupported control message; closing stream")
		cancel()
		return
	}
	if handled, err := c.handleGovernanceControl(connCtx, msg, sendCh); handled {
		if err == nil && governed != nil && msg.GetGovernanceAck() != nil {
			err = governed.startRecovery()
		}
		if err != nil {
			c.logger.Warn("Governance synchronization rejected; closing stream")
			cancel()
		}
		return
	}
	if governed != nil {
		if handled, err := governed.handle(msg); handled {
			if err != nil {
				c.logger.Warn("Governed control message rejected; closing stream")
				cancel()
			}
			return
		}
	}
	switch payload := msg.GetPayload().(type) {
	case *nodev1.CenterToNode_QueryTask:
		task := payload.QueryTask
		jobID := task.GetJobId()
		if !inflight.add(jobID) {
			c.logger.Warn("Duplicate in-flight query task ignored", logger.String("job_id", jobID))
			return
		}
		taskWG.Add(1)
		go func() {
			defer taskWG.Done()
			defer inflight.remove(jobID)
			result := c.handleQueryTask(connCtx, task)
			if result == nil {
				if connCtx.Err() == nil {
					// A lower-layer interruption left this stream alive. Force a
					// reconnect so the control plane resends the checkpointed job.
					cancel()
				}
				return
			}
			select {
			case sendCh <- &nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_QueryResult{QueryResult: result}}:
			case <-connCtx.Done():
			}
		}()
	case *nodev1.CenterToNode_UpdateAdvisory:
		advisory := payload.UpdateAdvisory
		c.deps.Advisories.SetFromWire(advisory)
		c.logger.Info("Received update advisory",
			logger.String("version", advisory.GetVersion()),
			logger.String("severity", advisory.GetSeverity().String()))
	default:
		// Metadata synchronization does not authorize governed execution.
		// Never reinterpret an unsupported task as legacy work.
		c.logger.Warn("Unsupported control message; closing stream")
		cancel()
	}
}

// handleQueryTask validates and executes task, returning the QueryResult to
// send. It returns nil when nothing should be sent: a canceled/expired ctx
// means the connection is going away mid-job, and decision 0005's checkpoint
// makes the next delivery of the same job_id resume rather than restart, so
// there is nothing useful to report now.
func (c *NodeClient) handleQueryTask(ctx context.Context, task *nodev1.QueryTask) *nodev1.QueryResult {
	if err := federatedwire.ValidateQueryTaskV1(ctx, task, c.deps.EnabledQueryFieldRepo); err != nil {
		if validationErr, ok := federatedwire.AsQueryValidationError(err); ok {
			return &nodev1.QueryResult{JobId: task.GetJobId(), Status: validationErr.Status, Reason: validationErr.Reason}
		}
		c.logger.Error("Query task validation failed", logger.String("job_id", task.GetJobId()), logger.Err(err))
		return &nodev1.QueryResult{
			JobId:  task.GetJobId(),
			Status: nodev1.QueryResultStatus_QUERY_RESULT_STATUS_ERROR,
			Reason: "internal error",
		}
	}

	result, err := federatedwire.ExecuteQueryTaskV1(ctx, task, c.deps.CohortCounter, c.deps.SuppressionThreshold)
	if err != nil {
		if result == nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				c.logger.Warn("Query task execution interrupted; reconnecting for retry",
					logger.String("job_id", task.GetJobId()), logger.Err(err))
				return nil
			}
			result = &nodev1.QueryResult{
				JobId: task.GetJobId(), Status: nodev1.QueryResultStatus_QUERY_RESULT_STATUS_ERROR,
				Reason: "query execution failed",
			}
		}
		c.logger.Error("Query task execution failed", logger.String("job_id", task.GetJobId()), logger.Err(err))
	}
	return result
}

func (c *NodeClient) sendLoop(
	connCtx context.Context,
	cancel context.CancelFunc,
	stream nodev1.NodeControl_ConnectClient,
	sendCh <-chan *nodev1.NodeToCenter,
) {
	for {
		select {
		case <-connCtx.Done():
			return
		case msg := <-sendCh:
			if err := stream.Send(msg); err != nil {
				c.logger.Warn("Control stream send failed", logger.Err(err))
				cancel()
				return
			}
		}
	}
}

func (c *NodeClient) heartbeatLoop(connCtx context.Context, sendCh chan<- *nodev1.NodeToCenter, inflight *inflightJobs) {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-connCtx.Done():
			return
		case <-ticker.C:
			msg := &nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_Heartbeat{Heartbeat: &nodev1.Heartbeat{
				SentAt:         timestamppb.Now(),
				InflightJobIds: inflight.snapshot(),
			}}}
			select {
			case sendCh <- msg:
			case <-connCtx.Done():
				return
			}
		}
	}
}

func registerMessage(config Config) *nodev1.NodeToCenter {
	register := &nodev1.Register{
		NodeId:             config.NodeID,
		AgentVersion:       config.AgentVersion,
		QuerySchemaVersion: config.QuerySchemaVersion,
	}
	if config.GovernanceSyncEnabled {
		register.SupportedGovernanceProfiles = []string{"governed-cohort/v1"}
		register.GovernanceMessageSchemaVersion = 1
	}
	if config.GovernedExecutionEnabled {
		register.SupportedReleaseModes = []nodev1.ReleaseMode{nodev1.ReleaseMode_RELEASE_MODE_AUTO, nodev1.ReleaseMode_RELEASE_MODE_MANUAL}
	}
	return &nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_Register{Register: register}}
}

// waitForReady blocks until conn reaches connectivity.Ready or timeout
// elapses. This is the documented replacement for the deprecated
// grpc.WithBlock() now that grpc.NewClient connects lazily.
func waitForReady(ctx context.Context, conn *grpc.ClientConn, timeout time.Duration) error {
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if state == connectivity.Shutdown {
			return errors.New("control center connection shut down")
		}
		if !conn.WaitForStateChange(attemptCtx, state) {
			if err := attemptCtx.Err(); err != nil {
				return err
			}
			return errors.New("control center connection did not become ready")
		}
	}
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next <= 0 || next > max {
		return max
	}
	return next
}

// jitter returns a random duration in [d/2, d], so simultaneous reconnect
// attempts (e.g. after a shared network blip) do not all retry in lockstep.
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(d-half+1)))
}
