package friendship

import (
	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)

type Repository interface {
	Create(f *models.Friendship) error
	FindByID(id uuid.UUID) (*models.Friendship, error)
	FindBetween(userA, userB uuid.UUID) (*models.Friendship, error)
	UpdateStatus(id uuid.UUID, status models.FriendshipStatus) error
	Delete(id uuid.UUID) error
	AreFriends(userA, userB uuid.UUID) (bool, error)
	ListFriends(userID uuid.UUID) ([]models.User, error)
	ListIncoming(userID uuid.UUID) ([]models.Friendship, error)
	ListOutgoing(userID uuid.UUID) ([]models.Friendship, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(f *models.Friendship) error {
	return r.db.Create(f).Error
}

func (r *repository) FindByID(id uuid.UUID) (*models.Friendship, error) {
	var f models.Friendship
	err := r.db.Where("id = ?", id).First(&f).Error
	return &f, err
}

func (r *repository) FindBetween(userA, userB uuid.UUID) (*models.Friendship, error) {
	var f models.Friendship
	err := r.db.Where(
		"(requester_id = ? AND addressee_id = ?) OR (requester_id = ? AND addressee_id = ?)",
		userA, userB, userB, userA,
	).First(&f).Error
	return &f, err
}

func (r *repository) UpdateStatus(id uuid.UUID, status models.FriendshipStatus) error {
	return r.db.Model(&models.Friendship{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       status,
		"responded_at": gorm.Expr("NOW()"),
	}).Error
}

func (r *repository) Delete(id uuid.UUID) error {
	return r.db.Where("id = ?", id).Delete(&models.Friendship{}).Error
}

func (r *repository) AreFriends(userA, userB uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&models.Friendship{}).Where(
		"status = ? AND ((requester_id = ? AND addressee_id = ?) OR (requester_id = ? AND addressee_id = ?))",
		models.FriendshipAccepted, userA, userB, userB, userA,
	).Count(&count).Error
	return count > 0, err
}

func (r *repository) ListFriends(userID uuid.UUID) ([]models.User, error) {
	var users []models.User
	err := r.db.Joins(
		"JOIN friendships ON (friendships.requester_id = ? AND friendships.addressee_id = users.id) OR (friendships.addressee_id = ? AND friendships.requester_id = users.id)",
		userID, userID,
	).Where("friendships.status = ?", models.FriendshipAccepted).Find(&users).Error
	return users, err
}

func (r *repository) ListIncoming(userID uuid.UUID) ([]models.Friendship, error) {
	var requests []models.Friendship
	err := r.db.Where("addressee_id = ? AND status = ?", userID, models.FriendshipPending).Find(&requests).Error
	return requests, err
}

func (r *repository) ListOutgoing(userID uuid.UUID) ([]models.Friendship, error) {
	var requests []models.Friendship
	err := r.db.Where("requester_id = ? AND status = ?", userID, models.FriendshipPending).Find(&requests).Error
	return requests, err
}
