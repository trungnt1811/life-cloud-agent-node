package types

import "time"

// Provenance traces a normalized specimen back to a hospital export cell/row.
type Provenance struct {
	SourceDataset   string
	SourceFile      string
	SourceRowNumber int
	SourceRecordID  string
}

// NormalizedObservation is one measurement after site-specific unit conversion.
type NormalizedObservation struct {
	FieldCode string
	Value     string // canonical decimal string, unrounded
	Censored  bool
	RawValue  string
	RawUnit   string
}

// NormalizedSpecimen is one collected sample ready for D3 persistence.
type NormalizedSpecimen struct {
	ExternalPatientID string
	CollectedAt       time.Time
	Observations      []NormalizedObservation
	Provenance        Provenance
}

// Anomaly is a row or measurement that could not be normalized.
// It is recorded, never silently corrected or dropped.
type Anomaly struct {
	SourceFile      string
	SourceRowNumber int
	Reason          string
}
