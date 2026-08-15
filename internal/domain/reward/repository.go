package reward

import (
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
	FindByGiver(giverID uuid.UUID) ([]models.Reward, error)
	FindRedeemableByUser(userID, giverID uuid.UUID, availableOnly bool) ([]models.Reward, error)
	AddVisibilityTx(tx *gorm.DB, rewardID uuid.UUID, userIDs []uuid.UUID) error
	IsVisibleTo(rewardID, userID uuid.UUID) (bool, error)

	CreateClaimTx(tx *gorm.DB, claim *models.RewardClaim) error
	UpdateClaimTx(tx *gorm.DB, claim *models.RewardClaim) error
	FindClaimByID(id uuid.UUID) (*models.RewardClaim, error)
	FindClaimsByRedeemer(redeemerID uuid.UUID) ([]models.RewardClaim, error)
	FindClaimsByGiver(giverID uuid.UUID) ([]models.RewardClaim, error)
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

func (r *repository) FindRedeemableByUser(userID, giverID uuid.UUID, availableOnly bool) ([]models.Reward, error) {
	var rewards []models.Reward
	q := r.db.Where(
		`reward_giver_id = ? AND (
			visibility = ? OR EXISTS (SELECT 1 FROM reward_visibilities WHERE reward_visibilities.reward_id = rewards.id AND reward_visibilities.user_id = ?)
		)`,
		giverID, models.Public, userID,
	)
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

func (r *repository) UpdateClaimTx(tx *gorm.DB, claim *models.RewardClaim) error {
	return tx.Save(claim).Error
}

func (r *repository) FindClaimByID(id uuid.UUID) (*models.RewardClaim, error) {
	var claim models.RewardClaim
	err := r.db.Where("id = ?", id).First(&claim).Error
	return &claim, err
}

func (r *repository) FindClaimsByRedeemer(redeemerID uuid.UUID) ([]models.RewardClaim, error) {
	var claims []models.RewardClaim
	err := r.db.Where("redeemer_id = ?", redeemerID).Find(&claims).Error
	return claims, err
}

func (r *repository) FindClaimsByGiver(giverID uuid.UUID) ([]models.RewardClaim, error) {
	var claims []models.RewardClaim
	err := r.db.Where("giver_id = ?", giverID).Find(&claims).Error
	return claims, err
}
