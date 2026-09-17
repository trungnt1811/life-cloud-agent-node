package ingestion

import (
	"fmt"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	domaintypes "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

const (
	reasonMissingValue          = "missing_value"
	reasonMissingUnit           = "missing_unit"
	reasonUnknownUnit           = "unknown_unit"
	reasonInvalidNumericValue   = "invalid_numeric_value"
	reasonAmbiguousDate         = "ambiguous_date"
	reasonInvalidDate           = "invalid_date"
	reasonInvalidRevisionStatus = "invalid_revision_or_status"
	maxNormalizedNumericRunes   = 64
)

type testMapping struct {
	Code          string
	AcceptedUnits map[string]string // raw unit → multiply factor (decimal string)
}

type columnNames struct {
	PatientID  string
	SpecimenID string
	Date       string
	DateFormat string
	Revision   string
	Status     string
}

func anomaly(path string, rowNumber int, reason string) domaintypes.Anomaly {
	return domaintypes.Anomaly{
		SourceFile:      filepath.Base(path),
		SourceRowNumber: rowNumber,
		Reason:          reason,
	}
}

func parseRowNumber(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return n
}

func parseCollectedAt(rawDate, rawFormat, expectedFormat, goLayout string) (time.Time, string) {
	rawDate = strings.TrimSpace(rawDate)
	rawFormat = strings.TrimSpace(rawFormat)
	if rawFormat == "" {
		return time.Time{}, reasonAmbiguousDate
	}
	if rawFormat != expectedFormat {
		return time.Time{}, reasonInvalidDate
	}
	parsed, err := time.Parse(goLayout, rawDate)
	if err != nil {
		return time.Time{}, reasonInvalidDate
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), ""
}

// validateRowHeader performs the patient/specimen/status/revision checks
// shared by every row format before any field-specific normalization is
// attempted. Returns the trimmed patient and specimen IDs and the parsed
// revision, or a non-empty anomaly reason when any check fails (the row
// must be skipped).
func validateRowHeader(row map[string]string, columns columnNames) (patientID, specimenID string, revision int, reason string) {
	patientID = strings.TrimSpace(row[columns.PatientID])
	if patientID == "" {
		return "", "", 0, "missing_patient_id"
	}
	specimenID = strings.TrimSpace(row[columns.SpecimenID])
	if specimenID == "" {
		return "", "", 0, "missing_specimen_id"
	}
	if r := validateFinalRevision(row[columns.Status], row[columns.Revision]); r != "" {
		return "", "", 0, r
	}
	rev, err := strconv.Atoi(strings.TrimSpace(row[columns.Revision]))
	if err != nil {
		return "", "", 0, "invalid_revision_or_status"
	}
	return patientID, specimenID, rev, ""
}

func validateFinalRevision(status, revision string) string {
	if strings.TrimSpace(status) != "final" {
		return reasonInvalidRevisionStatus
	}
	n, err := strconv.Atoi(strings.TrimSpace(revision))
	if err != nil || n < 1 {
		return reasonInvalidRevisionStatus
	}
	return ""
}

func normalizeMeasurement(
	rawValue, rawUnit, decimalSeparator string,
	mapping testMapping,
) (domaintypes.NormalizedObservation, string) {
	rawValue = strings.TrimSpace(rawValue)
	rawUnit = strings.TrimSpace(rawUnit)
	if rawValue == "" {
		return domaintypes.NormalizedObservation{}, reasonMissingValue
	}
	if rawUnit == "" {
		return domaintypes.NormalizedObservation{}, reasonMissingUnit
	}
	factor, ok := mapping.AcceptedUnits[rawUnit]
	if !ok {
		return domaintypes.NormalizedObservation{}, reasonUnknownUnit
	}

	censored := false
	text := rawValue
	if strings.HasPrefix(text, "<") || strings.HasPrefix(text, ">") {
		censored = true
		text = strings.TrimSpace(text[1:])
	}
	if decimalSeparator != "" && decimalSeparator != "." {
		text = strings.ReplaceAll(text, decimalSeparator, ".")
	}
	if !isBoundedNumericLiteral(text) {
		return domaintypes.NormalizedObservation{}, reasonInvalidNumericValue
	}

	// Length is capped by isBoundedNumericLiteral (gosec G113).
	valueRat, ok := new(big.Rat).SetString(text) //nolint:gosec
	if !ok {
		return domaintypes.NormalizedObservation{}, reasonInvalidNumericValue
	}
	if valueRat.Sign() < 0 {
		return domaintypes.NormalizedObservation{}, reasonInvalidNumericValue
	}
	if !isBoundedNumericLiteral(factor) {
		return domaintypes.NormalizedObservation{}, reasonInvalidNumericValue
	}
	// Factors come from compile-time mapping constants (gosec G113).
	factorRat, ok := new(big.Rat).SetString(factor) //nolint:gosec
	if !ok {
		return domaintypes.NormalizedObservation{}, reasonInvalidNumericValue
	}
	valueRat.Mul(valueRat, factorRat)

	return domaintypes.NormalizedObservation{
		FieldCode: mapping.Code,
		Value:     formatRat(valueRat),
		Censored:  censored,
		RawValue:  rawValue,
		RawUnit:   rawUnit,
	}, ""
}

func isBoundedNumericLiteral(text string) bool {
	if text == "" || len([]rune(text)) > maxNormalizedNumericRunes {
		return false
	}
	for _, r := range text {
		if unicode.IsDigit(r) || r == '.' || r == '-' || r == '+' || r == '/' || r == 'e' || r == 'E' {
			continue
		}
		return false
	}
	return true
}

func formatRat(value *big.Rat) string {
	if value == nil {
		return ""
	}
	if value.IsInt() {
		return value.Num().String()
	}
	s := value.FloatString(18)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

func provenanceFromRow(row map[string]string) domaintypes.Provenance {
	return domaintypes.Provenance{
		SourceDataset:   strings.TrimSpace(row["source_dataset"]),
		SourceFile:      strings.TrimSpace(row["source_file"]),
		SourceRowNumber: parseRowNumber(row["source_row_number"]),
		SourceRecordID:  strings.TrimSpace(row["source_record_id"]),
	}
}

func requireColumns(headers []string, required ...string) error {
	present := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		present[header] = struct{}{}
	}
	for _, name := range required {
		if _, ok := present[name]; !ok {
			return fmt.Errorf("missing required column %q", name)
		}
	}
	return nil
}
