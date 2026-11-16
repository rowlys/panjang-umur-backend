package reward

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)

type CreateRewardInput struct {
	Title          string                      `json:"title"`
	Description    string                      `json:"description"`
	Cost           int                         `json:"cost"`
	Visibility     models.RewardVisibilityMode `json:"visibility"`
	Stock          int                         `json:"stock"`
	AllowedUserIDs []uuid.UUID                 `json:"allowedUserIds"`
	GiverID        uuid.UUID                   `json:"giverId"`
}

// UpdateRewardInput carries only the fields the owner sent. A nil pointer means
// "leave it alone". AllowedUserIDs is the complete new recipient list and is
// only read when Visibility is set to Restricted.
type UpdateRewardInput struct {
	Title          *string
	Description    *string
	Cost           *int
	Stock          *int
	Visibility     *models.RewardVisibilityMode
	AllowedUserIDs []uuid.UUID
}

type RewardClaimRedeemed struct {
	ID		   uuid.UUID
	RewardID   uuid.UUID
	RewardTitle string
	RedeemerID uuid.UUID
	GiverID    uuid.UUID
	Price      int
	Status     models.ClaimStatus
	RedeemedAt time.Time
	FulfilledAt *time.Time
	ResolvedAt  *time.Time
	GiverUsername string
}

type RewardClaimGiven struct {
	ID		   	 	uuid.UUID
	RewardID   	 	uuid.UUID
	RewardTitle 	string
	RedeemerID 	 	uuid.UUID
	GiverID    	 	uuid.UUID
	Price      	 	int
	Status     	 	models.ClaimStatus
	RedeemedAt 	 	time.Time
	FulfilledAt 	*time.Time
	ResolvedAt  	*time.Time
	RedeemerUsername string
}

type UserService interface {
	GetByIDs(userIDs []uuid.UUID) ([]*models.BareUserDTO, error)
}

type FriendService interface {
	IsFriend(userA, userB uuid.UUID) (bool, error)
}

type TransactionService interface {
	RecordEarnedTx(db *gorm.DB, ownerID, giverID uuid.UUID, amount int, referenceID uuid.UUID, referenceType models.TransactionReferenceType) error
	RecordSpentTx(db *gorm.DB, ownerID, giverID uuid.UUID, amount int, referenceID uuid.UUID, referenceType models.TransactionReferenceType) error
	GetBalance(ownerID, giverID uuid.UUID) (int, error)
	GetBalanceTx(db *gorm.DB, ownerID, giverID uuid.UUID) (int, error)
}

type Service interface {
	Create(ctx context.Context, input CreateRewardInput) (*models.Reward, error)
	Update(ctx context.Context, rewardID uuid.UUID, callerID uuid.UUID, input UpdateRewardInput) (*models.Reward, error)
	Redeem(ctx context.Context, rewardID uuid.UUID, redeemerID uuid.UUID) (*models.Reward, error)
	GetByID(rewardID uuid.UUID, callerID uuid.UUID) (*models.Reward, error)
	// GetAllowedUserIDs returns the recipient list of each restricted reward.
	// Public rewards have no entries.
	GetAllowedUserIDs(rewardIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
	GetShopByGiver(callerID uuid.UUID, giverID uuid.UUID, availableOnly bool) ([]models.Reward, error)
	// GetClaimsGiven returns claims against callerID's rewards, optionally narrowed to a
	// single reward (in which case callerID must be that reward's giver).
	GetClaimsGiven(callerID uuid.UUID, rewardID *uuid.UUID, before *time.Time, limit int) ([]RewardClaimGiven, error)

	MarkClaimFulfilled(claimID uuid.UUID, callerID uuid.UUID) (*models.RewardClaim, error)
	MarkClaimRefundRequested(claimID uuid.UUID, callerID uuid.UUID, reason string) (*models.RewardClaim, error)
	MarkClaimRefunded(claimID uuid.UUID, callerID uuid.UUID) (*models.RewardClaim, error)

	// GetClaimsRedeemed returns claims redeemerID has made, optionally narrowed to claims
	// against a single giver's rewards.
	GetClaimsRedeemed(redeemerID uuid.UUID, giverID *uuid.UUID, before *time.Time, limit int) ([]RewardClaimRedeemed, error)
	// GetClaimContexts resolves History-ready context (reward title, participant
	// IDs) for the given claim IDs. Used by the transaction handler to enrich
	// the ledger without importing this package.
	GetClaimContexts(ids []uuid.UUID) ([]models.ClaimContext, error)
}

type service struct {
	repo               Repository
	userService        UserService
	friendService      FriendService
	transactionService TransactionService
}

func NewService(repo Repository, userService UserService, friendService FriendService, transactionService TransactionService) Service {
	return &service{repo: repo, userService: userService, friendService: friendService, transactionService: transactionService}
}

func (s *service) Create(ctx context.Context, input CreateRewardInput) (*models.Reward, error) {
	if input.Visibility == models.Restricted {
		if len(input.AllowedUserIDs) == 0 {
			return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Select at least one friend for a restricted reward"}
		}
		for _, allowedID := range input.AllowedUserIDs {
			isFriend, err := s.friendService.IsFriend(input.GiverID, allowedID)
			if err != nil {
				return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to verify friendship"}
			}
			if !isFriend {
				return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You can only restrict rewards to your friends"}
			}
		}
	}

	reward := &models.Reward{
		ID:            uuid.New(),
		Title:         input.Title,
		Description:   input.Description,
		Cost:          input.Cost,
		RewardGiverID: input.GiverID,
		Visibility:    input.Visibility,
		Stock:         input.Stock,
		IsAvailable:   true,
	}

	if err := s.repo.Transact(func(tx *gorm.DB) error {
		if err := s.repo.CreateTx(tx, reward); err != nil {
			return err
		}
		if reward.Visibility == models.Restricted {
			return s.repo.SyncVisibilityTx(tx, reward.ID, input.AllowedUserIDs)
		}
		return nil
	}); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to create reward"}
	}
	return reward, nil
}

func (s *service) Update(ctx context.Context, rewardID uuid.UUID, callerID uuid.UUID, input UpdateRewardInput) (*models.Reward, error) {
	existing, err := s.repo.FindByID(rewardID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Reward not found"}
	}
	if existing.RewardGiverID != callerID {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "Only the reward giver can edit it"}
	}

	if input.Title != nil && strings.TrimSpace(*input.Title) == "" {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Title is required"}
	}
	if input.Cost != nil && *input.Cost <= 0 {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Cost must be greater than zero"}
	}
	if input.Stock != nil && *input.Stock < 0 {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Stock cannot be negative"}
	}

	var allowedUserIDs []uuid.UUID
	if input.Visibility != nil {
		if *input.Visibility != models.Public && *input.Visibility != models.Restricted {
			return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Invalid visibility"}
		}
		if *input.Visibility == models.Restricted {
			if len(input.AllowedUserIDs) == 0 {
				return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Select at least one friend for a restricted reward"}
			}
			for _, allowedID := range input.AllowedUserIDs {
				isFriend, err := s.friendService.IsFriend(callerID, allowedID)
				if err != nil {
					return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to verify friendship"}
				}
				if !isFriend {
					return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You can only restrict rewards to your friends"}
				}
			}
			allowedUserIDs = input.AllowedUserIDs
		}
	}

	var updated *models.Reward
	if err := s.repo.Transact(func(tx *gorm.DB) error {
		reward, err := s.repo.FindByIDForUpdate(tx, rewardID)
		if err != nil {
			return &httputil.ServiceError{Code: http.StatusNotFound, Message: "Reward not found"}
		}
		if reward.RewardGiverID != callerID {
			return &httputil.ServiceError{Code: http.StatusForbidden, Message: "Only the reward giver can edit it"}
		}

		if input.Title != nil {
			reward.Title = strings.TrimSpace(*input.Title)
		}
		if input.Description != nil {
			reward.Description = *input.Description
		}
		if input.Cost != nil {
			reward.Cost = *input.Cost
		}
		if input.Stock != nil {
			reward.Stock = *input.Stock
		}
		if input.Visibility != nil {
			reward.Visibility = *input.Visibility
			if err := s.repo.SyncVisibilityTx(tx, reward.ID, allowedUserIDs); err != nil {
				return err
			}
		}

		reward.IsAvailable = reward.Stock > 0
		if err := s.repo.SaveTx(tx, reward); err != nil {
			return err
		}

		updated = reward
		return nil
	}); err != nil {
		var svcErr *httputil.ServiceError
		if errors.As(err, &svcErr) {
			return nil, svcErr
		}
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to update reward"}
	}

	return updated, nil
}

func (s *service) Redeem(ctx context.Context, rewardID uuid.UUID, redeemerID uuid.UUID) (*models.Reward, error) {
	reward, err := s.repo.FindByID(rewardID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Reward not found"}
	}

	if reward.Stock <= 0 {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Reward has already been fully redeemed"}
	}

	isFriend, err := s.friendService.IsFriend(redeemerID, reward.RewardGiverID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to verify friendship"}
	}
	if !isFriend {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You are not friends with the reward giver"}
	}

	if reward.Visibility == models.Restricted {
		visible, err := s.repo.IsVisibleTo(rewardID, redeemerID)
		if err != nil {
			return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to verify reward visibility"}
		}
		if !visible {
			return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "This reward is not available to you"}
		}
	}

	var redeemed *models.Reward
	if err := s.repo.Transact(func(tx *gorm.DB) error {
		// Re-read under a row lock: the checks above ran on a snapshot that
		// another redeem or an edit may already have changed.
		locked, err := s.repo.FindByIDForUpdate(tx, rewardID)
		if err != nil {
			return &httputil.ServiceError{Code: http.StatusNotFound, Message: "Reward not found"}
		}
		if locked.Stock <= 0 {
			return &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Reward has already been fully redeemed"}
		}

		balance, err := s.transactionService.GetBalanceTx(tx, redeemerID, locked.RewardGiverID)
		if err != nil {
			return err
		}
		if balance < locked.Cost {
			return &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Insufficient points"}
		}

		locked.Stock--
		locked.IsAvailable = locked.Stock > 0

		if err := s.repo.SaveTx(tx, locked); err != nil {
			return err
		}

		claimID := uuid.New()
		if err := s.repo.CreateClaimTx(tx, &models.RewardClaim{
			ID:          claimID,
			RewardID:    locked.ID,
			RewardTitle: locked.Title,
			RedeemerID:  redeemerID,
			GiverID:     locked.RewardGiverID,
			Price:       locked.Cost,
			Status:      models.ClaimStatusPending,
		}); err != nil {
			return err
		}

		if err := s.transactionService.RecordSpentTx(tx, redeemerID, locked.RewardGiverID, locked.Cost, claimID, models.ReferenceClaim); err != nil {
			return err
		}

		redeemed = locked
		return nil
	}); err != nil {
		var svcErr *httputil.ServiceError
		if errors.As(err, &svcErr) {
			return nil, svcErr
		}
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to redeem reward"}
	}

	return redeemed, nil
}

func (s *service) GetByID(rewardID uuid.UUID, callerID uuid.UUID) (*models.Reward, error) {
	reward, err := s.repo.FindByID(rewardID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Reward not found"}
	}

	if reward.RewardGiverID != callerID {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You do not have access to this reward"}
	}

	return reward, nil
}

func (s *service) GetAllowedUserIDs(rewardIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	visibility, err := s.repo.FindVisibilityByRewardIDs(rewardIDs)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch reward visibility"}
	}
	return visibility, nil
}

func (s *service) GetShopByGiver(callerID uuid.UUID, giverID uuid.UUID, availableOnly bool) ([]models.Reward, error) {
	if callerID != giverID {
		isFriend, err := s.friendService.IsFriend(callerID, giverID)
		if err != nil {
			return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to verify friendship"}
		}
		if !isFriend {
			return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You are not friends with this user"}
		}
	}
	
	rewards, err := s.repo.FindShopByGiver(callerID, giverID, availableOnly)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch rewards"}
	}
	return rewards, nil
}

func clampClaimsLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func (s *service) GetClaimContexts(ids []uuid.UUID) ([]models.ClaimContext, error) {
	if len(ids) == 0 {
		return []models.ClaimContext{}, nil
	}

	claims, err := s.repo.FindClaimsByIDs(ids)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch claim details"}
	}

	contexts := make([]models.ClaimContext, len(claims))
	for i, claim := range claims {
		contexts[i] = models.ClaimContext{
			ClaimID:     claim.ID,
			RewardID:    claim.RewardID,
			RewardTitle: claim.RewardTitle,
			RedeemerID:  claim.RedeemerID,
			GiverID:     claim.GiverID,
		}
	}
	return contexts, nil
}

func (s *service) enrichClaimsWithRedeemerUsernames(claims []models.RewardClaim) ([]RewardClaimGiven, error) {
	if len(claims) == 0 {
		return []RewardClaimGiven{}, nil
	}

	uniqueRedeemerIDs := make(map[uuid.UUID]struct{})
	var redeemerIDs []uuid.UUID

	for _, claim := range claims {
		if _, exists := uniqueRedeemerIDs[claim.RedeemerID]; !exists {
			uniqueRedeemerIDs[claim.RedeemerID] = struct{}{}
			redeemerIDs = append(redeemerIDs, claim.RedeemerID)
		}
	}

	users, err := s.userService.GetByIDs(redeemerIDs)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch user details"}
	}

	redeemerUsernameMap := make(map[uuid.UUID]string)
	for _, user := range users {
		redeemerUsernameMap[user.ID] = user.Username
	}

	given := make([]RewardClaimGiven, len(claims))
	for i, claim := range claims {
		redeemerUsername, exists := redeemerUsernameMap[claim.RedeemerID]
		if !exists {
			redeemerUsername = "Unknown"
		}

		given[i] = RewardClaimGiven{
			ID:               claim.ID,
			RewardID:         claim.RewardID,
			RewardTitle:      claim.RewardTitle,
			RedeemerID:       claim.RedeemerID,
			GiverID:          claim.GiverID,
			Price:            claim.Price,
			Status:           claim.Status,
			RedeemedAt:       claim.RedeemedAt,
			FulfilledAt:      claim.FulfilledAt,
			ResolvedAt:       claim.ResolvedAt,
			RedeemerUsername: redeemerUsername,
		}
	}

	return given, nil
}

// GetClaimsGiven returns claims against callerID's rewards, optionally narrowed to a
// single reward (in which case callerID must be that reward's giver).
func (s *service) GetClaimsGiven(callerID uuid.UUID, rewardID *uuid.UUID, before *time.Time, limit int) ([]RewardClaimGiven, error) {
	if rewardID != nil {
		reward, err := s.repo.FindByID(*rewardID)
		if err != nil {
			return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Reward not found"}
		}
		if reward.RewardGiverID != callerID {
			return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You do not have access to this reward's claims"}
		}
	}

	limit = clampClaimsLimit(limit)

	claims, err := s.repo.FindClaimsGiven(callerID, rewardID, before, limit)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch claims"}
	}

	return s.enrichClaimsWithRedeemerUsernames(claims)
}

func (s *service) MarkClaimFulfilled(claimID uuid.UUID, callerID uuid.UUID) (*models.RewardClaim, error) {
	claim, err := s.repo.FindClaimByID(claimID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Claim not found"}
	}

	if claim.RedeemerID != callerID {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "Only the redeemer can mark the claim as fulfilled"}
	}

	now := time.Now()
	fulfilled := false
	if err := s.repo.Transact(func(tx *gorm.DB) error {
		ok, err := s.repo.FulfillClaimTx(tx, claimID, now)
		fulfilled = ok
		return err
	}); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to update claim status"}
	}
	if !fulfilled {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Claim is not in a pending state"}
	}

	claim.Status = models.ClaimStatusFulfilled
	claim.FulfilledAt = &now

	return claim, nil
}

func (s *service) MarkClaimRefundRequested(claimID uuid.UUID, callerID uuid.UUID, reason string) (*models.RewardClaim, error) {
	claim, err := s.repo.FindClaimByID(claimID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Claim not found"}
	}

	if claim.RedeemerID != callerID {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "Only the redeemer can request a refund"}
	}

	requested := false
	if err := s.repo.Transact(func(tx *gorm.DB) error {
		ok, err := s.repo.RequestRefundTx(tx, claimID, reason)
		requested = ok
		return err
	}); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to update claim status"}
	}
	if !requested {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Claim is not in a pending state"}
	}

	claim.Status = models.ClaimStatusRefundRequested
	claim.RefundReason = &reason

	return claim, nil
}

func (s *service) MarkClaimRefunded(claimID uuid.UUID, callerID uuid.UUID) (*models.RewardClaim, error) {
	claim, err := s.repo.FindClaimByID(claimID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Claim not found"}
	}

	if claim.GiverID != callerID {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "Only the giver can mark the claim as refunded"}
	}

	now := time.Now()
	refunded := false
	if err := s.repo.Transact(func(tx *gorm.DB) error {
		ok, err := s.repo.RefundClaimTx(tx, claimID, now)
		refunded = ok
		if err != nil || !ok {
			return err
		}
		return s.transactionService.RecordEarnedTx(tx, claim.RedeemerID, claim.GiverID, claim.Price, claim.ID, models.ReferenceClaim)
	}); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to process refund"}
	}
	if !refunded {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Claim is not in a refund requested state"}
	}

	claim.Status = models.ClaimStatusRefunded
	claim.ResolvedAt = &now

	return claim, nil
}

// GetClaimsRedeemed returns claims redeemerID has made, optionally narrowed to claims
// against a single giver's rewards.
func (s *service) GetClaimsRedeemed(redeemerID uuid.UUID, giverID *uuid.UUID, before *time.Time, limit int) ([]RewardClaimRedeemed, error) {
	limit = clampClaimsLimit(limit)

	claims, err := s.repo.FindClaimsRedeemed(redeemerID, giverID, before, limit)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch claims"}
	}

	if len(claims) == 0 {
		return []RewardClaimRedeemed{}, nil
	}

	uniqueGiverIDs := make(map[uuid.UUID]struct{})
	var giverIDs []uuid.UUID

	for _, claim := range claims {
		if _, exists := uniqueGiverIDs[claim.GiverID]; !exists {
			uniqueGiverIDs[claim.GiverID] = struct{}{}
			giverIDs = append(giverIDs, claim.GiverID)
		}
	}

	users, err := s.userService.GetByIDs(giverIDs)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch user details"}
	}

	giverUsernameMap := make(map[uuid.UUID]string)
	for _, user := range users {
		giverUsernameMap[user.ID] = user.Username
	}

	history := make([]RewardClaimRedeemed, len(claims))
	for i, claim := range claims {
		giverUsername, exists := giverUsernameMap[claim.GiverID]
		if !exists {
			giverUsername = "Unknown"
		}

		history[i] = RewardClaimRedeemed{
			ID:			   claim.ID,
			RewardID:      claim.RewardID,
			RewardTitle:   claim.RewardTitle,
			RedeemerID:    claim.RedeemerID,
			GiverID:       claim.GiverID,
			Price:         claim.Price,
			Status:        claim.Status,
			RedeemedAt:    claim.RedeemedAt,
			FulfilledAt:   claim.FulfilledAt,
			ResolvedAt:    claim.ResolvedAt,
			GiverUsername: giverUsername,
		}
	}

	return history, nil
}
