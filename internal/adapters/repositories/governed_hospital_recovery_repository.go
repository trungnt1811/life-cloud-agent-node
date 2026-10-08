package repositories

import (
	"context"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func (r *governedHospitalJobRepository) ListUnconfirmed(ctx context.Context, nodeID, after string, limit int) ([]entities.GovernedHospitalOutboundEvent, error) {
	if limit < 1 || limit > 100 || !validRecoveryCursor(after) {
		return nil, entities.ErrGovernedHospitalJobInvalid
	}
	query := r.db.WithContext(ctx).Where("node_id = ? AND delivery_state = ?", nodeID, entities.GovernedHospitalSentUnconfirmed)
	if after != "" {
		query = query.Where("event_id > ?", after)
	}
	var rows []models.GovernedHospitalOutboundEvent
	if err := query.Order("event_id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]entities.GovernedHospitalOutboundEvent, 0, len(rows))
	for _, row := range rows {
		event, err := hospitalOutboundEventFromModel(row)
		if err != nil {
			return nil, err
		}
		items = append(items, *event)
	}
	return items, nil
}

func validRecoveryCursor(after string) bool {
	if after == "" {
		return true
	}
	id, err := uuid.Parse(after)
	return err == nil && id != uuid.Nil && id.String() == after
}
