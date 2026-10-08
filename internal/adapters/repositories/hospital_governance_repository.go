package repositories

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	domainrepos "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

type hospitalGovernanceRepository struct{ db *gorm.DB }

func NewHospitalGovernanceRepository(db *gorm.DB, _ logger.Logger) domainrepos.HospitalGovernanceRepository {
	return &hospitalGovernanceRepository{db: db}
}

func (r *hospitalGovernanceRepository) WithTx(tx *gorm.DB) domainrepos.HospitalGovernanceRepository {
	return &hospitalGovernanceRepository{db: tx}
}

// The transaction-scoped fence serializes initialization as well as updates.
func (r *hospitalGovernanceRepository) Lock(ctx context.Context, nodeID string) error {
	return r.db.WithContext(ctx).Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 730031))", nodeID).Error
}

func (r *hospitalGovernanceRepository) Get(ctx context.Context, nodeID string) (*entities.HospitalGovernance, error) {
	var model models.HospitalGovernanceState
	err := r.db.WithContext(ctx).First(&model, "node_id = ?", nodeID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var record entities.HospitalGovernanceRecord
	if err = json.Unmarshal([]byte(model.State), &record); err != nil {
		return nil, err
	}
	if record.NodeID != nodeID {
		return nil, entities.ErrHospitalGovernanceInvalid
	}
	return entities.NewHospitalGovernanceFromRecord(record), nil
}

func (r *hospitalGovernanceRepository) Save(ctx context.Context, state *entities.HospitalGovernance) error {
	if state == nil {
		return entities.ErrHospitalGovernanceInvalid
	}
	record := state.Record()
	if record.NodeID == "" || record.LocalRevision < 1 {
		return entities.ErrHospitalGovernanceInvalid
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	model := models.HospitalGovernanceState{NodeID: record.NodeID, State: string(encoded)}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "node_id"}}, DoUpdates: clause.AssignmentColumns([]string{"state"}),
	}).Create(&model).Error
}

func (r *hospitalGovernanceRepository) AppendAudit(ctx context.Context, e entities.HospitalGovernanceAuditRecord) error {
	return r.db.WithContext(ctx).Create(&models.HospitalGovernanceAudit{
		EventID: e.EventID, NodeID: e.NodeID, Type: e.Type, Actor: e.Actor, LocalRevision: e.LocalRevision,
		SnapshotRevision: e.SnapshotRevision, PermitID: e.PermitID, PermitVersion: e.PermitVersion,
		PolicyHash: e.PolicyHash, OccurredAt: e.OccurredAt,
		JobID: e.JobID, OutboundEventID: e.OutboundEventID,
		QueryID: e.QueryID, CommandID: e.CommandID,
	}).Error
}

func (r *hospitalGovernanceRepository) GetCommand(ctx context.Context, nodeID, actor, key string) (*entities.HospitalGovernanceCommandRecord, error) {
	var model models.HospitalGovernanceCommand
	err := r.db.WithContext(ctx).First(&model, "node_id = ? AND actor = ? AND command_key = ?", nodeID, actor, key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var response entities.HospitalGovernanceRecord
	if err = json.Unmarshal([]byte(model.Response), &response); err != nil {
		return nil, err
	}
	if response.NodeID != nodeID {
		return nil, entities.ErrHospitalGovernanceInvalid
	}
	return &entities.HospitalGovernanceCommandRecord{
		NodeID: model.NodeID, Actor: model.Actor, Key: model.CommandKey, Fingerprint: model.Fingerprint,
		Response: response, CreatedAt: model.CreatedAt,
	}, nil
}

func (r *hospitalGovernanceRepository) SaveCommand(ctx context.Context, c entities.HospitalGovernanceCommandRecord) error {
	response, err := json.Marshal(c.Response)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&models.HospitalGovernanceCommand{
		NodeID: c.NodeID, Actor: c.Actor, CommandKey: c.Key, Fingerprint: c.Fingerprint,
		Response: string(response), CreatedAt: c.CreatedAt,
	}).Error
}
