package domain

import (
	"time"

	"gorm.io/gorm"
)

type ProcessedEvent struct {
	EventID     string `gorm:"type:varchar(26);primaryKey"`
	MerchantID  string `gorm:"type:varchar(26);not null"`
	EventType   string `gorm:"type:varchar(50);not null"`
	ProcessedAt time.Time
}

type ProcessedEventRepoitory interface {
	IsEventProcessed(eventID string) (bool, error)
	WithTx(tx *gorm.DB) ProcessedEventRepoitory
	MarkEventProcessed(eventID string, merchantID string, eventType string) error
}
