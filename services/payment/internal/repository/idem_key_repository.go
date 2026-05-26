package repository

import (
	"errors"
	"time"

	"github.com/peltastic/payment-microservices-reference/payment/internal/db"
	"github.com/peltastic/payment-microservices-reference/payment/internal/domain"
	"gorm.io/gorm"
)

type IdemKeyRepository struct {
	db db.IDatabase
}

func NewIdemKeyRepository(db db.IDatabase) *IdemKeyRepository {
	return &IdemKeyRepository{
		db: db,
	}
}

func (r *IdemKeyRepository) Create(idemKey *domain.IdemKey) error {
	return r.db.GetDB().Create(idemKey).Error
}

func (r *IdemKeyRepository) GetByKey(key string) (*domain.IdemKey, error) {
	var idemKey domain.IdemKey
	if err := r.db.GetDB().Where("key = ?", key).First(&idemKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &idemKey, nil
}

func (r *IdemKeyRepository) UpdateResponse(key, responseBody string, expiresAt time.Time) error {
	return r.db.GetDB().Model(&domain.IdemKey{}).Where("key = ?", key).Updates(map[string]interface{}{
		"response_body": responseBody,
		"expires_at":    expiresAt,
	}).Error
}
