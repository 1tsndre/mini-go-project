package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type postgresDB struct {
	db *sqlx.DB
}

func NewPostgresDB(dsn string, env string) (databases.Database, error) {
	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid database DSN: %w", err)
	}
	if env == constant.EnvDevelopment {
		connConfig.Tracer = queryLogger{}
	}

	db := sqlx.NewDb(stdlib.OpenDB(*connConfig), "pgx")
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &postgresDB{db: db}, nil
}

func (p *postgresDB) DB() *sqlx.DB {
	return p.db
}

func (p *postgresDB) Close() error {
	return p.db.Close()
}

type queryLogger struct{}

type queryStartKey struct{}

type queryStart struct {
	sql string
	at  time.Time
}

func (queryLogger) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryStartKey{}, queryStart{sql: data.SQL, at: time.Now()})
}

func (queryLogger) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	start, _ := ctx.Value(queryStartKey{}).(queryStart)
	fields := map[string]interface{}{
		"sql":      start.sql,
		"duration": time.Since(start.at).String(),
		"rows":     data.CommandTag.RowsAffected(),
	}
	if data.Err != nil {
		logger.Error(ctx, "query failed", data.Err, fields)
		return
	}
	logger.Debug(ctx, "query", fields)
}
