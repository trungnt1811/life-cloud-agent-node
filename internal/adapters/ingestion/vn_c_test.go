package ingestion_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/ingestion"
)

func TestVNCAdapter_ParsesSemicolonBOMAndCommaDecimals(t *testing.T) {
	adapter := ingestion.NewVNCAdapter()
	specimens, anomalies, err := adapter.ParseFile(testdataPath(t, "VN_C", "results.csv"))
	require.NoError(t, err)

	require.Len(t, specimens, 4)
	require.True(t, hasAnomalyReason(anomalies, "ambiguous_date"))

	var foundPatient bool
	var foundCensored bool
	for _, specimen := range specimens {
		require.Len(t, specimen.Observations, 9)
		switch specimen.ExternalPatientID {
		case "0000001":
			foundPatient = true
			require.Equal(t, time.Date(2024, 5, 9, 0, 0, 0, 0, time.UTC), specimen.CollectedAt)
			hb := observationValue(t, specimen, "HB")
			require.Equal(t, "13.2", hb.Value)
			require.Equal(t, "g / dL", hb.RawUnit)
			require.Equal(t, "13,2", hb.RawValue)
			require.Equal(t, "86.7", observationValue(t, specimen, "MCV").Value)
		case "9000010":
			hbf := observationValue(t, specimen, "HBF")
			require.True(t, hbf.Censored)
			require.Equal(t, "0.1", hbf.Value)
			foundCensored = true
		}
	}
	require.True(t, foundPatient)
	require.True(t, foundCensored)
}
