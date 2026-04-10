package models

import (
	"time"
	"github.com/google/uuid"
)

type Group struct {
	ID          uuid.UUID    `gorm:"primaryKey;type:uuid" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	
	Users       []User    `gorm:"many2many:user_groups;" json:"users,omitempty"` 	
}