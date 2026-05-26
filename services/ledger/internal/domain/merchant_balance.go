package domain

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type MerchantBalance struct {
	MerchantID string `gorm:"type:varchar(26);primaryKey"`
	Available  int64  `gorm:"not null;default:0"`
	Pending    int64  `gorm:"not null;default:0"`
	Currency   string `gorm:"type:varchar(3);not null"`
	UpdatedAt  time.Time
}

type MerchantBalanceRepository interface {
	IncrementPendingBalance(ctx context.Context, merchantID string, amount int64) error
	WithTx(tx *gorm.DB) MerchantBalanceRepository
	GetMaterialisedBalance(ctx context.Context, merchantID string) (*MerchantBalance, error)
	ComputeBalanceFromJournal(ctx context.Context, merchantID string) (*MerchantBalance, error)
}
