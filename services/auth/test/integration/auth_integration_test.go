//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	authdb "github.com/peltastic/payment-microservices-reference/auth/internal/db"
	"github.com/peltastic/payment-microservices-reference/auth/internal/domain"
	"github.com/peltastic/payment-microservices-reference/auth/internal/repository"
	"github.com/peltastic/payment-microservices-reference/auth/internal/service"
)

func TestMerchantService_GivenValidMerchantAndKey_WhenCreated_ThenCommitsBothRows(t *testing.T) {

	database := openAuthIntegrationDB(t)
	keysRepo := repository.NewKeysRepository(database)
	merchantsRepo := repository.NewMerchantsRepository(database)
	merchantService := service.NewMerchantService(database, keysRepo, merchantsRepo)
	email := fmt.Sprintf("merchant-%s@example.com", ulid.Make().String())

	apiKey, err := merchantService.CreateMerchant(context.Background(), "Integration Merchant", email, "payments:write", nil)

	if err != nil {
		t.Fatalf("expected merchant and key creation to commit: %v", err)
	}
	if !strings.HasPrefix(apiKey, "pk_live_") {
		t.Fatalf("expected generated api key with live prefix, got %q", apiKey)
	}

	var merchant domain.Merchant
	if err := database.GetDB().Where("email = ?", email).First(&merchant).Error; err != nil {
		t.Fatalf("expected merchant row to be committed: %v", err)
	}

	var key domain.Keys
	if err := database.GetDB().Where("merchant_id = ?", merchant.ID).First(&key).Error; err != nil {
		t.Fatalf("expected api key row to be committed: %v", err)
	}
	if key.Scope != "payments:write" {
		t.Fatalf("expected key scope payments:write, got %q", key.Scope)
	}
}

func TestKeyService_GivenActiveKey_WhenRevoked_ThenDeactivatesKeyCreatesRevocationAndInvalidatesCache(t *testing.T) {

	database := openAuthIntegrationDB(t)
	keysRepo := repository.NewKeysRepository(database)
	cache := &authIntegrationCache{}
	keyService := service.NewKeyService(cache, keysRepo, database)
	merchantID := ulid.Make().String()
	keyID := ulid.Make().String()
	keyHash := "2f73c3935c9f0a7f7af42d54f9f974bf4b48fa7ad12708f2a17ad03a15cd8021"

	if err := database.GetDB().Create(&domain.Merchant{
		ID:    merchantID,
		Name:  "Integration Merchant",
		Email: fmt.Sprintf("revoke-%s@example.com", merchantID),
	}).Error; err != nil {
		t.Fatalf("failed to seed merchant: %v", err)
	}
	if err := database.GetDB().Create(&domain.Keys{
		ID:         keyID,
		MerchantID: merchantID,
		KeyHash:    keyHash,
		KeyPrefix:  "pk_live_",
		Scope:      "full",
		IsActive:   true,
		ExpiresAt:  time.Now().UTC().Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("failed to seed key: %v", err)
	}

	err := keyService.RevokeKey(context.Background(), keyID, "rotation")

	if err != nil {
		t.Fatalf("expected key revocation to commit: %v", err)
	}
	var key domain.Keys
	if err := database.GetDB().Where("id = ?", keyID).First(&key).Error; err != nil {
		t.Fatalf("expected key row to remain queryable: %v", err)
	}
	if key.IsActive {
		t.Fatal("expected revoked key to be deactivated")
	}
	var revoked domain.RevokedKeys
	if err := database.GetDB().Where("key_hash = ?", keyHash).First(&revoked).Error; err != nil {
		t.Fatalf("expected revoked key row to be created: %v", err)
	}
	if len(cache.deletedKeys) != 1 || cache.deletedKeys[0] != "keyhash:"+keyHash {
		t.Fatalf("expected validation cache to be invalidated, got %#v", cache.deletedKeys)
	}
}

func openAuthIntegrationDB(t *testing.T) authdb.IDatabase {
	t.Helper()
	dsn := os.Getenv("PAYMENT_REFERENCE_AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PAYMENT_REFERENCE_AUTH_TEST_DATABASE_URL is required for auth integration tests")
	}

	database, err := authdb.NewDatabase(dsn)
	if err != nil {
		t.Fatalf("failed to open auth integration database: %v", err)
	}
	sqlDB, err := database.GetDB().DB()
	if err != nil {
		t.Fatalf("failed to get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	schema := "auth_it_" + ulid.Make().String()
	if err := database.GetDB().Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}
	if err := database.GetDB().Exec(`SET search_path TO "` + schema + `"`).Error; err != nil {
		t.Fatalf("failed to set test schema: %v", err)
	}
	if err := database.GetDB().AutoMigrate(&domain.Merchant{}, &domain.Keys{}, &domain.RevokedKeys{}); err != nil {
		t.Fatalf("failed to migrate auth integration schema: %v", err)
	}

	t.Cleanup(func() {
		_ = database.GetDB().Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`).Error
		_ = sqlDB.Close()
	})

	return database
}

type authIntegrationCache struct {
	deletedKeys []string
}

func (c *authIntegrationCache) Set(_ context.Context, _ string, _ interface{}, _ time.Duration) error {
	return nil
}

func (c *authIntegrationCache) Get(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (c *authIntegrationCache) Delete(_ context.Context, key string) error {
	c.deletedKeys = append(c.deletedKeys, key)
	return nil
}
