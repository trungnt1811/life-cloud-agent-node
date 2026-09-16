package repositories

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/repohelpers"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// exampleRepository implements repositories.ExampleRepository with GORM.
type exampleRepository struct {
	db     *gorm.DB
	logger logger.Logger
}

// NewExampleRepository creates a new ExampleRepository.
func NewExampleRepository(db *gorm.DB, logger logger.Logger) repositories.ExampleRepository {
	return &exampleRepository{
		db:     db,
		logger: logger,
	}
}

func (r *exampleRepository) dbWithContext(ctx context.Context) *gorm.DB {
	return repohelpers.DBWithContext(ctx, r.db)
}

// Create creates a new example entity.
func (r *exampleRepository) Create(ctx context.Context, entity *entities.ExampleEntity) error {
	if entity == nil {
		return nil
	}

	model := exampleModelFromEntity(entity)
	if err := r.dbWithContext(ctx).Create(model).Error; err != nil {
		r.logger.Error("Failed to create example", logger.Err(err))
		return err
	}
	return nil
}

// Update persists editable example fields.
func (r *exampleRepository) Update(ctx context.Context, entity *entities.ExampleEntity) error {
	if entity == nil || entity.ID() == uuid.Nil {
		return nil
	}

	record := entity.Record()
	if err := r.dbWithContext(ctx).
		Model(&models.Example{}).
		Where("id = ?", record.ID).
		Updates(map[string]any{
			"name":        record.Name,
			"description": record.Description,
			"updated_at":  record.UpdatedAt,
		}).Error; err != nil {
		r.logger.Error("Failed to update example", logger.String("id", record.ID.String()), logger.Err(err))
		return err
	}
	return nil
}

// Delete removes an example entity by ID.
func (r *exampleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return nil
	}
	if err := r.dbWithContext(ctx).Delete(&models.Example{}, "id = ?", id).Error; err != nil {
		r.logger.Error("Failed to delete example", logger.String("id", id.String()), logger.Err(err))
		return err
	}
	return nil
}

// GetByID retrieves an example entity by its ID.
func (r *exampleRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.ExampleEntity, error) {
	var model models.Example
	if err := r.dbWithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		r.logger.Error("Failed to get example by ID", logger.String("id", id.String()), logger.Err(err))
		return nil, err
	}

	return exampleEntityFromModel(&model), nil
}

// List retrieves example entities with deterministic pagination.
func (r *exampleRepository) List(ctx context.Context, limit, offset int) ([]*entities.ExampleEntity, error) {
	if limit <= 0 {
		return []*entities.ExampleEntity{}, nil
	}
	if offset < 0 {
		offset = 0
	}

	var modelsList []models.Example
	if err := r.dbWithContext(ctx).
		Order("created_at DESC, id ASC").
		Limit(limit).
		Offset(offset).
		Find(&modelsList).Error; err != nil {
		r.logger.Error("Failed to list examples", logger.Int("limit", limit), logger.Int("offset", offset), logger.Err(err))
		return nil, err
	}

	return exampleEntitiesFromModels(modelsList), nil
}

// ListByIDs retrieves example entities by IDs.
func (r *exampleRepository) ListByIDs(ctx context.Context, ids []uuid.UUID) ([]*entities.ExampleEntity, error) {
	queryIDs := uniqueNonNilUUIDs(ids)
	if len(queryIDs) == 0 {
		return []*entities.ExampleEntity{}, nil
	}

	var modelsList []models.Example
	if err := r.dbWithContext(ctx).Where("id IN ?", queryIDs).Find(&modelsList).Error; err != nil {
		r.logger.Error("Failed to list examples by IDs", logger.Err(err))
		return nil, err
	}

	return examplesInIDOrder(ids, exampleEntitiesFromModels(modelsList)), nil
}

// Count returns the total number of examples.
func (r *exampleRepository) Count(ctx context.Context) (int, error) {
	var count int64
	if err := r.dbWithContext(ctx).Model(&models.Example{}).Count(&count).Error; err != nil {
		r.logger.Error("Failed to count examples", logger.Err(err))
		return 0, err
	}
	return int(count), nil
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *exampleRepository) WithTx(tx *gorm.DB) repositories.ExampleRepository {
	if tx == nil {
		return r
	}
	return &exampleRepository{
		db:     tx,
		logger: r.logger,
	}
}

func uniqueNonNilUUIDs(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}

	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func examplesInIDOrder(ids []uuid.UUID, entitiesList []*entities.ExampleEntity) []*entities.ExampleEntity {
	if len(ids) == 0 || len(entitiesList) == 0 {
		return []*entities.ExampleEntity{}
	}

	byID := make(map[uuid.UUID]*entities.ExampleEntity, len(entitiesList))
	for _, entity := range entitiesList {
		if entity == nil {
			continue
		}
		byID[entity.ID()] = entity
	}

	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]*entities.ExampleEntity, 0, len(byID))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if entity := byID[id]; entity != nil {
			out = append(out, entity)
		}
	}
	return out
}
