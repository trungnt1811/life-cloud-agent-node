package usecases

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/contracts"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/queryfields"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	domainerrors "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
	loggerpkg "github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

const (
	eqfErrCodeInvalidFieldCode = "INVALID_FIELD_CODE"
	eqfErrCodeUnknownFieldCode = "UNKNOWN_FIELD_CODE"
	eqfErrCodeInvalidUpdatedBy = "INVALID_UPDATED_BY"
	eqfErrCodeRepositoryError  = "ENABLED_QUERY_FIELD_REPOSITORY_ERROR"
	eqfErrInvalidFieldMessage  = "field_code is required"
	eqfErrUnknownFieldMessage  = "field_code is not in schema v1"
	eqfErrInvalidUpdatedByMsg  = "updated_by is required"
	eqfErrInternalMessage      = "failed to process enabled query field request"
)

// enabledQueryFieldUseCase contains D5 whitelist admin business logic.
type enabledQueryFieldUseCase struct {
	repo   repositories.EnabledQueryFieldRepository
	logger loggerpkg.Logger
}

// NewEnabledQueryFieldUseCase creates a new enabled-query-field use case.
func NewEnabledQueryFieldUseCase(
	repo repositories.EnabledQueryFieldRepository,
	logger loggerpkg.Logger,
) interfaces.EnabledQueryFieldUseCase {
	if logger == nil {
		logger = loggerpkg.GetLogger()
	}
	return &enabledQueryFieldUseCase{
		repo:   repo,
		logger: logger,
	}
}

// ListQueryFields returns all schema v1 field codes merged with this node's state.
func (u *enabledQueryFieldUseCase) ListQueryFields(ctx context.Context) ([]*contracts.EnabledQueryFieldOutput, error) {
	if u.repo == nil {
		u.logger.Error("Enabled query field repository is not configured")
		return nil, domainerrors.NewInternalError(eqfErrCodeRepositoryError, eqfErrInternalMessage)
	}

	stored, err := u.repo.ListAll(ctx)
	if err != nil {
		u.logger.Error("Failed to list enabled query fields", loggerpkg.Err(err))
		return nil, domainerrors.NewInternalErrorWithCause(eqfErrCodeRepositoryError, eqfErrInternalMessage, err)
	}

	byCode := make(map[string]*entities.EnabledQueryField, len(stored))
	for _, entity := range stored {
		if entity == nil {
			continue
		}
		byCode[entity.FieldCode()] = entity
	}

	out := make([]*contracts.EnabledQueryFieldOutput, 0, len(queryfields.SchemaV1FieldCodes))
	for _, code := range queryfields.SchemaV1FieldCodes {
		if entity := byCode[code]; entity != nil {
			out = append(out, enabledQueryFieldOutputFromEntity(entity))
			continue
		}
		// Never-persisted codes stay disabled with no audit timestamp.
		out = append(out, &contracts.EnabledQueryFieldOutput{
			FieldCode: code,
			Enabled:   false,
		})
	}
	return out, nil
}

// UpdateQueryField toggles a schema v1 field on this node's whitelist.
func (u *enabledQueryFieldUseCase) UpdateQueryField(
	ctx context.Context,
	input contracts.UpdateEnabledQueryFieldInput,
) (*contracts.EnabledQueryFieldOutput, error) {
	fieldCode := queryfields.NormalizeFieldCode(input.FieldCode)
	if fieldCode == "" {
		return nil, domainerrors.NewValidationError(eqfErrCodeInvalidFieldCode, eqfErrInvalidFieldMessage).
			WithDetails(domainerrors.ErrorDetail{
				Field:   "field_code",
				Code:    eqfErrCodeInvalidFieldCode,
				Message: eqfErrInvalidFieldMessage,
			})
	}
	if !queryfields.IsSchemaV1FieldCode(fieldCode) {
		return nil, domainerrors.NewValidationError(eqfErrCodeUnknownFieldCode, eqfErrUnknownFieldMessage).
			WithDetails(domainerrors.ErrorDetail{
				Field:   "field_code",
				Code:    eqfErrCodeUnknownFieldCode,
				Message: eqfErrUnknownFieldMessage,
			})
	}

	updatedBy := strings.TrimSpace(input.UpdatedBy)
	if updatedBy == "" {
		return nil, domainerrors.NewValidationError(eqfErrCodeInvalidUpdatedBy, eqfErrInvalidUpdatedByMsg).
			WithDetails(domainerrors.ErrorDetail{
				Field:   "updated_by",
				Code:    eqfErrCodeInvalidUpdatedBy,
				Message: eqfErrInvalidUpdatedByMsg,
			})
	}
	if u.repo == nil {
		u.logger.Error("Enabled query field repository is not configured")
		return nil, domainerrors.NewInternalError(eqfErrCodeRepositoryError, eqfErrInternalMessage)
	}

	now := time.Now().UTC()
	entity, err := u.repo.GetByFieldCode(ctx, fieldCode)
	if err != nil {
		u.logger.Error("Failed to get enabled query field", loggerpkg.String("field_code", fieldCode), loggerpkg.Err(err))
		return nil, domainerrors.NewInternalErrorWithCause(eqfErrCodeRepositoryError, eqfErrInternalMessage, err)
	}
	if entity == nil {
		entity = entities.NewEnabledQueryField(fieldCode, input.Enabled, updatedBy, now)
	} else {
		entity.SetEnabled(input.Enabled, updatedBy, now)
	}

	if err := u.repo.Upsert(ctx, entity); err != nil {
		u.logger.Error("Failed to upsert enabled query field", loggerpkg.String("field_code", fieldCode), loggerpkg.Err(err))
		return nil, u.wrapFailure(err)
	}

	return enabledQueryFieldOutputFromEntity(entity), nil
}

func (u *enabledQueryFieldUseCase) wrapFailure(err error) error {
	var domainErr *domainerrors.DomainError
	if errors.As(err, &domainErr) {
		return err
	}
	return domainerrors.NewInternalErrorWithCause(eqfErrCodeRepositoryError, eqfErrInternalMessage, err)
}

func enabledQueryFieldOutputFromEntity(entity *entities.EnabledQueryField) *contracts.EnabledQueryFieldOutput {
	if entity == nil {
		return nil
	}
	updatedAt := entity.UpdatedAt()
	var updatedAtPtr *time.Time
	if !updatedAt.IsZero() {
		updatedAtPtr = &updatedAt
	}
	return &contracts.EnabledQueryFieldOutput{
		FieldCode: entity.FieldCode(),
		Enabled:   entity.Enabled(),
		UpdatedAt: updatedAtPtr,
		UpdatedBy: entity.UpdatedBy(),
	}
}
