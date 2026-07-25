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

type Transaction struct {
	ID          uuid.UUID       `gorm:"primaryKey;type:uuid" json:"id"`
	UserID      uuid.UUID       `gorm:"type:uuid;not null;index" json:"userId"`
	GiverID     uuid.UUID       `gorm:"type:uuid;not null;index" json:"giverId"`
	Amount      int             `gorm:"not null" json:"amount"`
	Type        TransactionType `gorm:"not null" json:"type"`
	ReferenceID uuid.UUID       `gorm:"type:uuid;not null" json:"referenceId"`
	Timestamp   time.Time       `gorm:"autoCreateTime" json:"timestamp"`
}
