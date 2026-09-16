package wire

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
)

const demoQuerySchemaVersion uint32 = 1

type demoQueryDefinition struct {
	DateStart    string   `json:"date_start"`
	DateEnd      string   `json:"date_end"`
	CBCRequired  []string `json:"cbc_required"`
	HPLCRequired []string `json:"hplc_required"`
	Rule         struct {
		MCVLtFL int `json:"MCV_lt_fL"`
		MCHLtPg int `json:"MCH_lt_pg"`
	} `json:"rule"`
}

// CompileDemoQueryTaskFromFixture reads the embedded demo fixture and returns
// the schema-v1 QueryTask equivalent (decision 0004 mapping table).
func CompileDemoQueryTaskFromFixture() (*nodev1.QueryTask, error) {
	fixturePath, err := demoFixturePath()
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(fixturePath)
	if err != nil {
		return nil, fmt.Errorf("read demo fixture: %w", err)
	}

	var fixture demoQueryDefinition
	if err := json.Unmarshal(content, &fixture); err != nil {
		return nil, fmt.Errorf("decode demo fixture: %w", err)
	}

	return compileDemoQueryTask(fixture), nil
}

// CompileDemoQueryTask builds the schema-v1 task from decoded fixture fields.
func CompileDemoQueryTask(fixture demoQueryDefinition) *nodev1.QueryTask {
	return compileDemoQueryTask(fixture)
}

func compileDemoQueryTask(fixture demoQueryDefinition) *nodev1.QueryTask {
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

// ValidateQueryTaskStructureV1 applies schema-v1 structural rules that protobuf
// decode alone does not enforce (decision 0004).
func ValidateQueryTaskStructureV1(task *nodev1.QueryTask) error {
	if task == nil {
		return fmt.Errorf("query task is nil")
	}
	if len(task.GetGroupBy()) > 0 {
		return fmt.Errorf("group_by is reserved in schema v1 and must be empty")
	}
	return nil
}

func demoFixturePath() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve fixture path")
	}
	return filepath.Join(filepath.Dir(currentFile), "testdata", "query_definition.json"), nil
}
