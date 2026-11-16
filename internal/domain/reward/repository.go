package reward

import (
	"time"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	CreateTx(tx *gorm.DB, reward *models.Reward) error
	SaveTx(db *gorm.DB, reward *models.Reward) error
	Transact(fn func(*gorm.DB) error) error
	FindByID(id uuid.UUID) (*models.Reward, error)
	// FindByIDForUpdate reads a reward while holding a row lock, so the caller
	// can change stock and snapshot its terms without another redeem racing in.
	FindByIDForUpdate(tx *gorm.DB, id uuid.UUID) (*models.Reward, error)
	FindShopByGiver(userID, giverID uuid.UUID, availableOnly bool) ([]models.Reward, error)
	// SyncVisibilityTx makes userIDs the complete recipient list of a restricted
	// reward. Passing no users clears the list.
	SyncVisibilityTx(tx *gorm.DB, rewardID uuid.UUID, userIDs []uuid.UUID) error
	IsVisibleTo(rewardID, userID uuid.UUID) (bool, error)
	FindVisibilityByRewardIDs(rewardIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)

	CreateClaimTx(tx *gorm.DB, claim *models.RewardClaim) error
	FindClaimByID(id uuid.UUID) (*models.RewardClaim, error)
	FindClaimsByIDs(ids []uuid.UUID) ([]models.RewardClaim, error)
	// FindClaimsRedeemed returns claims made by redeemerID (their own redemption history),
	// optionally narrowed to claims against a single giver's rewards.
	FindClaimsRedeemed(redeemerID uuid.UUID, giverID *uuid.UUID, before *time.Time, limit int) ([]models.RewardClaim, error)
	// FindClaimsGiven returns claims made against giverID's rewards,
	// optionally narrowed to a single reward.
	FindClaimsGiven(giverID uuid.UUID, rewardID *uuid.UUID, before *time.Time, limit int) ([]models.RewardClaim, error)
	// FulfillClaimTx, RequestRefundTx and RefundClaimTx only move a claim while it
	// is in the expected state, and report whether this call did the move.
	FulfillClaimTx(tx *gorm.DB, id uuid.UUID, at time.Time) (bool, error)
	RequestRefundTx(tx *gorm.DB, id uuid.UUID, reason string) (bool, error)
	RefundClaimTx(tx *gorm.DB, id uuid.UUID, at time.Time) (bool, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateTx(tx *gorm.DB, reward *models.Reward) error {
	return tx.Create(reward).Error
}

func (r *repository) SaveTx(db *gorm.DB, reward *models.Reward) error {
	return db.Save(reward).Error
}

func (r *repository) Transact(fn func(*gorm.DB) error) error {
	return r.db.Transaction(fn)
}

func (r *repository) FindByID(id uuid.UUID) (*models.Reward, error) {
	var reward models.Reward
	err := r.db.Where("id = ?", id).First(&reward).Error
	return &reward, err
}

func (r *repository) FindByIDForUpdate(tx *gorm.DB, id uuid.UUID) (*models.Reward, error) {
	var reward models.Reward
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).
		First(&reward).Error
	return &reward, err
}

func (r *repository) FindShopByGiver(userID, giverID uuid.UUID, availableOnly bool) ([]models.Reward, error) {
	var rewards []models.Reward

	q := r.db.Where("reward_giver_id = ?", giverID)

	if userID != giverID {
		q = q.Where(
			"visibility = ? OR EXISTS (SELECT 1 FROM reward_visibilities WHERE reward_visibilities.reward_id = rewards.id AND reward_visibilities.user_id = ?)",
			models.Public, userID,
		)
	}

	if availableOnly {
		q = q.Where("is_available = true")
	}

	err := q.Find(&rewards).Error

	return rewards, err
}

func (r *repository) SyncVisibilityTx(tx *gorm.DB, rewardID uuid.UUID, userIDs []uuid.UUID) error {
	if len(userIDs) == 0 {
		return tx.Where("reward_id = ?", rewardID).Delete(&models.RewardVisibility{}).Error
	}

	if err := tx.Where("reward_id = ? AND user_id NOT IN ?", rewardID, userIDs).
		Delete(&models.RewardVisibility{}).Error; err != nil {
		return err
	}

	visibilities := make([]models.RewardVisibility, len(userIDs))
	for i, userID := range userIDs {
		visibilities[i] = models.RewardVisibility{RewardID: rewardID, UserID: userID}
	}

	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&visibilities).Error
}

func (r *repository) IsVisibleTo(rewardID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&models.RewardVisibility{}).Where("reward_id = ? AND user_id = ?", rewardID, userID).Count(&count).Error
	return count > 0, err
}

func (r *repository) FindVisibilityByRewardIDs(rewardIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	result := make(map[uuid.UUID][]uuid.UUID, len(rewardIDs))
	if len(rewardIDs) == 0 {
		return result, nil
	}

	var rows []models.RewardVisibility
	if err := r.db.Where("reward_id IN ?", rewardIDs).Find(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		result[row.RewardID] = append(result[row.RewardID], row.UserID)
	}
	return result, nil
}

func (r *repository) CreateClaimTx(tx *gorm.DB, claim *models.RewardClaim) error {
	return tx.Create(claim).Error
}

func (r *repository) FindClaimByID(id uuid.UUID) (*models.RewardClaim, error) {
	var claim models.RewardClaim
	err := r.db.Where("id = ?", id).First(&claim).Error
	return &claim, err
}

func (r *repository) FindClaimsByIDs(ids []uuid.UUID) ([]models.RewardClaim, error) {
	var claims []models.RewardClaim
	if len(ids) == 0 {
		return claims, nil
	}
	err := r.db.Where("id IN ?", ids).Find(&claims).Error
	return claims, err
}

func (r *repository) FindClaimsRedeemed(redeemerID uuid.UUID, giverID *uuid.UUID, before *time.Time, limit int) ([]models.RewardClaim, error) {
	var claims []models.RewardClaim
	q := r.db.Where("redeemer_id = ?", redeemerID)
	if giverID != nil {
		q = q.Where("giver_id = ?", *giverID)
	}
	if before != nil {
		q = q.Where("redeemed_at < ?", *before)
	}
	err := q.Order("redeemed_at desc").Limit(limit).Find(&claims).Error
	return claims, err
}

func (r *repository) FindClaimsGiven(giverID uuid.UUID, rewardID *uuid.UUID, before *time.Time, limit int) ([]models.RewardClaim, error) {
	var claims []models.RewardClaim
	q := r.db.Where("giver_id = ?", giverID)
	if rewardID != nil {
		q = q.Where("reward_id = ?", *rewardID)
	}
	if before != nil {
		q = q.Where("redeemed_at < ?", *before)
	}
	err := q.Order("redeemed_at desc").Limit(limit).Find(&claims).Error
	return claims, err
}

func (r *repository) FulfillClaimTx(tx *gorm.DB, id uuid.UUID, at time.Time) (bool, error) {
	result := tx.Model(&models.RewardClaim{}).
		Where("id = ? AND status = ?", id, models.ClaimStatusPending).
		Updates(map[string]any{
			"status":       models.ClaimStatusFulfilled,
			"fulfilled_at": at,
		})
	return result.RowsAffected > 0, result.Error
}

func (r *repository) RequestRefundTx(tx *gorm.DB, id uuid.UUID, reason string) (bool, error) {
	result := tx.Model(&models.RewardClaim{}).
		Where("id = ? AND status = ?", id, models.ClaimStatusPending).
		Updates(map[string]any{
			"status":        models.ClaimStatusRefundRequested,
			"refund_reason": reason,
		})
	return result.RowsAffected > 0, result.Error
}

func (r *repository) RefundClaimTx(tx *gorm.DB, id uuid.UUID, at time.Time) (bool, error) {
	result := tx.Model(&models.RewardClaim{}).
		Where("id = ? AND status = ?", id, models.ClaimStatusRefundRequested).
		Updates(map[string]any{
			"status":      models.ClaimStatusRefunded,
			"resolved_at": at,
		})
	return result.RowsAffected > 0, result.Error
}
