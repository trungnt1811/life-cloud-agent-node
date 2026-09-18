// Package queryfields holds the schema v1 global field dictionary used by
// the local enabled_query_fields store (D5) and later whitelist validation.
package queryfields

import "strings"

// SchemaV1FieldCodes is the global maximum set of field_code values for
// query_schema_version = 1. Normative source:
// docs/product/query-field-dictionary.md.
var SchemaV1FieldCodes = []string{
	"HB",
	"MCV",
	"MCH",
	"RBC",
	"MCHC",
	"RDW",
	"HBA0",
	"HBA2",
	"HBF",
}

var schemaV1FieldCodeSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(SchemaV1FieldCodes))
	for _, code := range SchemaV1FieldCodes {
		set[code] = struct{}{}
	}
	return set
}()

// NormalizeFieldCode trims and uppercases a field code for dictionary lookup.
func NormalizeFieldCode(fieldCode string) string {
	return strings.ToUpper(strings.TrimSpace(fieldCode))
}

// IsSchemaV1FieldCode reports whether fieldCode is in the schema v1 global set.
func IsSchemaV1FieldCode(fieldCode string) bool {
	_, ok := schemaV1FieldCodeSet[NormalizeFieldCode(fieldCode)]
	return ok
}
