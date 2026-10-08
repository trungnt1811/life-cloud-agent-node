package usecases

import (
	"context"
	"errors"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func (u *governedHospitalJobUseCase) Unconfirmed(ctx context.Context, after string, limit int) ([]entities.GovernedHospitalOutboundEvent, error) {
	if limit < 1 || limit > 100 || (after != "" && !governedHospitalJobID(after)) {
		return nil, governedHospitalJobError(entities.ErrGovernedHospitalJobInvalid)
	}
	if u.deps.Repository == nil {
		return nil, governedHospitalJobError(errors.New("governed hospital repository missing"))
	}
	items, err := u.deps.Repository.ListUnconfirmed(ctx, u.deps.NodeID, after, limit)
	return items, governedHospitalJobError(err)
}
