package client

import (
	"context"
	"time"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/wire"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func (c *NodeClient) handleGovernanceControl(ctx context.Context, packet *nodev1.CenterToNode, send chan<- *nodev1.NodeToCenter) (bool, error) {
	if !c.config.GovernanceSyncEnabled {
		return false, nil
	}
	switch p := packet.GetPayload().(type) {
	case *nodev1.CenterToNode_GovernanceSnapshot:
		snapshot, err := wire.HospitalSnapshotFromProto(c.config.NodeID, p.GovernanceSnapshot)
		if err != nil {
			return true, err
		}
		if _, err := c.deps.HospitalGovernance.ApplySnapshot(ctx, snapshot); err != nil {
			return true, err
		}
		return true, c.sendGovernanceReport(ctx, send)
	case *nodev1.CenterToNode_GovernanceAck:
		a := p.GovernanceAck
		if a == nil || len(a.ProtoReflect().GetUnknown()) != 0 {
			return true, entities.ErrHospitalGovernanceInvalid
		}
		return true, c.deps.HospitalGovernance.Acknowledge(ctx, a.SessionEpoch, a.SnapshotRevision, a.LocalRevision)
	case *nodev1.CenterToNode_InvalidateWork:
		command, err := hospitalInvalidationFromProto(p.InvalidateWork)
		if err != nil {
			return true, err
		}
		if err := c.deps.HospitalGovernance.Invalidate(ctx, command); err != nil {
			return true, err
		}
		select {
		case send <- &nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_CommandAck{CommandAck: &nodev1.CommandAck{CommandId: command.CommandID, SessionEpoch: command.SessionEpoch}}}:
			return true, c.sendGovernanceReport(ctx, send)
		case <-ctx.Done():
			return true, ctx.Err()
		}
	default:
		return false, nil
	}
}

func hospitalInvalidationFromProto(p *nodev1.InvalidateWork) (entities.HospitalInvalidationCommand, error) {
	if p == nil || len(p.ProtoReflect().GetUnknown()) != 0 {
		return entities.HospitalInvalidationCommand{}, entities.ErrHospitalGovernanceInvalid
	}
	var reason string
	switch p.Reason {
	case nodev1.GovernanceReason_GOVERNANCE_REASON_PERMIT_REVOKED:
		reason = types.GovernanceReasonPermitRevoked
	case nodev1.GovernanceReason_GOVERNANCE_REASON_PERMIT_SUPERSEDED:
		reason = types.GovernanceReasonPermitSuperseded
	case nodev1.GovernanceReason_GOVERNANCE_REASON_QUERY_CANCELED:
		reason = types.GovernanceReasonQueryCanceled
	case nodev1.GovernanceReason_GOVERNANCE_REASON_QUERY_EXPIRED:
		reason = types.GovernanceReasonQueryExpired
	default:
		return entities.HospitalInvalidationCommand{}, entities.ErrHospitalGovernanceInvalid
	}
	queryReason := reason == types.GovernanceReasonQueryCanceled || reason == types.GovernanceReasonQueryExpired
	if queryReason != (p.QueryId != "") {
		return entities.HospitalInvalidationCommand{}, entities.ErrHospitalGovernanceInvalid
	}
	return entities.HospitalInvalidationCommand{CommandID: p.CommandId, TenantID: p.TenantId, QueryID: p.QueryId, PermitID: p.PermitId, ThroughVersion: p.ThroughPermitVersion, Reason: reason, SessionEpoch: p.SessionEpoch}, nil
}

func (c *NodeClient) sendGovernanceReport(ctx context.Context, send chan<- *nodev1.NodeToCenter) error {
	// Serialize reads with enqueueing so a newer local revision cannot overtake an older report.
	c.governanceSendMu.Lock()
	defer c.governanceSendMu.Unlock()
	r, err := c.deps.HospitalGovernance.Read(ctx)
	if err != nil {
		return err
	}
	if r == nil {
		return entities.ErrHospitalGovernanceInvalid
	}
	if r.Snapshot.SessionEpoch == "" || r.Synchronized() {
		return nil
	}
	packet, err := wire.HospitalReportToProto(*r)
	if err != nil {
		return err
	}
	select {
	case send <- &nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_GovernanceReport{GovernanceReport: packet}}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *NodeClient) governanceReportLoop(ctx context.Context, cancel context.CancelFunc, send chan<- *nodev1.NodeToCenter) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.sendGovernanceReport(ctx, send); err != nil {
				cancel()
				return
			}
		}
	}
}
