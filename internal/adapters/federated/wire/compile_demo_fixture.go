package wire

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
)

const demoQuerySchemaVersion uint32 = 1

//go:embed testdata/query_definition.json
var demoQueryDefinitionJSON []byte

// DemoQueryDefinition is the engineering cohort fixture shape from
// life-cloud federated_mvp/benchmark/query_definition.json.
type DemoQueryDefinition struct {
	DateStart    string   `json:"date_start"`
	DateEnd      string   `json:"date_end"`
	CBCRequired  []string `json:"cbc_required"`
	HPLCRequired []string `json:"hplc_required"`
	Rule         struct {
		MCVLtFL int `json:"MCV_lt_fL"`
		MCHLtPg int `json:"MCH_lt_pg"`
	} `json:"rule"`
}

// CompileDemoQueryTaskFromFixture decodes the embedded demo fixture and
// returns the schema-v1 QueryTask equivalent (decision 0004 mapping table).
func CompileDemoQueryTaskFromFixture() (*nodev1.QueryTask, error) {
	var fixture DemoQueryDefinition
	if err := json.Unmarshal(demoQueryDefinitionJSON, &fixture); err != nil {
		return nil, fmt.Errorf("decode demo fixture: %w", err)
	}
	return CompileDemoQueryTask(fixture), nil
}

// CompileDemoQueryTask builds the schema-v1 task from decoded fixture fields.
func CompileDemoQueryTask(fixture DemoQueryDefinition) *nodev1.QueryTask {
	return &nodev1.QueryTask{
		JobId:              "demo-cohort",
		QuerySchemaVersion: demoQuerySchemaVersion,
		TimeRange: &nodev1.DateRange{
			From: fixture.DateStart,
			To:   fixture.DateEnd,
		},
		SpecimenPolicy: nodev1.SpecimenPolicy_SPECIMEN_POLICY_LATEST_IN_RANGE,
		Conditions: []*nodev1.QueryCondition{
			{
				FieldCode: "MCV",
				Op:        nodev1.ComparisonOperator_COMPARISON_OPERATOR_LT,
				Value: &nodev1.ConditionValue{
					Kind: &nodev1.ConditionValue_NumberValue{
						NumberValue: strconv.Itoa(fixture.Rule.MCVLtFL),
					},
				},
			},
			{
				FieldCode: "MCH",
				Op:        nodev1.ComparisonOperator_COMPARISON_OPERATOR_LT,
				Value: &nodev1.ConditionValue{
					Kind: &nodev1.ConditionValue_NumberValue{
						NumberValue: strconv.Itoa(fixture.Rule.MCHLtPg),
					},
				},
			},
		},
		RequiredPanels: []*nodev1.RequiredPanel{
			{
				FieldCodes:      append([]string(nil), fixture.CBCRequired...),
				ValueConstraint: nodev1.ValueConstraint_VALUE_CONSTRAINT_EXACT,
			},
			{
				FieldCodes:      append([]string(nil), fixture.HPLCRequired...),
				ValueConstraint: nodev1.ValueConstraint_VALUE_CONSTRAINT_ALLOW_CENSORED,
			},
		},
	}
}

// ValidateQueryTaskStructureV1 applies schema-v1 wire rules that protobuf
// decode alone does not enforce (decision 0004): required enums, a required
// time_range, and reserved group_by. Schema version is layer 4
// (ValidateQueryTaskVersionV1). Whitelist and semantic checks are layers 2–3.
//
// Canonicalizes in place: trims time_range bounds and normalizes field_code
// values so later layers see the same keys D5 / the dictionary use.
func ValidateQueryTaskStructureV1(task *nodev1.QueryTask) error {
	if err := requireTask(task); err != nil {
		return err
	}
	if task.GetSpecimenPolicy() != nodev1.SpecimenPolicy_SPECIMEN_POLICY_LATEST_IN_RANGE {
		return rejectedInvalidQuery("specimen_policy must be LATEST_IN_RANGE for schema v1")
	}
	if task.GetTimeRange() == nil {
		return rejectedInvalidQuery("time_range is required")
	}
	task.TimeRange.From = strings.TrimSpace(task.TimeRange.GetFrom())
	task.TimeRange.To = strings.TrimSpace(task.TimeRange.GetTo())
	if task.TimeRange.From == "" {
		return rejectedInvalidQuery("time_range.from is required")
	}
	if task.TimeRange.To == "" {
		return rejectedInvalidQuery("time_range.to is required")
	}
	if len(task.GetGroupBy()) > 0 {
		return rejectedInvalidQuery("group_by is reserved in schema v1 and must be empty")
	}
	for i, condition := range task.GetConditions() {
		if condition == nil {
			return rejectedInvalidQuery(fmt.Sprintf("conditions[%d] is nil", i))
		}
		condition.FieldCode = queryfields.NormalizeFieldCode(condition.GetFieldCode())
		if condition.FieldCode == "" {
			return rejectedInvalidQuery(fmt.Sprintf("conditions[%d].field_code is required", i))
		}
		if condition.GetOp() == nodev1.ComparisonOperator_COMPARISON_OPERATOR_UNSPECIFIED {
			return rejectedInvalidQuery(fmt.Sprintf("conditions[%d].op must be set", i))
		}
		if condition.GetValue() == nil || condition.GetValue().GetKind() == nil {
			return rejectedInvalidQuery(fmt.Sprintf("conditions[%d].value is required", i))
		}
	}
	for i, panel := range task.GetRequiredPanels() {
		if panel == nil {
			return rejectedInvalidQuery(fmt.Sprintf("required_panels[%d] is nil", i))
		}
		if len(panel.GetFieldCodes()) == 0 {
			return rejectedInvalidQuery(fmt.Sprintf("required_panels[%d].field_codes must not be empty", i))
		}
		for j, rawCode := range panel.FieldCodes {
			normalized := queryfields.NormalizeFieldCode(rawCode)
			if normalized == "" {
				return rejectedInvalidQuery(fmt.Sprintf(
					"required_panels[%d].field_codes[%d] is required", i, j,
				))
			}
			panel.FieldCodes[j] = normalized
		}
		if panel.GetValueConstraint() == nodev1.ValueConstraint_VALUE_CONSTRAINT_UNSPECIFIED {
			return rejectedInvalidQuery(fmt.Sprintf("required_panels[%d].value_constraint must be set", i))
		}
	}
	return nil
}
