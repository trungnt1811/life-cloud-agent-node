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
	ExampleRepo              repoInterfaces.ExampleRepository
	EnabledQueryFieldRepo    repoInterfaces.EnabledQueryFieldRepository
	PatientRegistryRepo      repoInterfaces.PatientRegistryRepository
	JobProgressRepo          repoInterfaces.JobProgressRepository
	HospitalGovernanceRepo   repoInterfaces.HospitalGovernanceRepository
	GovernedHospitalJobsRepo repoInterfaces.GovernedHospitalJobRepository
	TransactionManager       repoInterfaces.TransactionManager
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
	hospitalGovernanceRepo := repositories.NewHospitalGovernanceRepository(db, appLogger)
	governedHospitalJobsRepo := repositories.NewGovernedHospitalJobRepository(db)

	return &Repos{
		ExampleRepo:              exampleRepo,
		EnabledQueryFieldRepo:    repositories.NewEnabledQueryFieldRepository(db, appLogger),
		PatientRegistryRepo:      repositories.NewPatientRegistryRepository(db, appLogger),
		JobProgressRepo:          repositories.NewJobProgressRepository(db, appLogger),
		HospitalGovernanceRepo:   hospitalGovernanceRepo,
		GovernedHospitalJobsRepo: governedHospitalJobsRepo,
		TransactionManager: transactionrepo.NewTransactionManager(db, transactionrepo.TransactionManagerDeps{
			ExampleRepo:              exampleRepo,
			HospitalGovernanceRepo:   hospitalGovernanceRepo,
			GovernedHospitalJobsRepo: governedHospitalJobsRepo,
		}),
	}
}

// UseCases holds all initialized use cases.
type UseCases struct {
	ExampleUseCase             interfaces.ExampleUseCase
	EnabledQueryFieldUseCase   interfaces.EnabledQueryFieldUseCase
	CohortCountUseCase         interfaces.CohortCountUseCase
	HospitalGovernanceUseCase  interfaces.HospitalGovernanceUseCase
	GovernedHospitalJobUseCase interfaces.GovernedHospitalJobUseCase
}

// InitializeUseCases initializes all use cases. jobChunkSize is decision
// 0005's resumable chunk size; below 1 means usecases.NewCohortCountUseCase's
// own default.
func InitializeUseCases(repos *Repos, jobChunkSize int, nodeID string, appLogger logger.Logger) *UseCases {
	var exampleRepo repoInterfaces.ExampleRepository
	var enabledQueryFieldRepo repoInterfaces.EnabledQueryFieldRepository
	var patientRegistryRepo repoInterfaces.PatientRegistryRepository
	var jobProgressRepo repoInterfaces.JobProgressRepository
	var hospitalGovernanceRepo repoInterfaces.HospitalGovernanceRepository
	var governedHospitalJobsRepo repoInterfaces.GovernedHospitalJobRepository
	if repos != nil {
		exampleRepo = repos.ExampleRepo
		enabledQueryFieldRepo = repos.EnabledQueryFieldRepo
		patientRegistryRepo = repos.PatientRegistryRepo
		jobProgressRepo = repos.JobProgressRepo
		hospitalGovernanceRepo = repos.HospitalGovernanceRepo
		governedHospitalJobsRepo = repos.GovernedHospitalJobsRepo
	}

	return &UseCases{
		ExampleUseCase:           usecases.NewExampleUseCase(exampleRepo, transactionManager(repos), appLogger),
		EnabledQueryFieldUseCase: usecases.NewEnabledQueryFieldUseCase(enabledQueryFieldRepo, appLogger),
		CohortCountUseCase:       usecases.NewCohortCountUseCase(patientRegistryRepo, jobProgressRepo, jobChunkSize, appLogger),
		HospitalGovernanceUseCase: usecases.NewHospitalGovernanceUseCase(usecases.HospitalGovernanceDeps{
			NodeID: nodeID, Repository: hospitalGovernanceRepo, Transactions: transactionManager(repos),
		}),
		GovernedHospitalJobUseCase: usecases.NewGovernedHospitalJobUseCase(usecases.GovernedHospitalJobDeps{NodeID: nodeID, Repository: governedHospitalJobsRepo, Transactions: transactionManager(repos)}),
	}
}

func transactionManager(repos *Repos) repoInterfaces.TransactionManager {
	if repos == nil {
		return nil
	}
	return repos.TransactionManager
}
