package challenge

import (
	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)


type Repository interface {
	Create(challenge *models.Challenge) error
	CreateTx(tx *gorm.DB, challenge *models.Challenge) error
	Save(challenge *models.Challenge) error
	SaveTx(tx *gorm.DB, challenge *models.Challenge) error
	Transact(fn func(tx *gorm.DB) error) error
	FindAll(statuses []models.ChallengeStatus) ([]models.Challenge, error)
	FindVisibleToUser(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	FindByID(id uuid.UUID) (*models.Challenge, error)
	FindByCreator(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	FindByAssignee(userID uuid.UUID, statuses []models.AssignmentStatus) ([]models.Challenge, error)

	CreateAssignment(a *models.ChallengeAssignment) error
	CreateAssignmentTx(tx *gorm.DB, a *models.ChallengeAssignment) error
	SaveAssignment(a *models.ChallengeAssignment) error
	SaveAssignmentTx(tx *gorm.DB, a *models.ChallengeAssignment) error
	FindAssignment(challengeID, assigneeID uuid.UUID) (*models.ChallengeAssignment, error)
	FindAssignmentByID(id uuid.UUID) (*models.ChallengeAssignment, error)
	FindAssignmentsByChallenge(challengeID uuid.UUID) ([]models.ChallengeAssignment, error)
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

func (r *repository) CreateTx(tx *gorm.DB, challenge *models.Challenge) error {
	return tx.Create(challenge).Error
}

func (r *repository) Save(challenge *models.Challenge) error {
	return r.db.Save(challenge).Error
}

func (r *repository) SaveTx(tx *gorm.DB, challenge *models.Challenge) error {
	return tx.Save(challenge).Error
}

func (r *repository) Transact(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

func (r *repository) FindAll(statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	var challenges []models.Challenge
	err := r.db.Where("status IN ? AND (expires_at IS NULL OR expires_at > NOW() OR status != ?)", statuses, models.StatusActive).Find(&challenges).Error
	return challenges, err
}

func (r *repository) FindVisibleToUser(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	var challenges []models.Challenge
	err := r.db.Where(
		`(
			creator_id = ?
			OR EXISTS (SELECT 1 FROM challenge_assignments WHERE challenge_assignments.challenge_id = challenges.id AND challenge_assignments.assignee_id = ?)
			OR (
				NOT restricted
				AND EXISTS (
					SELECT 1 FROM friendships
					WHERE friendships.status = ?
					AND (
						(friendships.requester_id = challenges.creator_id AND friendships.addressee_id = ?)
						OR (friendships.requester_id = ? AND friendships.addressee_id = challenges.creator_id)
					)
				)
			)
		)
		AND status IN ? AND (expires_at IS NULL OR expires_at > NOW() OR status != ?)`,
		userID, userID, models.FriendshipAccepted, userID, userID, statuses, models.StatusActive,
	).Find(&challenges).Error
	return challenges, err
}

func (r *repository) FindByID(id uuid.UUID) (*models.Challenge, error) {
	var challenge models.Challenge
	err := r.db.Where("id = ?", id).First(&challenge).Error
	return &challenge, err
}

func (r *repository) FindByCreator(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error) {
	var challenges []models.Challenge
	err := r.db.Where("creator_id = ? AND status IN ? AND (expires_at IS NULL OR expires_at > NOW() OR status != ?)", userID, statuses, models.StatusActive).Find(&challenges).Error
	return challenges, err
}

func (r *repository) FindByAssignee(userID uuid.UUID, statuses []models.AssignmentStatus) ([]models.Challenge, error) {
	var challenges []models.Challenge
	err := r.db.Joins("JOIN challenge_assignments ON challenge_assignments.challenge_id = challenges.id").
		Where("challenge_assignments.assignee_id = ? AND challenge_assignments.status IN ?", userID, statuses).
		Find(&challenges).Error
	return challenges, err
}

func (r *repository) CreateAssignment(a *models.ChallengeAssignment) error {
	return r.db.Create(a).Error
}

func (r *repository) CreateAssignmentTx(tx *gorm.DB, a *models.ChallengeAssignment) error {
	return tx.Create(a).Error
}

func (r *repository) SaveAssignment(a *models.ChallengeAssignment) error {
	return r.db.Save(a).Error
}

func (r *repository) SaveAssignmentTx(tx *gorm.DB, a *models.ChallengeAssignment) error {
	return tx.Save(a).Error
}

func (r *repository) FindAssignment(challengeID, assigneeID uuid.UUID) (*models.ChallengeAssignment, error) {
	var a models.ChallengeAssignment
	err := r.db.Where("challenge_id = ? AND assignee_id = ?", challengeID, assigneeID).First(&a).Error
	return &a, err
}

func (r *repository) FindAssignmentByID(id uuid.UUID) (*models.ChallengeAssignment, error) {
	var a models.ChallengeAssignment
	err := r.db.Where("id = ?", id).First(&a).Error
	return &a, err
}

func (r *repository) FindAssignmentsByChallenge(challengeID uuid.UUID) ([]models.ChallengeAssignment, error) {
	var assignments []models.ChallengeAssignment
	err := r.db.Where("challenge_id = ?", challengeID).Find(&assignments).Error
	return assignments, err
}
