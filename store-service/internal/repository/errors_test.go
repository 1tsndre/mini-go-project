package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestTranslateError(t *testing.T) {
	plain := errors.New("connection reset")
	checkViolation := &pgconn.PgError{Code: "23514"}

	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "no rows", err: sql.ErrNoRows, want: ErrNotFound},
		{name: "wrapped no rows", err: fmt.Errorf("scan: %w", sql.ErrNoRows), want: ErrNotFound},
		{name: "unique violation", err: &pgconn.PgError{Code: "23505"}, want: ErrDuplicateKey},
		{name: "wrapped foreign key violation", err: fmt.Errorf("exec: %w", &pgconn.PgError{Code: "23503"}), want: ErrForeignKeyViolation},
		{name: "other constraint violation is passed through", err: checkViolation, want: checkViolation},
		{name: "other error is passed through", err: plain, want: plain},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ErrorIs(t, translateError(tt.err), tt.want)
		})
	}

	t.Run("nil", func(t *testing.T) {
		assert.NoError(t, translateError(nil))
	})

	t.Run("constraint violations keep the driver error for logging", func(t *testing.T) {
		var pgErr *pgconn.PgError
		err := translateError(&pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"})
		if assert.ErrorAs(t, err, &pgErr) {
			assert.Equal(t, "users_email_key", pgErr.ConstraintName)
		}
	})
}
