package reward

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

type CreateRewardInput struct {
	Title          string                      `json:"title"`
	Description    string                      `json:"description"`
	Cost           int                         `json:"cost"`
	Visibility     models.RewardVisibilityMode `json:"visibility"`
	Stock          int                         `json:"stock"`
	AllowedUserIDs []uuid.UUID                 `json:"allowedUserIds"`
	GiverID        uuid.UUID                   `json:"giverId"`
}

type RewardClaimHistory struct {
	ID		   uuid.UUID
	RewardID   uuid.UUID
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
	RecordEarnedTx(db *gorm.DB, ownerID, giverID uuid.UUID, amount int, referenceID uuid.UUID) error
	RecordSpentTx(db *gorm.DB, ownerID, giverID uuid.UUID, amount int, referenceID uuid.UUID) error
	GetBalance(ownerID, giverID uuid.UUID) (int, error)
	GetBalanceTx(db *gorm.DB, ownerID, giverID uuid.UUID) (int, error)
}

type Service interface {
	Create(ctx context.Context, input CreateRewardInput) (*models.Reward, error)
	Redeem(ctx context.Context, rewardID uuid.UUID, redeemerID uuid.UUID) (*models.Reward, error)
	GetByID(rewardID uuid.UUID, callerID uuid.UUID) (*models.Reward, error)
	UpdateStock(ctx context.Context, rewardID uuid.UUID, callerID uuid.UUID, stock int) (*models.Reward, error)
	GetShopByGiver(callerID uuid.UUID, giverID uuid.UUID, availableOnly bool) ([]models.Reward, error)
	GetClaimsGivenByID(giverID uuid.UUID, before *time.Time, limit int) ([]RewardClaimGiven, error)
	GetClaimsForReward(rewardID uuid.UUID, callerID uuid.UUID, before *time.Time, limit int) ([]RewardClaimGiven, error)

	MarkClaimFulfilled(claimID uuid.UUID, callerID uuid.UUID) (*models.RewardClaim, error)
	MarkClaimRefundRequested(claimID uuid.UUID, callerID uuid.UUID, reason string) (*models.RewardClaim, error)
	MarkClaimRefunded(claimID uuid.UUID, callerID uuid.UUID) (*models.RewardClaim, error)

	GetClaimByID(claimID uuid.UUID) (*models.RewardClaim, error)
	GetClaimHistory(redeemerID uuid.UUID, before *time.Time, limit int) ([]RewardClaimHistory, error)
	GetClaimsByGiver(giverID uuid.UUID) ([]models.RewardClaim, error)
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
			return s.repo.AddVisibilityTx(tx, reward.ID, input.AllowedUserIDs)
		}
		return nil
	}); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to create reward"}
	}
	return reward, nil
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

	reward.Stock -= 1

	if err := s.repo.Transact(func(tx *gorm.DB) error {
		balance, err := s.transactionService.GetBalanceTx(tx, redeemerID, reward.RewardGiverID)
		if err != nil {
			return err
		}
		if balance < reward.Cost {
			return &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Insufficient points"}
		}

		if reward.Stock <= 0 {
			reward.IsAvailable = false
		}

		if err := s.repo.SaveTx(tx, reward); err != nil {
			return err
		}

		if err := s.repo.CreateClaimTx(tx, &models.RewardClaim{
			ID:         uuid.New(),
			RewardID:   reward.ID,
			RedeemerID: redeemerID,
			GiverID:    reward.RewardGiverID,
			Price:      reward.Cost,
			Status:     models.ClaimStatusPending,
		}); err != nil {
			return err
		}

		return s.transactionService.RecordSpentTx(tx, redeemerID, reward.RewardGiverID, reward.Cost, reward.ID)
	}); err != nil {
		var svcErr *httputil.ServiceError
		if errors.As(err, &svcErr) {
			return nil, svcErr
		}
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to redeem reward"}
	}

	return reward, nil
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

func (s *service) UpdateStock(ctx context.Context, rewardID uuid.UUID, callerID uuid.UUID, stock int) (*models.Reward, error) {
	if stock < 0 {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Stock cannot be negative"}
	}

	reward, err := s.repo.FindByID(rewardID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Reward not found"}
	}

	if reward.RewardGiverID != callerID {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "Only the reward giver can update its stock"}
	}

	reward.Stock = stock
	reward.IsAvailable = stock > 0

	if err := s.repo.Save(reward); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to update stock"}
	}

	return reward, nil
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

func (s *service) GetClaimsGivenByID(giverID uuid.UUID, before *time.Time, limit int) ([]RewardClaimGiven, error) {
	limit = clampClaimsLimit(limit)

	claims, err := s.repo.FindClaimsGivenByID(giverID, before, limit)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch claims"}
	}

	return s.enrichClaimsWithRedeemerUsernames(claims)
}

func (s *service) GetClaimsForReward(rewardID uuid.UUID, callerID uuid.UUID, before *time.Time, limit int) ([]RewardClaimGiven, error) {
	reward, err := s.repo.FindByID(rewardID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Reward not found"}
	}

	if reward.RewardGiverID != callerID {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You do not have access to this reward's claims"}
	}

	limit = clampClaimsLimit(limit)

	claims, err := s.repo.FindClaimsByReward(rewardID, before, limit)
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

	if claim.Status != models.ClaimStatusPending {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Claim is not in a pending state"}
	}

	now := time.Now()
	claim.Status = models.ClaimStatusFulfilled
	claim.FulfilledAt = &now

	if err := s.repo.SaveClaim(claim); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to update claim status"}
	}

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

	if claim.Status != models.ClaimStatusPending {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Claim is not in a pending state"}
	}

	claim.Status = models.ClaimStatusRefundRequested
	claim.RefundReason = &reason

	if err := s.repo.SaveClaim(claim); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to update claim status"}
	}

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

	if claim.Status != models.ClaimStatusRefundRequested {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Claim is not in a refund requested state"}
	}

	claim.Status = models.ClaimStatusRefunded
	now := time.Now()
	claim.ResolvedAt = &now

	if err := s.repo.Transact(func(tx *gorm.DB) error {
		if err := s.repo.SaveClaimTx(tx, claim); err != nil {
			return err
		}
		return s.transactionService.RecordEarnedTx(tx, claim.RedeemerID, claim.GiverID, claim.Price, claim.ID)
	}); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to process refund"}
	}

	return claim, nil
}

func (s *service) GetClaimByID(claimID uuid.UUID) (*models.RewardClaim, error) {
	claim, err := s.repo.FindClaimByID(claimID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Claim not found"}
	}
	return claim, nil
}

func (s *service) GetClaimHistory(redeemerID uuid.UUID, before *time.Time, limit int) ([]RewardClaimHistory, error) {
	limit = clampClaimsLimit(limit)

	claims, err := s.repo.FindClaimsByRedeemer(redeemerID, before, limit)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch claims"}
	}

	if len(claims) == 0 {
		return []RewardClaimHistory{}, nil
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

	history := make([]RewardClaimHistory, len(claims))
	for i, claim := range claims {
		giverUsername, exists := giverUsernameMap[claim.GiverID]
		if !exists {
			giverUsername = "Unknown"
		}

		history[i] = RewardClaimHistory{
			ID:			   claim.ID,
			RewardID:      claim.RewardID,
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

func (s *service) GetClaimsByGiver(giverID uuid.UUID) ([]models.RewardClaim, error) {
	claims, err := s.repo.FindClaimsByGiver(giverID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch claims"}
	}
	return claims, nil
}
