// internal/service/recovery.go
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/peltastic/payment-microservices-reference/payment/internal/domain"
	"github.com/peltastic/payment-microservices-reference/payment/internal/logger"
	"github.com/peltastic/payment-microservices-reference/payment/internal/metrics"
)

type RecoveryService struct {
	paymentRepo    domain.PaymentRepository
	paymentService *PaymentService
}

func NewRecoveryService(paymentRepo domain.PaymentRepository, paymentService *PaymentService) *RecoveryService {
	slog.Default().With("component", "recovery_service").Info("recovery service initialized")
	return &RecoveryService{
		paymentRepo:    paymentRepo,
		paymentService: paymentService,
	}
}
func (s *RecoveryService) StartRecoveryJob(ctx context.Context) {
	log := logger.FromContext(ctx).With("component", "recovery_service")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log.Info("recovery job started")

	for {
		select {
		case <-ticker.C:
			s.runRecovery(ctx)
		case <-ctx.Done():
			log.Info("recovery job stopped")
			return
		}
	}
}

func (s *RecoveryService) runRecovery(c context.Context) {
	log := logger.FromContext(c).With("component", "recovery_service")
	metrics.RecoveryJobRuns.Inc()
	log.Debug("recovery scan started")
	payments, err := s.paymentRepo.GetAllUnpublished()
	if err != nil {
		log.Error("recovery job failed to query unpublished events", "error", err)
		return
	}

	if len(payments) == 0 {
		return
	}

	log.Info("recovery job found unpublished events", "count", len(payments))

	for _, payment := range payments {
		if err := s.paymentService.PublishEvent(c, payment); err != nil {
			log.Error("recovery job failed to publish event",
				"payment_id", payment.ID,
				"merchant_id", payment.MerchantID,
				"status", payment.Status,
				"error", err,
			)
			continue
		}

		log.Info("recovery job published event",
			"payment_id", payment.ID,
			"merchant_id", payment.MerchantID,
			"status", payment.Status,
		)
		metrics.RecoveryJobEvents.Inc()
	}
}
