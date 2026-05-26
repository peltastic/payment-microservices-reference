package dto

import "time"

type HandlePaymentSucceededRequest struct {
	ID        string                    `json:"id" binding:"required"`
	Type      string                    `json:"type" binding:"required"`
	Version   string                    `json:"version"`
	Timestamp time.Time                 `json:"timestamp"`
	Source    string                    `json:"source"`
	Data      PaymentSucceededEventData `json:"data" binding:"required"`
}

type PaymentSucceededEventData struct {
	PaymentID     string `json:"payment_id" binding:"required"`
	MerchantID    string `json:"merchant_id" binding:"required"`
	Amount        int64  `json:"amount" binding:"required,gt=0"`
	Currency      string `json:"currency" binding:"required"`
	Status        string `json:"status"`
	CustomerEmail string `json:"customer_email"`
	CustomerName  string `json:"customer_name"`
}
