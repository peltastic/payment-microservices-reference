package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/peltastic/payment-microservices-reference/payment/internal/bank"
	"github.com/peltastic/payment-microservices-reference/payment/internal/domain"
	"github.com/peltastic/payment-microservices-reference/payment/internal/kafka"
	"github.com/peltastic/payment-microservices-reference/payment/internal/logger"
	"github.com/peltastic/payment-microservices-reference/payment/internal/logsafe"
	"github.com/peltastic/payment-microservices-reference/payment/internal/metrics"
)

type CreatePaymentInput struct {
	MerchantID     string
	Amount         int64
	CustomerEmail  string
	CustomerName   string
	Metadata       string
	IdempotencyKey string
}

type ListPaymentsInput struct {
	MerchantID string
	Status     string
	Page       int
	Limit      int
}

type PaymentService struct {
	paymentRepo domain.PaymentRepository
	bankClient  *bank.Client
	cacheRepo   domain.CacheRepository
	idemKeyRepo domain.IdemKeyRepository
	kafka       *kafka.Producer
}

func NewPaymentService(paymentRepo domain.PaymentRepository, cacheRepo domain.CacheRepository, idemKeyRepo domain.IdemKeyRepository, kafka *kafka.Producer, bankClient *bank.Client) *PaymentService {

	return &PaymentService{
		paymentRepo: paymentRepo,
		bankClient:  bankClient,
		cacheRepo:   cacheRepo,
		idemKeyRepo: idemKeyRepo,
		kafka:       kafka,
	}
}

func (s *PaymentService) logger(ctx context.Context) *slog.Logger {
	return logger.FromContext(ctx).With("component", "payment_service")
}

func (s *PaymentService) CreatePayment(c context.Context, input CreatePaymentInput) (*domain.Payment, error) {
	start := time.Now()
	defer func() {
		metrics.PaymentDuration.Observe(time.Since(start).Seconds())
	}()

	log := s.logger(c)
	log.Info("payment creation started",
		"merchant_id", input.MerchantID,
		"amount", input.Amount,
		"metadata_size", len(input.Metadata),
		"idempotency_key_hash", logsafe.ShortHash(input.IdempotencyKey),
	)

	payment := &domain.Payment{
		ID:             ulid.Make().String(),
		MerchantID:     input.MerchantID,
		Amount:         input.Amount,
		Currency:       "NGN",
		Status:         domain.StatusProcessing,
		CustomerEmail:  input.CustomerEmail,
		CustomerName:   input.CustomerName,
		Metadata:       input.Metadata,
		IdempotencyKey: input.IdempotencyKey,
		BankReference:  ulid.Make().String(),
		EventPublished: false,
	}
	cacheKey := idempotencyCacheKey(payment.MerchantID, payment.IdempotencyKey)
	lockKey := "lock:" + cacheKey
	expiresAt := time.Now().Add(24 * time.Hour)
	log.Info("payment model prepared",
		"payment_id", payment.ID,
		"merchant_id", payment.MerchantID,
		"status", payment.Status,
		"currency", payment.Currency,
		"cache_key_hash", logsafe.ShortHash(cacheKey),
		"lock_key_hash", logsafe.ShortHash(lockKey),
	)
	if existingPayment, err := s.getIdempotencyResult(c, cacheKey); err != nil {
		log.Error("failed to check idempotency result before lock acquisition",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"idempotency_key_hash", logsafe.ShortHash(payment.IdempotencyKey),
			"error", err,
		)
		return nil, err
	} else if existingPayment != nil {
		log.Info("idempotency result found before lock acquisition",
			"cache_key_hash", logsafe.ShortHash(cacheKey),
			"payment_id", existingPayment.ID,
			"status", existingPayment.Status,
		)
		return existingPayment, nil
	}

	acquired, err := s.cacheRepo.SetNx(c, lockKey, "1", 10*time.Second)
	if err != nil {
		log.Error("failed to acquire payment idempotency lock",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"idempotency_key_hash", logsafe.ShortHash(payment.IdempotencyKey),
			"error", err,
		)
		return s.pollForResults(c, cacheKey)
	}
	if !acquired {
		log.Warn("payment idempotency lock already held, polling for existing result",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"idempotency_key_hash", logsafe.ShortHash(payment.IdempotencyKey),
		)
		return s.pollForResults(c, cacheKey)
	}
	log.Info("payment idempotency lock acquired",
		"payment_id", payment.ID,
		"merchant_id", payment.MerchantID,
		"idempotency_key_hash", logsafe.ShortHash(payment.IdempotencyKey),
	)
	if existingPayment, err := s.getIdempotencyResult(c, cacheKey); err != nil {
		log.Error("failed to recheck idempotency result after lock acquisition",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"idempotency_key_hash", logsafe.ShortHash(payment.IdempotencyKey),
			"error", err,
		)
		return nil, err
	} else if existingPayment != nil {
		log.Info("idempotency result found after lock acquisition",
			"cache_key_hash", logsafe.ShortHash(cacheKey),
			"payment_id", existingPayment.ID,
			"status", existingPayment.Status,
		)
		return existingPayment, nil
	}

	err = s.paymentRepo.Create(payment)

	if err != nil {
		log.Error("failed to persist payment",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"error", err,
		)
		return nil, err
	}
	log.Info("payment persisted",
		"payment_id", payment.ID,
		"merchant_id", payment.MerchantID,
		"status", payment.Status,
	)

	paymentBytes, err := json.Marshal(payment)
	if err != nil {
		log.Error("failed to serialize payment for idempotency storage",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"error", err,
		)
		return nil, err
	}

	err = s.idemKeyRepo.Create(&domain.IdemKey{
		Key:          cacheKey,
		MerchantID:   input.MerchantID,
		ResponseBody: string(paymentBytes),
		ExpiresAt:    expiresAt,
	})

	if err != nil {
		log.Error("failed to create idempotency record",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"cache_key_hash", logsafe.ShortHash(cacheKey),
			"error", err,
		)
		return nil, err
	}
	log.Info("payment idempotency record created",
		"payment_id", payment.ID,
		"merchant_id", payment.MerchantID,
		"expires_at", expiresAt,
	)

	log.Info("authorizing payment with bank",
		"payment_id", payment.ID,
		"merchant_id", payment.MerchantID,
		"amount", payment.Amount,
		"currency", payment.Currency,
	)
	bankStart := time.Now()
	bankResp, err := s.bankClient.AuthorizePayment(c)
	metrics.BankCallDuration.Observe(time.Since(bankStart).Seconds())

	if err != nil {
		metrics.BankCallsTotal.WithLabelValues("unreachable").Inc()
		metrics.PaymentsTotal.WithLabelValues("failed", payment.Currency).Inc()
		log.Error("bank authorization failed",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"error", err,
		)
		payment.Status = domain.StatusFailed
		payment.FailedReason = "payment provider unavailable"
		if updateErr := s.paymentRepo.Update(payment); updateErr != nil {
			log.Error("failed to persist provider failure payment status",
				"payment_id", payment.ID,
				"merchant_id", payment.MerchantID,
				"error", updateErr,
			)
			return nil, fmt.Errorf("%w: %w", ErrPaymentPersistenceFailed, updateErr)
		}
		if storeErr := s.storeIdempotencyResult(c, cacheKey, payment, expiresAt); storeErr != nil {
			log.Error("failed to store provider failure idempotency result",
				"payment_id", payment.ID,
				"merchant_id", payment.MerchantID,
				"error", storeErr,
			)
			return nil, fmt.Errorf("%w: %w", ErrPaymentPersistenceFailed, storeErr)
		}
		return nil, fmt.Errorf("%w: %w", ErrPaymentProviderFailed, err)
	}
	log.Info("bank authorization response received",
		"payment_id", payment.ID,
		"merchant_id", payment.MerchantID,
		"bank_success", bankResp.Success,
		"bank_code", bankResp.Code,
	)

	if !bankResp.Success {
		metrics.BankCallsTotal.WithLabelValues("declined").Inc()
		payment.Status = domain.StatusFailed
		payment.FailedReason = bankResp.Message
		err := s.paymentRepo.Update(payment)
		if err != nil {
			log.Error("failed to persist failed payment status",
				"payment_id", payment.ID,
				"merchant_id", payment.MerchantID,
				"error", err,
			)
			return nil, err
		}
		if err := s.storeIdempotencyResult(c, cacheKey, payment, expiresAt); err != nil {
			log.Error("failed to store failed payment idempotency result",
				"payment_id", payment.ID,
				"merchant_id", payment.MerchantID,
				"error", err,
			)
			return nil, err
		}
		log.Warn("payment authorization declined",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"failed_reason", payment.FailedReason,
		)
		metrics.PaymentsTotal.WithLabelValues("failed", payment.Currency).Inc()
		return nil, PaymentDeclinedError{Reason: bankResp.Message}
	}

	metrics.BankCallsTotal.WithLabelValues("success").Inc()
	payment.Status = domain.StatusCompleted
	payment.BankReference = bankResp.Reference
	err = s.paymentRepo.Update(payment)
	if err != nil {
		log.Error("failed to persist completed payment status",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"error", err,
		)
		return nil, err
	}
	log.Info("payment status updated",
		"payment_id", payment.ID,
		"merchant_id", payment.MerchantID,
		"status", payment.Status,
	)
	if err := s.storeIdempotencyResult(c, cacheKey, payment, expiresAt); err != nil {
		log.Error("failed to store completed payment idempotency result",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"error", err,
		)
		return nil, err
	}
	metrics.PaymentsTotal.WithLabelValues("succeeded", payment.Currency).Inc()

	if err := s.PublishEvent(c, payment); err != nil {
		log.Error("failed to publish payment event, recovery job will retry",
			"payment_id", payment.ID,
			"error", err,
		)
	}

	log.Info("payment creation completed",
		"payment_id", payment.ID,
		"merchant_id", payment.MerchantID,
		"status", payment.Status,
		"amount", payment.Amount,
		"currency", payment.Currency,
	)
	return payment, nil
}

func (s *PaymentService) GetPayment(ctx context.Context, merchantId string, paymentId string) (*domain.Payment, error) {
	log := s.logger(ctx)
	log.Info("loading payment",
		"merchant_id", merchantId,
		"payment_id", paymentId,
	)
	payment, err := s.paymentRepo.FindByID(paymentId)
	if err != nil {
		log.Error("failed to load payment",
			"merchant_id", merchantId,
			"payment_id", paymentId,
			"error", err,
		)
		return nil, err
	}

	if payment.MerchantID != merchantId {
		log.Warn("payment merchant mismatch",
			"requested_merchant_id", merchantId,
			"payment_merchant_id", payment.MerchantID,
			"payment_id", paymentId,
		)
		return nil, ErrPaymentNotFound
	}

	log.Info("payment loaded",
		"merchant_id", payment.MerchantID,
		"payment_id", payment.ID,
		"status", payment.Status,
		"amount", payment.Amount,
	)
	return payment, nil
}

func (s *PaymentService) ListPayments(ctx context.Context, input ListPaymentsInput) ([]*domain.Payment, error) {
	log := s.logger(ctx)
	log.Info("listing payments",
		"merchant_id", input.MerchantID,
		"status", input.Status,
		"page", input.Page,
		"limit", input.Limit,
	)
	payments, err := s.paymentRepo.FindAllByID(input.MerchantID, input.Page, input.Limit)
	if err != nil {
		log.Error("failed to list payments",
			"merchant_id", input.MerchantID,
			"status", input.Status,
			"page", input.Page,
			"limit", input.Limit,
			"error", err,
		)
		return nil, err
	}

	log.Info("payments listed",
		"merchant_id", input.MerchantID,
		"status", input.Status,
		"page", input.Page,
		"limit", input.Limit,
		"count", len(payments),
	)
	return payments, nil
}

func (s *PaymentService) pollForResults(c context.Context, cacheKey string) (*domain.Payment, error) {
	log := s.logger(c)
	log.Info("polling for idempotency result", "cache_key_hash", logsafe.ShortHash(cacheKey))
	delays := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1600 * time.Millisecond,
	}

	for _, delay := range delays {
		select {
		case <-c.Done():
			log.Error("idempotency polling cancelled", "cache_key_hash", logsafe.ShortHash(cacheKey), "error", c.Err())
			return nil, c.Err()
		case <-time.After(delay):
			if payment, err := s.getIdempotencyResult(c, cacheKey); payment != nil {
				log.Info("idempotency result found",
					"cache_key_hash", logsafe.ShortHash(cacheKey),
					"payment_id", payment.ID,
					"status", payment.Status,
				)
				return payment, err
			}
		}
	}
	if payment, _ := s.getIdempotencyResult(c, cacheKey); payment != nil {
		log.Info("idempotency result found after final lookup",
			"cache_key_hash", logsafe.ShortHash(cacheKey),
			"payment_id", payment.ID,
			"status", payment.Status,
		)
		return payment, nil
	}
	log.Warn("idempotency result still processing after timeout", "cache_key_hash", logsafe.ShortHash(cacheKey))
	return nil, ErrIdempotencyInProgress
}

func (s *PaymentService) getIdempotencyResult(c context.Context, cacheKey string) (*domain.Payment, error) {
	log := s.logger(c)
	if cached, err := s.cacheRepo.Get(c, cacheKey); err == nil && cached != "" {
		metrics.IdempotencyCacheHits.Inc()
		log.Info("idempotency cache hit", "cache_key_hash", logsafe.ShortHash(cacheKey))
		var payment domain.Payment
		if err := json.Unmarshal([]byte(cached), &payment); err != nil {
			log.Error("failed to unmarshal cached idempotency payment",
				"cache_key_hash", logsafe.ShortHash(cacheKey),
				"error", err,
			)
			return nil, err
		}
		return &payment, nil
	} else if err != nil {
		log.Error("failed to read idempotency cache",
			"cache_key_hash", logsafe.ShortHash(cacheKey),
			"error", err,
		)
	}

	log.Info("idempotency cache miss, loading from database", "cache_key_hash", logsafe.ShortHash(cacheKey))
	record, err := s.idemKeyRepo.GetByKey(cacheKey)
	if err != nil {
		log.Error("failed to load idempotency record",
			"cache_key_hash", logsafe.ShortHash(cacheKey),
			"error", err,
		)
		return nil, err
	}
	if record == nil {
		log.Debug("idempotency record not found", "cache_key_hash", logsafe.ShortHash(cacheKey))
		return nil, nil
	}
	var p domain.Payment
	if err := json.Unmarshal([]byte(record.ResponseBody), &p); err != nil {
		log.Error("failed to unmarshal idempotency record response",
			"cache_key_hash", logsafe.ShortHash(cacheKey),
			"error", err,
		)
		return nil, err
	}

	if err := s.cacheRepo.Set(c, cacheKey, record.ResponseBody, time.Until(record.ExpiresAt)); err != nil {
		log.Error("failed to repopulate idempotency cache", "cache_key_hash", logsafe.ShortHash(cacheKey), "error", err)
	}

	return &p, nil
}

func (s *PaymentService) storeIdempotencyResult(c context.Context, cacheKey string, payment *domain.Payment, expiresAt time.Time) error {
	log := s.logger(c)
	log.Info("storing idempotency result",
		"cache_key_hash", logsafe.ShortHash(cacheKey),
		"payment_id", payment.ID,
		"merchant_id", payment.MerchantID,
		"status", payment.Status,
		"expires_at", expiresAt,
	)
	serialized, err := json.Marshal(payment)
	if err != nil {
		log.Error("failed to serialize idempotency result",
			"cache_key_hash", logsafe.ShortHash(cacheKey),
			"payment_id", payment.ID,
			"error", err,
		)
		return err
	}

	if err := s.idemKeyRepo.UpdateResponse(cacheKey, string(serialized), expiresAt); err != nil {
		log.Error("failed to update idempotency record",
			"cache_key_hash", logsafe.ShortHash(cacheKey),
			"payment_id", payment.ID,
			"error", err,
		)
		return err
	}

	if err := s.cacheRepo.Set(c, cacheKey, serialized, time.Until(expiresAt)); err != nil {
		log.Error("failed to cache idempotency result", "cache_key_hash", logsafe.ShortHash(cacheKey), "error", err)
	}

	log.Info("idempotency result stored",
		"cache_key_hash", logsafe.ShortHash(cacheKey),
		"payment_id", payment.ID,
		"merchant_id", payment.MerchantID,
		"status", payment.Status,
	)
	return nil
}

func idempotencyCacheKey(merchantID string, key string) string {
	return fmt.Sprintf("idem:%s:%s", merchantID, logsafe.ShortHash(key))
}

func (s *PaymentService) PublishEvent(c context.Context, payment *domain.Payment) error {
	log := s.logger(c)
	eventType := "payment.succeeded"
	if payment.Status == domain.StatusFailed {
		eventType = "payment.failed"
	}
	log.Info("publishing payment event",
		"payment_id", payment.ID,
		"merchant_id", payment.MerchantID,
		"event_type", eventType,
		"status", payment.Status,
	)

	data := map[string]interface{}{
		"payment_id":     payment.ID,
		"merchant_id":    payment.MerchantID,
		"amount":         payment.Amount,
		"currency":       payment.Currency,
		"status":         payment.Status,
		"customer_email": payment.CustomerEmail,
		"customer_name":  payment.CustomerName,
		"bank_reference": payment.BankReference,
		"failed_reason":  payment.FailedReason,
	}

	err := s.kafka.Publish(c, eventType, data)
	if err != nil {
		metrics.KafkaPublishTotal.WithLabelValues(s.kafka.Topic(), "failed").Inc()
		log.Error("failed to publish payment event",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"event_type", eventType,
			"error", err,
		)
		return fmt.Errorf("failed to publish event: %w", err)
	}
	metrics.KafkaPublishTotal.WithLabelValues(s.kafka.Topic(), "success").Inc()
	if err := s.paymentRepo.MarkEventPublished(payment.ID); err != nil {
		log.Error("failed to mark payment event as published",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"event_type", eventType,
			"error", err,
		)
		return err
	}
	log.Info("payment event published",
		"payment_id", payment.ID,
		"merchant_id", payment.MerchantID,
		"event_type", eventType,
	)
	return nil
}
