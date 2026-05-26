package repository

import (
	"github.com/peltastic/payment-microservices-reference/payment/internal/db"
	"github.com/peltastic/payment-microservices-reference/payment/internal/domain"
)

type PaymentsRepository struct {
	db db.IDatabase
}

func NewPaymentsRepository(db db.IDatabase) *PaymentsRepository {
	return &PaymentsRepository{
		db: db,
	}
}

func (p *PaymentsRepository) Create(payment *domain.Payment) error {
	return p.db.GetDB().Create(payment).Error
}

func (p *PaymentsRepository) Update(payment *domain.Payment) error {
	return p.db.GetDB().Save(payment).Error
}

func (p *PaymentsRepository) FindByID(id string) (*domain.Payment, error) {
	var payment domain.Payment
	if err := p.db.GetDB().Where("id = ?", id).First(&payment).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

func (p *PaymentsRepository) MarkEventPublished(paymentID string) error {
	return p.db.GetDB().Model(&domain.Payment{}).Where("id = ?", paymentID).Update("event_published", true).Error
}

func (p *PaymentsRepository) GetAllUnpublished() ([]*domain.Payment, error) {
	var payments []*domain.Payment
	if err := p.db.GetDB().
		Where("event_published = ?", false).
		Where("status != ?", domain.StatusProcessing).
		Find(&payments).Error; err != nil {
		return nil, err
	}
	return payments, nil
}

func (p *PaymentsRepository) FindAllByID(merchantID string, page, limit int) ([]*domain.Payment, error) {
	var payments []*domain.Payment
	offset := (page - 1) * limit
	if err := p.db.GetDB().
		Where("merchant_id = ?", merchantID).
		Offset(offset).
		Limit(limit).
		Find(&payments).Error; err != nil {
		return nil, err
	}
	return payments, nil
}
