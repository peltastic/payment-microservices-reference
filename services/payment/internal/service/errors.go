package service

import (
	"errors"
	"fmt"
)

var (
	ErrIdempotencyInProgress    = errors.New("idempotency result is still processing")
	ErrPaymentDeclined          = errors.New("payment authorization declined")
	ErrPaymentNotFound          = errors.New("payment not found")
	ErrPaymentProviderFailed    = errors.New("payment provider request failed")
	ErrPaymentPersistenceFailed = errors.New("payment persistence failed")
)

type PaymentDeclinedError struct {
	Reason string
}

func (e PaymentDeclinedError) Error() string {
	if e.Reason == "" {
		return ErrPaymentDeclined.Error()
	}
	return fmt.Sprintf("%s: %s", ErrPaymentDeclined, e.Reason)
}

func (e PaymentDeclinedError) Unwrap() error {
	return ErrPaymentDeclined
}
