package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Errors the repositories return for expected database outcomes, so services can
// tell them apart with errors.Is without depending on the driver.
var (
	ErrNotFound     = errors.New("record not found")
	ErrDuplicateKey = errors.New("duplicate key")
	// ErrForeignKeyViolation means a write referenced a row that does not exist, or
	// a delete removed a row that is still referenced.
	ErrForeignKeyViolation = errors.New("foreign key violation")
)

// PostgreSQL error codes: https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

// translateError maps driver errors to the errors above. Constraint violations
// keep the driver error in the chain so it still shows up in logs.
func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return fmt.Errorf("%w: %w", ErrDuplicateKey, err)
		case pgForeignKeyViolation:
			return fmt.Errorf("%w: %w", ErrForeignKeyViolation, err)
		}
	}
	return err
}
