package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/peltastic/payment-microservices-reference/auth/internal/db"
	"github.com/peltastic/payment-microservices-reference/auth/internal/domain"
	appLogger "github.com/peltastic/payment-microservices-reference/auth/internal/logger"
	"github.com/peltastic/payment-microservices-reference/auth/internal/logsafe"
	"gorm.io/gorm"
)

type KeyService struct {
	cacheRepo domain.CacheRepository
	keysRepo  domain.KeysRepository
	db        db.IDatabase
}

type ValidateKeyResult struct {
	MerchantID string
	Scope      string
	Valid      bool
}

func NewKeyService(cacheRepo domain.CacheRepository, keysRepo domain.KeysRepository, db db.IDatabase) *KeyService {
	slog.Default().With("component", "key_service").Info("key service initialized")
	return &KeyService{
		cacheRepo: cacheRepo,
		keysRepo:  keysRepo,
		db:        db,
	}
}

func (s *KeyService) logger(ctx context.Context) *slog.Logger {
	return appLogger.FromContext(ctx).With("component", "key_service")
}

func (s *KeyService) IsRevoked(ctx context.Context, keyHash string) (bool, error) {
	log := s.logger(ctx)
	log.Debug("checking api key revocation status", "key_hash_prefix", logsafe.ShortHash(keyHash))
	revokedKey, err := s.keysRepo.GetRevokedByHash(keyHash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Debug("api key is not revoked", "key_hash_prefix", logsafe.ShortHash(keyHash))
			return false, nil
		}
		log.Error("failed to check revoked api key", "key_hash_prefix", logsafe.ShortHash(keyHash), "error", err)
		return false, err
	}
	log.Warn("api key is revoked", "key_hash_prefix", logsafe.ShortHash(keyHash))
	return revokedKey != nil, nil
}

func (s *KeyService) ValidateKey(c context.Context, apiKey string) (*ValidateKeyResult, error) {
	log := s.logger(c)
	hash := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(hash[:])
	log.Info("validating api key", "key_hash_prefix", logsafe.ShortHash(keyHash))

	cacheKey := "keyhash:" + keyHash

	if cached, err := s.cacheRepo.Get(c, cacheKey); err == nil && cached != "" {
		log.Info("api key cache hit", "key_hash_prefix", logsafe.ShortHash(keyHash))
		var cachedResult struct {
			MerchantID string `json:"merchant_id"`
			Scope      string `json:"scope"`
		}

		if err := json.Unmarshal([]byte(cached), &cachedResult); err == nil && cachedResult.MerchantID != "" {
			if cachedResult.Scope == "" {
				cachedResult.Scope = "full"
			}
			log.Info("api key validated from cache",
				"merchant_id", cachedResult.MerchantID,
				"scope", cachedResult.Scope,
			)
			return &ValidateKeyResult{
				MerchantID: cachedResult.MerchantID,
				Scope:      cachedResult.Scope,
				Valid:      true,
			}, nil
		} else if err != nil {
			log.Error("failed to unmarshal api key cache entry",
				"key_hash_prefix", logsafe.ShortHash(keyHash),
				"error", err,
			)
		}
	} else if err != nil {
		log.Error("failed to read api key cache",
			"key_hash_prefix", logsafe.ShortHash(keyHash),
			"error", err,
		)
	} else {
		log.Debug("api key cache miss", "key_hash_prefix", logsafe.ShortHash(keyHash))
	}

	isKeyRevoked, err := s.IsRevoked(c, keyHash)
	if err != nil {
		log.Error("api key revocation check failed", "key_hash_prefix", logsafe.ShortHash(keyHash), "error", err)
		return nil, err
	}
	if isKeyRevoked {
		log.Warn("api key validation rejected because key is revoked", "key_hash_prefix", logsafe.ShortHash(keyHash))
		return &ValidateKeyResult{Valid: false}, domain.ErrKeyRevoked
	}

	log.Debug("loading api key from repository", "key_hash_prefix", logsafe.ShortHash(keyHash))
	keyData, err := s.keysRepo.GetByHash(keyHash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("api key validation rejected because key was not found", "key_hash_prefix", logsafe.ShortHash(keyHash))
			return &ValidateKeyResult{Valid: false}, domain.ErrInvalidKey
		}
		log.Error("failed to load api key by hash", "key_hash_prefix", logsafe.ShortHash(keyHash), "error", err)
		return nil, err
	}
	if !keyData.IsActive {
		log.Warn("api key validation rejected because key is inactive",
			"key_id", keyData.ID,
			"merchant_id", keyData.MerchantID,
		)
		return &ValidateKeyResult{Valid: false}, domain.ErrInvalidKey
	}
	if !keyData.ExpiresAt.IsZero() && !keyData.ExpiresAt.After(time.Now().UTC()) {
		log.Warn("api key validation rejected because key is expired",
			"key_id", keyData.ID,
			"merchant_id", keyData.MerchantID,
			"expires_at", keyData.ExpiresAt,
		)
		return &ValidateKeyResult{Valid: false}, domain.ErrInvalidKey
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.keysRepo.UpdateLastUsedAt(ctx, keyData.ID); err != nil {
			log.Error("failed to update api key last used timestamp",
				"key_id", keyData.ID,
				"merchant_id", keyData.MerchantID,
				"error", err,
			)
			return
		}
		log.Debug("api key last used timestamp updated",
			"key_id", keyData.ID,
			"merchant_id", keyData.MerchantID,
		)
	}()

	serialized, err := json.Marshal(map[string]interface{}{
		"merchant_id": keyData.MerchantID,
		"scope":       keyData.Scope,
	})
	if err != nil {
		log.Error("failed to serialize api key cache entry",
			"key_id", keyData.ID,
			"merchant_id", keyData.MerchantID,
			"error", err,
		)
		return nil, err
	}
	cacheTTL := keyValidationCacheTTL(keyData.ExpiresAt)
	if err := s.cacheRepo.Set(c, cacheKey, string(serialized), cacheTTL); err != nil {
		log.Error("failed to cache api key validation result",
			"key_id", keyData.ID,
			"merchant_id", keyData.MerchantID,
			"error", err,
		)
	} else {
		log.Debug("api key validation result cached",
			"key_id", keyData.ID,
			"merchant_id", keyData.MerchantID,
			"ttl_seconds", int(cacheTTL.Seconds()),
		)
	}

	log.Info("api key validated",
		"key_id", keyData.ID,
		"merchant_id", keyData.MerchantID,
		"scope", keyData.Scope,
	)

	return &ValidateKeyResult{
		MerchantID: keyData.MerchantID,
		Scope:      keyData.Scope,
		Valid:      true,
	}, nil

}

func keyValidationCacheTTL(expiresAt time.Time) time.Duration {
	const fallback = 30 * time.Second
	if expiresAt.IsZero() {
		return fallback
	}

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return time.Second
	}
	if ttl < fallback {
		return ttl
	}
	return fallback
}

func (s *KeyService) RevokeKey(c context.Context, keyID string, reason string) error {
	log := s.logger(c)
	log.Info("revoking api key", "key_id", keyID, "reason", reason)
	key, err := s.keysRepo.FindByID(keyID)

	if err != nil {
		log.Error("failed to find api key for revocation", "key_id", keyID, "error", err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}

	revokedKey := &domain.RevokedKeys{
		KeyHash:   key.KeyHash,
		Reason:    reason,
		RevokedAt: time.Now().UTC(),
	}
	err = s.db.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := s.keysRepo.WithTx(tx).DeactivateKey(keyID); err != nil {
			log.Error("failed to deactivate api key",
				"key_id", keyID,
				"merchant_id", key.MerchantID,
				"error", err,
			)
			return err
		}

		if err := s.keysRepo.WithTx(tx).CreateRevokedKey(revokedKey); err != nil {
			log.Error("failed to create revoked api key record",
				"key_id", keyID,
				"merchant_id", key.MerchantID,
				"error", err,
			)
			return err
		}

		return nil
	})

	if err != nil {
		log.Error("api key revocation transaction failed",
			"key_id", keyID,
			"merchant_id", key.MerchantID,
			"error", err,
		)
		return err
	}

	cacheKey := "keyhash:" + key.KeyHash
	if err := s.cacheRepo.Delete(c, cacheKey); err != nil {
		log.Error("failed to delete api key validation cache",
			"key_id", keyID,
			"merchant_id", key.MerchantID,
			"error", err,
		)
	} else {
		log.Debug("api key validation cache deleted",
			"key_id", keyID,
			"merchant_id", key.MerchantID,
		)
	}

	log.Info("api key revoked",
		"key_id", keyID,
		"merchant_id", key.MerchantID,
		"reason", reason,
	)

	return nil
}
