package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	pkgconstant "github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/1tsndre/mini-go-project/pkg/logger"
	"github.com/1tsndre/mini-go-project/store-service/internal/constant"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

func NewPostgresDB(dsn string, env string) (*sqlx.DB, error) {
	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf(constant.MsgInvalidDSN, err)
	}

	if env == pkgconstant.EnvDevelopment {
		connConfig.Tracer = queryLogger{}
	}

	db := sqlx.NewDb(stdlib.OpenDB(*connConfig), constant.DBDriver)
	db.SetMaxOpenConns(constant.DBMaxConns)
	db.SetMaxIdleConns(constant.DBMaxConns)
	db.SetConnMaxIdleTime(constant.DBConnMaxIdleTime)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf(constant.MsgDBConnectFailed, err)
	}

	return db, nil
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
	fields := logger.Fields{
		"sql":      strings.Join(strings.Fields(start.sql), " "),
		"duration": time.Since(start.at).String(),
		"rows":     data.CommandTag.RowsAffected(),
	}
	if data.Err != nil {
		logger.Error(ctx, constant.LogQueryFailed, data.Err, fields)
		return
	}
	logger.Debug(ctx, constant.LogQuery, fields)
}
