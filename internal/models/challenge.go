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
	StatusExpired
	StatusCancelled
	StatusCompleted
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
	ResetDay	int             `gorm:"not null;default:0" json:"resetDay"`
	CreatorID   uuid.UUID         `gorm:"type:uuid;not null;index" json:"creatorId"`
	Restricted  bool           `gorm:"not null;default:false" json:"restricted"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	ExpiresAt   *time.Time     `json:"expiresAt"`
}