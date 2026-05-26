//go:build e2e

package e2e_test

import (
	"bytes"
	"net/http"
	"os"
	"testing"
)

func TestPaymentE2E_GivenRunningService_WhenHealthChecked_ThenReturnsOK(t *testing.T) {
	// Arrange
	baseURL := paymentE2EBaseURL(t)

	// Act
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("expected payment health request to succeed: %v", err)
	}
	defer resp.Body.Close()

	// Assert
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected payment health status 200, got %d", resp.StatusCode)
	}
}

func TestPaymentE2E_GivenUnsignedCreatePaymentRequest_WhenPosted_ThenRejectsWithUnauthorized(t *testing.T) {
	// Arrange
	baseURL := paymentE2EBaseURL(t)
	body := `{"amount":1000,"customer_email":"customer@example.com","customer_name":"Customer","idempotency_key":"idem_e2e_unsigned"}`

	// Act
	resp, err := http.Post(baseURL+"/payments/", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("expected payment create request to complete: %v", err)
	}
	defer resp.Body.Close()

	// Assert
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unsigned payment create request to return 401, got %d", resp.StatusCode)
	}
}

func paymentE2EBaseURL(t *testing.T) string {
	t.Helper()
	baseURL := os.Getenv("PAYMENT_REFERENCE_PAYMENT_E2E_URL")
	if baseURL == "" {
		t.Skip("PAYMENT_REFERENCE_PAYMENT_E2E_URL is required for payment e2e tests")
	}
	return baseURL
}
