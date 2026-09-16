package integration

import (
	"os"
	"testing"

	"github.com/lifenetwork-ai/life-cloud-agent-node/cmd/app"
	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	"github.com/lifenetwork-ai/life-cloud-agent-node/tests/integration/testutil"
)

func TestMain(m *testing.M) {
	suite := testutil.NewTestSuite(testutil.SuiteConfig{
		Config: &conf.Configuration{
			AppName:   "life-cloud-agent-node-test",
			AppPort:   8080,
			Env:       "TEST",
			LogLevel:  "info",
			CacheType: constants.CacheTypeInMemory,
		},
		RouterSetup: app.SetupRouter,
		SkipIfShort: true,
	})

	os.Exit(suite.Run(m))
}
