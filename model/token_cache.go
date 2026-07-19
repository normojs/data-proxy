package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/cachex"

	"github.com/samber/hot"
)

const tokenCacheNamespace = "token:v1"

var (
	tokenMemCacheOnce sync.Once
	tokenMemCache     *cachex.HybridCache[Token]
)

// Process-local token cache (HMAC of key as id). Redis HASH path is unchanged
// when Redis is enabled so HINCRBY on remain_quota keeps working.
func getTokenMemCache() *cachex.HybridCache[Token] {
	tokenMemCacheOnce.Do(func() {
		ttl := time.Duration(common.RedisKeyCacheSeconds()) * time.Second
		if ttl <= 0 {
			ttl = 60 * time.Second
		}
		tokenMemCache = cachex.NewHybridCache[Token](cachex.HybridCacheConfig[Token]{
			Namespace:    cachex.Namespace(tokenCacheNamespace),
			RedisEnabled: func() bool { return false },
			RedisCodec:   cachex.JSONCodec[Token]{},
			Memory: func() *hot.HotCache[string, Token] {
				return hot.NewHotCache[string, Token](hot.LRU, 50_000).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return tokenMemCache
}

func tokenCacheTTL() time.Duration {
	sec := common.RedisKeyCacheSeconds()
	if sec <= 0 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}

func tokenCacheRedisKey(hmacKey string) string {
	return fmt.Sprintf("token:%s", hmacKey)
}

func shouldUpdateTokenCache(fromDB bool, err error) bool {
	return fromDB && err == nil
}

func cacheSetToken(token Token) error {
	rawKey := token.Key
	hmacKey := common.GenerateHMAC(rawKey)
	token.Clean()
	if common.RedisEnabled {
		return common.RedisHSetObj(tokenCacheRedisKey(hmacKey), &token, tokenCacheTTL())
	}
	// store without raw secret; restore Key on get from caller-provided key
	return getTokenMemCache().SetWithTTL(hmacKey, token, tokenCacheTTL())
}

func cacheDeleteToken(key string) error {
	hmacKey := common.GenerateHMAC(key)
	if common.RedisEnabled {
		return common.RedisDelKey(tokenCacheRedisKey(hmacKey))
	}
	_, err := getTokenMemCache().DeleteMany([]string{hmacKey})
	return err
}

func cacheIncrTokenQuota(key string, increment int64) error {
	hmacKey := common.GenerateHMAC(key)
	if common.RedisEnabled {
		return common.RedisHIncrBy(tokenCacheRedisKey(hmacKey), constant.TokenFiledRemainQuota, increment)
	}
	v, found, err := getTokenMemCache().Get(hmacKey)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	v.RemainQuota += int(increment)
	return getTokenMemCache().SetWithTTL(hmacKey, v, tokenCacheTTL())
}

func cacheDecrTokenQuota(key string, decrement int64) error {
	return cacheIncrTokenQuota(key, -decrement)
}

func cacheSetTokenField(key string, field string, value string) error {
	hmacKey := common.GenerateHMAC(key)
	if common.RedisEnabled {
		return common.RedisHSetField(tokenCacheRedisKey(hmacKey), field, value)
	}
	// Field patches are Redis-oriented; for memory refresh whole object from DB later.
	// Best-effort: only remain_quota / status commonly patched — invalidate on unknown.
	v, found, err := getTokenMemCache().Get(hmacKey)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	switch field {
	case constant.TokenFiledRemainQuota:
		var n int
		if _, err := fmt.Sscanf(value, "%d", &n); err == nil {
			v.RemainQuota = n
		}
	case "Status":
		var n int
		if _, err := fmt.Sscanf(value, "%d", &n); err == nil {
			v.Status = n
		}
	default:
		// drop entry so next read reloads from DB
		_, _ = getTokenMemCache().DeleteMany([]string{hmacKey})
		return nil
	}
	return getTokenMemCache().SetWithTTL(hmacKey, v, tokenCacheTTL())
}

// cacheGetTokenByKey loads token from Redis HASH or process-local cache.
// Restores the plaintext key on the returned object (not stored in cache).
func cacheGetTokenByKey(key string) (*Token, error) {
	hmacKey := common.GenerateHMAC(key)
	if common.RedisEnabled {
		var token Token
		err := common.RedisHGetObj(tokenCacheRedisKey(hmacKey), &token)
		if err != nil {
			return nil, err
		}
		token.Key = key
		return &token, nil
	}
	v, found, err := getTokenMemCache().Get(hmacKey)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("token cache miss")
	}
	v.Key = key
	return &v, nil
}
