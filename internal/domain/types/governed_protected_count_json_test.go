package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGovernedProtectedJSONRejectsMissingDisclosureKeysAndHiddenValues(t *testing.T) {
	for _, encoded := range []string{
		`{"count":{"kind":"EXACT","value":"0"},"effective_k":"10"}`,
		`{"count":{"kind":"EXACT","value":"9","lower_bound":null,"upper_bound":null},"effective_k":"10"}`,
		`{"count":{"kind":"SUPPRESSED","value":"7","lower_bound":"1","upper_bound":"9"},"effective_k":"10"}`,
		`{"count":{"kind":"SUPPRESSED","value":null,"lower_bound":"1","upper_bound":"8"},"effective_k":"10"}`,
		`{"count":{"kind":"EXACT","value":"0","lower_bound":null,"upper_bound":null,"raw_count":"7"},"effective_k":"10"}`,
		`{"count":{"kind":"EXACT","value":"9223372036854775808","lower_bound":null,"upper_bound":null},"effective_k":"10"}`,
	} {
		var count GovernedProtectedCount
		require.Error(t, json.Unmarshal([]byte(encoded), &count), encoded)
	}
	for _, raw := range []uint64{0, 1, 9, 10} {
		count, err := ProtectGovernedCount(raw, 10)
		require.NoError(t, err)
		encoded, err := json.Marshal(count)
		require.NoError(t, err)
		var restored GovernedProtectedCount
		require.NoError(t, json.Unmarshal(encoded, &restored))
		require.Equal(t, count, restored)
	}
}
