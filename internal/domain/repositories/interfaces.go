package repositories

import (
	"context"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

// ExampleRepository defines the interface for a generic example repository.
//
//go:generate mockgen -source=interfaces.go -destination=../../mocks/mock_example_repository.go -package=mocks
type ExampleRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entities.ExampleEntity, error)
	Create(ctx context.Context, entity *entities.ExampleEntity) error
	Update(ctx context.Context, entity *entities.ExampleEntity) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]*entities.ExampleEntity, error)
	ListByIDs(ctx context.Context, ids []uuid.UUID) ([]*entities.ExampleEntity, error)
	Count(ctx context.Context) (int, error)
}
