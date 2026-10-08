package repositories

import (
	"context"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

//go:generate mockgen -source=hospital_governance.go -destination=../../mocks/mock_hospital_governance_repository.go -package=mocks
type HospitalGovernanceRepository interface {
	Lock(ctx context.Context, nodeID string) error
	Get(ctx context.Context, nodeID string) (*entities.HospitalGovernance, error)
	Save(ctx context.Context, state *entities.HospitalGovernance) error
	AppendAudit(ctx context.Context, event entities.HospitalGovernanceAuditRecord) error
	GetCommand(ctx context.Context, nodeID, actor, key string) (*entities.HospitalGovernanceCommandRecord, error)
	SaveCommand(ctx context.Context, command entities.HospitalGovernanceCommandRecord) error
}
