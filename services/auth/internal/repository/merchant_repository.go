package repository

import (
	"github.com/peltastic/payment-microservices-reference/auth/internal/db"
	"github.com/peltastic/payment-microservices-reference/auth/internal/domain"
	"gorm.io/gorm"
)

type MerchantsRepository struct {
	db db.IDatabase
}

func NewMerchantsRepository(db db.IDatabase) *MerchantsRepository {
	return &MerchantsRepository{
		db: db,
	}
}

func (m *MerchantsRepository) Create(merchant *domain.Merchant) error {
	return m.db.GetDB().Create(merchant).Error
}

func (m *MerchantsRepository) WithTx(tx *gorm.DB) domain.MerchantRepository {
	return &MerchantsRepository{db: m.db.WithTx(tx)}
}
