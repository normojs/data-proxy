package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const enterpriseQuotaCounterStressEnabledEnv = "ENTERPRISE_QUOTA_COUNTER_STRESS"

type enterpriseQuotaCounterStressConfig struct {
	Modes               []string
	Workers             int
	Operations          int
	SaturationAttempts  int
	SaturationSuccesses int
	ReserveQuota        int64
	RefundEvery         int
	ActualJitter        int
	RedisBackend        string
}

type enterpriseQuotaCounterStressFixture struct {
	Policy   model.EnterpriseQuotaPolicy
	Context  *EnterpriseContext
	Now      time.Time
	RedisKey string
}

type enterpriseQuotaCounterStressResult struct {
	Reservation *Reservation
	Err         error
	Rejected    bool
	Refunded    bool
	ActualQuota int64
}

func TestEnterpriseQuotaCounterStress(t *testing.T) {
	if os.Getenv(enterpriseQuotaCounterStressEnabledEnv) != "1" {
		t.Skipf("set %s=1 to run the enterprise quota counter stress smoke", enterpriseQuotaCounterStressEnabledEnv)
	}

	config := enterpriseQuotaCounterStressConfigFromEnv(t)
	for _, mode := range config.Modes {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Run("mixed_settle_refund", func(t *testing.T) {
				runEnterpriseQuotaCounterMixedStress(t, config, mode)
			})
			t.Run("saturate_and_release", func(t *testing.T) {
				runEnterpriseQuotaCounterSaturationStress(t, config, mode)
			})
		})
	}
}

func runEnterpriseQuotaCounterMixedStress(t *testing.T, config enterpriseQuotaCounterStressConfig, mode string) {
	limit := int64(config.Operations)*config.ReserveQuota + config.ReserveQuota
	fixture := setupEnterpriseQuotaCounterStressFixture(t, config, mode, "mixed", limit)

	results := runEnterpriseQuotaCounterStressJobs(config.Operations, config.Workers, func(index int) enterpriseQuotaCounterStressResult {
		reservation, err := ReserveEnterpriseQuota(PolicyEvaluationRequest{
			EnterpriseContext: fixture.Context,
			Estimated:         UsageAmount{Quota: config.ReserveQuota},
			RequestId:         fmt.Sprintf("quota-counter-stress-%s-mixed-%d", mode, index),
			Now:               fixture.Now,
		}, []model.EnterpriseQuotaPolicy{fixture.Policy})
		if err != nil {
			return enterpriseQuotaCounterStressResult{Err: err}
		}
		if reservation == nil {
			return enterpriseQuotaCounterStressResult{Err: errors.New("expected enterprise quota reservation")}
		}
		if config.RefundEvery > 0 && (index+1)%config.RefundEvery == 0 {
			return enterpriseQuotaCounterStressResult{Refunded: true, Err: RefundEnterpriseReservation(reservation)}
		}
		actualQuota := config.ReserveQuota
		if config.ActualJitter > 1 {
			actualQuota -= int64(index % config.ActualJitter)
			if actualQuota <= 0 {
				actualQuota = config.ReserveQuota
			}
		}
		return enterpriseQuotaCounterStressResult{
			ActualQuota: actualQuota,
			Err:         SettleEnterpriseReservation(reservation, UsageAmount{Quota: actualQuota}),
		}
	})

	var expectedUsed int64
	for _, result := range results {
		require.NoError(t, result.Err)
		if !result.Refunded {
			expectedUsed += result.ActualQuota
		}
	}
	assertEnterpriseQuotaCounterStressState(t, fixture.Policy.Id, expectedUsed, 0)
	if mode == "redis" {
		assertEnterpriseQuotaCounterStressBackendSnapshot(t, fixture.RedisKey, expectedUsed, 0)
	}
}

func runEnterpriseQuotaCounterSaturationStress(t *testing.T, config enterpriseQuotaCounterStressConfig, mode string) {
	limit := int64(config.SaturationSuccesses) * config.ReserveQuota
	fixture := setupEnterpriseQuotaCounterStressFixture(t, config, mode, "saturation", limit)

	results := runEnterpriseQuotaCounterStressJobs(config.SaturationAttempts, config.Workers, func(index int) enterpriseQuotaCounterStressResult {
		reservation, err := ReserveEnterpriseQuota(PolicyEvaluationRequest{
			EnterpriseContext: fixture.Context,
			Estimated:         UsageAmount{Quota: config.ReserveQuota},
			RequestId:         fmt.Sprintf("quota-counter-stress-%s-saturate-%d", mode, index),
			Now:               fixture.Now,
		}, []model.EnterpriseQuotaPolicy{fixture.Policy})
		if err != nil {
			var quotaErr EnterpriseQuotaExceededError
			if errors.As(err, &quotaErr) {
				return enterpriseQuotaCounterStressResult{Rejected: true}
			}
			return enterpriseQuotaCounterStressResult{Err: err}
		}
		if reservation == nil {
			return enterpriseQuotaCounterStressResult{Err: errors.New("expected enterprise quota reservation")}
		}
		return enterpriseQuotaCounterStressResult{Reservation: reservation}
	})

	reservations := make([]*Reservation, 0, config.SaturationSuccesses)
	rejected := 0
	for _, result := range results {
		require.NoError(t, result.Err)
		if result.Rejected {
			rejected++
			continue
		}
		require.NotNil(t, result.Reservation)
		reservations = append(reservations, result.Reservation)
	}
	require.Len(t, reservations, config.SaturationSuccesses)
	require.Equal(t, config.SaturationAttempts-config.SaturationSuccesses, rejected)
	assertEnterpriseQuotaCounterStressState(t, fixture.Policy.Id, 0, limit)
	if mode == "redis" {
		assertEnterpriseQuotaCounterStressBackendSnapshot(t, fixture.RedisKey, 0, limit)
	}

	refundResults := runEnterpriseQuotaCounterStressJobs(len(reservations), config.Workers, func(index int) enterpriseQuotaCounterStressResult {
		return enterpriseQuotaCounterStressResult{Err: RefundEnterpriseReservation(reservations[index])}
	})
	for _, result := range refundResults {
		require.NoError(t, result.Err)
	}
	assertEnterpriseQuotaCounterStressState(t, fixture.Policy.Id, 0, 0)
	if mode == "redis" {
		assertEnterpriseQuotaCounterStressBackendSnapshot(t, fixture.RedisKey, 0, 0)
	}
}

func setupEnterpriseQuotaCounterStressFixture(t *testing.T, config enterpriseQuotaCounterStressConfig, mode string, name string, limit int64) enterpriseQuotaCounterStressFixture {
	t.Helper()
	setupEnterprisePolicyServiceTestDB(t)
	configureEnterpriseQuotaCounterStressMode(t, config, mode)

	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       12001,
		Username: "quota-counter-stress-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         fmt.Sprintf("quota counter stress %s %s", mode, name),
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   limit,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	start, _, err := ResolveEnterprisePolicyPeriod(policy, now)
	require.NoError(t, err)
	redisKey := enterpriseQuotaCounterRedisKey(policy, start)
	if mode == "redis" && config.RedisBackend == "real" {
		require.NoError(t, common.RDB.Del(context.Background(), redisKey).Err())
		t.Cleanup(func() {
			_ = common.RDB.Del(context.Background(), redisKey).Err()
		})
	}

	return enterpriseQuotaCounterStressFixture{
		Policy:   policy,
		Context:  &EnterpriseContext{Enabled: true, EnterpriseId: enterprise.Id, UserId: 12001},
		Now:      now,
		RedisKey: redisKey,
	}
}

func configureEnterpriseQuotaCounterStressMode(t *testing.T, config enterpriseQuotaCounterStressConfig, mode string) {
	t.Helper()
	common.EnterpriseQuotaRedisCounterEnabled = false
	if mode != "redis" {
		return
	}

	common.EnterpriseQuotaRedisCounterEnabled = true
	if config.RedisBackend == "fake" {
		enterpriseQuotaCounterBackend = newFakeEnterpriseQuotaAtomicCounter()
		return
	}

	if os.Getenv("REDIS_CONN_STRING") == "" {
		t.Fatal("REDIS_CONN_STRING is required when ENTERPRISE_QUOTA_COUNTER_STRESS_REDIS_BACKEND=real")
	}
	originalRedisEnabled := common.RedisEnabled
	originalRDB := common.RDB
	require.NoError(t, common.InitRedisClient())
	require.True(t, common.RedisEnabled)
	require.NotNil(t, common.RDB)
	enterpriseQuotaCounterBackend = redisEnterpriseQuotaAtomicCounter{}
	t.Cleanup(func() {
		if common.RDB != nil && common.RDB != originalRDB {
			_ = common.RDB.Close()
		}
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRDB
	})
}

func runEnterpriseQuotaCounterStressJobs(count int, workers int, fn func(index int) enterpriseQuotaCounterStressResult) []enterpriseQuotaCounterStressResult {
	if count <= 0 {
		return nil
	}
	if workers <= 0 || workers > count {
		workers = count
	}
	jobs := make(chan int)
	results := make([]enterpriseQuotaCounterStressResult, count)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				results[index] = fn(index)
			}
		}()
	}
	for index := 0; index < count; index++ {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	return results
}

func assertEnterpriseQuotaCounterStressState(t *testing.T, policyId int, expectedUsed int64, expectedReserved int64) {
	t.Helper()
	var counter model.EnterpriseQuotaCounter
	require.NoError(t, model.DB.Where("policy_id = ?", policyId).First(&counter).Error)
	assert.EqualValues(t, expectedUsed, counter.UsedValue)
	assert.EqualValues(t, expectedReserved, counter.ReservedValue)
}

func assertEnterpriseQuotaCounterStressBackendSnapshot(t *testing.T, key string, expectedUsed int64, expectedReserved int64) {
	t.Helper()
	snapshot, found, err := enterpriseQuotaCounterBackend.Snapshot(context.Background(), key)
	require.NoError(t, err)
	require.True(t, found)
	assert.EqualValues(t, expectedUsed, snapshot.UsedValue)
	assert.EqualValues(t, expectedReserved, snapshot.ReservedValue)
}

func enterpriseQuotaCounterStressConfigFromEnv(t *testing.T) enterpriseQuotaCounterStressConfig {
	t.Helper()
	workers := enterpriseQuotaCounterStressEnvInt(t, "ENTERPRISE_QUOTA_COUNTER_STRESS_WORKERS", 32, 1)
	operations := enterpriseQuotaCounterStressEnvInt(t, "ENTERPRISE_QUOTA_COUNTER_STRESS_OPERATIONS", 200, 1)
	saturationAttempts := enterpriseQuotaCounterStressEnvInt(t, "ENTERPRISE_QUOTA_COUNTER_STRESS_SATURATION_ATTEMPTS", workers*4, 2)
	saturationSuccesses := enterpriseQuotaCounterStressEnvInt(t, "ENTERPRISE_QUOTA_COUNTER_STRESS_SATURATION_SUCCESSES", saturationAttempts/2, 1)
	if saturationSuccesses >= saturationAttempts {
		t.Fatalf("ENTERPRISE_QUOTA_COUNTER_STRESS_SATURATION_SUCCESSES must be less than ENTERPRISE_QUOTA_COUNTER_STRESS_SATURATION_ATTEMPTS")
	}

	redisBackend := strings.ToLower(strings.TrimSpace(os.Getenv("ENTERPRISE_QUOTA_COUNTER_STRESS_REDIS_BACKEND")))
	if redisBackend == "" {
		redisBackend = "fake"
	}
	if redisBackend != "fake" && redisBackend != "real" {
		t.Fatalf("unsupported ENTERPRISE_QUOTA_COUNTER_STRESS_REDIS_BACKEND %q; use fake or real", redisBackend)
	}

	return enterpriseQuotaCounterStressConfig{
		Modes:               enterpriseQuotaCounterStressModesFromEnv(t),
		Workers:             workers,
		Operations:          operations,
		SaturationAttempts:  saturationAttempts,
		SaturationSuccesses: saturationSuccesses,
		ReserveQuota:        int64(enterpriseQuotaCounterStressEnvInt(t, "ENTERPRISE_QUOTA_COUNTER_STRESS_RESERVE_QUOTA", 10, 1)),
		RefundEvery:         enterpriseQuotaCounterStressEnvInt(t, "ENTERPRISE_QUOTA_COUNTER_STRESS_REFUND_EVERY", 5, 0),
		ActualJitter:        enterpriseQuotaCounterStressEnvInt(t, "ENTERPRISE_QUOTA_COUNTER_STRESS_ACTUAL_JITTER", 3, 1),
		RedisBackend:        redisBackend,
	}
}

func enterpriseQuotaCounterStressModesFromEnv(t *testing.T) []string {
	t.Helper()
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("ENTERPRISE_QUOTA_COUNTER_STRESS_MODE")))
	if raw == "" || raw == "all" {
		return []string{"db", "redis"}
	}
	parts := strings.Split(raw, ",")
	modes := make([]string, 0, len(parts))
	for _, part := range parts {
		mode := strings.TrimSpace(part)
		if mode == "" {
			continue
		}
		if mode != "db" && mode != "redis" {
			t.Fatalf("unsupported ENTERPRISE_QUOTA_COUNTER_STRESS_MODE %q; use db, redis, or all", mode)
		}
		modes = append(modes, mode)
	}
	if len(modes) == 0 {
		t.Fatal("ENTERPRISE_QUOTA_COUNTER_STRESS_MODE did not contain any runnable mode")
	}
	return modes
}

func enterpriseQuotaCounterStressEnvInt(t *testing.T, key string, fallback int, min int) int {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	require.NoErrorf(t, err, "invalid %s=%q", key, raw)
	if value < min {
		t.Fatalf("%s must be >= %d", key, min)
	}
	return value
}
