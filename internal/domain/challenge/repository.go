package challenge

import (
	"time"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)


type Repository interface {
	Save(challenge *models.Challenge) error
	Delete(id uuid.UUID) error

	CreateTx(tx *gorm.DB, challenge *models.Challenge) error
	SaveTx(tx *gorm.DB, challenge *models.Challenge) error
	Transact(fn func(tx *gorm.DB) error) error

	FindVisibleToUser(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	FindByID(id uuid.UUID) (*models.Challenge, error)
	FindByIDs(ids []uuid.UUID) ([]models.Challenge, error)
	FindByCreator(userID uuid.UUID, statuses []models.ChallengeStatus) ([]models.Challenge, error)
	FindChallengesInvolvingUser(userID uuid.UUID) ([]models.Challenge, error)

	CreateAssigneeTx(tx *gorm.DB, a *models.ChallengeAssignee) error
	FindAssignee(challengeID, userID uuid.UUID) (*models.ChallengeAssignee, error)
	FindAssigneesByChallenges(challengeIDs []uuid.UUID) ([]models.ChallengeAssignee, error)

	CreateSubmission(s *models.ChallengeSubmission) error
	ApproveSubmissionTx(tx *gorm.DB, id uuid.UUID, at time.Time) (bool, error)
	FindSubmissionForPeriod(challengeID, userID uuid.UUID, periodStart time.Time) (*models.ChallengeSubmission, error)
	FindSubmissionByID(id uuid.UUID) (*models.ChallengeSubmission, error)
	FindLatestSubmission(challengeID, userID uuid.UUID) (*models.ChallengeSubmission, error)
	// FindSubmissionsSubmitted returns submissions userID made, optionally narrowed to a single challenge.
	FindSubmissionsSubmitted(userID uuid.UUID, challengeID *uuid.UUID, statusFilter *models.SubmissionStatus, before *time.Time, limit int) ([]models.ChallengeSubmission, error)
	// FindSubmissionsReceived returns submissions made on challenges creatorID created,
	// optionally narrowed to a single challenge.
	FindSubmissionsReceived(creatorID uuid.UUID, challengeID *uuid.UUID, statusFilter *models.SubmissionStatus, before *time.Time, limit int) ([]models.ChallengeSubmission, error)
	FindSubmissionsByUserAndChallenges(userID uuid.UUID, challengeIDs []uuid.UUID) ([]models.ChallengeSubmission, error)
	FindSubmissionsByIDs(ids []uuid.UUID) ([]models.ChallengeSubmission, error)
	CountAssigneesWithoutApprovedSubmission(tx *gorm.DB, challengeID uuid.UUID) (int64, error)

	// FindProofImagesToPurge returns approved submissions that still hold a
	// stored proof image and were approved before cutoff, oldest first.
	FindProofImagesToPurge(cutoff time.Time, limit int) ([]models.ChallengeSubmission, error)
	// ClearProofImage forgets a submission's image once its file is gone.
	ClearProofImage(id uuid.UUID, imageID uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Save(challenge *models.Challenge) error {
	return r.db.Save(challenge).Error
}

// Delete is a soft delete (sets deleted_at); submissions and ledger rows are kept.
func (r *repository) Delete(id uuid.UUID) error {
	return r.db.Where("id = ?", id).Delete(&models.Challenge{}).Error
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

func (r *repository) FindByIDs(ids []uuid.UUID) ([]models.Challenge, error) {
	var challenges []models.Challenge
	if len(ids) == 0 {
		return challenges, nil
	}
	// Unscoped so transaction history still shows titles of deleted challenges.
	err := r.db.Unscoped().Where("id IN ?", ids).Find(&challenges).Error
	return challenges, err
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

func (r *repository) CreateAssigneeTx(tx *gorm.DB, a *models.ChallengeAssignee) error {
	return tx.Create(a).Error
}

func (r *repository) FindAssignee(challengeID, userID uuid.UUID) (*models.ChallengeAssignee, error) {
	var a models.ChallengeAssignee
	err := r.db.Where("challenge_id = ? AND user_id = ?", challengeID, userID).First(&a).Error
	return &a, err
}

func (r *repository) FindAssigneesByChallenges(challengeIDs []uuid.UUID) ([]models.ChallengeAssignee, error) {
	var assignees []models.ChallengeAssignee
	if len(challengeIDs) == 0 {
		return assignees, nil
	}
	err := r.db.Where("challenge_id IN ?", challengeIDs).Find(&assignees).Error
	return assignees, err
}

func (r *repository) CreateSubmission(s *models.ChallengeSubmission) error {
	return r.db.Create(s).Error
}

// ApproveSubmissionTx approves a submission only while it is still pending, so
// two concurrent approvals cannot both award points. It reports whether this
// call was the one that moved the row.
func (r *repository) ApproveSubmissionTx(tx *gorm.DB, id uuid.UUID, at time.Time) (bool, error) {
	result := tx.Model(&models.ChallengeSubmission{}).
		Where("id = ? AND status = ?", id, models.SubmissionSubmitted).
		Updates(map[string]any{
			"status":      models.SubmissionApproved,
			"approved_at": at,
		})
	return result.RowsAffected > 0, result.Error
}

// FindProofImagesToPurge returns approved submissions that still hold a stored
// proof image and were approved before cutoff, oldest first. Rows whose image
// was already cleared are skipped by the IS NOT NULL filter. Legacy approvals
// without approved_at fall back to submitted_at.
func (r *repository) FindProofImagesToPurge(cutoff time.Time, limit int) ([]models.ChallengeSubmission, error) {
	var submissions []models.ChallengeSubmission
	err := r.db.
		Select("id", "proof_image_id", "approved_at").
		Where("status = ? AND proof_image_id IS NOT NULL AND COALESCE(approved_at, submitted_at) < ?",
			models.SubmissionApproved, cutoff).
		Order("COALESCE(approved_at, submitted_at) ASC").
		Limit(limit).
		Find(&submissions).Error
	return submissions, err
}

// ClearProofImage forgets a submission's image. The image ID is part of the
// predicate so a concurrent purge cannot clear a newer reference.
func (r *repository) ClearProofImage(id uuid.UUID, imageID uuid.UUID) error {
	return r.db.Model(&models.ChallengeSubmission{}).
		Where("id = ? AND proof_image_id = ?", id, imageID).
		Update("proof_image_id", nil).Error
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

func (r *repository) FindSubmissionsSubmitted(userID uuid.UUID, challengeID *uuid.UUID, statusFilter *models.SubmissionStatus, before *time.Time, limit int) ([]models.ChallengeSubmission, error) {
	var submissions []models.ChallengeSubmission
	query := r.db.Model(&models.ChallengeSubmission{}).
		Select("challenge_submissions.*").
		Joins("JOIN challenges ON challenges.id = challenge_submissions.challenge_id AND challenges.deleted_at IS NULL").
		Where("challenge_submissions.user_id = ?", userID)
	if challengeID != nil {
		query = query.Where("challenge_submissions.challenge_id = ?", *challengeID)
	}
	if statusFilter != nil {
		query = query.Where("challenge_submissions.status = ?", *statusFilter)
	}
	if before != nil {
		query = query.Where("challenge_submissions.submitted_at < ?", *before)
	}
	err := query.Order("challenge_submissions.submitted_at desc").Limit(limit).Find(&submissions).Error
	return submissions, err
}

func (r *repository) FindSubmissionsReceived(creatorID uuid.UUID, challengeID *uuid.UUID, statusFilter *models.SubmissionStatus, before *time.Time, limit int) ([]models.ChallengeSubmission, error) {
	var submissions []models.ChallengeSubmission
	query := r.db.Model(&models.ChallengeSubmission{}).
		Select("challenge_submissions.*").
		Joins("JOIN challenges ON challenges.id = challenge_submissions.challenge_id AND challenges.deleted_at IS NULL").
		Where("challenges.creator_id = ?", creatorID).
		// Pending submissions on a challenge that is no longer active can never be approved, so hide them.
		Where("(challenges.status = ? OR challenge_submissions.status != ?)", models.StatusActive, models.SubmissionSubmitted)
	if challengeID != nil {
		query = query.Where("challenge_submissions.challenge_id = ?", *challengeID)
	}
	if statusFilter != nil {
		query = query.Where("challenge_submissions.status = ?", *statusFilter)
	}
	if before != nil {
		query = query.Where("challenge_submissions.submitted_at < ?", *before)
	}
	err := query.Order("challenge_submissions.submitted_at desc").Limit(limit).Find(&submissions).Error
	return submissions, err
}

func (r *repository) FindSubmissionsByUserAndChallenges(userID uuid.UUID, challengeIDs []uuid.UUID) ([]models.ChallengeSubmission, error) {
    var submissions []models.ChallengeSubmission
    err := r.db.Where("user_id = ? AND challenge_id IN ?", userID, challengeIDs).
        Find(&submissions).Error
    return submissions, err
}

func (r *repository) FindSubmissionsByIDs(ids []uuid.UUID) ([]models.ChallengeSubmission, error) {
	var submissions []models.ChallengeSubmission
	if len(ids) == 0 {
		return submissions, nil
	}
	err := r.db.Where("id IN ?", ids).Find(&submissions).Error
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