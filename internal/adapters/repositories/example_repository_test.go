package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/lifenetwork-ai/life-cloud-agent-node/infrastructures/caching"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	repositorytest "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/testsupport"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	domainrepos "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

func openExampleRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := repositorytest.OpenPostgresDB(t)
	repositorytest.RequirePostgresDialect(t, db)
	repositorytest.ResetPostgresRepositoryTables(t, db)
	return db
}

func TestExampleRepository_CRUDAndListByIDs(t *testing.T) {
	db := openExampleRepositoryTestDB(t)
	ctx := context.Background()
	repo := NewExampleRepository(db, logger.GetLogger())

	first := entities.NewExampleEntity(uuid.New(), "first", "one", time.Now().UTC())
	second := entities.NewExampleEntity(uuid.New(), "second", "two", time.Now().UTC())
	require.NoError(t, repo.Create(ctx, first))
	require.NoError(t, repo.Create(ctx, second))

	got, err := repo.GetByID(ctx, first.ID())
	require.NoError(t, err)
	require.Equal(t, first.ID(), got.ID())
	require.Equal(t, "first", got.Name())

	total, err := repo.Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, total)

	page, err := repo.List(ctx, 1, 0)
	require.NoError(t, err)
	require.Len(t, page, 1)

	first.UpdateDetails("first updated", "changed", time.Now().UTC())
	require.NoError(t, repo.Update(ctx, first))

	got, err = repo.GetByID(ctx, first.ID())
	require.NoError(t, err)
	require.Equal(t, "first updated", got.Name())
	require.Equal(t, "changed", got.Description())

	missing, err := repo.GetByID(ctx, uuid.New())
	require.NoError(t, err)
	require.Nil(t, missing)

	items, err := repo.ListByIDs(ctx, []uuid.UUID{second.ID(), uuid.Nil, first.ID(), second.ID(), uuid.New()})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, second.ID(), items[0].ID())
	require.Equal(t, first.ID(), items[1].ID())

	require.NoError(t, repo.Delete(ctx, first.ID()))
	deleted, err := repo.GetByID(ctx, first.ID())
	require.NoError(t, err)
	require.Nil(t, deleted)
}

func TestExampleRepositoryCache_ReadThroughAndTxBypass(t *testing.T) {
	db := openExampleRepositoryTestDB(t)
	ctx := context.Background()

	baseRepo := NewExampleRepository(db, logger.GetLogger())
	cacheClient := caching.NewGoCacheClient(gocache.New(time.Minute, time.Minute))
	cacheRepo := caching.NewCachingRepository(ctx, "life-cloud-agent-node-test", cacheClient)
	repo := NewExampleRepositoryCache(baseRepo, cacheRepo, logger.GetLogger())

	entity := entities.NewExampleEntity(uuid.New(), "cached", "initial", time.Now().UTC())
	require.NoError(t, repo.Create(ctx, entity))

	require.NoError(t, db.Model(&models.Example{}).
		Where("id = ?", entity.ID()).
		Update("name", "updated-in-db").Error)

	got, err := repo.GetByID(ctx, entity.ID())
	require.NoError(t, err)
	require.Equal(t, "cached", got.Name())

	entity.UpdateDetails("cache-updated", "changed", time.Now().UTC())
	require.NoError(t, repo.Update(ctx, entity))

	require.NoError(t, db.Model(&models.Example{}).
		Where("id = ?", entity.ID()).
		Update("name", "stale-db-write").Error)

	got, err = repo.GetByID(ctx, entity.ID())
	require.NoError(t, err)
	require.Equal(t, "cache-updated", got.Name())

	tx := db.Begin()
	require.NoError(t, tx.Error)

	require.NoError(t, tx.Model(&models.Example{}).
		Where("id = ?", entity.ID()).
		Update("name", "updated-in-tx").Error)

	txRepo, ok := repo.(interface {
		WithTx(*gorm.DB) domainrepos.ExampleRepository
	})
	require.True(t, ok)

	got, err = txRepo.WithTx(tx).GetByID(ctx, entity.ID())
	require.NoError(t, err)
	require.Equal(t, "updated-in-tx", got.Name())
	require.NoError(t, tx.Rollback().Error)

	require.NoError(t, repo.Delete(ctx, entity.ID()))
	missing, err := repo.GetByID(ctx, entity.ID())
	require.NoError(t, err)
	require.Nil(t, missing)
}
