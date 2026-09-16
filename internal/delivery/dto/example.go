package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/contracts"
	domaintypes "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

// CreateExampleRequest is the HTTP request body for creating an example.
type CreateExampleRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateExampleRequest is the HTTP request body for updating an example.
type UpdateExampleRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// ExampleDTO represents the DTO for example entities.
type ExampleDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ExampleListDTO represents a paginated list of examples.
type ExampleListDTO struct {
	Items      []ExampleDTO `json:"items"`
	TotalCount int          `json:"total_count"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
	NextPage   *int         `json:"next_page,omitempty"`
}

// ExampleDeletedDTO represents a successful example deletion response.
type ExampleDeletedDTO struct {
	ID      uuid.UUID `json:"id"`
	Deleted bool      `json:"deleted"`
}

// ToInput maps the request to a create use-case command.
func (r CreateExampleRequest) ToInput() contracts.CreateExampleInput {
	return contracts.CreateExampleInput{
		Name:        r.Name,
		Description: r.Description,
	}
}

// ToInput maps the request to an update use-case command.
func (r UpdateExampleRequest) ToInput(id uuid.UUID) contracts.UpdateExampleInput {
	return contracts.UpdateExampleInput{
		ID:          id,
		Name:        r.Name,
		Description: r.Description,
	}
}

// NewExampleDTOFromOutput maps an example use-case output to an HTTP DTO.
func NewExampleDTOFromOutput(output *contracts.ExampleOutput) *ExampleDTO {
	if output == nil {
		return nil
	}
	return &ExampleDTO{
		ID:          output.ID,
		Name:        output.Name,
		Description: output.Description,
		CreatedAt:   output.CreatedAt,
		UpdatedAt:   output.UpdatedAt,
	}
}

// NewExampleDTOListFromOutputs maps example use-case outputs to HTTP DTOs.
func NewExampleDTOListFromOutputs(outputs []*contracts.ExampleOutput) []ExampleDTO {
	if outputs == nil {
		return nil
	}

	dtos := make([]ExampleDTO, 0, len(outputs))
	for _, output := range outputs {
		if output != nil {
			dtos = append(dtos, *NewExampleDTOFromOutput(output))
		}
	}
	return dtos
}

// NewExampleListDTOFromOutput maps a paginated use-case output to an HTTP DTO.
func NewExampleListDTOFromOutput(result *domaintypes.PaginatedResponse[*contracts.ExampleOutput]) *ExampleListDTO {
	if result == nil {
		return nil
	}
	return &ExampleListDTO{
		Items:      NewExampleDTOListFromOutputs(result.Items),
		TotalCount: result.TotalCount,
		Page:       result.Page,
		PageSize:   result.PageSize,
		NextPage:   result.NextPage,
	}
}

// NewExampleDeletedDTO builds a successful deletion DTO.
func NewExampleDeletedDTO(id uuid.UUID) *ExampleDeletedDTO {
	return &ExampleDeletedDTO{
		ID:      id,
		Deleted: true,
	}
}
