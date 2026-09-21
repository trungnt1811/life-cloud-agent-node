package usecases

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	domaintypes "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
	loggerpkg "github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

var errNonAdvancingCursor = errors.New("cohort chunk cursor did not advance")

type cohortCountUseCase struct {
	patients  repositories.PatientRegistryRepository
	progress  repositories.JobProgressRepository
	chunkSize int
	logger    loggerpkg.Logger
	now       func() time.Time
}

// NewCohortCountUseCase creates the checkpointed cohort counter. A chunkSize
// below 1 means constants.DefaultJobChunkSize.
func NewCohortCountUseCase(
	patients repositories.PatientRegistryRepository,
	progress repositories.JobProgressRepository,
	chunkSize int,
	logger loggerpkg.Logger,
) interfaces.CohortCountUseCase {
	if logger == nil {
		logger = loggerpkg.GetLogger()
	}
	if chunkSize < 1 {
		chunkSize = constants.DefaultJobChunkSize
	}
	return &cohortCountUseCase{
		patients:  patients,
		progress:  progress,
		chunkSize: chunkSize,
		logger:    logger,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func (u *cohortCountUseCase) CountMatchingCohort(
	ctx context.Context,
	jobID string,
	criteria domaintypes.CohortCriteria,
) (uint64, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return 0, errors.New("job_id is required")
	}
	hash := criteria.Fingerprint()

	u.pruneFinished(ctx)

	progress, err := u.progress.Get(ctx, jobID)
	if err != nil {
		return 0, err
	}
	if progress != nil && progress.CriteriaHash() != hash {
		u.logger.Warn("job_id reused with different criteria; discarding checkpoint", loggerpkg.String("job_id", jobID))
		progress = nil
	}
	if progress.Done() {
		return progress.RunningCount(), nil
	}
	if progress == nil {
		progress = entities.NewJobProgress(jobID, hash, u.now())
	}

	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}

		chunk, err := u.patients.CountMatchingCohortChunk(ctx, criteria, progress.LastPatientID(), u.chunkSize)
		if err != nil {
			return 0, err
		}
		if chunk.PatientsScanned > 0 {
			// A cursor that fails to advance would re-read the same chunk
			// forever (and double-count it); fail loudly instead.
			if previous := progress.LastPatientID(); bytes.Compare(chunk.LastPatientID[:], previous[:]) <= 0 {
				return 0, errNonAdvancingCursor
			}
			progress.CompleteChunk(chunk.LastPatientID, chunk.MatchingCount, u.now())
		}
		// A short chunk is the last one: nothing follows it.
		finished := chunk.PatientsScanned < u.chunkSize
		if finished {
			progress.MarkDone(u.now())
		}
		if err := u.progress.Save(ctx, progress); err != nil {
			return 0, err
		}
		if finished {
			return progress.RunningCount(), nil
		}
	}
}

// pruneFinished is best-effort housekeeping: a failure must not fail the job.
func (u *cohortCountUseCase) pruneFinished(ctx context.Context) {
	if _, err := u.progress.DeleteFinishedBefore(ctx, u.now().Add(-constants.JobProgressRetention)); err != nil {
		u.logger.Warn("failed to prune finished job progress", loggerpkg.Err(err))
	}
}
