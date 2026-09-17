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
	revision   int
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
	Revision   int
	CreatedAt  time.Time
}

// NewLabObservationParams are the inputs to NewLabObservation, grouped in a
// struct so same-typed fields (fieldCode/value/rawValue/rawUnit are all
// strings) can't be silently transposed at a call site the way positional
// string arguments can.
type NewLabObservationParams struct {
	ID         uuid.UUID
	SpecimenID uuid.UUID
	FieldCode  string
	Value      string
	Censored   bool
	RawValue   string
	RawUnit    string
	// Revision is the source's own revision/version number for this
	// result. A re-ingested observation only overwrites a stored one when
	// its Revision is >= the stored Revision - "highest revision wins"
	// regardless of which ingest run happens to run last.
	Revision int
	Now      time.Time
}

// NewLabObservation creates a lab observation with normalized string fields.
func NewLabObservation(params NewLabObservationParams) *LabObservation {
	return &LabObservation{
		id:         ensureID(params.ID),
		specimenID: params.SpecimenID,
		fieldCode:  strings.TrimSpace(params.FieldCode),
		value:      strings.TrimSpace(params.Value),
		censored:   params.Censored,
		rawValue:   strings.TrimSpace(params.RawValue),
		rawUnit:    strings.TrimSpace(params.RawUnit),
		revision:   params.Revision,
		createdAt:  ensureTimestamp(params.Now),
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
		revision:   record.Revision,
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
		Revision:   o.revision,
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

func (o *LabObservation) Revision() int {
	if o == nil {
		return 0
	}
	return o.revision
}

func (o *LabObservation) CreatedAt() time.Time {
	if o == nil {
		return time.Time{}
	}
	return o.createdAt
}
