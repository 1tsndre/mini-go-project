package databases

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Database interface {
	DB() *sqlx.DB
	Close() error
}

type quietKey struct{}

// WithoutQueryLog marks ctx so its successful queries stay out of the development
// query log, for queries that run in the background all the time.
func WithoutQueryLog(ctx context.Context) context.Context {
	return context.WithValue(ctx, quietKey{}, true)
}

// QueryLogSuppressed reports whether WithoutQueryLog marked ctx.
func QueryLogSuppressed(ctx context.Context) bool {
	quiet, _ := ctx.Value(quietKey{}).(bool)
	return quiet
}
