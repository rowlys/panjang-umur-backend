package models

import "time"

type TransactionType int

const (
	Earned TransactionType = iota // 0
	Spent                         // 1
)

type Transaction struct {
	ID          string          `gorm:"primaryKey;type:varchar(50)" json:"id"`
	UserID      string          `gorm:"not null;index" json:"userId"`
	Amount      int             `gorm:"not null" json:"amount"`
	Type        TransactionType `gorm:"not null" json:"type"`
	ReferenceID string          `gorm:"not null" json:"referenceId"`
	Timestamp   time.Time       `gorm:"autoCreateTime" json:"timestamp"` // GORM handles insertion time automatically
}