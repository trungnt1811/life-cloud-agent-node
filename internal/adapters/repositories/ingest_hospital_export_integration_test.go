package repositories_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/ingestion"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	repositorytest "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/testsupport"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

func openIngestTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := repositorytest.OpenPostgresDB(t)
	repositorytest.RequirePostgresDialect(t, db)
	repositorytest.ResetPostgresRepositoryTables(t, db)
	return db
}

func ingestionFixture(t *testing.T, profile, file string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(
		filepath.Dir(thisFile),
		"..",
		"ingestion",
		"testdata",
		profile,
		file,
	))
}

func TestIngestHospitalExport_EndToEndFixtures(t *testing.T) {
	db := openIngestTestDB(t)
	ctx := context.Background()
	appLogger := logger.GetLogger()
	ucase := usecases.NewIngestHospitalExportUseCase(
		repositories.NewPatientRegistryRepository(db, appLogger),
		ingestion.DefaultAdapters(),
		appLogger,
	)

	cases := []struct {
		profile              string
		wantPatients         int
		wantSpecimens        int
		wantObservations     int
		wantAnomaliesAtLeast int
	}{
		{ingestion.ProfileVNA, 4, 4, 36, 1},
		{ingestion.ProfileVNB, 3, 3, 27, 1},
		{ingestion.ProfileVNC, 4, 4, 36, 1},
	}

	for _, tc := range cases {
		t.Run(tc.profile, func(t *testing.T) {
			repositorytest.ResetPostgresRepositoryTables(t, db)
			result, err := ucase.IngestFiles(ctx, tc.profile, []string{ingestionFixture(t, tc.profile, "results.csv")})
			require.NoError(t, err)
			require.GreaterOrEqual(t, result.Anomalies, tc.wantAnomaliesAtLeast)
			require.Equal(t, tc.wantSpecimens, result.SpecimensSaved)
			require.Equal(t, tc.wantObservations, result.ObservationsSaved)

			repo := repositories.NewPatientRegistryRepository(db, appLogger)
			patients, err := repo.CountPatients(ctx)
			require.NoError(t, err)
			require.Equal(t, tc.wantPatients, patients)

			specimens, err := repo.CountSpecimens(ctx)
			require.NoError(t, err)
			require.Equal(t, tc.wantSpecimens, specimens)

			observations, err := repo.CountLabObservations(ctx)
			require.NoError(t, err)
			require.Equal(t, tc.wantObservations, observations)
		})
	}
}
