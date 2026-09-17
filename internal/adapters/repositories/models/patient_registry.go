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
	ID uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	// ExternalSpecimenID + PatientID + SourceDataset is the specimen's real
	// identity: the hospital's own specimen number, not file/row
	// provenance. A corrected re-export of the same specimen (new
	// filename, shifted row numbers) must still dedupe to this same row.
	PatientID          uuid.UUID `gorm:"type:uuid;not null;index:specimens_patient_id_collected_at_idx,priority:1;uniqueIndex:specimens_natural_key,priority:2"`
	ExternalSpecimenID string    `gorm:"type:varchar(255);not null;uniqueIndex:specimens_natural_key,priority:3"`
	CollectedAt        time.Time `gorm:"type:date;not null;index:specimens_patient_id_collected_at_idx,priority:2"`
	SourceDataset      string    `gorm:"type:varchar(255);not null;default:'';uniqueIndex:specimens_natural_key,priority:1"`
	SourceFile         string    `gorm:"type:varchar(512);not null;default:''"`
	SourceRowNumber    int       `gorm:"type:integer;not null;default:0"`
	SourceRecordID     string    `gorm:"type:varchar(255);not null;default:''"`
	CreatedAt          time.Time `gorm:"type:timestamp with time zone;not null;default:CURRENT_TIMESTAMP"`
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
	// Revision is the source's own revision/version number. A write only
	// overwrites a stored observation when its Revision is >= the stored
	// one - "highest revision wins" regardless of ingest call order.
	Revision  int       `gorm:"type:integer;not null;default:1"`
	CreatedAt time.Time `gorm:"type:timestamp with time zone;not null;default:CURRENT_TIMESTAMP"`
}

func (LabObservation) TableName() string {
	return "lab_observations"
}
