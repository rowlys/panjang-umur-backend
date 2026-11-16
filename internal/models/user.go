package models

import (
	"time"
	"github.com/google/uuid"
)

type Gender int

const (
	GenderUnspecified Gender = iota
	GenderMale
	GenderFemale
)

type User struct {
	ID           uuid.UUID    `gorm:"primaryKey;type:uuid" json:"id"`
	Username     string    `gorm:"unique;not null" json:"username"` 
	Name         string    `gorm:"not null" json:"name"`
	PasswordHash string    `gorm:"not null" json:"-"`               
	Gender       Gender    `gorm:"not null;default:0" json:"gender"`
	// AvatarURL is the public URL of the user's square profile picture. Empty
	// means the app should fall back to the user's initial.
	AvatarURL    string    `gorm:"not null;default:''" json:"avatarUrl"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type BareUserDTO struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatarUrl"`
}