package usecases

import (
	"context"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func (u *hospitalGovernanceUseCase) Invalidate(ctx context.Context, command entities.HospitalInvalidationCommand) error {
	if command.SessionEpoch == "" {
		return hospitalGovernanceError(entities.ErrHospitalGovernanceInvalid)
	}
	schema, event := "permit-invalidation-command/v1", "PERMIT_INVALIDATED"
	intent := map[string]any{
		"command_id": command.CommandID, "tenant_id": command.TenantID, "permit_id": command.PermitID,
		"through_version": command.ThroughVersion, "reason": command.Reason,
	}
	if command.QueryID != "" {
		schema, event = "query-invalidation-command/v1", "QUERY_INVALIDATED"
		intent["query_id"] = command.QueryID
	}
	fingerprint, err := types.GovernanceDigest(schema, intent)
	if err != nil {
		return hospitalGovernanceError(err)
	}
	_, err = u.command(ctx, "control-plane", command.CommandID, fingerprint, event, command.PermitID, command.ThroughVersion, command.SessionEpoch, command.QueryID, func(state *entities.HospitalGovernance, _ time.Time) error {
		if command.QueryID != "" {
			return state.InvalidateQuery(command)
		}
		return state.InvalidatePermit(command)
	})
	return err
}
