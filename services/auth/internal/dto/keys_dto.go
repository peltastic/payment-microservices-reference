package dto

import "time"

type CreateKeyRequest struct {
	MerchantName  string     `json:"merchant_name" binding:"required"`
	MerchantEmail string     `json:"merchant_email" binding:"required,email"`
	Scope         string     `json:"scope"`
	ExpiresAt     *time.Time `json:"expires_at"`
}

type ValidateKeyRequest struct {
	APIKey string `json:"api_key" binding:"required"`
}

type RevokeKeyRequest struct {
	Reason string `json:"reason" binding:"required"`
}
