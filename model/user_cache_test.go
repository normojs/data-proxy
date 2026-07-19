package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestUserBaseMemoryCacheRoundTrip(t *testing.T) {
	origRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = origRedis
		_ = getUserBaseMemCache().Purge()
	})

	user := User{
		Id:       4242,
		Group:    "default",
		Quota:    1000,
		Status:   common.UserStatusEnabled,
		Username: "lite-user",
		Email:    "lite@example.com",
		Setting:  `{"language":"zh"}`,
	}
	if err := updateUserCache(user); err != nil {
		t.Fatalf("updateUserCache: %v", err)
	}

	got, err := cacheGetUserBase(user.Id)
	if err != nil {
		t.Fatalf("cacheGetUserBase: %v", err)
	}
	if got.Username != "lite-user" || got.Quota != 1000 || got.Group != "default" {
		t.Fatalf("unexpected cache payload: %+v", got)
	}

	if err := cacheIncrUserQuota(user.Id, 50); err != nil {
		t.Fatalf("cacheIncrUserQuota: %v", err)
	}
	got, err = cacheGetUserBase(user.Id)
	if err != nil {
		t.Fatalf("after incr: %v", err)
	}
	if got.Quota != 1050 {
		t.Fatalf("quota want 1050 got %d", got.Quota)
	}

	if err := updateUserGroupCache(user.Id, "vip"); err != nil {
		t.Fatalf("updateUserGroupCache: %v", err)
	}
	got, err = cacheGetUserBase(user.Id)
	if err != nil {
		t.Fatalf("after group: %v", err)
	}
	if got.Group != "vip" {
		t.Fatalf("group want vip got %q", got.Group)
	}

	if err := invalidateUserCache(user.Id); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	if _, err := cacheGetUserBase(user.Id); err == nil {
		t.Fatal("expected miss after invalidate")
	}
}

func TestShouldUpdateUserCacheWithoutRedis(t *testing.T) {
	if !shouldUpdateUserCache(true, nil) {
		t.Fatal("DB success should refresh memory cache")
	}
	if shouldUpdateUserCache(false, nil) {
		t.Fatal("cache hit path should not force write")
	}
	if shouldUpdateUserCache(true, errors.New("x")) {
		t.Fatal("errors should not write")
	}
}
