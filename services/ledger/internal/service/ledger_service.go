package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/db"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/domain"
	appLogger "github.com/peltastic/payment-microservices-reference/ledger/internal/logger"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/metrics"
	"gorm.io/gorm"
)

var ErrInvalidPaymentEvent = errors.New("invalid payment event")

type PaymentEventData struct {
	PaymentID     string
	MerchantID    string
	Amount        int64
	Currency      string
	Status        string
	CustomerEmail string
	CustomerName  string
}

type PaymentEvent struct {
	ID        string
	Type      string
	Version   string
	Timestamp time.Time
	Source    string
	Data      PaymentEventData
}

type LedgerService struct {
	processedEventRepo  domain.ProcessedEventRepoitory
	journalEntryRepo    domain.JournalEntryRepository
	merchantBalanceRepo domain.MerchantBalanceRepository
	db                  db.IDatabase
}

func NewLedgerService(processedEventRepo domain.ProcessedEventRepoitory, journalEntryRepo domain.JournalEntryRepository, merchantBalanceRepo domain.MerchantBalanceRepository, db db.IDatabase) *LedgerService {
	slog.Default().With("component", "ledger_service").Info("ledger service initialized")
	return &LedgerService{
		processedEventRepo:  processedEventRepo,
		journalEntryRepo:    journalEntryRepo,
		merchantBalanceRepo: merchantBalanceRepo,
		db:                  db,
	}
}

func (s *LedgerService) logger(ctx context.Context) *slog.Logger {
	return appLogger.FromContext(ctx).With("component", "ledger_service")
}

func (s *LedgerService) HandlePaymentSucceeded(c context.Context, event PaymentEvent) error {
	log := s.logger(c)
	log.Info("ledger payment event processing started",
		"event_id", event.ID,
		"event_type", event.Type,
		"version", event.Version,
		"source", event.Source,
		"payment_id", event.Data.PaymentID,
		"merchant_id", event.Data.MerchantID,
		"amount", event.Data.Amount,
		"currency", event.Data.Currency,
		"status", event.Data.Status,
	)
	if err := validatePaymentSucceededEvent(event); err != nil {
		log.Warn("ledger payment event rejected",
			"event_id", event.ID,
			"event_type", event.Type,
			"source", event.Source,
			"payment_id", event.Data.PaymentID,
			"merchant_id", event.Data.MerchantID,
			"status", event.Data.Status,
			"error", err,
		)
		return err
	}

	if isProcessed, err := s.processedEventRepo.IsEventProcessed(event.ID); err != nil {
		log.Error("failed to check processed event",
			"event_id", event.ID,
			"event_type", event.Type,
			"error", err,
		)
		return err
	} else if isProcessed {
		log.Info("ledger payment event already processed, skipping",
			"event_id", event.ID,
			"event_type", event.Type,
			"payment_id", event.Data.PaymentID,
			"merchant_id", event.Data.MerchantID,
		)
		return nil
	}
	log.Info("ledger payment event transaction started",
		"event_id", event.ID,
		"payment_id", event.Data.PaymentID,
		"merchant_id", event.Data.MerchantID,
	)
	err := s.db.GetDB().Transaction(func(tx *gorm.DB) error {
		debit := domain.JournalEntry{
			ID:          ulid.Make().String(),
			MerchantID:  event.Data.MerchantID,
			PaymentID:   event.Data.PaymentID,
			Amount:      event.Data.Amount,
			Currency:    event.Data.Currency,
			EntryType:   domain.EntryTypeDebit,
			Description: "Payment succeeded - debit",
		}
		if err := s.journalEntryRepo.WithTx(tx).Create(&debit); err != nil {
			log.Error("failed to create debit journal entry",
				"event_id", event.ID,
				"journal_entry_id", debit.ID,
				"payment_id", event.Data.PaymentID,
				"merchant_id", event.Data.MerchantID,
				"amount", event.Data.Amount,
				"error", err,
			)
			return err
		}
		log.Info("debit journal entry created",
			"event_id", event.ID,
			"journal_entry_id", debit.ID,
			"payment_id", event.Data.PaymentID,
			"merchant_id", event.Data.MerchantID,
			"amount", event.Data.Amount,
			"currency", event.Data.Currency,
		)

		credit := domain.JournalEntry{
			ID:          ulid.Make().String(),
			MerchantID:  event.Data.MerchantID,
			PaymentID:   event.Data.PaymentID,
			Amount:      event.Data.Amount,
			Currency:    event.Data.Currency,
			EntryType:   domain.EntryTypeCredit,
			Description: "Payment succeeded - credit",
		}
		if err := s.journalEntryRepo.WithTx(tx).Create(&credit); err != nil {
			log.Error("failed to create credit journal entry",
				"event_id", event.ID,
				"journal_entry_id", credit.ID,
				"payment_id", event.Data.PaymentID,
				"merchant_id", event.Data.MerchantID,
				"amount", event.Data.Amount,
				"error", err,
			)
			return err
		}
		log.Info("credit journal entry created",
			"event_id", event.ID,
			"journal_entry_id", credit.ID,
			"payment_id", event.Data.PaymentID,
			"merchant_id", event.Data.MerchantID,
			"amount", event.Data.Amount,
			"currency", event.Data.Currency,
		)

		if err := s.merchantBalanceRepo.WithTx(tx).IncrementPendingBalance(c, event.Data.MerchantID, event.Data.Amount); err != nil {
			log.Error("failed to increment pending balance",
				"event_id", event.ID,
				"payment_id", event.Data.PaymentID,
				"merchant_id", event.Data.MerchantID,
				"amount", event.Data.Amount,
				"error", err,
			)
			return err
		}
		log.Info("pending balance incremented",
			"event_id", event.ID,
			"payment_id", event.Data.PaymentID,
			"merchant_id", event.Data.MerchantID,
			"amount", event.Data.Amount,
		)
		if err := s.processedEventRepo.WithTx(tx).MarkEventProcessed(event.ID, event.Data.MerchantID, event.Type); err != nil {
			log.Error("failed to mark event processed",
				"event_id", event.ID,
				"event_type", event.Type,
				"error", err,
			)
			return err
		}
		log.Info("event marked processed", "event_id", event.ID, "event_type", event.Type)
		return nil

	})
	if err != nil {
		log.Error("ledger payment event processing failed",
			"event_id", event.ID,
			"event_type", event.Type,
			"payment_id", event.Data.PaymentID,
			"merchant_id", event.Data.MerchantID,
			"error", err,
		)
		return err
	}
	metrics.JournalEntriesTotal.WithLabelValues(string(domain.EntryTypeDebit)).Inc()
	metrics.JournalEntriesTotal.WithLabelValues(string(domain.EntryTypeCredit)).Inc()

	log.Info("ledger payment event processing completed",
		"event_id", event.ID,
		"event_type", event.Type,
		"payment_id", event.Data.PaymentID,
		"merchant_id", event.Data.MerchantID,
		"amount", event.Data.Amount,
		"currency", event.Data.Currency,
	)
	return nil
}

func validatePaymentSucceededEvent(event PaymentEvent) error {
	switch {
	case strings.TrimSpace(event.ID) == "":
		return fmt.Errorf("%w: event id is required", ErrInvalidPaymentEvent)
	case event.Type != "payment.succeeded":
		return fmt.Errorf("%w: unexpected event type %q", ErrInvalidPaymentEvent, event.Type)
	case event.Source != "payment-service":
		return fmt.Errorf("%w: unexpected source %q", ErrInvalidPaymentEvent, event.Source)
	case strings.TrimSpace(event.Data.PaymentID) == "":
		return fmt.Errorf("%w: payment id is required", ErrInvalidPaymentEvent)
	case strings.TrimSpace(event.Data.MerchantID) == "":
		return fmt.Errorf("%w: merchant id is required", ErrInvalidPaymentEvent)
	case event.Data.Amount <= 0:
		return fmt.Errorf("%w: amount must be positive", ErrInvalidPaymentEvent)
	case event.Data.Currency != "NGN":
		return fmt.Errorf("%w: unsupported currency %q", ErrInvalidPaymentEvent, event.Data.Currency)
	case event.Data.Status != "completed":
		return fmt.Errorf("%w: unexpected payment status %q", ErrInvalidPaymentEvent, event.Data.Status)
	default:
		return nil
	}
}

func (s *LedgerService) GetBalance(c context.Context, merchantID string) (*domain.MerchantBalance, error) {
	log := s.logger(c)
	log.Info("loading merchant balance", "merchant_id", merchantID)
	balance, err := s.merchantBalanceRepo.GetMaterialisedBalance(c, merchantID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("failed to load materialized balance",
			"merchant_id", merchantID,
			"error", err,
		)
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Info("merchant balance not found, returning zero balance", "merchant_id", merchantID)
		return &domain.MerchantBalance{
			MerchantID: merchantID,
			Available:  0,
			Pending:    0,
			Currency:   "NGN",
			UpdatedAt:  time.Now(),
		}, nil
	}

	log.Info("merchant balance loaded",
		"merchant_id", merchantID,
		"available", balance.Available,
		"pending", balance.Pending,
		"currency", balance.Currency,
	)
	return balance, nil
}

func (s *LedgerService) VerifyBalance(c context.Context, merchantID string) (bool, error) {
	log := s.logger(c)
	log.Info("verifying merchant balance", "merchant_id", merchantID)
	materialized, err := s.merchantBalanceRepo.GetMaterialisedBalance(c, merchantID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("failed to load materialized balance for verification",
			"merchant_id", merchantID,
			"error", err,
		)
		return false, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		materialized = &domain.MerchantBalance{
			MerchantID: merchantID,
			Available:  0,
			Pending:    0,
			Currency:   "NGN",
			UpdatedAt:  time.Now(),
		}
	}

	computed, err := s.merchantBalanceRepo.ComputeBalanceFromJournal(c, merchantID)
	if err != nil {
		log.Error("failed to compute journal balance for verification",
			"merchant_id", merchantID,
			"error", err,
		)
		return false, err
	}

	if materialized.Available != computed.Available {
		log.Warn("balance mismatch",
			"merchant_id", merchantID,
			"materialized_available", materialized.Available,
			"computed_available", computed.Available,
		)
		return false, nil
	}

	log.Info("merchant balance verified",
		"merchant_id", merchantID,
		"available", materialized.Available,
	)
	return true, nil
}
