package ingestion_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/ingestion"
	domaintypes "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func testdataPath(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	args := append([]string{filepath.Dir(file), "testdata"}, parts...)
	return filepath.Join(args...)
}

func observationValue(t *testing.T, specimen domaintypes.NormalizedSpecimen, field string) domaintypes.NormalizedObservation {
	t.Helper()
	for _, observation := range specimen.Observations {
		if observation.FieldCode == field {
			return observation
		}
	}
	t.Fatalf("missing field %s", field)
	return domaintypes.NormalizedObservation{}
}

func hasAnomalyReason(anomalies []domaintypes.Anomaly, reason string) bool {
	for _, anomaly := range anomalies {
		if anomaly.Reason == reason || strings.HasPrefix(anomaly.Reason, reason+":") {
			return true
		}
	}
	return false
}

func TestVNAAdapter_ParsesWideFixture(t *testing.T) {
	adapter := ingestion.NewVNAAdapter()
	specimens, anomalies, err := adapter.ParseFile(testdataPath(t, "VN_A", "results.csv"))
	require.NoError(t, err)

	require.Len(t, specimens, 4)
	require.True(t, hasAnomalyReason(anomalies, "ambiguous_date"))

	byPatient := map[string]int{}
	for _, specimen := range specimens {
		byPatient[specimen.ExternalPatientID]++
		require.NotZero(t, specimen.CollectedAt)
		require.Len(t, specimen.Observations, 9)
	}
	require.Equal(t, 1, byPatient["0000001"])
	require.Equal(t, 1, byPatient["9000010"])

	var foundCensored bool
	for _, specimen := range specimens {
		if specimen.ExternalPatientID != "9000010" {
			continue
		}
		require.Equal(t, time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), specimen.CollectedAt)
		hbf := observationValue(t, specimen, "HBF")
		require.True(t, hbf.Censored)
		require.Equal(t, "<0.1", hbf.RawValue)
		require.Equal(t, "0.1", hbf.Value)
		require.Equal(t, "10.8", observationValue(t, specimen, "HB").Value)
		foundCensored = true
	}
	require.True(t, foundCensored)
}
