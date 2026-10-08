package conf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGovernedExecutionRequiresExplicitSyncAndMutualTLS(t *testing.T) {
	c := DefaultConfiguration()
	require.False(t, c.GovernedExecutionEnabled)
	c.GovernedExecutionEnabled = true
	require.ErrorContains(t, ValidateConfiguration(c), "GOVERNED_EXECUTION_ENABLED")
	c.GovernanceSyncEnabled = true
	c.ControlCenterAddress = "cp.example:9090"
	c.ControlCenterClientCertFile, c.ControlCenterClientKeyFile = "client.pem", "client-key.pem"
	require.NoError(t, ValidateConfiguration(c))
	c.ControlCenterInsecure = true
	require.Error(t, ValidateConfiguration(c))
}
