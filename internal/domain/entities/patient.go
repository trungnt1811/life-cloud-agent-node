package entities

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Patient is a registry person identified by a hospital-local external ID.
type Patient struct {
	id                uuid.UUID
	externalPatientID string
	createdAt         time.Time
}

// PatientRecord is a persistence snapshot used at repository boundaries.
type PatientRecord struct {
	ID                uuid.UUID
	ExternalPatientID string
	CreatedAt         time.Time
}

// NewPatient creates a patient with a normalized external identifier.
func NewPatient(id uuid.UUID, externalPatientID string, now time.Time) *Patient {
	if id == uuid.Nil {
		id = uuid.New()
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &Patient{
		id:                id,
		externalPatientID: strings.TrimSpace(externalPatientID),
		createdAt:         now.UTC(),
	}
}

// NewPatientFromRecord hydrates a patient from persistence data.
func NewPatientFromRecord(record PatientRecord) *Patient {
	if record.ID == uuid.Nil {
		return nil
	}
	return &Patient{
		id:                record.ID,
		externalPatientID: record.ExternalPatientID,
		createdAt:         record.CreatedAt,
	}
}

// Record returns a persistence snapshot.
func (p *Patient) Record() PatientRecord {
	if p == nil {
		return PatientRecord{}
	}
	return PatientRecord{
		ID:                p.id,
		ExternalPatientID: p.externalPatientID,
		CreatedAt:         p.createdAt,
	}
}

func (p *Patient) ID() uuid.UUID {
	if p == nil {
		return uuid.Nil
	}
	return p.id
}

func (p *Patient) ExternalPatientID() string {
	if p == nil {
		return ""
	}
	return p.externalPatientID
}

func (p *Patient) CreatedAt() time.Time {
	if p == nil {
		return time.Time{}
	}
	return p.createdAt
}
