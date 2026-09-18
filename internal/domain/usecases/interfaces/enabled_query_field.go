package interfaces

import (
	"context"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/contracts"
)

// EnabledQueryFieldUseCase defines admin operations for the local field whitelist.
//
//go:generate mockgen -source=enabled_query_field.go -destination=../../../mocks/mock_enabled_query_field_ucase.go -package=mocks
type EnabledQueryFieldUseCase interface {
	ListQueryFields(ctx context.Context) ([]*contracts.EnabledQueryFieldOutput, error)
	UpdateQueryField(ctx context.Context, input contracts.UpdateEnabledQueryFieldInput) (*contracts.EnabledQueryFieldOutput, error)
}
