package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestTokenMemoryCacheRoundTrip(t *testing.T) {
	origRedis := common.RedisEnabled
	origSecret := common.CryptoSecret
	common.RedisEnabled = false
	common.CryptoSecret = "test-crypto-secret-for-token-cache"
	t.Cleanup(func() {
		common.RedisEnabled = origRedis
		common.CryptoSecret = origSecret
		_ = getTokenMemCache().Purge()
	})

	rawKey := "sk-lite-token-cache-test-key-123456"
	tok := Token{
		Id:             7,
		UserId:         1,
		Key:            rawKey,
		Status:         common.TokenStatusEnabled,
		Name:           "lite",
		RemainQuota:    500,
		UnlimitedQuota: false,
		Group:          "default",
	}
	if err := cacheSetToken(tok); err != nil {
		t.Fatalf("cacheSetToken: %v", err)
	}

	got, err := cacheGetTokenByKey(rawKey)
	if err != nil {
		t.Fatalf("cacheGetTokenByKey: %v", err)
	}
	if got.Key != rawKey {
		t.Fatalf("key restored want %q got %q", rawKey, got.Key)
	}
	if got.RemainQuota != 500 || got.Name != "lite" {
		t.Fatalf("unexpected token: %+v", got)
	}

	if err := cacheIncrTokenQuota(rawKey, 25); err != nil {
		t.Fatalf("incr: %v", err)
	}
	got, err = cacheGetTokenByKey(rawKey)
	if err != nil {
		t.Fatalf("after incr: %v", err)
	}
	if got.RemainQuota != 525 {
		t.Fatalf("quota want 525 got %d", got.RemainQuota)
	}

	if err := cacheDeleteToken(rawKey); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := cacheGetTokenByKey(rawKey); err == nil {
		t.Fatal("expected miss after delete")
	}
}

func TestShouldUpdateTokenCache(t *testing.T) {
	if !shouldUpdateTokenCache(true, nil) {
		t.Fatal("expected true")
	}
	if shouldUpdateTokenCache(false, nil) {
		t.Fatal("expected false on cache hit path")
	}
}
