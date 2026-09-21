package repositories

import (
	"context"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

// PatientRegistryRepository is the D3 local patient registry store.
//
//go:generate mockgen -source=patient_registry.go -destination=../../mocks/mock_patient_registry_repository.go -package=mocks
type PatientRegistryRepository interface {
	GetPatientByID(ctx context.Context, id uuid.UUID) (*entities.Patient, error)
	GetPatientByExternalID(ctx context.Context, externalPatientID string) (*entities.Patient, error)
	GetOrCreatePatient(ctx context.Context, externalPatientID string) (*entities.Patient, error)
	GetSpecimenByID(ctx context.Context, id uuid.UUID) (*entities.Specimen, error)
	SaveSpecimen(ctx context.Context, specimen *entities.Specimen, observations []*entities.LabObservation) error
	ListObservationsBySpecimenID(ctx context.Context, specimenID uuid.UUID) ([]*entities.LabObservation, error)
	// CountMatchingCohort returns how many patients match LATEST_IN_RANGE
	// specimen selection plus the given panels and conditions (process 3.5).
	CountMatchingCohort(ctx context.Context, criteria types.CohortCriteria) (uint64, error)
	// CountMatchingCohortChunk counts matches among the next limit candidate
	// patients (those with a specimen in the window) whose id is greater than
	// afterPatientID, in patient-id order; uuid.Nil starts from the beginning.
	// It is the resumable form of CountMatchingCohort (decision 0005): summing
	// every chunk equals the unchunked count.
	CountMatchingCohortChunk(
		ctx context.Context,
		criteria types.CohortCriteria,
		afterPatientID uuid.UUID,
		limit int,
	) (types.CohortChunkResult, error)
	CountPatients(ctx context.Context) (int, error)
	CountSpecimens(ctx context.Context) (int, error)
	CountLabObservations(ctx context.Context) (int, error)
}
