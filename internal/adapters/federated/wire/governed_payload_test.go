package wire_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/wire"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func TestGovernedEgressProducesOnlyProtectedPayload(t *testing.T) {
	var suppressedWire []byte
	var suppressedDigest string
	for raw := uint64(0); raw <= 10; raw++ {
		payload, digest, err := wire.ProtectGovernedMatchingCount(raw, 5)
		require.NoError(t, err)
		require.EqualValues(t, 10, payload.GetEffectiveK())
		encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload)
		require.NoError(t, err)
		readback := new(nodev1.ProtectedCohortPayload)
		require.NoError(t, proto.Unmarshal(encoded, readback))
		if raw > 0 && raw < 10 {
			require.IsType(t, &nodev1.ProtectedCohortPayload_SuppressedCount{}, readback.GetCount())
			require.EqualValues(t, 1, readback.GetSuppressedCount().GetLowerBound())
			require.EqualValues(t, 9, readback.GetSuppressedCount().GetUpperBound())
			if suppressedWire == nil {
				suppressedWire, suppressedDigest = encoded, digest
			}
			require.Equal(t, suppressedWire, encoded)
			require.Equal(t, suppressedDigest, digest)
		} else {
			require.IsType(t, &nodev1.ProtectedCohortPayload_ExactCount{}, readback.GetCount())
			require.Equal(t, raw, readback.GetExactCount())
		}
	}
	for _, test := range []struct{ raw, k uint64 }{{math.MaxUint64, 5}, {1, 0}, {1, math.MaxUint64}} {
		payload, digest, err := wire.ProtectGovernedMatchingCount(test.raw, test.k)
		require.ErrorIs(t, err, types.ErrInvalidProtectedCount)
		require.Nil(t, payload)
		require.Empty(t, digest)
	}
}
