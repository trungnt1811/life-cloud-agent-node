package usecases

import (
	"context"
	"errors"
	"math"
	"reflect"
	"time"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	domainerrors "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
)

type GovernedHospitalJobDeps struct {
	NodeID       string
	Repository   repositories.GovernedHospitalJobRepository
	Transactions repositories.TransactionManager
	Now          func() time.Time
}

type governedHospitalJobUseCase struct{ deps GovernedHospitalJobDeps }

func NewGovernedHospitalJobUseCase(deps GovernedHospitalJobDeps) interfaces.GovernedHospitalJobUseCase {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &governedHospitalJobUseCase{deps: deps}
}

func (u *governedHospitalJobUseCase) Read(ctx context.Context, jobID string) (*entities.GovernedHospitalJobRecord, error) {
	if u.deps.Repository == nil {
		return nil, governedHospitalJobError(errors.New("governed hospital repository missing"))
	}
	if !governedHospitalJobID(jobID) {
		return nil, governedHospitalJobError(entities.ErrGovernedHospitalJobInvalid)
	}
	job, err := u.deps.Repository.Get(ctx, u.deps.NodeID, jobID)
	if err != nil {
		return nil, governedHospitalJobError(err)
	}
	if job == nil {
		return nil, nil
	}
	record := job.Record()
	return &record, nil
}

func (u *governedHospitalJobUseCase) ListEvents(ctx context.Context, page, size int) ([]entities.GovernedHospitalOutboundEvent, int64, error) {
	if page < 1 || size < 1 || size > 100 || page-1 > math.MaxInt/size {
		return nil, 0, domainerrors.NewValidationError("INVALID_PAGINATION", "invalid outbound event pagination")
	}
	if u.deps.Repository == nil {
		return nil, 0, governedHospitalJobError(errors.New("governed hospital repository missing"))
	}
	items, total, err := u.deps.Repository.ListEvents(ctx, u.deps.NodeID, size, (page-1)*size)
	if err != nil {
		return nil, 0, governedHospitalJobError(err)
	}
	return items, total, nil
}

func (u *governedHospitalJobUseCase) Begin(ctx context.Context, task entities.GovernedHospitalTask) (*entities.GovernedHospitalJobRecord, error) {
	if task.NodeID != u.deps.NodeID || !governedHospitalJobID(task.JobID) {
		return nil, governedHospitalJobError(entities.ErrGovernedHospitalJobInvalid)
	}
	var output *entities.GovernedHospitalJobRecord
	err := u.transaction(ctx, task.SessionEpoch, func(governance repositories.HospitalGovernanceRepository, jobs repositories.GovernedHospitalJobRepository, state *entities.HospitalGovernance, now time.Time) error {
		job, err := jobs.Get(ctx, u.deps.NodeID, task.JobID)
		if err != nil {
			return err
		}
		if job != nil {
			before := job.Record()
			if err := job.Resume(task, state, now); err != nil {
				return err
			}
			record := job.Record()
			output = &record
			event := "GOVERNED_EXECUTION_REBOUND"
			if before.Phase != record.Phase {
				event = "GOVERNED_LOCAL_TERMINATED"
			}
			return u.persistChanged(ctx, governance, jobs, state, job, before, event, now)
		}
		job, err = entities.NewGovernedHospitalJob(task, state, now)
		if errors.Is(err, entities.ErrGovernedHospitalJobInvalid) {
			job, err = entities.NewGovernedHospitalRefusal(task, state, now)
		}
		if err != nil {
			return err
		}
		if err := jobs.Save(ctx, job); err != nil {
			return err
		}
		event := "GOVERNED_EXECUTION_STARTED"
		if job.Record().ExecutionStartedAt.IsZero() {
			event = "GOVERNED_LOCAL_REFUSED"
		}
		if err := u.audit(ctx, governance, state.Record(), job.Record(), event, now); err != nil {
			return err
		}
		record := job.Record()
		output = &record
		return nil
	})
	if err != nil {
		return nil, governedHospitalJobError(err)
	}
	return output, nil
}

func (u *governedHospitalJobUseCase) Prepare(ctx context.Context, jobID, epoch string, raw uint64) (*entities.GovernedHospitalJobRecord, error) {
	return u.mutate(ctx, jobID, epoch, "GOVERNED_PAYLOAD_PROTECTED", func(job *entities.GovernedHospitalJob, state *entities.HospitalGovernance, now time.Time) error {
		return job.Prepare(raw, state, now)
	})
}

func (u *governedHospitalJobUseCase) Intent(ctx context.Context, jobID, epoch string) (*entities.GovernedHospitalJobRecord, error) {
	return u.mutate(ctx, jobID, epoch, "GOVERNED_RELEASE_INTENT", func(job *entities.GovernedHospitalJob, state *entities.HospitalGovernance, now time.Time) error {
		_, err := job.ReleaseIntent(state, now)
		return err
	})
}

func (u *governedHospitalJobUseCase) Send(ctx context.Context, jobID string, grant entities.GovernedHospitalGrant) (*entities.GovernedHospitalJobRecord, error) {
	return u.mutate(ctx, jobID, grant.Binding.SessionEpoch, "GOVERNED_EGRESS_PREPARED", func(job *entities.GovernedHospitalJob, state *entities.HospitalGovernance, now time.Time) error {
		return job.AuthorizeSend(grant, state, now)
	})
}

func (u *governedHospitalJobUseCase) Receive(ctx context.Context, receipt entities.GovernedHospitalReceipt) (*entities.GovernedHospitalJobRecord, error) {
	// A committed receipt reconciles prior egress even after expiry/revocation.
	// It proves commitment, not permission to send fresh bytes.
	return u.receiveReceipt(ctx, receipt)
}

func (u *governedHospitalJobUseCase) AcknowledgeStage(ctx context.Context, ack entities.GovernedHospitalStageAck) (*entities.GovernedHospitalJobRecord, error) {
	return u.mutate(ctx, ack.JobID, ack.SessionEpoch, "GOVERNED_STAGE_ACKNOWLEDGED", func(job *entities.GovernedHospitalJob, _ *entities.HospitalGovernance, _ time.Time) error {
		return job.AcknowledgeStage(ack)
	})
}

func (u *governedHospitalJobUseCase) Terminate(ctx context.Context, jobID, epoch, phase, reason string) (*entities.GovernedHospitalJobRecord, error) {
	return u.mutate(ctx, jobID, epoch, "GOVERNED_LOCAL_TERMINATED", func(job *entities.GovernedHospitalJob, _ *entities.HospitalGovernance, now time.Time) error {
		return job.Terminate(phase, reason, now)
	})
}

func (u *governedHospitalJobUseCase) mutate(ctx context.Context, jobID, epoch, event string, apply func(*entities.GovernedHospitalJob, *entities.HospitalGovernance, time.Time) error) (*entities.GovernedHospitalJobRecord, error) {
	if !governedHospitalJobID(jobID) {
		return nil, governedHospitalJobError(entities.ErrGovernedHospitalJobInvalid)
	}
	var output *entities.GovernedHospitalJobRecord
	err := u.transaction(ctx, epoch, func(governance repositories.HospitalGovernanceRepository, jobs repositories.GovernedHospitalJobRepository, state *entities.HospitalGovernance, now time.Time) error {
		job, err := jobs.Get(ctx, u.deps.NodeID, jobID)
		if err != nil {
			return err
		}
		if job == nil {
			return entities.ErrGovernedHospitalJobInvalid
		}
		before := job.Record()
		if err := apply(job, state, now); err != nil {
			return err
		}
		record := job.Record()
		output = &record
		return u.persistChanged(ctx, governance, jobs, state, job, before, event, now)
	})
	if err != nil {
		return nil, governedHospitalJobError(err)
	}
	return output, nil
}

func (u *governedHospitalJobUseCase) persistChanged(ctx context.Context, governance repositories.HospitalGovernanceRepository, jobs repositories.GovernedHospitalJobRepository, state *entities.HospitalGovernance, job *entities.GovernedHospitalJob, before entities.GovernedHospitalJobRecord, event string, now time.Time) error {
	record := job.Record()
	if reflect.DeepEqual(before, record) {
		return nil
	}
	if err := jobs.Save(ctx, job); err != nil {
		return err
	}
	if record.Binding.EventID != "" {
		outbound, err := job.OutboundEvent(now)
		if err != nil {
			return err
		}
		if err := jobs.SaveEvent(ctx, outbound); err != nil {
			return err
		}
	}
	return u.audit(ctx, governance, state.Record(), record, event, now)
}

func (u *governedHospitalJobUseCase) transaction(ctx context.Context, epoch string, fn func(repositories.HospitalGovernanceRepository, repositories.GovernedHospitalJobRepository, *entities.HospitalGovernance, time.Time) error) error {
	if u.deps.Transactions == nil || !validHospitalCommandPart(u.deps.NodeID, 128) {
		return errors.New("governed hospital transactions or node identity missing")
	}
	return u.deps.Transactions.WithinTx(ctx, func(repos repositories.TxRepositories) error {
		governance, jobs := repos.HospitalGovernance(), repos.GovernedHospitalJobs()
		if governance == nil || jobs == nil {
			return errors.New("transaction-bound hospital repositories missing")
		}
		// Fixed local policy -> job -> ledger -> audit order; no network in this TX.
		if err := governance.Lock(ctx, u.deps.NodeID); err != nil {
			return err
		}
		state, err := governance.Get(ctx, u.deps.NodeID)
		if err != nil {
			return err
		}
		if state == nil || (epoch != "" && state.Record().Snapshot.SessionEpoch != epoch) {
			return entities.ErrHospitalGovernanceConflict
		}
		return fn(governance, jobs, state, u.deps.Now().UTC().Truncate(time.Microsecond))
	})
}

func (u *governedHospitalJobUseCase) audit(ctx context.Context, repo repositories.HospitalGovernanceRepository, state entities.HospitalGovernanceRecord, job entities.GovernedHospitalJobRecord, event string, now time.Time) error {
	return u.auditAs(ctx, repo, state, job, event, "node-agent", now)
}

func (u *governedHospitalJobUseCase) auditAs(ctx context.Context, repo repositories.HospitalGovernanceRepository, state entities.HospitalGovernanceRecord, job entities.GovernedHospitalJobRecord, event, actor string, now time.Time) error {
	jobID := job.Task.JobID
	var eventID *string
	if job.Binding.EventID != "" {
		id := job.Binding.EventID
		eventID = &id
	}
	queryID := job.Task.QueryID
	return repo.AppendAudit(ctx, entities.HospitalGovernanceAuditRecord{EventID: uuid.NewString(), NodeID: u.deps.NodeID, Type: event, Actor: actor, LocalRevision: state.LocalRevision, SnapshotRevision: state.Snapshot.Revision, PermitID: job.Task.Permit.ID, PermitVersion: job.Task.Permit.Version, PolicyHash: state.Policy.PolicyHash, OccurredAt: now, JobID: &jobID, OutboundEventID: eventID, QueryID: &queryID})
}

func governedHospitalJobID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil
}

func governedHospitalJobError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, entities.ErrHospitalGovernanceConflict) {
		return hospitalGovernanceError(err)
	}
	if errors.Is(err, entities.ErrGovernedHospitalJobInvalid) {
		return domainerrors.NewConflictError("GOVERNED_JOB_CONFLICT", "governed job scope, state or release binding is no longer valid")
	}
	return &domainerrors.DomainError{Type: domainerrors.ErrorTypeUnavailable, Code: "GOVERNANCE_STORE_UNAVAILABLE", Message: "hospital governance store is unavailable", Cause: err}
}
