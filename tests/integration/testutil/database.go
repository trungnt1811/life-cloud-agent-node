package testutil

import (
	"fmt"
	"testing"

	"gorm.io/gorm"
)

var sharedDB *gorm.DB

// SetDB sets the shared DB handle for integration tests.
func SetDB(db *gorm.DB) {
	sharedDB = db
}

// GetDB returns the shared DB handle for integration tests.
func GetDB() *gorm.DB {
	return sharedDB
}

// TruncateTables truncates tables with CASCADE.
func TruncateTables(tables ...string) {
	if sharedDB == nil {
		panic("testutil.SetDB() must be called before TruncateTables()")
	}

	for _, table := range tables {
		var exists *string
		if err := sharedDB.Raw("SELECT to_regclass(?)", "public."+table).Scan(&exists).Error; err != nil {
			panic(fmt.Sprintf("failed to check table %s: %v", table, err))
		}
		if exists == nil {
			continue
		}

		if err := sharedDB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error; err != nil {
			panic(fmt.Sprintf("failed to truncate table %s: %v", table, err))
		}
	}
}

// TruncateTablesT truncates tables with testing helpers.
func TruncateTablesT(t *testing.T, tables ...string) {
	t.Helper()
	if sharedDB == nil {
		t.Fatal("testutil.SetDB() must be called before TruncateTablesT()")
	}

	for _, table := range tables {
		var exists *string
		if err := sharedDB.Raw("SELECT to_regclass(?)", "public."+table).Scan(&exists).Error; err != nil {
			t.Fatalf("failed to check table %s: %v", table, err)
		}
		if exists == nil {
			continue
		}

		if err := sharedDB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error; err != nil {
			t.Fatalf("failed to truncate table %s: %v", table, err)
		}
	}
}

// CleanAllTables truncates all public schema tables.
func CleanAllTables(t *testing.T) {
	t.Helper()
	if sharedDB == nil {
		t.Fatal("testutil.SetDB() must be called before CleanAllTables()")
	}

	var tables []string
	if err := sharedDB.Raw(`
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename DESC
	`).Scan(&tables).Error; err != nil {
		t.Fatalf("failed to list tables: %v", err)
	}

	for _, table := range tables {
		if err := sharedDB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error; err != nil {
			t.Fatalf("failed to truncate table %s: %v", table, err)
		}
	}
}

// Insert inserts one or many entities.
func Insert(entities any) {
	if sharedDB == nil {
		panic("testutil.SetDB() must be called before Insert()")
	}
	if err := sharedDB.Create(entities).Error; err != nil {
		panic(fmt.Sprintf("failed to insert entities: %v", err))
	}
}

// InsertT inserts entities with testing helpers.
func InsertT(t *testing.T, entities any) {
	t.Helper()
	if sharedDB == nil {
		t.Fatal("testutil.SetDB() must be called before InsertT()")
	}
	if err := sharedDB.Create(entities).Error; err != nil {
		t.Fatalf("failed to insert entities: %v", err)
	}
}

// Delete deletes entities using gorm conditions.
func Delete(model any, where ...any) {
	if sharedDB == nil {
		panic("testutil.SetDB() must be called before Delete()")
	}
	if err := sharedDB.Delete(model, where...).Error; err != nil {
		panic(fmt.Sprintf("failed to delete entities: %v", err))
	}
}

// DeleteT deletes entities with testing helpers.
func DeleteT(t *testing.T, model any, where ...any) {
	t.Helper()
	if sharedDB == nil {
		t.Fatal("testutil.SetDB() must be called before DeleteT()")
	}
	if err := sharedDB.Delete(model, where...).Error; err != nil {
		t.Fatalf("failed to delete entities: %v", err)
	}
}

// Count counts records by model and condition.
func Count(model any, where ...any) int64 {
	if sharedDB == nil {
		panic("testutil.SetDB() must be called before Count()")
	}

	var count int64
	if len(where) == 0 {
		if err := sharedDB.Model(model).Count(&count).Error; err != nil {
			panic(fmt.Sprintf("failed to count entities: %v", err))
		}
		return count
	}

	if err := sharedDB.Model(model).Where(where[0], where[1:]...).Count(&count).Error; err != nil {
		panic(fmt.Sprintf("failed to count entities: %v", err))
	}
	return count
}

// Exists checks if at least one record matches condition.
func Exists(model any, where ...any) bool {
	return Count(model, where...) > 0
}
