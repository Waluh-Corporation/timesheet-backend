package domain_test

import (
	"errors"
	"net/http"
	"testing"

	"timesheet-backend/internal/domain"
)

func TestMapErrorToHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"nil error", nil, http.StatusOK},
		{"not found", domain.ErrNotFound, http.StatusNotFound},
		{"unauthorized", domain.ErrUnauthorized, http.StatusUnauthorized},
		{"forbidden", domain.ErrForbidden, http.StatusForbidden},
		{"account disabled", domain.ErrAccountDisabled, http.StatusForbidden},
		{"invalid input", domain.ErrInvalidInput, http.StatusBadRequest},
		{"conflict", domain.ErrConflict, http.StatusConflict},
		{"unknown error", errors.New("something broke"), http.StatusInternalServerError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.MapErrorToHTTPStatus(tc.err)
			if got != tc.expected {
				t.Errorf("expected status %d, got %d", tc.expected, got)
			}
		})
	}
}

func TestUserError(t *testing.T) {
	msg := "No activities recorded for this period"
	userErr := domain.NewUserError(domain.ErrInvalidInput, msg)

	if userErr.Error() != msg {
		t.Errorf("expected error message %q, got %q", msg, userErr.Error())
	}

	if !errors.Is(userErr, domain.ErrInvalidInput) {
		t.Errorf("expected userErr to match domain.ErrInvalidInput via Unwrap")
	}

	if status := domain.MapErrorToHTTPStatus(userErr); status != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 Bad Request, got %d", status)
	}
}
