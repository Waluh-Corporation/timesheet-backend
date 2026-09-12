package observability_test

import (
	"context"
	"testing"

	"timesheet-backend/internal/observability"
)

func TestLogger(t *testing.T) {
	l := observability.Logger()
	if l == nil {
		t.Fatal("expected default logger to be initialized")
	}

	ctx := context.WithValue(context.Background(), observability.RequestIDKey, "req-12345")
	ctx = context.WithValue(ctx, observability.UserIDKey, uint(42))

	ctxLogger := observability.FromContext(ctx)
	if ctxLogger == nil {
		t.Fatal("expected contextual logger to not be nil")
	}
}
