package db

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/Keoroanthony/go-ecommerce/internal/models"
)

var DB *gorm.DB

func Init() {

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Africa/Nairobi",
		getEnv("POSTGRES_HOST", "localhost"),
		getEnv("POSTGRES_USER", "test"),
		getEnv("POSTGRES_PASSWORD", "test"),
		getEnv("POSTGRES_DB", "test"),
		getEnv("DB_PORT", "5432"),
	)

	var err error

	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {

		log.Fatalf("Failed to connect to DB: %v", err)
	}

	err = DB.AutoMigrate(
		&models.Category{},
		&models.Product{},
		&models.Customer{},
		&models.Order{},
		&models.OrderItem{},
		&models.User{},
	)

	if err != nil {

		log.Fatalf("Failed to migrate DB: %v", err)
	}

	log.Println("Database connected and migrated successfully")
}

func SetTestDB(testDB *gorm.DB) {
	DB = testDB
}

func getEnv(key, fallback string) string {

	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return fallback
}
