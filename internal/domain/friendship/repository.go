package friendship

import (
	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)

type FriendDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Name     string    `json:"name"`
}

type Repository interface {
	Create(f *models.Friendship) error
	FindByID(id uuid.UUID) (*models.Friendship, error)
	FindBetween(userA, userB uuid.UUID) (*models.Friendship, error)
	UpdateStatus(id uuid.UUID, status models.FriendshipStatus) error
	Delete(id uuid.UUID) error
	AreFriends(userA, userB uuid.UUID) (bool, error)
	ListFriends(userID uuid.UUID) ([]FriendDTO, error)
	ListIncoming(userID uuid.UUID) ([]models.Friendship, error)
	ListOutgoing(userID uuid.UUID) ([]models.Friendship, error)
	GetBulkStatuses(callerID uuid.UUID, otherIDs []uuid.UUID) (map[uuid.UUID]int, error)
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

func (r *repository) ListFriends(userID uuid.UUID) ([]FriendDTO, error) {
	var users []FriendDTO

    q1 := r.db.Model(&models.User{}).
		Select("users.id, users.username, users.name").
        Joins("INNER JOIN friendships ON friendships.addressee_id = users.id").
        Where("friendships.requester_id = ?", userID).
        Where("friendships.status = ?", models.FriendshipAccepted)

    q2 := r.db.Model(&models.User{}).
		Select("users.id, users.username, users.name").
        Joins("INNER JOIN friendships ON friendships.requester_id = users.id").
        Where("friendships.addressee_id = ?", userID).
        Where("friendships.status = ?", models.FriendshipAccepted)

    err := r.db.Raw("? UNION ?", q1, q2).Scan(&users).Error



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

func (r *repository) GetBulkStatuses(callerID uuid.UUID, otherIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	if len(otherIDs) == 0 {
		return make(map[uuid.UUID]int), nil
	}

	var records []struct {
		RequesterID uuid.UUID
		AddresseeID uuid.UUID
		Status  int
	}
	

	err := r.db.Table("friendships").
		Where("(requester_id = ? AND addressee_id IN ?) OR (requester_id IN ? AND addressee_id = ?)", callerID, otherIDs, otherIDs, callerID).
		Find(&records).Error
	
	if err != nil {
		return nil, err
	}

	statusMap := make(map[uuid.UUID]int)
	for _, record := range records {
		if record.RequesterID == callerID {
			statusMap[record.AddresseeID] = record.Status
		} else {
			statusMap[record.RequesterID] = record.Status
		}

	}

	return statusMap, nil
}
