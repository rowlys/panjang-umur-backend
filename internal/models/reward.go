package models

import (
	"time"
	"github.com/google/uuid"
)

type RewardVisibilityMode int

const (
	Public RewardVisibilityMode = iota
	Restricted
)

type Reward struct {
	ID            uuid.UUID            `gorm:"primaryKey;type:uuid" json:"id"`
	Title         string               `gorm:"not null" json:"title"`
	Description   string               `json:"description"`
	Cost          int                  `gorm:"not null" json:"cost"`
	RewardGiverID uuid.UUID            `gorm:"type:uuid;not null;index" json:"rewardGiverId"`
	Visibility    RewardVisibilityMode `gorm:"not null;default:0" json:"visibility"`
	RedeemedByID  *uuid.UUID           `gorm:"type:uuid;index" json:"redeemedById"`
	IsAvailable   bool                 `gorm:"not null;default:true" json:"isAvailable"`
	CreatedAt     time.Time            `json:"createdAt"`
	UpdatedAt     time.Time            `json:"updatedAt"`
}