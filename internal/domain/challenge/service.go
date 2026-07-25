package challenge

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)

type CreateChallengeInput struct {
	Title       string      `json:"title" binding:"required"`
	Description string      `json:"description"`
	Points      int         `json:"points" binding:"required,gt=0"`
	Type        int         `json:"type"`
	CreatorID   uuid.UUID   `json:"creatorId"`
	AssigneeIDs []uuid.UUID `json:"assigneeIds"`
	ExpiresAt   *time.Time  `json:"expiresAt"`
}

type FriendService interface {
	IsFriend(userA, userB uuid.UUID) (bool, error)
}

type TransactionService interface {
	RecordEarnedTx(db *gorm.DB, ownerID, giverID uuid.UUID, amount int, referenceID uuid.UUID) error
}


type Service interface {
	Create(ctx context.Context, input CreateChallengeInput) (*models.Challenge, error)
	Submit(ctx context.Context, challengeID uuid.UUID, userID uuid.UUID) (*models.Challenge, error)
	Approve(ctx context.Context, assignmentID uuid.UUID, callerID uuid.UUID) (*models.ChallengeAssignment, error)
	Cancel(ctx context.Context, challengeID uuid.UUID, callerID uuid.UUID) (*models.Challenge, error)
	GetAll(callerID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	GetByID(callerID uuid.UUID, id uuid.UUID) (*models.Challenge, error)
	GetByAssignee(userID uuid.UUID, statuses []models.AssignmentStatus) ([]models.Challenge, error)
	GetByCreator(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	GetAssignments(callerID uuid.UUID, challengeID uuid.UUID) ([]models.ChallengeAssignment, error)
}

type service struct {
	repo          Repository
	friendService FriendService
	transactionService TransactionService
}

func NewService(repo Repository, friendService FriendService, transactionService TransactionService) Service {
	return &service{repo: repo, friendService: friendService, transactionService: transactionService}
}

func (s *service) Create(ctx context.Context, input CreateChallengeInput) (*models.Challenge, error) {
	for _, assigneeID := range input.AssigneeIDs {
		if assigneeID == input.CreatorID {
			return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "You cannot assign a challenge to yourself"}
		}
		isFriend, err := s.friendService.IsFriend(input.CreatorID, assigneeID)
		if err != nil {
			return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to verify friendship"}
		}
		if !isFriend {
			return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You can only assign challenges to your friends"}
		}
	}

	challenge := &models.Challenge{
		ID:          uuid.New(),
		Title:       input.Title,
		Description: input.Description,
		Points:      input.Points,
		Type:        models.ChallengeType(input.Type),
		CreatorID:   input.CreatorID,
		Restricted:  len(input.AssigneeIDs) > 0,
		ExpiresAt:   input.ExpiresAt,
		Status:      models.StatusActive,
	}

	if err := s.repo.Transact(func(tx *gorm.DB) error {
		if err := s.repo.CreateTx(tx, challenge); err != nil {
			return err
		}
		for _, assigneeID := range input.AssigneeIDs {
			assignment := &models.ChallengeAssignment{
				ID:          uuid.New(),
				ChallengeID: challenge.ID,
				AssigneeID:  assigneeID,
				Status:      models.AssignmentAssigned,
			}
			if err := s.repo.CreateAssignmentTx(tx, assignment); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to create challenge"}
	}
	return challenge, nil
}

func (s *service) Submit(ctx context.Context, challengeID uuid.UUID, userID uuid.UUID) (*models.Challenge, error) {
	challenge, err := s.repo.FindByID(challengeID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Challenge not found"}
	}

	if challenge.ExpiresAt != nil && time.Now().After(*challenge.ExpiresAt) {
		challenge.Status = models.StatusExpired
		s.repo.Save(challenge) //nolint:errcheck
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Challenge has expired"}
	}

	if challenge.Status != models.StatusActive {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Challenge is not active"}
	}

	assignment, err := s.repo.FindAssignment(challengeID, userID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to look up assignment"}
		}

		if challenge.Restricted {
			return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You have not been assigned this challenge"}
		}

		isFriend, err := s.friendService.IsFriend(challenge.CreatorID, userID)
		if err != nil {
			return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to verify friendship"}
		}
		if !isFriend {
			return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You are not friends with the challenge creator"}
		}

		now := time.Now()
		newAssignment := &models.ChallengeAssignment{
			ID:          uuid.New(),
			ChallengeID: challengeID,
			AssigneeID:  userID,
			Status:      models.AssignmentSubmitted,
			SubmittedAt: &now,
		}
		if err := s.repo.CreateAssignment(newAssignment); err != nil {
			return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to submit challenge"}
		}
		return challenge, nil
	}

	if assignment.Status != models.AssignmentAssigned {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Challenge has already been submitted"}
	}

	now := time.Now()
	assignment.Status = models.AssignmentSubmitted
	assignment.SubmittedAt = &now
	if err := s.repo.SaveAssignment(assignment); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to submit challenge"}
	}

	return challenge, nil
}

func (s *service) Approve(ctx context.Context, assignmentID uuid.UUID, callerID uuid.UUID) (*models.ChallengeAssignment, error) {
	assignment, err := s.repo.FindAssignmentByID(assignmentID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Assignment not found"}
	}

	if assignment.Status != models.AssignmentSubmitted {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Assignment is not pending approval"}
	}

	challenge, err := s.repo.FindByID(assignment.ChallengeID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Challenge not found"}
	}

	if challenge.CreatorID != callerID {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "User is not the creator of this challenge"}
	}

	if challenge.Status != models.StatusActive {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Challenge is no longer active"}
	}

	now := time.Now()
	assignment.Status = models.AssignmentApproved
	assignment.ApprovedAt = &now

	if err := s.repo.Transact(func(tx *gorm.DB) error {
		if err := s.repo.SaveAssignmentTx(tx, assignment); err != nil {
			return err
		}
		return s.transactionService.RecordEarnedTx(tx, assignment.AssigneeID, challenge.CreatorID, challenge.Points, assignment.ID)
	}); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to approve challenge"}
	}

	return assignment, nil
}



func (s *service) Cancel(ctx context.Context, challengeID uuid.UUID, callerID uuid.UUID) (*models.Challenge, error) {
	challenge, err := s.repo.FindByID(challengeID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Challenge not found"}
	}

	if challenge.CreatorID != callerID {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "Only the challenge creator can cancel it"}
	}

	if challenge.Status != models.StatusActive {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Only active challenges can be cancelled"}
	}

	challenge.Status = models.StatusCancelled
	if err := s.repo.Save(challenge); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to cancel challenge"}
	}

	return challenge, nil
}

func (s *service) GetAll(callerID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	return s.repo.FindVisibleToUser(callerID, statuses)
}

func (s *service) GetByID(callerID uuid.UUID, id uuid.UUID) (*models.Challenge, error) {
	challenge, err := s.repo.FindByID(id)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Challenge not found"}
	}
	if challenge.CreatorID == callerID {
		return challenge, nil
	}
	if _, err := s.repo.FindAssignment(id, callerID); err == nil {
		return challenge, nil
	}
	if !challenge.Restricted {
		isFriend, err := s.friendService.IsFriend(challenge.CreatorID, callerID)
		if err != nil {
			return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to verify friendship"}
		}
		if isFriend {
			return challenge, nil
		}
	}
	return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You do not have access to this challenge"}
}

func (s *service) GetByAssignee(userID uuid.UUID, statuses []models.AssignmentStatus) ([]models.Challenge, error) {
	return s.repo.FindByAssignee(userID, statuses)
}

func (s *service) GetByCreator(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	return s.repo.FindByCreator(userID, statuses)
}

func (s *service) GetAssignments(callerID uuid.UUID, challengeID uuid.UUID) ([]models.ChallengeAssignment, error) {
	if _, err := s.GetByID(callerID, challengeID); err != nil {
		return nil, err
	}
	return s.repo.FindAssignmentsByChallenge(challengeID)
}
