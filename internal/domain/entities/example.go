package entities

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ExampleEntity is the domain representation of an example record.
type ExampleEntity struct {
	id          uuid.UUID
	name        string
	description string
	createdAt   time.Time
	updatedAt   time.Time
}

// ExampleRecord is a persistence snapshot used at repository boundaries.
type ExampleRecord struct {
	ID          uuid.UUID
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewExampleEntity creates a new domain entity with normalized fields.
func NewExampleEntity(id uuid.UUID, name, description string, now time.Time) *ExampleEntity {
	if id == uuid.Nil {
		id = uuid.New()
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &ExampleEntity{
		id:          id,
		name:        strings.TrimSpace(name),
		description: strings.TrimSpace(description),
		createdAt:   now,
		updatedAt:   now,
	}
}

// NewExampleEntityFromRecord hydrates a domain entity from persistence data.
func NewExampleEntityFromRecord(record ExampleRecord) *ExampleEntity {
	if record.ID == uuid.Nil {
		return nil
	}
	return &ExampleEntity{
		id:          record.ID,
		name:        record.Name,
		description: record.Description,
		createdAt:   record.CreatedAt,
		updatedAt:   record.UpdatedAt,
	}
}

// Record returns a persistence snapshot.
func (e *ExampleEntity) Record() ExampleRecord {
	if e == nil {
		return ExampleRecord{}
	}
	return ExampleRecord{
		ID:          e.id,
		Name:        e.name,
		Description: e.description,
		CreatedAt:   e.createdAt,
		UpdatedAt:   e.updatedAt,
	}
}

// UpdateDetails changes editable example fields and advances the update timestamp.
func (e *ExampleEntity) UpdateDetails(name, description string, now time.Time) {
	if e == nil {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	e.name = strings.TrimSpace(name)
	e.description = strings.TrimSpace(description)
	e.updatedAt = now
}

func (e *ExampleEntity) ID() uuid.UUID {
	if e == nil {
		return uuid.Nil
	}
	return e.id
}

func (e *ExampleEntity) Name() string {
	if e == nil {
		return ""
	}
	return e.name
}

func (e *ExampleEntity) Description() string {
	if e == nil {
		return ""
	}
	return e.description
}

func (e *ExampleEntity) CreatedAt() time.Time {
	if e == nil {
		return time.Time{}
	}
	return e.createdAt
}

func (e *ExampleEntity) UpdatedAt() time.Time {
	if e == nil {
		return time.Time{}
	}
	return e.updatedAt
}
