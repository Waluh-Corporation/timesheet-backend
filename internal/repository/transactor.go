package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"timesheet-backend/internal/domain"
)

// Transactor is re-exported from domain.Transactor.
type Transactor = domain.Transactor

type txContextKey struct{}

// HasTx reports whether an active transaction is stored in context.
func HasTx(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	tx, ok := ctx.Value(txContextKey{}).(*gorm.DB)
	return ok && tx != nil
}

// InjectTx places a GORM transaction instance into context safely.
func InjectTx(ctx context.Context, tx *gorm.DB) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, txContextKey{}, tx)
}

// ExtractTx extracts an active GORM transaction from context, falling back to db if none is present.
func ExtractTx(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if ctx != nil {
		if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok && tx != nil {
			return tx.WithContext(ctx)
		}
	}
	if fallback != nil && ctx != nil {
		return fallback.WithContext(ctx)
	}
	return fallback
}

// GormTransactor implements domain.Transactor using GORM db.Transaction.
type GormTransactor struct {
	db *gorm.DB
}

// NewGormTransactor constructs a new GormTransactor.
func NewGormTransactor(db *gorm.DB) *GormTransactor {
	return &GormTransactor{db: db}
}

// WithinTransaction executes fn within an ACID database transaction.
func (t *GormTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if fn == nil {
		return nil
	}
	if t == nil || t.db == nil {
		return errors.New("transactor database connection is nil")
	}
	db := ExtractTx(ctx, t.db)
	if db == nil {
		return errors.New("transactor database connection is nil")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		return fn(InjectTx(ctx, tx))
	})
}
