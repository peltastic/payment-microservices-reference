package repository

import (
	"context"

	"github.com/peltastic/payment-microservices-reference/auth/internal/db"
	"github.com/peltastic/payment-microservices-reference/auth/internal/domain"
	"gorm.io/gorm"
)

type KeysRepository struct {
	db db.IDatabase
}

func NewKeysRepository(db db.IDatabase) *KeysRepository {
	return &KeysRepository{
		db: db,
	}
}

func (k *KeysRepository) Create(key *domain.Keys) error {
	return k.db.GetDB().Create(key).Error
}

func (k *KeysRepository) WithTx(tx *gorm.DB) domain.KeysRepository {
	return &KeysRepository{db: k.db.WithTx(tx)}
}

func (k *KeysRepository) GetByHash(keyHash string) (*domain.Keys, error) {
	var key domain.Keys
	if err := k.db.GetDB().Where("key_hash = ?", keyHash).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (k *KeysRepository) GetRevokedByHash(keyHash string) (*domain.RevokedKeys, error) {
	var revokedKey domain.RevokedKeys
	if err := k.db.GetDB().Where("key_hash = ?", keyHash).First(&revokedKey).Error; err != nil {
		return nil, err
	}
	return &revokedKey, nil
}

func (k *KeysRepository) CreateRevokedKey(revokedKey *domain.RevokedKeys) error {
	return k.db.GetDB().Create(revokedKey).Error
}

func (k *KeysRepository) UpdateLastUsedAt(ctx context.Context, keyID string) error {
	return k.db.GetDB().WithContext(ctx).Model(&domain.Keys{}).Where("id = ?", keyID).Update("last_used_at", gorm.Expr("NOW()")).Error
}

func (k *KeysRepository) DeactivateKey(keyID string) error {
	return k.db.GetDB().Model(&domain.Keys{}).Where("id = ?", keyID).Update("is_active", false).Error
}

func (k *KeysRepository) FindByID(keyID string) (*domain.Keys, error) {
	var key domain.Keys
	if err := k.db.GetDB().Where("id = ?", keyID).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}
