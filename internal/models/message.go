package models

import (
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID          uuid.UUID  `gorm:"primaryKey;type:uuid" json:"id"`
	SenderID    uuid.UUID  `gorm:"type:uuid;not null;index:idx_message_sender_recipient,priority:1;index:idx_message_recipient_sender,priority:2" json:"senderId"`
	RecipientID uuid.UUID  `gorm:"type:uuid;not null;index:idx_message_sender_recipient,priority:2;index:idx_message_recipient_sender,priority:1" json:"recipientId"`
	Body        string     `gorm:"not null" json:"body"`
	CreatedAt   time.Time  `gorm:"index:idx_message_sender_recipient,priority:3;index:idx_message_recipient_sender,priority:3" json:"createdAt"`
	ReadAt      *time.Time `json:"readAt"`
}
