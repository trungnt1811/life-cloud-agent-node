package testsupport

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	apppostgres "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/postgres"
)

const (
	postgresRepositoryTestEnvPrefix = "POSTGRES_REPOSITORY_TEST_"
	postgresRepositoryTestImageEnv  = "POSTGRES_REPOSITORY_TEST_IMAGE"
	postgresRepositoryTestDBEnv     = "POSTGRES_REPOSITORY_TEST_DB"
	postgresRepositoryTestUserEnv   = "POSTGRES_REPOSITORY_TEST_USER"
	postgresRepositoryTestSkipEnv   = "POSTGRES_REPOSITORY_TEST_SKIP_CONTAINER"
	postgresRepositoryTestReuseEnv  = "POSTGRES_REPOSITORY_TEST_REUSE_CONTAINER"
	postgresRepositoryTestScopeEnv  = "POSTGRES_REPOSITORY_TEST_REUSE_SCOPE"
	postgresRepositoryTestNameEnv   = "POSTGRES_REPOSITORY_TEST_CONTAINER_NAME"

	defaultPostgresRepositoryTestImage      = "postgres:15-alpine"
	defaultPostgresRepositoryTestDB         = "life_cloud_agent_node_repository_tests"
	defaultPostgresRepositoryTestUser       = "postgres"
	defaultPostgresRepositoryTestPass       = "postgres"
	defaultPostgresRepositoryTestNamePrefix = "life-cloud-agent-node-repository-tests-postgres"

	postgresRepositoryTemplateLockKey1 = 724101
	postgresRepositoryTemplateLockKey2 = 1

	postgresRepositoryReuseScopeTest    = "test"
	postgresRepositoryReuseScopePackage = "package"
)

var postgresRepositoryTestCredentialEnv = postgresRepositoryTestEnvPrefix + "PASS" + "WORD"

// OpenPostgresDB starts an isolated PostgreSQL container and applies the real
// migration scripts. Repository tests use this to prove production lock, CTE,
// RETURNING, JSONB, and ON CONFLICT behavior.
func OpenPostgresDB(t *testing.T) *gorm.DB {
	t.Helper()

	if envBool(postgresRepositoryTestSkipEnv) {
		t.Skipf("%s=true; skipping PostgreSQL container repository test", postgresRepositoryTestSkipEnv)
	}

	cfg := postgresRepositoryTestConfig{
		image:          envString(postgresRepositoryTestImageEnv, defaultPostgresRepositoryTestImage),
		database:       envString(postgresRepositoryTestDBEnv, defaultPostgresRepositoryTestDB),
		user:           envString(postgresRepositoryTestUserEnv, defaultPostgresRepositoryTestUser),
		password:       envString(postgresRepositoryTestCredentialEnv, defaultPostgresRepositoryTestPass),
		reuseContainer: envBool(postgresRepositoryTestReuseEnv),
		reuseScope:     postgresRepositoryReuseScope(),
		scopeKey:       postgresRepositoryCallerScopeKey(),
	}
	cfg.containerName = postgresRepositoryContainerName(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	db, cleanup, err := openPostgresDB(ctx, cfg, t.Logf)
	if err != nil {
		cancel()
		t.Fatalf("open PostgreSQL repository test DB: %v", err)
	}
	t.Cleanup(func() {
		cleanup()
		cancel()
	})
	return db
}

// OpenPostgresDBForTestMain starts the same repository PostgreSQL harness for
// package-level integration TestMain setup, where no *testing.T is available.
func OpenPostgresDBForTestMain(ctx context.Context) (*gorm.DB, func(), error) {
	if envBool(postgresRepositoryTestSkipEnv) {
		return nil, nil, fmt.Errorf("%s=true; cannot skip from TestMain PostgreSQL setup", postgresRepositoryTestSkipEnv)
	}
	cfg := postgresRepositoryTestConfig{
		image:          envString(postgresRepositoryTestImageEnv, defaultPostgresRepositoryTestImage),
		database:       envString(postgresRepositoryTestDBEnv, defaultPostgresRepositoryTestDB),
		user:           envString(postgresRepositoryTestUserEnv, defaultPostgresRepositoryTestUser),
		password:       envString(postgresRepositoryTestCredentialEnv, defaultPostgresRepositoryTestPass),
		reuseContainer: envBool(postgresRepositoryTestReuseEnv),
		reuseScope:     postgresRepositoryReuseScope(),
		scopeKey:       postgresRepositoryCallerScopeKey(),
	}
	cfg.containerName = postgresRepositoryContainerName(cfg)
	return openPostgresDB(ctx, cfg, func(format string, args ...any) {
		fmt.Printf(format+"\n", args...)
	})
}

func openPostgresDB(ctx context.Context, cfg postgresRepositoryTestConfig, logf func(string, ...any)) (*gorm.DB, func(), error) {
	logPostgresTestf(logf, "starting repository PostgreSQL test container: image=%s database=%s user=%s reuse=%t", cfg.image, cfg.database, cfg.user, cfg.reuseContainer)
	if cfg.reuseContainer && !envBool("TESTCONTAINERS_RYUK_DISABLED") {
		logPostgresTestf(logf, "repository PostgreSQL reusable container is enabled without TESTCONTAINERS_RYUK_DISABLED=true; the container may not survive across go test processes")
	}

	if cfg.reuseContainer {
		return openReusablePostgresDB(ctx, cfg, logf)
	}

	options := []testcontainers.ContainerCustomizer{
		tcpostgres.WithDatabase(cfg.database),
		tcpostgres.WithUsername(cfg.user),
		tcpostgres.WithPassword(cfg.password),
		tcpostgres.BasicWaitStrategies(),
	}

	container, err := tcpostgres.Run(ctx, cfg.image, options...)
	if err != nil {
		return nil, nil, fmt.Errorf("start PostgreSQL repository test container: %w", err)
	}
	cleanupContainer := func() {
		if cfg.reuseContainer {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := container.Terminate(cleanupCtx); err != nil {
			logPostgresTestf(logf, "terminate PostgreSQL repository test container: %v", err)
		}
	}

	return openFreshPostgresDB(ctx, container, cleanupContainer, logf)
}

func openFreshPostgresDB(ctx context.Context, container *tcpostgres.PostgresContainer, cleanupContainer func(), logf func(string, ...any)) (*gorm.DB, func(), error) {
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cleanupContainer()
		return nil, nil, fmt.Errorf("build PostgreSQL repository test DSN: %w", err)
	}

	db, cleanup, err := openGormPostgresDB(ctx, dsn, cleanupContainer, logf)
	if err != nil {
		return nil, nil, err
	}

	scriptsPath, err := repositoryMigrationScriptsPath()
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	if err := apppostgres.RunMigrations(db, scriptsPath); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("run PostgreSQL repository test migrations: %w", err)
	}
	logPostgresVersion(logf, db)
	return db, cleanup, nil
}

func openGormPostgresDB(ctx context.Context, dsn string, cleanupContainer func(), logf func(string, ...any)) (*gorm.DB, func(), error) {
	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
	if err != nil {
		if cleanupContainer != nil {
			cleanupContainer()
		}
		return nil, nil, fmt.Errorf("open PostgreSQL repository test DB: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		if cleanupContainer != nil {
			cleanupContainer()
		}
		return nil, nil, fmt.Errorf("resolve PostgreSQL repository test sql.DB: %w", err)
	}
	cleanup := func() {
		if err := sqlDB.Close(); err != nil {
			logPostgresTestf(logf, "close PostgreSQL repository test DB: %v", err)
		}
		if cleanupContainer != nil {
			cleanupContainer()
		}
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("ping PostgreSQL repository test DB: %w", err)
	}
	return db, cleanup, nil
}

// ResetPostgresRepositoryTables truncates all public base tables after
// migrations. Use it when a package shares a PostgreSQL test DB across tests.
func ResetPostgresRepositoryTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := resetPostgresRepositoryTables(context.Background(), db); err != nil {
		t.Fatal(err)
	}
}

type postgresRepositoryTestConfig struct {
	image          string
	database       string
	user           string
	password       string
	reuseContainer bool
	reuseScope     string
	containerName  string
	scopeKey       string
}

func envString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// RequirePostgresDialect fails the test when the supplied DB is not backed by
// the production repository dialect.
func RequirePostgresDialect(t *testing.T, db *gorm.DB) {
	t.Helper()

	if db == nil || db.Dialector == nil {
		t.Fatal("expected PostgreSQL DB, got nil DB or dialector")
	}
	if db.Dialector.Name() != "postgres" {
		t.Fatalf("expected PostgreSQL DB, got %q", db.Dialector.Name())
	}
}
