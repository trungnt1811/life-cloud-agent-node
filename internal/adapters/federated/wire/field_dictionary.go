package wire

import (
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
)

// FieldKind is the schema-v1 field kind used by semantic validation.
// Normative source: docs/product/query-field-dictionary.md.
type FieldKind string

const (
	// FieldKindMeasurement is a numeric lab measurement (v1: all 9 codes).
	FieldKindMeasurement FieldKind = "measurement"
)

// schemaV1FieldKinds is derived from queryfields.SchemaV1FieldCodes so the
// kind map cannot drift from the domain dictionary / migration seed.
// For schema v1 every listed code is a numeric measurement (query-field-dictionary.md).
var schemaV1FieldKinds = func() map[string]FieldKind {
	kinds := make(map[string]FieldKind, len(queryfields.SchemaV1FieldCodes))
	for _, code := range queryfields.SchemaV1FieldCodes {
		kinds[code] = FieldKindMeasurement
	}
	return kinds
}()

// FieldKindV1 returns the schema-v1 kind for fieldCode, if known.
func FieldKindV1(fieldCode string) (FieldKind, bool) {
	kind, ok := schemaV1FieldKinds[queryfields.NormalizeFieldCode(fieldCode)]
	return kind, ok
}
