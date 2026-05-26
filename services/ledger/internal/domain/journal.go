package domain

import (
	"time"

	"gorm.io/gorm"
)

type EntryTypes string

const (
	EntryTypeDebit  EntryTypes = "debit"
	EntryTypeCredit EntryTypes = "credit"
)

type JournalEntry struct {
	ID          string     `gorm:"type:varchar(26);primaryKey"`
	MerchantID  string     `gorm:"type:varchar(26);not null"`
	PaymentID   string     `gorm:"type:varchar(26);not null"`
	EntryType   EntryTypes `gorm:"type:varchar(20);not null"`
	Amount      int64      `gorm:"not null"`
	Currency    string     `gorm:"type:varchar(3);not null"`
	Description string     `gorm:"type:varchar(255)"`
	CreatedAt   time.Time
}

func (JournalEntry) TableName() string {
	return "journals"
}

type JournalEntryRepository interface {
	Create(entry *JournalEntry) error
	WithTx(tx *gorm.DB) JournalEntryRepository
}
