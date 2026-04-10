package models

import (
	"time"
	"gorm.io/gorm"

	"github.com/google/uuid"
)

type ChallengeStatus int
type ChallengeType int

const (
	StatusActive    ChallengeStatus = iota
	StatusPending
	StatusCompleted
	StatusExpired
)

const (
	Bounty    ChallengeType = iota
	Daily
	Weekly
)

type Challenge struct {
	ID          uuid.UUID     `gorm:"primaryKey;type:uuid" json:"id"`
	Title       string         `gorm:"not null" json:"title"`
	Description string         `json:"description"`
	Points  	int            `gorm:"not null" json:"points"`
	Status      ChallengeStatus `gorm:"not null;default:0" json:"status"` 
	Type        ChallengeType   `gorm:"not null;default:0" json:"type"` 
	CreatorID   uuid.UUID         `gorm:"type: uuid;not null;index" json:"creatorId"`
	AssigneeID  uuid.UUID        `gorm:"type: uuid;index" json:"assigneeId"`          // Pointer makes it nullable
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`                   // Hidden from JSON responses
	ExpiresAt   *time.Time     `json:"expiresAt"`

	GroupID     uuid.UUID        `gorm:"type: uuid;not null;index" json:"groupId"`
}