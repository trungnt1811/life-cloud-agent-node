package entities

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Specimen is one collected sample for a patient in the local registry.
type Specimen struct {
	id              uuid.UUID
	patientID       uuid.UUID
	collectedAt     time.Time
	sourceDataset   string
	sourceFile      string
	sourceRowNumber int
	sourceRecordID  string
	createdAt       time.Time
}

// SpecimenRecord is a persistence snapshot used at repository boundaries.
type SpecimenRecord struct {
	ID              uuid.UUID
	PatientID       uuid.UUID
	CollectedAt     time.Time
	SourceDataset   string
	SourceFile      string
	SourceRowNumber int
	SourceRecordID  string
	CreatedAt       time.Time
}

// NewSpecimen creates a specimen. CollectedAt is stored as a UTC calendar date.
func NewSpecimen(
	id, patientID uuid.UUID,
	collectedAt time.Time,
	sourceDataset, sourceFile, sourceRecordID string,
	sourceRowNumber int,
	now time.Time,
) *Specimen {
	if id == uuid.Nil {
		id = uuid.New()
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &Specimen{
		id:              id,
		patientID:       patientID,
		collectedAt:     calendarDateUTC(collectedAt),
		sourceDataset:   strings.TrimSpace(sourceDataset),
		sourceFile:      strings.TrimSpace(sourceFile),
		sourceRowNumber: sourceRowNumber,
		sourceRecordID:  strings.TrimSpace(sourceRecordID),
		createdAt:       now.UTC(),
	}
}

// NewSpecimenFromRecord hydrates a specimen from persistence data.
func NewSpecimenFromRecord(record SpecimenRecord) *Specimen {
	if record.ID == uuid.Nil {
		return nil
	}
	return &Specimen{
		id:              record.ID,
		patientID:       record.PatientID,
		collectedAt:     calendarDateUTC(record.CollectedAt),
		sourceDataset:   record.SourceDataset,
		sourceFile:      record.SourceFile,
		sourceRowNumber: record.SourceRowNumber,
		sourceRecordID:  record.SourceRecordID,
		createdAt:       record.CreatedAt,
	}
}

// Record returns a persistence snapshot.
func (s *Specimen) Record() SpecimenRecord {
	if s == nil {
		return SpecimenRecord{}
	}
	return SpecimenRecord{
		ID:              s.id,
		PatientID:       s.patientID,
		CollectedAt:     s.collectedAt,
		SourceDataset:   s.sourceDataset,
		SourceFile:      s.sourceFile,
		SourceRowNumber: s.sourceRowNumber,
		SourceRecordID:  s.sourceRecordID,
		CreatedAt:       s.createdAt,
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

func calendarDateUTC(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
