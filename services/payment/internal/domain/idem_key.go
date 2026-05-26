package domain

import "time"

type IdemKey struct {
	Key          string `gorm:"primaryKey;varchar(255);unique;not null"`
	MerchantID   string `gorm:"varchar(26);not null;reference:merchants(id)"`
	CreatedAt    time.Time
	ResponseBody string `gorm:"type:jsonb;not null"`
	ExpiresAt    time.Time
}

type IdemKeyRepository interface {
	Create(idemKey *IdemKey) error
	GetByKey(key string) (*IdemKey, error)
	UpdateResponse(key, responseBody string, expiresAt time.Time) error
}
