package client

import (
	"context"
	"errors"
	"sync"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/wire"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

type governedReplyKind uint8

const (
	governedReplyStage governedReplyKind = iota
	governedReplyGrant
	governedReplyDenial
	governedReplyReceipt
	governedMaxConcurrentActors = 100
)

type governedReply struct {
	kind     governedReplyKind
	eventID  string
	sequence int64
	grant    *entities.GovernedHospitalGrant
	record   *entities.GovernedHospitalJobRecord
}

type governedRuntime struct {
	ctx      context.Context
	cancel   context.CancelFunc
	client   *NodeClient
	send     chan<- *nodev1.NodeToCenter
	inflight *inflightJobs
	wg       *sync.WaitGroup
	mu       sync.Mutex
	actors   map[string]*governedActor
	recovery sync.Once
}

type governedActor struct {
	runtime     *governedRuntime
	task        entities.GovernedHospitalTask
	fingerprint string
	replies     chan governedReply
}

func newGovernedRuntime(ctx context.Context, cancel context.CancelFunc, c *NodeClient, send chan<- *nodev1.NodeToCenter, inflight *inflightJobs, wg *sync.WaitGroup) *governedRuntime {
	return &governedRuntime{ctx: ctx, cancel: cancel, client: c, send: send, inflight: inflight, wg: wg, actors: make(map[string]*governedActor)}
}

func (r *governedRuntime) start(task entities.GovernedHospitalTask) error {
	task = task.Clone()
	if task.NodeID != r.client.config.NodeID || task.ValidateEnvelope() != nil {
		return entities.ErrGovernedHospitalJobInvalid
	}
	fingerprint, err := task.Fingerprint()
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if prior := r.actors[task.JobID]; prior != nil {
		if prior.fingerprint != fingerprint || prior.task.SessionEpoch != task.SessionEpoch {
			return entities.ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	if r.ctx.Err() != nil {
		return r.ctx.Err()
	}
	if len(r.actors) >= governedMaxConcurrentActors || !r.inflight.add(task.JobID) {
		return errors.New("governed transport work capacity unavailable")
	}
	a := &governedActor{runtime: r, task: task, fingerprint: fingerprint, replies: make(chan governedReply, 8)}
	r.actors[task.JobID] = a
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer func() { r.mu.Lock(); delete(r.actors, task.JobID); r.mu.Unlock(); r.inflight.remove(task.JobID) }()
		if err := a.execute(); err != nil && r.ctx.Err() == nil {
			// Never log a counter error, payload, grant or hidden value.
			r.client.logger.Warn("Governed execution interrupted; closing stream", logger.String("job_id", task.JobID))
			r.cancel()
		}
	}()
	return nil
}

func (r *governedRuntime) handle(packet *nodev1.CenterToNode) (bool, error) {
	switch p := packet.GetPayload().(type) {
	case *nodev1.CenterToNode_GovernedQueryTask:
		task, err := wire.GovernedHospitalTaskFromProto(p.GovernedQueryTask)
		if err != nil {
			return true, err
		}
		return true, r.start(task)
	case *nodev1.CenterToNode_StageAck:
		ack, err := wire.GovernedHospitalStageAckFromProto(p.StageAck)
		if err != nil {
			return true, err
		}
		record, err := r.client.deps.GovernedHospitalJobs.AcknowledgeStage(r.ctx, ack)
		if err != nil {
			return true, err
		}
		return true, r.deliver(ack.JobID, governedReply{kind: governedReplyStage, eventID: ack.EventID, sequence: ack.Sequence, record: record})
	case *nodev1.CenterToNode_ReleaseDecision:
		return true, r.releaseDecision(p.ReleaseDecision)
	case *nodev1.CenterToNode_ResultReceipt:
		receipt, err := wire.GovernedHospitalReceiptFromProto(p.ResultReceipt)
		if err != nil {
			return true, err
		}
		record, err := r.client.deps.GovernedHospitalJobs.Receive(r.ctx, receipt)
		if err != nil {
			return true, err
		}
		return true, r.deliver(receipt.JobID, governedReply{kind: governedReplyReceipt, eventID: receipt.EventID, record: record})
	default:
		return false, nil
	}
}

func (r *governedRuntime) releaseDecision(p *nodev1.ReleaseDecision) error {
	if p == nil || len(p.ProtoReflect().GetUnknown()) != 0 {
		return entities.ErrGovernedHospitalJobInvalid
	}
	switch outcome := p.GetOutcome().(type) {
	case *nodev1.ReleaseDecision_Grant:
		grant, err := wire.GovernedHospitalGrantFromProto(outcome.Grant)
		if err != nil {
			return err
		}
		return r.deliver(grant.Binding.JobID, governedReply{kind: governedReplyGrant, eventID: grant.Binding.EventID, grant: &grant})
	case *nodev1.ReleaseDecision_Denied:
		denial, err := wire.GovernedHospitalDenialFromProto(outcome.Denied)
		if err != nil {
			return err
		}
		record, err := r.client.deps.GovernedHospitalJobs.Deny(r.ctx, denial)
		if err != nil {
			return err
		}
		return r.deliver(denial.Binding.JobID, governedReply{kind: governedReplyDenial, eventID: denial.Binding.EventID, record: record})
	default:
		return entities.ErrGovernedHospitalJobInvalid
	}
}

func (r *governedRuntime) deliver(jobID string, reply governedReply) error {
	if reply.kind != governedReplyGrant && (reply.record == nil || reply.record.Task.JobID != jobID) {
		return entities.ErrGovernedHospitalJobInvalid
	}
	r.mu.Lock()
	a := r.actors[jobID]
	r.mu.Unlock()
	if a == nil {
		// Known historical proofs were reconciled durably above. A grant without
		// a live intent consumer cannot be treated as fresh egress authority.
		if reply.kind == governedReplyGrant {
			return entities.ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	select {
	case a.replies <- reply:
		return nil
	case <-r.ctx.Done():
		return r.ctx.Err()
	default:
		return errors.New("governed reply queue full")
	}
}
