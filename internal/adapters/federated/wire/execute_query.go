package wire

import (
	"context"
	"errors"
	"fmt"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
)

// SuppressMatchingCount applies process 3.6 small-cell suppression.
// When 0 < raw < threshold, the visible count is zero and suppressed=true.
// It fails closed: threshold 0 means "not configured" and uses
// constants.DefaultSuppressionThreshold, never "disabled" - otherwise a
// mis-wired caller would leak raw counts of 1-4. A threshold of 1 hides
// nothing.
func SuppressMatchingCount(raw, threshold uint64) (matchingCount uint64, suppressed bool) {
	if threshold == 0 {
		threshold = constants.DefaultSuppressionThreshold
	}
	if raw > 0 && raw < threshold {
		return 0, true
	}
	return raw, false
}

// ExecuteQueryTaskV1 runs process 3.5 then 3.6 against a schema-v1
// QueryTask. It re-runs the pure validation layers (version, structural,
// semantic) itself so a mis-ordered caller cannot get an OK count for a v2
// task, a group_by query, or a non-decimal value, and maps those failures to
// their QueryResult status. The whitelist layer needs D5, so the caller must
// still run ValidateQueryTaskV1 first (Phase 7).
//
// A canceled or expired ctx is returned as a Go error, not a terminal ERROR
// result, so the caller can tell an interrupted job (resumable, Phase 6)
// from a failed one.
func ExecuteQueryTaskV1(
	ctx context.Context,
	task *nodev1.QueryTask,
	counter interfaces.CohortCountUseCase,
	suppressionThreshold uint64,
) (*nodev1.QueryResult, error) {
	if task == nil {
		return nil, fmt.Errorf("query task is nil")
	}
	if counter == nil {
		return nil, fmt.Errorf("cohort count use case is not configured")
	}

	for _, validate := range []func(*nodev1.QueryTask) error{
		ValidateQueryTaskVersionV1,
		ValidateQueryTaskStructureV1,
		ValidateQueryTaskSemanticV1,
	} {
		if err := validate(task); err != nil {
			if validationErr, ok := AsQueryValidationError(err); ok {
				return &nodev1.QueryResult{
					JobId:  task.GetJobId(),
					Status: validationErr.Status,
					Reason: validationErr.Reason,
				}, nil
			}
			return nil, err
		}
	}

	criteria, err := CohortCriteriaFromQueryTaskV1(task)
	if err != nil {
		return &nodev1.QueryResult{
			JobId:  task.GetJobId(),
			Status: nodev1.QueryResultStatus_QUERY_RESULT_STATUS_ERROR,
			Reason: err.Error(),
		}, nil
	}

	rawCount, err := counter.CountMatchingCohort(ctx, task.GetJobId(), criteria)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return &nodev1.QueryResult{
			JobId:  task.GetJobId(),
			Status: nodev1.QueryResultStatus_QUERY_RESULT_STATUS_ERROR,
			Reason: "query execution failed",
		}, nil
	}

	matching, suppressed := SuppressMatchingCount(rawCount, suppressionThreshold)
	return &nodev1.QueryResult{
		JobId:         task.GetJobId(),
		Status:        nodev1.QueryResultStatus_QUERY_RESULT_STATUS_OK,
		MatchingCount: matching,
		Suppressed:    suppressed,
	}, nil
}

// CohortCriteriaFromQueryTaskV1 maps a schema-v1 wire task to domain criteria.
func CohortCriteriaFromQueryTaskV1(task *nodev1.QueryTask) (types.CohortCriteria, error) {
	if task == nil {
		return types.CohortCriteria{}, fmt.Errorf("query task is nil")
	}
	tr := task.GetTimeRange()
	if tr == nil {
		return types.CohortCriteria{}, fmt.Errorf("time_range is required")
	}
	from, err := parseSchemaV1Date(tr.GetFrom(), "time_range.from")
	if err != nil {
		return types.CohortCriteria{}, err
	}
	to, err := parseSchemaV1Date(tr.GetTo(), "time_range.to")
	if err != nil {
		return types.CohortCriteria{}, err
	}
	if from.After(to) {
		return types.CohortCriteria{}, fmt.Errorf("time_range.from must be less than or equal to time_range.to")
	}

	conditions := make([]types.CohortCondition, 0, len(task.GetConditions()))
	for i, condition := range task.GetConditions() {
		mapped, err := mapCondition(i, condition)
		if err != nil {
			return types.CohortCriteria{}, err
		}
		conditions = append(conditions, mapped)
	}

	panels := make([]types.CohortRequiredPanel, 0, len(task.GetRequiredPanels()))
	for i, panel := range task.GetRequiredPanels() {
		mapped, err := mapRequiredPanel(i, panel)
		if err != nil {
			return types.CohortCriteria{}, err
		}
		panels = append(panels, mapped)
	}

	return types.CohortCriteria{
		From:           from,
		To:             to,
		Conditions:     conditions,
		RequiredPanels: panels,
	}, nil
}

func mapCondition(index int, condition *nodev1.QueryCondition) (types.CohortCondition, error) {
	if condition == nil {
		return types.CohortCondition{}, fmt.Errorf("conditions[%d] is nil", index)
	}
	code := queryfields.NormalizeFieldCode(condition.GetFieldCode())
	if code == "" {
		return types.CohortCondition{}, fmt.Errorf("conditions[%d].field_code is required", index)
	}
	op, err := mapComparisonOp(condition.GetOp())
	if err != nil {
		return types.CohortCondition{}, fmt.Errorf("conditions[%d]: %w", index, err)
	}
	value := condition.GetValue()
	if value == nil {
		return types.CohortCondition{}, fmt.Errorf("conditions[%d].value is required", index)
	}
	switch value.GetKind().(type) {
	case *nodev1.ConditionValue_NumberValue:
		number := value.GetNumberValue()
		if number == "" {
			return types.CohortCondition{}, fmt.Errorf("conditions[%d].value.number_value is required", index)
		}
		return types.CohortCondition{
			FieldCode:   code,
			Op:          op,
			NumberValue: number,
		}, nil
	default:
		return types.CohortCondition{}, fmt.Errorf("conditions[%d].value must be number_value", index)
	}
}

func mapComparisonOp(op nodev1.ComparisonOperator) (types.ComparisonOp, error) {
	switch op {
	case nodev1.ComparisonOperator_COMPARISON_OPERATOR_EQ:
		return types.ComparisonOpEQ, nil
	case nodev1.ComparisonOperator_COMPARISON_OPERATOR_NE:
		return types.ComparisonOpNE, nil
	case nodev1.ComparisonOperator_COMPARISON_OPERATOR_LT:
		return types.ComparisonOpLT, nil
	case nodev1.ComparisonOperator_COMPARISON_OPERATOR_LTE:
		return types.ComparisonOpLTE, nil
	case nodev1.ComparisonOperator_COMPARISON_OPERATOR_GT:
		return types.ComparisonOpGT, nil
	case nodev1.ComparisonOperator_COMPARISON_OPERATOR_GTE:
		return types.ComparisonOpGTE, nil
	default:
		return "", fmt.Errorf("unsupported comparison op")
	}
}

func mapRequiredPanel(index int, panel *nodev1.RequiredPanel) (types.CohortRequiredPanel, error) {
	if panel == nil {
		return types.CohortRequiredPanel{}, fmt.Errorf("required_panels[%d] is nil", index)
	}
	constraint, err := mapValueConstraint(panel.GetValueConstraint())
	if err != nil {
		return types.CohortRequiredPanel{}, fmt.Errorf("required_panels[%d]: %w", index, err)
	}
	codes := make([]string, 0, len(panel.GetFieldCodes()))
	for j, raw := range panel.GetFieldCodes() {
		code := queryfields.NormalizeFieldCode(raw)
		if code == "" {
			return types.CohortRequiredPanel{}, fmt.Errorf(
				"required_panels[%d].field_codes[%d] is required", index, j,
			)
		}
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		return types.CohortRequiredPanel{}, fmt.Errorf("required_panels[%d].field_codes must not be empty", index)
	}
	return types.CohortRequiredPanel{
		FieldCodes: codes,
		Constraint: constraint,
	}, nil
}

func mapValueConstraint(constraint nodev1.ValueConstraint) (types.PanelValueConstraint, error) {
	switch constraint {
	case nodev1.ValueConstraint_VALUE_CONSTRAINT_EXACT:
		return types.PanelValueExact, nil
	case nodev1.ValueConstraint_VALUE_CONSTRAINT_ALLOW_CENSORED:
		return types.PanelValueAllowCensored, nil
	default:
		return "", fmt.Errorf("unsupported value constraint")
	}
}
