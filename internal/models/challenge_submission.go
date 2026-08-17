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

type ChallengeSubmission struct {
	ID          uuid.UUID        `gorm:"primaryKey;type:uuid" json:"id"`
	ChallengeID uuid.UUID        `gorm:"type:uuid;not null;index;uniqueIndex:idx_challenge_submission_period" json:"challengeId"`
	UserID      uuid.UUID        `gorm:"type:uuid;not null;index;uniqueIndex:idx_challenge_submission_period" json:"userId"`
	ProofImageID *uuid.UUID        `gorm:"type:uuid" json:"proofImageId"`
	PeriodStart time.Time        `gorm:"not null;default:'0001-01-01 00:00:00+00';uniqueIndex:idx_challenge_submission_period" json:"periodStart"`
	Status      SubmissionStatus `gorm:"not null;default:0" json:"status"`
	SubmittedAt time.Time        `gorm:"not null" json:"submittedAt"`
	ApprovedAt  *time.Time       `json:"approvedAt"`
}
