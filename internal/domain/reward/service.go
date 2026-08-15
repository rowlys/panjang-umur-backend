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
	Cancel(ctx context.Context, rewardID uuid.UUID, callerID uuid.UUID) (*models.Reward, error)
	GetShopByGiver(callerID uuid.UUID, giverID uuid.UUID, availableOnly bool) ([]models.Reward, error)
	GetByGiver(giverID uuid.UUID) ([]models.Reward, error)

	MarkClaimFulfilled(claimID uuid.UUID, callerID uuid.UUID) (*models.RewardClaim, error)
	MarkClaimRefundRequested(claimID uuid.UUID, callerID uuid.UUID, reason string) (*models.RewardClaim, error)
	MarkClaimRefunded(claimID uuid.UUID, callerID uuid.UUID) (*models.RewardClaim, error)

	GetClaimByID(claimID uuid.UUID) (*models.RewardClaim, error)
	GetClaimsByRedeemer(redeemerID uuid.UUID) ([]models.RewardClaim, error)
	GetClaimsByGiver(giverID uuid.UUID) ([]models.RewardClaim, error)
}

type service struct {
	repo               Repository
	friendService      FriendService
	transactionService TransactionService
}

func NewService(repo Repository, friendService FriendService, transactionService TransactionService) Service {
	return &service{repo: repo, friendService: friendService, transactionService: transactionService}
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
		// IsAvailable:   true,
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
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Reward has already been redeemed"}
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

func (s *service) Cancel(ctx context.Context, rewardID uuid.UUID, callerID uuid.UUID) (*models.Reward, error) {
	reward, err := s.repo.FindByID(rewardID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "Reward not found"}
	}

	if reward.RewardGiverID != callerID {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "Only the reward giver can cancel it"}
	}

	if reward.Stock <= 0 {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Reward has already been redeemed"}
	}

	reward.Stock = 0
	if err := s.repo.Save(reward); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to cancel reward"}
	}

	return reward, nil
}

func (s *service) GetShopByGiver(callerID uuid.UUID, giverID uuid.UUID, availableOnly bool) ([]models.Reward, error) {
	isFriend, err := s.friendService.IsFriend(callerID, giverID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to verify friendship"}
	}
	if !isFriend {
		return nil, &httputil.ServiceError{Code: http.StatusForbidden, Message: "You are not friends with this user"}
	}
	rewards, err := s.repo.FindRedeemableByUser(callerID, giverID, availableOnly)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch rewards"}
	}
	return rewards, nil
}

func (s *service) GetByGiver(giverID uuid.UUID) ([]models.Reward, error) {
	rewards, err := s.repo.FindByGiver(giverID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch rewards"}
	}
	return rewards, nil
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

	if err := s.repo.UpdateClaimTx(nil, claim); err != nil {
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
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Claim is not in a fulfilled state"}
	}

	claim.Status = models.ClaimStatusRefundRequested
	claim.RefundReason = &reason

	if err := s.repo.UpdateClaimTx(nil, claim); err != nil {
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
		if err := s.repo.UpdateClaimTx(tx, claim); err != nil {
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

func (s *service) GetClaimsByRedeemer(redeemerID uuid.UUID) ([]models.RewardClaim, error) {
	claims, err := s.repo.FindClaimsByRedeemer(redeemerID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch claims"}
	}
	return claims, nil
}

func (s *service) GetClaimsByGiver(giverID uuid.UUID) ([]models.RewardClaim, error) {
	claims, err := s.repo.FindClaimsByGiver(giverID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to fetch claims"}
	}
	return claims, nil
}
