package repositories

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/repohelpers"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

type jobProgressRepository struct {
	db     *gorm.DB
	logger logger.Logger
}

// NewJobProgressRepository creates a GORM-backed JobProgressRepository.
func NewJobProgressRepository(db *gorm.DB, logger logger.Logger) repositories.JobProgressRepository {
	return &jobProgressRepository{
		db:     db,
		logger: logger,
	}
}

func (r *jobProgressRepository) dbWithContext(ctx context.Context) *gorm.DB {
	return repohelpers.DBWithContext(ctx, r.db)
}

// Get returns the checkpoint for jobID, or nil when there is none.
func (r *jobProgressRepository) Get(ctx context.Context, jobID string) (*entities.JobProgress, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, nil
	}

	var model models.JobProgress
	if err := r.dbWithContext(ctx).First(&model, "job_id = ?", jobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		r.logger.Error("Failed to get job progress", logger.String("job_id", jobID), logger.Err(err))
		return nil, err
	}
	return jobProgressEntityFromModel(&model), nil
}

// Save inserts or overwrites the checkpoint row in one atomic statement.
func (r *jobProgressRepository) Save(ctx context.Context, progress *entities.JobProgress) error {
	// progress.JobID() is nil-receiver safe, so this also catches nil.
	if strings.TrimSpace(progress.JobID()) == "" {
		return errors.New("job progress: job_id is required")
	}

	model := jobProgressModelFromEntity(progress)
	if err := r.dbWithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "job_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"criteria_hash", "last_patient_id", "running_count", "status", "updated_at",
			}),
		}).
		Create(model).Error; err != nil {
		r.logger.Error("Failed to save job progress", logger.String("job_id", model.JobID), logger.Err(err))
		return err
	}
	return nil
}

// DeleteFinishedBefore removes done checkpoints last updated before cutoff.
func (r *jobProgressRepository) DeleteFinishedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result := r.dbWithContext(ctx).
		Where("status = ? AND updated_at < ?", string(entities.JobProgressDone), cutoff).
		Delete(&models.JobProgress{})
	if result.Error != nil {
		r.logger.Error("Failed to delete finished job progress", logger.Err(result.Error))
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
