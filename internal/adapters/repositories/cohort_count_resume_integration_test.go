package repositories

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// Interrupt the walk mid-chunk, resume with the same job_id, and require the
// final count to equal an uninterrupted run over the same data - no missed and
// no double-counted chunk (plan Phase 6, decision 0005).
func TestCohortCount_InterruptedThenResumedEqualsUninterrupted(t *testing.T) {
	db := openPatientRegistryRepositoryTestDB(t)
	real := NewPatientRegistryRepository(db, logger.GetLogger())
	progressRepo := NewJobProgressRepository(db, logger.GetLogger())
	criteria := seedDemoCohortFixture(t, real)
	ctx := context.Background()

	want, err := real.CountMatchingCohort(ctx, criteria)
	require.NoError(t, err)
	require.Equal(t, uint64(2), want)

	const chunkSize = 2 // 7 candidate patients => chunks of 2, 2, 2, 1

	ctrl := gomock.NewController(t)
	patients := mocks.NewMockPatientRegistryRepository(ctrl)

	// First run: the third chunk's query is cut off by a canceled context.
	interruptCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	firstRunCalls := 0
	patients.EXPECT().
		CountMatchingCohortChunk(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(c context.Context, cr types.CohortCriteria, after uuid.UUID, limit int) (types.CohortChunkResult, error) {
			firstRunCalls++
			if firstRunCalls == 3 {
				cancel()
			}
			return real.CountMatchingCohortChunk(c, cr, after, limit)
		}).
		Times(3)

	uc := usecases.NewCohortCountUseCase(patients, progressRepo, chunkSize, nil)
	_, err = uc.CountMatchingCohort(interruptCtx, "job-resume", criteria)
	require.ErrorIs(t, err, context.Canceled)

	checkpoint, err := progressRepo.Get(ctx, "job-resume")
	require.NoError(t, err)
	require.NotNil(t, checkpoint, "two completed chunks must have been checkpointed")
	require.False(t, checkpoint.Done())
	require.NotEqual(t, uuid.Nil, checkpoint.LastPatientID())

	// Resume: the walk must start at the checkpoint, not from the beginning.
	var resumeAfter []uuid.UUID
	patients.EXPECT().
		CountMatchingCohortChunk(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(c context.Context, cr types.CohortCriteria, after uuid.UUID, limit int) (types.CohortChunkResult, error) {
			resumeAfter = append(resumeAfter, after)
			return real.CountMatchingCohortChunk(c, cr, after, limit)
		}).
		AnyTimes()

	got, err := uc.CountMatchingCohort(ctx, "job-resume", criteria)
	require.NoError(t, err)
	require.Equal(t, want, got, "resumed count must equal the uninterrupted count")
	require.Equal(t, checkpoint.LastPatientID(), resumeAfter[0], "resume starts at the checkpoint")
	require.Len(t, resumeAfter, 2, "only the two unfinished chunks are re-read")

	done, err := progressRepo.Get(ctx, "job-resume")
	require.NoError(t, err)
	require.True(t, done.Done())
	require.Equal(t, want, done.RunningCount())

	// A repeat of the finished job is served from D4 without touching D3.
	resumeAfter = nil
	again, err := uc.CountMatchingCohort(ctx, "job-resume", criteria)
	require.NoError(t, err)
	require.Equal(t, want, again)
	require.Empty(t, resumeAfter)
}
