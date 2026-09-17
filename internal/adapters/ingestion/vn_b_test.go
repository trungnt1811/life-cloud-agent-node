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
}
