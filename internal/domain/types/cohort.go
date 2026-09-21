package types

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ComparisonOp is a domain-owned comparison operator for cohort conditions.
// Values mirror the schema-v1 wire enum without importing gen/.
type ComparisonOp string

const (
	ComparisonOpEQ  ComparisonOp = "EQ"
	ComparisonOpNE  ComparisonOp = "NE"
	ComparisonOpLT  ComparisonOp = "LT"
	ComparisonOpLTE ComparisonOp = "LTE"
	ComparisonOpGT  ComparisonOp = "GT"
	ComparisonOpGTE ComparisonOp = "GTE"
)

// PanelValueConstraint is how a required panel treats censored lab values.
type PanelValueConstraint string

const (
	// PanelValueExact requires every listed field present and non-censored.
	PanelValueExact PanelValueConstraint = "EXACT"
	// PanelValueAllowCensored requires presence only; censored values count.
	PanelValueAllowCensored PanelValueConstraint = "ALLOW_CENSORED"
)

// CohortCondition is one measurement predicate against the selected specimen.
type CohortCondition struct {
	FieldCode   string
	Op          ComparisonOp
	NumberValue string // exact decimal string; never binary float
}

// CohortRequiredPanel requires a set of field_codes on the selected specimen.
type CohortRequiredPanel struct {
	FieldCodes []string
	Constraint PanelValueConstraint
}

// CohortCriteria is the domain input to CountMatchingCohort — decoupled from
// the wire QueryTask (decision 0002 / plan Phase 5).
type CohortCriteria struct {
	From           time.Time
	To             time.Time
	Conditions     []CohortCondition
	RequiredPanels []CohortRequiredPanel
}

// CohortChunkResult is one patient chunk's contribution to a cohort count.
// PatientsScanned == 0 means the walk is finished; LastPatientID is then
// uuid.Nil.
type CohortChunkResult struct {
	MatchingCount   uint64
	PatientsScanned int
	LastPatientID   uuid.UUID
}

// Fingerprint returns a stable SHA-256 hex digest of the criteria, ignoring
// the order of conditions, panels and panel field codes (all are ANDed sets),
// so equivalent queries hash equal and different ones do not.
func (c CohortCriteria) Fingerprint() string {
	conditions := make([]string, 0, len(c.Conditions))
	for _, condition := range c.Conditions {
		conditions = append(conditions, strings.Join(
			[]string{condition.FieldCode, string(condition.Op), condition.NumberValue}, "\x1f"))
	}
	sort.Strings(conditions)

	panels := make([]string, 0, len(c.RequiredPanels))
	for _, panel := range c.RequiredPanels {
		codes := append([]string(nil), panel.FieldCodes...)
		sort.Strings(codes)
		panels = append(panels, string(panel.Constraint)+"\x1f"+strings.Join(codes, "\x1e"))
	}
	sort.Strings(panels)

	canonical := strings.Join([]string{
		c.From.Format("2006-01-02"),
		c.To.Format("2006-01-02"),
		strings.Join(conditions, "\x1d"),
		strings.Join(panels, "\x1d"),
	}, "\x1c")
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}
