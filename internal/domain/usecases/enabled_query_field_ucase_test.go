package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/contracts"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
	domainerrors "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestEnabledQueryFieldUseCase_ListMergesSchemaWithStoredState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	repo.EXPECT().
		ListAll(gomock.Any()).
		Return([]*entities.EnabledQueryField{
			entities.NewEnabledQueryField("HB", true, "alice", now),
		}, nil)

	uc := usecases.NewEnabledQueryFieldUseCase(repo, nil)
	out, err := uc.ListQueryFields(context.Background())
	require.NoError(t, err)
	require.Len(t, out, len(queryfields.SchemaV1FieldCodes))
	require.Equal(t, "HB", out[0].FieldCode)
	require.True(t, out[0].Enabled)
	require.Equal(t, "alice", out[0].UpdatedBy)
	require.Equal(t, "MCV", out[1].FieldCode)
	require.False(t, out[1].Enabled)
	require.Empty(t, out[1].UpdatedBy)
}

func TestEnabledQueryFieldUseCase_UpdateRejectsUnknownField(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := usecases.NewEnabledQueryFieldUseCase(mocks.NewMockEnabledQueryFieldRepository(ctrl), nil)
	_, err := uc.UpdateQueryField(context.Background(), contracts.UpdateEnabledQueryFieldInput{
		FieldCode: "NOT_A_FIELD",
		Enabled:   true,
		UpdatedBy: "alice",
	})
	require.Error(t, err)
	var domainErr *domainerrors.DomainError
	require.ErrorAs(t, err, &domainErr)
	require.Equal(t, "UNKNOWN_FIELD_CODE", domainErr.Code)
}

func TestEnabledQueryFieldUseCase_UpdateUpsertsEnabledField(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEnabledQueryFieldRepository(ctrl)
	repo.EXPECT().GetByFieldCode(gomock.Any(), "HB").Return(nil, nil)
	repo.EXPECT().
		Upsert(gomock.Any(), gomock.AssignableToTypeOf(&entities.EnabledQueryField{})).
		DoAndReturn(func(_ context.Context, entity *entities.EnabledQueryField) error {
			require.Equal(t, "HB", entity.FieldCode())
			require.True(t, entity.Enabled())
			require.Equal(t, "alice", entity.UpdatedBy())
			return nil
		})

	uc := usecases.NewEnabledQueryFieldUseCase(repo, nil)
	out, err := uc.UpdateQueryField(context.Background(), contracts.UpdateEnabledQueryFieldInput{
		FieldCode: "hb",
		Enabled:   true,
		UpdatedBy: "alice",
	})
	require.NoError(t, err)
	require.Equal(t, "HB", out.FieldCode)
	require.True(t, out.Enabled)
	require.Equal(t, "alice", out.UpdatedBy)
}
