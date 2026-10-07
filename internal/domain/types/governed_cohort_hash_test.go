package types

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGovernedDefinitionHashSharedVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/definition-hash-vectors.json")
	require.NoError(t, err)
	var fixture struct {
		Vectors []struct {
			Name   string `json:"name"`
			Hash   string `json:"expected_hash"`
			Params struct {
				From       string `json:"date_from"`
				To         string `json:"date_to"`
				Conditions []struct {
					Code  string       `json:"field_code"`
					Op    ComparisonOp `json:"op"`
					Value string       `json:"value"`
				} `json:"conditions"`
				Panels []struct {
					Codes      []string             `json:"field_codes"`
					Constraint PanelValueConstraint `json:"value_constraint"`
				} `json:"required_panels"`
			} `json:"params"`
		} `json:"vectors"`
	}
	require.NoError(t, json.Unmarshal(data, &fixture))
	require.NotEmpty(t, fixture.Vectors)
	for _, vector := range fixture.Vectors {
		t.Run(vector.Name, func(t *testing.T) {
			from, err := time.Parse("2006-01-02", vector.Params.From)
			require.NoError(t, err)
			to, err := time.Parse("2006-01-02", vector.Params.To)
			require.NoError(t, err)
			criteria := CohortCriteria{From: from, To: to}
			for _, item := range vector.Params.Conditions {
				criteria.Conditions = append(criteria.Conditions, CohortCondition{FieldCode: item.Code, Op: item.Op, NumberValue: item.Value})
			}
			for _, item := range vector.Params.Panels {
				criteria.RequiredPanels = append(criteria.RequiredPanels, CohortRequiredPanel{FieldCodes: item.Codes, Constraint: item.Constraint})
			}
			legacy := criteria.Fingerprint()
			hash, err := criteria.GovernedDefinitionHash()
			require.NoError(t, err)
			require.Equal(t, vector.Hash, hash)
			require.Equal(t, legacy, criteria.Fingerprint(), "legacy fingerprints and caller criteria must not change")
		})
	}
}

func TestGovernedDefinitionHashRejectsTimestampCriteria(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, start := range []time.Time{from.Add(time.Second), from.Add(time.Nanosecond), time.Date(2024, 1, 1, 0, 0, 0, 0, time.FixedZone("hospital", 7*3600))} {
		_, err := (CohortCriteria{From: start, To: from.AddDate(1, 0, 0)}).GovernedDefinitionHash()
		require.Error(t, err, "dates must match the CP YYYY-MM-DD UTC contract")
	}
}

func TestGovernedDefinitionHashRejectsUnsafeCriteria(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, condition := range []CohortCondition{
		{FieldCode: "PATIENT_NAME", Op: ComparisonOpLT, NumberValue: "80"},
		{FieldCode: "MCV", Op: "SQL", NumberValue: "80"},
		{FieldCode: "MCV", Op: ComparisonOpLT, NumberValue: "8e1"},
		{FieldCode: "MCV", Op: ComparisonOpLT, NumberValue: "NaN"},
		{FieldCode: "MCV", Op: ComparisonOpLT, NumberValue: " 80 "},
	} {
		_, err := (CohortCriteria{From: from, To: from, Conditions: []CohortCondition{condition}}).GovernedDefinitionHash()
		require.Error(t, err)
	}
	for _, panel := range []CohortRequiredPanel{
		{FieldCodes: []string{"MCV"}, Constraint: "UNKNOWN"},
		{FieldCodes: []string{"PATIENT_NAME"}, Constraint: PanelValueExact},
		{Constraint: PanelValueExact},
	} {
		_, err := (CohortCriteria{From: from, To: from, RequiredPanels: []CohortRequiredPanel{panel}}).GovernedDefinitionHash()
		require.Error(t, err)
	}
}
