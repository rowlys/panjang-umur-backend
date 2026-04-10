package challenge

import (
	"github.com/google/uuid"
	// "github.com/rowlys/panjang-umur-backend/internal/database"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)


type Repository interface {
	Create(challenge *models.Challenge) error
	Save(challenge *models.Challenge) error	
	FindAll(statuses []models.ChallengeStatus) ([]models.Challenge, error)
	FindByID(id uuid.UUID) (*models.Challenge, error)
	FindByAssignee(userIDD uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	FindByAssigneeAndGroup(userID uuid.UUID, groupID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	FindByCreator(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	FindByCreatorAndGroup(userID uuid.UUID, groupID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(challenge *models.Challenge) error {
	return r.db.Create(challenge).Error
}

func (r *repository) Save(challenge *models.Challenge) error {
	return r.db.Save(challenge).Error
}

func (r *repository) FindAll(statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	var challenges []models.Challenge
	err := r.db.Where("status IN ?", statuses).Find(&challenges).Error
	return challenges, err
}

func (r *repository) FindByID(id uuid.UUID) (*models.Challenge, error) {
	var challenge models.Challenge
	err := r.db.Where("id = ?", id).First(&challenge).Error
	return &challenge, err
}

func (r *repository) FindByAssignee(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	var challenges []models.Challenge
	err := r.db.Where("assignee_id = ? AND status IN ?", userID, statuses).Find(&challenges).Error
	return challenges, err
}

func (r *repository) FindByAssigneeAndGroup(userID uuid.UUID, groupID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	var challenges []models.Challenge
	err := r.db.Where("assignee_id = ? AND group_id = ? AND status IN ?", userID, groupID, statuses).
		Find(&challenges).Error
	return challenges, err
}

func (r *repository) FindByCreator(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	var challenges []models.Challenge
	err := r.db.Where("creator_id = ? AND status IN ?", userID, statuses).Find(&challenges).Error
	return challenges, err
}

func (r *repository) FindByCreatorAndGroup(userID uuid.UUID, groupID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	var challenges []models.Challenge
	err := r.db.Where("creator_id = ? AND group_id = ? AND status IN ?", userID, groupID, statuses).
		Find(&challenges).Error
	return challenges, err
}

