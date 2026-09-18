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

// schemaV1FieldKinds maps every schema-v1 field_code to its kind.
// Keep in sync with docs/product/query-field-dictionary.md and
// queryfields.SchemaV1FieldCodes.
var schemaV1FieldKinds = map[string]FieldKind{
	"HB":   FieldKindMeasurement,
	"MCV":  FieldKindMeasurement,
	"MCH":  FieldKindMeasurement,
	"RBC":  FieldKindMeasurement,
	"MCHC": FieldKindMeasurement,
	"RDW":  FieldKindMeasurement,
	"HBA0": FieldKindMeasurement,
	"HBA2": FieldKindMeasurement,
	"HBF":  FieldKindMeasurement,
}

// FieldKindV1 returns the schema-v1 kind for fieldCode, if known.
func FieldKindV1(fieldCode string) (FieldKind, bool) {
	kind, ok := schemaV1FieldKinds[queryfields.NormalizeFieldCode(fieldCode)]
	return kind, ok
}
