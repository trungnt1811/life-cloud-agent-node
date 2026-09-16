package interfaces

import (
	"context"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/contracts"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

// ExampleUseCase defines the interface for example use case operations.
//
//go:generate mockgen -source=example.go -destination=../../../mocks/mock_example_ucase.go -package=mocks
type ExampleUseCase interface {
	GetExample(ctx context.Context, id uuid.UUID) (*contracts.ExampleOutput, error)
	CreateExample(ctx context.Context, input contracts.CreateExampleInput) (*contracts.ExampleOutput, error)
	UpdateExample(ctx context.Context, input contracts.UpdateExampleInput) (*contracts.ExampleOutput, error)
	DeleteExample(ctx context.Context, id uuid.UUID) error
	ListExamples(ctx context.Context, input contracts.ListExamplesInput) (*types.PaginatedResponse[*contracts.ExampleOutput], error)
}
