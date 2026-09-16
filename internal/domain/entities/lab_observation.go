package entities

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// LabObservation is one measurement on a specimen (schema v1 field_code).
type LabObservation struct {
	id         uuid.UUID
	specimenID uuid.UUID
	fieldCode  string
	value      string
	censored   bool
	rawValue   string
	rawUnit    string
	createdAt  time.Time
}

// LabObservationRecord is a persistence snapshot used at repository boundaries.
type LabObservationRecord struct {
	ID         uuid.UUID
	SpecimenID uuid.UUID
	FieldCode  string
	Value      string
	Censored   bool
	RawValue   string
	RawUnit    string
	CreatedAt  time.Time
}

// NewLabObservation creates a lab observation with normalized string fields.
func NewLabObservation(
	id, specimenID uuid.UUID,
	fieldCode, value, rawValue, rawUnit string,
	censored bool,
	now time.Time,
) *LabObservation {
	if id == uuid.Nil {
		id = uuid.New()
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &LabObservation{
		id:         id,
		specimenID: specimenID,
		fieldCode:  strings.TrimSpace(fieldCode),
		value:      strings.TrimSpace(value),
		censored:   censored,
		rawValue:   strings.TrimSpace(rawValue),
		rawUnit:    strings.TrimSpace(rawUnit),
		createdAt:  now.UTC(),
	}
}

// NewLabObservationFromRecord hydrates a lab observation from persistence data.
func NewLabObservationFromRecord(record LabObservationRecord) *LabObservation {
	if record.ID == uuid.Nil {
		return nil
	}
	return &LabObservation{
		id:         record.ID,
		specimenID: record.SpecimenID,
		fieldCode:  record.FieldCode,
		value:      record.Value,
		censored:   record.Censored,
		rawValue:   record.RawValue,
		rawUnit:    record.RawUnit,
		createdAt:  record.CreatedAt,
	}
}

// Record returns a persistence snapshot.
func (o *LabObservation) Record() LabObservationRecord {
	if o == nil {
		return LabObservationRecord{}
	}
	return LabObservationRecord{
		ID:         o.id,
		SpecimenID: o.specimenID,
		FieldCode:  o.fieldCode,
		Value:      o.value,
		Censored:   o.censored,
		RawValue:   o.rawValue,
		RawUnit:    o.rawUnit,
		CreatedAt:  o.createdAt,
	}
}

func (o *LabObservation) ID() uuid.UUID {
	if o == nil {
		return uuid.Nil
	}
	return o.id
}

func (o *LabObservation) SpecimenID() uuid.UUID {
	if o == nil {
		return uuid.Nil
	}
	return o.specimenID
}

func (o *LabObservation) FieldCode() string {
	if o == nil {
		return ""
	}
	return o.fieldCode
}

func (o *LabObservation) Value() string {
	if o == nil {
		return ""
	}
	return o.value
}

func (o *LabObservation) Censored() bool {
	if o == nil {
		return false
	}
	return o.censored
}

func (o *LabObservation) RawValue() string {
	if o == nil {
		return ""
	}
	return o.rawValue
}

func (o *LabObservation) RawUnit() string {
	if o == nil {
		return ""
	}
	return o.rawUnit
}

func (o *LabObservation) CreatedAt() time.Time {
	if o == nil {
		return time.Time{}
	}
	return o.createdAt
}
