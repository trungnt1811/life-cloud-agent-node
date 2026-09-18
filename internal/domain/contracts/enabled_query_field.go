package contracts

import "time"

// UpdateEnabledQueryFieldInput is the use-case command for toggling a field.
type UpdateEnabledQueryFieldInput struct {
	FieldCode string
	Enabled   bool
	UpdatedBy string
}

// EnabledQueryFieldOutput is the read model returned by enabled-query-field use cases.
// UpdatedAt is nil when the field has never been persisted on this node.
type EnabledQueryFieldOutput struct {
	FieldCode string
	Enabled   bool
	UpdatedAt *time.Time
	UpdatedBy string
}
