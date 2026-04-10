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
	PointBalance int       `gorm:"not null;default:0" json:"pointBalance"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`

	Groups       []Group   `gorm:"many2many:user_groups;" json:"-"`
}