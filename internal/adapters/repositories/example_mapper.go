package repositories

import (
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func exampleModelFromEntity(entity *entities.ExampleEntity) *models.Example {
	if entity == nil {
		return nil
	}
	record := entity.Record()
	return &models.Example{
		ID:          record.ID,
		Name:        record.Name,
		Description: record.Description,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}

func exampleEntityFromModel(model *models.Example) *entities.ExampleEntity {
	if model == nil {
		return nil
	}
	return entities.NewExampleEntityFromRecord(entities.ExampleRecord{
		ID:          model.ID,
		Name:        model.Name,
		Description: model.Description,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	})
}

func exampleEntitiesFromModels(modelsList []models.Example) []*entities.ExampleEntity {
	out := make([]*entities.ExampleEntity, 0, len(modelsList))
	for i := range modelsList {
		if entity := exampleEntityFromModel(&modelsList[i]); entity != nil {
			out = append(out, entity)
		}
	}
	return out
}
