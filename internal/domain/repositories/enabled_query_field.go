package repositories

import (
	"context"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

// EnabledQueryFieldRepository is the D5 local enabled-query-fields store.
//
//go:generate mockgen -source=enabled_query_field.go -destination=../../mocks/mock_enabled_query_field_repository.go -package=mocks
type EnabledQueryFieldRepository interface {
	ListAll(ctx context.Context) ([]*entities.EnabledQueryField, error)
	GetByFieldCode(ctx context.Context, fieldCode string) (*entities.EnabledQueryField, error)
	Upsert(ctx context.Context, entity *entities.EnabledQueryField) error
}
