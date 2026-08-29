package challenge

import (
	"time"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)


type Repository interface {
	Create(challenge *models.Challenge) error
	Save(challenge *models.Challenge) error
	Delete(challenge *models.Challenge) error

	CreateTx(tx *gorm.DB, challenge *models.Challenge) error
	SaveTx(tx *gorm.DB, challenge *models.Challenge) error
	Transact(fn func(tx *gorm.DB) error) error

	FindAll(statuses []models.ChallengeStatus) ([]models.Challenge, error)
	FindVisibleToUser(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	FindByID(id uuid.UUID) (*models.Challenge, error)
	FindByCreator(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	FindChallengesInvolvingUser(userID uuid.UUID) ([]models.Challenge, error)

	CreateAssignee(a *models.ChallengeAssignee) error
	CreateAssigneeTx(tx *gorm.DB, a *models.ChallengeAssignee) error
	FindAssignee(challengeID, userID uuid.UUID) (*models.ChallengeAssignee, error)
	FindAssigneesByChallenge(challengeID uuid.UUID) ([]models.ChallengeAssignee, error)

	CreateSubmission(s *models.ChallengeSubmission) error
	CreateSubmissionTx(tx *gorm.DB, s *models.ChallengeSubmission) error
	SaveSubmission(s *models.ChallengeSubmission) error
	SaveSubmissionTx(tx *gorm.DB, s *models.ChallengeSubmission) error
	FindSubmissionForPeriod(challengeID, userID uuid.UUID, periodStart time.Time) (*models.ChallengeSubmission, error)
	FindSubmissionByID(id uuid.UUID) (*models.ChallengeSubmission, error)
	FindLatestSubmission(challengeID, userID uuid.UUID) (*models.ChallengeSubmission, error)
	FindSubmissionsByUser(userID uuid.UUID) ([]models.ChallengeSubmission, error)
	FindSubmissionsByChallenge(challengeID uuid.UUID, statusFilter *models.SubmissionStatus, before *time.Time, limit int) ([]models.ChallengeSubmission, error)
	FindSubmissionsByUserAndChallenges(userID uuid.UUID, challengeIDs []uuid.UUID) ([]models.ChallengeSubmission, error)
	CountAssigneesWithoutApprovedSubmission(tx *gorm.DB, challengeID uuid.UUID) (int64, error)
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

func (r *repository) Delete(challenge *models.Challenge) error {
	return r.db.Delete(challenge).Error
}

func (r *repository) CreateTx(tx *gorm.DB, challenge *models.Challenge) error {
	return tx.Create(challenge).Error
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
			OR EXISTS (SELECT 1 FROM challenge_assignees WHERE challenge_assignees.challenge_id = challenges.id AND challenge_assignees.user_id = ?)
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

func (r *repository) FindChallengesInvolvingUser(userID uuid.UUID) ([]models.Challenge, error) {
	var challenges []models.Challenge
	err := r.db.Where(
		`status = ? AND (
			EXISTS (SELECT 1 FROM challenge_assignees WHERE challenge_assignees.challenge_id = challenges.id AND challenge_assignees.user_id = ?)
			OR EXISTS (SELECT 1 FROM challenge_submissions WHERE challenge_submissions.challenge_id = challenges.id AND challenge_submissions.user_id = ?)
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
		)`,
		models.StatusActive, userID, userID, models.FriendshipAccepted, userID, userID,
	).Find(&challenges).Error
	return challenges, err
}

func (r *repository) CreateAssignee(a *models.ChallengeAssignee) error {
	return r.db.Create(a).Error
}

func (r *repository) CreateAssigneeTx(tx *gorm.DB, a *models.ChallengeAssignee) error {
	return tx.Create(a).Error
}

func (r *repository) FindAssignee(challengeID, userID uuid.UUID) (*models.ChallengeAssignee, error) {
	var a models.ChallengeAssignee
	err := r.db.Where("challenge_id = ? AND user_id = ?", challengeID, userID).First(&a).Error
	return &a, err
}

func (r *repository) FindAssigneesByChallenge(challengeID uuid.UUID) ([]models.ChallengeAssignee, error) {
	var assignees []models.ChallengeAssignee
	err := r.db.Where("challenge_id = ?", challengeID).Find(&assignees).Error
	return assignees, err
}

func (r *repository) CreateSubmission(s *models.ChallengeSubmission) error {
	return r.db.Create(s).Error
}

func (r *repository) CreateSubmissionTx(tx *gorm.DB, s *models.ChallengeSubmission) error {
	return tx.Create(s).Error
}

func (r *repository) SaveSubmission(s *models.ChallengeSubmission) error {
	return r.db.Save(s).Error
}

func (r *repository) SaveSubmissionTx(tx *gorm.DB, s *models.ChallengeSubmission) error {
	return tx.Save(s).Error
}

func (r *repository) FindSubmissionForPeriod(challengeID, userID uuid.UUID, periodStart time.Time) (*models.ChallengeSubmission, error) {
	var s models.ChallengeSubmission
	err := r.db.Where("challenge_id = ? AND user_id = ? AND period_start = ?", challengeID, userID, periodStart).First(&s).Error
	return &s, err
}

func (r *repository) FindSubmissionByID(id uuid.UUID) (*models.ChallengeSubmission, error) {
	var s models.ChallengeSubmission
	err := r.db.Where("id = ?", id).First(&s).Error
	return &s, err
}

func (r *repository) FindLatestSubmission(challengeID, userID uuid.UUID) (*models.ChallengeSubmission, error) {
	var s models.ChallengeSubmission
	err := r.db.Where("challenge_id = ? AND user_id = ?", challengeID, userID).
		Order("period_start DESC").First(&s).Error
	return &s, err
}

func (r *repository) FindSubmissionsByUser(userID uuid.UUID) ([]models.ChallengeSubmission, error) {
	var submissions []models.ChallengeSubmission
	err := r.db.Where("user_id = ?", userID).Find(&submissions).Error
	return submissions, err
}

func (r *repository) FindSubmissionsByChallenge(challengeID uuid.UUID, statusFilter *models.SubmissionStatus, before *time.Time, limit int) ([]models.ChallengeSubmission, error) {
	var submissions []models.ChallengeSubmission
	query := r.db.Where("challenge_id = ?", challengeID)
	if statusFilter != nil {
		query = query.Where("status = ?", *statusFilter)
	}
	if before != nil {
		query = query.Where("submitted_at < ?", *before)
	}
	err := query.Order("submitted_at desc").Limit(limit).Find(&submissions).Error
	return submissions, err
}

func (r *repository) FindSubmissionsByUserAndChallenges(userID uuid.UUID, challengeIDs []uuid.UUID) ([]models.ChallengeSubmission, error) {
    var submissions []models.ChallengeSubmission
    err := r.db.Where("user_id = ? AND challenge_id IN ?", userID, challengeIDs).
        Find(&submissions).Error
    return submissions, err
}

func (r *repository) CountAssigneesWithoutApprovedSubmission(tx *gorm.DB, challengeID uuid.UUID) (int64, error) {
    var missingCount int64
    err := tx.Model(&models.ChallengeAssignee{}).
        Joins(`LEFT JOIN challenge_submissions cs 
               ON challenge_assignees.challenge_id = cs.challenge_id 
               AND challenge_assignees.user_id = cs.user_id 
               AND cs.status = ?`, models.SubmissionApproved).
        Where("challenge_assignees.challenge_id = ?", challengeID).
        Where("cs.id IS NULL").
        Count(&missingCount).Error
        
    return missingCount, err
}