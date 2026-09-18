package wire

import (
	"fmt"
	"regexp"
	"time"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
)

const schemaV1DateLayout = "2006-01-02"

// maxDecimalLen bounds number_value before it reaches big.Rat, so a peer
// cannot make layer 3 burn CPU on a multi-megabyte numeric string. Lab
// values in the v1 dictionary need a handful of digits.
const maxDecimalLen = 32

// exactDecimalPattern is the canonical plain-decimal form: optional minus,
// digits, optional fraction. No '+', hex/binary/octal prefixes, '_'
// separators, exponents, or padding - what big.Rat.SetString alone accepts
// is far broader than the wire contract's "decimal string".
var exactDecimalPattern = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// ValidateQueryTaskSemanticV1 is layer 3: operators match field kinds,
// condition values are exact decimals, and time_range bounds are valid
// ordered YYYY-MM-DD calendar dates.
func ValidateQueryTaskSemanticV1(task *nodev1.QueryTask) error {
	if err := requireTask(task); err != nil {
		return err
	}

	if err := validateTimeRangeSemantic(task.GetTimeRange()); err != nil {
		return err
	}

	for i, condition := range task.GetConditions() {
		if condition == nil {
			continue
		}
		if err := validateConditionSemantic(i, condition); err != nil {
			return err
		}
	}

	for i, panel := range task.GetRequiredPanels() {
		if panel == nil {
			continue
		}
		for j, rawCode := range panel.GetFieldCodes() {
			if _, ok := FieldKindV1(rawCode); !ok {
				return rejectedInvalidQuery(fmt.Sprintf(
					"required_panels[%d].field_codes[%d] %q is not in schema v1 dictionary",
					i, j, rawCode,
				))
			}
		}
	}

	return nil
}

func validateTimeRangeSemantic(tr *nodev1.DateRange) error {
	if tr == nil {
		return rejectedInvalidQuery("time_range is required")
	}
	from, err := parseSchemaV1Date(tr.GetFrom(), "time_range.from")
	if err != nil {
		return err
	}
	to, err := parseSchemaV1Date(tr.GetTo(), "time_range.to")
	if err != nil {
		return err
	}
	if from.After(to) {
		return rejectedInvalidQuery("time_range.from must be less than or equal to time_range.to")
	}
	return nil
}

// parseSchemaV1Date rejects padded input rather than trimming it: the task is
// never rewritten, so accepting " 2024-01-01 " here would hand the executor a
// string that fails to parse.
func parseSchemaV1Date(raw, field string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, rejectedInvalidQuery(field + " is required")
	}
	parsed, err := time.Parse(schemaV1DateLayout, raw)
	if err != nil || parsed.Format(schemaV1DateLayout) != raw {
		return time.Time{}, rejectedInvalidQuery(field + " must be a valid YYYY-MM-DD calendar date")
	}
	return parsed, nil
}

func validateConditionSemantic(index int, condition *nodev1.QueryCondition) error {
	code := queryfields.NormalizeFieldCode(condition.GetFieldCode())
	kind, ok := FieldKindV1(code)
	if !ok {
		return rejectedInvalidQuery(fmt.Sprintf(
			"conditions[%d].field_code %q is not in schema v1 dictionary",
			index, condition.GetFieldCode(),
		))
	}

	value := condition.GetValue()
	if value == nil || value.GetKind() == nil {
		return rejectedInvalidQuery(formatIndex("conditions", index, "value is required"))
	}

	switch kind {
	case FieldKindMeasurement:
		return validateMeasurementCondition(index, condition.GetOp(), value)
	default:
		return rejectedInvalidQuery(fmt.Sprintf(
			"conditions[%d].field_code %q has unsupported field kind %q",
			index, condition.GetFieldCode(), kind,
		))
	}
}

func validateMeasurementCondition(index int, op nodev1.ComparisonOperator, value *nodev1.ConditionValue) error {
	switch op {
	case nodev1.ComparisonOperator_COMPARISON_OPERATOR_EQ,
		nodev1.ComparisonOperator_COMPARISON_OPERATOR_NE,
		nodev1.ComparisonOperator_COMPARISON_OPERATOR_LT,
		nodev1.ComparisonOperator_COMPARISON_OPERATOR_LTE,
		nodev1.ComparisonOperator_COMPARISON_OPERATOR_GT,
		nodev1.ComparisonOperator_COMPARISON_OPERATOR_GTE:
		// All six comparison ops are eligible for v1 numeric measurements.
	default:
		return rejectedInvalidQuery(formatIndex("conditions", index, "op is not eligible for measurement fields"))
	}

	switch value.GetKind().(type) {
	case *nodev1.ConditionValue_NumberValue:
		if err := parseExactDecimalString(value.GetNumberValue()); err != nil {
			return rejectedInvalidQuery(fmt.Sprintf(
				"conditions[%d].value.number_value is not an exact decimal: %v",
				index, err,
			))
		}
		return nil
	case *nodev1.ConditionValue_StringValue:
		return rejectedInvalidQuery(formatIndex(
			"conditions", index, "value must be number_value for measurement fields",
		))
	default:
		return rejectedInvalidQuery(formatIndex("conditions", index, "value kind is unsupported"))
	}
}

// parseExactDecimalString accepts only canonical, bounded, unrounded decimal
// strings (no binary float). The task is never rewritten, so padding is
// rejected rather than trimmed.
func parseExactDecimalString(raw string) error {
	if raw == "" {
		return fmt.Errorf("empty")
	}
	if len(raw) > maxDecimalLen {
		return fmt.Errorf("longer than %d characters", maxDecimalLen)
	}
	if !exactDecimalPattern.MatchString(raw) {
		return fmt.Errorf("invalid decimal %q", raw)
	}
	return nil
}
