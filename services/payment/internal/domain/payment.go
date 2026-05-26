package domain

import (
	"time"
)

type PaymentStatus string

const (
	StatusPending    PaymentStatus = "pending"
	StatusProcessing PaymentStatus = "processing"
	StatusCompleted  PaymentStatus = "completed"

	StatusCancelled PaymentStatus = "cancelled"
	StatusFailed    PaymentStatus = "failed"
)

type Payment struct {
	ID             string        `gorm:"primaryKey;varchar(26)"`
	MerchantID     string        `gorm:"varchar(26);not null"`
	Amount         int64         `gorm:"bigint;not null"`
	Currency       string        `gorm:"varchar(3);not null"`
	Status         PaymentStatus `gorm:"varchar(20);not null;default:'pending'"`
	IdempotencyKey string        `gorm:"varchar(255);not null"`
	CustomerEmail  string        `gorm:"varchar(255);not null"`
	CustomerName   string        `gorm:"varchar(255);not null"`
	Metadata       string        `gorm:"type:jsonb;not null;default:'{}'"`
	BankReference  string        `gorm:"varchar(100)"`
	FailedReason   string        `gorm:"varchar(255)"`
	EventPublished bool          `gorm:"not null;default:false"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type PaymentRepository interface {
	Create(payment *Payment) error
	Update(payment *Payment) error
	FindByID(id string) (*Payment, error)
	FindAllByID(merchantID string, page, limit int) ([]*Payment, error)
	MarkEventPublished(paymentID string) error
	GetAllUnpublished() ([]*Payment, error)
}
