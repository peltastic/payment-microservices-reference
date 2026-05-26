package domain

import (
	"time"

	"gorm.io/gorm"
)

type Merchant struct {
	ID         string `gorm:"primaryKey;varchar(26)"`
	Name       string `gorm:"varchar(255);not null"`
	Email      string `gorm:"varchar(255);unique;not null"`
	created_at time.Time
	updated_at time.Time
}

type MerchantRepository interface {
	Create(merchant *Merchant) error
	WithTx(tx *gorm.DB) MerchantRepository
}
