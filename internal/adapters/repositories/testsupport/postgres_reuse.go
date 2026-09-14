package testsupport

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"

	apppostgres "github.com/lifenetwork-ai/go-backend-template/internal/adapters/postgres"
)

var (
	reusablePostgresHarnessMu sync.Mutex
	reusablePostgresHarnesses = make(map[string]*reusablePostgresHarness)
)

type reusablePostgresHarness struct {
	adminDB     *gorm.DB
	adminDSN    string
	fingerprint string
	mu          sync.Mutex
	packageDBs  map[string]*reusablePostgresPackageDB
	templateDB  string
}

type reusablePostgresPackageDB struct {
	db *gorm.DB
	mu sync.Mutex
}

func openReusablePostgresDB(ctx context.Context, cfg postgresRepositoryTestConfig, logf func(string, ...any)) (*gorm.DB, func(), error) {
	harness, err := reusablePostgresHarnessFor(ctx, cfg, logf)
	if err != nil {
		return nil, nil, err
	}
	return harness.openTestDB(ctx, cfg, logf)
}

func reusablePostgresHarnessFor(ctx context.Context, cfg postgresRepositoryTestConfig, logf func(string, ...any)) (*reusablePostgresHarness, error) {
	key := reusablePostgresHarnessKey(cfg)
	reusablePostgresHarnessMu.Lock()
	defer reusablePostgresHarnessMu.Unlock()

	if harness, ok := reusablePostgresHarnesses[key]; ok {
		logPostgresTestf(logf, "reusing in-process PostgreSQL repository harness: %s", cfg.containerName)
		return harness, nil
	}

	options := []testcontainers.ContainerCustomizer{
		tcpostgres.WithDatabase(cfg.database),
		tcpostgres.WithUsername(cfg.user),
		tcpostgres.WithPassword(cfg.password),
		tcpostgres.BasicWaitStrategies(),
		testcontainers.CustomizeRequestOption(func(req *testcontainers.GenericContainerRequest) error {
			req.Name = cfg.containerName
			req.Reuse = true
			return nil
		}),
	}
	container, err := tcpostgres.Run(ctx, cfg.image, options...)
	if err != nil {
		return nil, fmt.Errorf("start reusable PostgreSQL repository test container: %w", err)
	}
	adminDSN, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("build PostgreSQL repository test admin DSN: %w", err)
	}
	adminDB, _, err := openGormPostgresDB(ctx, adminDSN, nil, logf)
	if err != nil {
		return nil, err
	}
	scriptsPath, err := repositoryMigrationScriptsPath()
	if err != nil {
		return nil, err
	}
	fingerprint, err := repositoryMigrationFingerprint(scriptsPath)
	if err != nil {
		return nil, err
	}
	templateDB := postgresRepositoryTemplateDatabaseName(fingerprint)
	if err := ensureReusableTemplateDatabase(ctx, adminDB, adminDSN, cfg.user, templateDB, scriptsPath, logf); err != nil {
		return nil, err
	}

	harness := &reusablePostgresHarness{
		adminDB:     adminDB,
		adminDSN:    adminDSN,
		fingerprint: fingerprint,
		packageDBs:  make(map[string]*reusablePostgresPackageDB),
		templateDB:  templateDB,
	}
	reusablePostgresHarnesses[key] = harness
	return harness, nil
}

func (h *reusablePostgresHarness) openTestDB(ctx context.Context, cfg postgresRepositoryTestConfig, logf func(string, ...any)) (*gorm.DB, func(), error) {
	if cfg.reuseScope == postgresRepositoryReuseScopePackage {
		db, err := h.openPackageDB(ctx, cfg, logf)
		if err != nil {
			return nil, nil, err
		}
		cleanup := func() {}
		logPostgresVersion(logf, db)
		return db, cleanup, nil
	}

	testDB := postgresRepositoryEphemeralDatabaseName(cfg.database)
	if err := createPostgresDatabaseFromTemplate(ctx, h.adminDB, testDB, h.templateDB, cfg.user); err != nil {
		return nil, nil, fmt.Errorf("create reusable PostgreSQL repository test DB %s: %w", testDB, err)
	}
	testDSN, err := postgresConnectionStringForDatabase(h.adminDSN, testDB)
	if err != nil {
		_ = dropPostgresDatabase(context.Background(), h.adminDB, testDB)
		return nil, nil, err
	}
	db, closeDB, err := openGormPostgresDB(ctx, testDSN, nil, logf)
	if err != nil {
		_ = dropPostgresDatabase(context.Background(), h.adminDB, testDB)
		return nil, nil, err
	}

	cleanup := func() {
		closeDB()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := dropPostgresDatabase(cleanupCtx, h.adminDB, testDB); err != nil {
			logPostgresTestf(logf, "drop reusable PostgreSQL repository test DB %s: %v", testDB, err)
		}
	}
	logPostgresVersion(logf, db)
	return db, cleanup, nil
}

func (h *reusablePostgresHarness) openPackageDB(ctx context.Context, cfg postgresRepositoryTestConfig, logf func(string, ...any)) (*gorm.DB, error) {
	name := postgresRepositoryPackageDatabaseName(cfg.scopeKey, h.fingerprint)
	h.mu.Lock()
	defer h.mu.Unlock()

	packageDB, ok := h.packageDBs[name]
	if !ok {
		if err := ensureReusableTestDatabase(ctx, h.adminDB, name, h.templateDB, cfg.user); err != nil {
			return nil, err
		}
		dsn, err := postgresConnectionStringForDatabase(h.adminDSN, name)
		if err != nil {
			return nil, err
		}
		db, _, err := openGormPostgresDB(ctx, dsn, nil, logf)
		if err != nil {
			return nil, err
		}
		packageDB = &reusablePostgresPackageDB{
			db: db,
		}
		h.packageDBs[name] = packageDB
	}

	packageDB.mu.Lock()
	defer packageDB.mu.Unlock()
	if err := resetPostgresRepositoryTables(ctx, packageDB.db); err != nil {
		return nil, err
	}
	return packageDB.db, nil
}

func reusablePostgresHarnessKey(cfg postgresRepositoryTestConfig) string {
	return cfg.image + "|" + cfg.database + "|" + cfg.user + "|" + cfg.containerName
}

func ensureReusableTemplateDatabase(ctx context.Context, adminDB *gorm.DB, adminDSN, owner, templateDB, scriptsPath string, logf func(string, ...any)) error {
	return withPostgresRepositoryTemplateLock(ctx, adminDB, func() error {
		exists, err := postgresDatabaseExists(ctx, adminDB, templateDB)
		if err != nil {
			return err
		}
		if exists {
			logPostgresTestf(logf, "reusing migrated PostgreSQL repository template DB: %s", templateDB)
			return nil
		}
		if err := createPostgresDatabase(ctx, adminDB, templateDB, owner); err != nil {
			return fmt.Errorf("create reusable PostgreSQL repository template DB %s: %w", templateDB, err)
		}
		templateDSN, err := postgresConnectionStringForDatabase(adminDSN, templateDB)
		if err != nil {
			_ = dropPostgresDatabase(context.Background(), adminDB, templateDB)
			return err
		}
		templateDBConn, closeTemplate, err := openGormPostgresDB(ctx, templateDSN, nil, logf)
		if err != nil {
			_ = dropPostgresDatabase(context.Background(), adminDB, templateDB)
			return err
		}
		migrationErr := apppostgres.RunMigrations(templateDBConn, scriptsPath)
		closeTemplate()
		if migrationErr != nil {
			_ = dropPostgresDatabase(context.Background(), adminDB, templateDB)
			return fmt.Errorf("run PostgreSQL repository template migrations: %w", migrationErr)
		}
		if err := adminDB.WithContext(ctx).Exec("ALTER DATABASE " + quotePostgresIdentifier(templateDB) + " WITH is_template TRUE").Error; err != nil {
			_ = dropPostgresDatabase(context.Background(), adminDB, templateDB)
			return fmt.Errorf("mark reusable PostgreSQL repository template DB %s: %w", templateDB, err)
		}
		logPostgresTestf(logf, "created migrated PostgreSQL repository template DB: %s", templateDB)
		return nil
	})
}

func withPostgresRepositoryTemplateLock(ctx context.Context, db *gorm.DB, fn func() error) error {
	if err := db.WithContext(ctx).Exec("SELECT pg_advisory_lock(?, ?)", postgresRepositoryTemplateLockKey1, postgresRepositoryTemplateLockKey2).Error; err != nil {
		return fmt.Errorf("acquire PostgreSQL repository template lock: %w", err)
	}
	fnErr := fn()
	unlockErr := db.WithContext(ctx).Exec("SELECT pg_advisory_unlock(?, ?)", postgresRepositoryTemplateLockKey1, postgresRepositoryTemplateLockKey2).Error
	if fnErr != nil {
		return fnErr
	}
	if unlockErr != nil {
		return fmt.Errorf("release PostgreSQL repository template lock: %w", unlockErr)
	}
	return nil
}

func postgresDatabaseExists(ctx context.Context, db *gorm.DB, name string) (bool, error) {
	var exists bool
	err := db.WithContext(ctx).Raw("SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = ?)", name).Scan(&exists).Error
	return exists, err
}

func ensureReusableTestDatabase(ctx context.Context, adminDB *gorm.DB, name, templateDB, owner string) error {
	return withPostgresRepositoryTemplateLock(ctx, adminDB, func() error {
		exists, err := postgresDatabaseExists(ctx, adminDB, name)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		if err := createPostgresDatabaseFromTemplate(ctx, adminDB, name, templateDB, owner); err != nil {
			return fmt.Errorf("create reusable PostgreSQL repository package DB %s: %w", name, err)
		}
		return nil
	})
}

func createPostgresDatabaseFromTemplate(ctx context.Context, db *gorm.DB, name, templateName, owner string) error {
	return db.WithContext(ctx).Exec(
		"CREATE DATABASE " + quotePostgresIdentifier(name) +
			" WITH TEMPLATE " + quotePostgresIdentifier(templateName) +
			" OWNER " + quotePostgresIdentifier(owner),
	).Error
}

func createPostgresDatabase(ctx context.Context, db *gorm.DB, name, owner string) error {
	return db.WithContext(ctx).Exec(
		"CREATE DATABASE " + quotePostgresIdentifier(name) +
			" OWNER " + quotePostgresIdentifier(owner),
	).Error
}

func dropPostgresDatabase(ctx context.Context, db *gorm.DB, name string) error {
	return db.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + quotePostgresIdentifier(name) + " WITH (FORCE)").Error
}
