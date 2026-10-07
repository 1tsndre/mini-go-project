package databases

import "github.com/jmoiron/sqlx"

type Database interface {
	DB() *sqlx.DB
	Close() error
}
