package models

import (
	"time"
	"github.com/google/uuid"
)

type ClaimStatus int

const (
	ClaimStatusPending ClaimStatus = iota
	ClaimStatusFulfilled
	ClaimStatusRefundRequested
	ClaimStatusRefunded
)

type RewardClaim struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid" json:"id"`
	RewardID  uuid.UUID `gorm:"type:uuid;not null;index" json:"rewardId"`
	// RewardTitle is the reward's title at the moment it was redeemed. It is a
	// snapshot: editing the reward later must not rewrite what was bought.
	RewardTitle string    `gorm:"not null;default:''" json:"rewardTitle"`
	RedeemerID    uuid.UUID `gorm:"type:uuid;not null;index" json:"redeemerId"`
	GiverID  uuid.UUID `gorm:"type:uuid;not null;index" json:"giverId"`
	Price	 int       `gorm:"not null" json:"price"`
	Status	ClaimStatus `gorm:"not null;default:0" json:"status"`
	RefundReason *string    `json:"refundReason"`
	RedeemedAt time.Time `gorm:"autoCreateTime" json:"redeemedAt"`
	FulfilledAt *time.Time `json:"fulfilledAt"`
	ResolvedAt  *time.Time `json:"resolvedAt"`
}
