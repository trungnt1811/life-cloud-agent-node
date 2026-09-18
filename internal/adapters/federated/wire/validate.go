package wire

import (
	"context"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
)

// ValidateQueryTaskV1 runs Fig. 3's four validation layers in order:
// structural → whitelist → semantic → version.
func ValidateQueryTaskV1(
	ctx context.Context,
	task *nodev1.QueryTask,
	repo repositories.EnabledQueryFieldRepository,
) error {
	if err := ValidateQueryTaskStructureV1(task); err != nil {
		return err
	}
	if err := ValidateQueryTaskWhitelistV1(ctx, task, repo); err != nil {
		return err
	}
	if err := ValidateQueryTaskSemanticV1(task); err != nil {
		return err
	}
	if err := ValidateQueryTaskVersionV1(task); err != nil {
		return err
	}
	return nil
}
