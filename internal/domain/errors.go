package domain

import "errors"

// Sentinel errors crossed at the usecase/handler boundary. The handler-side
// error middleware maps each to an HTTP status + error envelope.
var (
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("conflict")
	ErrForbidden        = errors.New("forbidden")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrValidation       = errors.New("validation")
	ErrIdemMismatch     = errors.New("idempotency key reused with different body")
	ErrIdemInFlight     = errors.New("idempotency key currently in-flight")
)
