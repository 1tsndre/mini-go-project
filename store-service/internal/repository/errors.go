package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound            = errors.New("record not found")
	ErrDuplicateKey        = errors.New("duplicate key")
	ErrForeignKeyViolation = errors.New("foreign key violation")
)

func translateDatabaseError(err error) error {
	var pgErr *pgconn.PgError

	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case constant.PGUniqueViolation:
			return fmt.Errorf("%w: %w", ErrDuplicateKey, err)
		case constant.PGForeignKeyViolation:
			return fmt.Errorf("%w: %w", ErrForeignKeyViolation, err)
		}
	}
	return err
}
