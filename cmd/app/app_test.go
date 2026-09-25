package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
)

func TestMergeAppErrorChannelsPropagatesWorkerFailure(t *testing.T) {
	workerErrors := make(chan error, 1)
	workerErrors <- errors.New("client credentials changed")
	merged := mergeAppErrorChannels(context.Background(), make(chan error), workerErrors)
	require.ErrorContains(t, <-merged, "federated client worker: client credentials changed")
}

func TestMergeAppErrorChannelsIdentifiesHTTPFailure(t *testing.T) {
	serverErrors := make(chan error, 1)
	serverErrors <- errors.New("listen failed")
	merged := mergeAppErrorChannels(context.Background(), serverErrors, nil)
	require.ErrorContains(t, <-merged, "http server: listen failed")
}

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
