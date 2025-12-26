package main

import (
	"backend/internal/app/ds"
	"backend/internal/app/dsn"
	"log"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Загрузка переменных окружения из .env файла
	_ = godotenv.Load()

	// Получение DSN строки подключения
	dsnStr := dsn.FromEnv()
	if dsnStr == "" {
		log.Fatal("DSN string is empty. Check your .env file")
	}

	// Подключение к базе данных
	db, err := gorm.Open(postgres.Open(dsnStr), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Connected to database successfully")

	// Миграция всех моделей
	err = db.AutoMigrate(
		&ds.User{},
		&ds.LicenseService{},
		&ds.LicensePaymentRequest{},
		&ds.LicensePaymentRequestService{},
	)
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database migration completed successfully")
}
