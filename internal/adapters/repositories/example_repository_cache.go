package repositories

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/lifenetwork-ai/go-backend-template/constants"
	cachetypes "github.com/lifenetwork-ai/go-backend-template/infrastructures/caching/types"
	"github.com/lifenetwork-ai/go-backend-template/internal/adapters/repositories/txhooks"
	"github.com/lifenetwork-ai/go-backend-template/internal/domain/entities"
	domainrepos "github.com/lifenetwork-ai/go-backend-template/internal/domain/repositories"
	"github.com/lifenetwork-ai/go-backend-template/internal/platform/logger"
)

// exampleRepositoryCache decorates ExampleRepository with a cache layer.
type exampleRepositoryCache struct {
	repo   domainrepos.ExampleRepository
	cache  cachetypes.CacheRepository
	logger logger.Logger
	tx     *gorm.DB
}

// NewExampleRepositoryCache creates a cached ExampleRepository wrapper.
func NewExampleRepositoryCache(
	repo domainrepos.ExampleRepository,
	cache cachetypes.CacheRepository,
	logger logger.Logger,
) domainrepos.ExampleRepository {
	if cache == nil {
		return repo
	}
	return &exampleRepositoryCache{
		repo:   repo,
		cache:  cache,
		logger: logger,
	}
}

func exampleKeyEntityByID(id uuid.UUID) *cachetypes.Keyer {
	return &cachetypes.Keyer{Raw: "example:entity:id:" + id.String()}
}

// Create creates an example entity and caches it after a successful write.
func (c *exampleRepositoryCache) Create(ctx context.Context, entity *entities.ExampleEntity) error {
	if err := c.repo.Create(ctx, entity); err != nil {
		return err
	}
	if entity == nil || entity.ID() == uuid.Nil {
		return nil
	}

	c.afterCommit(func() { c.saveEntity(entity) })
	return nil
}

// Update updates an example entity and refreshes cache after a successful write.
func (c *exampleRepositoryCache) Update(ctx context.Context, entity *entities.ExampleEntity) error {
	if err := c.repo.Update(ctx, entity); err != nil {
		return err
	}
	if entity == nil || entity.ID() == uuid.Nil {
		return nil
	}

	c.afterCommit(func() { c.saveEntity(entity) })
	return nil
}

// Delete deletes an example entity and invalidates cache after a successful write.
func (c *exampleRepositoryCache) Delete(ctx context.Context, id uuid.UUID) error {
	if err := c.repo.Delete(ctx, id); err != nil {
		return err
	}
	if id == uuid.Nil {
		return nil
	}

	c.afterCommit(func() {
		if err := c.cache.RemoveItem(exampleKeyEntityByID(id)); err != nil {
			c.logger.Error("example cache remove failed", logger.String("id", id.String()), logger.Err(err))
		}
	})
	return nil
}

// GetByID retrieves an example entity by ID, using cache outside transactions.
func (c *exampleRepositoryCache) GetByID(ctx context.Context, id uuid.UUID) (*entities.ExampleEntity, error) {
	if c.tx != nil || id == uuid.Nil {
		return c.repo.GetByID(ctx, id)
	}

	var cached entities.ExampleRecord
	if err := c.cache.RetrieveItem(exampleKeyEntityByID(id), &cached); err == nil {
		if cachedEntity := entities.NewExampleEntityFromRecord(cached); cachedEntity != nil {
			return cachedEntity, nil
		}
	}

	entity, err := c.repo.GetByID(ctx, id)
	if err != nil || entity == nil {
		return entity, err
	}

	c.saveEntity(entity)
	return entity, nil
}

// List retrieves example entities and warms per-entity cache outside transactions.
func (c *exampleRepositoryCache) List(ctx context.Context, limit, offset int) ([]*entities.ExampleEntity, error) {
	entitiesList, err := c.repo.List(ctx, limit, offset)
	if err != nil || c.tx != nil {
		return entitiesList, err
	}

	c.saveEntities(entitiesList)
	return entitiesList, nil
}

// ListByIDs retrieves example entities by IDs, fetching cache misses from the wrapped repo.
func (c *exampleRepositoryCache) ListByIDs(ctx context.Context, ids []uuid.UUID) ([]*entities.ExampleEntity, error) {
	queryIDs := uniqueNonNilUUIDs(ids)
	if len(queryIDs) == 0 {
		return []*entities.ExampleEntity{}, nil
	}
	if c.tx != nil {
		return c.repo.ListByIDs(ctx, ids)
	}

	byID := make(map[uuid.UUID]*entities.ExampleEntity, len(queryIDs))
	missingIDs := make([]uuid.UUID, 0, len(queryIDs))
	for _, id := range queryIDs {
		var cached entities.ExampleRecord
		if err := c.cache.RetrieveItem(exampleKeyEntityByID(id), &cached); err == nil {
			if cachedEntity := entities.NewExampleEntityFromRecord(cached); cachedEntity != nil {
				byID[id] = cachedEntity
				continue
			}
		}
		missingIDs = append(missingIDs, id)
	}

	if len(missingIDs) > 0 {
		fetched, err := c.repo.ListByIDs(ctx, missingIDs)
		if err != nil {
			return nil, err
		}
		for _, entity := range fetched {
			if entity == nil {
				continue
			}
			byID[entity.ID()] = entity
		}
		c.saveEntities(fetched)
	}

	return examplesFromMapByIDs(ids, byID), nil
}

// Count returns the total number of examples.
func (c *exampleRepositoryCache) Count(ctx context.Context) (int, error) {
	return c.repo.Count(ctx)
}

// WithTx returns a cached repository wrapper bound to the provided transaction.
func (c *exampleRepositoryCache) WithTx(tx *gorm.DB) domainrepos.ExampleRepository {
	if tx == nil {
		return c
	}
	txRepo, ok := c.repo.(interface {
		WithTx(*gorm.DB) domainrepos.ExampleRepository
	})
	if !ok {
		return c
	}
	return &exampleRepositoryCache{
		repo:   txRepo.WithTx(tx),
		cache:  c.cache,
		logger: c.logger,
		tx:     tx,
	}
}

func (c *exampleRepositoryCache) afterCommit(fn func()) {
	if fn == nil {
		return
	}
	if c.tx != nil {
		txhooks.Defer(c.tx, fn)
		return
	}
	fn()
}

func (c *exampleRepositoryCache) saveEntity(entity *entities.ExampleEntity) {
	if entity == nil || entity.ID() == uuid.Nil {
		return
	}
	if err := c.cache.SaveItem(exampleKeyEntityByID(entity.ID()), entity.Record(), constants.DefaultExpiration); err != nil {
		c.logger.Error("example cache save failed", logger.Err(err))
	}
}

func (c *exampleRepositoryCache) saveEntities(entitiesList []*entities.ExampleEntity) {
	if len(entitiesList) == 0 {
		return
	}

	items := make([]cachetypes.RepoBatchItem, 0, len(entitiesList))
	for _, entity := range entitiesList {
		if entity == nil || entity.ID() == uuid.Nil {
			continue
		}
		items = append(items, cachetypes.RepoBatchItem{
			Key:        exampleKeyEntityByID(entity.ID()),
			Value:      entity.Record(),
			Expiration: constants.DefaultExpiration,
		})
	}
	if len(items) == 0 {
		return
	}
	if err := c.cache.SaveItems(items); err != nil {
		c.logger.Error("example cache batch save failed", logger.Err(err))
	}
}

func examplesFromMapByIDs(ids []uuid.UUID, byID map[uuid.UUID]*entities.ExampleEntity) []*entities.ExampleEntity {
	if len(ids) == 0 || len(byID) == 0 {
		return []*entities.ExampleEntity{}
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
