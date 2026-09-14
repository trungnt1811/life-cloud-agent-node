package dto

// ErrorDTOResponse documents the public HTTP error payload.
//
// Runtime code writes errors through internal/delivery/http/response.Error; this
// DTO keeps Swagger annotations stable and explicit.
type ErrorDTOResponse struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Errors  any    `json:"errors,omitempty"`
}
