package usecases

import (
	"context"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
)

func (u *governedHospitalJobUseCase) Deny(ctx context.Context, denial entities.GovernedHospitalDenial) (*entities.GovernedHospitalJobRecord, error) {
	if denial.Validate() != nil || denial.Binding.NodeID != u.deps.NodeID {
		return nil, governedHospitalJobError(entities.ErrGovernedHospitalJobInvalid)
	}
	var output *entities.GovernedHospitalJobRecord
	err := u.transaction(ctx, "", func(governance repositories.HospitalGovernanceRepository, jobs repositories.GovernedHospitalJobRepository, state *entities.HospitalGovernance, now time.Time) error {
		job, err := jobs.Get(ctx, u.deps.NodeID, denial.Binding.JobID)
		if err != nil {
			return err
		}
		if job == nil {
			return entities.ErrGovernedHospitalJobInvalid
		}
		before := job.Record()
		if before.Binding.EventID != denial.Binding.EventID {
			prior, err := jobs.GetEvent(ctx, u.deps.NodeID, denial.Binding.EventID)
			if err != nil {
				return err
			}
			if prior == nil || prior.Binding.JobID != denial.Binding.JobID || !prior.MatchesDenial(denial) {
				return entities.ErrGovernedHospitalJobInvalid
			}
			output = &before
			return nil
		}
		if err := job.DenyGrant(denial, now); err != nil {
			return err
		}
		record := job.Record()
		output = &record
		return u.persistChanged(ctx, governance, jobs, state, job, before, "GOVERNED_GRANT_DENIED", now)
	})
	if err != nil {
		return nil, governedHospitalJobError(err)
	}
	return output, nil
}
