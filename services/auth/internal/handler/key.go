package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/peltastic/payment-microservices-reference/auth/internal/domain"
	"github.com/peltastic/payment-microservices-reference/auth/internal/dto"
	"github.com/peltastic/payment-microservices-reference/auth/internal/httpx"
	appLogger "github.com/peltastic/payment-microservices-reference/auth/internal/logger"
	"github.com/peltastic/payment-microservices-reference/auth/internal/service"
)

type KeysHandler struct {
	merchantService *service.MerchantService
	keysService     *service.KeyService
}

func NewKeysHandler(merchantService *service.MerchantService, keysService *service.KeyService) *KeysHandler {
	return &KeysHandler{
		merchantService: merchantService,
		keysService:     keysService,
	}
}

func (h *KeysHandler) CreateKey(c *gin.Context) {
	log := appLogger.FromContext(c.Request.Context()).With("component", "keys_handler")
	log.Info("create key request received")

	var req dto.CreateKeyRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		log.Warn("create key request validation failed", "error", err)
		httpx.ValidationError(c, err)
		return
	}

	scope, err := service.NormalizeScope(req.Scope)
	if err != nil {
		log.Warn("create key request rejected because scope is invalid", "scope", req.Scope, "error", err)
		httpx.Error(c, http.StatusBadRequest, "validation_error", "invalid scope")
		return
	}
	req.Scope = scope

	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now().UTC()) {
		log.Warn("create key request rejected because expiration is in the past")
		httpx.Error(c, http.StatusBadRequest, "validation_error", "expires_at must be in the future")
		return
	}
	log.Info("create key request accepted",
		"merchant_name", req.MerchantName,
		"merchant_email", req.MerchantEmail,
		"scope", req.Scope,
	)

	apiKey, err := h.merchantService.CreateMerchant(c.Request.Context(), req.MerchantName, req.MerchantEmail, req.Scope, req.ExpiresAt)
	if err != nil {
		log.Error("create key request failed",
			"merchant_email", req.MerchantEmail,
			"scope", req.Scope,
			"error", err,
		)
		httpx.Error(c, http.StatusInternalServerError, "internal_error", "failed to create key")
		return
	}

	log.Info("create key request completed",
		"merchant_email", req.MerchantEmail,
		"scope", req.Scope,
	)
	httpx.JSON(c, http.StatusCreated, map[string]any{
		"message":    "key created successfully",
		"api_key":    apiKey,
		"created_at": time.Now().UTC(),
	})
}

func (h *KeysHandler) RevokeKey(c *gin.Context) {
	log := appLogger.FromContext(c.Request.Context()).With("component", "keys_handler")
	keyID := httpx.Param(c, "id")
	log.Info("revoke key request received", "key_id", keyID)

	var req dto.RevokeKeyRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		log.Warn("revoke key request validation failed", "key_id", keyID, "error", err)
		httpx.ValidationError(c, err)
		return
	}

	log.Info("revoke key request accepted", "key_id", keyID, "reason", req.Reason)

	if err := h.keysService.RevokeKey(c.Request.Context(), keyID, req.Reason); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("revoke key request failed because key was not found", "key_id", keyID)
			httpx.Error(c, http.StatusNotFound, "not_found", "the specified key was not found")
		} else {
			log.Error("revoke key request failed", "key_id", keyID, "error", err)
			httpx.Error(c, http.StatusInternalServerError, "internal_error", "failed to revoke key")
		}
		return
	}

	log.Info("revoke key request completed", "key_id", keyID)
	httpx.JSON(c, http.StatusOK, map[string]any{
		"message":    "key revoked successfully",
		"revoked_at": time.Now().UTC(),
	})
}
