package repositories

import (
	"context"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

//go:generate mockgen -source=governed_hospital_job.go -destination=../../mocks/mock_governed_hospital_job_repository.go -package=mocks
type GovernedHospitalJobRepository interface {
	Get(context.Context, string, string) (*entities.GovernedHospitalJob, error)
	Save(context.Context, *entities.GovernedHospitalJob) error
	GetEvent(context.Context, string, string) (*entities.GovernedHospitalOutboundEvent, error)
	SaveEvent(context.Context, entities.GovernedHospitalOutboundEvent) error
	ListEvents(context.Context, string, int, int) ([]entities.GovernedHospitalOutboundEvent, int64, error)
	ListUnconfirmed(context.Context, string, string, int) ([]entities.GovernedHospitalOutboundEvent, error)
	ListAwaitingApproval(context.Context, string, int, int) ([]entities.GovernedHospitalJobRecord, int64, error)
	ListManualResumable(context.Context, string, string, int) ([]entities.GovernedHospitalJobRecord, error)
}
