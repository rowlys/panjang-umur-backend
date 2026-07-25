package models

import (
	"time"

	"github.com/google/uuid"
)

type FriendshipStatus int

const (
	FriendshipPending FriendshipStatus = iota
	FriendshipAccepted
)

type Friendship struct {
	ID          uuid.UUID        `gorm:"primaryKey;type:uuid" json:"id"`
	RequesterID uuid.UUID        `gorm:"type:uuid;not null;index;uniqueIndex:idx_friendship_pair" json:"requesterId"`
	AddresseeID uuid.UUID        `gorm:"type:uuid;not null;index;uniqueIndex:idx_friendship_pair" json:"addresseeId"`
	Status      FriendshipStatus `gorm:"not null;default:0" json:"status"`
	CreatedAt   time.Time        `json:"createdAt"`
	RespondedAt *time.Time       `json:"respondedAt"`
}
