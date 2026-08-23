package friendship

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)

type Service interface {
	SendRequest(ctx context.Context, requesterID, addresseeID uuid.UUID) (*models.Friendship, error)
	Accept(ctx context.Context, requestID, callerID uuid.UUID) (*models.Friendship, error)
	Decline(ctx context.Context, requestID, callerID uuid.UUID) error
	Unfriend(ctx context.Context, callerID, otherID uuid.UUID) error
	ListFriends(userID uuid.UUID) ([]FriendDTO, error)
	ListIncoming(userID uuid.UUID) ([]IncomingRequestWithUserDTO, error)
	ListOutgoing(userID uuid.UUID) ([]OutgoingRequestWithUserDTO, error)
	IsFriend(userA, userB uuid.UUID) (bool, error)

	GetBulkStatuses(callerID uuid.UUID, otherIDs []uuid.UUID) (map[uuid.UUID]int, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) SendRequest(ctx context.Context, requesterID, addresseeID uuid.UUID) (*models.Friendship, error) {
	if requesterID == addresseeID {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "You cannot send a friend request to yourself"}
	}

	existing, err := s.repo.FindBetween(requesterID, addresseeID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to check existing friendship"}
	}
	if err == nil {
		if existing.Status == models.FriendshipAccepted {
			return nil, &httputil.ServiceError{Code: http.StatusConflict, Message: "You are already friends"}
		}
		return nil, &httputil.ServiceError{Code: http.StatusConflict, Message: "A friend request already exists between these users"}
	}

	f := &models.Friendship{
		ID:          uuid.New(),
		RequesterID: requesterID,
		AddresseeID: addresseeID,
		Status:      models.FriendshipPending,
	}
	if err := s.repo.Create(f); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to send friend request"}
	}
	return f, nil
}

func (s *service) Accept(ctx context.Context, requestID, callerID uuid.UUID) (*models.Friendship, error) {
	f, err := s.repo.FindByID(requestID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Friend request not found"}
	}
	if f.AddresseeID != callerID {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "Only the addressee can accept this request"}
	}
	if f.Status != models.FriendshipPending {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Friend request is not pending"}
	}

	if err := s.repo.UpdateStatus(requestID, models.FriendshipAccepted); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to accept friend request"}
	}
	f.Status = models.FriendshipAccepted
	return f, nil
}

func (s *service) Decline(ctx context.Context, requestID, callerID uuid.UUID) error {
	f, err := s.repo.FindByID(requestID)
	if err != nil {
		return &httputil.ServiceError{Code: http.StatusNotFound, Message: "Friend request not found"}
	}
	if f.Status != models.FriendshipPending {
		return &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Friend request is not pending"}
	}
	if f.AddresseeID != callerID && f.RequesterID != callerID {
		return &httputil.ServiceError{Code: http.StatusForbidden, Message: "You are not part of this friend request"}
	}

	if err := s.repo.Delete(requestID); err != nil {
		return &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to decline friend request"}
	}
	return nil
}

func (s *service) Unfriend(ctx context.Context, callerID, otherID uuid.UUID) error {
	f, err := s.repo.FindBetween(callerID, otherID)
	if err != nil {
		return &httputil.ServiceError{Code: http.StatusNotFound, Message: "Friendship not found"}
	}
	if f.Status != models.FriendshipAccepted {
		return &httputil.ServiceError{Code: http.StatusBadRequest, Message: "You are not friends with this user"}
	}

	if err := s.repo.Delete(f.ID); err != nil {
		return &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to remove friend"}
	}
	return nil
}

func (s *service) ListFriends(userID uuid.UUID) ([]FriendDTO, error) {
	return s.repo.ListFriends(userID)
}

func (s *service) ListIncoming(userID uuid.UUID) ([]IncomingRequestWithUserDTO, error) {
	return s.repo.ListIncoming(userID)
}

func (s *service) ListOutgoing(userID uuid.UUID) ([]OutgoingRequestWithUserDTO, error) {
	return s.repo.ListOutgoing(userID)
}

func (s *service) IsFriend(userA, userB uuid.UUID) (bool, error) {
	return s.repo.AreFriends(userA, userB)
}

func (s *service) GetBulkStatuses(callerID uuid.UUID, otherIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	return s.repo.GetBulkStatuses(callerID, otherIDs)
}