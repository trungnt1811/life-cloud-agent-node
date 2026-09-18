package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/contracts"
)

func TestEnabledQueryFieldDTOOmitsZeroUpdatedAt(t *testing.T) {
	t.Parallel()

	withoutTimestamp, err := json.Marshal(NewEnabledQueryFieldDTOFromOutput(&contracts.EnabledQueryFieldOutput{
		FieldCode: "HB",
		Enabled:   false,
	}))
	require.NoError(t, err)
	require.NotContains(t, string(withoutTimestamp), "updated_at")

	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	withTimestamp, err := json.Marshal(NewEnabledQueryFieldDTOFromOutput(&contracts.EnabledQueryFieldOutput{
		FieldCode: "HB",
		Enabled:   true,
		UpdatedAt: &now,
		UpdatedBy: "alice",
	}))
	require.NoError(t, err)
	require.Contains(t, string(withTimestamp), "updated_at")
}

func TestUpdateEnabledQueryFieldRequestRequiresEnabled(t *testing.T) {
	t.Parallel()

	enabled := true
	input := UpdateEnabledQueryFieldRequest{
		Enabled: &enabled,
	}.ToInput("hb", "alice")
	require.Equal(t, "hb", input.FieldCode)
	require.True(t, input.Enabled)
	require.Equal(t, "alice", input.UpdatedBy, "updated_by comes from ToInput's parameter (the authenticated identity), not the request body")
}
