package interfaces

import (
	"context"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

//go:generate mockgen -source=governed_hospital_job.go -destination=../../../mocks/mock_governed_hospital_job_usecase.go -package=mocks
type GovernedHospitalJobUseCase interface {
	Begin(context.Context, entities.GovernedHospitalTask) (*entities.GovernedHospitalJobRecord, error)
	Prepare(context.Context, string, string, uint64) (*entities.GovernedHospitalJobRecord, error)
	Intent(context.Context, string, string) (*entities.GovernedHospitalJobRecord, error)
	Send(context.Context, string, entities.GovernedHospitalGrant) (*entities.GovernedHospitalJobRecord, error)
	Receive(context.Context, entities.GovernedHospitalReceipt) (*entities.GovernedHospitalJobRecord, error)
	Deny(context.Context, entities.GovernedHospitalDenial) (*entities.GovernedHospitalJobRecord, error)
	Terminate(context.Context, string, string, string, string) (*entities.GovernedHospitalJobRecord, error)
	AcknowledgeStage(context.Context, entities.GovernedHospitalStageAck) (*entities.GovernedHospitalJobRecord, error)
	Read(context.Context, string) (*entities.GovernedHospitalJobRecord, error)
	ListEvents(context.Context, int, int) ([]entities.GovernedHospitalOutboundEvent, int64, error)
	ListApprovals(context.Context, int, int) ([]entities.GovernedHospitalJobRecord, int64, error)
	ResumableManual(context.Context, string, int) ([]entities.GovernedHospitalJobRecord, error)
	Approve(context.Context, string, string, int64, string) (*entities.GovernedHospitalJobRecord, error)
	Decline(context.Context, string, string, int64, string) (*entities.GovernedHospitalJobRecord, error)
	ExpireApproval(context.Context, string) (*entities.GovernedHospitalJobRecord, error)
	Unconfirmed(context.Context, string, int) ([]entities.GovernedHospitalOutboundEvent, error)
}
