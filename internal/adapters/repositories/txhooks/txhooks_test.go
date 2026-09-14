package txhooks

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	repositorytest "github.com/lifenetwork-ai/go-backend-template/internal/adapters/repositories/testsupport"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := repositorytest.OpenPostgresDB(t)
	repositorytest.RequirePostgresDialect(t, db)
	return db
}

func TestDeferCommitRun(t *testing.T) {
	db := openTestDB(t)
	tx := db.Begin()

	calls := make([]string, 0, 2)
	Defer(tx, func() { calls = append(calls, "first") })
	Defer(tx, func() { calls = append(calls, "second") })
	Defer(tx, nil)

	err := Commit(tx)
	require.NoError(t, err)
	require.Equal(t, []string{"first", "second"}, calls)

	// Run again should not re-execute
	Run(tx)
	require.Equal(t, []string{"first", "second"}, calls)
}

func TestWithTransaction_Success(t *testing.T) {
	db := openTestDB(t)
	executed := false

	err := WithTransaction(context.Background(), db, func(tx *gorm.DB) error {
		Defer(tx, func() { executed = true })
		return nil
	})

	require.NoError(t, err)
	require.True(t, executed)
}

func TestWithTransaction_Error(t *testing.T) {
	db := openTestDB(t)
	executed := false

	err := WithTransaction(context.Background(), db, func(tx *gorm.DB) error {
		Defer(tx, func() { executed = true })
		return errors.New("boom")
	})

	require.Error(t, err)
	require.False(t, executed)
}

func TestNilGuards(t *testing.T) {
	require.NoError(t, Commit(nil))
	Run(nil)
	Defer(nil, func() {})
	Defer(&gorm.DB{}, func() {})
	Run(&gorm.DB{})
	err := WithTransaction(context.Background(), nil, func(tx *gorm.DB) error { return nil })
	require.ErrorIs(t, err, ErrDBRequired)
	err = WithTransaction(context.Background(), openTestDB(t), nil)
	require.NoError(t, err)
}
