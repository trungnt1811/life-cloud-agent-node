package contracts

import (
	"time"

	"github.com/google/uuid"
)

// CreateExampleInput is the use-case command for creating an example record.
type CreateExampleInput struct {
	Name        string
	Description string
}

// UpdateExampleInput is the use-case command for updating an example record.
type UpdateExampleInput struct {
	ID          uuid.UUID
	Name        string
	Description string
}

// ListExamplesInput is the use-case query for listing example records.
type ListExamplesInput struct {
	Page     int
	PageSize int
}

// ExampleOutput is the read model returned by example use cases.
type ExampleOutput struct {
	ID          uuid.UUID
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
