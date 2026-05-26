package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/peltastic/payment-microservices-reference/payment/internal/dto"
	"github.com/peltastic/payment-microservices-reference/payment/internal/httpx"
	"github.com/peltastic/payment-microservices-reference/payment/internal/logger"
	"github.com/peltastic/payment-microservices-reference/payment/internal/logsafe"
	"github.com/peltastic/payment-microservices-reference/payment/internal/service"
	"gorm.io/gorm"
)

const maxListPaymentsLimit = 100

type PaymentHandler struct {
	paymentService *service.PaymentService
}

func NewPaymentHandler(paymentService *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentService: paymentService}
}

func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	log := logger.FromContext(c.Request.Context()).With("component", "payment_handler")
	merchantID, ok := httpx.RequiredHeader(c, "x-merchant-id")
	if !ok {
		log.Warn("request missing merchant id header",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)
		httpx.Error(c, http.StatusBadRequest, "validation_error", "x-merchant-id header is required")
		return
	}
	log.Info("create payment request received")

	var req dto.CreatePaymentRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		log.Warn("create payment request validation failed", "error", err)
		httpx.ValidationError(c, err)
		return
	}

	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = httpx.Header(c, "Idempotency-Key")
	}
	if idempotencyKey == "" {
		log.Warn("create payment request missing idempotency key")
		httpx.Error(c, http.StatusBadRequest, "validation_error", "idempotency_key is required")
		return
	}

	metadata := "{}"
	if len(req.Metadata) > 0 {
		metadata = string(req.Metadata)
	}
	log.Info("create payment request accepted",
		"amount", req.Amount,
		"metadata_size", len(metadata),
		"idempotency_key_hash", logsafe.ShortHash(idempotencyKey),
	)

	payment, err := h.paymentService.CreatePayment(c.Request.Context(), service.CreatePaymentInput{
		MerchantID:     merchantID,
		Amount:         req.Amount,
		CustomerEmail:  req.CustomerEmail,
		CustomerName:   req.CustomerName,
		Metadata:       metadata,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		log.Error("create payment request failed",
			"amount", req.Amount,
			"idempotency_key_hash", logsafe.ShortHash(idempotencyKey),
			"error", err,
		)
		switch {
		case errors.Is(err, service.ErrIdempotencyInProgress):
			httpx.Error(c, http.StatusConflict, "conflict", "request is still processing")
		case errors.Is(err, service.ErrPaymentDeclined):
			httpx.Error(c, http.StatusBadRequest, "payment_failed", err.Error())
		case errors.Is(err, service.ErrPaymentProviderFailed):
			httpx.Error(c, http.StatusServiceUnavailable, "payment_provider_unavailable", "payment provider is temporarily unavailable")
		default:
			httpx.Error(c, http.StatusInternalServerError, "internal_error", "failed to create payment")
		}
		return
	}

	log.Info("create payment request completed",
		"payment_id", payment.ID,
		"status", payment.Status,
		"amount", payment.Amount,
	)
	httpx.JSON(c, http.StatusCreated, payment)
}

func (h *PaymentHandler) GetPayment(c *gin.Context) {
	log := logger.FromContext(c.Request.Context()).With("component", "payment_handler")
	merchantID, ok := httpx.RequiredHeader(c, "x-merchant-id")
	if !ok {
		log.Warn("request missing merchant id header",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)
		httpx.Error(c, http.StatusBadRequest, "validation_error", "x-merchant-id header is required")
		return
	}

	paymentID := httpx.Param(c, "id")
	log.Info("get payment request received", "payment_id", paymentID)
	payment, err := h.paymentService.GetPayment(c.Request.Context(), merchantID, paymentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, service.ErrPaymentNotFound) {
			log.Warn("get payment request returned not found", "payment_id", paymentID, "error", err)
			httpx.Error(c, http.StatusNotFound, "not_found", "payment not found")
			return
		}

		log.Error("get payment request failed", "payment_id", paymentID, "error", err)
		httpx.Error(c, http.StatusInternalServerError, "internal_error", "failed to fetch payment")
		return
	}

	log.Info("get payment request completed",
		"payment_id", payment.ID,
		"status", payment.Status,
	)
	httpx.JSON(c, http.StatusOK, payment)
}

func (h *PaymentHandler) ListPayments(c *gin.Context) {
	log := logger.FromContext(c.Request.Context()).With("component", "payment_handler")
	merchantID, ok := httpx.RequiredHeader(c, "x-merchant-id")
	if !ok {
		log.Warn("request missing merchant id header",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)
		httpx.Error(c, http.StatusBadRequest, "validation_error", "x-merchant-id header is required")
		return
	}

	page, err := httpx.PositiveIntQuery(c, "page", 1)
	if err != nil {
		log.Warn("list payments request validation failed", "error", err)
		httpx.ValidationError(c, err)
		return
	}
	limit, err := httpx.PositiveIntQuery(c, "limit", 20)
	if err != nil {
		log.Warn("list payments request validation failed", "error", err)
		httpx.ValidationError(c, err)
		return
	}
	if limit > maxListPaymentsLimit {
		log.Warn("list payments request validation failed", "limit", limit, "max_limit", maxListPaymentsLimit)
		httpx.Error(c, http.StatusBadRequest, "validation_error", "limit must be less than or equal to 100")
		return
	}

	input := service.ListPaymentsInput{
		MerchantID: merchantID,
		Status:     httpx.Query(c, "status"),
		Page:       page,
		Limit:      limit,
	}
	log.Info("list payments request received",
		"status", input.Status,
		"page", input.Page,
		"limit", input.Limit,
	)
	payments, err := h.paymentService.ListPayments(c.Request.Context(), input)
	if err != nil {
		log.Error("list payments request failed",
			"status", input.Status,
			"page", input.Page,
			"limit", input.Limit,
			"error", err,
		)
		httpx.Error(c, http.StatusInternalServerError, "internal_error", "failed to list payments")
		return
	}

	log.Info("list payments request completed",
		"count", len(payments),
		"page", input.Page,
		"limit", input.Limit,
	)
	httpx.JSON(c, http.StatusOK, payments)
}
