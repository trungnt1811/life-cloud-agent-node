package repositories

import (
	"context"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

// JobProgressRepository is the D4 job progress checkpoint store.
//
//go:generate mockgen -source=job_progress.go -destination=../../mocks/mock_job_progress_repository.go -package=mocks
type JobProgressRepository interface {
	// Get returns the checkpoint for jobID, or nil when there is none.
	Get(ctx context.Context, jobID string) (*entities.JobProgress, error)
	// Save inserts or overwrites the checkpoint row atomically. It returns an
	// error for a nil checkpoint or blank job_id rather than no-oping.
	Save(ctx context.Context, progress *entities.JobProgress) error
	// DeleteFinishedBefore removes done checkpoints last updated before
	// cutoff and returns how many rows it deleted. In-progress rows are
	// never deleted.
	DeleteFinishedBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
