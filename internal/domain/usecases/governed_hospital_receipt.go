package usecases

import (
	"context"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
)

func (u *governedHospitalJobUseCase) receiveReceipt(ctx context.Context, receipt entities.GovernedHospitalReceipt) (*entities.GovernedHospitalJobRecord, error) {
	if !governedHospitalJobID(receipt.JobID) || !governedHospitalJobID(receipt.EventID) || !governedHospitalJobID(receipt.ID) {
		return nil, governedHospitalJobError(entities.ErrGovernedHospitalJobInvalid)
	}
	var output *entities.GovernedHospitalJobRecord
	err := u.transaction(ctx, "", func(governance repositories.HospitalGovernanceRepository, jobs repositories.GovernedHospitalJobRepository, state *entities.HospitalGovernance, now time.Time) error {
		job, err := jobs.Get(ctx, u.deps.NodeID, receipt.JobID)
		if err != nil {
			return err
		}
		if job == nil {
			return entities.ErrGovernedHospitalJobInvalid
		}
		before := job.Record()
		if before.Binding.EventID != receipt.EventID {
			// A prior event can prove only its own recorded receipt. Never apply
			// it to the current intent, advance a revision, or authorize a send.
			prior, err := jobs.GetEvent(ctx, u.deps.NodeID, receipt.EventID)
			if err != nil {
				return err
			}
			if prior == nil || prior.Binding.JobID != receipt.JobID || !prior.MatchesReceipt(receipt) {
				return entities.ErrGovernedHospitalJobInvalid
			}
			output = &before
			return nil
		}
		if err := job.Receive(receipt); err != nil {
			return err
		}
		record := job.Record()
		output = &record
		return u.persistChanged(ctx, governance, jobs, state, job, before, "GOVERNED_RESULT_RECEIPT", now)
	})
	if err != nil {
		return nil, governedHospitalJobError(err)
	}
	return output, nil
}
