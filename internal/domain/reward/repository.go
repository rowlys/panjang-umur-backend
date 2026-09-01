package reward

import (
	"time"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)

type Repository interface {
	Create(reward *models.Reward) error
	CreateTx(tx *gorm.DB, reward *models.Reward) error
	Save(reward *models.Reward) error
	SaveTx(db *gorm.DB, reward *models.Reward) error
	Transact(fn func(*gorm.DB) error) error
	FindByID(id uuid.UUID) (*models.Reward, error)
	FindByIDs(ids []uuid.UUID) ([]models.Reward, error)
	FindByGiver(giverID uuid.UUID) ([]models.Reward, error)
	FindShopByGiver(userID, giverID uuid.UUID, availableOnly bool) ([]models.Reward, error)
	AddVisibilityTx(tx *gorm.DB, rewardID uuid.UUID, userIDs []uuid.UUID) error
	IsVisibleTo(rewardID, userID uuid.UUID) (bool, error)
	
	CreateClaimTx(tx *gorm.DB, claim *models.RewardClaim) error
	SaveClaim(claim *models.RewardClaim) error
	SaveClaimTx(tx *gorm.DB, claim *models.RewardClaim) error
	FindClaimByID(id uuid.UUID) (*models.RewardClaim, error)
	FindClaimsByIDs(ids []uuid.UUID) ([]models.RewardClaim, error)
	// FindClaimsRedeemed returns claims made by redeemerID (their own redemption history),
	// optionally narrowed to claims against a single giver's rewards.
	FindClaimsRedeemed(redeemerID uuid.UUID, giverID *uuid.UUID, before *time.Time, limit int) ([]models.RewardClaim, error)
	FindClaimsByGiver(giverID uuid.UUID) ([]models.RewardClaim, error)
	// FindClaimsGiven returns claims made against giverID's rewards,
	// optionally narrowed to a single reward.
	FindClaimsGiven(giverID uuid.UUID, rewardID *uuid.UUID, before *time.Time, limit int) ([]models.RewardClaim, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(reward *models.Reward) error {
	return r.db.Create(reward).Error
}

func (r *repository) CreateTx(tx *gorm.DB, reward *models.Reward) error {
	return tx.Create(reward).Error
}

func (r *repository) Save(reward *models.Reward) error {
	return r.db.Save(reward).Error
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

func (r *repository) FindByGiver(giverID uuid.UUID) ([]models.Reward, error) {
	var rewards []models.Reward
	err := r.db.Where("reward_giver_id = ?", giverID).Find(&rewards).Error
	return rewards, err
}

func (r *repository) FindByIDs(ids []uuid.UUID) ([]models.Reward, error) {
	var rewards []models.Reward
	if len(ids) == 0 {
		return rewards, nil
	}
	err := r.db.Where("id IN ?", ids).Find(&rewards).Error
	return rewards, err
}

func (r *repository) AddVisibilityTx(tx *gorm.DB, rewardID uuid.UUID, userIDs []uuid.UUID) error {
	if len(userIDs) == 0 {
		return nil
	}
	visibilities := make([]models.RewardVisibility, len(userIDs))
	for i, userID := range userIDs {
		visibilities[i] = models.RewardVisibility{RewardID: rewardID, UserID: userID}
	}
	return tx.Create(&visibilities).Error
}

func (r *repository) IsVisibleTo(rewardID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&models.RewardVisibility{}).Where("reward_id = ? AND user_id = ?", rewardID, userID).Count(&count).Error
	return count > 0, err
}

func (r *repository) CreateClaimTx(tx *gorm.DB, claim *models.RewardClaim) error {
	return tx.Create(claim).Error
}

func (r *repository) SaveClaim(claim *models.RewardClaim) error {
	return r.db.Save(claim).Error
}

func (r *repository) SaveClaimTx(tx *gorm.DB, claim *models.RewardClaim) error {
	return tx.Save(claim).Error
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

func (r *repository) FindClaimsByGiver(giverID uuid.UUID) ([]models.RewardClaim, error) {
	var claims []models.RewardClaim
	err := r.db.Where("giver_id = ?", giverID).Find(&claims).Error
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
