package di

import (
	"context"

	"gorm.io/gorm"

	"github.com/lifenetwork-ai/go-backend-template/internal/adapters/repositories"
	transactionrepo "github.com/lifenetwork-ai/go-backend-template/internal/adapters/repositories/transaction"
	"github.com/lifenetwork-ai/go-backend-template/internal/di/instances"
	repoInterfaces "github.com/lifenetwork-ai/go-backend-template/internal/domain/repositories"
	"github.com/lifenetwork-ai/go-backend-template/internal/domain/usecases"
	"github.com/lifenetwork-ai/go-backend-template/internal/domain/usecases/interfaces"
	"github.com/lifenetwork-ai/go-backend-template/internal/platform/logger"
	"github.com/lifenetwork-ai/go-backend-template/internal/runtimeconfig"
)

// Repos holds all initialized repositories.
type Repos struct {
	ExampleRepo        repoInterfaces.ExampleRepository
	TransactionManager repoInterfaces.TransactionManager
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
		ExampleRepo: exampleRepo,
		TransactionManager: transactionrepo.NewTransactionManager(db, transactionrepo.TransactionManagerDeps{
			ExampleRepo: exampleRepo,
		}),
	}
}

// UseCases holds all initialized use cases.
type UseCases struct {
	ExampleUseCase interfaces.ExampleUseCase
}

// InitializeUseCases initializes all use cases.
func InitializeUseCases(repos *Repos, appLogger logger.Logger) *UseCases {
	var exampleRepo repoInterfaces.ExampleRepository
	if repos != nil {
		exampleRepo = repos.ExampleRepo
	}

	return &UseCases{
		ExampleUseCase: usecases.NewExampleUseCase(exampleRepo, transactionManager(repos), appLogger),
	}
}

func transactionManager(repos *Repos) repoInterfaces.TransactionManager {
	if repos == nil {
		return nil
	}
	return repos.TransactionManager
}
