package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	repositorytest "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/testsupport"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

func openPatientRegistryRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := repositorytest.OpenPostgresDB(t)
	repositorytest.RequirePostgresDialect(t, db)
	repositorytest.ResetPostgresRepositoryTables(t, db)
	return db
}

func TestPatientRegistryRepository_PersistAndReadGraph(t *testing.T) {
	db := openPatientRegistryRepositoryTestDB(t)
	ctx := context.Background()
	repo := NewPatientRegistryRepository(db, logger.GetLogger())
	now := time.Now().UTC()

	patient, err := repo.GetOrCreatePatient(ctx, "EXT-100")
	require.NoError(t, err)
	require.NotNil(t, patient)
	require.Equal(t, "EXT-100", patient.ExternalPatientID())

	samePatient, err := repo.GetOrCreatePatient(ctx, "EXT-100")
	require.NoError(t, err)
	require.Equal(t, patient.ID(), samePatient.ID())

	byExternal, err := repo.GetPatientByExternalID(ctx, "EXT-100")
	require.NoError(t, err)
	require.Equal(t, patient.ID(), byExternal.ID())

	byID, err := repo.GetPatientByID(ctx, patient.ID())
	require.NoError(t, err)
	require.Equal(t, "EXT-100", byID.ExternalPatientID())

	specimenID := uuid.New()
	specimen := entities.NewSpecimen(entities.NewSpecimenParams{
		ID:              specimenID,
		PatientID:       patient.ID(),
		CollectedAt:     time.Date(2024, 3, 10, 15, 0, 0, 0, time.UTC),
		SourceDataset:   "VN_A",
		SourceFile:      "cbc.csv",
		SourceRecordID:  "rec-1",
		SourceRowNumber: 1,
		Now:             now,
	})
	observations := []*entities.LabObservation{
		entities.NewLabObservation(entities.NewLabObservationParams{
			ID: uuid.New(), SpecimenID: specimenID, FieldCode: "HB", Value: "12.4", RawValue: "12.4", RawUnit: "g/dL", Now: now,
		}),
		entities.NewLabObservation(entities.NewLabObservationParams{
			ID: uuid.New(), SpecimenID: specimenID, FieldCode: "MCV", Value: "79", RawValue: "79", RawUnit: "fL", Now: now,
		}),
		entities.NewLabObservation(entities.NewLabObservationParams{
			ID: uuid.New(), SpecimenID: specimenID, FieldCode: "HBF", Value: "0.1", RawValue: "<0.1", RawUnit: "%", Censored: true, Now: now,
		}),
	}
	require.NoError(t, repo.SaveSpecimen(ctx, specimen, observations))

	gotSpecimen, err := repo.GetSpecimenByID(ctx, specimenID)
	require.NoError(t, err)
	require.NotNil(t, gotSpecimen)
	require.Equal(t, patient.ID(), gotSpecimen.PatientID())
	require.Equal(t, time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC), gotSpecimen.CollectedAt())
	require.Equal(t, "VN_A", gotSpecimen.SourceDataset())
	require.Equal(t, "cbc.csv", gotSpecimen.SourceFile())
	require.Equal(t, 1, gotSpecimen.SourceRowNumber())
	require.Equal(t, "rec-1", gotSpecimen.SourceRecordID())

	gotObservations, err := repo.ListObservationsBySpecimenID(ctx, specimenID)
	require.NoError(t, err)
	require.Len(t, gotObservations, 3)
	require.Equal(t, "HB", gotObservations[0].FieldCode())
	require.Equal(t, "HBF", gotObservations[1].FieldCode())
	require.Equal(t, "MCV", gotObservations[2].FieldCode())
	require.True(t, gotObservations[1].Censored())
	require.Equal(t, "<0.1", gotObservations[1].RawValue())

	patients, err := repo.CountPatients(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, patients)

	specimens, err := repo.CountSpecimens(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, specimens)

	labs, err := repo.CountLabObservations(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, labs)

	missingPatient, err := repo.GetPatientByID(ctx, uuid.New())
	require.NoError(t, err)
	require.Nil(t, missingPatient)

	missingSpecimen, err := repo.GetSpecimenByID(ctx, uuid.New())
	require.NoError(t, err)
	require.Nil(t, missingSpecimen)
}

func TestPatientRegistryRepository_SaveSpecimenIsIdempotentOnRetry(t *testing.T) {
	db := openPatientRegistryRepositoryTestDB(t)
	ctx := context.Background()
	repo := NewPatientRegistryRepository(db, logger.GetLogger())
	now := time.Now().UTC()

	patient, err := repo.GetOrCreatePatient(ctx, "EXT-300")
	require.NoError(t, err)

	buildBatch := func() (*entities.Specimen, []*entities.LabObservation) {
		specimen := entities.NewSpecimen(entities.NewSpecimenParams{
			ID:              uuid.New(),
			PatientID:       patient.ID(),
			CollectedAt:     time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
			SourceDataset:   "VN_A",
			SourceFile:      "cbc.csv",
			SourceRecordID:  "rec-retry-1",
			SourceRowNumber: 7,
			Now:             now,
		})
		observations := []*entities.LabObservation{
			entities.NewLabObservation(entities.NewLabObservationParams{
				ID: uuid.New(), SpecimenID: specimen.ID(), FieldCode: "HB", Value: "12.0", RawValue: "12.0", RawUnit: "g/dL", Now: now,
			}),
		}
		return specimen, observations
	}

	// First attempt: a fresh, client-generated specimen ID, as a real
	// ingestion run would produce.
	firstSpecimen, firstObservations := buildBatch()
	require.NoError(t, repo.SaveSpecimen(ctx, firstSpecimen, firstObservations))

	// Retry after a simulated crash/timeout: same source row, but a NEW
	// client-generated specimen ID (nothing on the client remembers the
	// first attempt succeeded). This must not create a duplicate specimen,
	// must not fail the transaction, and must not orphan observations
	// against an ID that was never actually written.
	retrySpecimen, retryObservations := buildBatch()
	require.NotEqual(t, firstSpecimen.ID(), retrySpecimen.ID())
	require.NoError(t, repo.SaveSpecimen(ctx, retrySpecimen, retryObservations))

	specimens, err := repo.CountSpecimens(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, specimens, "retry must not create a second specimen row")

	labs, err := repo.CountLabObservations(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, labs, "retry must not duplicate lab observations")

	canonical, err := repo.GetSpecimenByID(ctx, firstSpecimen.ID())
	require.NoError(t, err)
	require.NotNil(t, canonical, "observations must attach to the specimen actually persisted")
}

func TestPatientRegistryRepository_RejectsObservationSpecimenMismatch(t *testing.T) {
	db := openPatientRegistryRepositoryTestDB(t)
	ctx := context.Background()
	repo := NewPatientRegistryRepository(db, logger.GetLogger())
	now := time.Now().UTC()

	patient, err := repo.GetOrCreatePatient(ctx, "EXT-200")
	require.NoError(t, err)

	specimenID := uuid.New()
	specimen := entities.NewSpecimen(entities.NewSpecimenParams{
		ID:              specimenID,
		PatientID:       patient.ID(),
		CollectedAt:     now,
		SourceDataset:   "VN_B",
		SourceFile:      "lab.csv",
		SourceRecordID:  "r1",
		SourceRowNumber: 1,
		Now:             now,
	})
	badObservation := entities.NewLabObservation(entities.NewLabObservationParams{
		ID: uuid.New(), SpecimenID: uuid.New(), FieldCode: "HB", Value: "13", RawValue: "13", RawUnit: "g/dL", Now: now,
	})

	err = repo.SaveSpecimen(ctx, specimen, []*entities.LabObservation{badObservation})
	require.Error(t, err)

	count, err := repo.CountSpecimens(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}
