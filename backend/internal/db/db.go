package db

import (
	"Backend/internal/config"
	"Backend/internal/domain"
	"Backend/internal/domain/backend_entities"
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var cfg = config.LoadDB()

type DB struct {
	*gorm.DB
}

func NewDB() (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)

	log.Println("DSN:", dsn)

	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	log.Println("connected to postgres")

	gormDB.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`)

	if err := autoMigrate(gormDB); err != nil {
		return nil, err
	}

	return &DB{gormDB}, nil
}

func autoMigrate(gormDB *gorm.DB) error {
	return gormDB.AutoMigrate(
		domain.User{},
		domain.Quiz{},
		domain.Answer{},
		domain.Question{},

		domain.GameSession{},
		domain.Participant{},
		domain.PlayerAnswer{},

		backend_entities.Token{},
	)
}
