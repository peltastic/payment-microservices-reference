package repository

import (
	"errors"

	"github.com/peltastic/payment-microservices-reference/ledger/internal/db"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/domain"
	"gorm.io/gorm"
)

type ProcessedEventRepository struct {
	db db.IDatabase
}

func NewProcessedEventRepository(db db.IDatabase) *ProcessedEventRepository {
	return &ProcessedEventRepository{
		db: db,
	}
}

func (r *ProcessedEventRepository) WithTx(tx *gorm.DB) domain.ProcessedEventRepoitory {
	return &ProcessedEventRepository{
		db: r.db.WithTx(tx),
	}
}

func (r *ProcessedEventRepository) IsEventProcessed(eventID string) (bool, error) {
	var event domain.ProcessedEvent
	if err := r.db.GetDB().Model(&domain.ProcessedEvent{}).Where("event_id = ?", eventID).First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *ProcessedEventRepository) MarkEventProcessed(eventID string, merchantID string, eventType string) error {
	return r.db.GetDB().Create(&domain.ProcessedEvent{
		EventID:    eventID,
		MerchantID: merchantID,
		EventType:  eventType,
	}).Error
}
