package domain

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Keys struct {
	ID         string `gorm:"primaryKey;varchar(26)"`
	MerchantID string `gorm:"varchar(26);not null;reference:merchants(id)"`
	KeyHash    string `gorm:"varchar(64);unique;not null"`
	KeyPrefix  string `gorm:"varchar(12);not null"`
	Scope      string `gorm:"varchar(255);not null;default:'full'"`
	IsActive   bool   `gorm:"not null;default:true"`
	LastUsedAt time.Time
	ExpiresAt  time.Time
	created_at time.Time
}

func (Keys) TableName() string {
	return "api_keys"
}

type RevokedKeys struct {
	KeyHash   string `gorm:"primaryKey;varchar(64);unique;not null"`
	Reason    string `gorm:"varchar(100)"`
	RevokedAt time.Time
}

type KeysRepository interface {
	Create(key *Keys) error
	UpdateLastUsedAt(ctx context.Context, keyID string) error
	GetByHash(keyHash string) (*Keys, error)
	WithTx(tx *gorm.DB) KeysRepository
	GetRevokedByHash(keyHash string) (*RevokedKeys, error)
	CreateRevokedKey(revokedKey *RevokedKeys) error
	FindByID(keyID string) (*Keys, error)
	DeactivateKey(keyID string) error
}
