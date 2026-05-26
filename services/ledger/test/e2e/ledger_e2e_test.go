//go:build e2e

package e2e_test

import (
	"net/http"
	"os"
	"testing"
)

func TestLedgerE2E_GivenRunningService_WhenHealthChecked_ThenReturnsOK(t *testing.T) {
	// Arrange
	baseURL := ledgerE2EBaseURL(t)

	// Act
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("expected ledger health request to succeed: %v", err)
	}
	defer resp.Body.Close()

	// Assert
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ledger health status 200, got %d", resp.StatusCode)
	}
}

func TestLedgerE2E_GivenUnsignedBalanceRequest_WhenRequested_ThenRejectsWithUnauthorized(t *testing.T) {
	// Arrange
	baseURL := ledgerE2EBaseURL(t)

	// Act
	resp, err := http.Get(baseURL + "/balance/")
	if err != nil {
		t.Fatalf("expected ledger balance request to complete: %v", err)
	}
	defer resp.Body.Close()

	// Assert
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unsigned ledger balance request to return 401, got %d", resp.StatusCode)
	}
}

func ledgerE2EBaseURL(t *testing.T) string {
	t.Helper()
	baseURL := os.Getenv("PAYMENT_REFERENCE_LEDGER_E2E_URL")
	if baseURL == "" {
		t.Skip("PAYMENT_REFERENCE_LEDGER_E2E_URL is required for ledger e2e tests")
	}
	return baseURL
}
