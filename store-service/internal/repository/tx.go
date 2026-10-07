package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// withTx runs fn in a transaction and commits it if fn returns nil. An error or
// a panic in fn rolls the transaction back.
func withTx(ctx context.Context, db *sqlx.DB, fn func(tx *sqlx.Tx) error) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	// A no-op once Commit has succeeded.
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
