package models

import (
	"time"

	"github.com/google/uuid"
)

// ChallengeAssignee is a plain, immutable allowlist row created once per assignee
// at challenge-creation time. It records who a restricted challenge is targeted at
// and never changes afterwards; actual progress is tracked separately by ChallengeSubmission.
type ChallengeAssignee struct {
	ID          uuid.UUID `gorm:"primaryKey;type:uuid" json:"id"`
	ChallengeID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:idx_challenge_assignee_user" json:"challengeId"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:idx_challenge_assignee_user" json:"userId"`
	CreatedAt   time.Time `json:"createdAt"`
}
