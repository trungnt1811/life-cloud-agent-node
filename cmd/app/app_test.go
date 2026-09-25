package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
)

func TestRunRejectsInvalidFederatedTLSBeforeOpeningDatabase(t *testing.T) {
	config := conf.DefaultConfiguration()
	config.ControlCenterAddress = "127.0.0.1:9090"
	config.NodeID = "hospital-a"
	config.ControlCenterCAFile = filepath.Join(t.TempDir(), "missing-ca.pem")
	config.Database.DBHost = "127.0.0.1"
	config.Database.DBPort = "1"

	err := Run(context.Background(), &config)
	require.ErrorContains(t, err, "read control center CA file")
}
