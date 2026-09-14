package errors

import "fmt"

// ErrorType represents the type of domain error
type ErrorType string

const (
	ErrorTypeValidation   ErrorType = "validation"
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeUnauthorized ErrorType = "unauthorized"
	ErrorTypeConflict     ErrorType = "conflict"
	ErrorTypeRateLimit    ErrorType = "rate_limit"
	ErrorTypeInternal     ErrorType = "internal"
)

type DomainError struct {
	Type    ErrorType
	Code    string
	Message string
	Details []ErrorDetail
	Cause   error
}

type ErrorDetail struct {
	Field   string `json:"field,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e *DomainError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *DomainError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *DomainError) WithDetails(details ...ErrorDetail) *DomainError {
	if e == nil || len(details) == 0 {
		return e
	}
	e.Details = append(e.Details, details...)
	return e
}

func NewInternalError(code, message string) *DomainError {
	return &DomainError{
		Type:    ErrorTypeInternal,
		Code:    code,
		Message: message,
	}
}

func NewInternalErrorWithCause(code, message string, cause error) *DomainError {
	return &DomainError{
		Type:    ErrorTypeInternal,
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func NewValidationError(code, message string) *DomainError {
	return &DomainError{
		Type:    ErrorTypeValidation,
		Code:    code,
		Message: message,
	}
}

func NewNotFoundError(code, message string) *DomainError {
	return &DomainError{
		Type:    ErrorTypeNotFound,
		Code:    code,
		Message: message,
	}
}

func NewUnauthorizedError(code, message string) *DomainError {
	return &DomainError{
		Type:    ErrorTypeUnauthorized,
		Code:    code,
		Message: message,
	}
}

func NewConflictError(code, message string) *DomainError {
	return &DomainError{
		Type:    ErrorTypeConflict,
		Code:    code,
		Message: message,
	}
}

func NewRateLimitError(code, message string) *DomainError {
	return &DomainError{
		Type:    ErrorTypeRateLimit,
		Code:    code,
		Message: message,
	}
}
