package app

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lifenetwork-ai/go-backend-template/conf"
	"github.com/lifenetwork-ai/go-backend-template/internal/adapters/postgres"
	httprouter "github.com/lifenetwork-ai/go-backend-template/internal/delivery/http/router"
	"github.com/lifenetwork-ai/go-backend-template/internal/di"
	"github.com/lifenetwork-ai/go-backend-template/internal/di/instances"
	"github.com/lifenetwork-ai/go-backend-template/internal/platform/logger"
	"github.com/lifenetwork-ai/go-backend-template/internal/runtimeconfig"
	"github.com/lifenetwork-ai/go-backend-template/internal/server"
)

func RunApp(config *conf.Configuration) error {
	return Run(context.Background(), config)
}

func Run(ctx context.Context, config *conf.Configuration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	initializeLoggerAndMode(config)
	moduleConfigs := runtimeconfig.ModuleConfigsFromConfiguration(config)
	db := instances.DBInstance(moduleConfigs.Database)

	// Run database migrations if enabled
	if config.EnableAutoMigrate {
		logger.GetLogger().Info("Auto-migration is enabled. Running database migrations...")
		if err := postgres.RunMigrations(db, "internal/adapters/postgres/scripts"); err != nil {
			return err
		}
	}

	appLogger := instances.LoggerInstance()
	r := SetupRouter(runCtx, db, config, appLogger)
	httpServer, serverErrC := server.StartHTTP(r, server.HTTPConfig{Port: config.AppPort}, appLogger)
	return server.WaitForShutdownSignal(runCtx, cancel, httpServer, serverErrC, server.GracefulTimeout(0), appLogger)
}

// SetupRouter initializes the complete HTTP router for production and integration tests.
func SetupRouter(ctx context.Context, db *gorm.DB, config *conf.Configuration, appLogger logger.Logger) *gin.Engine {
	if ctx == nil {
		ctx = context.Background()
	}
	repos := di.InitializeRepos(ctx, db, appLogger, runtimeconfig.ModuleConfigsFromConfiguration(config))
	useCases := di.InitializeUseCases(repos, appLogger)
	return httprouter.SetupWithDependencies(useCases, appLogger, httprouter.OptionsFromConfiguration(config))
}

// Helper Functions
func initializeLoggerAndMode(config *conf.Configuration) {
	// Validate configuration
	if config == nil {
		panic("configuration cannot be nil")
	}

	// Set Gin mode based on environment
	switch strings.ToLower(config.LogLevel) {
	case "debug":
		gin.SetMode(gin.DebugMode) // Development mode
	case "info":
		gin.SetMode(gin.ReleaseMode) // Production mode
	default:
		// Default to release mode if unspecified or invalid
		gin.SetMode(gin.ReleaseMode)
	}

	appLogger := logger.GetLogger()
	appLogger.Info(
		"Application starting",
		logger.String("app_name", config.AppName),
		logger.String("log_level", config.LogLevel),
		logger.String("env", config.Env),
	)

	// Log additional details for debugging
	if strings.ToLower(config.LogLevel) == "debug" {
		appLogger.Debug("Debugging mode enabled. Verbose logging is active.")
	}
}
