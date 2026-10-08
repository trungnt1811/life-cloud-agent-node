package conf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGovernedHospitalSyncRequiresVerifiedMutualTLSConfiguration(t *testing.T) {
	c := DefaultConfiguration()
	require.False(t, c.GovernanceSyncEnabled)
	c.GovernanceSyncEnabled = true
	require.ErrorContains(t, ValidateConfiguration(c), "CONTROL_CENTER")
	c.ControlCenterAddress = "governance-cp.test:9090"
	c.ControlCenterInsecure = true
	require.ErrorContains(t, ValidateConfiguration(c), "CONTROL_CENTER")
	c.ControlCenterInsecure = false
	c.ControlCenterClientCertFile, c.ControlCenterClientKeyFile = "client.pem", "client-key.pem"
	require.NoError(t, ValidateConfiguration(c))
}
