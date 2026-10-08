package entities

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

func TestGovernedQueryPinnedHashBytes(t *testing.T) {
	encoded, err := os.ReadFile("testdata/query-hash-vectors.json")
	require.NoError(t, err)
	var fixture struct {
		Request           json.RawMessage `json:"request_input"`
		RequestHash       string          `json:"request_hash"`
		Authorization     json.RawMessage `json:"authorization_input"`
		AuthorizationHash string          `json:"authorization_hash"`
	}
	require.NoError(t, json.Unmarshal(encoded, &fixture))
	for _, test := range []struct {
		schema, expected string
		input            json.RawMessage
	}{
		{"governed-request/v1", fixture.RequestHash, fixture.Request},
		{"authorization-snapshot/v1", fixture.AuthorizationHash, fixture.Authorization},
	} {
		hash, err := types.GovernanceDigest(test.schema, test.input)
		require.NoError(t, err)
		require.Equal(t, test.expected, hash)
	}
}
