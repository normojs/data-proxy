package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/cachex"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/samber/hot"
)

// UserBase struct remains the same as it represents the cached data structure
type UserBase struct {
	Id          int    `json:"id"`
	Group       string `json:"group"`
	Email       string `json:"email"`
	Quota       int    `json:"quota"`
	Status      int    `json:"status"`
	Username    string `json:"username"`
	Setting     string `json:"setting"`
	TokenGroups string `json:"token_groups"`
}

func (user *UserBase) WriteContext(c *gin.Context) {
	common.SetContextKey(c, constant.ContextKeyUserGroup, user.Group)
	common.SetContextKey(c, constant.ContextKeyUserQuota, user.Quota)
	common.SetContextKey(c, constant.ContextKeyUserStatus, user.Status)
	common.SetContextKey(c, constant.ContextKeyUserEmail, user.Email)
	common.SetContextKey(c, constant.ContextKeyUserName, user.Username)
	common.SetContextKey(c, constant.ContextKeyUserSetting, user.GetSetting())
	common.SetContextKey(c, constant.ContextKeyUserTokenGroups, user.GetTokenGroups())
}

func (user *UserBase) GetSetting() dto.UserSetting {
	setting := dto.UserSetting{}
	if user.Setting != "" {
		err := common.Unmarshal([]byte(user.Setting), &setting)
		if err != nil {
			common.SysLog("failed to unmarshal setting: " + err.Error())
		}
	}
	return setting
}

func (user *UserBase) GetTokenGroups() []string {
	if user == nil {
		return []string{}
	}
	return ParseTokenGroups(user.TokenGroups)
}

const userBaseCacheNamespace = "user_base:v1"

var (
	userBaseMemCacheOnce sync.Once
	userBaseMemCache     *cachex.HybridCache[UserBase]
)

// getUserBaseMemCache is process-local only (HybridCache with Redis disabled).
// When Redis is enabled, user cache still uses legacy Redis HASH helpers so
// HINCRBY on Quota and field patches keep working.
func getUserBaseMemCache() *cachex.HybridCache[UserBase] {
	userBaseMemCacheOnce.Do(func() {
		ttl := time.Duration(common.RedisKeyCacheSeconds()) * time.Second
		if ttl <= 0 {
			ttl = 60 * time.Second
		}
		userBaseMemCache = cachex.NewHybridCache[UserBase](cachex.HybridCacheConfig[UserBase]{
			Namespace: cachex.Namespace(userBaseCacheNamespace),
			// Force memory path even if common.RDB is non-nil during tests.
			RedisEnabled: func() bool { return false },
			RedisCodec:   cachex.JSONCodec[UserBase]{},
			Memory: func() *hot.HotCache[string, UserBase] {
				return hot.NewHotCache[string, UserBase](hot.LRU, 50_000).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return userBaseMemCache
}

func userCacheTTL() time.Duration {
	sec := common.RedisKeyCacheSeconds()
	if sec <= 0 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}

// getUserCacheKey returns the key for user cache (Redis hash key / mem raw key)
func getUserCacheKey(userId int) string {
	return fmt.Sprintf("user:%d", userId)
}

func shouldUpdateUserCache(fromDB bool, err error) bool {
	return fromDB && err == nil
}

// invalidateUserCache clears user cache
func invalidateUserCache(userId int) error {
	if common.RedisEnabled {
		return common.RedisDelKey(getUserCacheKey(userId))
	}
	_, err := getUserBaseMemCache().DeleteMany([]string{getUserCacheKey(userId)})
	return err
}

// InvalidateUserCache is the exported version of invalidateUserCache.
// 供 controller 等上层包在用户状态变更（如禁用、删除、角色变更）后主动清理缓存。
func InvalidateUserCache(userId int) error {
	return invalidateUserCache(userId)
}

// updateUserCache updates all user cache fields
func updateUserCache(user User) error {
	base := user.ToBaseUser()
	if base == nil {
		return nil
	}
	if common.RedisEnabled {
		return common.RedisHSetObj(
			getUserCacheKey(user.Id),
			base,
			userCacheTTL(),
		)
	}
	return getUserBaseMemCache().SetWithTTL(getUserCacheKey(user.Id), *base, userCacheTTL())
}

// GetUserCache gets complete user cache (Redis HASH or process-local HybridCache)
func GetUserCache(userId int) (userCache *UserBase, err error) {
	var user *User
	var fromDB bool
	defer func() {
		if shouldUpdateUserCache(fromDB, err) && user != nil {
			gopool.Go(func() {
				if err := updateUserCache(*user); err != nil {
					common.SysLog("failed to update user status cache: " + err.Error())
				}
			})
		}
	}()

	userCache, err = cacheGetUserBase(userId)
	if err == nil {
		return userCache, nil
	}

	fromDB = true
	user, err = GetUserById(userId, false)
	if err != nil {
		return nil, err
	}

	userCache = &UserBase{
		Id:          user.Id,
		Group:       user.Group,
		Quota:       user.Quota,
		Status:      user.Status,
		Username:    user.Username,
		Setting:     user.Setting,
		Email:       user.Email,
		TokenGroups: user.TokenGroups,
	}

	return userCache, nil
}

func cacheGetUserBase(userId int) (*UserBase, error) {
	if common.RedisEnabled {
		var userCache UserBase
		err := common.RedisHGetObj(getUserCacheKey(userId), &userCache)
		if err != nil {
			return nil, err
		}
		return &userCache, nil
	}

	v, found, err := getUserBaseMemCache().Get(getUserCacheKey(userId))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("user cache miss")
	}
	// return a copy pointer so callers cannot mutate the hot entry in place
	cp := v
	return &cp, nil
}

func patchUserBaseCache(userId int, patch func(*UserBase)) error {
	if common.RedisEnabled {
		return fmt.Errorf("patchUserBaseCache is memory-only")
	}
	key := getUserCacheKey(userId)
	v, found, err := getUserBaseMemCache().Get(key)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	patch(&v)
	return getUserBaseMemCache().SetWithTTL(key, v, userCacheTTL())
}

// Add atomic quota operations using hash fields (Redis) or RMW on memory entry
func cacheIncrUserQuota(userId int, delta int64) error {
	if common.RedisEnabled {
		return common.RedisHIncrBy(getUserCacheKey(userId), "Quota", delta)
	}
	return patchUserBaseCache(userId, func(u *UserBase) {
		u.Quota += int(delta)
	})
}

func cacheDecrUserQuota(userId int, delta int64) error {
	return cacheIncrUserQuota(userId, -delta)
}

// Helper functions to get individual fields if needed
func getUserGroupCache(userId int) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Group, nil
}

func getUserQuotaCache(userId int) (int, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return 0, err
	}
	return cache.Quota, nil
}

func getUserStatusCache(userId int) (int, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return 0, err
	}
	return cache.Status, nil
}

func getUserNameCache(userId int) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Username, nil
}

func getUserSettingCache(userId int) (dto.UserSetting, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return dto.UserSetting{}, err
	}
	return cache.GetSetting(), nil
}

// New functions for individual field updates
func updateUserStatusCache(userId int, status bool) error {
	statusInt := common.UserStatusEnabled
	if !status {
		statusInt = common.UserStatusDisabled
	}
	if common.RedisEnabled {
		return common.RedisHSetField(getUserCacheKey(userId), "Status", fmt.Sprintf("%d", statusInt))
	}
	return patchUserBaseCache(userId, func(u *UserBase) {
		u.Status = statusInt
	})
}

func updateUserQuotaCache(userId int, quota int) error {
	if common.RedisEnabled {
		return common.RedisHSetField(getUserCacheKey(userId), "Quota", fmt.Sprintf("%d", quota))
	}
	return patchUserBaseCache(userId, func(u *UserBase) {
		u.Quota = quota
	})
}

func updateUserGroupCache(userId int, group string) error {
	if common.RedisEnabled {
		return common.RedisHSetField(getUserCacheKey(userId), "Group", group)
	}
	return patchUserBaseCache(userId, func(u *UserBase) {
		u.Group = group
	})
}

func UpdateUserGroupCache(userId int, group string) error {
	return updateUserGroupCache(userId, group)
}

func updateUserNameCache(userId int, username string) error {
	if common.RedisEnabled {
		return common.RedisHSetField(getUserCacheKey(userId), "Username", username)
	}
	return patchUserBaseCache(userId, func(u *UserBase) {
		u.Username = username
	})
}

func updateUserSettingCache(userId int, setting string) error {
	if common.RedisEnabled {
		return common.RedisHSetField(getUserCacheKey(userId), "Setting", setting)
	}
	return patchUserBaseCache(userId, func(u *UserBase) {
		u.Setting = setting
	})
}

// GetUserLanguage returns the user's language preference from cache
// Uses the existing GetUserCache mechanism for efficiency
func GetUserLanguage(userId int) string {
	userCache, err := GetUserCache(userId)
	if err != nil {
		return ""
	}
	return userCache.GetSetting().Language
}
