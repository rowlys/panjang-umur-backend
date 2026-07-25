package models

import (
	"time"

	"github.com/google/uuid"
)

type AssignmentStatus int

const (
	AssignmentAssigned AssignmentStatus = iota
	AssignmentSubmitted
	AssignmentApproved
)

type ChallengeAssignment struct {
	ID          uuid.UUID        `gorm:"primaryKey;type:uuid" json:"id"`
	ChallengeID uuid.UUID        `gorm:"type:uuid;not null;index;uniqueIndex:idx_challenge_assignee" json:"challengeId"`
	AssigneeID  uuid.UUID        `gorm:"type:uuid;not null;index;uniqueIndex:idx_challenge_assignee" json:"assigneeId"`
	Status      AssignmentStatus `gorm:"not null;default:0" json:"status"`
	SubmittedAt *time.Time       `json:"submittedAt"`
	ApprovedAt  *time.Time       `json:"approvedAt"`
}
