package entities

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Specimen is one collected sample for a patient in the local registry.
type Specimen struct {
	id                 uuid.UUID
	patientID          uuid.UUID
	externalSpecimenID string
	collectedAt        time.Time
	sourceDataset      string
	sourceFile         string
	sourceRowNumber    int
	sourceRecordID     string
	createdAt          time.Time
}

// SpecimenRecord is a persistence snapshot used at repository boundaries.
type SpecimenRecord struct {
	ID                 uuid.UUID
	PatientID          uuid.UUID
	ExternalSpecimenID string
	CollectedAt        time.Time
	SourceDataset      string
	SourceFile         string
	SourceRowNumber    int
	SourceRecordID     string
	CreatedAt          time.Time
}

// NewSpecimenParams are the inputs to NewSpecimen, grouped in a struct so
// same-typed fields (the source-provenance strings) can't be silently
// transposed at a call site the way positional string arguments can.
type NewSpecimenParams struct {
	ID        uuid.UUID
	PatientID uuid.UUID
	// ExternalSpecimenID is the hospital's own specimen/sample identifier
	// (e.g. SpecimenNo, sample_id, MaMau). Together with SourceDataset and
	// PatientID, this is the real-world identity used to detect a
	// corrected re-export of the same specimen - not file/row provenance.
	ExternalSpecimenID string
	CollectedAt        time.Time
	SourceDataset      string
	SourceFile         string
	SourceRowNumber    int
	SourceRecordID     string
	Now                time.Time
}

// NewSpecimen creates a specimen. CollectedAt is stored as a calendar date.
func NewSpecimen(params NewSpecimenParams) *Specimen {
	return &Specimen{
		id:                 ensureID(params.ID),
		patientID:          params.PatientID,
		externalSpecimenID: strings.TrimSpace(params.ExternalSpecimenID),
		collectedAt:        calendarDate(params.CollectedAt),
		sourceDataset:      strings.TrimSpace(params.SourceDataset),
		sourceFile:         strings.TrimSpace(params.SourceFile),
		sourceRowNumber:    params.SourceRowNumber,
		sourceRecordID:     strings.TrimSpace(params.SourceRecordID),
		createdAt:          ensureTimestamp(params.Now),
	}
}

// NewSpecimenFromRecord hydrates a specimen from persistence data.
func NewSpecimenFromRecord(record SpecimenRecord) *Specimen {
	if record.ID == uuid.Nil {
		return nil
	}
	return &Specimen{
		id:                 record.ID,
		patientID:          record.PatientID,
		externalSpecimenID: record.ExternalSpecimenID,
		collectedAt:        calendarDate(record.CollectedAt),
		sourceDataset:      record.SourceDataset,
		sourceFile:         record.SourceFile,
		sourceRowNumber:    record.SourceRowNumber,
		sourceRecordID:     record.SourceRecordID,
		createdAt:          record.CreatedAt,
	}
}

// Record returns a persistence snapshot.
func (s *Specimen) Record() SpecimenRecord {
	if s == nil {
		return SpecimenRecord{}
	}
	return SpecimenRecord{
		ID:                 s.id,
		PatientID:          s.patientID,
		ExternalSpecimenID: s.externalSpecimenID,
		CollectedAt:        s.collectedAt,
		SourceDataset:      s.sourceDataset,
		SourceFile:         s.sourceFile,
		SourceRowNumber:    s.sourceRowNumber,
		SourceRecordID:     s.sourceRecordID,
		CreatedAt:          s.createdAt,
	}
}

func (s *Specimen) ID() uuid.UUID {
	if s == nil {
		return uuid.Nil
	}
	return s.id
}

func (s *Specimen) PatientID() uuid.UUID {
	if s == nil {
		return uuid.Nil
	}
	return s.patientID
}

func (s *Specimen) ExternalSpecimenID() string {
	if s == nil {
		return ""
	}
	return s.externalSpecimenID
}

func (s *Specimen) CollectedAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.collectedAt
}

func (s *Specimen) SourceDataset() string {
	if s == nil {
		return ""
	}
	return s.sourceDataset
}

func (s *Specimen) SourceFile() string {
	if s == nil {
		return ""
	}
	return s.sourceFile
}

func (s *Specimen) SourceRowNumber() int {
	if s == nil {
		return 0
	}
	return s.sourceRowNumber
}

func (s *Specimen) SourceRecordID() string {
	if s == nil {
		return ""
	}
	return s.sourceRecordID
}

func (s *Specimen) CreatedAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.createdAt
}

// calendarDate keeps the calendar date (year/month/day) as given, without
// first converting across timezones. Converting to UTC before truncating
// would shift the date backward a day for any positive-offset local
// timestamp (e.g. an early-morning collection in Vietnam, UTC+7).
func calendarDate(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
