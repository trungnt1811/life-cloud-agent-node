package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/contracts"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	domainrepos "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	domainerrors "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func requireExampleDomainError(
	t *testing.T,
	err error,
	errType domainerrors.ErrorType,
	code string,
) {
	t.Helper()

	var domainErr *domainerrors.DomainError
	require.ErrorAs(t, err, &domainErr)
	require.Equal(t, errType, domainErr.Type)
	require.Equal(t, code, domainErr.Code)
}

func requireExampleDomainErrorDetail(t *testing.T, err error, field string) {
	t.Helper()

	var domainErr *domainerrors.DomainError
	require.ErrorAs(t, err, &domainErr)
	require.NotEmpty(t, domainErr.Details)
	require.Equal(t, field, domainErr.Details[0].Field)
}

func expectExampleTx(
	t *testing.T,
	ctrl *gomock.Controller,
	txManager *mocks.MockTransactionManager,
	repo domainrepos.ExampleRepository,
) {
	t.Helper()

	txRepos := mocks.NewMockTxRepositories(ctrl)
	txManager.EXPECT().
		WithinTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(domainrepos.TxRepositories) error) error {
			txRepos.EXPECT().Examples().Return(repo).Times(1)
			return fn(txRepos)
		}).
		Times(1)
}

func TestExampleUseCase_GetExample(t *testing.T) {
	ctx := context.Background()
	exampleID := uuid.New()
	now := time.Now().UTC()

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := mocks.NewMockExampleRepository(ctrl)
		entity := entities.NewExampleEntity(exampleID, "Alice", "first example", now)
		repo.EXPECT().GetByID(ctx, exampleID).Return(entity, nil).Times(1)

		got, err := NewExampleUseCase(repo, nil, nil).GetExample(ctx, exampleID)

		require.NoError(t, err)
		require.Equal(t, &contracts.ExampleOutput{
			ID:          exampleID,
			Name:        "Alice",
			Description: "first example",
			CreatedAt:   now,
			UpdatedAt:   now,
		}, got)
	})

	t.Run("invalid_id", func(t *testing.T) {
		got, err := NewExampleUseCase(nil, nil, nil).GetExample(ctx, uuid.Nil)

		require.Nil(t, got)
		requireExampleDomainError(t, err, domainerrors.ErrorTypeValidation, exampleErrCodeInvalidExampleID)
		requireExampleDomainErrorDetail(t, err, "id")
	})

	t.Run("not_found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := mocks.NewMockExampleRepository(ctrl)
		repo.EXPECT().GetByID(ctx, exampleID).Return(nil, nil).Times(1)

		got, err := NewExampleUseCase(repo, nil, nil).GetExample(ctx, exampleID)

		require.Nil(t, got)
		requireExampleDomainError(t, err, domainerrors.ErrorTypeNotFound, exampleErrCodeExampleNotFound)
	})

	t.Run("repository_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := mocks.NewMockExampleRepository(ctrl)
		repoErr := errors.New("database error")
		repo.EXPECT().GetByID(ctx, exampleID).Return(nil, repoErr).Times(1)

		got, err := NewExampleUseCase(repo, nil, nil).GetExample(ctx, exampleID)

		require.Nil(t, got)
		requireExampleDomainError(t, err, domainerrors.ErrorTypeInternal, exampleErrCodeRepositoryError)
	})
}

func TestExampleUseCase_ListExamples(t *testing.T) {
	ctx := context.Background()
	first := entities.NewExampleEntity(uuid.New(), "first", "one", time.Now().UTC())
	second := entities.NewExampleEntity(uuid.New(), "second", "two", time.Now().UTC())

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := mocks.NewMockExampleRepository(ctrl)
		repo.EXPECT().Count(ctx).Return(8, nil).Times(1)
		repo.EXPECT().List(ctx, constants.DefaultMinPageSize, constants.DefaultMinPageSize).Return([]*entities.ExampleEntity{first, second}, nil).Times(1)

		got, err := NewExampleUseCase(repo, nil, nil).ListExamples(ctx, contracts.ListExamplesInput{
			Page:     2,
			PageSize: constants.DefaultMinPageSize,
		})

		require.NoError(t, err)
		require.Len(t, got.Items, 2)
		require.Equal(t, 8, got.TotalCount)
		require.Equal(t, 2, got.Page)
		require.Equal(t, constants.DefaultMinPageSize, got.PageSize)
		require.NotNil(t, got.NextPage)
		require.Equal(t, 3, *got.NextPage)
	})

	t.Run("default_pagination", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := mocks.NewMockExampleRepository(ctrl)
		repo.EXPECT().Count(ctx).Return(0, nil).Times(1)
		repo.EXPECT().List(ctx, constants.DefaultPageSize, 0).Return([]*entities.ExampleEntity{}, nil).Times(1)

		got, err := NewExampleUseCase(repo, nil, nil).ListExamples(ctx, contracts.ListExamplesInput{})

		require.NoError(t, err)
		require.Empty(t, got.Items)
		require.Equal(t, 0, got.TotalCount)
		require.Equal(t, constants.DefaultPage, got.Page)
		require.Equal(t, constants.DefaultPageSize, got.PageSize)
		require.Nil(t, got.NextPage)
	})

	t.Run("invalid_page_size", func(t *testing.T) {
		got, err := NewExampleUseCase(nil, nil, nil).ListExamples(ctx, contracts.ListExamplesInput{
			Page:     1,
			PageSize: constants.DefaultMaxPageSize + 1,
		})

		require.Nil(t, got)
		requireExampleDomainError(t, err, domainerrors.ErrorTypeValidation, exampleErrCodeInvalidPage)
		requireExampleDomainErrorDetail(t, err, constants.AltPageSizeText)
	})
}

func TestExampleUseCase_CreateExample(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := mocks.NewMockExampleRepository(ctrl)
		txManager := mocks.NewMockTransactionManager(ctrl)
		expectExampleTx(t, ctrl, txManager, repo)
		repo.EXPECT().
			Create(ctx, gomock.Any()).
			DoAndReturn(func(ctx context.Context, entity *entities.ExampleEntity) error {
				require.NotEqual(t, uuid.Nil, entity.ID())
				require.Equal(t, "Alice", entity.Name())
				require.Equal(t, "first example", entity.Description())
				return nil
			}).
			Times(1)

		got, err := NewExampleUseCase(repo, txManager, nil).CreateExample(ctx, contracts.CreateExampleInput{
			Name:        "  Alice  ",
			Description: " first example ",
		})

		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, got.ID)
		require.Equal(t, "Alice", got.Name)
		require.Equal(t, "first example", got.Description)
	})

	t.Run("validation_error", func(t *testing.T) {
		got, err := NewExampleUseCase(nil, nil, nil).CreateExample(ctx, contracts.CreateExampleInput{
			Name: "   ",
		})

		require.Nil(t, got)
		requireExampleDomainError(t, err, domainerrors.ErrorTypeValidation, exampleErrCodeInvalidName)
		requireExampleDomainErrorDetail(t, err, "name")
	})

	t.Run("missing_transaction_manager", func(t *testing.T) {
		got, err := NewExampleUseCase(nil, nil, nil).CreateExample(ctx, contracts.CreateExampleInput{
			Name: "Alice",
		})

		require.Nil(t, got)
		requireExampleDomainError(t, err, domainerrors.ErrorTypeInternal, exampleErrCodeTxManagerError)
	})

	t.Run("repository_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := mocks.NewMockExampleRepository(ctrl)
		txManager := mocks.NewMockTransactionManager(ctrl)
		repoErr := errors.New("database error")
		expectExampleTx(t, ctrl, txManager, repo)
		repo.EXPECT().Create(ctx, gomock.Any()).Return(repoErr).Times(1)

		got, err := NewExampleUseCase(repo, txManager, nil).CreateExample(ctx, contracts.CreateExampleInput{
			Name: "Alice",
		})

		require.Nil(t, got)
		requireExampleDomainError(t, err, domainerrors.ErrorTypeInternal, exampleErrCodeRepositoryError)
	})
}

func TestExampleUseCase_UpdateExample(t *testing.T) {
	ctx := context.Background()
	exampleID := uuid.New()

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := mocks.NewMockExampleRepository(ctrl)
		txManager := mocks.NewMockTransactionManager(ctrl)
		entity := entities.NewExampleEntity(exampleID, "Alice", "old", time.Now().UTC())
		expectExampleTx(t, ctrl, txManager, repo)
		repo.EXPECT().GetByID(ctx, exampleID).Return(entity, nil).Times(1)
		repo.EXPECT().Update(ctx, entity).Return(nil).Times(1)

		got, err := NewExampleUseCase(repo, txManager, nil).UpdateExample(ctx, contracts.UpdateExampleInput{
			ID:          exampleID,
			Name:        "  Bob  ",
			Description: " updated ",
		})

		require.NoError(t, err)
		require.Equal(t, exampleID, got.ID)
		require.Equal(t, "Bob", got.Name)
		require.Equal(t, "updated", got.Description)
	})

	t.Run("not_found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := mocks.NewMockExampleRepository(ctrl)
		txManager := mocks.NewMockTransactionManager(ctrl)
		expectExampleTx(t, ctrl, txManager, repo)
		repo.EXPECT().GetByID(ctx, exampleID).Return(nil, nil).Times(1)

		got, err := NewExampleUseCase(repo, txManager, nil).UpdateExample(ctx, contracts.UpdateExampleInput{
			ID:   exampleID,
			Name: "Bob",
		})

		require.Nil(t, got)
		requireExampleDomainError(t, err, domainerrors.ErrorTypeNotFound, exampleErrCodeExampleNotFound)
	})
}

func TestExampleUseCase_DeleteExample(t *testing.T) {
	ctx := context.Background()
	exampleID := uuid.New()

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := mocks.NewMockExampleRepository(ctrl)
		txManager := mocks.NewMockTransactionManager(ctrl)
		entity := entities.NewExampleEntity(exampleID, "Alice", "old", time.Now().UTC())
		expectExampleTx(t, ctrl, txManager, repo)
		repo.EXPECT().GetByID(ctx, exampleID).Return(entity, nil).Times(1)
		repo.EXPECT().Delete(ctx, exampleID).Return(nil).Times(1)

		err := NewExampleUseCase(repo, txManager, nil).DeleteExample(ctx, exampleID)

		require.NoError(t, err)
	})

	t.Run("invalid_id", func(t *testing.T) {
		err := NewExampleUseCase(nil, nil, nil).DeleteExample(ctx, uuid.Nil)

		requireExampleDomainError(t, err, domainerrors.ErrorTypeValidation, exampleErrCodeInvalidExampleID)
		requireExampleDomainErrorDetail(t, err, "id")
	})
}
