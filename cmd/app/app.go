package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/client"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/postgres"
	httprouter "github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/router"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/di"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/di/instances"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/runtimeconfig"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/server"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/workers"
)

func RunApp(config *conf.Configuration) error {
	return Run(context.Background(), config)
}

func Run(ctx context.Context, config *conf.Configuration) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	if err := conf.ValidateConfiguration(*config); err != nil {
		return err
	}
	if strings.TrimSpace(config.ControlCenterAddress) != "" {
		if err := federatedClientConfig(config).Validate(); err != nil {
			return err
		}
	}
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
	advisories := client.NewAdvisoryStore()
	r := SetupRouter(runCtx, db, config, advisories, appLogger)
	workerDone, workerErrC := startFederatedClientWorker(runCtx, db, config, advisories, appLogger)
	httpServer, serverErrC := server.StartHTTP(r, server.HTTPConfig{Port: config.AppPort}, appLogger)
	runErr := server.WaitForShutdownSignal(runCtx, cancel, httpServer,
		mergeAppErrorChannels(runCtx, serverErrC, workerErrC), server.GracefulTimeout(0), appLogger)

	// cancel() (called by WaitForShutdownSignal) already told the worker to
	// stop; give it the same graceful window as the HTTP server so its
	// in-flight control stream closes cleanly instead of being cut off by
	// process exit.
	if workerDone != nil {
		select {
		case <-workerDone:
		case <-time.After(server.GracefulTimeout(0)):
			appLogger.Warn("Federated client worker did not stop within the graceful shutdown timeout")
		}
	}
	return runErr
}

// SetupRouter initializes the complete HTTP router for production and
// integration tests. advisories backs GET /admin/status (decision 0003); a
// nil store just means the endpoint always reports no advisory received.
func SetupRouter(
	ctx context.Context,
	db *gorm.DB,
	config *conf.Configuration,
	advisories *client.AdvisoryStore,
	appLogger logger.Logger,
) *gin.Engine {
	if ctx == nil {
		ctx = context.Background()
	}
	repos := di.InitializeRepos(ctx, db, appLogger, runtimeconfig.ModuleConfigsFromConfiguration(config))
	useCases := di.InitializeUseCases(repos, config.JobChunkSize, config.NodeID, appLogger)
	options := httprouter.OptionsFromConfiguration(config)
	options.Advisories = advisories
	return httprouter.SetupWithDependencies(useCases, appLogger, options)
}

// startFederatedClientWorker starts the gRPC control-center client
// (decision 0001) in the background when CONTROL_CENTER_ADDRESS is
// configured. It shuts down when ctx is done, alongside the HTTP server. The
// returned channel closes once the worker has stopped, or is nil when no
// worker was started. A blank address is a valid deployment (no control
// center to dial yet) and only logs, matching the admin-routes
// partial-config pattern elsewhere.
func startFederatedClientWorker(
	ctx context.Context,
	db *gorm.DB,
	config *conf.Configuration,
	advisories *client.AdvisoryStore,
	appLogger logger.Logger,
) (<-chan struct{}, <-chan error) {
	if strings.TrimSpace(config.ControlCenterAddress) == "" {
		appLogger.Warn("Federated client worker not started; CONTROL_CENTER_ADDRESS is not set")
		return nil, nil
	}

	repos := di.InitializeRepos(ctx, db, appLogger, runtimeconfig.ModuleConfigsFromConfiguration(config))
	useCases := di.InitializeUseCases(repos, config.JobChunkSize, config.NodeID, appLogger)

	nodeClient := client.NewNodeClient(federatedClientConfig(config), client.Dependencies{
		EnabledQueryFieldRepo: repos.EnabledQueryFieldRepo,
		CohortCounter:         useCases.CohortCountUseCase,
		SuppressionThreshold:  config.SuppressionThreshold,
		Advisories:            advisories,
		HospitalGovernance:    useCases.HospitalGovernanceUseCase,
		GovernedHospitalJobs:  useCases.GovernedHospitalJobUseCase,
	}, appLogger, nil)

	worker := workers.NewFederatedClientWorker(nodeClient, appLogger)
	done := make(chan struct{})
	errC := make(chan error, 1)
	go func() {
		defer close(done)
		if err := worker.Run(ctx); err != nil {
			errC <- err
		} else if ctx.Err() == nil {
			errC <- fmt.Errorf("federated client stopped unexpectedly")
		}
	}()
	return done, errC
}

func mergeAppErrorChannels(ctx context.Context, serverErrC, workerErrC <-chan error) <-chan error {
	out := make(chan error, 1)
	go func() {
		select {
		case err := <-serverErrC:
			if err != nil {
				out <- fmt.Errorf("http server: %w", err)
			} else {
				out <- nil
			}
		case err := <-workerErrC:
			out <- fmt.Errorf("federated client worker: %w", err)
		case <-ctx.Done():
		}
	}()
	return out
}

func federatedClientConfig(config *conf.Configuration) client.Config {
	return client.Config{
		Address:                  config.ControlCenterAddress,
		NodeID:                   config.NodeID,
		AgentVersion:             config.AgentVersion,
		QuerySchemaVersion:       1,
		GovernanceSyncEnabled:    config.GovernanceSyncEnabled,
		GovernedExecutionEnabled: config.GovernedExecutionEnabled,
		TLS: client.TLSConfig{
			Insecure:       config.ControlCenterInsecure,
			CAFile:         config.ControlCenterCAFile,
			ClientCertFile: config.ControlCenterClientCertFile,
			ClientKeyFile:  config.ControlCenterClientKeyFile,
		},
	}
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
