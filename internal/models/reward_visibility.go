package models

import "github.com/google/uuid"

type RewardVisibility struct {
	RewardID uuid.UUID `gorm:"primaryKey;type:uuid" json:"rewardId"`
	UserID   uuid.UUID `gorm:"primaryKey;type:uuid" json:"userId"`
}
