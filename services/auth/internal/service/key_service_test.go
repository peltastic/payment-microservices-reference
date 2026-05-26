package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/peltastic/payment-microservices-reference/auth/internal/domain"
	"gorm.io/gorm"
)

func TestValidateKey_GivenCachedKey_WhenValidated_ThenReturnsCachedResultWithoutRepositoryLookup(t *testing.T) {
	// Arrange
	cache := &fakeAuthCacheRepository{
		getValue: `{"merchant_id":"mrc_cached","scope":"payments:write"}`,
	}
	keys := &fakeAuthKeysRepository{}
	service := NewKeyService(cache, keys, nil)

	// Act
	result, err := service.ValidateKey(context.Background(), "pk_test_cached")

	// Assert
	if err != nil {
		t.Fatalf("expected cached key to validate without error: %v", err)
	}
	if result == nil || !result.Valid {
		t.Fatalf("expected cached key to be valid, got %#v", result)
	}
	if result.MerchantID != "mrc_cached" || result.Scope != "payments:write" {
		t.Fatalf("unexpected cached validation result: %#v", result)
	}
	if keys.getByHashCalls != 0 || keys.getRevokedByHashCalls != 0 {
		t.Fatalf("expected repository not to be queried on cache hit, got get=%d revoked=%d", keys.getByHashCalls, keys.getRevokedByHashCalls)
	}
}

func TestValidateKey_GivenRevokedKey_WhenValidated_ThenReturnsKeyRevoked(t *testing.T) {
	// Arrange
	apiKey := "pk_test_revoked"
	keyHash := hashAPIKey(apiKey)
	cache := &fakeAuthCacheRepository{}
	keys := &fakeAuthKeysRepository{
		revokedByHash: map[string]*domain.RevokedKeys{
			keyHash: {KeyHash: keyHash, Reason: "compromised"},
		},
	}
	service := NewKeyService(cache, keys, nil)

	// Act
	result, err := service.ValidateKey(context.Background(), apiKey)

	// Assert
	if !errors.Is(err, domain.ErrKeyRevoked) {
		t.Fatalf("expected ErrKeyRevoked, got result=%#v err=%v", result, err)
	}
	if result == nil || result.Valid {
		t.Fatalf("expected invalid validation result for revoked key, got %#v", result)
	}
	if keys.getByHashCalls != 0 {
		t.Fatalf("expected active key lookup not to run for revoked key, got %d calls", keys.getByHashCalls)
	}
}

func TestValidateKey_GivenActiveKey_WhenValidated_ThenUpdatesLastUsedWithoutBlockingResponse(t *testing.T) {
	// Arrange
	apiKey := "pk_test_active"
	keyHash := hashAPIKey(apiKey)
	updateStarted := make(chan struct{}, 1)
	releaseUpdate := make(chan struct{})
	cache := &fakeAuthCacheRepository{}
	keys := &fakeAuthKeysRepository{
		keysByHash: map[string]*domain.Keys{
			keyHash: {
				ID:         "key_active",
				MerchantID: "mrc_active",
				KeyHash:    keyHash,
				Scope:      "full",
				IsActive:   true,
				ExpiresAt:  time.Now().UTC().Add(time.Hour),
			},
		},
		updateStarted: updateStarted,
		releaseUpdate: releaseUpdate,
	}
	service := NewKeyService(cache, keys, nil)
	defer close(releaseUpdate)

	// Act
	result, err := service.ValidateKey(context.Background(), apiKey)

	// Assert
	if err != nil {
		t.Fatalf("expected active key to validate without error: %v", err)
	}
	if result == nil || !result.Valid || result.MerchantID != "mrc_active" {
		t.Fatalf("unexpected validation result: %#v", result)
	}
	select {
	case <-updateStarted:
	case <-time.After(time.Second):
		t.Fatal("expected last-used update goroutine to start")
	}
}

func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

type fakeAuthCacheRepository struct {
	getValue   string
	getErr     error
	setCalls   int
	deleteKeys []string
}

func (f *fakeAuthCacheRepository) Set(_ context.Context, _ string, _ interface{}, _ time.Duration) error {
	f.setCalls++
	return nil
}

func (f *fakeAuthCacheRepository) Get(_ context.Context, _ string) (string, error) {
	return f.getValue, f.getErr
}

func (f *fakeAuthCacheRepository) Delete(_ context.Context, key string) error {
	f.deleteKeys = append(f.deleteKeys, key)
	return nil
}

type fakeAuthKeysRepository struct {
	keysByHash            map[string]*domain.Keys
	revokedByHash         map[string]*domain.RevokedKeys
	getByHashCalls        int
	getRevokedByHashCalls int
	updateStarted         chan struct{}
	releaseUpdate         chan struct{}
}

func (f *fakeAuthKeysRepository) Create(_ *domain.Keys) error {
	return nil
}

func (f *fakeAuthKeysRepository) UpdateLastUsedAt(ctx context.Context, _ string) error {
	if f.updateStarted != nil {
		f.updateStarted <- struct{}{}
	}
	if f.releaseUpdate != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-f.releaseUpdate:
		}
	}
	return nil
}

func (f *fakeAuthKeysRepository) GetByHash(keyHash string) (*domain.Keys, error) {
	f.getByHashCalls++
	key := f.keysByHash[keyHash]
	if key == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return key, nil
}

func (f *fakeAuthKeysRepository) WithTx(_ *gorm.DB) domain.KeysRepository {
	return f
}

func (f *fakeAuthKeysRepository) GetRevokedByHash(keyHash string) (*domain.RevokedKeys, error) {
	f.getRevokedByHashCalls++
	revoked := f.revokedByHash[keyHash]
	if revoked == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return revoked, nil
}

func (f *fakeAuthKeysRepository) CreateRevokedKey(_ *domain.RevokedKeys) error {
	return nil
}

func (f *fakeAuthKeysRepository) FindByID(_ string) (*domain.Keys, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAuthKeysRepository) DeactivateKey(_ string) error {
	return nil
}
