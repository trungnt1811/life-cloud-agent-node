package wire

import (
	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

// ProtectGovernedMatchingCount builds only the reviewed disclosure and digest.
// It does not authorize execution/egress or bypass the future durable ledger.
func ProtectGovernedMatchingCount(raw, localK uint64) (*nodev1.ProtectedCohortPayload, string, error) {
	count, err := types.ProtectGovernedCount(raw, localK)
	if err != nil {
		return nil, "", err
	}
	digest, err := count.Digest()
	if err != nil {
		return nil, "", err
	}
	payload := &nodev1.ProtectedCohortPayload{EffectiveK: count.EffectiveK()}
	if value, exact := count.ExactValue(); exact {
		payload.Count = &nodev1.ProtectedCohortPayload_ExactCount{ExactCount: value}
	} else {
		lower, upper, _ := count.Bounds()
		payload.Count = &nodev1.ProtectedCohortPayload_SuppressedCount{SuppressedCount: &nodev1.PositiveSuppressedCount{LowerBound: lower, UpperBound: upper}}
	}
	return payload, digest, nil
}
