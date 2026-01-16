package postgres

import (
	"fmt"

	"github.com/1tsndre/mini-go-project/pkg/constant"
	"github.com/1tsndre/mini-go-project/store-service/internal/repository/databases"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type postgresDB struct {
	db *gorm.DB
}

func NewPostgresDB(dsn string, env string) (databases.Database, error) {
	logLevel := logger.Silent
	if env == constant.EnvDevelopment {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &postgresDB{db: db}, nil
}

func (p *postgresDB) DB() *gorm.DB {
	return p.db
}
