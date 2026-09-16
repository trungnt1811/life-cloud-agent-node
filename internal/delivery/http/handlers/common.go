package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	httpresponse "github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/response"
	domainerrors "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

// HandleDomainError is a centralized error handler for domain errors
func HandleDomainError(ctx *gin.Context, err *domainerrors.DomainError) {
	switch err.Type {
	case domainerrors.ErrorTypeValidation:
		writeDomainError(ctx, http.StatusBadRequest, err)
	case domainerrors.ErrorTypeNotFound:
		writeDomainError(ctx, http.StatusNotFound, err)
	case domainerrors.ErrorTypeUnauthorized:
		writeDomainError(ctx, http.StatusUnauthorized, err)
	case domainerrors.ErrorTypeConflict:
		writeDomainError(ctx, http.StatusConflict, err)
	case domainerrors.ErrorTypeRateLimit:
		writeDomainError(ctx, http.StatusTooManyRequests, err)
	case domainerrors.ErrorTypeInternal:
		// Log internal errors for debugging
		logger.GetLogger().Error("Internal domain error", logger.String("code", err.Code), logger.Err(err))
		writeDomainError(ctx, http.StatusInternalServerError, err)
	default:
		// Fallback for unknown error types
		logger.GetLogger().Error("Unknown domain error type", logger.String("code", err.Code), logger.Err(err))
		writeDomainError(ctx, http.StatusInternalServerError, err)
	}
}

func writeDomainError(ctx *gin.Context, status int, err *domainerrors.DomainError) {
	httpresponse.Error(ctx, status, err.Code, err.Message, err.Details)
}
