package transaction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	adapterrepos "github.com/lifenetwork-ai/go-backend-template/internal/adapters/repositories"
	repositorytest "github.com/lifenetwork-ai/go-backend-template/internal/adapters/repositories/testsupport"
	"github.com/lifenetwork-ai/go-backend-template/internal/adapters/repositories/txhooks"
	"github.com/lifenetwork-ai/go-backend-template/internal/domain/entities"
	domainrepos "github.com/lifenetwork-ai/go-backend-template/internal/domain/repositories"
	"github.com/lifenetwork-ai/go-backend-template/internal/platform/logger"
)

func openTransactionManagerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := repositorytest.OpenPostgresDB(t)
	repositorytest.RequirePostgresDialect(t, db)
	repositorytest.ResetPostgresRepositoryTables(t, db)
	return db
}

func TestTransactionManagerWithinTxCommitsBoundRepositories(t *testing.T) {
	ctx := context.Background()
	db := openTransactionManagerTestDB(t)
	exampleRepo := adapterrepos.NewExampleRepository(db, logger.GetLogger())
	manager := NewTransactionManager(db, TransactionManagerDeps{ExampleRepo: exampleRepo})

	entity := entities.NewExampleEntity(uuid.New(), "tx-commit", "commit@example.com", time.Now().UTC())
	err := manager.WithinTx(ctx, func(repos domainrepos.TxRepositories) error {
		return repos.Examples().Create(ctx, entity)
	})
	require.NoError(t, err)

	got, err := exampleRepo.GetByID(ctx, entity.ID())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, entity.ID(), got.ID())
}

func TestTransactionManagerWithinTxRollsBackBoundRepositories(t *testing.T) {
	ctx := context.Background()
	db := openTransactionManagerTestDB(t)
	exampleRepo := adapterrepos.NewExampleRepository(db, logger.GetLogger())
	manager := NewTransactionManager(db, TransactionManagerDeps{ExampleRepo: exampleRepo})
	expectedErr := errors.New("stop transaction")

	entity := entities.NewExampleEntity(uuid.New(), "tx-rollback", "rollback@example.com", time.Now().UTC())
	err := manager.WithinTx(ctx, func(repos domainrepos.TxRepositories) error {
		require.NoError(t, repos.Examples().Create(ctx, entity))
		return expectedErr
	})
	require.ErrorIs(t, err, expectedErr)

	got, err := exampleRepo.GetByID(ctx, entity.ID())
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestTransactionManagerNilGuards(t *testing.T) {
	ctx := context.Background()
	manager := NewTransactionManager(nil, TransactionManagerDeps{})

	err := manager.WithinTx(ctx, func(repos domainrepos.TxRepositories) error { return nil })
	require.ErrorIs(t, err, txhooks.ErrDBRequired)

	db := openTransactionManagerTestDB(t)
	manager = NewTransactionManager(db, TransactionManagerDeps{})
	require.NoError(t, manager.WithinTx(ctx, nil))
}
