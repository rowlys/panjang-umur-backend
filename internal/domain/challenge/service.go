package challenge

import (
	"context"
	"errors"
	"net/http"
	"time"
	"fmt"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"github.com/rowlys/panjang-umur-backend/internal/pkg/image_storage"
	"gorm.io/gorm"
)

type CreateChallengeInput struct {
	Title       string      `json:"title" binding:"required"`
	Description string      `json:"description"`
	Points      int         `json:"points" binding:"required,gt=0"`
	Type        int         `json:"type"`
	ResetDay    *int        `json:"resetDay"`
	CreatorID   uuid.UUID   `json:"creatorId"`
	AssigneeIDs []uuid.UUID `json:"assigneeIds"`
	ExpiresAt   *time.Time  `json:"expiresAt"`
}

type FriendService interface {
	IsFriend(userA, userB uuid.UUID) (bool, error)
}

type TransactionService interface {
	RecordEarnedTx(db *gorm.DB, ownerID, giverID uuid.UUID, amount int, referenceID uuid.UUID, referenceType models.TransactionReferenceType) error
}

type Service interface {
	Create(ctx context.Context, input CreateChallengeInput) (*models.Challenge, error)
	Submit(ctx context.Context, challengeID uuid.UUID, userID uuid.UUID, proofImageID *uuid.UUID) (*models.Challenge, error)
	Approve(ctx context.Context, submissionID uuid.UUID, callerID uuid.UUID) (*models.ChallengeSubmission, error)
	Cancel(ctx context.Context, challengeID uuid.UUID, callerID uuid.UUID) (*models.Challenge, error)
	GetAll(callerID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	GetByID(callerID uuid.UUID, id uuid.UUID) (*models.Challenge, error)
	GetByIDs(ids []uuid.UUID) ([]models.Challenge, error)
	GetByAssignee(userID uuid.UUID) ([]models.Challenge, error)
	GetByCreator(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)

	// GetSubmissionsSubmitted returns submissions callerID made, optionally narrowed to a single challenge.
	GetSubmissionsSubmitted(callerID uuid.UUID, challengeID *uuid.UUID, statusFilter string, before *time.Time, limit int) ([]models.ChallengeSubmission, error)
	// GetSubmissionsReceived returns submissions made on challenges callerID created,
	// optionally narrowed to a single challenge (in which case callerID must be its creator).
	GetSubmissionsReceived(callerID uuid.UUID, challengeID *uuid.UUID, statusFilter string, before *time.Time, limit int) ([]models.ChallengeSubmission, error)
	GetMySubmissionStatus(callerID uuid.UUID, challenge *models.Challenge) (*models.SubmissionStatus, error)
	// GetSubmissionContexts resolves History-ready context (challenge title,
	// participant IDs) for the given submission IDs. Used by the transaction
	// handler to enrich the ledger without importing this package.
	GetSubmissionContexts(ids []uuid.UUID) ([]models.SubmissionContext, error)

	GenerateProofUploadURL(ctx context.Context) (string, uuid.UUID, error)
	GetProofURL(imageID *uuid.UUID) *string
}

type service struct {
	repo               Repository
	friendService      FriendService
	transactionService TransactionService
	imageStorage 	   image_storage.Service
}

func NewService(repo Repository, friendService FriendService, transactionService TransactionService, imageStorage image_storage.Service) Service {
	return &service{repo: repo, friendService: friendService, transactionService: transactionService, imageStorage: imageStorage}
}

func currentPeriodStart(t models.ChallengeType, at time.Time, resetDay int) time.Time {
	at = at.UTC()
	switch t {
	case models.Daily:
		return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	case models.Weekly:
		targetDay := resetDay

		daysSince := (int(at.Weekday()) - targetDay + 7) % 7

		d := at.AddDate(0, 0, -daysSince)
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)

	default:
		return time.Time{}
	}
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

	resetDay := 0

	if input.ResetDay != nil {
		resetDay = *input.ResetDay
	}

	challenge := &models.Challenge{
		ID:          uuid.New(),
		Title:       input.Title,
		Description: input.Description,
		Points:      input.Points,
		Type:        models.ChallengeType(input.Type),
		ResetDay:    resetDay,
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
			assignee := &models.ChallengeAssignee{
				ID:          uuid.New(),
				ChallengeID: challenge.ID,
				UserID:      assigneeID,
			}
			if err := s.repo.CreateAssigneeTx(tx, assignee); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to create challenge"}
	}
	return challenge, nil
}

func (s *service) Submit(ctx context.Context, challengeID uuid.UUID, userID uuid.UUID, proofImageID *uuid.UUID) (*models.Challenge, error) {
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

	period := currentPeriodStart(challenge.Type, time.Now(), challenge.ResetDay)
	existing, err := s.repo.FindSubmissionForPeriod(challengeID, userID, period)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to look up submission"}
		}

		if challenge.Restricted {
			if _, err := s.repo.FindAssignee(challengeID, userID); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You have not been assigned this challenge"}
				}
				return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to look up assignee"}
			}
		} else {
			isFriend, err := s.friendService.IsFriend(challenge.CreatorID, userID)
			if err != nil {
				return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to verify friendship"}
			}
			if !isFriend {
				return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You are not friends with the challenge creator"}
			}
		}

		if proofImageID != nil && *proofImageID != uuid.Nil {
			tmpKey := fmt.Sprintf("tmp/%s", proofImageID.String())
			destKey := fmt.Sprintf("submissions/%s", proofImageID.String())

			err := s.imageStorage.PromoteFile(ctx, tmpKey, destKey)
			if err != nil {
				return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to promote proof image"}
			}
		}

		submission := &models.ChallengeSubmission{
			ID:          uuid.New(),
			ChallengeID: challengeID,
			UserID:      userID,
			ProofImageID: proofImageID,	
			PeriodStart: period,
			Status:      models.SubmissionSubmitted,
			SubmittedAt: time.Now(),
		}
		if err := s.repo.CreateSubmission(submission); err != nil {
			return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to submit challenge"}
		}
		return challenge, nil
	}

	if existing.Status == models.SubmissionApproved {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Challenge has already been completed for this period"}
	}

	return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Challenge has already been submitted"}
}

func (s *service) Approve(ctx context.Context, submissionID uuid.UUID, callerID uuid.UUID) (*models.ChallengeSubmission, error) {
	submission, err := s.repo.FindSubmissionByID(submissionID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Submission not found"}
	}

	if submission.Status != models.SubmissionSubmitted {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Submission is not pending approval"}
	}

	challenge, err := s.repo.FindByID(submission.ChallengeID)
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
	submission.Status = models.SubmissionApproved
	submission.ApprovedAt = &now

	if err := s.repo.Transact(func(tx *gorm.DB) error {
		if err := s.repo.SaveSubmissionTx(tx, submission); err != nil {
			return err
		}
		if err := s.transactionService.RecordEarnedTx(tx, submission.UserID, challenge.CreatorID, challenge.Points, submission.ID, models.ReferenceSubmission); err != nil {
			return err
		}

		if challenge.Type == models.Bounty && challenge.Restricted {
			missingCount, err := s.repo.CountAssigneesWithoutApprovedSubmission(tx, challenge.ID)
			if err != nil {
				return err
			}

			if missingCount == 0 {
				challenge.Status = models.StatusCompleted
				if err := s.repo.SaveTx(tx, challenge); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to approve submission"}
	}

	return submission, nil
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
	if _, err := s.repo.FindAssignee(id, callerID); err == nil {
		return challenge, nil
	}
	if _, err := s.repo.FindLatestSubmission(id, callerID); err == nil {
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

func (s *service) GetByAssignee(userID uuid.UUID) ([]models.Challenge, error) {
	candidates, err := s.repo.FindChallengesInvolvingUser(userID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to load challenges"}
	}

	if len(candidates) == 0 {
		return candidates, nil
	}

	challengeIDs := make([]uuid.UUID, len(candidates))
	for i, c := range candidates {
		challengeIDs[i] = c.ID
	}

	submissions, err := s.repo.FindSubmissionsByUserAndChallenges(userID, challengeIDs)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to load submissions"}
	}

	// Track every period the user has already submitted for (submitted OR approved),
	// per challenge, so a challenge with submission history across multiple periods
	// (e.g. a daily challenge done on previous days) doesn't shadow the current period.
	submittedPeriods := make(map[uuid.UUID]map[int64]struct{})
	for _, sub := range submissions {
		if submittedPeriods[sub.ChallengeID] == nil {
			submittedPeriods[sub.ChallengeID] = make(map[int64]struct{})
		}
		submittedPeriods[sub.ChallengeID][sub.PeriodStart.UnixNano()] = struct{}{}
	}

	var result []models.Challenge
	for _, c := range candidates {
		if c.Status != models.StatusActive {
			continue
		}
		currentPeriod := currentPeriodStart(c.Type, time.Now(), c.ResetDay)
		if periods, exists := submittedPeriods[c.ID]; exists {
			if _, alreadySubmitted := periods[currentPeriod.UnixNano()]; alreadySubmitted {
				continue
			}
		}
		result = append(result, c)
	}
	return result, nil
}

func (s *service) GetMySubmissionStatus(callerID uuid.UUID, challenge *models.Challenge) (*models.SubmissionStatus, error) {
	period := currentPeriodStart(challenge.Type, time.Now(), challenge.ResetDay)
	submission, err := s.repo.FindSubmissionForPeriod(challenge.ID, callerID, period)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to look up submission"}
	}
	return &submission.Status, nil
}

func (s *service) GetByCreator(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	return s.repo.FindByCreator(userID, statuses)
}

func (s *service) GetByIDs(ids []uuid.UUID) ([]models.Challenge, error) {
	challenges, err := s.repo.FindByIDs(ids)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch challenge details"}
	}
	return challenges, nil
}

func (s *service) GetSubmissionContexts(ids []uuid.UUID) ([]models.SubmissionContext, error) {
	if len(ids) == 0 {
		return []models.SubmissionContext{}, nil
	}

	submissions, err := s.repo.FindSubmissionsByIDs(ids)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch submission details"}
	}

	uniqueChallengeIDs := make(map[uuid.UUID]struct{})
	var challengeIDs []uuid.UUID
	for _, submission := range submissions {
		if _, exists := uniqueChallengeIDs[submission.ChallengeID]; !exists {
			uniqueChallengeIDs[submission.ChallengeID] = struct{}{}
			challengeIDs = append(challengeIDs, submission.ChallengeID)
		}
	}

	challenges, err := s.repo.FindByIDs(challengeIDs)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch challenge details"}
	}
	challengeMap := make(map[uuid.UUID]models.Challenge)
	for _, ch := range challenges {
		challengeMap[ch.ID] = ch
	}

	contexts := make([]models.SubmissionContext, len(submissions))
	for i, submission := range submissions {
		ch := challengeMap[submission.ChallengeID]
		contexts[i] = models.SubmissionContext{
			SubmissionID:   submission.ID,
			ChallengeID:    submission.ChallengeID,
			ChallengeTitle: ch.Title,
			SubmitterID:    submission.UserID,
			CreatorID:      ch.CreatorID,
		}
	}
	return contexts, nil
}

func clampSubmissionsLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func parseSubmissionStatusFilter(statusFilter string) *models.SubmissionStatus {
	switch statusFilter {
	case "submitted":
		submitted := models.SubmissionSubmitted
		return &submitted
	case "approved":
		approved := models.SubmissionApproved
		return &approved
	default:
		return nil
	}
}

// GetSubmissionsSubmitted returns submissions callerID made, optionally narrowed to a single challenge.
func (s *service) GetSubmissionsSubmitted(callerID uuid.UUID, challengeID *uuid.UUID, statusFilter string, before *time.Time, limit int) ([]models.ChallengeSubmission, error) {
	limit = clampSubmissionsLimit(limit)
	status := parseSubmissionStatusFilter(statusFilter)

	submissions, err := s.repo.FindSubmissionsSubmitted(callerID, challengeID, status, before, limit)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to load submissions"}
	}
	return submissions, nil
}

// GetSubmissionsReceived returns submissions made on challenges callerID created,
// optionally narrowed to a single challenge (in which case callerID must be its creator).
func (s *service) GetSubmissionsReceived(callerID uuid.UUID, challengeID *uuid.UUID, statusFilter string, before *time.Time, limit int) ([]models.ChallengeSubmission, error) {
	if challengeID != nil {
		challenge, err := s.GetByID(callerID, *challengeID)
		if err != nil {
			return nil, err
		}
		if challenge.CreatorID != callerID {
			return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You do not have access to this challenge's submissions"}
		}
	}

	limit = clampSubmissionsLimit(limit)
	status := parseSubmissionStatusFilter(statusFilter)

	submissions, err := s.repo.FindSubmissionsReceived(callerID, challengeID, status, before, limit)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to load submissions"}
	}
	return submissions, nil
}

func (s *service) GenerateProofUploadURL(ctx context.Context) (string, uuid.UUID, error) {
	imageID := uuid.New()
	tmpKey := fmt.Sprintf("tmp/%s", imageID.String())

	url, err := s.imageStorage.GeneratePresignedURL(ctx, tmpKey, 15*time.Minute)


	if err != nil {
		return "", uuid.Nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to generate presigned URL"}
	}

	return url, imageID, nil
}

func (s *service) GetProofURL(imageID *uuid.UUID) *string {
    if imageID == nil || *imageID == uuid.Nil {
        return nil
    }
    
    key := fmt.Sprintf("submissions/%s", imageID.String())
    
    url := s.imageStorage.GetPublicURL(key)
    return &url
}