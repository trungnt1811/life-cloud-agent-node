package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	domainrepos "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
)

type governedHospitalJobRepository struct{ db *gorm.DB }

func NewGovernedHospitalJobRepository(db *gorm.DB) domainrepos.GovernedHospitalJobRepository {
	return &governedHospitalJobRepository{db: db}
}

func (r *governedHospitalJobRepository) WithTx(tx *gorm.DB) domainrepos.GovernedHospitalJobRepository {
	return &governedHospitalJobRepository{db: tx}
}

func (r *governedHospitalJobRepository) Get(ctx context.Context, nodeID, jobID string) (*entities.GovernedHospitalJob, error) {
	var model models.GovernedHospitalJob
	err := r.db.WithContext(ctx).First(&model, "node_id = ? AND job_id = ?", nodeID, jobID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var record entities.GovernedHospitalJobRecord
	if err := json.Unmarshal([]byte(model.Record), &record); err != nil {
		return nil, err
	}
	return governedHospitalJobFromModel(model, record)
}

func governedHospitalJobFromModel(model models.GovernedHospitalJob, record entities.GovernedHospitalJobRecord) (*entities.GovernedHospitalJob, error) {
	if record.Task.NodeID != model.NodeID || record.Task.JobID != model.JobID || record.Task.QueryID != model.QueryID || record.Phase != model.Phase || !sameRepositoryDeadline(record.ApprovalDeadline, model.ApprovalDeadline) {
		return nil, entities.ErrGovernedHospitalJobInvalid
	}
	return entities.NewGovernedHospitalJobFromRecord(record)
}

// All mutations are serialized by the transaction-bound local-policy fence.
func (r *governedHospitalJobRepository) Save(ctx context.Context, job *entities.GovernedHospitalJob) error {
	if job == nil {
		return entities.ErrGovernedHospitalJobInvalid
	}
	record := job.Record()
	if _, err := entities.NewGovernedHospitalJobFromRecord(record); err != nil {
		return err
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	model := models.GovernedHospitalJob{NodeID: record.Task.NodeID, JobID: record.Task.JobID, QueryID: record.Task.QueryID, Phase: record.Phase, Record: string(encoded)}
	if !record.ApprovalDeadline.IsZero() {
		deadline := record.ApprovalDeadline.UTC().Truncate(time.Microsecond)
		model.ApprovalDeadline = &deadline
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "node_id"}, {Name: "job_id"}}, DoUpdates: clause.AssignmentColumns([]string{"phase", "record", "approval_deadline"})}).Create(&model).Error
}

func (r *governedHospitalJobRepository) GetEvent(ctx context.Context, nodeID, eventID string) (*entities.GovernedHospitalOutboundEvent, error) {
	var model models.GovernedHospitalOutboundEvent
	err := r.db.WithContext(ctx).First(&model, "node_id = ? AND event_id = ?", nodeID, eventID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return hospitalOutboundEventFromModel(model)
}

func hospitalOutboundEventFromModel(model models.GovernedHospitalOutboundEvent) (*entities.GovernedHospitalOutboundEvent, error) {
	var record entities.GovernedHospitalOutboundEvent
	if err := json.Unmarshal([]byte(model.Record), &record); err != nil {
		return nil, err
	}
	digest, err := record.ProtectedCount.Digest()
	if err != nil || record.Binding.NodeID != model.NodeID || record.Binding.JobID != model.JobID || record.Binding.EventID != model.EventID || record.Binding.ProtectedPayloadDigest != digest || record.DeliveryState != model.DeliveryState {
		return nil, entities.ErrGovernedHospitalJobInvalid
	}
	if err := record.Validate(); err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *governedHospitalJobRepository) SaveEvent(ctx context.Context, event entities.GovernedHospitalOutboundEvent) error {
	prior, err := r.GetEvent(ctx, event.Binding.NodeID, event.Binding.EventID)
	if err != nil {
		return err
	}
	if prior != nil {
		if !event.CanReplace(*prior) {
			return entities.ErrGovernedHospitalJobInvalid
		}
		event.CreatedAt = prior.CreatedAt
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	model := models.GovernedHospitalOutboundEvent{NodeID: event.Binding.NodeID, EventID: event.Binding.EventID, JobID: event.Binding.JobID, DeliveryState: event.DeliveryState, Record: string(encoded), CreatedAt: event.CreatedAt, UpdatedAt: event.UpdatedAt}
	if _, err := hospitalOutboundEventFromModel(model); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "node_id"}, {Name: "event_id"}}, DoUpdates: clause.AssignmentColumns([]string{"delivery_state", "record", "updated_at"})}).Create(&model).Error
}

func (r *governedHospitalJobRepository) ListEvents(ctx context.Context, nodeID string, limit, offset int) ([]entities.GovernedHospitalOutboundEvent, int64, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, 0, entities.ErrGovernedHospitalJobInvalid
	}
	base := r.db.WithContext(ctx).Model(&models.GovernedHospitalOutboundEvent{}).Where("node_id = ?", nodeID)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var modelsList []models.GovernedHospitalOutboundEvent
	if err := base.Order("created_at DESC, event_id ASC").Limit(limit).Offset(offset).Find(&modelsList).Error; err != nil {
		return nil, 0, err
	}
	items := make([]entities.GovernedHospitalOutboundEvent, 0, len(modelsList))
	for _, model := range modelsList {
		event, err := hospitalOutboundEventFromModel(model)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *event)
	}
	return items, total, nil
}

func (r *governedHospitalJobRepository) ListAwaitingApproval(ctx context.Context, nodeID string, limit, offset int) ([]entities.GovernedHospitalJobRecord, int64, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, 0, entities.ErrGovernedHospitalJobInvalid
	}
	base := r.db.WithContext(ctx).Model(&models.GovernedHospitalJob{}).Where("node_id = ? AND phase = ? AND approval_deadline IS NOT NULL", nodeID, entities.GovernedHospitalWaitingApproval)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.GovernedHospitalJob
	if err := base.Order("job_id ASC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]entities.GovernedHospitalJobRecord, 0, len(rows))
	for _, row := range rows {
		var record entities.GovernedHospitalJobRecord
		if err := json.Unmarshal([]byte(row.Record), &record); err != nil {
			return nil, 0, err
		}
		job, err := governedHospitalJobFromModel(row, record)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, job.Record())
	}
	return items, total, nil
}

func (r *governedHospitalJobRepository) ListManualResumable(ctx context.Context, nodeID, after string, limit int) ([]entities.GovernedHospitalJobRecord, error) {
	if limit < 1 || limit > 100 || !validRecoveryCursor(after) {
		return nil, entities.ErrGovernedHospitalJobInvalid
	}
	query := r.db.WithContext(ctx).Where("node_id = ? AND record #>> '{Task,ReleaseMode}' = ?", nodeID, entities.HospitalReleaseManual).
		Where("phase IN ? OR (phase = ? AND record->'Stages' @> ?::jsonb)", []string{entities.GovernedHospitalWaitingApproval, entities.GovernedHospitalReady}, entities.GovernedHospitalApprovalExpired, `[{"Phase":"APPROVAL_EXPIRED","Acknowledged":false}]`)
	if after != "" {
		query = query.Where("job_id > ?", after)
	}
	var rows []models.GovernedHospitalJob
	if err := query.Order("job_id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]entities.GovernedHospitalJobRecord, 0, len(rows))
	for _, row := range rows {
		var record entities.GovernedHospitalJobRecord
		if err := json.Unmarshal([]byte(row.Record), &record); err != nil {
			return nil, err
		}
		job, err := governedHospitalJobFromModel(row, record)
		if err != nil {
			return nil, err
		}
		items = append(items, job.Record())
	}
	return items, nil
}

func sameRepositoryDeadline(record time.Time, column *time.Time) bool {
	if record.IsZero() {
		return column == nil
	}
	return column != nil && record.UTC().Truncate(time.Microsecond).Equal(column.UTC().Truncate(time.Microsecond))
}
