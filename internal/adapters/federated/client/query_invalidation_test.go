package client

import (
	"testing"

	"github.com/stretchr/testify/require"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
)

func TestGovernedQueryInvalidationMapperKeepsQueryScopeSeparateFromPermitScope(t *testing.T) {
	p := &nodev1.InvalidateWork{CommandId: "6c990c65-61b0-4311-86f9-6e4b6beb53cc", TenantId: "53a96e1c-87aa-4def-b926-675e7e74df6c", QueryId: "7fe1b949-22ee-4341-9a02-812590d30b8b", PermitId: "demo", ThroughPermitVersion: 2, Reason: nodev1.GovernanceReason_GOVERNANCE_REASON_QUERY_CANCELED, SessionEpoch: "current"}
	command, err := hospitalInvalidationFromProto(p)
	require.NoError(t, err)
	require.Equal(t, p.QueryId, command.QueryID)
	require.Equal(t, "QUERY_CANCELED", command.Reason)
	p.Reason = nodev1.GovernanceReason_GOVERNANCE_REASON_PERMIT_REVOKED
	_, err = hospitalInvalidationFromProto(p)
	require.Error(t, err, "a query-scoped command cannot revoke a permit")
	p.QueryId = ""
	_, err = hospitalInvalidationFromProto(p)
	require.NoError(t, err)
	p.Reason = nodev1.GovernanceReason_GOVERNANCE_REASON_QUERY_EXPIRED
	_, err = hospitalInvalidationFromProto(p)
	require.Error(t, err, "a query reason must specify the query")
	p.QueryId = command.QueryID
	p.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	_, err = hospitalInvalidationFromProto(p)
	require.Error(t, err)
}
