package usecases

import (
	"context"
	"errors"
	"math"
	"reflect"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
)

func (u *governedHospitalJobUseCase) ListApprovals(ctx context.Context, page, size int) ([]entities.GovernedHospitalJobRecord, int64, error) {
	if page < 1 || size < 1 || size > 100 || page-1 > math.MaxInt/size {
		return nil, 0, governedHospitalJobError(entities.ErrGovernedHospitalJobInvalid)
	}
	if u.deps.Repository == nil {
		return nil, 0, governedHospitalJobError(errors.New("governed hospital repository missing"))
	}
	items, total, err := u.deps.Repository.ListAwaitingApproval(ctx, u.deps.NodeID, size, (page-1)*size)
	return items, total, governedHospitalJobError(err)
}

func (u *governedHospitalJobUseCase) ResumableManual(ctx context.Context, after string, limit int) ([]entities.GovernedHospitalJobRecord, error) {
	if limit < 1 || limit > 100 || (after != "" && !governedHospitalJobID(after)) {
		return nil, governedHospitalJobError(entities.ErrGovernedHospitalJobInvalid)
	}
	if u.deps.Repository == nil {
		return nil, governedHospitalJobError(errors.New("governed hospital repository missing"))
	}
	items, err := u.deps.Repository.ListManualResumable(ctx, u.deps.NodeID, after, limit)
	return items, governedHospitalJobError(err)
}

func (u *governedHospitalJobUseCase) Approve(ctx context.Context, actor, jobID string, expectedRevision int64, commandID string) (*entities.GovernedHospitalJobRecord, error) {
	return u.decideApproval(ctx, actor, jobID, expectedRevision, commandID, true)
}

func (u *governedHospitalJobUseCase) Decline(ctx context.Context, actor, jobID string, expectedRevision int64, commandID string) (*entities.GovernedHospitalJobRecord, error) {
	return u.decideApproval(ctx, actor, jobID, expectedRevision, commandID, false)
}

func (u *governedHospitalJobUseCase) ExpireApproval(ctx context.Context, jobID string) (*entities.GovernedHospitalJobRecord, error) {
	return u.decideApproval(ctx, "node-agent", jobID, 0, "", false)
}

func (u *governedHospitalJobUseCase) decideApproval(ctx context.Context, actor, jobID string, expectedRevision int64, commandID string, approve bool) (*entities.GovernedHospitalJobRecord, error) {
	if !governedHospitalJobID(jobID) {
		return nil, governedHospitalJobError(entities.ErrGovernedHospitalJobInvalid)
	}
	var output *entities.GovernedHospitalJobRecord
	err := u.transaction(ctx, "", func(governance repositories.HospitalGovernanceRepository, jobs repositories.GovernedHospitalJobRepository, state *entities.HospitalGovernance, now time.Time) error {
		job, err := jobs.Get(ctx, u.deps.NodeID, jobID)
		if err != nil {
			return err
		}
		if job == nil {
			return entities.ErrGovernedHospitalJobInvalid
		}
		before := job.Record()
		event, auditActor, err := applyGovernedApprovalDecision(job, state, actor, expectedRevision, commandID, approve, now)
		if err != nil {
			return err
		}
		record := job.Record()
		output = &record
		if reflect.DeepEqual(before, record) {
			return nil
		}
		if err := jobs.Save(ctx, job); err != nil {
			return err
		}
		return u.auditAs(ctx, governance, state.Record(), record, event, auditActor, now)
	})
	if err != nil {
		return nil, governedHospitalJobError(err)
	}
	return output, nil
}

func applyGovernedApprovalDecision(job *entities.GovernedHospitalJob, state *entities.HospitalGovernance, actor string, expected int64, command string, approve bool, now time.Time) (string, string, error) {
	r := job.Record()
	switch {
	case r.Phase == entities.GovernedHospitalWaitingApproval && !r.ApprovalDeadline.IsZero() && !now.Before(r.ApprovalDeadline):
		return "APPROVAL_EXPIRED", "node-agent", job.ExpireApproval(now)
	case r.Phase == entities.GovernedHospitalWaitingApproval && !now.Before(r.Task.QueryDeadline):
		return "QUERY_EXPIRED", "node-agent", job.Terminate(entities.GovernedHospitalCanceled, "QUERY_EXPIRED", now)
	case expected == 0 && command == "" && r.Phase == entities.GovernedHospitalApprovalExpired:
		return "", "", nil
	case approve:
		return "LOCAL_APPROVED", actor, job.Approve(expected, command, actor, state, now)
	case expected != 0 || command != "":
		return "LOCAL_DECLINED", actor, job.Decline(expected, command, actor, state, now)
	default:
		return "", "", entities.ErrGovernedHospitalJobInvalid
	}
}

var _ interfaces.GovernedHospitalJobUseCase = (*governedHospitalJobUseCase)(nil)
