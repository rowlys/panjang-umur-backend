package database

import (
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/rowlys/panjang-umur-backend/internal/models"
)

func Seed() {
	log.Println("Seeding database with initial data...")

	DB.Exec("TRUNCATE TABLE users, friendships, challenges, challenge_assignees, challenge_submissions, rewards, reward_visibilities, reward_claims, transactions, user_point_balances, messages CASCADE")

	bastenUser := SeedUser("basten", "Basten", "password123")
	alleeceUser := SeedUser("alleece", "Alleece", "password123")

	SeedFriendship(bastenUser, alleeceUser)

	dailyChallenge := SeedChallenge(bastenUser, "Morning walk", 10, models.Daily, []uuid.UUID{alleeceUser.ID})
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	yesterdayPeriod := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)
	SeedChallengeSubmission(dailyChallenge, alleeceUser, yesterdayPeriod, models.SubmissionApproved)

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

func SeedChallenge(creator models.User, title string, points int, challengeType models.ChallengeType, assigneeIDs []uuid.UUID) models.Challenge {
	challenge := models.Challenge{
		ID:         uuid.New(),
		Title:      title,
		Points:     points,
		Type:       challengeType,
		CreatorID:  creator.ID,
		Restricted: len(assigneeIDs) > 0,
		Status:     models.StatusActive,
	}

	if result := DB.Create(&challenge); result.Error != nil {
		log.Fatalf("Failed to create challenge %s: %v", title, result.Error)
	}

	for _, assigneeID := range assigneeIDs {
		assignee := models.ChallengeAssignee{
			ID:          uuid.New(),
			ChallengeID: challenge.ID,
			UserID:      assigneeID,
		}
		if result := DB.Create(&assignee); result.Error != nil {
			log.Fatalf("Failed to create challenge assignee for challenge %s: %v", title, result.Error)
		}
	}

	return challenge
}

func SeedChallengeSubmission(challenge models.Challenge, assignee models.User, periodStart time.Time, status models.SubmissionStatus) models.ChallengeSubmission {
	now := time.Now()
	submission := models.ChallengeSubmission{
		ID:          uuid.New(),
		ChallengeID: challenge.ID,
		UserID:      assignee.ID,
		PeriodStart: periodStart,
		Status:      status,
		SubmittedAt: now,
	}
	if status == models.SubmissionApproved {
		submission.ApprovedAt = &now
	}

	if result := DB.Create(&submission); result.Error != nil {
		log.Fatalf("Failed to create challenge submission for %s: %v", assignee.Username, result.Error)
	}

	return submission
}