package database

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/rowlys/panjang-umur-backend/internal/config"
	"github.com/rowlys/panjang-umur-backend/internal/models"
)

var DB *gorm.DB

func Connect() {
	// 1. Construct the DSN using your environment variables
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta",
		config.GetEnv("DB_HOST", "localhost"),
		config.GetEnv("DB_USER", "postgres"),
		config.GetEnv("DB_PASSWORD", ""),
		config.GetEnv("DB_NAME", "panjang_umur"),
		config.GetEnv("DB_PORT", "5432"),
	)

	// 2. Open the connection
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v\nCheck if PostgreSQL is running and credentials are correct.", err)
	}

	log.Println("Successfully connected to PostgreSQL!")

	// 3. Execute AutoMigrate
	log.Println("Running AutoMigrate...")
	err = db.AutoMigrate(
		&models.User{},
		&models.Challenge{},
		&models.Reward{},
		&models.Transaction{},
		&models.Group{},
	)
	
	if err != nil {
		log.Fatalf("Failed to migrate database schemas: %v", err)
	}

	log.Println("Database schemas migrated successfully!")
	
	DB = db
}