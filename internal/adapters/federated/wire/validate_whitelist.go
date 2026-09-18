package wire

import (
	"context"
	"fmt"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
)

// ValidateQueryTaskWhitelistV1 is layer 2: every referenced field_code must
// exist and be enabled in this node's EnabledQueryFieldRepository (D5).
func ValidateQueryTaskWhitelistV1(
	ctx context.Context,
	task *nodev1.QueryTask,
	repo repositories.EnabledQueryFieldRepository,
) error {
	if err := requireTask(task); err != nil {
		return err
	}
	if repo == nil {
		return rejectedInvalidQuery("enabled query field repository is not configured")
	}

	stored, err := repo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list enabled query fields: %w", err)
	}

	enabled := make(map[string]bool, len(stored))
	for _, field := range stored {
		if field == nil {
			continue
		}
		code := queryfields.NormalizeFieldCode(field.FieldCode())
		if code == "" {
			continue
		}
		enabled[code] = field.Enabled()
	}

	for i, condition := range task.GetConditions() {
		if condition == nil {
			continue
		}
		code := queryfields.NormalizeFieldCode(condition.GetFieldCode())
		if !enabled[code] {
			return rejectedInvalidQuery(fmt.Sprintf(
				"conditions[%d].field_code %q is unknown or not enabled on this node",
				i, condition.GetFieldCode(),
			))
		}
	}

	for i, panel := range task.GetRequiredPanels() {
		if panel == nil {
			continue
		}
		for j, rawCode := range panel.GetFieldCodes() {
			code := queryfields.NormalizeFieldCode(rawCode)
			if !enabled[code] {
				return rejectedInvalidQuery(fmt.Sprintf(
					"required_panels[%d].field_codes[%d] %q is unknown or not enabled on this node",
					i, j, rawCode,
				))
			}
		}
	}

	return nil
}
