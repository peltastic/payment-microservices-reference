package repository

import (
	"github.com/peltastic/payment-microservices-reference/ledger/internal/db"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/domain"
	"gorm.io/gorm"
)

type JournalEntryRepository struct {
	db db.IDatabase
}

func NewJournalEntryRepository(db db.IDatabase) *JournalEntryRepository {
	return &JournalEntryRepository{
		db: db,
	}
}

func (r *JournalEntryRepository) Create(entry *domain.JournalEntry) error {
	return r.db.GetDB().Create(entry).Error
}

func (r *JournalEntryRepository) WithTx(tx *gorm.DB) domain.JournalEntryRepository {
	return &JournalEntryRepository{
		db: r.db.WithTx(tx),
	}
}
