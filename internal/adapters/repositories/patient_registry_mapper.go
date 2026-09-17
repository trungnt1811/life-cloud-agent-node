package repositories

import (
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func patientModelFromEntity(entity *entities.Patient) *models.Patient {
	if entity == nil {
		return nil
	}
	record := entity.Record()
	return &models.Patient{
		ID:                record.ID,
		ExternalPatientID: record.ExternalPatientID,
		CreatedAt:         record.CreatedAt,
	}
}

func patientEntityFromModel(model *models.Patient) *entities.Patient {
	if model == nil {
		return nil
	}
	return entities.NewPatientFromRecord(entities.PatientRecord{
		ID:                model.ID,
		ExternalPatientID: model.ExternalPatientID,
		CreatedAt:         model.CreatedAt,
	})
}

func specimenModelFromEntity(entity *entities.Specimen) *models.Specimen {
	if entity == nil {
		return nil
	}
	record := entity.Record()
	return &models.Specimen{
		ID:                 record.ID,
		PatientID:          record.PatientID,
		ExternalSpecimenID: record.ExternalSpecimenID,
		CollectedAt:        record.CollectedAt,
		SourceDataset:      record.SourceDataset,
		SourceFile:         record.SourceFile,
		SourceRowNumber:    record.SourceRowNumber,
		SourceRecordID:     record.SourceRecordID,
		CreatedAt:          record.CreatedAt,
	}
}

func specimenEntityFromModel(model *models.Specimen) *entities.Specimen {
	if model == nil {
		return nil
	}
	return entities.NewSpecimenFromRecord(entities.SpecimenRecord{
		ID:                 model.ID,
		PatientID:          model.PatientID,
		ExternalSpecimenID: model.ExternalSpecimenID,
		CollectedAt:        model.CollectedAt,
		SourceDataset:      model.SourceDataset,
		SourceFile:         model.SourceFile,
		SourceRowNumber:    model.SourceRowNumber,
		SourceRecordID:     model.SourceRecordID,
		CreatedAt:          model.CreatedAt,
	})
}

func labObservationModelFromEntity(entity *entities.LabObservation) *models.LabObservation {
	if entity == nil {
		return nil
	}
	record := entity.Record()
	return &models.LabObservation{
		ID:         record.ID,
		SpecimenID: record.SpecimenID,
		FieldCode:  record.FieldCode,
		Value:      record.Value,
		Censored:   record.Censored,
		RawValue:   record.RawValue,
		RawUnit:    record.RawUnit,
		Revision:   record.Revision,
		CreatedAt:  record.CreatedAt,
	}
}

func labObservationEntityFromModel(model *models.LabObservation) *entities.LabObservation {
	if model == nil {
		return nil
	}
	return entities.NewLabObservationFromRecord(entities.LabObservationRecord{
		ID:         model.ID,
		SpecimenID: model.SpecimenID,
		FieldCode:  model.FieldCode,
		Value:      model.Value,
		Censored:   model.Censored,
		RawValue:   model.RawValue,
		RawUnit:    model.RawUnit,
		Revision:   model.Revision,
		CreatedAt:  model.CreatedAt,
	})
}

func labObservationEntitiesFromModels(modelsList []models.LabObservation) []*entities.LabObservation {
	out := make([]*entities.LabObservation, 0, len(modelsList))
	for i := range modelsList {
		if entity := labObservationEntityFromModel(&modelsList[i]); entity != nil {
			out = append(out, entity)
		}
	}
	return out
}
