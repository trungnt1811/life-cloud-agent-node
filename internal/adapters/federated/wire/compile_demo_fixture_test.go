package wire_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	federatedwire "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/wire"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestCompileDemoQueryTask_RoundTrip(t *testing.T) {
	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	require.NoError(t, federatedwire.ValidateQueryTaskStructureV1(task))
	require.NoError(t, federatedwire.ValidateQueryTaskVersionV1(task))

	require.Equal(t, uint32(1), task.GetQuerySchemaVersion())
	require.Equal(t, "2024-01-01", task.GetTimeRange().GetFrom())
	require.Equal(t, "2024-12-31", task.GetTimeRange().GetTo())
	require.Equal(t, nodev1.SpecimenPolicy_SPECIMEN_POLICY_LATEST_IN_RANGE, task.GetSpecimenPolicy())
	require.Len(t, task.GetConditions(), 2)
	require.Equal(t, "MCV", task.GetConditions()[0].GetFieldCode())
	require.Equal(t, "80", task.GetConditions()[0].GetValue().GetNumberValue())
	require.Equal(t, "MCH", task.GetConditions()[1].GetFieldCode())
	require.Equal(t, "27", task.GetConditions()[1].GetValue().GetNumberValue())
	require.Len(t, task.GetRequiredPanels(), 2)
	require.Equal(t, []string{"HB", "MCV", "MCH", "RBC"}, task.GetRequiredPanels()[0].GetFieldCodes())
	require.Equal(t, nodev1.ValueConstraint_VALUE_CONSTRAINT_EXACT, task.GetRequiredPanels()[0].GetValueConstraint())
	require.Equal(t, []string{"HBA0", "HBA2", "HBF"}, task.GetRequiredPanels()[1].GetFieldCodes())
	require.Equal(t, nodev1.ValueConstraint_VALUE_CONSTRAINT_ALLOW_CENSORED, task.GetRequiredPanels()[1].GetValueConstraint())

	payload, err := proto.Marshal(task)
	require.NoError(t, err)

	var decoded nodev1.QueryTask
	require.NoError(t, proto.Unmarshal(payload, &decoded))
	require.True(t, proto.Equal(task, &decoded))
}

func TestCompileDemoQueryTask_ExportedAPI(t *testing.T) {
	task := federatedwire.CompileDemoQueryTask(federatedwire.DemoQueryDefinition{
		DateStart:    "2024-01-01",
		DateEnd:      "2024-12-31",
		CBCRequired:  []string{"HB"},
		HPLCRequired: []string{"HBF"},
	})
	require.Equal(t, "2024-01-01", task.GetTimeRange().GetFrom())
	require.Equal(t, []string{"HB"}, task.GetRequiredPanels()[0].GetFieldCodes())
}

func TestQueryTaskV1_RejectsStructuralViolations(t *testing.T) {
	t.Run("non_empty_group_by", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.GroupBy = []string{"HB"}
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		requireRejectedInvalidQuery(t, err, "group_by")
	})

	t.Run("unspecified_specimen_policy", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.SpecimenPolicy = nodev1.SpecimenPolicy_SPECIMEN_POLICY_UNSPECIFIED
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		requireRejectedInvalidQuery(t, err, "specimen_policy")
	})

	t.Run("unspecified_operator", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.Conditions[0].Op = nodev1.ComparisonOperator_COMPARISON_OPERATOR_UNSPECIFIED
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		requireRejectedInvalidQuery(t, err, "op")
	})

	t.Run("unspecified_value_constraint", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.RequiredPanels[0].ValueConstraint = nodev1.ValueConstraint_VALUE_CONSTRAINT_UNSPECIFIED
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		requireRejectedInvalidQuery(t, err, "value_constraint")
	})

	t.Run("missing_time_range", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.TimeRange = nil
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		requireRejectedInvalidQuery(t, err, "time_range")
	})

	t.Run("missing_time_range_bound", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.TimeRange.To = ""
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		requireRejectedInvalidQuery(t, err, "time_range.to")
	})

	t.Run("whitespace_only_field_code", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.Conditions[0].FieldCode = "   "
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		requireRejectedInvalidQuery(t, err, "field_code is required")
	})

	t.Run("whitespace_only_time_range_from", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.TimeRange.From = "  "
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		requireRejectedInvalidQuery(t, err, "time_range.from")
	})

	t.Run("does_not_mutate_task", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.Conditions[0].FieldCode = "  mcv "
		task.RequiredPanels[0].FieldCodes[0] = " hb "
		task.TimeRange.From = " 2024-01-01 "
		before := proto.Clone(task)
		require.NoError(t, federatedwire.ValidateQueryTaskStructureV1(task))
		require.True(t, proto.Equal(before, task), "structural validation must not rewrite the task")

		// Also holds on failure: nothing half-normalized.
		task.RequiredPanels[1].ValueConstraint = nodev1.ValueConstraint_VALUE_CONSTRAINT_UNSPECIFIED
		before = proto.Clone(task)
		require.Error(t, federatedwire.ValidateQueryTaskStructureV1(task))
		require.True(t, proto.Equal(before, task))
	})
}

func TestQueryTaskV1_RejectsWhitelistViolations(t *testing.T) {
	t.Run("disabled_condition_field", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)

		repo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
		repo.EXPECT().ListAll(gomock.Any()).Return(enabledFieldsExcept("MCV"), nil)

		err = federatedwire.ValidateQueryTaskWhitelistV1(context.Background(), task, repo)
		requireRejectedInvalidQuery(t, err, "MCV")
	})

	t.Run("disabled_required_panel_field", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)

		repo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
		repo.EXPECT().ListAll(gomock.Any()).Return(enabledFieldsExcept("HBF"), nil)

		err = federatedwire.ValidateQueryTaskWhitelistV1(context.Background(), task, repo)
		requireRejectedInvalidQuery(t, err, "HBF")
	})

	t.Run("unknown_field_missing_from_store", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.Conditions[0].FieldCode = "NOT_A_FIELD"

		repo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
		repo.EXPECT().ListAll(gomock.Any()).Return(allEnabledFields(), nil)

		err = federatedwire.ValidateQueryTaskWhitelistV1(context.Background(), task, repo)
		requireRejectedInvalidQuery(t, err, "NOT_A_FIELD")
	})

	t.Run("colliding_rows_fail_closed_regardless_of_order", func(t *testing.T) {
		now := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
		for name, rows := range map[string][]*entities.EnabledQueryField{
			"enabled_last":  {entities.NewEnabledQueryField("MCV", false, "t", now), entities.NewEnabledQueryField("mcv", true, "t", now)},
			"disabled_last": {entities.NewEnabledQueryField("mcv", true, "t", now), entities.NewEnabledQueryField("MCV", false, "t", now)},
		} {
			t.Run(name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				task, err := federatedwire.CompileDemoQueryTaskFromFixture()
				require.NoError(t, err)
				repo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
				repo.EXPECT().ListAll(gomock.Any()).Return(append(enabledFieldsExcept("MCV"), rows...), nil)

				err = federatedwire.ValidateQueryTaskWhitelistV1(context.Background(), task, repo)
				requireRejectedInvalidQuery(t, err, "MCV")
			})
		}
	})

	t.Run("nil_repository_is_internal_error", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		err = federatedwire.ValidateQueryTaskWhitelistV1(context.Background(), task, nil)
		requireQueryStatus(t, err, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_ERROR, "repository")
	})

	t.Run("list_all_failure_is_internal_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)

		repo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
		repo.EXPECT().ListAll(gomock.Any()).Return(nil, errors.New("db down"))

		err = federatedwire.ValidateQueryTaskWhitelistV1(context.Background(), task, repo)
		requireQueryStatus(t, err, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_ERROR, "list enabled query fields")
		require.ErrorContains(t, err, "db down")
	})
}

func TestQueryTaskV1_RejectsSemanticViolations(t *testing.T) {
	t.Run("invalid_decimal_number_value", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.Conditions[0].Value = &nodev1.ConditionValue{
			Kind: &nodev1.ConditionValue_NumberValue{NumberValue: "1e2"},
		}
		err = federatedwire.ValidateQueryTaskSemanticV1(task)
		requireRejectedInvalidQuery(t, err, "number_value")
	})

	t.Run("empty_decimal_number_value", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.Conditions[0].Value = &nodev1.ConditionValue{
			Kind: &nodev1.ConditionValue_NumberValue{NumberValue: ""},
		}
		err = federatedwire.ValidateQueryTaskSemanticV1(task)
		requireRejectedInvalidQuery(t, err, "number_value")
	})

	t.Run("non_decimal_number_value_forms", func(t *testing.T) {
		for _, raw := range []string{
			"0x10", "0b101", "0o17", "1_000", "0x1p4", "+5", "-.5", "5.", ".5",
			" 80 ", "80 ", "1/2", "1E2", "NaN", "Inf", "--1", "1.2.3",
		} {
			task, err := federatedwire.CompileDemoQueryTaskFromFixture()
			require.NoError(t, err)
			task.Conditions[0].Value = &nodev1.ConditionValue{
				Kind: &nodev1.ConditionValue_NumberValue{NumberValue: raw},
			}
			err = federatedwire.ValidateQueryTaskSemanticV1(task)
			requireRejectedInvalidQuery(t, err, "number_value")
		}
	})

	t.Run("canonical_decimals_accepted", func(t *testing.T) {
		for _, raw := range []string{"0", "80", "79.5", "-1", "0.001", "007"} {
			task, err := federatedwire.CompileDemoQueryTaskFromFixture()
			require.NoError(t, err)
			task.Conditions[0].Value = &nodev1.ConditionValue{
				Kind: &nodev1.ConditionValue_NumberValue{NumberValue: raw},
			}
			require.NoError(t, federatedwire.ValidateQueryTaskSemanticV1(task), raw)
		}
	})

	t.Run("oversized_number_value", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.Conditions[0].Value = &nodev1.ConditionValue{
			Kind: &nodev1.ConditionValue_NumberValue{NumberValue: strings.Repeat("9", 33)},
		}
		err = federatedwire.ValidateQueryTaskSemanticV1(task)
		requireRejectedInvalidQuery(t, err, "longer than")
	})

	t.Run("padded_time_range_rejected", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.TimeRange.From = " 2024-01-01 "
		err = federatedwire.ValidateQueryTaskSemanticV1(task)
		requireRejectedInvalidQuery(t, err, "YYYY-MM-DD")
	})

	t.Run("string_value_for_measurement", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.Conditions[0].Value = &nodev1.ConditionValue{
			Kind: &nodev1.ConditionValue_StringValue{StringValue: "low"},
		}
		err = federatedwire.ValidateQueryTaskSemanticV1(task)
		requireRejectedInvalidQuery(t, err, "number_value")
	})

	t.Run("unknown_dictionary_field", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.Conditions[0].FieldCode = "NOT_A_FIELD"
		err = federatedwire.ValidateQueryTaskSemanticV1(task)
		requireRejectedInvalidQuery(t, err, "schema v1 dictionary")
	})

	t.Run("invalid_time_range_format", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.TimeRange.From = "2024-1-01"
		err = federatedwire.ValidateQueryTaskSemanticV1(task)
		requireRejectedInvalidQuery(t, err, "YYYY-MM-DD")
	})

	t.Run("time_range_from_after_to", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.TimeRange.From = "2024-12-31"
		task.TimeRange.To = "2024-01-01"
		err = federatedwire.ValidateQueryTaskSemanticV1(task)
		requireRejectedInvalidQuery(t, err, "time_range.from")
	})
}

func TestQueryTaskV1_RejectsUnsupportedVersion(t *testing.T) {
	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	task.QuerySchemaVersion = 2
	err = federatedwire.ValidateQueryTaskVersionV1(task)
	require.Error(t, err)
	var validationErr *federatedwire.QueryValidationError
	require.ErrorAs(t, err, &validationErr)
	require.Equal(t, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_UNSUPPORTED_VERSION, validationErr.Status)
	require.Contains(t, validationErr.Reason, "query_schema_version")
}

func TestValidateQueryTaskV1_LayerOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)

	repo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
	repo.EXPECT().ListAll(gomock.Any()).Return(allEnabledFields(), nil)

	require.NoError(t, federatedwire.ValidateQueryTaskV1(context.Background(), task, repo))

	t.Run("version_before_everything_and_skips_d5", func(t *testing.T) {
		bad, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		bad.QuerySchemaVersion = 99
		bad.GroupBy = []string{"HB"}
		bad.TimeRange.From = "not-a-date"
		// No ListAll expectation: an unsupported version must not read D5.
		err = federatedwire.ValidateQueryTaskV1(context.Background(), bad, mocks.NewMockEnabledQueryFieldRepository(ctrl))
		requireQueryStatus(t, err, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_UNSUPPORTED_VERSION, "query_schema_version")
	})

	t.Run("unsupported_version_survives_d5_outage", func(t *testing.T) {
		bad, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		bad.QuerySchemaVersion = 2
		downRepo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
		err = federatedwire.ValidateQueryTaskV1(context.Background(), bad, downRepo)
		requireQueryStatus(t, err, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_UNSUPPORTED_VERSION, "query_schema_version")
	})

	t.Run("structural_before_whitelist", func(t *testing.T) {
		bad, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		bad.GroupBy = []string{"HB"}
		// Repo must not be consulted when structure fails.
		err = federatedwire.ValidateQueryTaskV1(context.Background(), bad, mocks.NewMockEnabledQueryFieldRepository(ctrl))
		requireRejectedInvalidQuery(t, err, "group_by")
	})

	t.Run("whitelist_before_semantic", func(t *testing.T) {
		bad, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		bad.Conditions[0].Value = &nodev1.ConditionValue{
			Kind: &nodev1.ConditionValue_NumberValue{NumberValue: "not-a-decimal"},
		}
		disabledRepo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
		disabledRepo.EXPECT().ListAll(gomock.Any()).Return(enabledFieldsExcept("MCV"), nil)
		err = federatedwire.ValidateQueryTaskV1(context.Background(), bad, disabledRepo)
		requireRejectedInvalidQuery(t, err, "MCV")
	})

	t.Run("semantic_last", func(t *testing.T) {
		bad, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		bad.TimeRange.From = "not-a-date"
		okRepo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
		okRepo.EXPECT().ListAll(gomock.Any()).Return(allEnabledFields(), nil)
		err = federatedwire.ValidateQueryTaskV1(context.Background(), bad, okRepo)
		requireRejectedInvalidQuery(t, err, "YYYY-MM-DD")
	})

	t.Run("does_not_mutate_padded_but_valid_codes", func(t *testing.T) {
		padded, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		padded.Conditions[0].FieldCode = " mcv "
		before := proto.Clone(padded)
		okRepo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
		okRepo.EXPECT().ListAll(gomock.Any()).Return(allEnabledFields(), nil)
		require.NoError(t, federatedwire.ValidateQueryTaskV1(context.Background(), padded, okRepo))
		require.True(t, proto.Equal(before, padded))
	})
}

func TestFieldDictionary_CoversSchemaV1Codes(t *testing.T) {
	require.Len(t, queryfields.SchemaV1FieldCodes, 9)
	for _, code := range queryfields.SchemaV1FieldCodes {
		kind, ok := federatedwire.FieldKindV1(code)
		require.Truef(t, ok, "missing kind for %s", code)
		require.Equal(t, federatedwire.FieldKindMeasurement, kind)
	}
	_, ok := federatedwire.FieldKindV1("NOPE")
	require.False(t, ok)
}

func TestNodeControlConnect_ServiceRegistered(t *testing.T) {
	require.NotNil(t, nodev1.NodeControl_ServiceDesc.Streams)
	require.Len(t, nodev1.NodeControl_ServiceDesc.Streams, 1)
	require.Equal(t, "Connect", nodev1.NodeControl_ServiceDesc.Streams[0].StreamName)
	require.True(t, nodev1.NodeControl_ServiceDesc.Streams[0].ServerStreams)
	require.True(t, nodev1.NodeControl_ServiceDesc.Streams[0].ClientStreams)
}

func requireRejectedInvalidQuery(t *testing.T, err error, reasonFragment string) {
	t.Helper()
	requireQueryStatus(t, err, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_REJECTED_INVALID_QUERY, reasonFragment)
}

func requireQueryStatus(t *testing.T, err error, status nodev1.QueryResultStatus, reasonFragment string) {
	t.Helper()
	require.Error(t, err)
	var validationErr *federatedwire.QueryValidationError
	require.True(t, errors.As(err, &validationErr), "got %T: %v", err, err)
	require.Equal(t, status, validationErr.Status)
	require.Contains(t, validationErr.Reason, reasonFragment)
}

func allEnabledFields() []*entities.EnabledQueryField {
	now := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	out := make([]*entities.EnabledQueryField, 0, len(queryfields.SchemaV1FieldCodes))
	for _, code := range queryfields.SchemaV1FieldCodes {
		out = append(out, entities.NewEnabledQueryField(code, true, "test", now))
	}
	return out
}

func enabledFieldsExcept(disabled string) []*entities.EnabledQueryField {
	now := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	disabled = queryfields.NormalizeFieldCode(disabled)
	out := make([]*entities.EnabledQueryField, 0, len(queryfields.SchemaV1FieldCodes))
	for _, code := range queryfields.SchemaV1FieldCodes {
		out = append(out, entities.NewEnabledQueryField(code, code != disabled, "test", now))
	}
	return out
}
