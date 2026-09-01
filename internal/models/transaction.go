package models

import (
	"time"

	"github.com/google/uuid"
)

type TransactionType int

const (
	Earned TransactionType = iota
	Spent
)

// TransactionReferenceType tells you what a Transaction.ReferenceID points to.
type TransactionReferenceType int

const (
	ReferenceClaim TransactionReferenceType = iota
	ReferenceSubmission
)

type Transaction struct {
	ID            uuid.UUID                `gorm:"primaryKey;type:uuid" json:"id"`
	UserID        uuid.UUID                `gorm:"type:uuid;not null;index" json:"userId"`
	GiverID       uuid.UUID                `gorm:"type:uuid;not null;index" json:"giverId"`
	Amount        int                      `gorm:"not null" json:"amount"`
	Type          TransactionType          `gorm:"not null" json:"type"`
	ReferenceID   uuid.UUID                `gorm:"type:uuid;not null" json:"referenceId"`
	ReferenceType TransactionReferenceType `gorm:"not null;default:0" json:"referenceType"`
	Timestamp     time.Time                `gorm:"autoCreateTime" json:"timestamp"`
}

// ClaimContext is the History-ready context for a reward claim, resolved by
// the reward domain so callers (e.g. the transaction handler) don't need to
// import it directly.
type ClaimContext struct {
	ClaimID     uuid.UUID
	RewardID    uuid.UUID
	RewardTitle string
	RedeemerID  uuid.UUID
	GiverID     uuid.UUID
}

// SubmissionContext is the History-ready context for a challenge submission,
// resolved by the challenge domain so callers (e.g. the transaction handler)
// don't need to import it directly.
type SubmissionContext struct {
	SubmissionID   uuid.UUID
	ChallengeID    uuid.UUID
	ChallengeTitle string
	SubmitterID    uuid.UUID
	CreatorID      uuid.UUID
}
