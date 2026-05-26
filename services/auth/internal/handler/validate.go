package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/peltastic/payment-microservices-reference/auth/internal/domain"
	"github.com/peltastic/payment-microservices-reference/auth/internal/dto"
	"github.com/peltastic/payment-microservices-reference/auth/internal/httpx"
	appLogger "github.com/peltastic/payment-microservices-reference/auth/internal/logger"
	"github.com/peltastic/payment-microservices-reference/auth/internal/service"
)

type InternalValidateHandler struct {
	keysService *service.KeyService
}

func NewInternalValidateHandler(keysService *service.KeyService) *InternalValidateHandler {
	return &InternalValidateHandler{
		keysService: keysService,
	}
}

func (h *InternalValidateHandler) ValidateKey(c *gin.Context) {
	log := appLogger.FromContext(c.Request.Context()).With("component", "internal_validate_handler")
	log.Info("validate key request received")

	var req dto.ValidateKeyRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		log.Warn("validate key request validation failed", "error", err)
		httpx.ValidationError(c, err)
		return
	}

	log.Info("validate key request accepted")
	result, err := h.keysService.ValidateKey(c.Request.Context(), req.APIKey)

	if err != nil {
		if errors.Is(err, domain.ErrKeyRevoked) {
			log.Warn("validate key request rejected because key is revoked")
			httpx.Error(c, http.StatusForbidden, "key_revoked", "the provided API key has been revoked")
		} else if errors.Is(err, domain.ErrInvalidKey) {
			log.Warn("validate key request rejected because key is invalid")
			httpx.Error(c, http.StatusUnauthorized, "invalid_key", "the provided API key is invalid")
		} else {
			log.Error("validate key request failed", "error", err)
			httpx.Error(c, http.StatusInternalServerError, "internal_error", "failed to validate key")
		}
		return
	}

	if result == nil || !result.Valid {
		log.Warn("validate key request rejected because validation result was invalid")
		httpx.Error(c, http.StatusUnauthorized, "invalid_key", "the provided API key is invalid")
		return
	}

	log.Info("validate key request completed",
		"merchant_id", result.MerchantID,
		"scope", result.Scope,
	)
	httpx.JSON(c, http.StatusOK, result)
}
