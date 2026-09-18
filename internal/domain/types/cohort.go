package types

import "time"

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
