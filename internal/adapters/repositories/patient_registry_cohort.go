package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

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
	if err := validateCohortWindow(criteria); err != nil {
		return 0, err
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

// CountMatchingCohortChunk is the resumable form of CountMatchingCohort
// (decision 0005): the same predicate over the next `limit` candidate patients
// after afterPatientID.
func (r *patientRegistryRepository) CountMatchingCohortChunk(
	ctx context.Context,
	criteria types.CohortCriteria,
	afterPatientID uuid.UUID,
	limit int,
) (types.CohortChunkResult, error) {
	if err := validateCohortWindow(criteria); err != nil {
		return types.CohortChunkResult{}, err
	}
	if limit < 1 {
		return types.CohortChunkResult{}, errors.New("cohort chunk limit must be >= 1")
	}

	sql, args, err := buildCountMatchingCohortChunkSQL(criteria, afterPatientID, limit)
	if err != nil {
		r.logger.Error("Failed to build cohort chunk query", logger.Err(err))
		return types.CohortChunkResult{}, err
	}

	var row struct {
		MatchingCount   int64
		PatientsScanned int64
		LastPatientID   *uuid.UUID
	}
	if err := r.dbWithContext(ctx).Raw(sql, args...).Scan(&row).Error; err != nil {
		r.logger.Error("Failed to count matching cohort chunk", logger.Err(err))
		return types.CohortChunkResult{}, err
	}

	result := types.CohortChunkResult{
		MatchingCount:   uint64(row.MatchingCount),
		PatientsScanned: int(row.PatientsScanned),
	}
	if row.LastPatientID != nil {
		result.LastPatientID = *row.LastPatientID
	}
	return result, nil
}

func validateCohortWindow(criteria types.CohortCriteria) error {
	if criteria.From.IsZero() || criteria.To.IsZero() {
		return errors.New("cohort time range is required")
	}
	if criteria.From.After(criteria.To) {
		return errors.New("cohort time range from must be <= to")
	}
	return nil
}

// latestSpecimenPartitionSQL is the LATEST_IN_RANGE ranking shared by the
// chunked and unchunked queries. Ties on collected_at go to the byte-lexically
// greatest external_specimen_id (decision 0004 / life-cloud benchmark: ORDER
// BY day DESC, specimen DESC). COLLATE "C" pins byte order; the column's
// default collation (en_US.UTF-8, ICU) would rank 'S-a1' vs 'S-B1'
// differently and pick another "latest".
const latestSpecimenPartitionSQL = `ROW_NUMBER() OVER (
			PARTITION BY patient_id
			ORDER BY collected_at DESC, external_specimen_id COLLATE "C" DESC
		) AS rn`

// cohortFilterSQL renders the required-panel and condition predicates over a
// `latest ls` row, each starting with AND.
func cohortFilterSQL(criteria types.CohortCriteria) (string, []any, error) {
	var b strings.Builder
	args := make([]any, 0, len(criteria.Conditions)*2+len(criteria.RequiredPanels)*4)

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

func buildCountMatchingCohortSQL(criteria types.CohortCriteria) (string, []any, error) {
	filter, filterArgs, err := cohortFilterSQL(criteria)
	if err != nil {
		return "", nil, err
	}

	sql := `
WITH ranked AS (
	SELECT id, patient_id,
		` + latestSpecimenPartitionSQL + `
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
WHERE TRUE` + filter
	args := append([]any{criteria.From, criteria.To}, filterArgs...)
	return sql, args, nil
}

// buildCountMatchingCohortChunkSQL counts matches among the next `limit`
// candidate patients after afterPatientID (uuid.Nil = from the start). The
// candidate set is chosen first, then the latest in-range specimen is looked
// up for each patient. Every patient is judged against the same specimen as
// the unchunked query, so a chunked walk sums to the unchunked count.
func buildCountMatchingCohortChunkSQL(
	criteria types.CohortCriteria,
	afterPatientID uuid.UUID,
	limit int,
) (string, []any, error) {
	filter, filterArgs, err := cohortFilterSQL(criteria)
	if err != nil {
		return "", nil, err
	}

	args := []any{criteria.From, criteria.To}
	after := ""
	if afterPatientID != uuid.Nil {
		after = " AND patient_id > ?::uuid"
		args = append(args, afterPatientID.String())
	}
	args = append(args, limit, criteria.From, criteria.To)

	sql := `
WITH chunk AS (
	SELECT DISTINCT patient_id
	FROM specimens
	WHERE collected_at >= ?::date AND collected_at <= ?::date` + after + `
	ORDER BY patient_id
	LIMIT ?
),
latest AS (
	SELECT s.id AS specimen_id, c.patient_id
	FROM chunk c
	CROSS JOIN LATERAL (
		SELECT id
		FROM specimens s
		WHERE s.patient_id = c.patient_id
			AND s.collected_at >= ?::date AND s.collected_at <= ?::date
		ORDER BY s.collected_at DESC, s.external_specimen_id COLLATE "C" DESC
		LIMIT 1
	) s
)
SELECT
	(SELECT COUNT(*) FROM latest ls WHERE TRUE` + filter + `)::bigint AS matching_count,
	(SELECT COUNT(*) FROM chunk)::bigint AS patients_scanned,
	(SELECT patient_id FROM chunk ORDER BY patient_id DESC LIMIT 1) AS last_patient_id`
	// Placeholders appear in text order: chunk window/cursor/limit, latest
	// specimen window, then the filter's arguments inside the first subselect.
	args = append(args, filterArgs...)
	return sql, args, nil
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
