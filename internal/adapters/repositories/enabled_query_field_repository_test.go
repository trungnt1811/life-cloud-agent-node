package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	repositorytest "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/testsupport"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

func openEnabledQueryFieldRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := repositorytest.OpenPostgresDB(t)
	repositorytest.RequirePostgresDialect(t, db)
	repositorytest.ResetPostgresRepositoryTables(t, db)
	return db
}

func TestEnabledQueryFieldRepository_SeedDefaultsDisabled(t *testing.T) {
	db := openEnabledQueryFieldRepositoryTestDB(t)
	ctx := context.Background()
	repo := NewEnabledQueryFieldRepository(db, logger.GetLogger())

	require.NoError(t, db.Exec(`
INSERT INTO enabled_query_fields (field_code, enabled, updated_by) VALUES
    ('HB', FALSE, ''),
    ('MCV', FALSE, ''),
    ('MCH', FALSE, ''),
    ('RBC', FALSE, ''),
    ('MCHC', FALSE, ''),
    ('RDW', FALSE, ''),
    ('HBA0', FALSE, ''),
    ('HBA2', FALSE, ''),
    ('HBF', FALSE, '')
ON CONFLICT (field_code) DO NOTHING
`).Error)

	listed, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, listed, len(queryfields.SchemaV1FieldCodes))
	for _, entity := range listed {
		require.Contains(t, queryfields.SchemaV1FieldCodes, entity.FieldCode())
		require.False(t, entity.Enabled())
		require.Empty(t, entity.UpdatedBy())
	}
}

func TestEnabledQueryFieldRepository_UpsertGetAndList(t *testing.T) {
	db := openEnabledQueryFieldRepositoryTestDB(t)
	ctx := context.Background()
	repo := NewEnabledQueryFieldRepository(db, logger.GetLogger())
	now := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)

	require.NoError(t, repo.Upsert(ctx, entities.NewEnabledQueryField("hb", true, "alice", now)))
	require.NoError(t, repo.Upsert(ctx, entities.NewEnabledQueryField("MCV", false, "bob", now.Add(time.Minute))))

	got, err := repo.GetByFieldCode(ctx, "HB")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "HB", got.FieldCode())
	require.True(t, got.Enabled())
	require.Equal(t, "alice", got.UpdatedBy())

	missing, err := repo.GetByFieldCode(ctx, "HBF")
	require.NoError(t, err)
	require.Nil(t, missing)

	listed, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.Equal(t, "HB", listed[0].FieldCode())
	require.Equal(t, "MCV", listed[1].FieldCode())

	require.NoError(t, repo.Upsert(ctx, entities.NewEnabledQueryField("HB", false, "carol", now.Add(2*time.Minute))))
	updated, err := repo.GetByFieldCode(ctx, "HB")
	require.NoError(t, err)
	require.False(t, updated.Enabled())
	require.Equal(t, "carol", updated.UpdatedBy())
}
