package repositories

import (
	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func jobProgressModelFromEntity(progress *entities.JobProgress) *models.JobProgress {
	record := progress.Record()
	model := &models.JobProgress{
		JobID:        record.JobID,
		CriteriaHash: record.CriteriaHash,
		RunningCount: int64(record.RunningCount),
		Status:       string(record.Status),
		UpdatedAt:    record.UpdatedAt,
	}
	if record.LastPatientID != uuid.Nil {
		lastPatientID := record.LastPatientID
		model.LastPatientID = &lastPatientID
	}
	return model
}

func jobProgressEntityFromModel(model *models.JobProgress) *entities.JobProgress {
	record := entities.JobProgressRecord{
		JobID:        model.JobID,
		CriteriaHash: model.CriteriaHash,
		RunningCount: uint64(model.RunningCount),
		Status:       entities.JobProgressStatus(model.Status),
		UpdatedAt:    model.UpdatedAt,
	}
	if model.LastPatientID != nil {
		record.LastPatientID = *model.LastPatientID
	}
	return entities.NewJobProgressFromRecord(record)
}
