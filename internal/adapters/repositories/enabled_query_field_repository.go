package repositories

import (
	"context"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/repohelpers"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

type enabledQueryFieldRepository struct {
	db     *gorm.DB
	logger logger.Logger
}

// NewEnabledQueryFieldRepository creates a GORM-backed EnabledQueryFieldRepository.
func NewEnabledQueryFieldRepository(db *gorm.DB, logger logger.Logger) repositories.EnabledQueryFieldRepository {
	return &enabledQueryFieldRepository{
		db:     db,
		logger: logger,
	}
}

func (r *enabledQueryFieldRepository) dbWithContext(ctx context.Context) *gorm.DB {
	return repohelpers.DBWithContext(ctx, r.db)
}

// ListAll returns stored whitelist rows ordered by field_code.
func (r *enabledQueryFieldRepository) ListAll(ctx context.Context) ([]*entities.EnabledQueryField, error) {
	var modelsList []models.EnabledQueryField
	if err := r.dbWithContext(ctx).
		Order("field_code ASC").
		Find(&modelsList).Error; err != nil {
		r.logger.Error("Failed to list enabled query fields", logger.Err(err))
		return nil, err
	}
	return enabledQueryFieldEntitiesFromModels(modelsList), nil
}

// GetByFieldCode retrieves one whitelist row by canonical field code.
func (r *enabledQueryFieldRepository) GetByFieldCode(ctx context.Context, fieldCode string) (*entities.EnabledQueryField, error) {
	code := queryfields.NormalizeFieldCode(fieldCode)
	if code == "" {
		return nil, nil
	}

	var model models.EnabledQueryField
	if err := r.dbWithContext(ctx).First(&model, "field_code = ?", code).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		r.logger.Error("Failed to get enabled query field", logger.String("field_code", code), logger.Err(err))
		return nil, err
	}
	return enabledQueryFieldEntityFromModel(&model), nil
}

// Upsert inserts or overwrites a whitelist row.
func (r *enabledQueryFieldRepository) Upsert(ctx context.Context, entity *entities.EnabledQueryField) error {
	if entity == nil || strings.TrimSpace(entity.FieldCode()) == "" {
		return nil
	}

	model := enabledQueryFieldModelFromEntity(entity)
	if err := r.dbWithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "field_code"}},
			DoUpdates: clause.AssignmentColumns([]string{"enabled", "updated_at", "updated_by"}),
		}).
		Create(model).Error; err != nil {
		r.logger.Error("Failed to upsert enabled query field", logger.String("field_code", model.FieldCode), logger.Err(err))
		return err
	}
	return nil
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *enabledQueryFieldRepository) WithTx(tx *gorm.DB) repositories.EnabledQueryFieldRepository {
	if tx == nil {
		return r
	}
	return &enabledQueryFieldRepository{
		db:     tx,
		logger: r.logger,
	}
}
