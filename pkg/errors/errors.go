// Package apperrors provides structured, typed error handling for the CodePilot AI
// application. It defines domain-specific error types (NotFound, Validation, Internal,
// Conflict) that map cleanly to HTTP status codes for consistent API responses.
package apperrors

import (
	"fmt"
	"net/http"
)

// ErrorResponse represents a structured error response returned by the API.
type ErrorResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// --- NotFoundError ---

// NotFoundError indicates that a requested resource could not be found.
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s with id '%s' not found", e.Resource, e.ID)
}

// NewNotFound creates a new NotFoundError for the given resource and identifier.
func NewNotFound(resource, id string) *NotFoundError {
	return &NotFoundError{Resource: resource, ID: id}
}

// --- ValidationError ---

// ValidationError indicates that the input data failed validation.
type ValidationError struct {
	Message string
	Field   string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// NewValidation creates a new ValidationError with a message.
func NewValidation(msg string) *ValidationError {
	return &ValidationError{Message: msg}
}

// NewValidationField creates a new ValidationError with a specific field name.
func NewValidationField(field, msg string) *ValidationError {
	return &ValidationError{Message: msg, Field: field}
}

// --- InternalError ---

// InternalError represents an unexpected internal server error.
// It wraps the underlying cause for logging while exposing a safe message to clients.
type InternalError struct {
	Message string
	Err     error
}

func (e *InternalError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("internal error: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("internal error: %s", e.Message)
}

func (e *InternalError) Unwrap() error {
	return e.Err
}

// NewInternal creates a new InternalError wrapping an underlying error.
func NewInternal(msg string, err error) *InternalError {
	return &InternalError{Message: msg, Err: err}
}

// --- ConflictError ---

// ConflictError indicates a resource conflict (e.g., duplicate creation).
type ConflictError struct {
	Resource string
	Message  string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict on %s: %s", e.Resource, e.Message)
}

// NewConflict creates a new ConflictError.
func NewConflict(resource, msg string) *ConflictError {
	return &ConflictError{Resource: resource, Message: msg}
}

// --- UnauthorizedError ---

// UnauthorizedError indicates that the request lacks valid authentication.
type UnauthorizedError struct {
	Message string
}

func (e *UnauthorizedError) Error() string {
	return fmt.Sprintf("unauthorized: %s", e.Message)
}

// NewUnauthorized creates a new UnauthorizedError.
func NewUnauthorized(msg string) *UnauthorizedError {
	return &UnauthorizedError{Message: msg}
}

// --- HTTP Status Mapping ---

// ToHTTPStatus maps an application error to the corresponding HTTP status code.
// Unrecognized error types default to 500 Internal Server Error.
func ToHTTPStatus(err error) int {
	switch err.(type) {
	case *NotFoundError:
		return http.StatusNotFound
	case *ValidationError:
		return http.StatusBadRequest
	case *ConflictError:
		return http.StatusConflict
	case *UnauthorizedError:
		return http.StatusUnauthorized
	case *InternalError:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// ToErrorResponse converts an application error into a structured ErrorResponse.
func ToErrorResponse(err error) ErrorResponse {
	return ErrorResponse{
		Code:    ToHTTPStatus(err),
		Message: err.Error(),
	}
}
