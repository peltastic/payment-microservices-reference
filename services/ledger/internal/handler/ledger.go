package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/dto"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/httpx"
	appLogger "github.com/peltastic/payment-microservices-reference/ledger/internal/logger"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/service"
)

type LedgerHandler struct {
	ledgerService *service.LedgerService
}

func NewLedgerHandler(ledgerService *service.LedgerService) *LedgerHandler {
	return &LedgerHandler{ledgerService: ledgerService}
}

func (h *LedgerHandler) HandlePaymentSucceeded(c *gin.Context) {
	log := appLogger.FromContext(c.Request.Context()).With("component", "ledger_handler")
	var req dto.HandlePaymentSucceededRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		log.Warn("payment succeeded event request validation failed", "error", err)
		httpx.ValidationError(c, err)
		return
	}

	log.Info("payment succeeded event request accepted",
		"event_id", req.ID,
		"event_type", req.Type,
		"version", req.Version,
		"source", req.Source,
		"payment_id", req.Data.PaymentID,
		"merchant_id", req.Data.MerchantID,
		"amount", req.Data.Amount,
		"currency", req.Data.Currency,
		"status", req.Data.Status,
	)
	err := h.ledgerService.HandlePaymentSucceeded(c.Request.Context(), service.PaymentEvent{
		ID:        req.ID,
		Type:      req.Type,
		Version:   req.Version,
		Timestamp: req.Timestamp,
		Source:    req.Source,
		Data: service.PaymentEventData{
			PaymentID:     req.Data.PaymentID,
			MerchantID:    req.Data.MerchantID,
			Amount:        req.Data.Amount,
			Currency:      req.Data.Currency,
			Status:        req.Data.Status,
			CustomerEmail: req.Data.CustomerEmail,
			CustomerName:  req.Data.CustomerName,
		},
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidPaymentEvent) {
			log.Warn("payment succeeded event request rejected",
				"event_id", req.ID,
				"payment_id", req.Data.PaymentID,
				"merchant_id", req.Data.MerchantID,
				"error", err,
			)
			httpx.Error(c, http.StatusBadRequest, "validation_error", "invalid payment succeeded event")
			return
		}

		log.Error("payment succeeded event request failed",
			"event_id", req.ID,
			"payment_id", req.Data.PaymentID,
			"merchant_id", req.Data.MerchantID,
			"error", err,
		)
		httpx.Error(c, http.StatusInternalServerError, "internal_error", "failed to process payment succeeded event")
		return
	}

	log.Info("payment succeeded event request completed",
		"event_id", req.ID,
		"payment_id", req.Data.PaymentID,
		"merchant_id", req.Data.MerchantID,
	)
	httpx.JSON(c, http.StatusOK, map[string]any{"message": "payment succeeded event handled"})
}

func (h *LedgerHandler) GetBalance(c *gin.Context) {
	log := appLogger.FromContext(c.Request.Context()).With("component", "ledger_handler")
	merchantID, ok := httpx.RequiredHeader(c, "x-merchant-id")
	if !ok {
		log.Warn("balance request missing merchant id header",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)
		httpx.Error(c, http.StatusBadRequest, "validation_error", "x-merchant-id header is required")
		return
	}
	log.Info("balance request accepted", "merchant_id", merchantID)

	balance, err := h.ledgerService.GetBalance(c.Request.Context(), merchantID)
	if err != nil {
		log.Error("balance request failed", "merchant_id", merchantID, "error", err)
		httpx.Error(c, http.StatusInternalServerError, "internal_error", "failed to fetch balance")
		return
	}

	log.Info("balance request completed",
		"merchant_id", merchantID,
		"available", balance.Available,
		"pending", balance.Pending,
		"currency", balance.Currency,
	)
	httpx.JSON(c, http.StatusOK, balance)
}

func (h *LedgerHandler) VerifyBalance(c *gin.Context) {
	log := appLogger.FromContext(c.Request.Context()).With("component", "ledger_handler")
	merchantID, ok := httpx.RequiredHeader(c, "x-merchant-id")
	if !ok {
		log.Warn("balance verification request missing merchant id header",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)
		httpx.Error(c, http.StatusBadRequest, "validation_error", "x-merchant-id header is required")
		return
	}
	log.Info("balance verification request accepted", "merchant_id", merchantID)

	balanced, err := h.ledgerService.VerifyBalance(c.Request.Context(), merchantID)
	if err != nil {
		log.Error("balance verification request failed", "merchant_id", merchantID, "error", err)
		httpx.Error(c, http.StatusInternalServerError, "internal_error", "failed to verify balance")
		return
	}

	log.Info("balance verification request completed",
		"merchant_id", merchantID,
		"balanced", balanced,
	)
	httpx.JSON(c, http.StatusOK, map[string]any{
		"merchant_id": merchantID,
		"balanced":    balanced,
	})
}
