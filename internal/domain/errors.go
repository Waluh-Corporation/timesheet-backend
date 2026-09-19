package domain

import (
	"errors"
	"net/http"
)

// Standard domain sentinel errors
var (
	ErrNotFound         = errors.New("resource not found")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrInvalidInput     = errors.New("invalid input")
	ErrConflict         = errors.New("resource already exists")
	ErrUsernameConflict = errors.New("username already exists")
	ErrEmailConflict    = errors.New("email already exists")
	ErrInternal         = errors.New("internal server error")
	ErrAccountDisabled  = errors.New("account is disabled")
)

// UserError wraps an underlying sentinel error with a clean message suitable for clients and front-ends.
type UserError struct {
	Err error
	Msg string
}

func (e *UserError) Error() string {
	return e.Msg
}

func (e *UserError) Unwrap() error {
	return e.Err
}

// NewUserError creates a UserError wrapping a sentinel error while presenting only the user-facing message.
func NewUserError(err error, message string) error {
	return &UserError{
		Err: err,
		Msg: message,
	}
}

// MapErrorToHTTPStatus maps standard domain errors to corresponding HTTP status codes.
func MapErrorToHTTPStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrAccountDisabled):
		return http.StatusForbidden
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, ErrConflict), errors.Is(err, ErrUsernameConflict), errors.Is(err, ErrEmailConflict):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
