package repository

import (
	"backend/internal/app/ds"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(dsn string) (*Repository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Автоматическая миграция всех таблиц
	err = db.AutoMigrate(
		&ds.User{},
		&ds.LicenseService{},
		&ds.LicensePaymentRequest{},
		&ds.LicensePaymentRequestService{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &Repository{
		db: db,
	}, nil
}
