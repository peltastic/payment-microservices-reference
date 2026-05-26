//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	paymentdb "github.com/peltastic/payment-microservices-reference/payment/internal/db"
	"github.com/peltastic/payment-microservices-reference/payment/internal/domain"
	"github.com/peltastic/payment-microservices-reference/payment/internal/repository"
)

func TestPaymentsRepository_GivenMultipleMerchants_WhenFindAllByID_ThenFiltersAndPaginates(t *testing.T) {
	// Arrange
	database := openPaymentIntegrationDB(t)
	repo := repository.NewPaymentsRepository(database)
	merchantA := ulid.Make().String()
	merchantB := ulid.Make().String()
	createIntegrationPayment(t, repo, merchantA, "pay_"+ulid.Make().String())
	createIntegrationPayment(t, repo, merchantB, "pay_"+ulid.Make().String())
	secondMerchantAPayment := "pay_" + ulid.Make().String()
	createIntegrationPayment(t, repo, merchantA, secondMerchantAPayment)

	// Act
	pageOne, err := repo.FindAllByID(merchantA, 1, 1)
	if err != nil {
		t.Fatalf("expected first page without error: %v", err)
	}
	pageTwo, err := repo.FindAllByID(merchantA, 2, 1)
	if err != nil {
		t.Fatalf("expected second page without error: %v", err)
	}

	// Assert
	if len(pageOne) != 1 || len(pageTwo) != 1 {
		t.Fatalf("expected one payment per page, got page one=%d page two=%d", len(pageOne), len(pageTwo))
	}
	if pageOne[0].MerchantID != merchantA || pageTwo[0].MerchantID != merchantA {
		t.Fatalf("expected results to be filtered by merchant %s, got %#v %#v", merchantA, pageOne[0], pageTwo[0])
	}
	if pageOne[0].ID == pageTwo[0].ID {
		t.Fatalf("expected pagination to return different rows, got %s twice", pageOne[0].ID)
	}
}

func TestPaymentsRepository_GivenProcessingPayment_WhenUpdated_ThenPersistsCompletedStatus(t *testing.T) {
	// Arrange
	database := openPaymentIntegrationDB(t)
	repo := repository.NewPaymentsRepository(database)
	paymentID := "pay_" + ulid.Make().String()
	payment := createIntegrationPayment(t, repo, ulid.Make().String(), paymentID)

	// Act
	payment.Status = domain.StatusCompleted
	payment.BankReference = "bank_ref_integration"
	if err := repo.Update(payment); err != nil {
		t.Fatalf("expected payment update without error: %v", err)
	}
	loaded, err := repo.FindByID(paymentID)

	// Assert
	if err != nil {
		t.Fatalf("expected updated payment to be loadable: %v", err)
	}
	if loaded.Status != domain.StatusCompleted || loaded.BankReference != "bank_ref_integration" {
		t.Fatalf("expected completed payment status to persist, got %#v", loaded)
	}
}

func TestIdemKeyRepository_GivenStoredResponse_WhenUpdated_ThenLoadsLatestResponse(t *testing.T) {
	// Arrange
	database := openPaymentIntegrationDB(t)
	repo := repository.NewIdemKeyRepository(database)
	key := "idem:" + ulid.Make().String()
	if err := repo.Create(&domain.IdemKey{
		Key:          key,
		MerchantID:   ulid.Make().String(),
		ResponseBody: `{"status":"processing"}`,
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatalf("failed to seed idempotency key: %v", err)
	}

	// Act
	if err := repo.UpdateResponse(key, `{"status":"completed"}`, time.Now().UTC().Add(2*time.Hour)); err != nil {
		t.Fatalf("expected idempotency update without error: %v", err)
	}
	loaded, err := repo.GetByKey(key)

	// Assert
	if err != nil {
		t.Fatalf("expected idempotency key to load without error: %v", err)
	}
	if loaded == nil || !jsonEqual(t, loaded.ResponseBody, `{"status":"completed"}`) {
		t.Fatalf("expected latest idempotency response, got %#v", loaded)
	}
}

func jsonEqual(t *testing.T, actual string, expected string) bool {
	t.Helper()

	var actualPayload any
	if err := json.Unmarshal([]byte(actual), &actualPayload); err != nil {
		t.Fatalf("failed to unmarshal actual json %q: %v", actual, err)
	}

	var expectedPayload any
	if err := json.Unmarshal([]byte(expected), &expectedPayload); err != nil {
		t.Fatalf("failed to unmarshal expected json %q: %v", expected, err)
	}

	return reflect.DeepEqual(actualPayload, expectedPayload)
}

func createIntegrationPayment(t *testing.T, repo *repository.PaymentsRepository, merchantID string, paymentID string) *domain.Payment {
	t.Helper()
	payment := &domain.Payment{
		ID:             paymentID,
		MerchantID:     merchantID,
		Amount:         1500,
		Currency:       "NGN",
		Status:         domain.StatusProcessing,
		IdempotencyKey: fmt.Sprintf("idem-%s", paymentID),
		CustomerEmail:  "customer@example.com",
		CustomerName:   "Customer",
		Metadata:       "{}",
		BankReference:  "bank_ref_seed",
	}
	if err := repo.Create(payment); err != nil {
		t.Fatalf("failed to seed payment: %v", err)
	}
	return payment
}

func openPaymentIntegrationDB(t *testing.T) paymentdb.IDatabase {
	t.Helper()
	dsn := os.Getenv("PAYMENT_REFERENCE_PAYMENT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PAYMENT_REFERENCE_PAYMENT_TEST_DATABASE_URL is required for payment integration tests")
	}

	database, err := paymentdb.NewDatabase(dsn)
	if err != nil {
		t.Fatalf("failed to open payment integration database: %v", err)
	}
	sqlDB, err := database.GetDB().DB()
	if err != nil {
		t.Fatalf("failed to get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	schema := "payment_it_" + ulid.Make().String()
	if err := database.GetDB().Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}
	if err := database.GetDB().Exec(`SET search_path TO "` + schema + `"`).Error; err != nil {
		t.Fatalf("failed to set test schema: %v", err)
	}
	if err := database.GetDB().AutoMigrate(&domain.Payment{}, &domain.IdemKey{}); err != nil {
		t.Fatalf("failed to migrate payment integration schema: %v", err)
	}

	t.Cleanup(func() {
		_ = database.GetDB().Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`).Error
		_ = sqlDB.Close()
	})

	return database
}
