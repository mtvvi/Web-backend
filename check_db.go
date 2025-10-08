package main

import (
	"backend/internal/app/ds"
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "host=localhost user=postgres password=password dbname=license_db port=5433 sslmode=disable TimeZone=Europe/Moscow"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	var services []ds.LicenseService
	err = db.Find(&services).Error
	if err != nil {
		log.Fatal("Failed to get services:", err)
	}

	fmt.Println("Services in database:")
	for _, service := range services {
		imageURL := "NULL"
		if service.ImageURL != nil {
			imageURL = *service.ImageURL
		}
		fmt.Printf("ID: %d, Name: %s, ImageURL: %s\n", service.ID, service.Name, imageURL)
	}
}
