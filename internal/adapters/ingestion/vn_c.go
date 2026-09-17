package ingestion

// NewVNCAdapter parses VN_C semicolon UTF-8-BOM CSV exports (DD/MM/YYYY, comma decimals).
func NewVNCAdapter() *wideProfile {
	return &wideProfile{
		name:               ProfileVNC,
		delimiter:          ';',
		stripBOM:           true,
		decimalSeparator:   ",",
		expectedDateFormat: "%d/%m/%Y",
		dateLayout:         "02/01/2006",
		columns: columnNames{
			PatientID:  "MaBN",
			SpecimenID: "MaMau",
			Date:       "NgayXN",
			DateFormat: "KieuNgay",
			Revision:   "LanSua",
			Status:     "TrangThai",
		},
		tests: map[string]testMapping{
			"Huyet sac to":  {Code: "HB", AcceptedUnits: map[string]string{"g / dL": "1"}},
			"The tich HC":   {Code: "MCV", AcceptedUnits: map[string]string{"fL": "1"}},
			"Luong Hb HC":   {Code: "MCH", AcceptedUnits: map[string]string{"pg": "1"}},
			"So luong HC":   {Code: "RBC", AcceptedUnits: map[string]string{"10^12/L": "1"}},
			"Nong do Hb HC": {Code: "MCHC", AcceptedUnits: map[string]string{"g / dL": "1"}},
			"RDW":           {Code: "RDW", AcceptedUnits: map[string]string{"%": "1"}},
			"Hb A0":         {Code: "HBA0", AcceptedUnits: map[string]string{"%": "1"}},
			"Hb A2":         {Code: "HBA2", AcceptedUnits: map[string]string{"%": "1"}},
			"Hb F":          {Code: "HBF", AcceptedUnits: map[string]string{"%": "1"}},
		},
	}
}
