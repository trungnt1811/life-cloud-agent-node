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
		ID:                 specimenID,
		PatientID:          patient.ID(),
		ExternalSpecimenID: "SPEC-100",
		CollectedAt:        time.Date(2024, 3, 10, 15, 0, 0, 0, time.UTC),
		SourceDataset:      "VN_A",
		SourceFile:         "cbc.csv",
		SourceRecordID:     "rec-1",
		SourceRowNumber:    1,
		Now:                now,
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

	// Same real-world specimen (same ExternalSpecimenID) but DIFFERENT file
	// provenance on the retry, as a genuinely re-exported file would have -
	// proving dedup keys off specimen identity, not file/row provenance.
	buildBatch := func(sourceFile, sourceRecordID string) (*entities.Specimen, []*entities.LabObservation) {
		specimen := entities.NewSpecimen(entities.NewSpecimenParams{
			ID:                 uuid.New(),
			PatientID:          patient.ID(),
			ExternalSpecimenID: "SPEC-RETRY-1",
			CollectedAt:        time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
			SourceDataset:      "VN_A",
			SourceFile:         sourceFile,
			SourceRecordID:     sourceRecordID,
			SourceRowNumber:    7,
			Now:                now,
		})
		observations := []*entities.LabObservation{
			entities.NewLabObservation(entities.NewLabObservationParams{
				ID: uuid.New(), SpecimenID: specimen.ID(), FieldCode: "HB", Value: "12.0", RawValue: "12.0", RawUnit: "g/dL", Revision: 1, Now: now,
			}),
		}
		return specimen, observations
	}

	// First attempt: a fresh, client-generated specimen ID, as a real
	// ingestion run would produce.
	firstSpecimen, firstObservations := buildBatch("cbc.csv", "rec-retry-1")
	require.NoError(t, repo.SaveSpecimen(ctx, firstSpecimen, firstObservations))

	// Retry after a simulated crash/timeout: same real specimen, but a NEW
	// client-generated specimen ID and DIFFERENT file/row provenance
	// (nothing on the client remembers the first attempt succeeded, and a
	// real re-export commonly lands under a new filename). This must not
	// create a duplicate specimen, must not fail the transaction, and must
	// not orphan observations against an ID that was never actually written.
	retrySpecimen, retryObservations := buildBatch("cbc_reexport.csv", "rec-retry-1-b")
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

func TestPatientRegistryRepository_SaveSpecimenHigherRevisionWinsRegardlessOfCallOrder(t *testing.T) {
	db := openPatientRegistryRepositoryTestDB(t)
	ctx := context.Background()
	repo := NewPatientRegistryRepository(db, logger.GetLogger())
	now := time.Now().UTC()

	// Same real specimen, but each call simulates a genuinely different
	// re-export file (new SourceFile) - proving revision resolution works
	// across separate ingest runs, not just within one parsed file.
	buildSpecimen := func(patientID uuid.UUID, sourceFile string) *entities.Specimen {
		return entities.NewSpecimen(entities.NewSpecimenParams{
			ID:                 uuid.New(),
			PatientID:          patientID,
			ExternalSpecimenID: "SPEC-REV-1",
			CollectedAt:        time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
			SourceDataset:      "VN_A",
			SourceFile:         sourceFile,
			SourceRecordID:     "rec-rev-1",
			SourceRowNumber:    9,
			Now:                now,
		})
	}
	mcvObservation := func(specimenID uuid.UUID, value string, revision int) *entities.LabObservation {
		return entities.NewLabObservation(entities.NewLabObservationParams{
			ID: uuid.New(), SpecimenID: specimenID, FieldCode: "MCV", Value: value, RawValue: value, RawUnit: "fL", Revision: revision, Now: now,
		})
	}

	t.Run("higher revision ingested after lower revision wins", func(t *testing.T) {
		repositorytest.ResetPostgresRepositoryTables(t, db)
		patient, err := repo.GetOrCreatePatient(ctx, "EXT-401")
		require.NoError(t, err)

		original := buildSpecimen(patient.ID(), "cbc_v1.csv")
		require.NoError(t, repo.SaveSpecimen(ctx, original, []*entities.LabObservation{mcvObservation(original.ID(), "70", 1)}))

		// A later hospital export corrects MCV (70 -> 85) at a higher
		// revision. The correction must win, not be discarded because a
		// row already exists for (specimen, field_code).
		revised := buildSpecimen(patient.ID(), "cbc_v2_corrected.csv")
		require.NoError(t, repo.SaveSpecimen(ctx, revised, []*entities.LabObservation{mcvObservation(revised.ID(), "85", 2)}))

		observations, err := repo.ListObservationsBySpecimenID(ctx, original.ID())
		require.NoError(t, err)
		require.Len(t, observations, 1)
		require.Equal(t, "85", observations[0].Value(), "the higher-revision correction must win")
		require.Equal(t, 2, observations[0].Revision())
	})

	t.Run("lower revision ingested after higher revision does not overwrite", func(t *testing.T) {
		repositorytest.ResetPostgresRepositoryTables(t, db)
		patient, err := repo.GetOrCreatePatient(ctx, "EXT-402")
		require.NoError(t, err)

		corrected := buildSpecimen(patient.ID(), "cbc_v2_corrected.csv")
		require.NoError(t, repo.SaveSpecimen(ctx, corrected, []*entities.LabObservation{mcvObservation(corrected.ID(), "85", 2)}))

		// An operator re-runs ingest on the stale v1 file (e.g. by mistake,
		// or a cron job that re-scans an old directory). The stale, lower
		// revision must NOT clobber the already-corrected value.
		stale := buildSpecimen(patient.ID(), "cbc_v1.csv")
		require.NoError(t, repo.SaveSpecimen(ctx, stale, []*entities.LabObservation{mcvObservation(stale.ID(), "70", 1)}))

		observations, err := repo.ListObservationsBySpecimenID(ctx, corrected.ID())
		require.NoError(t, err)
		require.Len(t, observations, 1)
		require.Equal(t, "85", observations[0].Value(), "a stale lower-revision re-ingest must not overwrite the correction")
		require.Equal(t, 2, observations[0].Revision())
	})
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
		ID:                 specimenID,
		PatientID:          patient.ID(),
		ExternalSpecimenID: "SPEC-200",
		CollectedAt:        now,
		SourceDataset:      "VN_B",
		SourceFile:         "lab.csv",
		SourceRecordID:     "r1",
		SourceRowNumber:    1,
		Now:                now,
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
