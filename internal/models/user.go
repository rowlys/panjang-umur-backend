package models

import (
	"time"
	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID    `gorm:"primaryKey;type:uuid" json:"id"`
	Username     string    `gorm:"unique;not null" json:"username"` 
	Name         string    `gorm:"not null" json:"name"`
	PasswordHash string    `gorm:"not null" json:"-"`               
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type BareUserDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Name     string    `json:"name"`
}