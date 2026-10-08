package client

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

type governedGuardFailure struct {
	phase, reason string
	err           error
}

func (a *governedActor) count(parent context.Context, record entities.GovernedHospitalJobRecord) (uint64, string, string, error) {
	ctx, cancel := context.WithDeadline(parent, record.ExecutionDeadline)
	defer cancel()
	if failure := a.guard(ctx); failure.err != nil {
		return 0, failure.phase, failure.reason, failure.err
	}
	failures := make(chan governedGuardFailure, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if failure := a.guard(ctx); failure.err != nil {
					failures <- failure
					cancel()
					return
				}
			}
		}
	}()
	raw, err := a.runtime.client.deps.CohortCounter.CountMatchingCohort(ctx, a.task.JobID, a.task.Criteria)
	executionErr := ctx.Err()
	cancel()
	wg.Wait()
	select {
	case failure := <-failures:
		return 0, failure.phase, failure.reason, failure.err
	default:
	}
	if parent.Err() != nil {
		return 0, "", "", parent.Err()
	}
	if executionErr != nil {
		return 0, entities.GovernedHospitalTimedOut, "INTERNAL_FAILURE", executionErr
	}
	if err != nil {
		return 0, entities.GovernedHospitalFailed, "INTERNAL_FAILURE", err
	}
	return raw, "", "", nil
}

func (a *governedActor) guard(ctx context.Context) governedGuardFailure {
	r, err := a.runtime.client.deps.HospitalGovernance.Read(ctx)
	if err != nil {
		return governedGuardFailure{err: err}
	}
	if r == nil {
		return governedGuardFailure{err: entities.ErrHospitalGovernanceInvalid}
	}
	deny := func(phase, reason string) governedGuardFailure {
		return governedGuardFailure{phase: phase, reason: reason, err: entities.ErrGovernedHospitalJobInvalid}
	}
	if r.Paused {
		return deny(entities.GovernedHospitalDeclined, "HOSPITAL_PAUSED")
	}
	for _, fence := range r.InvalidatedQueries {
		if fence.TenantID == a.task.TenantID && fence.QueryID == a.task.QueryID {
			return deny(entities.GovernedHospitalCanceled, fence.Reason)
		}
	}
	for _, fence := range r.InvalidatedPermits {
		if fence.TenantID == a.task.TenantID && fence.PermitID == a.task.Permit.ID && a.task.Permit.Version <= fence.ThroughVersion {
			reason := types.GovernanceReasonPermitRevoked
			for _, permit := range r.Snapshot.Permits {
				if permit.TenantID == a.task.TenantID && permit.ID == a.task.Permit.ID && permit.Version > a.task.Permit.Version {
					reason = types.GovernanceReasonPermitSuperseded
				}
			}
			return deny(entities.GovernedHospitalPolicyDenied, reason)
		}
	}
	if !r.Synchronized() || r.Snapshot.SessionEpoch != a.task.SessionEpoch {
		return governedGuardFailure{err: errors.New("governed synchronization not current")}
	}
	state := entities.NewHospitalGovernanceFromRecord(*r)
	if a.task.Validate(state, time.Now().UTC(), false) != nil {
		return deny(entities.GovernedHospitalPolicyDenied, "SCOPE_DENIED")
	}
	return governedGuardFailure{}
}
