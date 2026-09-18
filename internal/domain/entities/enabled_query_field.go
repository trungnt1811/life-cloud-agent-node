package entities

import (
	"strings"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
)

// EnabledQueryField is one hospital-local whitelist entry for a schema field.
type EnabledQueryField struct {
	fieldCode string
	enabled   bool
	updatedAt time.Time
	updatedBy string
}

// EnabledQueryFieldRecord is a persistence snapshot used at repository boundaries.
type EnabledQueryFieldRecord struct {
	FieldCode string
	Enabled   bool
	UpdatedAt time.Time
	UpdatedBy string
}

// NewEnabledQueryField creates a whitelist entry with normalized fields.
func NewEnabledQueryField(fieldCode string, enabled bool, updatedBy string, now time.Time) *EnabledQueryField {
	return &EnabledQueryField{
		fieldCode: queryfields.NormalizeFieldCode(fieldCode),
		enabled:   enabled,
		updatedAt: ensureTimestamp(now),
		updatedBy: strings.TrimSpace(updatedBy),
	}
}

// NewEnabledQueryFieldFromRecord hydrates a whitelist entry from persistence
// data. Normalizes fieldCode the same way NewEnabledQueryField does, so a
// stray non-canonical row (hand-run SQL, a future import path) still
// hydrates to the same key ListQueryFields/GetByFieldCode look up by,
// instead of silently falling out of both.
func NewEnabledQueryFieldFromRecord(record EnabledQueryFieldRecord) *EnabledQueryField {
	fieldCode := queryfields.NormalizeFieldCode(record.FieldCode)
	if fieldCode == "" {
		return nil
	}
	return &EnabledQueryField{
		fieldCode: fieldCode,
		enabled:   record.Enabled,
		updatedAt: record.UpdatedAt,
		updatedBy: record.UpdatedBy,
	}
}

// Record returns a persistence snapshot.
func (e *EnabledQueryField) Record() EnabledQueryFieldRecord {
	if e == nil {
		return EnabledQueryFieldRecord{}
	}
	return EnabledQueryFieldRecord{
		FieldCode: e.fieldCode,
		Enabled:   e.enabled,
		UpdatedAt: e.updatedAt,
		UpdatedBy: e.updatedBy,
	}
}

// SetEnabled updates the enabled flag and audit fields.
func (e *EnabledQueryField) SetEnabled(enabled bool, updatedBy string, now time.Time) {
	if e == nil {
		return
	}
	e.enabled = enabled
	e.updatedBy = strings.TrimSpace(updatedBy)
	e.updatedAt = ensureTimestamp(now)
}

func (e *EnabledQueryField) FieldCode() string {
	if e == nil {
		return ""
	}
	return e.fieldCode
}

func (e *EnabledQueryField) Enabled() bool {
	if e == nil {
		return false
	}
	return e.enabled
}

func (e *EnabledQueryField) UpdatedAt() time.Time {
	if e == nil {
		return time.Time{}
	}
	return e.updatedAt
}

func (e *EnabledQueryField) UpdatedBy() string {
	if e == nil {
		return ""
	}
	return e.updatedBy
}
