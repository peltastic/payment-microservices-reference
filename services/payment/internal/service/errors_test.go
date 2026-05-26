package service

import (
	"errors"
	"testing"
)

func TestPaymentDeclinedErrorMatchesSentinel(t *testing.T) {
	err := PaymentDeclinedError{Reason: "card declined"}

	if !errors.Is(err, ErrPaymentDeclined) {
		t.Fatalf("expected PaymentDeclinedError to match ErrPaymentDeclined")
	}

	if got := err.Error(); got != "payment authorization declined: card declined" {
		t.Fatalf("unexpected error message: %q", got)
	}
}
