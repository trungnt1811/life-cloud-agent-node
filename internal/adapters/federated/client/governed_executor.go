package client

import (
	"context"
	"time"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/wire"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func (a *governedActor) execute() error {
	reconcileCtx, stopReconcile := context.WithTimeout(a.runtime.ctx, 5*time.Second)
	defer stopReconcile()
	jobs := a.runtime.client.deps.GovernedHospitalJobs
	stored, err := jobs.Read(reconcileCtx, a.task.JobID)
	if err != nil {
		return err
	}
	if stored != nil && stored.Fingerprint != a.fingerprint {
		return entities.ErrGovernedHospitalJobInvalid
	}
	if stored != nil && stored.Phase == entities.GovernedHospitalWaitingApproval && !stored.ApprovalDeadline.IsZero() && !a.runtime.client.deps.Now().UTC().Before(stored.ApprovalDeadline) {
		stored, err = jobs.ExpireApproval(reconcileCtx, a.task.JobID)
		if err != nil {
			return err
		}
	}
	if stored != nil && stored.Phase == entities.GovernedHospitalApprovalExpired {
		return a.replayApprovalExpiry()
	}
	if stored != nil && stored.DeliveryState == entities.GovernedHospitalSentUnconfirmed {
		stored, err = a.reconcile(reconcileCtx, stored)
		if err != nil {
			return err
		}
		if stored.Phase != entities.GovernedHospitalReady {
			return nil
		}
	}
	ctx, stop := context.WithDeadline(a.runtime.ctx, a.task.QueryDeadline)
	defer stop()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	record, err := jobs.Begin(ctx, a.task)
	if err != nil {
		return err
	}
	record, err = a.ackStages(ctx, record)
	if err != nil {
		return err
	}
	if record.Phase == entities.GovernedHospitalExecuting {
		record, err = a.prepareCount(ctx, record)
		if err != nil {
			return err
		}
	}
	if record.Phase == entities.GovernedHospitalWaitingApproval {
		record, err = a.ackStages(ctx, record)
		if err != nil {
			return err
		}
		record, err = a.waitForApproval(ctx, record)
		if err != nil {
			return err
		}
		if record.Phase == entities.GovernedHospitalApprovalExpired {
			return a.replayApprovalExpiry()
		}
	}
	for record.Phase == entities.GovernedHospitalReady && ctx.Err() == nil {
		record, err = jobs.Intent(ctx, a.task.JobID, a.task.SessionEpoch)
		if err != nil {
			return err
		}
		record, err = a.ackStages(ctx, record)
		if err != nil {
			return err
		}
		record, err = a.release(ctx, record)
		if err != nil {
			return err
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	_, err = a.ackStages(ctx, record)
	return err
}

func (a *governedActor) replayApprovalExpiry() error {
	ctx, cancel := context.WithTimeout(a.runtime.ctx, 5*time.Second)
	defer cancel()
	// Expired tasks may report their durable count-free journal, never recount
	// or request a grant. Rebind through the current synchronized connection.
	record, err := a.runtime.client.deps.GovernedHospitalJobs.Begin(ctx, a.task)
	if err != nil {
		return err
	}
	if record == nil || record.Phase != entities.GovernedHospitalApprovalExpired {
		return entities.ErrGovernedHospitalJobInvalid
	}
	_, err = a.ackStages(ctx, record)
	return err
}

func (a *governedActor) waitForApproval(ctx context.Context, record *entities.GovernedHospitalJobRecord) (*entities.GovernedHospitalJobRecord, error) {
	if record == nil || record.Phase != entities.GovernedHospitalWaitingApproval || record.ApprovalDeadline.IsZero() {
		return nil, entities.ErrGovernedHospitalJobInvalid
	}
	ticker := time.NewTicker(a.runtime.client.config.ApprovalPollInterval)
	defer ticker.Stop()
	jobs := a.runtime.client.deps.GovernedHospitalJobs
	for {
		now := a.runtime.client.deps.Now().UTC()
		if !now.Before(record.ApprovalDeadline) {
			return jobs.ExpireApproval(ctx, a.task.JobID)
		}
		if failure := a.guard(ctx); failure.err != nil {
			if failure.phase == "" {
				return nil, failure.err
			}
			return jobs.Terminate(ctx, a.task.JobID, a.task.SessionEpoch, failure.phase, failure.reason)
		}
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded && !a.runtime.client.deps.Now().UTC().Before(a.task.QueryDeadline) {
				terminalCtx, cancel := context.WithTimeout(a.runtime.ctx, 5*time.Second)
				defer cancel()
				return jobs.ExpireApproval(terminalCtx, a.task.JobID)
			}
			return nil, ctx.Err()
		case <-ticker.C:
			var err error
			record, err = jobs.Read(ctx, a.task.JobID)
			if err != nil {
				return nil, err
			}
			if record == nil {
				return nil, entities.ErrGovernedHospitalJobInvalid
			}
			if record.Phase != entities.GovernedHospitalWaitingApproval {
				return record, nil
			}
		}
	}
}

func (a *governedActor) reconcile(ctx context.Context, stored *entities.GovernedHospitalJobRecord) (*entities.GovernedHospitalJobRecord, error) {
	packet, err := wire.GovernedHospitalReceiptLookupToProto(*stored)
	if err != nil {
		return nil, err
	}
	if err := a.send(ctx, &nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_ReceiptLookup{ReceiptLookup: packet}}); err != nil {
		return nil, err
	}
	reply, err := a.wait(ctx, stored.Binding.EventID, 0, governedReplyReceipt)
	return reply.record, err
}

func (a *governedActor) prepareCount(ctx context.Context, record *entities.GovernedHospitalJobRecord) (*entities.GovernedHospitalJobRecord, error) {
	jobs := a.runtime.client.deps.GovernedHospitalJobs
	raw, phase, reason, err := a.count(ctx, *record)
	if err == nil {
		return jobs.Prepare(ctx, a.task.JobID, a.task.SessionEpoch, raw)
	}
	if ctx.Err() != nil || phase == "" {
		return nil, err
	}
	return jobs.Terminate(ctx, a.task.JobID, a.task.SessionEpoch, phase, reason)
}

func (a *governedActor) ackStages(ctx context.Context, record *entities.GovernedHospitalJobRecord) (*entities.GovernedHospitalJobRecord, error) {
	if record == nil {
		return nil, entities.ErrGovernedHospitalJobInvalid
	}
	job, err := entities.NewGovernedHospitalJobFromRecord(*record)
	if err != nil {
		return nil, err
	}
	for _, stage := range job.PendingStages() {
		packet, err := wire.GovernedHospitalStageToProto(stage)
		if err != nil {
			return nil, err
		}
		if err := a.send(ctx, &nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_JobStage{JobStage: packet}}); err != nil {
			return nil, err
		}
		reply, err := a.wait(ctx, stage.Binding.EventID, stage.Sequence, governedReplyStage)
		if err != nil {
			return nil, err
		}
		record = reply.record
	}
	return record, nil
}

func (a *governedActor) release(ctx context.Context, record *entities.GovernedHospitalJobRecord) (*entities.GovernedHospitalJobRecord, error) {
	request, err := wire.GovernedHospitalReleaseRequestToProto(record.Binding)
	if err != nil {
		return nil, err
	}
	if err := a.send(ctx, &nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_ReleaseRequest{ReleaseRequest: request}}); err != nil {
		return nil, err
	}
	reply, err := a.wait(ctx, record.Binding.EventID, 0, governedReplyGrant, governedReplyDenial)
	if err != nil {
		return nil, err
	}
	if reply.kind == governedReplyDenial {
		return reply.record, nil
	}
	if reply.grant == nil {
		return nil, entities.ErrGovernedHospitalJobInvalid
	}
	record, err = a.runtime.client.deps.GovernedHospitalJobs.Send(ctx, a.task.JobID, *reply.grant)
	if err != nil {
		return nil, err
	}
	// The Send transaction commits protected intent/ledger/audit BEFORE enqueue.
	packet, err := wire.GovernedHospitalResultToProto(*record)
	if err != nil {
		return nil, err
	}
	if err := a.send(ctx, &nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_GovernedQueryResult{GovernedQueryResult: packet}}); err != nil {
		return nil, err
	}
	reply, err = a.wait(ctx, record.Binding.EventID, 0, governedReplyReceipt)
	if err != nil {
		return nil, err
	}
	return reply.record, nil
}

func (a *governedActor) send(ctx context.Context, packet *nodev1.NodeToCenter) error {
	select {
	case a.runtime.send <- packet:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *governedActor) wait(ctx context.Context, eventID string, sequence int64, kinds ...governedReplyKind) (governedReply, error) {
	for {
		select {
		case <-ctx.Done():
			return governedReply{}, ctx.Err()
		case reply := <-a.replies:
			if reply.eventID != eventID || (sequence != 0 && reply.sequence != sequence) {
				continue
			}
			for _, kind := range kinds {
				if reply.kind == kind {
					return reply, nil
				}
			}
		}
	}
}
