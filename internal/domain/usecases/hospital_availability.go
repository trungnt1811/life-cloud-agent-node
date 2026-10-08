package usecases

import (
	"context"
	"math"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func (u *hospitalGovernanceUseCase) ChangeAvailability(ctx context.Context, actor, key string, paused bool, expected int64) (*entities.HospitalGovernanceRecord, error) {
	if expected < 1 || expected == math.MaxInt64 {
		return nil, hospitalGovernanceError(entities.ErrHospitalGovernanceInvalid)
	}
	fingerprint, err := types.GovernanceDigest("hospital-availability-command/v1", map[string]any{"paused": paused, "expected_revision": expected})
	if err != nil {
		return nil, hospitalGovernanceError(err)
	}
	return u.command(ctx, actor, key, fingerprint, "HOSPITAL_AVAILABILITY_CHANGED", "", 0, "", "", func(state *entities.HospitalGovernance, _ time.Time) error {
		return state.ChangeAvailability(paused, expected)
	})
}
