package domain

import "context"

// Transactor defines the contract for executing business logic within an atomic database transaction.
type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
