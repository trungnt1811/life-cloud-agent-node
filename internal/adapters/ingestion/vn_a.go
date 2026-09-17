package ingestion

// NewVNAAdapter parses VN_A wide CSV exports (ISO dates, Hb already g/dL).
func NewVNAAdapter() *wideProfile {
	return &wideProfile{
		name:               ProfileVNA,
		delimiter:          ',',
		decimalSeparator:   ".",
		expectedDateFormat: "%Y-%m-%d",
		dateLayout:         "2006-01-02",
		columns: columnNames{
			PatientID:  "MRN",
			SpecimenID: "SpecimenNo",
			Date:       "Collected",
			DateFormat: "DateFormat",
			Revision:   "Revision",
			Status:     "Status",
		},
		tests: map[string]testMapping{
			"HGB":    {Code: "HB", AcceptedUnits: map[string]string{"g/dL": "1"}},
			"MCV":    {Code: "MCV", AcceptedUnits: map[string]string{"fL": "1"}},
			"MCH":    {Code: "MCH", AcceptedUnits: map[string]string{"pg": "1"}},
			"RBC":    {Code: "RBC", AcceptedUnits: map[string]string{"10^12/L": "1"}},
			"MCHC":   {Code: "MCHC", AcceptedUnits: map[string]string{"g/dL": "1"}},
			"RDW_CV": {Code: "RDW", AcceptedUnits: map[string]string{"%": "1"}},
			"HbA0":   {Code: "HBA0", AcceptedUnits: map[string]string{"%": "1"}},
			"HbA2":   {Code: "HBA2", AcceptedUnits: map[string]string{"%": "1"}},
			"HbF":    {Code: "HBF", AcceptedUnits: map[string]string{"%": "1"}},
		},
	}
}
