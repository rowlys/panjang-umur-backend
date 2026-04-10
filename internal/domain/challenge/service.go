package challenge

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
)


type ServiceError struct {
	Code   int
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}

type CreateChallengeInput struct {
  	Title    	string   	`json:"title" binding:"required"`
  	Description string   	`json:"description"`
 	Points   	int    		`json:"points" binding:"required,gt=0"` 
	Type    	int    		`json:"type"`                
	CreatorID 	uuid.UUID 	`json:"creatorId"`
	AssigneeID 	*uuid.UUID 	`json:"assigneeId"`
	GroupID 	uuid.UUID 	`json:"groupId"`
}

type UserService interface {
	AddPoints(userID uuid.UUID, points int) error
}

type GroupService interface {
	IsUserInGroup(userID uuid.UUID, groupID uuid.UUID) (bool, error)
}

type Service interface {
	Create(ctx context.Context, input CreateChallengeInput) (*models.Challenge, error)
	Submit(ctx context.Context, challengeID uuid.UUID, userID uuid.UUID) (*models.Challenge, error)
	Approve(ctx context.Context, challengeID uuid.UUID, userID uuid.UUID) (*models.Challenge, error)
	GetAll(statuses []models.ChallengeStatus) ([]models.Challenge, error)
	GetByID(id uuid.UUID) (*models.Challenge, error)
	GetByAssignee(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	GetByAssigneeAndGroup(userID uuid.UUID, groupID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	GetByCreator(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	GetByCreatorAndGroup(userID uuid.UUID, groupID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
}

type service struct {
	repo Repository
	userService UserService
}

func NewService(repo Repository, userService UserService) Service {
	return &service{repo: repo, userService: userService}
}


// Challenge modification functions

func (s *service) Create(ctx context.Context, input CreateChallengeInput) (*models.Challenge, error) {
	creatorIsMember, err := s.userService.IsUserInGroup(input.CreatorID, input.GroupID)
	if err != nil {
		return nil, &ServiceError{Code: http.StatusInternalServerError, Message: "Failed to verify group membership"}
	}
	if !creatorIsMember {
		return nil, &ServiceError{Code: http.StatusForbidden, Message: "You are not a member of the group"}
	}

	assigneeIsMember, err := s.userService.IsUserInGroup(*input.AssigneeID, input.GroupID)
	if err != nil {
		return nil, &ServiceError{Code: http.StatusInternalServerError, Message: "Failed to verify group membership"}
	}
	if !assigneeIsMember {
		return nil, &ServiceError{Code: http.StatusForbidden, Message: "Assignee is not a member of the group"}
	}
	
	challenge := &models.Challenge{
		ID:          uuid.New(),
		Title:       input.Title,
		Description: input.Description,
		Points:      input.Points,
		Type:        models.ChallengeType(input.Type),
		CreatorID:   input.CreatorID,
		AssigneeID:  input.AssigneeID,
		GroupID:     input.GroupID,
		Status:      models.StatusActive, 
	}
	
	if err := s.repo.Create(challenge); err != nil {
		return nil, &ServiceError{Code: http.StatusInternalServerError, Message: "Failed to create challenge"}
	}
	return challenge, nil
}

func (s *service) Submit(ctx context.Context, challengeID uuid.UUID, userID uuid.UUID) (*models.Challenge, error) {
	challenge, err := s.repo.FindByID(challengeID)
	if err != nil {
		return nil, &ServiceError{Code: http.StatusNotFound, Message: "Challenge not found"}
	}

	if challenge.Status != models.StatusActive {
		return nil, &ServiceError{Code: http.StatusBadRequest, Message: "Challenge is not active"}
	}

	if challenge.AssigneeID != nil && challenge.AssigneeID != userID {
		return nil, &ServiceError{Code: http.StatusForbidden, Message: "User is not the assignee of this challenge"}
	}

	challenge.Status = models.StatusPending
	if err := s.repo.Save(challenge); err != nil {
		return nil, &ServiceError{Code: http.StatusInternalServerError, Message: "Failed to submit challenge"}
	}

	return challenge, nil
}
		
func (s *service) Approve(ctx context.Context, challengeID uuid.UUID, userID uuid.UUID) (*models.Challenge, error) {
	challenge, err := s.repo.FindByID(challengeID)
	if err != nil {
		return nil, &ServiceError{Code: http.StatusNotFound, Message: "Challenge not found"}
	}

	if challenge.Status != models.StatusPending {
		return nil, &ServiceError{Code: http.StatusBadRequest, Message: "Challenge is not pending approval"}
	}

	if challenge.CreatorID != nil && challenge.CreatorID != userID {
		return nil, &ServiceError{Code: http.StatusForbidden, Message: "User is not the creator of this challenge"}
	}

	challenge.Status = models.StatusCompleted
	if err := s.repo.Save(challenge); err != nil {
		return nil, &ServiceError{Code: http.StatusInternalServerError, Message: "Failed to submit challenge"}
	}

	if err := s.userService.AddPoints(challenge.AssigneeID, challenge.Points); err != nil {
		return nil, &ServiceError{Code: http.StatusInternalServerError, Message: "Failed to award points to assignee"}
	}

	return challenge, nil
}



// Challenge retrieval functions

var allStatuses = []models.ChallengeStatus{models.StatusActive, models.StatusPending, models.StatusCompleted, models.StatusExpired}
var activeStatuses = []models.ChallengeStatus{models.StatusActive, models.StatusPending}
var inactiveStatuses = []models.ChallengeStatus{models.StatusCompleted, models.StatusExpired}

func (s *service) GetAll(statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	return s.repo.FindAll(statuses)
}

func (s *service) GetByID(id uuid.UUID) (*models.Challenge, error) {
	return s.repo.FindByID(id)
}

func (s *service) GetByAssignee(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	return s.repo.FindByAssignee(userID, statuses)
}

func (s *service) GetByAssigneeAndGroup(userID uuid.UUID, groupID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	return s.repo.FindByAssigneeAndGroup(userID, groupID, statuses)
}

func (s *service) GetByCreator(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	return s.repo.FindByCreator(userID, statuses)
}

func (s *service) GetByCreatorAndGroup(userID uuid.UUID, groupID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	return s.repo.FindByCreatorAndGroup(userID, groupID, statuses)
}


// ServiceError resolve function

func ResolveServiceError(err error) (int, string) {
	var serviceErr *ServiceError

	if errors.As(err, &serviceErr) {
		return serviceErr.Code, serviceErr.Message
	}
	return http.StatusInternalServerError, "Internal server error"
}