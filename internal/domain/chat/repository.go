package chat

import (
	"sort"
	"time"
	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)

type ConversationSummary struct {
	FriendID    uuid.UUID
	LastMessage models.Message
	UnreadCount int64
}

type Repository interface {
	Create(m *models.Message) error
	FindConversation(user1ID, user2ID uuid.UUID, before *time.Time, limit int) ([]models.Message, error)
	MarkAsRead(senderID, recipientID uuid.UUID) error
	CountUnread(recipientID, senderID uuid.UUID) (int64, error)
	FindConversationSummaries(userID uuid.UUID) ([]ConversationSummary, error)
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

func (r *repository) FindConversationSummaries(userID uuid.UUID) ([]ConversationSummary, error) {
	var lastMessages []models.Message
	err := r.db.Raw(`
		SELECT DISTINCT ON (other_user_id) id, sender_id, recipient_id, body, created_at, read_at
		FROM (
			SELECT *, CASE WHEN sender_id = ? THEN recipient_id ELSE sender_id END AS other_user_id
			FROM messages
			WHERE sender_id = ? OR recipient_id = ?
		) t
		ORDER BY other_user_id, created_at DESC
	`, userID, userID, userID).Scan(&lastMessages).Error
	if err != nil {
		return nil, err
	}

	var unreadRows []struct {
		SenderID uuid.UUID
		Count    int64
	}
	err = r.db.Model(&models.Message{}).
		Select("sender_id, count(*) as count").
		Where("recipient_id = ? AND read_at IS NULL", userID).
		Group("sender_id").
		Scan(&unreadRows).Error
	if err != nil {
		return nil, err
	}

	unreadByFriend := make(map[uuid.UUID]int64, len(unreadRows))
	for _, row := range unreadRows {
		unreadByFriend[row.SenderID] = row.Count
	}

	summaries := make([]ConversationSummary, 0, len(lastMessages))
	for _, m := range lastMessages {
		friendID := m.RecipientID
		if m.SenderID != userID {
			friendID = m.SenderID
		}
		summaries = append(summaries, ConversationSummary{
			FriendID:    friendID,
			LastMessage: m,
			UnreadCount: unreadByFriend[friendID],
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].LastMessage.CreatedAt.After(summaries[j].LastMessage.CreatedAt)
	})

	return summaries, nil
}