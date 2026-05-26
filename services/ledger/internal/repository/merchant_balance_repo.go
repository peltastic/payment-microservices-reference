package repository

import (
	"context"
	"time"

	"github.com/peltastic/payment-microservices-reference/ledger/internal/db"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/domain"
	"gorm.io/gorm"
)

type MerchantBalanceRepository struct {
	db db.IDatabase
}

func NewMerchantBalanceRepository(db db.IDatabase) *MerchantBalanceRepository {
	return &MerchantBalanceRepository{
		db: db,
	}
}

func (r *MerchantBalanceRepository) WithTx(tx *gorm.DB) domain.MerchantBalanceRepository {
	return &MerchantBalanceRepository{
		db: r.db.WithTx(tx),
	}
}

func (r *MerchantBalanceRepository) IncrementPendingBalance(ctx context.Context, merchantID string, amount int64) error {
	query := `
		INSERT INTO merchant_balances (merchant_id, pending, currency, updated_at)
		VALUES (?, ?, 'NGN', ?)
		ON CONFLICT (merchant_id) DO UPDATE
			SET pending    = merchant_balances.pending + ?,
			    updated_at = ?
	`

	now := time.Now()

	return r.db.GetDB().
		WithContext(ctx).
		Exec(query, merchantID, amount, now, amount, now).
		Error
}

func (r *MerchantBalanceRepository) GetMaterialisedBalance(ctx context.Context, merchantID string) (*domain.MerchantBalance, error) {
	var b domain.MerchantBalance

	if err := r.db.GetDB().
		WithContext(ctx).
		Where("merchant_id = ?", merchantID).
		First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *MerchantBalanceRepository) ComputeBalanceFromJournal(ctx context.Context, merchantID string) (*domain.MerchantBalance, error) {
	var available int64
	result := r.db.GetDB().WithContext(ctx).Raw(`
        SELECT
            COALESCE(
                SUM(CASE WHEN entry_type = 'credit' THEN amount ELSE -amount END),
                0
            ) as balance
        FROM journals
        WHERE merchant_id = ?
    `, merchantID).Scan(&available)

	if result.Error != nil {
		return nil, result.Error
	}

	return &domain.MerchantBalance{
		MerchantID: merchantID,
		Available:  available,
		Pending:    0,
		Currency:   "NGN",
		UpdatedAt:  time.Now(),
	}, nil
}
