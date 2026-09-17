package ingestion

// NewVNBAdapter parses VN_B long CSV exports (YYYYMMDD, Hb g/L, HPLC fractions).
func NewVNBAdapter() *longProfile {
	return &longProfile{
		name:               ProfileVNB,
		delimiter:          ',',
		decimalSeparator:   ".",
		expectedDateFormat: "%Y%m%d",
		dateLayout:         "20060102",
		columns: columnNames{
			PatientID:  "pid",
			SpecimenID: "sample_id",
			Date:       "ts",
			DateFormat: "ts_format",
			Revision:   "ver",
			Status:     "state",
		},
		testCodeColumn: "test_code",
		resultColumn:   "result",
		unitColumn:     "uom",
		tests: map[string]testMapping{
			"0301": {Code: "HB", AcceptedUnits: map[string]string{"g/L": "0.1"}},
			"0304": {Code: "MCV", AcceptedUnits: map[string]string{"fL": "1"}},
			"0305": {Code: "MCH", AcceptedUnits: map[string]string{"pg": "1"}},
			"0302": {Code: "RBC", AcceptedUnits: map[string]string{"10^12/L": "1"}},
			"0306": {Code: "MCHC", AcceptedUnits: map[string]string{"g/dL": "1"}},
			"0307": {Code: "RDW", AcceptedUnits: map[string]string{"%": "1"}},
			"H01":  {Code: "HBA0", AcceptedUnits: map[string]string{"fraction": "100"}},
			"H02":  {Code: "HBA2", AcceptedUnits: map[string]string{"fraction": "100"}},
			"H03":  {Code: "HBF", AcceptedUnits: map[string]string{"fraction": "100"}},
		},
	}
}
