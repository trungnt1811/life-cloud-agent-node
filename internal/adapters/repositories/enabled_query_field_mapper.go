package repositories

import (
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func enabledQueryFieldModelFromEntity(entity *entities.EnabledQueryField) *models.EnabledQueryField {
	if entity == nil {
		return nil
	}
	record := entity.Record()
	return &models.EnabledQueryField{
		FieldCode: record.FieldCode,
		Enabled:   record.Enabled,
		UpdatedAt: record.UpdatedAt,
		UpdatedBy: record.UpdatedBy,
	}
}

func enabledQueryFieldEntityFromModel(model *models.EnabledQueryField) *entities.EnabledQueryField {
	if model == nil {
		return nil
	}
	return entities.NewEnabledQueryFieldFromRecord(entities.EnabledQueryFieldRecord{
		FieldCode: model.FieldCode,
		Enabled:   model.Enabled,
		UpdatedAt: model.UpdatedAt,
		UpdatedBy: model.UpdatedBy,
	})
}

func enabledQueryFieldEntitiesFromModels(modelsList []models.EnabledQueryField) []*entities.EnabledQueryField {
	out := make([]*entities.EnabledQueryField, 0, len(modelsList))
	for i := range modelsList {
		if entity := enabledQueryFieldEntityFromModel(&modelsList[i]); entity != nil {
			out = append(out, entity)
		}
	}
	return out
}
