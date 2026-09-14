package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDomainErrorWithDetailsKeepsTypedDetails(t *testing.T) {
	err := NewValidationError("INVALID_NAME", "name is required").
		WithDetails(ErrorDetail{
			Field:   "name",
			Code:    "REQUIRED",
			Message: "name is required",
		})

	require.Equal(t, []ErrorDetail{
		{
			Field:   "name",
			Code:    "REQUIRED",
			Message: "name is required",
		},
	}, err.Details)
}

func TestDomainErrorUnwrapsCause(t *testing.T) {
	cause := errors.New("database unavailable")

	err := NewInternalErrorWithCause("REPOSITORY_ERROR", "failed to read", cause)

	require.ErrorIs(t, err, cause)
	require.Equal(t, "failed to read: database unavailable", err.Error())
}
