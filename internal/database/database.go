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
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta",
		config.GetEnv("DB_HOST", "localhost"),
		config.GetEnv("DB_USER", "postgres"),
		config.GetEnv("DB_PASSWORD", ""),
		config.GetEnv("DB_NAME", "panjang_umur"),
		config.GetEnv("DB_PORT", "5432"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v\nCheck if PostgreSQL is running and credentials are correct.", err)
	}

	log.Println("Successfully connected to PostgreSQL!")

	log.Println("Running AutoMigrate...")
	err = db.AutoMigrate(
		&models.User{},
		&models.Friendship{},
		&models.Challenge{},
		&models.ChallengeAssignment{},
		&models.Reward{},
		&models.RewardVisibility{},
		&models.Transaction{},
		&models.UserPointBalance{},
		&models.Message{},
	)
	
	if err != nil {
		log.Fatalf("Failed to migrate database schemas: %v", err)
	}

	log.Println("Database schemas migrated successfully!")
	
	DB = db
}