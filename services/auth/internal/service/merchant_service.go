package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/peltastic/payment-microservices-reference/auth/internal/db"
	"github.com/peltastic/payment-microservices-reference/auth/internal/domain"
	appLogger "github.com/peltastic/payment-microservices-reference/auth/internal/logger"
	"github.com/peltastic/payment-microservices-reference/auth/internal/logsafe"
	"gorm.io/gorm"
)

type MerchantService struct {
	keysRepo      domain.KeysRepository
	merchantsRepo domain.MerchantRepository
	db            db.IDatabase
}

func NewMerchantService(db db.IDatabase, keysRepo domain.KeysRepository, merchantsRepo domain.MerchantRepository) *MerchantService {
	slog.Default().With("component", "merchant_service").Info("merchant service initialized")
	return &MerchantService{
		keysRepo:      keysRepo,
		merchantsRepo: merchantsRepo,
		db:            db,
	}
}

func (s *MerchantService) logger(ctx context.Context) *slog.Logger {
	return appLogger.FromContext(ctx).With("component", "merchant_service")
}

func (s *MerchantService) CreateMerchant(ctx context.Context, merchantName string, merchantEmail string, scope string, requestedExpiresAt *time.Time) (string, error) {
	log := s.logger(ctx)
	expiresAt := time.Now().UTC().Add(365 * 24 * time.Hour)
	if requestedExpiresAt != nil {
		expiresAt = requestedExpiresAt.UTC()
	}

	log.Info("creating merchant and api key",
		"merchant_name", merchantName,
		"merchant_email", merchantEmail,
		"scope", scope,
		"expires_at", expiresAt,
	)

	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		log.Error("failed to generate api key bytes", "merchant_email", merchantEmail, "error", err)
		return "", err
	}

	const keyPrefix = "pk_live_"
	rawKey := keyPrefix + base64.URLEncoding.EncodeToString(rawBytes)
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	merchantID := ulid.Make().String()
	keyID := ulid.Make().String()
	log.Info("merchant and key identifiers generated",
		"merchant_id", merchantID,
		"key_id", keyID,
		"key_prefix", keyPrefix,
		"key_hash_prefix", logsafe.ShortHash(keyHash),
	)

	err := s.db.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := s.merchantsRepo.WithTx(tx).Create(&domain.Merchant{
			Name:  merchantName,
			Email: merchantEmail,
			ID:    merchantID,
		}); err != nil {
			log.Error("failed to create merchant record",
				"merchant_id", merchantID,
				"merchant_email", merchantEmail,
				"error", err,
			)
			return err
		}

		if err := s.keysRepo.WithTx(tx).Create(&domain.Keys{
			ID:         keyID,
			MerchantID: merchantID,
			KeyHash:    keyHash,
			KeyPrefix:  keyPrefix,
			Scope:      scope,
			ExpiresAt:  expiresAt,
		}); err != nil {
			log.Error("failed to create api key record",
				"merchant_id", merchantID,
				"key_id", keyID,
				"key_hash_prefix", logsafe.ShortHash(keyHash),
				"error", err,
			)
			return err
		}

		return nil
	})
	if err != nil {
		log.Error("merchant and api key creation failed",
			"merchant_id", merchantID,
			"key_id", keyID,
			"merchant_email", merchantEmail,
			"error", err,
		)
		return "", err
	}

	log.Info("merchant and api key created",
		"merchant_id", merchantID,
		"key_id", keyID,
		"merchant_email", merchantEmail,
		"scope", scope,
		"expires_at", expiresAt,
	)

	return rawKey, nil
}
