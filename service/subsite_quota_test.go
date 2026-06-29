package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPreCheckSubsiteQuotaRejectsExceededLimits(t *testing.T) {
	tests := []struct {
		name       string
		policy     model.SubsiteQuotaPolicy
		counter    model.SubsiteQuotaCounter
		estimate   int
		wantStatus int
		wantCode   string
	}{
		{
			name:       "site daily quota",
			policy:     model.SubsiteQuotaPolicy{SiteDailyQuota: 10},
			counter:    model.SubsiteQuotaCounter{UserId: 0, Scope: model.SubsiteCounterScopeSite, WindowType: model.SubsiteCounterWindowDaily, UsedQuota: 9},
			estimate:   2,
			wantStatus: http.StatusForbidden,
			wantCode:   model.SubsiteAccessCodeQuota,
		},
		{
			name:       "user daily quota",
			policy:     model.SubsiteQuotaPolicy{UserDailyQuota: 10},
			counter:    model.SubsiteQuotaCounter{UserId: 7, Scope: model.SubsiteCounterScopeUser, WindowType: model.SubsiteCounterWindowDaily, UsedQuota: 10},
			estimate:   1,
			wantStatus: http.StatusForbidden,
			wantCode:   model.SubsiteAccessCodeUserQuota,
		},
		{
			name:       "site daily requests",
			policy:     model.SubsiteQuotaPolicy{SiteDailyRequestLimit: 1},
			counter:    model.SubsiteQuotaCounter{UserId: 0, Scope: model.SubsiteCounterScopeSite, WindowType: model.SubsiteCounterWindowDaily, RequestCount: 1},
			estimate:   0,
			wantStatus: http.StatusTooManyRequests,
			wantCode:   model.SubsiteAccessCodeRateLimited,
		},
		{
			name:       "user rolling requests",
			policy:     model.SubsiteQuotaPolicy{UserWindowRequestLimit: 1, UserWindowSeconds: 3600},
			counter:    model.SubsiteQuotaCounter{UserId: 7, Scope: model.SubsiteCounterScopeUser, WindowType: model.SubsiteCounterWindowRolling, RequestCount: 1},
			estimate:   0,
			wantStatus: http.StatusTooManyRequests,
			wantCode:   model.SubsiteAccessCodeRateLimited,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupSubsiteQuotaServiceTestDB(t)
			subsiteId := int64(88)
			now := common.GetTimestamp()
			dailyStart, dailyEnd := dailyWindow(now)
			require.NoError(t, model.DB.Create(&model.SubsiteQuotaPolicy{
				SubsiteId:              subsiteId,
				SiteDailyQuota:         tt.policy.SiteDailyQuota,
				SiteWindowQuota:        tt.policy.SiteWindowQuota,
				UserDailyQuota:         tt.policy.UserDailyQuota,
				UserWindowQuota:        tt.policy.UserWindowQuota,
				SiteDailyRequestLimit:  tt.policy.SiteDailyRequestLimit,
				SiteWindowRequestLimit: tt.policy.SiteWindowRequestLimit,
				UserDailyRequestLimit:  tt.policy.UserDailyRequestLimit,
				UserWindowRequestLimit: tt.policy.UserWindowRequestLimit,
				SiteWindowSeconds:      tt.policy.SiteWindowSeconds,
				UserWindowSeconds:      tt.policy.UserWindowSeconds,
			}).Error)
			counter := tt.counter
			counter.SubsiteId = subsiteId
			if counter.WindowType == model.SubsiteCounterWindowRolling {
				counter.WindowStart = now - 60
				counter.WindowEnd = now + 60
			} else {
				counter.WindowStart = dailyStart
				counter.WindowEnd = dailyEnd
			}
			require.NoError(t, model.DB.Create(&counter).Error)

			ctx := newSubsiteQuotaTestContext(subsiteId, 7)
			err := PreCheckSubsiteQuota(ctx, tt.estimate)

			require.NotNil(t, err)
			require.Equal(t, tt.wantStatus, err.StatusCode)
			require.Equal(t, tt.wantCode, string(err.GetErrorCode()))
		})
	}
}

func TestSettleSubsiteQuotaUsageUpdatesCountersOnce(t *testing.T) {
	setupSubsiteQuotaServiceTestDB(t)
	subsiteId := int64(91)
	userId := 7
	require.NoError(t, model.DB.Create(&model.SubsiteQuotaPolicy{
		SubsiteId:         subsiteId,
		SiteWindowSeconds: 3600,
		UserWindowSeconds: 1800,
	}).Error)

	ctx := newSubsiteQuotaTestContext(subsiteId, userId)
	relayInfo := &relaycommon.RelayInfo{UserId: userId}
	require.NoError(t, SettleSubsiteQuotaUsage(ctx, relayInfo, 12))
	require.NoError(t, SettleSubsiteQuotaUsage(ctx, relayInfo, 12))

	var counters []model.SubsiteQuotaCounter
	require.NoError(t, model.DB.Where("subsite_id = ?", subsiteId).Find(&counters).Error)
	require.Len(t, counters, 4)

	siteDaily := requireSubsiteQuotaCounter(t, subsiteId, 0, model.SubsiteCounterScopeSite, model.SubsiteCounterWindowDaily)
	require.Equal(t, 12, siteDaily.UsedQuota)
	require.Equal(t, 1, siteDaily.RequestCount)

	userDaily := requireSubsiteQuotaCounter(t, subsiteId, userId, model.SubsiteCounterScopeUser, model.SubsiteCounterWindowDaily)
	require.Equal(t, 12, userDaily.UsedQuota)
	require.Equal(t, 1, userDaily.RequestCount)

	siteRolling := requireSubsiteQuotaCounter(t, subsiteId, 0, model.SubsiteCounterScopeSite, model.SubsiteCounterWindowRolling)
	require.Equal(t, 12, siteRolling.UsedQuota)
	require.Equal(t, 1, siteRolling.RequestCount)
	require.Greater(t, siteRolling.WindowEnd, siteRolling.WindowStart)

	userRolling := requireSubsiteQuotaCounter(t, subsiteId, userId, model.SubsiteCounterScopeUser, model.SubsiteCounterWindowRolling)
	require.Equal(t, 12, userRolling.UsedQuota)
	require.Equal(t, 1, userRolling.RequestCount)
	require.Greater(t, userRolling.WindowEnd, userRolling.WindowStart)
}

func TestSubsiteQuotaRedisReservationSettleUsesActualQuota(t *testing.T) {
	setupSubsiteQuotaServiceTestDB(t)
	fake := useFakeSubsiteQuotaCounterBackend(t)
	subsiteId := int64(92)
	userId := 7
	require.NoError(t, model.DB.Create(&model.SubsiteQuotaPolicy{
		SubsiteId:      subsiteId,
		SiteDailyQuota: 10,
	}).Error)

	ctx := newSubsiteQuotaTestContext(subsiteId, userId)
	require.Nil(t, PreCheckSubsiteQuota(ctx, 6))

	snapshot := fake.onlySnapshot(t)
	require.Equal(t, 0, snapshot.UsedQuota)
	require.Equal(t, 6, snapshot.ReservedQuota)
	require.Equal(t, 1, snapshot.ReservedRequests)

	require.NoError(t, SettleSubsiteQuotaUsage(ctx, &relaycommon.RelayInfo{UserId: userId}, 4))
	snapshot = fake.onlySnapshot(t)
	require.Equal(t, 4, snapshot.UsedQuota)
	require.Equal(t, 1, snapshot.RequestCount)
	require.Equal(t, 0, snapshot.ReservedQuota)
	require.Equal(t, 0, snapshot.ReservedRequests)

	siteDaily := requireSubsiteQuotaCounter(t, subsiteId, 0, model.SubsiteCounterScopeSite, model.SubsiteCounterWindowDaily)
	require.Equal(t, 4, siteDaily.UsedQuota)
	require.Equal(t, 1, siteDaily.RequestCount)
}

func TestSubsiteQuotaRedisReservationRefundsOnFailure(t *testing.T) {
	setupSubsiteQuotaServiceTestDB(t)
	fake := useFakeSubsiteQuotaCounterBackend(t)
	subsiteId := int64(93)
	userId := 7
	require.NoError(t, model.DB.Create(&model.SubsiteQuotaPolicy{
		SubsiteId:      subsiteId,
		SiteDailyQuota: 10,
	}).Error)

	first := newSubsiteQuotaTestContext(subsiteId, userId)
	require.Nil(t, PreCheckSubsiteQuota(first, 7))

	second := newSubsiteQuotaTestContext(subsiteId, userId)
	err := PreCheckSubsiteQuota(second, 4)
	require.NotNil(t, err)
	require.Equal(t, http.StatusForbidden, err.StatusCode)
	require.Equal(t, model.SubsiteAccessCodeQuota, string(err.GetErrorCode()))

	snapshot := fake.onlySnapshot(t)
	require.Equal(t, 7, snapshot.ReservedQuota)
	require.Equal(t, 1, snapshot.ReservedRequests)

	RefundSubsiteQuotaReservation(first)
	snapshot = fake.onlySnapshot(t)
	require.Equal(t, 0, snapshot.UsedQuota)
	require.Equal(t, 0, snapshot.RequestCount)
	require.Equal(t, 0, snapshot.ReservedQuota)
	require.Equal(t, 0, snapshot.ReservedRequests)

	third := newSubsiteQuotaTestContext(subsiteId, userId)
	require.Nil(t, PreCheckSubsiteQuota(third, 4))
}

func setupSubsiteQuotaServiceTestDB(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.SubsiteQuotaPolicy{}, &model.SubsiteQuotaCounter{}))
	clean := func() {
		model.DB.Exec("DELETE FROM subsite_quota_policies")
		model.DB.Exec("DELETE FROM subsite_quota_counters")
	}
	clean()
	t.Cleanup(clean)
}

func useFakeSubsiteQuotaCounterBackend(t *testing.T) *fakeSubsiteQuotaAtomicCounter {
	t.Helper()
	originalBackend := subsiteQuotaCounterBackend
	fake := newFakeSubsiteQuotaAtomicCounter()
	subsiteQuotaCounterBackend = fake
	t.Cleanup(func() {
		subsiteQuotaCounterBackend = originalBackend
	})
	return fake
}

type fakeSubsiteQuotaAtomicCounter struct {
	mu          sync.Mutex
	snapshots   map[string]subsiteQuotaCounterSnapshot
	initialized map[string]bool
}

func newFakeSubsiteQuotaAtomicCounter() *fakeSubsiteQuotaAtomicCounter {
	return &fakeSubsiteQuotaAtomicCounter{
		snapshots:   map[string]subsiteQuotaCounterSnapshot{},
		initialized: map[string]bool{},
	}
}

func (f *fakeSubsiteQuotaAtomicCounter) Enabled() bool {
	return true
}

func (f *fakeSubsiteQuotaAtomicCounter) Reserve(ctx context.Context, key string, quota int, requestCount int, quotaLimit int, requestLimit int, ttl time.Duration, seed subsiteQuotaCounterSnapshot) (subsiteQuotaCounterSnapshot, bool, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.initialized[key] {
		f.snapshots[key] = seed
		f.initialized[key] = true
	}
	snapshot := f.snapshots[key]
	if quotaLimit > 0 && snapshot.UsedQuota+snapshot.ReservedQuota+quota > quotaLimit {
		return snapshot, false, subsiteQuotaRedisRejectQuota, nil
	}
	if requestLimit > 0 && snapshot.RequestCount+snapshot.ReservedRequests+requestCount > requestLimit {
		return snapshot, false, subsiteQuotaRedisRejectRequest, nil
	}
	snapshot.ReservedQuota += quota
	snapshot.ReservedRequests += requestCount
	f.snapshots[key] = snapshot
	return snapshot, true, 0, nil
}

func (f *fakeSubsiteQuotaAtomicCounter) Settle(ctx context.Context, key string, reservedQuota int, reservedRequests int, actualQuota int, actualRequests int, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	snapshot := f.snapshots[key]
	snapshot.ReservedQuota -= reservedQuota
	snapshot.ReservedRequests -= reservedRequests
	if snapshot.ReservedQuota < 0 {
		snapshot.ReservedQuota = 0
	}
	if snapshot.ReservedRequests < 0 {
		snapshot.ReservedRequests = 0
	}
	snapshot.UsedQuota += actualQuota
	snapshot.RequestCount += actualRequests
	f.snapshots[key] = snapshot
	return nil
}

func (f *fakeSubsiteQuotaAtomicCounter) Refund(ctx context.Context, key string, quota int, requestCount int, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	snapshot := f.snapshots[key]
	snapshot.ReservedQuota -= quota
	snapshot.ReservedRequests -= requestCount
	if snapshot.ReservedQuota < 0 {
		snapshot.ReservedQuota = 0
	}
	if snapshot.ReservedRequests < 0 {
		snapshot.ReservedRequests = 0
	}
	f.snapshots[key] = snapshot
	return nil
}

func (f *fakeSubsiteQuotaAtomicCounter) onlySnapshot(t *testing.T) subsiteQuotaCounterSnapshot {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Len(t, f.snapshots, 1)
	for _, snapshot := range f.snapshots {
		return snapshot
	}
	return subsiteQuotaCounterSnapshot{}
}

func newSubsiteQuotaTestContext(subsiteId int64, userId int) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/s/test/v1/chat/completions", nil)
	ctx.Set("subsite_id", subsiteId)
	ctx.Set("id", userId)
	return ctx
}

func requireSubsiteQuotaCounter(t *testing.T, subsiteId int64, userId int, scope string, windowType string) model.SubsiteQuotaCounter {
	t.Helper()
	var counter model.SubsiteQuotaCounter
	require.NoError(t, model.DB.
		Where("subsite_id = ? AND user_id = ? AND scope = ? AND window_type = ?", subsiteId, userId, scope, windowType).
		First(&counter).Error)
	return counter
}
