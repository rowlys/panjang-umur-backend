package transaction

import (
	"errors"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	CreateTx(db *gorm.DB, t *models.Transaction) error
	UpsertBalanceTx(db *gorm.DB, ownerID, giverID uuid.UUID, delta int) error
	GetBalance(ownerID, giverID uuid.UUID) (int, error)
	GetBalanceTx(db *gorm.DB, ownerID, giverID uuid.UUID) (int, error)
	FindBalancesByOwner(ownerID uuid.UUID) ([]models.UserPointBalance, error)
	FindByOwner(ownerID uuid.UUID) ([]models.Transaction, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateTx(db *gorm.DB, t *models.Transaction) error {
	return db.Create(t).Error
}

func (r *repository) UpsertBalanceTx(db *gorm.DB, ownerID, giverID uuid.UUID, delta int) error {
	return db.Exec(
		`INSERT INTO user_point_balances (owner_id, giver_id, balance)
		 VALUES (?, ?, ?)
		 ON CONFLICT (owner_id, giver_id)
		 DO UPDATE SET balance = user_point_balances.balance + ?`,
		ownerID, giverID, delta, delta,
	).Error
}

func (r *repository) GetBalance(ownerID, giverID uuid.UUID) (int, error) {
	var bal models.UserPointBalance
	err := r.db.Where("owner_id = ? AND giver_id = ?", ownerID, giverID).First(&bal).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return bal.Balance, err
}

func (r *repository) GetBalanceTx(db *gorm.DB, ownerID, giverID uuid.UUID) (int, error) {
	var bal models.UserPointBalance
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("owner_id = ? AND giver_id = ?", ownerID, giverID).
		First(&bal).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return bal.Balance, err
}

func (r *repository) FindBalancesByOwner(ownerID uuid.UUID) ([]models.UserPointBalance, error) {
	var balances []models.UserPointBalance
	err := r.db.Where("owner_id = ?", ownerID).Find(&balances).Error
	return balances, err
}

func (r *repository) FindByOwner(ownerID uuid.UUID) ([]models.Transaction, error) {
	var txs []models.Transaction
	err := r.db.Where("user_id = ?", ownerID).Order("timestamp desc").Find(&txs).Error
	return txs, err
}
