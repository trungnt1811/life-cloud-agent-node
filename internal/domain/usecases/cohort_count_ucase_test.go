package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func cohortCriteria() types.CohortCriteria {
	return types.CohortCriteria{
		From:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
		Conditions: []types.CohortCondition{{FieldCode: "MCV", Op: types.ComparisonOpLT, NumberValue: "80"}},
	}
}

type cohortCountFixture struct {
	patients *mocks.MockPatientRegistryRepository
	progress *mocks.MockJobProgressRepository
}

func newCohortCountFixture(t *testing.T) cohortCountFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	m := cohortCountFixture{
		patients: mocks.NewMockPatientRegistryRepository(ctrl),
		progress: mocks.NewMockJobProgressRepository(ctrl),
	}
	// Housekeeping runs at the start of every job; not under test here.
	m.progress.EXPECT().DeleteFinishedBefore(gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	return m
}

func TestCohortCount_SumsChunksAndCheckpointsEach(t *testing.T) {
	m := newCohortCountFixture(t)
	criteria := cohortCriteria()
	p2, p3 := uuid.New(), uuid.New()

	m.progress.EXPECT().Get(gomock.Any(), "job-1").Return(nil, nil)
	gomock.InOrder(
		m.patients.EXPECT().CountMatchingCohortChunk(gomock.Any(), criteria, uuid.Nil, 2).
			Return(types.CohortChunkResult{MatchingCount: 1, PatientsScanned: 2, LastPatientID: p2}, nil),
		m.patients.EXPECT().CountMatchingCohortChunk(gomock.Any(), criteria, p2, 2).
			Return(types.CohortChunkResult{MatchingCount: 2, PatientsScanned: 1, LastPatientID: p3}, nil),
	)
	var saved []entities.JobProgressRecord
	m.progress.EXPECT().Save(gomock.Any(), gomock.Any()).Times(2).
		DoAndReturn(func(_ context.Context, progress *entities.JobProgress) error {
			saved = append(saved, progress.Record())
			return nil
		})

	uc := usecases.NewCohortCountUseCase(m.patients, m.progress, 2, nil)
	count, err := uc.CountMatchingCohort(context.Background(), "job-1", criteria)
	require.NoError(t, err)
	require.Equal(t, uint64(3), count)

	require.Len(t, saved, 2)
	require.Equal(t, entities.JobProgressInProgress, saved[0].Status)
	require.Equal(t, p2, saved[0].LastPatientID)
	require.Equal(t, uint64(1), saved[0].RunningCount, "cursor and count are saved together")
	require.Equal(t, entities.JobProgressDone, saved[1].Status, "a short chunk is the last")
	require.Equal(t, uint64(3), saved[1].RunningCount)
	require.Equal(t, criteria.Fingerprint(), saved[1].CriteriaHash)
}

func TestCohortCount_ExactMultipleEndsOnAnEmptyChunk(t *testing.T) {
	m := newCohortCountFixture(t)
	criteria := cohortCriteria()
	p2 := uuid.New()

	m.progress.EXPECT().Get(gomock.Any(), "job-1").Return(nil, nil)
	gomock.InOrder(
		m.patients.EXPECT().CountMatchingCohortChunk(gomock.Any(), criteria, uuid.Nil, 2).
			Return(types.CohortChunkResult{MatchingCount: 2, PatientsScanned: 2, LastPatientID: p2}, nil),
		m.patients.EXPECT().CountMatchingCohortChunk(gomock.Any(), criteria, p2, 2).
			Return(types.CohortChunkResult{}, nil),
	)
	m.progress.EXPECT().Save(gomock.Any(), gomock.Any()).Times(2).Return(nil)

	uc := usecases.NewCohortCountUseCase(m.patients, m.progress, 2, nil)
	count, err := uc.CountMatchingCohort(context.Background(), "job-1", criteria)
	require.NoError(t, err)
	require.Equal(t, uint64(2), count)
}

func TestCohortCount_ResumesFromCheckpointForSameCriteria(t *testing.T) {
	m := newCohortCountFixture(t)
	criteria := cohortCriteria()
	checkpoint, last := uuid.New(), uuid.New()

	stored := entities.NewJobProgress("job-1", criteria.Fingerprint(), time.Now())
	stored.CompleteChunk(checkpoint, 4, time.Now())
	m.progress.EXPECT().Get(gomock.Any(), "job-1").Return(stored, nil)
	m.patients.EXPECT().CountMatchingCohortChunk(gomock.Any(), criteria, checkpoint, 2).
		Return(types.CohortChunkResult{MatchingCount: 1, PatientsScanned: 1, LastPatientID: last}, nil)
	m.progress.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	uc := usecases.NewCohortCountUseCase(m.patients, m.progress, 2, nil)
	count, err := uc.CountMatchingCohort(context.Background(), "job-1", criteria)
	require.NoError(t, err)
	require.Equal(t, uint64(5), count, "4 already counted + 1 new; the first chunks are not re-read")
}

func TestCohortCount_DoneJobIsServedWithoutQuerying(t *testing.T) {
	m := newCohortCountFixture(t)
	criteria := cohortCriteria()

	stored := entities.NewJobProgress("job-1", criteria.Fingerprint(), time.Now())
	stored.CompleteChunk(uuid.New(), 9, time.Now())
	stored.MarkDone(time.Now())
	m.progress.EXPECT().Get(gomock.Any(), "job-1").Return(stored, nil)

	uc := usecases.NewCohortCountUseCase(m.patients, m.progress, 2, nil)
	count, err := uc.CountMatchingCohort(context.Background(), "job-1", criteria)
	require.NoError(t, err)
	require.Equal(t, uint64(9), count)
}

func TestCohortCount_ReusedJobIDWithDifferentCriteriaStartsOver(t *testing.T) {
	m := newCohortCountFixture(t)
	criteria := cohortCriteria()

	stale := entities.NewJobProgress("job-1", "hash-of-another-query", time.Now())
	stale.CompleteChunk(uuid.New(), 99, time.Now())
	stale.MarkDone(time.Now())
	m.progress.EXPECT().Get(gomock.Any(), "job-1").Return(stale, nil)
	m.patients.EXPECT().CountMatchingCohortChunk(gomock.Any(), criteria, uuid.Nil, 2).
		Return(types.CohortChunkResult{MatchingCount: 1, PatientsScanned: 1, LastPatientID: uuid.New()}, nil)
	m.progress.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	uc := usecases.NewCohortCountUseCase(m.patients, m.progress, 2, nil)
	count, err := uc.CountMatchingCohort(context.Background(), "job-1", criteria)
	require.NoError(t, err)
	require.Equal(t, uint64(1), count, "the other query's 99 must not leak in")
}

func TestCohortCount_FailuresAreReturnedNotSwallowed(t *testing.T) {
	criteria := cohortCriteria()

	t.Run("blank_job_id", func(t *testing.T) {
		m := newCohortCountFixture(t)
		uc := usecases.NewCohortCountUseCase(m.patients, m.progress, 2, nil)
		_, err := uc.CountMatchingCohort(context.Background(), "  ", criteria)
		require.Error(t, err)
	})

	t.Run("chunk_query_error_keeps_earlier_checkpoint", func(t *testing.T) {
		m := newCohortCountFixture(t)
		m.progress.EXPECT().Get(gomock.Any(), "job-1").Return(nil, nil)
		m.patients.EXPECT().CountMatchingCohortChunk(gomock.Any(), criteria, uuid.Nil, 2).
			Return(types.CohortChunkResult{}, context.Canceled)
		// No Save: nothing completed, and nothing is marked failed.

		uc := usecases.NewCohortCountUseCase(m.patients, m.progress, 2, nil)
		_, err := uc.CountMatchingCohort(context.Background(), "job-1", criteria)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("non_advancing_cursor_fails_instead_of_looping", func(t *testing.T) {
		m := newCohortCountFixture(t)
		stuck := uuid.New()
		stored := entities.NewJobProgress("job-1", criteria.Fingerprint(), time.Now())
		stored.CompleteChunk(stuck, 1, time.Now())
		m.progress.EXPECT().Get(gomock.Any(), "job-1").Return(stored, nil)
		m.patients.EXPECT().CountMatchingCohortChunk(gomock.Any(), criteria, stuck, 2).
			Return(types.CohortChunkResult{MatchingCount: 1, PatientsScanned: 2, LastPatientID: stuck}, nil)

		uc := usecases.NewCohortCountUseCase(m.patients, m.progress, 2, nil)
		_, err := uc.CountMatchingCohort(context.Background(), "job-1", criteria)
		require.ErrorContains(t, err, "did not advance")
	})

	t.Run("save_error", func(t *testing.T) {
		m := newCohortCountFixture(t)
		m.progress.EXPECT().Get(gomock.Any(), "job-1").Return(nil, nil)
		m.patients.EXPECT().CountMatchingCohortChunk(gomock.Any(), criteria, uuid.Nil, 2).
			Return(types.CohortChunkResult{MatchingCount: 1, PatientsScanned: 2, LastPatientID: uuid.New()}, nil)
		m.progress.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("db down"))

		uc := usecases.NewCohortCountUseCase(m.patients, m.progress, 2, nil)
		_, err := uc.CountMatchingCohort(context.Background(), "job-1", criteria)
		require.ErrorContains(t, err, "db down")
	})

	t.Run("canceled_context_before_first_chunk", func(t *testing.T) {
		m := newCohortCountFixture(t)
		m.progress.EXPECT().Get(gomock.Any(), "job-1").Return(nil, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		uc := usecases.NewCohortCountUseCase(m.patients, m.progress, 2, nil)
		_, err := uc.CountMatchingCohort(ctx, "job-1", criteria)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("prune_failure_does_not_fail_the_job", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		patients := mocks.NewMockPatientRegistryRepository(ctrl)
		progress := mocks.NewMockJobProgressRepository(ctrl)
		progress.EXPECT().DeleteFinishedBefore(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("boom"))
		progress.EXPECT().Get(gomock.Any(), "job-1").Return(nil, nil)
		patients.EXPECT().CountMatchingCohortChunk(gomock.Any(), criteria, uuid.Nil, 2).
			Return(types.CohortChunkResult{MatchingCount: 1, PatientsScanned: 1, LastPatientID: uuid.New()}, nil)
		progress.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

		uc := usecases.NewCohortCountUseCase(patients, progress, 2, nil)
		count, err := uc.CountMatchingCohort(context.Background(), "job-1", criteria)
		require.NoError(t, err)
		require.Equal(t, uint64(1), count)
	})
}
