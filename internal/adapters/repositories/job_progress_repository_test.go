package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

func TestJobProgressRepository_SaveGetAndOverwrite(t *testing.T) {
	db := openPatientRegistryRepositoryTestDB(t)
	repo := NewJobProgressRepository(db, logger.GetLogger())
	ctx := context.Background()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	missing, err := repo.Get(ctx, "nope")
	require.NoError(t, err)
	require.Nil(t, missing)

	progress := entities.NewJobProgress("job-1", "hash-a", now)
	require.NoError(t, repo.Save(ctx, progress))

	got, err := repo.Get(ctx, "job-1")
	require.NoError(t, err)
	require.Equal(t, uuid.Nil, got.LastPatientID(), "no chunk yet is stored as NULL")
	require.Equal(t, entities.JobProgressInProgress, got.Status())

	last := uuid.New()
	progress.CompleteChunk(last, 5, now.Add(time.Minute))
	require.NoError(t, repo.Save(ctx, progress))
	progress.MarkDone(now.Add(2 * time.Minute))
	require.NoError(t, repo.Save(ctx, progress))

	got, err = repo.Get(ctx, "job-1")
	require.NoError(t, err)
	want, have := progress.Record(), got.Record()
	require.True(t, want.UpdatedAt.Equal(have.UpdatedAt), "second and third Save overwrite the one row")
	want.UpdatedAt, have.UpdatedAt = time.Time{}, time.Time{}
	require.Equal(t, want, have)
	require.Equal(t, last, got.LastPatientID())
	require.Equal(t, uint64(5), got.RunningCount())
	require.True(t, got.Done())
}

func TestJobProgressRepository_SaveRejectsNilAndBlankJobID(t *testing.T) {
	db := openPatientRegistryRepositoryTestDB(t)
	repo := NewJobProgressRepository(db, logger.GetLogger())

	require.Error(t, repo.Save(context.Background(), nil))
	require.Error(t, repo.Save(context.Background(), entities.NewJobProgress("  ", "h", time.Now())))
}

func TestJobProgressRepository_DeleteFinishedBeforeKeepsInProgressAndRecent(t *testing.T) {
	db := openPatientRegistryRepositoryTestDB(t)
	repo := NewJobProgressRepository(db, logger.GetLogger())
	ctx := context.Background()
	cutoff := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)

	save := func(jobID string, updated time.Time, done bool) {
		t.Helper()
		progress := entities.NewJobProgress(jobID, "h", updated)
		if done {
			progress.MarkDone(updated)
		}
		require.NoError(t, repo.Save(ctx, progress))
	}
	save("old-done", cutoff.Add(-time.Hour), true)
	save("recent-done", cutoff.Add(time.Hour), true)
	save("old-in-progress", cutoff.Add(-48*time.Hour), false)

	deleted, err := repo.DeleteFinishedBefore(ctx, cutoff)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	for jobID, wantPresent := range map[string]bool{"old-done": false, "recent-done": true, "old-in-progress": true} {
		got, err := repo.Get(ctx, jobID)
		require.NoError(t, err)
		require.Equal(t, wantPresent, got != nil, jobID)
	}
}
