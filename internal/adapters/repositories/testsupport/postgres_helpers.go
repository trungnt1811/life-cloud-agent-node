package testsupport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

var postgresRepositoryDBCounter atomic.Uint64

func postgresConnectionStringForDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse PostgreSQL repository test DSN: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

func postgresRepositoryContainerName(cfg postgresRepositoryTestConfig) string {
	if configured := strings.TrimSpace(os.Getenv(postgresRepositoryTestNameEnv)); configured != "" {
		return configured
	}
	hash := shortHash(cfg.image + "|" + cfg.database + "|" + cfg.user)
	return defaultPostgresRepositoryTestNamePrefix + "-" + hash
}

func postgresRepositoryTemplateDatabaseName(fingerprint string) string {
	return "lwrt_template_" + fingerprint
}

func postgresRepositoryPackageDatabaseName(scopeKey, fingerprint string) string {
	return "lwrt_pkg_" + shortHash(scopeKey) + "_" + fingerprint
}

func postgresRepositoryEphemeralDatabaseName(base string) string {
	prefix := sanitizePostgresIdentifier(base)
	if len(prefix) > 24 {
		prefix = prefix[:24]
	}
	return prefix + "_" +
		strconv.Itoa(os.Getpid()) + "_" +
		strconv.FormatUint(postgresRepositoryDBCounter.Add(1), 36) + "_" +
		strconv.FormatInt(time.Now().UnixNano(), 36)
}

func repositoryMigrationFingerprint(path string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(path, func(filePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			return nil
		}
		rel, err := filepath.Rel(path, filePath)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		hash.Write([]byte(filepath.ToSlash(rel)))
		hash.Write([]byte{0})
		hash.Write(content)
		hash.Write([]byte{0})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("fingerprint PostgreSQL repository migrations: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil))[:12], nil
}

func sanitizePostgresIdentifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "repository_test"
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = "db_" + out
	}
	return out
}

func postgresRepositoryReuseScope() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(postgresRepositoryTestScopeEnv))) {
	case postgresRepositoryReuseScopePackage:
		return postgresRepositoryReuseScopePackage
	default:
		return postgresRepositoryReuseScopeTest
	}
}

func postgresRepositoryCallerScopeKey() string {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.File, "_test.go") {
			return filepath.ToSlash(filepath.Dir(frame.File))
		}
		if !more {
			break
		}
	}
	return "repository_tests"
}

func quotePostgresIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}

func resetPostgresRepositoryTables(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("reset PostgreSQL repository tables: nil DB")
	}
	tables, err := postgresBaseTables(ctx, db)
	if err != nil {
		return fmt.Errorf("list PostgreSQL repository tables: %w", err)
	}
	if len(tables) == 0 {
		return nil
	}
	quoted := make([]string, 0, len(tables))
	for _, table := range tables {
		quoted = append(quoted, quotePostgresIdentifier(table))
	}
	if err := db.WithContext(ctx).Exec("TRUNCATE TABLE " + strings.Join(quoted, ", ") + " RESTART IDENTITY CASCADE").Error; err != nil {
		return fmt.Errorf("truncate PostgreSQL repository tables: %w", err)
	}
	return nil
}

func postgresBaseTables(ctx context.Context, db *gorm.DB) ([]string, error) {
	var tables []string
	err := db.WithContext(ctx).Raw(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`).Scan(&tables).Error
	if err != nil {
		return nil, err
	}
	return tables, nil
}

func logPostgresVersion(logf func(string, ...any), db *gorm.DB) {
	var version string
	if err := db.Raw("SELECT version()").Scan(&version).Error; err != nil {
		logPostgresTestf(logf, "read PostgreSQL version: %v", err)
		return
	}
	logPostgresTestf(logf, "repository PostgreSQL test DB ready: %s", version)
}

func repositoryMigrationScriptsPath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve repository migration scripts path: runtime caller unavailable")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "internal", "adapters", "postgres", "scripts"))
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("resolve repository migration scripts path %s: %w", path, err)
	}
	return path, nil
}

func logPostgresTestf(logf func(string, ...any), format string, args ...any) {
	if logf != nil {
		logf(format, args...)
	}
}
