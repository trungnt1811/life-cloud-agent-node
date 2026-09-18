package wire

import (
	"context"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
)

// ValidateQueryTaskV1 runs the four validation layers (Fig. 3) with the
// version gate first: a task from a schema version this build does not
// understand must not be judged by v1 structural/whitelist/semantic rules,
// or a v2 task would come back REJECTED_INVALID_QUERY instead of
// UNSUPPORTED_VERSION, and would cost a D5 read (or fail on a D5 outage).
// Then: version -> structural -> whitelist -> semantic.
//
// The task is never modified. Field codes may arrive non-canonical (padding,
// lower case); every layer normalizes on read, and so must the executor
// (queryfields.NormalizeFieldCode).
func ValidateQueryTaskV1(
	ctx context.Context,
	task *nodev1.QueryTask,
	repo repositories.EnabledQueryFieldRepository,
) error {
	if err := ValidateQueryTaskVersionV1(task); err != nil {
		return err
	}
	if err := ValidateQueryTaskStructureV1(task); err != nil {
		return err
	}
	if err := ValidateQueryTaskWhitelistV1(ctx, task, repo); err != nil {
		return err
	}
	return ValidateQueryTaskSemanticV1(task)
}
