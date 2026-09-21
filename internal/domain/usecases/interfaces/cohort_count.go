package interfaces

import (
	"context"

	domaintypes "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

// CohortCountUseCase counts a cohort in checkpointed, resumable chunks
// (decision 0005). It returns the raw, unsuppressed total; small-cell
// suppression is the caller's final step.
//
//go:generate mockgen -source=cohort_count.go -destination=../../../mocks/mock_cohort_count_ucase.go -package=mocks
type CohortCountUseCase interface {
	// CountMatchingCohort resumes the job's checkpoint when one exists for the
	// same criteria, and otherwise starts from the beginning. A canceled or
	// expired ctx is returned as an error with the checkpoint left in place,
	// so calling again with the same jobID continues the count.
	CountMatchingCohort(ctx context.Context, jobID string, criteria domaintypes.CohortCriteria) (uint64, error)
}
