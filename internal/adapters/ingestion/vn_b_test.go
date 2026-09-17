package ingestion_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/ingestion"
)

func TestVNBAdapter_ConvertsUnitsAndGroupsLongRows(t *testing.T) {
	adapter := ingestion.NewVNBAdapter()
	specimens, anomalies, err := adapter.ParseFile(testdataPath(t, "VN_B", "results.csv"))
	require.NoError(t, err)

	require.Len(t, specimens, 3)
	require.True(t, hasAnomalyReason(anomalies, "ambiguous_date"))

	var foundCensored bool
	for _, specimen := range specimens {
		require.Len(t, specimen.Observations, 9)
		switch specimen.ExternalPatientID {
		case "0000001":
			require.Equal(t, time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC), specimen.CollectedAt)
			hb := observationValue(t, specimen, "HB")
			require.Equal(t, "11.5", hb.Value) // 115 g/L → 11.5 g/dL
			require.Equal(t, "g/L", hb.RawUnit)
			require.Equal(t, "115", hb.RawValue)
		case "9000010":
			hbf := observationValue(t, specimen, "HBF")
			require.True(t, hbf.Censored)
			require.Equal(t, "0.1", hbf.Value) // <0.001 fraction * 100
			require.Equal(t, "<0.001", hbf.RawValue)
			foundCensored = true
		}
	}
	require.True(t, foundCensored)
	for _, specimen := range specimens {
		require.NotEmpty(t, specimen.ExternalSpecimenID)
	}
}

const vnBHeader = "pid,visit_id,sample_id,ts,ts_format,ver,state,source_dataset,source_file,source_row_number,source_record_id,result_id,test_code,result,uom\n"

func TestVNBAdapter_FlagsDateMismatchWithinGroup(t *testing.T) {
	// Two rows for the same patient/specimen group disagree on the
	// collection date - a real data-quality conflict, not something to
	// resolve by silently keeping the first row's date.
	rows := vnBHeader +
		"P1,E1,S1,20240101,%Y%m%d,1,final,VN_B,t.csv,1,rec-1,r1,0301,100,g/L\n" +
		"P1,E1,S1,20240102,%Y%m%d,1,final,VN_B,t.csv,2,rec-2,r2,0304,80,fL\n"
	path := writeTempCSV(t, rows)

	adapter := ingestion.NewVNBAdapter()
	specimens, anomalies, err := adapter.ParseFile(path)
	require.NoError(t, err)
	require.Empty(t, specimens, "a group with disagreeing dates must be quarantined, not silently resolved")
	require.True(t, hasAnomalyReason(anomalies, "date_mismatch_in_group"))
}

func TestVNBAdapter_FlagsConflictingValueAtSameRevision(t *testing.T) {
	// Two rows claim the same revision for the same test code but disagree
	// on the value - excluded from the result and flagged, not silently
	// resolved by file order. The unaffected MCV field still comes through.
	rows := vnBHeader +
		"P2,E2,S2,20240101,%Y%m%d,1,final,VN_B,t.csv,1,rec-1,r1,0301,100,g/L\n" +
		"P2,E2,S2,20240101,%Y%m%d,1,final,VN_B,t.csv,2,rec-2,r2,0301,200,g/L\n" +
		"P2,E2,S2,20240101,%Y%m%d,1,final,VN_B,t.csv,3,rec-3,r3,0304,80,fL\n"
	path := writeTempCSV(t, rows)

	adapter := ingestion.NewVNBAdapter()
	specimens, anomalies, err := adapter.ParseFile(path)
	require.NoError(t, err)
	require.True(t, hasAnomalyReason(anomalies, "conflicting_value_same_revision:HB"))
	require.Len(t, specimens, 1)
	require.Len(t, specimens[0].Observations, 1, "the conflicting HB field is excluded; only MCV remains")
	require.Equal(t, "MCV", specimens[0].Observations[0].FieldCode)
}
