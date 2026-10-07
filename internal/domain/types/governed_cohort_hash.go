package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
)

const GovernedCohortProfile = "governed-cohort/v1"

var governedDecimalPattern = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// GovernedDefinitionHash is separate from the legacy checkpoint fingerprint.
// It only identifies validated criteria; it never authorizes execution/release.
func (c CohortCriteria) GovernedDefinitionHash() (string, error) {
	if !governedCalendarDate(c.From) || !governedCalendarDate(c.To) || c.From.After(c.To) || len(c.Conditions) > 32 || len(c.RequiredPanels) > 16 {
		return "", errors.New("invalid governed cohort criteria")
	}
	conditions, err := governedConditions(c.Conditions)
	if err != nil {
		return "", err
	}
	panels, err := governedPanels(c.RequiredPanels)
	if err != nil {
		return "", err
	}
	params := map[string]any{
		"query_schema_version": 1,
		"date_from":            c.From.Format("2006-01-02"), "date_to": c.To.Format("2006-01-02"),
		"specimen_policy": "LATEST_IN_RANGE", "group_by": []string{},
		"conditions": conditions, "required_panels": panels,
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(map[string]any{"job_type": "COUNT_MATCHING_COHORT", "job_version": "v1", "params": params}); err != nil {
		return "", err
	}
	canonical := "cohort-definition/v1\n" + strings.TrimSuffix(buffer.String(), "\n")
	digest := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func governedCalendarDate(value time.Time) bool {
	parsed, err := time.Parse("2006-01-02", value.Format("2006-01-02"))
	return err == nil && parsed.Equal(value)
}

func governedConditions(items []CohortCondition) ([]map[string]any, error) {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		code := queryfields.NormalizeFieldCode(item.FieldCode)
		op := strings.ToUpper(strings.TrimSpace(string(item.Op)))
		if !queryfields.IsSchemaV1FieldCode(code) || len(item.NumberValue) > 32 || !governedDecimalPattern.MatchString(item.NumberValue) {
			return nil, errors.New("invalid governed cohort condition")
		}
		switch ComparisonOp(op) {
		case ComparisonOpEQ, ComparisonOpNE, ComparisonOpLT, ComparisonOpLTE, ComparisonOpGT, ComparisonOpGTE:
		default:
			return nil, errors.New("invalid governed cohort operator")
		}
		result = append(result, map[string]any{"field_code": code, "op": op, "value": governedDecimal(item.NumberValue)})
	}
	return governedObjectSet(result), nil
}

func governedPanels(items []CohortRequiredPanel) ([]map[string]any, error) {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		constraint := strings.ToUpper(strings.TrimSpace(string(item.Constraint)))
		if len(item.FieldCodes) < 1 || len(item.FieldCodes) > 32 || (constraint != string(PanelValueExact) && constraint != string(PanelValueAllowCensored)) {
			return nil, errors.New("invalid governed required panel")
		}
		codes := make([]string, 0, len(item.FieldCodes))
		for _, raw := range item.FieldCodes {
			code := queryfields.NormalizeFieldCode(raw)
			if !queryfields.IsSchemaV1FieldCode(code) {
				return nil, errors.New("invalid governed panel field")
			}
			codes = append(codes, code)
		}
		slices.Sort(codes)
		result = append(result, map[string]any{"field_codes": slices.Compact(codes), "value_constraint": constraint})
	}
	return governedObjectSet(result), nil
}

func governedDecimal(value string) string {
	negative := strings.HasPrefix(value, "-")
	whole, fraction, _ := strings.Cut(strings.TrimPrefix(value, "-"), ".")
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	fraction = strings.TrimRight(fraction, "0")
	if fraction != "" {
		whole += "." + fraction
	}
	if negative && whole != "0" {
		whole = "-" + whole
	}
	return whole
}

func governedObjectSet(items []map[string]any) []map[string]any {
	byJSON := make(map[string]map[string]any, len(items))
	keys := make([]string, 0, len(items))
	for _, item := range items {
		encoded, _ := json.Marshal(item)
		key := string(encoded)
		if _, exists := byJSON[key]; !exists {
			keys = append(keys, key)
			byJSON[key] = item
		}
	}
	slices.Sort(keys)
	result := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		result = append(result, byJSON[key])
	}
	return result
}
