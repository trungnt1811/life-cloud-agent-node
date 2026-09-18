package dto

import (
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/contracts"
)

// UpdateEnabledQueryFieldRequest is the HTTP body for toggling a query field.
// Enabled is a pointer so omitted JSON does not silently disable the field.
// There is no client-supplied "updated_by": the handler derives it from the
// Basic Auth identity that authenticated the request, so the audit trail
// reflects a verified credential rather than free text the caller typed.
type UpdateEnabledQueryFieldRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

// EnabledQueryFieldDTO is the HTTP representation of one whitelist entry.
type EnabledQueryFieldDTO struct {
	FieldCode string     `json:"field_code"`
	Enabled   bool       `json:"enabled"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	UpdatedBy string     `json:"updated_by"`
}

// EnabledQueryFieldListDTO wraps the full schema v1 whitelist view.
type EnabledQueryFieldListDTO struct {
	Items []EnabledQueryFieldDTO `json:"items"`
}

// ToInput maps the request to an update use-case command. updatedBy comes
// from the authenticated Basic Auth identity (middleware.AuthenticatedUserContextKey),
// never from the request body.
func (r UpdateEnabledQueryFieldRequest) ToInput(fieldCode, updatedBy string) contracts.UpdateEnabledQueryFieldInput {
	enabled := false
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return contracts.UpdateEnabledQueryFieldInput{
		FieldCode: fieldCode,
		Enabled:   enabled,
		UpdatedBy: updatedBy,
	}
}

// NewEnabledQueryFieldDTOFromOutput maps a use-case output to an HTTP DTO.
func NewEnabledQueryFieldDTOFromOutput(output *contracts.EnabledQueryFieldOutput) *EnabledQueryFieldDTO {
	if output == nil {
		return nil
	}
	return &EnabledQueryFieldDTO{
		FieldCode: output.FieldCode,
		Enabled:   output.Enabled,
		UpdatedAt: output.UpdatedAt,
		UpdatedBy: output.UpdatedBy,
	}
}

// NewEnabledQueryFieldListDTOFromOutputs maps use-case outputs to an HTTP list DTO.
func NewEnabledQueryFieldListDTOFromOutputs(outputs []*contracts.EnabledQueryFieldOutput) *EnabledQueryFieldListDTO {
	items := make([]EnabledQueryFieldDTO, 0, len(outputs))
	for _, output := range outputs {
		if mapped := NewEnabledQueryFieldDTOFromOutput(output); mapped != nil {
			items = append(items, *mapped)
		}
	}
	return &EnabledQueryFieldListDTO{Items: items}
}
