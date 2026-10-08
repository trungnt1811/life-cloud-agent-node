package interfaces

import (
	"context"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

//go:generate mockgen -source=hospital_governance.go -destination=../../../mocks/mock_hospital_governance_usecase.go -package=mocks

type HospitalGovernanceUseCase interface {
	Read(context.Context) (*entities.HospitalGovernanceRecord, error)
	UpdatePolicy(context.Context, string, string, entities.HospitalPolicyRecord, int64) (*entities.HospitalGovernanceRecord, error)
	ChangeAcceptance(context.Context, string, string, entities.HospitalAcceptanceChange) (*entities.HospitalGovernanceRecord, error)
	ApplySnapshot(context.Context, entities.HospitalSnapshot) (*entities.HospitalGovernanceRecord, error)
	Acknowledge(context.Context, string, int64, int64) error
	FenceConnection(context.Context) error
	Invalidate(context.Context, entities.HospitalInvalidationCommand) error
	ChangeAvailability(context.Context, string, string, bool, int64) (*entities.HospitalGovernanceRecord, error)
}
