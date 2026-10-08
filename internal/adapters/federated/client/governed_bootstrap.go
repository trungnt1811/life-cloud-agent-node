package client

import (
	"context"
	"time"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/wire"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func (r *governedRuntime) startRecovery() error {
	ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
	defer cancel()
	state, err := r.client.deps.HospitalGovernance.Read(ctx)
	if err != nil {
		return err
	}
	if state == nil || !state.Synchronized() {
		return nil
	}
	epoch := state.Snapshot.SessionEpoch
	r.recovery.Do(func() {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			err := r.recoverReceipts(epoch)
			if err == nil && r.client.config.GovernedExecutionEnabled {
				err = r.recoverManualJobs(epoch)
			}
			if err != nil && r.ctx.Err() == nil {
				r.client.logger.Warn("Governed receipt recovery interrupted; closing stream")
				r.cancel()
			}
		}()
	})
	return nil
}

func (r *governedRuntime) recoverManualJobs(epoch string) error {
	after := ""
	for {
		ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
		records, err := r.client.deps.GovernedHospitalJobs.ResumableManual(ctx, after, 100)
		cancel()
		if err != nil {
			return err
		}
		for _, record := range records {
			if record.Task.NodeID != r.client.config.NodeID || record.Task.JobID <= after || record.Task.ReleaseMode != entities.HospitalReleaseManual {
				return entities.ErrGovernedHospitalJobInvalid
			}
			after = record.Task.JobID
			task := record.Task.Clone()
			task.SessionEpoch = epoch
			if err := r.start(task); err != nil {
				return err
			}
		}
		if len(records) < 100 {
			return nil
		}
	}
}

func (r *governedRuntime) recoverReceipts(epoch string) error {
	after := ""
	for {
		ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
		last, complete, err := r.recoverReceiptBatch(ctx, epoch, after)
		cancel()
		if err != nil || complete {
			return err
		}
		after = last
	}
}

func (r *governedRuntime) recoverReceiptBatch(ctx context.Context, epoch, after string) (string, bool, error) {
	state, err := r.client.deps.HospitalGovernance.Read(ctx)
	if err != nil {
		return "", false, err
	}
	if state == nil || !state.Synchronized() || state.Snapshot.SessionEpoch != epoch {
		return "", false, entities.ErrHospitalGovernanceConflict
	}
	items, err := r.client.deps.GovernedHospitalJobs.Unconfirmed(ctx, after, 100)
	if err != nil {
		return "", false, err
	}
	for _, event := range items {
		if event.Binding.NodeID != r.client.config.NodeID || event.Binding.EventID <= after {
			return "", false, entities.ErrGovernedHospitalJobInvalid
		}
		packet, err := wire.GovernedHospitalOutboundLookupToProto(event)
		if err != nil {
			return "", false, err
		}
		select {
		case r.send <- &nodev1.NodeToCenter{Payload: &nodev1.NodeToCenter_ReceiptLookup{ReceiptLookup: packet}}:
		case <-ctx.Done():
			return "", false, ctx.Err()
		}
		after = event.Binding.EventID
	}
	return after, len(items) < 100, nil
}
