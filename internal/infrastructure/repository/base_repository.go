package repository

import (
	"context"

	"github.com/yourname/go-clean-base/internal/infrastructure/database"
)

// baseRepository is embedded by all repository structs to provide transaction-aware query execution.
type baseRepository struct {
	db *database.Client
}

// conn returns the active *sqlx.Tx from context when inside WithinTransaction,
// otherwise falls back to the regular *sqlx.DB.
func (b *baseRepository) conn(ctx context.Context) database.Querier {
	if tx := database.TxFromContext(ctx); tx != nil {
		return tx
	}
	return b.db.DB
}
