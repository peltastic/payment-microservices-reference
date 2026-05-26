package service

import (
	"context"
	"errors"
	"testing"

	"github.com/peltastic/payment-microservices-reference/ledger/internal/domain"
	"gorm.io/gorm"
)

func TestHandlePaymentSucceeded_GivenInvalidCurrency_WhenHandled_ThenRejectsEventBeforeWritingEntries(t *testing.T) {
	// Arrange
	processedEvents := &fakeProcessedEventRepository{}
	journalEntries := &fakeJournalEntryRepository{}
	balances := &fakeMerchantBalanceRepository{}
	service := NewLedgerService(processedEvents, journalEntries, balances, nil)

	// Act
	err := service.HandlePaymentSucceeded(context.Background(), PaymentEvent{
		ID:      "evt_invalid_currency",
		Type:    "payment.succeeded",
		Source:  "payment-service",
		Version: "1.0",
		Data: PaymentEventData{
			PaymentID:  "pay_invalid_currency",
			MerchantID: "mrc_invalid_currency",
			Amount:     1000,
			Currency:   "USD",
			Status:     "completed",
		},
	})

	// Assert
	if !errors.Is(err, ErrInvalidPaymentEvent) {
		t.Fatalf("expected ErrInvalidPaymentEvent, got %v", err)
	}
	if processedEvents.isProcessedCalls != 0 || journalEntries.createCalls != 0 || balances.incrementCalls != 0 {
		t.Fatalf("expected invalid event to be rejected before repository writes, got processed=%d entries=%d balance=%d", processedEvents.isProcessedCalls, journalEntries.createCalls, balances.incrementCalls)
	}
}

func TestHandlePaymentSucceeded_GivenAlreadyProcessedEvent_WhenHandled_ThenSkipsDoubleEntryWrites(t *testing.T) {
	// Arrange
	processedEvents := &fakeProcessedEventRepository{processed: true}
	journalEntries := &fakeJournalEntryRepository{}
	balances := &fakeMerchantBalanceRepository{}
	service := NewLedgerService(processedEvents, journalEntries, balances, nil)

	// Act
	err := service.HandlePaymentSucceeded(context.Background(), validPaymentSucceededEvent())

	// Assert
	if err != nil {
		t.Fatalf("expected duplicate event to be skipped without error: %v", err)
	}
	if journalEntries.createCalls != 0 || balances.incrementCalls != 0 {
		t.Fatalf("expected duplicate event not to create entries or balances, got entries=%d balance=%d", journalEntries.createCalls, balances.incrementCalls)
	}
}

func validPaymentSucceededEvent() PaymentEvent {
	return PaymentEvent{
		ID:      "evt_valid",
		Type:    "payment.succeeded",
		Source:  "payment-service",
		Version: "1.0",
		Data: PaymentEventData{
			PaymentID:  "pay_valid",
			MerchantID: "mrc_valid",
			Amount:     1000,
			Currency:   "NGN",
			Status:     "completed",
		},
	}
}

type fakeProcessedEventRepository struct {
	processed        bool
	isProcessedCalls int
}

func (f *fakeProcessedEventRepository) IsEventProcessed(_ string) (bool, error) {
	f.isProcessedCalls++
	return f.processed, nil
}

func (f *fakeProcessedEventRepository) WithTx(_ *gorm.DB) domain.ProcessedEventRepoitory {
	return f
}

func (f *fakeProcessedEventRepository) MarkEventProcessed(_, _, _ string) error {
	return nil
}

type fakeJournalEntryRepository struct {
	createCalls int
}

func (f *fakeJournalEntryRepository) Create(_ *domain.JournalEntry) error {
	f.createCalls++
	return nil
}

func (f *fakeJournalEntryRepository) WithTx(_ *gorm.DB) domain.JournalEntryRepository {
	return f
}

type fakeMerchantBalanceRepository struct {
	incrementCalls int
}

func (f *fakeMerchantBalanceRepository) IncrementPendingBalance(_ context.Context, _ string, _ int64) error {
	f.incrementCalls++
	return nil
}

func (f *fakeMerchantBalanceRepository) WithTx(_ *gorm.DB) domain.MerchantBalanceRepository {
	return f
}

func (f *fakeMerchantBalanceRepository) GetMaterialisedBalance(_ context.Context, merchantID string) (*domain.MerchantBalance, error) {
	return &domain.MerchantBalance{MerchantID: merchantID, Currency: "NGN"}, nil
}

func (f *fakeMerchantBalanceRepository) ComputeBalanceFromJournal(_ context.Context, merchantID string) (*domain.MerchantBalance, error) {
	return &domain.MerchantBalance{MerchantID: merchantID, Currency: "NGN"}, nil
}
