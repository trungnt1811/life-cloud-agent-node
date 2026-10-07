package types_test

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func TestGovernedProtectedPayloadGoldenVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/protected-payload-vectors.json")
	require.NoError(t, err)
	var vectors []struct {
		Name          string  `json:"name"`
		RawCount      string  `json:"raw_count"`
		LocalK        string  `json:"local_k"`
		EffectiveK    string  `json:"effective_k"`
		Kind          string  `json:"kind"`
		Value         *string `json:"value"`
		LowerBound    *string `json:"lower_bound"`
		UpperBound    *string `json:"upper_bound"`
		CanonicalJSON string  `json:"canonical_json"`
		Digest        string  `json:"digest"`
	}
	require.NoError(t, json.Unmarshal(raw, &vectors))
	require.Len(t, vectors, 8)
	for _, vector := range vectors {
		t.Run(vector.Name, func(t *testing.T) {
			k, err := strconv.ParseUint(vector.EffectiveK, 10, 64)
			require.NoError(t, err)
			var count types.GovernedProtectedCount
			rawCount, parseErr := strconv.ParseUint(vector.RawCount, 10, 64)
			require.NoError(t, parseErr)
			localK, parseErr := strconv.ParseUint(vector.LocalK, 10, 64)
			require.NoError(t, parseErr)
			count, err = types.ProtectGovernedCount(rawCount, localK)
			require.NoError(t, err)
			require.Equal(t, k, count.EffectiveK())
			encoded, err := count.CanonicalJSON()
			require.NoError(t, err)
			require.Equal(t, vector.CanonicalJSON, string(encoded))
			digest, err := count.Digest()
			require.NoError(t, err)
			require.Equal(t, vector.Digest, digest)
			value, exact := count.ExactValue()
			require.Equal(t, vector.Kind == "EXACT", exact)
			if exact {
				require.NotNil(t, vector.Value)
				require.Equal(t, *vector.Value, strconv.FormatUint(value, 10))
			} else {
				require.Nil(t, vector.Value)
				lower, upper, suppressed := count.Bounds()
				require.True(t, suppressed)
				require.NotNil(t, vector.LowerBound)
				require.NotNil(t, vector.UpperBound)
				require.Equal(t, *vector.LowerBound, strconv.FormatUint(lower, 10))
				require.Equal(t, *vector.UpperBound, strconv.FormatUint(upper, 10))
			}
		})
	}
}
