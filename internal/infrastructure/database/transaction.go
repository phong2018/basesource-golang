package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Querier is the common interface satisfied by both *sqlx.DB and *sqlx.Tx,
// allowing repositories to work the same way inside or outside a transaction.
type Querier interface {
	// GetContext fetches exactly one row into dest — errors if zero or multiple rows matched.
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	// SelectContext fetches multiple rows into a slice dest.
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	// ExecContext runs a write statement (INSERT / UPDATE / DELETE) and returns no rows.
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type txKey struct{}

// WithinTransaction begins a DB transaction, injects it into ctx, and calls fn.
// Commits on success, rolls back on any error returned by fn.
// Repositories read the tx out of ctx via TxFromContext — no explicit tx passing needed.
func (c *Client) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := c.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// TxFromContext returns the active transaction stored in context, or nil.
func TxFromContext(ctx context.Context) *sqlx.Tx {
	tx, _ := ctx.Value(txKey{}).(*sqlx.Tx)
	return tx
}
