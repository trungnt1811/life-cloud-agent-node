package repositories

import (
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

// enabledQueryFieldModelFromEntity assumes entity is non-nil; both call
// sites (Upsert, after its own field_code check) already guarantee that.
func enabledQueryFieldModelFromEntity(entity *entities.EnabledQueryField) *models.EnabledQueryField {
	record := entity.Record()
	return &models.EnabledQueryField{
		FieldCode: record.FieldCode,
		Enabled:   record.Enabled,
		UpdatedAt: record.UpdatedAt,
		UpdatedBy: record.UpdatedBy,
	}
}

// enabledQueryFieldEntityFromModel assumes model is non-nil; every call site
// passes the address of a local or indexed struct, never a nil pointer. It
// can still return nil - entities.NewEnabledQueryFieldFromRecord rejects a
// blank field_code - which is the filter enabledQueryFieldEntitiesFromModels
// relies on.
func enabledQueryFieldEntityFromModel(model *models.EnabledQueryField) *entities.EnabledQueryField {
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
