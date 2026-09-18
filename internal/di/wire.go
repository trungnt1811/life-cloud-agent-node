package di

import (
	"context"

	"gorm.io/gorm"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	transactionrepo "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/transaction"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/di/instances"
	repoInterfaces "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/runtimeconfig"
)

// Repos holds all initialized repositories.
type Repos struct {
	ExampleRepo           repoInterfaces.ExampleRepository
	EnabledQueryFieldRepo repoInterfaces.EnabledQueryFieldRepository
	TransactionManager    repoInterfaces.TransactionManager
}

// InitializeRepos initializes all repositories.
func InitializeRepos(ctx context.Context, db *gorm.DB, appLogger logger.Logger, configs runtimeconfig.ModuleConfigs) *Repos {
	if ctx == nil {
		ctx = context.Background()
	}
	baseExampleRepo := repositories.NewExampleRepository(db, appLogger)
	exampleRepo := repositories.NewExampleRepositoryCache(
		baseExampleRepo,
		instances.CacheRepositoryInstance(ctx, configs.Cache),
		appLogger,
	)

	return &Repos{
		ExampleRepo:           exampleRepo,
		EnabledQueryFieldRepo: repositories.NewEnabledQueryFieldRepository(db, appLogger),
		TransactionManager: transactionrepo.NewTransactionManager(db, transactionrepo.TransactionManagerDeps{
			ExampleRepo: exampleRepo,
		}),
	}
}

// UseCases holds all initialized use cases.
type UseCases struct {
	ExampleUseCase           interfaces.ExampleUseCase
	EnabledQueryFieldUseCase interfaces.EnabledQueryFieldUseCase
}

// InitializeUseCases initializes all use cases.
func InitializeUseCases(repos *Repos, appLogger logger.Logger) *UseCases {
	var exampleRepo repoInterfaces.ExampleRepository
	var enabledQueryFieldRepo repoInterfaces.EnabledQueryFieldRepository
	if repos != nil {
		exampleRepo = repos.ExampleRepo
		enabledQueryFieldRepo = repos.EnabledQueryFieldRepo
	}

	return &UseCases{
		ExampleUseCase:           usecases.NewExampleUseCase(exampleRepo, transactionManager(repos), appLogger),
		EnabledQueryFieldUseCase: usecases.NewEnabledQueryFieldUseCase(enabledQueryFieldRepo, appLogger),
	}
}

func transactionManager(repos *Repos) repoInterfaces.TransactionManager {
	if repos == nil {
		return nil
	}
	return repos.TransactionManager
}
