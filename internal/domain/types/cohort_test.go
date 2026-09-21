package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func baseCriteria() CohortCriteria {
	return CohortCriteria{
		From: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
		Conditions: []CohortCondition{
			{FieldCode: "MCV", Op: ComparisonOpLT, NumberValue: "80"},
			{FieldCode: "MCH", Op: ComparisonOpLT, NumberValue: "27"},
		},
		RequiredPanels: []CohortRequiredPanel{
			{FieldCodes: []string{"HB", "MCV"}, Constraint: PanelValueExact},
			{FieldCodes: []string{"HBA0", "HBF"}, Constraint: PanelValueAllowCensored},
		},
	}
}

func TestCohortCriteriaFingerprint_IgnoresOrderOfAndedSets(t *testing.T) {
	t.Parallel()

	reordered := baseCriteria()
	reordered.Conditions[0], reordered.Conditions[1] = reordered.Conditions[1], reordered.Conditions[0]
	reordered.RequiredPanels[0], reordered.RequiredPanels[1] = reordered.RequiredPanels[1], reordered.RequiredPanels[0]
	reordered.RequiredPanels[0].FieldCodes = []string{"HBF", "HBA0"}

	require.Equal(t, baseCriteria().Fingerprint(), reordered.Fingerprint())
	require.Len(t, baseCriteria().Fingerprint(), 64)
}

func TestCohortCriteriaFingerprint_DistinguishesDifferentQueries(t *testing.T) {
	t.Parallel()

	base := baseCriteria().Fingerprint()
	mutations := map[string]func(*CohortCriteria){
		"from":       func(c *CohortCriteria) { c.From = c.From.AddDate(0, 0, 1) },
		"to":         func(c *CohortCriteria) { c.To = c.To.AddDate(0, 0, -1) },
		"op":         func(c *CohortCriteria) { c.Conditions[0].Op = ComparisonOpLTE },
		"value":      func(c *CohortCriteria) { c.Conditions[0].NumberValue = "81" },
		"field":      func(c *CohortCriteria) { c.Conditions[0].FieldCode = "RBC" },
		"constraint": func(c *CohortCriteria) { c.RequiredPanels[0].Constraint = PanelValueAllowCensored },
		"panel_code": func(c *CohortCriteria) { c.RequiredPanels[0].FieldCodes[0] = "RBC" },
		"drop_cond":  func(c *CohortCriteria) { c.Conditions = c.Conditions[:1] },
		"drop_panel": func(c *CohortCriteria) { c.RequiredPanels = c.RequiredPanels[:1] },
	}
	for name, mutate := range mutations {
		criteria := baseCriteria()
		mutate(&criteria)
		require.NotEqual(t, base, criteria.Fingerprint(), name)
	}
}

func TestCohortCriteriaFingerprint_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	criteria := baseCriteria()
	criteria.RequiredPanels[0].FieldCodes = []string{"MCV", "HB"}
	_ = criteria.Fingerprint()
	require.Equal(t, []string{"MCV", "HB"}, criteria.RequiredPanels[0].FieldCodes)
}
