package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/peltastic/payment-microservices-reference/payment/internal/domain"
	"gorm.io/gorm"
)

func TestCreatePayment_GivenCachedIdempotencyResult_WhenCalledAgain_ThenReturnsCachedPaymentWithoutDoubleWrite(t *testing.T) {
	// Arrange
	cachedPayment := &domain.Payment{
		ID:             "pay_cached",
		MerchantID:     "mrc_cached",
		Amount:         5000,
		Currency:       "NGN",
		Status:         domain.StatusCompleted,
		IdempotencyKey: "idem_cached",
		CustomerEmail:  "customer@example.com",
		CustomerName:   "Customer",
		Metadata:       "{}",
	}
	cachedBytes, err := json.Marshal(cachedPayment)
	if err != nil {
		t.Fatalf("failed to marshal cached payment: %v", err)
	}
	payments := &fakePaymentRepository{}
	idempotency := &fakeIdemKeyRepository{}
	cache := &fakePaymentCacheRepository{getValue: string(cachedBytes)}
	service := NewPaymentService(payments, cache, idempotency, nil, nil)

	// Act
	result, err := service.CreatePayment(context.Background(), CreatePaymentInput{
		MerchantID:     "mrc_cached",
		Amount:         5000,
		CustomerEmail:  "customer@example.com",
		CustomerName:   "Customer",
		Metadata:       "{}",
		IdempotencyKey: "idem_cached",
	})

	// Assert
	if err != nil {
		t.Fatalf("expected cached idempotency result without error: %v", err)
	}
	if result == nil || result.ID != cachedPayment.ID {
		t.Fatalf("expected cached payment %#v, got %#v", cachedPayment, result)
	}
	if payments.createCalls != 0 {
		t.Fatalf("expected no payment write on cached idempotency result, got %d create calls", payments.createCalls)
	}
	if idempotency.getByKeyCalls != 0 {
		t.Fatalf("expected database idempotency lookup to be skipped on cache hit, got %d calls", idempotency.getByKeyCalls)
	}
}

func TestGetPayment_GivenPaymentBelongsToDifferentMerchant_WhenLoaded_ThenReturnsPaymentNotFound(t *testing.T) {
	// Arrange
	payments := &fakePaymentRepository{
		paymentsByID: map[string]*domain.Payment{
			"pay_other": {
				ID:         "pay_other",
				MerchantID: "mrc_owner",
				Status:     domain.StatusCompleted,
			},
		},
	}
	service := NewPaymentService(payments, &fakePaymentCacheRepository{}, &fakeIdemKeyRepository{}, nil, nil)

	// Act
	result, err := service.GetPayment(context.Background(), "mrc_requester", "pay_other")

	// Assert
	if !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("expected ErrPaymentNotFound for merchant mismatch, got result=%#v err=%v", result, err)
	}
}

type fakePaymentRepository struct {
	paymentsByID map[string]*domain.Payment
	createCalls  int
}

func (f *fakePaymentRepository) Create(payment *domain.Payment) error {
	f.createCalls++
	if f.paymentsByID == nil {
		f.paymentsByID = map[string]*domain.Payment{}
	}
	f.paymentsByID[payment.ID] = payment
	return nil
}

func (f *fakePaymentRepository) Update(payment *domain.Payment) error {
	if f.paymentsByID == nil {
		f.paymentsByID = map[string]*domain.Payment{}
	}
	f.paymentsByID[payment.ID] = payment
	return nil
}

func (f *fakePaymentRepository) FindByID(id string) (*domain.Payment, error) {
	payment := f.paymentsByID[id]
	if payment == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return payment, nil
}

func (f *fakePaymentRepository) FindAllByID(merchantID string, _ int, _ int) ([]*domain.Payment, error) {
	var payments []*domain.Payment
	for _, payment := range f.paymentsByID {
		if payment.MerchantID == merchantID {
			payments = append(payments, payment)
		}
	}
	return payments, nil
}

func (f *fakePaymentRepository) MarkEventPublished(_ string) error {
	return nil
}

func (f *fakePaymentRepository) GetAllUnpublished() ([]*domain.Payment, error) {
	return nil, nil
}

type fakeIdemKeyRepository struct {
	records       map[string]*domain.IdemKey
	getByKeyCalls int
}

func (f *fakeIdemKeyRepository) Create(idemKey *domain.IdemKey) error {
	if f.records == nil {
		f.records = map[string]*domain.IdemKey{}
	}
	f.records[idemKey.Key] = idemKey
	return nil
}

func (f *fakeIdemKeyRepository) GetByKey(key string) (*domain.IdemKey, error) {
	f.getByKeyCalls++
	return f.records[key], nil
}

func (f *fakeIdemKeyRepository) UpdateResponse(key, responseBody string, expiresAt time.Time) error {
	if f.records == nil {
		f.records = map[string]*domain.IdemKey{}
	}
	record := f.records[key]
	if record == nil {
		record = &domain.IdemKey{Key: key}
		f.records[key] = record
	}
	record.ResponseBody = responseBody
	record.ExpiresAt = expiresAt
	return nil
}

type fakePaymentCacheRepository struct {
	getValue string
}

func (f *fakePaymentCacheRepository) Set(_ context.Context, _ string, _ interface{}, _ time.Duration) error {
	return nil
}

func (f *fakePaymentCacheRepository) Get(_ context.Context, _ string) (string, error) {
	return f.getValue, nil
}

func (f *fakePaymentCacheRepository) Delete(_ context.Context, _ string) error {
	return nil
}

func (f *fakePaymentCacheRepository) SetNx(_ context.Context, _ string, _ interface{}, _ time.Duration) (bool, error) {
	return true, nil
}
