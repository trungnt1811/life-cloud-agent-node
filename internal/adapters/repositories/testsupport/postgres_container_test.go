package testsupport

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenPostgresDBRunsMigrations(t *testing.T) {
	db := OpenPostgresDB(t)
	RequirePostgresDialect(t, db)

	var exists bool
	err := db.Raw("SELECT to_regclass(?) IS NOT NULL", "public.examples").Scan(&exists).Error
	require.NoError(t, err)
	require.True(t, exists, "expected migrated examples table")
}

func TestResetPostgresRepositoryTablesTruncatesMigratedTables(t *testing.T) {
	db := OpenPostgresDB(t)

	require.NoError(t, db.Exec(`
		INSERT INTO examples (
			id,
			name,
			description,
			created_at,
			updated_at
		) VALUES (
			'00000000-0000-0000-0000-000000000001',
			'repository-test-example',
			'created by repository tests',
			NOW(),
			NOW()
		)
	`).Error)

	var count int64
	require.NoError(t, db.Table("examples").Count(&count).Error)
	require.NotZero(t, count)

	ResetPostgresRepositoryTables(t, db)

	require.NoError(t, db.Table("examples").Count(&count).Error)
	require.Zero(t, count)
}

func TestPostgresRepositoryEphemeralDatabaseNameIsSafeAndBounded(t *testing.T) {
	name := postgresRepositoryEphemeralDatabaseName("Go Backend Template Repository Tests With Long Name !!!")

	require.LessOrEqual(t, len(name), 63)
	require.Regexp(t, regexp.MustCompile(`^[a-z_][a-z0-9_]*$`), name)
}

func TestPostgresRepositoryPackageDatabaseNameIsStableAndBounded(t *testing.T) {
	name := postgresRepositoryPackageDatabaseName("/repo/internal/adapters/repositories/example", "123456789abc")

	require.Equal(t, name, postgresRepositoryPackageDatabaseName("/repo/internal/adapters/repositories/example", "123456789abc"))
	require.LessOrEqual(t, len(name), 63)
	require.Regexp(t, regexp.MustCompile(`^[a-z_][a-z0-9_]*$`), name)
}

func TestPostgresConnectionStringForDatabaseSwitchesPath(t *testing.T) {
	got, err := postgresConnectionStringForDatabase("postgres://postgres:postgres@127.0.0.1:5432/base?sslmode=disable", "target_db")

	require.NoError(t, err)
	require.Equal(t, "postgres://postgres:postgres@127.0.0.1:5432/target_db?sslmode=disable", got)
}

func TestRepositoryMigrationFingerprintIsStable(t *testing.T) {
	path, err := repositoryMigrationScriptsPath()
	require.NoError(t, err)

	first, err := repositoryMigrationFingerprint(path)
	require.NoError(t, err)
	second, err := repositoryMigrationFingerprint(path)
	require.NoError(t, err)

	require.Len(t, first, 12)
	require.Equal(t, first, second)
}

func TestPostgresRepositoryReuseScopeDefaultsToTest(t *testing.T) {
	t.Setenv(postgresRepositoryTestScopeEnv, "")

	require.Equal(t, postgresRepositoryReuseScopeTest, postgresRepositoryReuseScope())
}

func TestPostgresRepositoryReuseScopeAllowsPackage(t *testing.T) {
	t.Setenv(postgresRepositoryTestScopeEnv, "package")

	require.Equal(t, postgresRepositoryReuseScopePackage, postgresRepositoryReuseScope())
}
