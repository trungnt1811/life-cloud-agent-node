package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// Hand-computed fixture for the demo cohort rule (MCV<80, MCH<27, CBC EXACT,
// HPLC ALLOW_CENSORED) over 2024-01-01..2024-12-31. Expected raw matches: 2
// (patients MATCH-A and MATCH-LEXICAL).
func TestPatientRegistryRepository_CountMatchingCohort_DemoRuleFixture(t *testing.T) {
	db := openPatientRegistryRepositoryTestDB(t)
	ctx := context.Background()
	repo := NewPatientRegistryRepository(db, logger.GetLogger())
	now := time.Now().UTC()

	seed := func(externalID string, specimens []cohortSpecimenSeed) {
		t.Helper()
		patient, err := repo.GetOrCreatePatient(ctx, externalID)
		require.NoError(t, err)
		for _, specimenSeed := range specimens {
			specimenID := uuid.New()
			specimen := entities.NewSpecimen(entities.NewSpecimenParams{
				ID:                 specimenID,
				PatientID:          patient.ID(),
				ExternalSpecimenID: specimenSeed.ExternalID,
				CollectedAt:        specimenSeed.CollectedAt,
				SourceDataset:      "TEST",
				SourceFile:         "fixture.csv",
				SourceRecordID:     specimenSeed.ExternalID,
				SourceRowNumber:    1,
				Now:                now,
			})
			observations := make([]*entities.LabObservation, 0, len(specimenSeed.Obs))
			for _, obs := range specimenSeed.Obs {
				observations = append(observations, entities.NewLabObservation(entities.NewLabObservationParams{
					ID:         uuid.New(),
					SpecimenID: specimenID,
					FieldCode:  obs.FieldCode,
					Value:      obs.Value,
					Censored:   obs.Censored,
					RawValue:   obs.Value,
					RawUnit:    "u",
					Revision:   1,
					Now:        now,
				}))
			}
			require.NoError(t, repo.SaveSpecimen(ctx, specimen, observations))
		}
	}

	cbcExact := func(hb, mcv, mch, rbc string) []cohortObsSeed {
		return []cohortObsSeed{
			{FieldCode: "HB", Value: hb},
			{FieldCode: "MCV", Value: mcv},
			{FieldCode: "MCH", Value: mch},
			{FieldCode: "RBC", Value: rbc},
		}
	}
	hplcAllow := func(censoredHBF bool) []cohortObsSeed {
		return []cohortObsSeed{
			{FieldCode: "HBA0", Value: "96.5"},
			{FieldCode: "HBA2", Value: "2.8"},
			{FieldCode: "HBF", Value: "0.1", Censored: censoredHBF},
		}
	}
	merge := func(parts ...[]cohortObsSeed) []cohortObsSeed {
		var out []cohortObsSeed
		for _, part := range parts {
			out = append(out, part...)
		}
		return out
	}

	inRange := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	older := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)
	outOfRange := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)

	// MATCH-A: single in-range specimen, full panels, rule match (incl. censored HBF).
	seed("MATCH-A", []cohortSpecimenSeed{{
		ExternalID:  "S-A1",
		CollectedAt: inRange,
		Obs:         merge(cbcExact("10.8", "70", "22", "5.1"), hplcAllow(true)),
	}})

	// NO-RULE: complete panels but MCV/MCH above thresholds.
	seed("NO-RULE", []cohortSpecimenSeed{{
		ExternalID:  "S-B1",
		CollectedAt: inRange,
		Obs:         merge(cbcExact("13.2", "88", "29", "4.5"), hplcAllow(false)),
	}})

	// INCOMPLETE-CBC: missing RBC → EXACT panel fails.
	seed("INCOMPLETE-CBC", []cohortSpecimenSeed{{
		ExternalID:  "S-C1",
		CollectedAt: inRange,
		Obs: merge(
			[]cohortObsSeed{
				{FieldCode: "HB", Value: "10.8"},
				{FieldCode: "MCV", Value: "70"},
				{FieldCode: "MCH", Value: "22"},
			},
			hplcAllow(false),
		),
	}})

	// MISSING-HPLC: CBC + rule ok, HPLC incomplete.
	seed("MISSING-HPLC", []cohortSpecimenSeed{{
		ExternalID:  "S-D1",
		CollectedAt: inRange,
		Obs: merge(cbcExact("10.8", "70", "22", "5.1"), []cohortObsSeed{
			{FieldCode: "HBA0", Value: "96.5"},
			{FieldCode: "HBA2", Value: "2.8"},
		}),
	}})

	// LATEST-WINS: older specimen would match; newer in-range specimen does not.
	seed("LATEST-WINS", []cohortSpecimenSeed{
		{
			ExternalID:  "S-E-OLD",
			CollectedAt: older,
			Obs:         merge(cbcExact("10.8", "70", "22", "5.1"), hplcAllow(false)),
		},
		{
			ExternalID:  "S-E-NEW",
			CollectedAt: newer,
			Obs:         merge(cbcExact("13.2", "88", "29", "4.5"), hplcAllow(false)),
		},
	})

	// MATCH-LEXICAL: same collected_at; higher lexical specimen id is selected and matches.
	seed("MATCH-LEXICAL", []cohortSpecimenSeed{
		{
			ExternalID:  "S-F-A",
			CollectedAt: inRange,
			Obs:         merge(cbcExact("13.2", "88", "29", "4.5"), hplcAllow(false)),
		},
		{
			ExternalID:  "S-F-B",
			CollectedAt: inRange,
			Obs:         merge(cbcExact("10.8", "70", "22", "5.1"), hplcAllow(false)),
		},
	})

	// OUT-OF-RANGE: only specimen outside the window → not counted.
	seed("OUT-OF-RANGE", []cohortSpecimenSeed{{
		ExternalID:  "S-G1",
		CollectedAt: outOfRange,
		Obs:         merge(cbcExact("10.8", "70", "22", "5.1"), hplcAllow(false)),
	}})

	// CENSORED-CBC: HB censored fails EXACT panel.
	seed("CENSORED-CBC", []cohortSpecimenSeed{{
		ExternalID:  "S-H1",
		CollectedAt: inRange,
		Obs: merge(
			[]cohortObsSeed{
				{FieldCode: "HB", Value: "10.8", Censored: true},
				{FieldCode: "MCV", Value: "70"},
				{FieldCode: "MCH", Value: "22"},
				{FieldCode: "RBC", Value: "5.1"},
			},
			hplcAllow(false),
		),
	}})

	criteria := types.CohortCriteria{
		From: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
		Conditions: []types.CohortCondition{
			{FieldCode: "MCV", Op: types.ComparisonOpLT, NumberValue: "80"},
			{FieldCode: "MCH", Op: types.ComparisonOpLT, NumberValue: "27"},
		},
		RequiredPanels: []types.CohortRequiredPanel{
			{
				FieldCodes: []string{"HB", "MCV", "MCH", "RBC"},
				Constraint: types.PanelValueExact,
			},
			{
				FieldCodes: []string{"HBA0", "HBA2", "HBF"},
				Constraint: types.PanelValueAllowCensored,
			},
		},
	}

	count, err := repo.CountMatchingCohort(ctx, criteria)
	require.NoError(t, err)
	// Hand-computed: MATCH-A + MATCH-LEXICAL only.
	require.Equal(t, uint64(2), count)
}

func TestPatientRegistryRepository_CountMatchingCohort_RejectsInvertedRange(t *testing.T) {
	db := openPatientRegistryRepositoryTestDB(t)
	repo := NewPatientRegistryRepository(db, logger.GetLogger())
	_, err := repo.CountMatchingCohort(context.Background(), types.CohortCriteria{
		From: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "from must be")
}

type cohortObsSeed struct {
	FieldCode string
	Value     string
	Censored  bool
}

type cohortSpecimenSeed struct {
	ExternalID  string
	CollectedAt time.Time
	Obs         []cohortObsSeed
}
