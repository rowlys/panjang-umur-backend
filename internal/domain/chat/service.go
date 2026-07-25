package chat

import (
	"context"
	"time"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
	"github.com/rowlys/panjang-umur-backend/internal/models"
)

type FriendService interface {
	IsFriend(user1ID, user2ID uuid.UUID) (bool, error)
}

type Pusher interface {
	PushToUser(userID uuid.UUID, message *models.Message)
}

type Service interface {
	SendMessage(ctx context.Context, senderID, recipientID uuid.UUID, body string) (*models.Message, error)
	GetConversation(ctx context.Context, user1ID, user2ID uuid.UUID, before *time.Time, limit int) ([]models.Message, error)
	MarkAsRead(ctx context.Context, user1ID, user2ID uuid.UUID) error
}

type service struct {
	repo          Repository
	friendService FriendService
	pusher        Pusher
}

func NewService(repo Repository, friendService FriendService, pusher Pusher) Service {
	return &service{repo: repo, friendService: friendService, pusher: pusher}
}

func (s *service) SendMessage(ctx context.Context, senderID, recipientID uuid.UUID, body string) (*models.Message, error) {
	if senderID == recipientID {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "cannot send message to oneself"}
	}

	isFriend, err := s.friendService.IsFriend(senderID, recipientID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "failed to check friendship status"}
	}
	if !isFriend {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "only friends can send messages to each other"}
	}

	body = strings.TrimSpace(body)
	if body == "" {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "message body cannot be empty"}
	}

	message := &models.Message{
		ID:          uuid.New(),
		SenderID:    senderID,
		RecipientID: recipientID,
		Body:        body,
		CreatedAt:   time.Now(),
	}
	if err := s.repo.Create(message); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "failed to send message"}
	}

	s.pusher.PushToUser(recipientID, message)
	s.pusher.PushToUser(senderID, message)

	return message, nil
}

func (s *service) GetConversation(ctx context.Context, user1ID, user2ID uuid.UUID, before *time.Time, limit int) ([]models.Message, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}

	messages, err := s.repo.FindConversation(user1ID, user2ID, before, limit)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "failed to retrieve conversation"}
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (s *service) MarkAsRead(ctx context.Context, user1ID, user2ID uuid.UUID) error {
	if err := s.repo.MarkAsRead(user2ID, user1ID); err != nil {
		return &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "failed to mark messages as read"}
	}
	return nil
}