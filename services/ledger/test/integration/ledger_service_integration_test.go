//go:build integration

package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	ledgerdb "github.com/peltastic/payment-microservices-reference/ledger/internal/db"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/domain"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/repository"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/service"
	"gorm.io/gorm"
)

func TestLedgerService_GivenPaymentSucceededEvent_WhenHandledTwice_ThenCreatesOneBalancedEntryPairAndOnePendingBalanceIncrement(t *testing.T) {
	// Arrange
	database := openLedgerIntegrationDB(t)
	ledgerService := service.NewLedgerService(
		repository.NewProcessedEventRepository(database),
		repository.NewJournalEntryRepository(database),
		repository.NewMerchantBalanceRepository(database),
		database,
	)
	event := ledgerPaymentSucceededEvent()

	// Act
	if err := ledgerService.HandlePaymentSucceeded(context.Background(), event); err != nil {
		t.Fatalf("expected first ledger event processing to succeed: %v", err)
	}
	if err := ledgerService.HandlePaymentSucceeded(context.Background(), event); err != nil {
		t.Fatalf("expected duplicate ledger event processing to be idempotent: %v", err)
	}

	// Assert
	var entries []domain.JournalEntry
	if err := database.GetDB().Where("payment_id = ?", event.Data.PaymentID).Find(&entries).Error; err != nil {
		t.Fatalf("expected journal entries to load: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected exactly one debit and one credit entry, got %d", len(entries))
	}
	var debitTotal, creditTotal int64
	for _, entry := range entries {
		switch entry.EntryType {
		case domain.EntryTypeDebit:
			debitTotal += entry.Amount
		case domain.EntryTypeCredit:
			creditTotal += entry.Amount
		}
	}
	if debitTotal != event.Data.Amount || creditTotal != event.Data.Amount {
		t.Fatalf("expected balanced debit/credit totals of %d, got debit=%d credit=%d", event.Data.Amount, debitTotal, creditTotal)
	}

	var balance domain.MerchantBalance
	if err := database.GetDB().Where("merchant_id = ?", event.Data.MerchantID).First(&balance).Error; err != nil {
		t.Fatalf("expected materialized balance to load: %v", err)
	}
	if balance.Pending != event.Data.Amount {
		t.Fatalf("expected one pending balance increment of %d, got %d", event.Data.Amount, balance.Pending)
	}
}

func TestLedgerService_GivenSecondJournalInsertFails_WhenHandled_ThenRollsBackFirstJournalEntry(t *testing.T) {
	// Arrange
	database := openLedgerIntegrationDB(t)
	callCount := 0
	journalRepo := &failingSecondJournalRepository{
		db:        database,
		callCount: &callCount,
	}
	ledgerService := service.NewLedgerService(
		repository.NewProcessedEventRepository(database),
		journalRepo,
		repository.NewMerchantBalanceRepository(database),
		database,
	)
	event := ledgerPaymentSucceededEvent()
	event.ID = ulid.Make().String()
	event.Data.PaymentID = ulid.Make().String()

	// Act
	err := ledgerService.HandlePaymentSucceeded(context.Background(), event)

	// Assert
	if err == nil {
		t.Fatal("expected ledger processing to fail when the second journal insert fails")
	}
	var count int64
	if err := database.GetDB().Model(&domain.JournalEntry{}).Where("payment_id = ?", event.Data.PaymentID).Count(&count).Error; err != nil {
		t.Fatalf("expected journal count query to succeed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected transaction rollback to remove first journal entry, got %d rows", count)
	}
}

func ledgerPaymentSucceededEvent() service.PaymentEvent {
	return service.PaymentEvent{
		ID:      ulid.Make().String(),
		Type:    "payment.succeeded",
		Source:  "payment-service",
		Version: "1.0",
		Data: service.PaymentEventData{
			PaymentID:  ulid.Make().String(),
			MerchantID: ulid.Make().String(),
			Amount:     2500,
			Currency:   "NGN",
			Status:     "completed",
		},
	}
}

func openLedgerIntegrationDB(t *testing.T) ledgerdb.IDatabase {
	t.Helper()
	dsn := os.Getenv("PAYMENT_REFERENCE_LEDGER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PAYMENT_REFERENCE_LEDGER_TEST_DATABASE_URL is required for ledger integration tests")
	}

	database, err := ledgerdb.NewDatabase(dsn)
	if err != nil {
		t.Fatalf("failed to open ledger integration database: %v", err)
	}
	sqlDB, err := database.GetDB().DB()
	if err != nil {
		t.Fatalf("failed to get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	schema := "ledger_it_" + ulid.Make().String()
	if err := database.GetDB().Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}
	if err := database.GetDB().Exec(`SET search_path TO "` + schema + `"`).Error; err != nil {
		t.Fatalf("failed to set test schema: %v", err)
	}
	if err := database.GetDB().AutoMigrate(&domain.JournalEntry{}, &domain.MerchantBalance{}, &domain.ProcessedEvent{}); err != nil {
		t.Fatalf("failed to migrate ledger integration schema: %v", err)
	}

	t.Cleanup(func() {
		_ = database.GetDB().Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`).Error
		_ = sqlDB.Close()
	})

	return database
}

type failingSecondJournalRepository struct {
	db        ledgerdb.IDatabase
	callCount *int
}

func (r *failingSecondJournalRepository) Create(entry *domain.JournalEntry) error {
	*r.callCount = *r.callCount + 1
	if *r.callCount == 2 {
		return errors.New("forced second journal insert failure")
	}
	return repository.NewJournalEntryRepository(r.db).Create(entry)
}

func (r *failingSecondJournalRepository) WithTx(tx *gorm.DB) domain.JournalEntryRepository {
	return &failingSecondJournalRepository{
		db:        r.db.WithTx(tx),
		callCount: r.callCount,
	}
}
