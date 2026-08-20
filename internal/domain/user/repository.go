package user

import (
	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)

type BareUserDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Name     string    `json:"name"`
}

type Repository interface {
	Create(user *models.User) error
	Save(user *models.User) error
	FindByID(id uuid.UUID) (*models.User, error)
	FindByUsername(username string) (*models.User, error)
	FindByIDs(ids []uuid.UUID) ([]*models.User, error)
	SearchByUsername(callerId uuid.UUID, prefix string, limit int) ([]BareUserDTO, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *repository) Save(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *repository) FindByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.Where("id = ?", id).First(&user).Error
	return &user, err
}

func (r *repository) FindByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.db.Where("username = ?", username).First(&user).Error

	return &user, err
}

func (r *repository) FindByIDs(ids []uuid.UUID) ([]*models.User, error) {
	var users []*models.User
	err := r.db.Where("id IN ?", ids).Find(&users).Error
	return users, err
}

func (r *repository) SearchByUsername(callerId uuid.UUID, prefix string, limit int) ([]BareUserDTO, error) {
	var results []BareUserDTO
	searchTerm := prefix + "%"

	err := r.db.Model(&models.User{}).
		Select("id, username, name").
		Where("username ILIKE ?", searchTerm).
		Limit(limit).
		Find(&results).Error

	return results, err
}
