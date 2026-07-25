package chat

import (
	"time"
	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)

type Repository interface {
	Create(m *models.Message) error
	FindConversation(user1ID, user2ID uuid.UUID, before *time.Time, limit int) ([]models.Message, error)
	MarkAsRead(senderID, recipientID uuid.UUID) error
	CountUnread(recipientID, senderID uuid.UUID) (int64, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(m *models.Message) error {
	return r.db.Create(m).Error
}

func (r *repository) FindConversation(user1ID, user2ID uuid.UUID, before *time.Time, limit int) ([]models.Message, error) {
	query := r.db.Where(
		"(sender_id = ? AND recipient_id = ?) OR (sender_id = ? AND recipient_id = ?)",
		user1ID, user2ID, user2ID, user1ID,
	)

	if before != nil {
		query = query.Where("created_at < ?", *before)
	}

	var messages []models.Message
	err := query.Order("created_at desc").Limit(limit).Find(&messages).Error
	return messages, err
}

func (r *repository) MarkAsRead(senderID, recipientID uuid.UUID) error {
	return r.db.Model(&models.Message{}).
		Where("sender_id = ? AND recipient_id = ? AND read_at IS NULL", senderID, recipientID).
		Updates(map[string]interface{}{"read_at": time.Now()}).Error
}

func (r *repository) CountUnread(recipientID, senderID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Message{}).
		Where("sender_id = ? AND recipient_id = ? AND read_at IS NULL", senderID, recipientID).
		Count(&count).Error
	return count, err
}