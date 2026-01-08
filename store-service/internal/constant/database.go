package constant

import "time"

const (
	DBDriver          = "pgx"
	DBScheme          = "postgres"
	DBMaxConns        = 25 // PostgreSQL allows 100 by default
	DBConnMaxIdleTime = 5 * time.Minute

	PGUniqueViolation     = "23505"
	PGForeignKeyViolation = "23503"

	MsgInvalidDSN      = "invalid database DSN: %w"
	MsgDBConnectFailed = "failed to connect to database: %w"
)
