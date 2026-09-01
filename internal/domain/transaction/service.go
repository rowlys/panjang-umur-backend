package transaction

import (
	"time"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
)

type Service interface {
	RecordEarnedTx(db *gorm.DB, ownerID, giverID uuid.UUID, amount int, referenceID uuid.UUID, referenceType models.TransactionReferenceType) error
	RecordSpentTx(db *gorm.DB, ownerID, giverID uuid.UUID, amount int, referenceID uuid.UUID, referenceType models.TransactionReferenceType) error
	GetBalance(ownerID, giverID uuid.UUID) (int, error)
	GetBalanceTx(db *gorm.DB, ownerID, giverID uuid.UUID) (int, error)
	GetAllBalances(ownerID uuid.UUID) ([]models.UserPointBalance, error)
	// GetHistory returns every transaction userID took part in, as either the
	// earner/spender or the counterparty (giver), most recent first,
	// optionally narrowed to a single reference type.
	GetHistory(userID uuid.UUID, referenceType *models.TransactionReferenceType, before *time.Time, limit int) ([]models.Transaction, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) RecordEarnedTx(db *gorm.DB, ownerID, giverID uuid.UUID, amount int, referenceID uuid.UUID, referenceType models.TransactionReferenceType) error {
	t := &models.Transaction{
		ID:            uuid.New(),
		UserID:        ownerID,
		GiverID:       giverID,
		Amount:        amount,
		Type:          models.Earned,
		ReferenceID:   referenceID,
		ReferenceType: referenceType,
	}
	if err := s.repo.CreateTx(db, t); err != nil {
		return err
	}
	return s.repo.UpsertBalanceTx(db, ownerID, giverID, amount)
}

func (s *service) RecordSpentTx(db *gorm.DB, ownerID, giverID uuid.UUID, amount int, referenceID uuid.UUID, referenceType models.TransactionReferenceType) error {
	t := &models.Transaction{
		ID:            uuid.New(),
		UserID:        ownerID,
		GiverID:       giverID,
		Amount:        amount,
		Type:          models.Spent,
		ReferenceID:   referenceID,
		ReferenceType: referenceType,
	}
	if err := s.repo.CreateTx(db, t); err != nil {
		return err
	}
	return s.repo.UpsertBalanceTx(db, ownerID, giverID, -amount)
}

func (s *service) GetBalance(ownerID, giverID uuid.UUID) (int, error) {
	return s.repo.GetBalance(ownerID, giverID)
}

func (s *service) GetBalanceTx(db *gorm.DB, ownerID, giverID uuid.UUID) (int, error) {
	return s.repo.GetBalanceTx(db, ownerID, giverID)
}

func (s *service) GetAllBalances(ownerID uuid.UUID) ([]models.UserPointBalance, error) {
	return s.repo.FindBalancesByOwner(ownerID)
}

func clampHistoryLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func (s *service) GetHistory(userID uuid.UUID, referenceType *models.TransactionReferenceType, before *time.Time, limit int) ([]models.Transaction, error) {
	return s.repo.FindByParticipant(userID, referenceType, before, clampHistoryLimit(limit))
}
