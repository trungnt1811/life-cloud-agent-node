package types_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func TestGovernedProtectionHidesPositiveSmallCellsNotZero(t *testing.T) {
	for _, test := range []struct {
		raw, localK, effectiveK uint64
		suppressed              bool
	}{
		{0, 5, 10, false},
		{1, 5, 10, true},
		{9, 5, 10, true},
		{10, 5, 10, false},
		{14, 20, 20, true},
		{19, 20, 20, true},
		{20, 20, 20, false},
		{math.MaxInt64, 5, 10, false},
	} {
		count, err := types.ProtectGovernedCount(test.raw, test.localK)
		require.NoError(t, err)
		require.Equal(t, test.effectiveK, count.EffectiveK())
		value, exact := count.ExactValue()
		require.Equal(t, !test.suppressed, exact)
		if exact {
			require.Equal(t, test.raw, value)
		}
		lower, upper, suppressed := count.Bounds()
		require.Equal(t, test.suppressed, suppressed)
		if suppressed {
			require.EqualValues(t, 1, lower)
			require.Equal(t, test.effectiveK-1, upper)
		}
	}
}

func TestGovernedProtectionNeverHashesHiddenRawCounts(t *testing.T) {
	var reference string
	for raw := uint64(1); raw < 10; raw++ {
		count, err := types.ProtectGovernedCount(raw, 5)
		require.NoError(t, err)
		digest, err := count.Digest()
		require.NoError(t, err)
		if reference == "" {
			reference = digest
		}
		require.Equal(t, reference, digest)
	}
	zero, err := types.ProtectGovernedCount(0, 5)
	require.NoError(t, err)
	digest, err := zero.Digest()
	require.NoError(t, err)
	require.NotEqual(t, reference, digest)
	for _, test := range []struct{ raw, k uint64 }{{math.MaxUint64, 5}, {1, 0}, {1, math.MaxUint64}} {
		count, err := types.ProtectGovernedCount(test.raw, test.k)
		require.ErrorIs(t, err, types.ErrInvalidProtectedCount)
		require.Equal(t, types.GovernedProtectedCount{}, count)
		require.NotContains(t, err.Error(), "18446744073709551615")
	}
}
