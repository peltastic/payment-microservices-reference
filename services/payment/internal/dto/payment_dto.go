package dto

import "encoding/json"

type CreatePaymentRequest struct {
	Amount         int64           `json:"amount" binding:"required,gt=0"`
	CustomerEmail  string          `json:"customer_email" binding:"required,email"`
	CustomerName   string          `json:"customer_name" binding:"required"`
	Metadata       json.RawMessage `json:"metadata"`
	IdempotencyKey string          `json:"idempotency_key"`
}
