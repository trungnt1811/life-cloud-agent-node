package models

import (
	"time"

	"github.com/google/uuid"
)

// Patient is the GORM persistence model for the patients table.
type Patient struct {
	ID                uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ExternalPatientID string    `gorm:"type:varchar(255);not null;uniqueIndex"`
	CreatedAt         time.Time `gorm:"type:timestamp with time zone;not null;default:CURRENT_TIMESTAMP"`
}

func (Patient) TableName() string {
	return "patients"
}

// Specimen is the GORM persistence model for the specimens table.
type Specimen struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	PatientID       uuid.UUID `gorm:"type:uuid;not null;index:specimens_patient_id_collected_at_idx,priority:1"`
	CollectedAt     time.Time `gorm:"type:date;not null;index:specimens_patient_id_collected_at_idx,priority:2"`
	SourceDataset   string    `gorm:"type:varchar(255);not null;default:'';uniqueIndex:specimens_source_provenance_key,priority:1"`
	SourceFile      string    `gorm:"type:varchar(512);not null;default:'';uniqueIndex:specimens_source_provenance_key,priority:2"`
	SourceRowNumber int       `gorm:"type:integer;not null;default:0"`
	SourceRecordID  string    `gorm:"type:varchar(255);not null;default:'';uniqueIndex:specimens_source_provenance_key,priority:3"`
	CreatedAt       time.Time `gorm:"type:timestamp with time zone;not null;default:CURRENT_TIMESTAMP"`
}

func (Specimen) TableName() string {
	return "specimens"
}

// LabObservation is the GORM persistence model for the lab_observations table.
type LabObservation struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	SpecimenID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:lab_observations_specimen_id_field_code_key,priority:1"`
	FieldCode  string    `gorm:"type:varchar(64);not null;uniqueIndex:lab_observations_specimen_id_field_code_key,priority:2"`
	Value      string    `gorm:"type:text;not null"`
	Censored   bool      `gorm:"type:boolean;not null;default:false"`
	RawValue   string    `gorm:"type:text;not null;default:''"`
	RawUnit    string    `gorm:"type:varchar(64);not null;default:''"`
	CreatedAt  time.Time `gorm:"type:timestamp with time zone;not null;default:CURRENT_TIMESTAMP"`
}

func (LabObservation) TableName() string {
	return "lab_observations"
}
