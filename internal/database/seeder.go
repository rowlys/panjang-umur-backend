package database

import (
	"log"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/rowlys/panjang-umur-backend/internal/models"
)

func Seed() {
	log.Println("Seeding database with initial data...")

	DB.Exec("TRUNCATE TABLE users, friendships, challenges, challenge_assignments, rewards, reward_visibilities, transactions, user_point_balances CASCADE")

	bastenUser := SeedUser("basten", "Basten", "password123")
	alleeceUser := SeedUser("alleece", "Alleece", "password123")

	SeedFriendship(bastenUser, alleeceUser)

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


func SeedFriendship(userA, userB models.User) models.Friendship {
	friendship := models.Friendship{
		ID:          uuid.New(),
		RequesterID: userA.ID,
		AddresseeID: userB.ID,
		Status:      models.FriendshipAccepted,
	}

	if result := DB.Create(&friendship); result.Error != nil {
		log.Fatalf("Failed to create friendship between %s and %s: %v", userA.Username, userB.Username, result.Error)
	}

	return friendship
}