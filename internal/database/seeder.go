package database

import (
	"log"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/rowlys/panjang-umur-backend/internal/models"
)

func Seed() {
	log.Println("Seeding database with initial data...")

	DB.Exec("TRUNCATE TABLE users, groups, challenges, rewards, transactions, user_groups CASCADE")

	bastenUser := SeedUser("basten", "Basten", "password123")
	alleeceUser := SeedUser("alleece", "Alleece", "password123")

	familyUsers := []models.User{bastenUser, alleeceUser}
	
	SeedGroup("Family", "Family group for daily challenges", familyUsers)

	log.Println("Database seeding completed!")
}

func SeedUser(username, name, password string) models.User {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password for user %s: %v", username, err)
	}

	user := models.User{
		ID:           uuid.New(),
		Username:     username,
		Name:         name,
		PasswordHash: string(hashedPassword),
	}

	if result := DB.Create(&user); result.Error != nil {
		log.Fatalf("Failed to create user %s: %v", username, result.Error)
	}

	return user	
}


func SeedGroup(name, description string, users []models.User) models.Group {
	group := models.Group{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		Users:       users,
	}

	if result := DB.Create(&group); result.Error != nil {
		log.Fatalf("Failed to create group %s: %v", name, result.Error)
	}

	return group
}