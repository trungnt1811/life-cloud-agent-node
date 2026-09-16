package repositories

import (
	"context"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

// PatientRegistryRepository is the D3 local patient registry store.
// Phase 1 is storage-only; cohort query methods land in Phase 5.
//
//go:generate mockgen -source=patient_registry.go -destination=../../mocks/mock_patient_registry_repository.go -package=mocks
type PatientRegistryRepository interface {
	GetPatientByID(ctx context.Context, id uuid.UUID) (*entities.Patient, error)
	GetPatientByExternalID(ctx context.Context, externalPatientID string) (*entities.Patient, error)
	GetOrCreatePatient(ctx context.Context, externalPatientID string) (*entities.Patient, error)
	GetSpecimenByID(ctx context.Context, id uuid.UUID) (*entities.Specimen, error)
	SaveSpecimen(ctx context.Context, specimen *entities.Specimen, observations []*entities.LabObservation) error
	ListObservationsBySpecimenID(ctx context.Context, specimenID uuid.UUID) ([]*entities.LabObservation, error)
	CountPatients(ctx context.Context) (int, error)
	CountSpecimens(ctx context.Context) (int, error)
	CountLabObservations(ctx context.Context) (int, error)
}
