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
	specimen := entities.NewSpecimen(
		specimenID,
		patient.ID(),
		time.Date(2024, 3, 10, 15, 0, 0, 0, time.UTC),
		"VN_A",
		"cbc.csv",
		"rec-1",
		1,
		now,
	)
	observations := []*entities.LabObservation{
		entities.NewLabObservation(uuid.New(), specimenID, "HB", "12.4", "12.4", "g/dL", false, now),
		entities.NewLabObservation(uuid.New(), specimenID, "MCV", "79", "79", "fL", false, now),
		entities.NewLabObservation(uuid.New(), specimenID, "HBF", "0.1", "<0.1", "%", true, now),
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

func TestPatientRegistryRepository_RejectsObservationSpecimenMismatch(t *testing.T) {
	db := openPatientRegistryRepositoryTestDB(t)
	ctx := context.Background()
	repo := NewPatientRegistryRepository(db, logger.GetLogger())
	now := time.Now().UTC()

	patient, err := repo.GetOrCreatePatient(ctx, "EXT-200")
	require.NoError(t, err)

	specimenID := uuid.New()
	specimen := entities.NewSpecimen(specimenID, patient.ID(), now, "VN_B", "lab.csv", "r1", 1, now)
	badObservation := entities.NewLabObservation(uuid.New(), uuid.New(), "HB", "13", "13", "g/dL", false, now)

	err = repo.SaveSpecimen(ctx, specimen, []*entities.LabObservation{badObservation})
	require.Error(t, err)

	count, err := repo.CountSpecimens(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}
