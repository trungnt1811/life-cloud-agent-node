package integration

import (
	"os"
	"testing"

	"github.com/lifenetwork-ai/go-backend-template/cmd/app"
	"github.com/lifenetwork-ai/go-backend-template/conf"
	"github.com/lifenetwork-ai/go-backend-template/constants"
	"github.com/lifenetwork-ai/go-backend-template/tests/integration/testutil"
)

func TestMain(m *testing.M) {
	suite := testutil.NewTestSuite(testutil.SuiteConfig{
		Config: &conf.Configuration{
			AppName:   "go-backend-template-test",
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
