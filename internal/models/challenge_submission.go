package models

import (
	"time"

	"github.com/google/uuid"
)

type SubmissionStatus int

const (
	SubmissionSubmitted SubmissionStatus = iota
	SubmissionApproved
)

// ChallengeSubmission is created only when a user actually submits a challenge for a
// given recurrence period (see currentPeriodStart in the challenge service) — there is
// no pre-created placeholder row. The unique index on (challenge, user, period) is what
// "have I already done this period" checks against directly.
type ChallengeSubmission struct {
	ID          uuid.UUID        `gorm:"primaryKey;type:uuid" json:"id"`
	ChallengeID uuid.UUID        `gorm:"type:uuid;not null;index;uniqueIndex:idx_challenge_submission_period" json:"challengeId"`
	UserID      uuid.UUID        `gorm:"type:uuid;not null;index;uniqueIndex:idx_challenge_submission_period" json:"userId"`
	// PeriodStart carries forward the same zero-value-sentinel convention as the old
	// ChallengeAssignment (see currentPeriodStart in service.go) so Bounty submissions,
	// which have no recurrence, still get a stable uniqueness key.
	PeriodStart time.Time        `gorm:"not null;default:'0001-01-01 00:00:00+00';uniqueIndex:idx_challenge_submission_period" json:"periodStart"`
	Status      SubmissionStatus `gorm:"not null;default:0" json:"status"`
	SubmittedAt time.Time        `gorm:"not null" json:"submittedAt"`
	ApprovedAt  *time.Time       `json:"approvedAt"`
}
