package models

import (
	"time"
	"github.com/google/uuid"
)

type Reward struct {
	ID            uuid.UUID    `gorm:"primaryKey;type:varchar(50)" json:"id"`
	Title         string    `gorm:"not null" json:"title"`
	Description   string    `json:"description"`
	Cost          int       `gorm:"not null" json:"cost"`
	RewardGiverID uuid.UUID    `gorm:"type: uuid;not null;index" json:"rewardGiverId"`
	ReceiverID    uuid.UUID    `gorm:"type: uuid;not null;index" json:"receiverId"`
	IsAvailable   bool      `gorm:"not null;default:true" json:"isAvailable"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`

	GroupID     uuid.UUID        `gorm:"type: uuid;not null;index" json:"groupId"`
}