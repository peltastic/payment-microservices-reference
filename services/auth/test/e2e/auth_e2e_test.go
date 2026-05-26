//go:build e2e

package e2e_test

import (
	"bytes"
	"net/http"
	"os"
	"testing"
)

func TestAuthE2E_GivenRunningService_WhenHealthChecked_ThenReturnsOK(t *testing.T) {
	baseURL := authE2EBaseURL(t)

	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("expected auth health request to succeed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected auth health status 200, got %d", resp.StatusCode)
	}
}

func TestAuthE2E_GivenUnsignedInternalValidateRequest_WhenPosted_ThenRejectsWithUnauthorized(t *testing.T) {
	baseURL := authE2EBaseURL(t)

	resp, err := http.Post(baseURL+"/api/v1/auth/internal/validate", "application/json", bytes.NewBufferString(`{"api_key":"pk_invalid"}`))
	if err != nil {
		t.Fatalf("expected auth validate request to complete: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unsigned auth validate request to return 401, got %d", resp.StatusCode)
	}
}

func authE2EBaseURL(t *testing.T) string {
	t.Helper()
	baseURL := os.Getenv("PAYMENT_REFERENCE_AUTH_E2E_URL")
	if baseURL == "" {
		t.Skip("PAYMENT_REFERENCE_AUTH_E2E_URL is required for auth e2e tests")
	}
	return baseURL
}
