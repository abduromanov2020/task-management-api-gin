package apperr

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/abduromanov2020/tasks-api/internal/domain"
)

// AppError is the canonical structured error type that flows through the
// handler boundary. Handlers should never write JSON for errors directly;
// they should attach an *AppError to gin.Context via c.Error and let the
// central error middleware render the envelope.
type AppError struct {
	HTTPStatus int            `json:"-"`
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
	Cause      error          `json:"-"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Cause }

func New(status int, code, msg string) *AppError {
	return &AppError{HTTPStatus: status, Code: code, Message: msg}
}

func Wrap(status int, code, msg string, cause error) *AppError {
	return &AppError{HTTPStatus: status, Code: code, Message: msg, Cause: cause}
}

func WithDetails(e *AppError, d map[string]any) *AppError {
	e.Details = d
	return e
}

// Common factories
func Validation(msg string, cause error) *AppError {
	return Wrap(http.StatusUnprocessableEntity, "VALIDATION_ERROR", msg, cause)
}
func Unauthorized(msg string) *AppError { return New(http.StatusUnauthorized, "UNAUTHORIZED", msg) }
func Forbidden(msg string) *AppError    { return New(http.StatusForbidden, "FORBIDDEN", msg) }
func NotFound(msg string) *AppError     { return New(http.StatusNotFound, "NOT_FOUND", msg) }
func Conflict(msg string) *AppError     { return New(http.StatusConflict, "CONFLICT", msg) }
func Internal(cause error) *AppError {
	return Wrap(http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", cause)
}
func IdemMismatch() *AppError {
	return New(http.StatusUnprocessableEntity, "IDEMPOTENCY_KEY_MISMATCH",
		"Idempotency-Key was reused with a different request body")
}
func IdemInFlight() *AppError {
	return New(http.StatusConflict, "IDEMPOTENCY_IN_FLIGHT",
		"A request with this Idempotency-Key is still being processed")
}

// From converts any error to an *AppError. Domain sentinels are mapped to
// canonical statuses; anything else becomes a 500.
func From(err error) *AppError {
	if err == nil {
		return nil
	}
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return Wrap(http.StatusNotFound, "NOT_FOUND", "Resource not found", err)
	case errors.Is(err, domain.ErrConflict):
		return Wrap(http.StatusConflict, "CONFLICT", "Resource conflict", err)
	case errors.Is(err, domain.ErrForbidden):
		return Wrap(http.StatusForbidden, "FORBIDDEN", "Operation not allowed", err)
	case errors.Is(err, domain.ErrUnauthorized):
		return Wrap(http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", err)
	case errors.Is(err, domain.ErrValidation):
		return Wrap(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid input", err)
	case errors.Is(err, domain.ErrIdemMismatch):
		return IdemMismatch()
	case errors.Is(err, domain.ErrIdemInFlight):
		return IdemInFlight()
	default:
		return Internal(err)
	}
}
