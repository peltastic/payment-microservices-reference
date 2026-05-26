package db

import (
	"log/slog"

	"github.com/peltastic/payment-microservices-reference/auth/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type IDatabase interface {
	GetDB() *gorm.DB
	WithTx(tx *gorm.DB) IDatabase
}

type Database struct {
	db *gorm.DB
}

func InitDB(cfg config.DatabaseConfig) (*Database, error) {
	log := slog.Default().With("component", "db")
	log.Info("opening database connection",
		"host", cfg.Host,
		"port", cfg.Port,
		"user", cfg.User,
		"dbname", cfg.Name,
		"sslmode", cfg.SSLMode,
	)

	database, err := NewDatabase(cfg.DataSourceName())
	if err != nil {
		log.Error("database connection failed", "error", err)
		return nil, err
	}
	log.Info("database connection established")
	return &Database{
		db: database.db,
	}, nil
}

func NewDatabase(dataSourceName string) (*Database, error) {
	log := slog.Default().With("component", "db")
	database, err := gorm.Open(postgres.Open(dataSourceName), &gorm.Config{})
	if err != nil {
		log.Error("gorm open failed", "error", err)
		return nil, err
	}
	sqlDB, err := database.DB()
	if err != nil {
		log.Error("database handle initialization failed", "error", err)
		return nil, err
	}
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetMaxOpenConns(200)

	return &Database{
		db: database,
	}, nil
}

func (d *Database) GetDB() *gorm.DB {
	return d.db
}

func (d *Database) WithTx(tx *gorm.DB) IDatabase {
	return &Database{db: tx}
}
