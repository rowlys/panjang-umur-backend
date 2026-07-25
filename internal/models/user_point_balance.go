package models

import "github.com/google/uuid"

type UserPointBalance struct {
	OwnerID uuid.UUID `gorm:"primaryKey;type:uuid" json:"ownerId"`
	GiverID uuid.UUID `gorm:"primaryKey;type:uuid" json:"giverId"`
	Balance int       `gorm:"not null;default:0" json:"balance"`
}
