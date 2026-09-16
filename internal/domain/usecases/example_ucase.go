package usecases

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/contracts"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	domaintypes "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	domainerrors "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
	loggerpkg "github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

const (
	exampleErrCodeInvalidExampleID = "INVALID_EXAMPLE_ID"
	exampleErrCodeInvalidName      = "INVALID_NAME"
	exampleErrCodeInvalidPage      = "INVALID_PAGE"
	exampleErrCodeExampleNotFound  = "EXAMPLE_NOT_FOUND"
	exampleErrCodeRepositoryError  = "EXAMPLE_REPOSITORY_ERROR"
	exampleErrCodeTxManagerError   = "EXAMPLE_TRANSACTION_MANAGER_ERROR"

	exampleErrInvalidExampleIDMessage = "invalid example ID"
	exampleErrInvalidNameMessage      = "example name is required"
	exampleErrInvalidPageMessage      = "invalid pagination parameters"
	exampleErrExampleNotFoundMessage  = "example not found"
	exampleErrInternalMessage         = "failed to process example request"
)

var errExampleTxRepositoryMissing = errors.New("example transaction repository is not configured")

// exampleUseCase contains example business logic.
type exampleUseCase struct {
	exampleRepo repositories.ExampleRepository
	txManager   repositories.TransactionManager
	logger      loggerpkg.Logger
}

// NewExampleUseCase creates a new example use case.
func NewExampleUseCase(
	exampleRepo repositories.ExampleRepository,
	txManager repositories.TransactionManager,
	logger loggerpkg.Logger,
) interfaces.ExampleUseCase {
	if logger == nil {
		logger = loggerpkg.GetLogger()
	}
	return &exampleUseCase{
		exampleRepo: exampleRepo,
		txManager:   txManager,
		logger:      logger,
	}
}

// GetExample retrieves an example record from persistence.
func (u *exampleUseCase) GetExample(ctx context.Context, id uuid.UUID) (*contracts.ExampleOutput, error) {
	if id == uuid.Nil {
		return nil, invalidExampleIDError()
	}
	if u.exampleRepo == nil {
		u.logger.Error("Example repository is not configured")
		return nil, domainerrors.NewInternalError(exampleErrCodeRepositoryError, exampleErrInternalMessage)
	}

	entity, err := u.exampleRepo.GetByID(ctx, id)
	if err != nil {
		u.logger.Error("Failed to get example", loggerpkg.String("example_id", id.String()), loggerpkg.Err(err))
		return nil, domainerrors.NewInternalErrorWithCause(exampleErrCodeRepositoryError, exampleErrInternalMessage, err)
	}
	if entity == nil {
		return nil, domainerrors.NewNotFoundError(exampleErrCodeExampleNotFound, exampleErrExampleNotFoundMessage)
	}

	return exampleOutputFromEntity(entity), nil
}

// ListExamples retrieves a paginated list of examples.
func (u *exampleUseCase) ListExamples(
	ctx context.Context,
	input contracts.ListExamplesInput,
) (*domaintypes.PaginatedResponse[*contracts.ExampleOutput], error) {
	page, pageSize, err := normalizeListExamplesInput(input)
	if err != nil {
		return nil, err
	}
	if u.exampleRepo == nil {
		u.logger.Error("Example repository is not configured")
		return nil, domainerrors.NewInternalError(exampleErrCodeRepositoryError, exampleErrInternalMessage)
	}

	total, err := u.exampleRepo.Count(ctx)
	if err != nil {
		u.logger.Error("Failed to count examples", loggerpkg.Err(err))
		return nil, domainerrors.NewInternalErrorWithCause(exampleErrCodeRepositoryError, exampleErrInternalMessage, err)
	}

	offset := (page - 1) * pageSize
	entitiesList, err := u.exampleRepo.List(ctx, pageSize, offset)
	if err != nil {
		u.logger.Error("Failed to list examples", loggerpkg.Int("page", page), loggerpkg.Int("page_size", pageSize), loggerpkg.Err(err))
		return nil, domainerrors.NewInternalErrorWithCause(exampleErrCodeRepositoryError, exampleErrInternalMessage, err)
	}

	return &domaintypes.PaginatedResponse[*contracts.ExampleOutput]{
		Items:      exampleOutputsFromEntities(entitiesList),
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
		NextPage:   nextExamplePage(page, pageSize, offset, len(entitiesList), total),
	}, nil
}

// CreateExample validates input, persists an example record, and returns its DTO.
func (u *exampleUseCase) CreateExample(
	ctx context.Context,
	input contracts.CreateExampleInput,
) (*contracts.ExampleOutput, error) {
	name, description, err := normalizeExampleDetails(input.Name, input.Description)
	if err != nil {
		return nil, err
	}

	entity := entities.NewExampleEntity(uuid.New(), name, description, time.Now().UTC())
	if err := u.writeExampleInTx(ctx, func(repo repositories.ExampleRepository) error {
		return repo.Create(ctx, entity)
	}); err != nil {
		u.logger.Error("Failed to create example", loggerpkg.String("name", name), loggerpkg.Err(err))
		return nil, u.wrapExampleFailure(err)
	}

	return exampleOutputFromEntity(entity), nil
}

// UpdateExample validates input, updates an existing example, and returns the updated record.
func (u *exampleUseCase) UpdateExample(
	ctx context.Context,
	input contracts.UpdateExampleInput,
) (*contracts.ExampleOutput, error) {
	if input.ID == uuid.Nil {
		return nil, invalidExampleIDError()
	}
	name, description, err := normalizeExampleDetails(input.Name, input.Description)
	if err != nil {
		return nil, err
	}

	var updated *entities.ExampleEntity
	if err := u.writeExampleInTx(ctx, func(repo repositories.ExampleRepository) error {
		entity, lookupErr := repo.GetByID(ctx, input.ID)
		if lookupErr != nil {
			return lookupErr
		}
		if entity == nil {
			return domainerrors.NewNotFoundError(exampleErrCodeExampleNotFound, exampleErrExampleNotFoundMessage)
		}

		entity.UpdateDetails(name, description, time.Now().UTC())
		if updateErr := repo.Update(ctx, entity); updateErr != nil {
			return updateErr
		}
		updated = entity
		return nil
	}); err != nil {
		u.logger.Error("Failed to update example", loggerpkg.String("example_id", input.ID.String()), loggerpkg.Err(err))
		return nil, u.wrapExampleFailure(err)
	}

	return exampleOutputFromEntity(updated), nil
}

// DeleteExample removes an example record by ID.
func (u *exampleUseCase) DeleteExample(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return invalidExampleIDError()
	}

	if err := u.writeExampleInTx(ctx, func(repo repositories.ExampleRepository) error {
		entity, lookupErr := repo.GetByID(ctx, id)
		if lookupErr != nil {
			return lookupErr
		}
		if entity == nil {
			return domainerrors.NewNotFoundError(exampleErrCodeExampleNotFound, exampleErrExampleNotFoundMessage)
		}
		return repo.Delete(ctx, id)
	}); err != nil {
		u.logger.Error("Failed to delete example", loggerpkg.String("example_id", id.String()), loggerpkg.Err(err))
		return u.wrapExampleFailure(err)
	}

	return nil
}

func (u *exampleUseCase) writeExampleInTx(ctx context.Context, fn func(repositories.ExampleRepository) error) error {
	if u.txManager == nil {
		u.logger.Error("Example transaction manager is not configured")
		return domainerrors.NewInternalError(exampleErrCodeTxManagerError, exampleErrInternalMessage)
	}

	return u.txManager.WithinTx(ctx, func(repos repositories.TxRepositories) error {
		if repos == nil {
			return errExampleTxRepositoryMissing
		}
		repo := repos.Examples()
		if repo == nil {
			return errExampleTxRepositoryMissing
		}
		return fn(repo)
	})
}

func (u *exampleUseCase) wrapExampleFailure(err error) error {
	var domainErr *domainerrors.DomainError
	if errors.As(err, &domainErr) {
		return err
	}
	return domainerrors.NewInternalErrorWithCause(exampleErrCodeRepositoryError, exampleErrInternalMessage, err)
}

func normalizeExampleDetails(name, description string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", domainerrors.NewValidationError(exampleErrCodeInvalidName, exampleErrInvalidNameMessage).
			WithDetails(domainerrors.ErrorDetail{
				Field:   "name",
				Code:    exampleErrCodeInvalidName,
				Message: exampleErrInvalidNameMessage,
			})
	}

	return name, strings.TrimSpace(description), nil
}

func normalizeListExamplesInput(input contracts.ListExamplesInput) (int, int, error) {
	page := input.Page
	if page <= 0 {
		page = constants.DefaultPage
	}

	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = constants.DefaultPageSize
	}
	if pageSize < constants.DefaultMinPageSize || pageSize > constants.DefaultMaxPageSize {
		return 0, 0, domainerrors.NewValidationError(exampleErrCodeInvalidPage, exampleErrInvalidPageMessage).
			WithDetails(domainerrors.ErrorDetail{
				Field:   constants.AltPageSizeText,
				Code:    exampleErrCodeInvalidPage,
				Message: exampleErrInvalidPageMessage,
			})
	}

	return page, pageSize, nil
}

func invalidExampleIDError() *domainerrors.DomainError {
	return domainerrors.NewValidationError(exampleErrCodeInvalidExampleID, exampleErrInvalidExampleIDMessage).
		WithDetails(domainerrors.ErrorDetail{
			Field:   "id",
			Code:    exampleErrCodeInvalidExampleID,
			Message: exampleErrInvalidExampleIDMessage,
		})
}

func nextExamplePage(page, pageSize, offset, returned, total int) *int {
	if total <= 0 || pageSize <= 0 || offset+returned >= total {
		return nil
	}
	next := page + 1
	return &next
}

func exampleOutputFromEntity(entity *entities.ExampleEntity) *contracts.ExampleOutput {
	if entity == nil {
		return nil
	}
	return &contracts.ExampleOutput{
		ID:          entity.ID(),
		Name:        entity.Name(),
		Description: entity.Description(),
		CreatedAt:   entity.CreatedAt(),
		UpdatedAt:   entity.UpdatedAt(),
	}
}

func exampleOutputsFromEntities(entitiesList []*entities.ExampleEntity) []*contracts.ExampleOutput {
	out := make([]*contracts.ExampleOutput, 0, len(entitiesList))
	for _, entity := range entitiesList {
		if output := exampleOutputFromEntity(entity); output != nil {
			out = append(out, output)
		}
	}
	return out
}
