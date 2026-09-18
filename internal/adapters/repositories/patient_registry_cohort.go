package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// CountMatchingCohort counts patients whose LATEST_IN_RANGE specimen
// (collected_at DESC, then external_specimen_id DESC) satisfies every
// required panel and every condition. Conditions compare exact values only:
// a censored observation ('<0.1' stored as 0.1, censored=true) is a bound,
// not a measurement, so it never satisfies a condition.
func (r *patientRegistryRepository) CountMatchingCohort(
	ctx context.Context,
	criteria types.CohortCriteria,
) (uint64, error) {
	if criteria.From.IsZero() || criteria.To.IsZero() {
		return 0, errors.New("cohort time range is required")
	}
	if criteria.From.After(criteria.To) {
		return 0, errors.New("cohort time range from must be <= to")
	}

	sql, args, err := buildCountMatchingCohortSQL(criteria)
	if err != nil {
		r.logger.Error("Failed to build cohort query", logger.Err(err))
		return 0, err
	}

	var count int64
	if err := r.dbWithContext(ctx).Raw(sql, args...).Scan(&count).Error; err != nil {
		r.logger.Error("Failed to count matching cohort", logger.Err(err))
		return 0, err
	}
	return uint64(count), nil
}

func buildCountMatchingCohortSQL(criteria types.CohortCriteria) (string, []any, error) {
	var b strings.Builder
	args := make([]any, 0, 8+len(criteria.Conditions)*2)

	// Specimens in the inclusive calendar-date window; latest per patient by
	// collected_at then lexical external_specimen_id (decision 0004 /
	// life-cloud benchmark: ORDER BY day DESC, specimen DESC). COLLATE "C"
	// pins byte order; the column's default collation (en_US.UTF-8, ICU)
	// would rank 'S-a1' vs 'S-B1' differently and pick another "latest".
	b.WriteString(`
WITH ranked AS (
	SELECT id, patient_id,
		ROW_NUMBER() OVER (
			PARTITION BY patient_id
			ORDER BY collected_at DESC, external_specimen_id COLLATE "C" DESC
		) AS rn
	FROM specimens
	WHERE collected_at >= ?::date AND collected_at <= ?::date
),
latest AS (
	SELECT id AS specimen_id, patient_id
	FROM ranked
	WHERE rn = 1
)
SELECT COUNT(*)::bigint
FROM latest ls
WHERE TRUE`)
	args = append(args, criteria.From, criteria.To)

	for i, panel := range criteria.RequiredPanels {
		clause, panelArgs, err := requiredPanelSQL(i, panel)
		if err != nil {
			return "", nil, err
		}
		b.WriteString(clause)
		args = append(args, panelArgs...)
	}

	for i, condition := range criteria.Conditions {
		clause, condArgs, err := conditionSQL(i, condition)
		if err != nil {
			return "", nil, err
		}
		b.WriteString(clause)
		args = append(args, condArgs...)
	}

	return b.String(), args, nil
}

func requiredPanelSQL(index int, panel types.CohortRequiredPanel) (string, []any, error) {
	codes := make([]string, 0, len(panel.FieldCodes))
	seen := make(map[string]struct{}, len(panel.FieldCodes))
	for _, raw := range panel.FieldCodes {
		code := queryfields.NormalizeFieldCode(raw)
		if code == "" {
			return "", nil, fmt.Errorf("required_panels[%d] contains an empty field_code", index)
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		return "", nil, fmt.Errorf("required_panels[%d].field_codes must not be empty", index)
	}

	placeholders := make([]string, len(codes))
	args := make([]any, 0, len(codes)+1)
	for i, code := range codes {
		placeholders[i] = "?"
		args = append(args, code)
	}
	args = append(args, len(codes))

	censoredFilter := ""
	switch panel.Constraint {
	case types.PanelValueExact:
		censoredFilter = " AND o.censored = FALSE"
	case types.PanelValueAllowCensored:
		// presence only
	default:
		return "", nil, fmt.Errorf("required_panels[%d] has unsupported value constraint %q", index, panel.Constraint)
	}

	clause := fmt.Sprintf(`
AND (
	SELECT COUNT(*)::int
	FROM lab_observations o
	WHERE o.specimen_id = ls.specimen_id
		AND o.field_code IN (%s)%s
) = ?`, strings.Join(placeholders, ", "), censoredFilter)
	return clause, args, nil
}

func conditionSQL(index int, condition types.CohortCondition) (string, []any, error) {
	code := queryfields.NormalizeFieldCode(condition.FieldCode)
	if code == "" {
		return "", nil, fmt.Errorf("conditions[%d].field_code is required", index)
	}
	if strings.TrimSpace(condition.NumberValue) == "" {
		return "", nil, fmt.Errorf("conditions[%d].number_value is required", index)
	}
	op, err := sqlComparisonOperator(condition.Op)
	if err != nil {
		return "", nil, fmt.Errorf("conditions[%d]: %w", index, err)
	}

	clause := fmt.Sprintf(`
AND EXISTS (
	SELECT 1
	FROM lab_observations o
	WHERE o.specimen_id = ls.specimen_id
		AND o.field_code = ?
		AND o.censored = FALSE
		AND o.value::numeric %s ?::numeric
)`, op)
	return clause, []any{code, strings.TrimSpace(condition.NumberValue)}, nil
}

func sqlComparisonOperator(op types.ComparisonOp) (string, error) {
	switch op {
	case types.ComparisonOpEQ:
		return "=", nil
	case types.ComparisonOpNE:
		return "<>", nil
	case types.ComparisonOpLT:
		return "<", nil
	case types.ComparisonOpLTE:
		return "<=", nil
	case types.ComparisonOpGT:
		return ">", nil
	case types.ComparisonOpGTE:
		return ">=", nil
	default:
		return "", fmt.Errorf("unsupported comparison op %q", op)
	}
}
