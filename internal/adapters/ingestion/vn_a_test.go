package ingestion_test

import (
	"os"
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
		require.NotEmpty(t, specimen.ExternalSpecimenID)
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
		require.Equal(t, "S9000010", specimen.ExternalSpecimenID)
		hbf := observationValue(t, specimen, "HBF")
		require.True(t, hbf.Censored)
		require.Equal(t, "<0.1", hbf.RawValue)
		require.Equal(t, "0.1", hbf.Value)
		require.Equal(t, "10.8", observationValue(t, specimen, "HB").Value)
		foundCensored = true
	}
	require.True(t, foundCensored)
}

func TestVNAAdapter_FlagsMissingSpecimenID(t *testing.T) {
	header := "MRN,EncounterNo,SpecimenNo,Collected,DateFormat,Revision,Status,source_dataset,source_file,source_row_number,source_record_id,HGB,HGB_unit,MCV,MCV_unit,MCH,MCH_unit,RBC,RBC_unit,MCHC,MCHC_unit,RDW_CV,RDW_CV_unit,HbA0,HbA0_unit,HbA2,HbA2_unit,HbF,HbF_unit\n"
	row := "1234567,E1,,2024-06-15,%Y-%m-%d,1,final,VN_A,t.csv,1,rec-1,10.8,g/dL,79,fL,27,pg,4.5,10^12/L,32,g/dL,14,%,0.9,%,0.026,%,0.2,%\n"
	path := writeTempCSV(t, header+row)

	adapter := ingestion.NewVNAAdapter()
	specimens, anomalies, err := adapter.ParseFile(path)
	require.NoError(t, err)
	require.Empty(t, specimens)
	require.True(t, hasAnomalyReason(anomalies, "missing_specimen_id"))
}

func writeTempCSV(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.csv")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
