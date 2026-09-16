package testutil

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// RouterSetupFunc defines the app router bootstrap function used by integration tests.
type RouterSetupFunc func(ctx context.Context, db *gorm.DB, config *conf.Configuration, log logger.Logger) *gin.Engine

// SuiteConfig holds integration suite configuration.
type SuiteConfig struct {
	Config      *conf.Configuration
	RouterSetup RouterSetupFunc
	SkipIfShort bool
}

// TestSuite manages shared integration test state.
type TestSuite struct {
	config SuiteConfig
	router http.Handler
}

// NewTestSuite creates a new integration suite.
func NewTestSuite(config SuiteConfig) *TestSuite {
	return &TestSuite{config: config}
}

// Run initializes the router once and runs tests.
func (s *TestSuite) Run(m *testing.M) int {
	if s == nil {
		return 0
	}

	gin.SetMode(gin.TestMode)
	s.router = s.config.RouterSetup(context.Background(), nil, s.config.Config, NewTestLogger())
	SetRouter(s.router)

	return m.Run()
}
