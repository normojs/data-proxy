package service

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupEnterprisePolicyServiceTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Enterprise{},
		&model.EnterpriseOrgUnit{},
		&model.EnterpriseOrgMembership{},
		&model.EnterprisePolicyGroup{},
		&model.EnterprisePolicyGroupMember{},
		&model.EnterpriseProject{},
		&model.EnterpriseProjectOrgUnit{},
		&model.EnterpriseQuotaPolicy{},
		&model.EnterpriseQuotaCounter{},
		&model.EnterpriseQuotaRequest{},
		&model.EnterpriseWebhook{},
		&model.EnterpriseNotificationRead{},
		&model.EnterpriseNotificationPreference{},
		&model.EnterpriseNotificationOutbox{},
		&model.EnterpriseUsageAttribution{},
		&model.EnterpriseAuditLog{},
	))
	originalDB := model.DB
	originalEnabled := common.EnterpriseGovernanceEnabled
	originalDryRun := common.EnterpriseGovernanceDryRunEnabled
	model.DB = db
	common.EnterpriseGovernanceEnabled = true
	common.EnterpriseGovernanceDryRunEnabled = false
	t.Cleanup(func() {
		model.DB = originalDB
		common.EnterpriseGovernanceEnabled = originalEnabled
		common.EnterpriseGovernanceDryRunEnabled = originalDryRun
		_ = sqlDB.Close()
	})
	require.NoError(t, model.EnsureDefaultEnterprise())
}

func TestEnterprisePolicyConditionBuildValidateAndEvaluate(t *testing.T) {
	isPlayground := false
	condition := PolicyCondition{
		Abilities:     []string{"image", "chat", "chat"},
		RuntimeGroups: []string{"vip", "default"},
		ModelPrefixes: []string{"gpt-4"},
		IsPlayground:  &isPlayground,
	}

	expr, err := BuildCELExpressionFromCondition(condition)
	require.NoError(t, err)
	assert.Equal(t, `request.ability in ["chat", "image"] && user.runtime_group in ["default", "vip"] && ["gpt-4"].exists(prefix, request.model.startsWith(prefix)) && request.is_playground == false`, expr)
	require.NoError(t, ValidatePolicyConditionExpression(expr))
	assert.Error(t, ValidatePolicyConditionExpression("request.prompt == 'secret'"))

	policy := model.EnterpriseQuotaPolicy{
		Id:            12,
		ConditionMode: model.PolicyConditionModeStructured,
		ConditionExpr: expr,
		ConditionHash: "condition-test",
	}
	ok, err := EvaluatePolicyCondition(policy, EnterprisePolicyCELInput{
		User: EnterprisePolicyCELUser{Id: 7, RuntimeGroup: "vip", Role: "user"},
		Org:  EnterprisePolicyCELOrg{EnterpriseId: 1},
		Request: EnterprisePolicyCELRequest{
			Model:        "gpt-4o",
			Ability:      "chat",
			IsPlayground: false,
		},
	})
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = EvaluatePolicyCondition(policy, EnterprisePolicyCELInput{
		User: EnterprisePolicyCELUser{Id: 7, RuntimeGroup: "vip", Role: "user"},
		Org:  EnterprisePolicyCELOrg{EnterpriseId: 1},
		Request: EnterprisePolicyCELRequest{
			Model:        "claude-sonnet-4",
			Ability:      "chat",
			IsPlayground: false,
		},
	})
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestEnterprisePolicyMatchUsesOrgAncestorsGroupsAndConditions(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1001,
		Username: "alice",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "vip",
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseOrgUnit{
		Id:           10,
		EnterpriseId: enterprise.Id,
		ParentId:     0,
		Name:         "研发部",
		Slug:         "engineering",
		Path:         "/10/",
		Depth:        1,
		Status:       model.OrgUnitStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseOrgUnit{
		Id:           11,
		EnterpriseId: enterprise.Id,
		ParentId:     10,
		Name:         "平台组",
		Slug:         "platform",
		Path:         "/10/11/",
		Depth:        2,
		Status:       model.OrgUnitStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseOrgMembership{
		EnterpriseId: enterprise.Id,
		UserId:       1001,
		OrgUnitId:    11,
		IsPrimary:    true,
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterprisePolicyGroup{
		Id:           20,
		EnterpriseId: enterprise.Id,
		Name:         "高阶模型试点",
		Slug:         "pilot",
		Status:       model.PolicyGroupStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterprisePolicyGroupMember{
		EnterpriseId:  enterprise.Id,
		PolicyGroupId: 20,
		UserId:        1001,
	}).Error)

	enterprisePolicy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "企业请求次数",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   100,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	orgPolicy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId:  enterprise.Id,
		Name:          "研发部 GPT-4",
		TargetType:    model.PolicyTargetOrgUnit,
		TargetId:      10,
		Metric:        model.PolicyMetricQuota,
		Period:        model.PolicyPeriodDay,
		LimitValue:    1000,
		ModelScope:    model.PolicyModelScopeSpecific,
		ModelsJson:    `["gpt-4o"]`,
		ConditionJson: `{"abilities":["chat"]}`,
		Status:        model.QuotaPolicyStatusEnabled,
	})
	groupPolicy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId:  enterprise.Id,
		Name:          "试点分组",
		TargetType:    model.PolicyTargetPolicyGroup,
		TargetId:      20,
		Metric:        model.PolicyMetricQuota,
		Period:        model.PolicyPeriodDay,
		LimitValue:    1000,
		ModelScope:    model.PolicyModelScopeAll,
		ConditionJson: `{"is_playground":false}`,
		Status:        model.QuotaPolicyStatusEnabled,
	})
	nonMatchingUserPolicy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId:  enterprise.Id,
		Name:          "Playground only",
		TargetType:    model.PolicyTargetUser,
		TargetId:      1001,
		Metric:        model.PolicyMetricQuota,
		Period:        model.PolicyPeriodDay,
		LimitValue:    1000,
		ModelScope:    model.PolicyModelScopeAll,
		ConditionJson: `{"is_playground":true}`,
		Status:        model.QuotaPolicyStatusEnabled,
	})

	ctx, err := ResolveEnterpriseContext(1001, 3001)
	require.NoError(t, err)
	assert.Equal(t, []int{10, 11}, ctx.OrgUnitIds)
	assert.Equal(t, []int{20}, ctx.PolicyGroupIds)

	policies, err := MatchEnterprisePolicies(PolicyEvaluationRequest{
		EnterpriseContext: ctx,
		ModelName:         "gpt-4o",
		Ability:           "chat",
		IsPlayground:      false,
		Now:               time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []int{enterprisePolicy.Id, orgPolicy.Id, groupPolicy.Id}, enterprisePolicyIds(policies))
	assert.NotContains(t, enterprisePolicyIds(policies), nonMatchingUserPolicy.Id)
}

func TestEnterprisePolicyModelPermissionUsesSpecificScopeIntersection(t *testing.T) {
	policies := []model.EnterpriseQuotaPolicy{
		{
			Id:         1,
			ModelScope: model.PolicyModelScopeSpecific,
			ModelsJson: `["gpt-4o","gpt-4o-mini"]`,
		},
		{
			Id:         2,
			ModelScope: model.PolicyModelScopeSpecific,
			ModelsJson: `["gpt-4o","claude-sonnet-4"]`,
		},
		{
			Id:         3,
			ModelScope: model.PolicyModelScopeAll,
		},
	}

	require.NoError(t, CheckEnterpriseModelPermission(PolicyEvaluationRequest{ModelName: "gpt-4o"}, policies))
	err := CheckEnterpriseModelPermission(PolicyEvaluationRequest{ModelName: "gpt-4o-mini"}, policies)
	require.Error(t, err)
	var modelErr EnterpriseModelNotAllowedError
	require.True(t, errors.As(err, &modelErr))
	assert.Equal(t, "gpt-4o-mini", modelErr.ModelName)
	assert.Equal(t, []int{1, 2}, modelErr.PolicyIds)
	assert.Equal(t, []string{"gpt-4o"}, modelErr.AllowedModels)
	assert.Contains(t, err.Error(), "policy_ids=1,2")
}

func TestEnterprisePolicyModelPermissionRejectsUnsupportedScope(t *testing.T) {
	err := CheckEnterpriseModelPermission(PolicyEvaluationRequest{ModelName: "gpt-4o"}, []model.EnterpriseQuotaPolicy{
		{
			Id:         8,
			ModelScope: "legacy",
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported enterprise policy model scope")
	assert.Contains(t, err.Error(), "policy_id=8")
}

func TestEnterprisePolicyEvaluateRejectsModelOutsideSpecificScope(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "enterprise model allow list",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   100,
		ModelScope:   model.PolicyModelScopeSpecific,
		ModelsJson:   `["gpt-4o"]`,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	ctx := &EnterpriseContext{Enabled: true, EnterpriseId: enterprise.Id, UserId: 1002}

	decision, reservation, err := EvaluateEnterprisePolicies(PolicyEvaluationRequest{
		EnterpriseContext: ctx,
		ModelName:         "claude-sonnet-4",
		Estimated:         UsageAmount{Quota: 1},
		Now:               time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
	})

	require.NoError(t, err)
	assert.Nil(t, reservation)
	assert.False(t, decision.Allowed)
	assert.True(t, decision.WouldReject)
	assert.Equal(t, []int{policy.Id}, decision.MatchedPolicyIds)
	var modelErr EnterpriseModelNotAllowedError
	require.True(t, errors.As(decision.DenyError, &modelErr))
	assert.Equal(t, []int{policy.Id}, modelErr.PolicyIds)
	var counterCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseQuotaCounter{}).Count(&counterCount).Error)
	assert.EqualValues(t, 0, counterCount)
}

func TestEnterpriseQuotaReservationRollsBackWhenAnyPolicyExceedsLimit(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 1002, Username: "bob", Status: common.UserStatusEnabled, Group: "default"}).Error)

	policyA := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "企业请求次数 A",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	policyB := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "企业请求次数 B",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   3,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})

	ctx := &EnterpriseContext{Enabled: true, EnterpriseId: enterprise.Id, UserId: 1002}
	reservation, err := ReserveEnterpriseQuota(PolicyEvaluationRequest{
		EnterpriseContext: ctx,
		Estimated:         UsageAmount{RequestCount: 5},
		Now:               time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
	}, []model.EnterpriseQuotaPolicy{policyA, policyB})

	require.Error(t, err)
	assert.Nil(t, reservation)
	var counterCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseQuotaCounter{}).Count(&counterCount).Error)
	assert.EqualValues(t, 0, counterCount)
}

func TestEnterpriseTemporaryQuotaExtendsEffectiveLimit(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 1008, Username: "temp-quota-user", Status: common.UserStatusEnabled, Group: "default"}).Error)
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "temporary quota base",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   3,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	require.NoError(t, model.DB.Create(&model.EnterpriseQuotaRequest{
		EnterpriseId:    enterprise.Id,
		ApplicantUserId: 1008,
		ApproverUserId:  1,
		PolicyId:        policy.Id,
		TargetType:      policy.TargetType,
		TargetId:        policy.TargetId,
		Metric:          policy.Metric,
		Period:          policy.Period,
		LimitDelta:      2,
		Reason:          "launch batch",
		Status:          model.EnterpriseQuotaRequestStatusApproved,
		EffectiveAt:     now.Add(-time.Hour).Unix(),
		ExpiresAt:       now.Add(time.Hour).Unix(),
		DecidedAt:       now.Add(-time.Hour).Unix(),
	}).Error)
	ctx := &EnterpriseContext{Enabled: true, EnterpriseId: enterprise.Id, UserId: 1008}
	reservation, err := ReserveEnterpriseQuota(PolicyEvaluationRequest{
		EnterpriseContext: ctx,
		Estimated:         UsageAmount{RequestCount: 5},
		Now:               now,
	}, []model.EnterpriseQuotaPolicy{policy})

	require.NoError(t, err)
	require.NotNil(t, reservation)
	var counter model.EnterpriseQuotaCounter
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&counter).Error)
	assert.EqualValues(t, 5, counter.ReservedValue)

	RefundEnterpriseReservation(reservation)
	require.NoError(t, model.DB.Model(&model.EnterpriseQuotaRequest{}).Where("policy_id = ?", policy.Id).Update("expires_at", now.Add(-time.Minute).Unix()).Error)
	reservation, err = ReserveEnterpriseQuota(PolicyEvaluationRequest{
		EnterpriseContext: ctx,
		Estimated:         UsageAmount{RequestCount: 5},
		Now:               now,
	}, []model.EnterpriseQuotaPolicy{policy})
	require.Error(t, err)
	assert.Nil(t, reservation)
	var quotaErr EnterpriseQuotaExceededError
	require.True(t, errors.As(err, &quotaErr))
	assert.EqualValues(t, 3, quotaErr.LimitValue)
}

func TestEnterprisePolicyEvaluateDryRunAllowsWouldReject(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 1003, Username: "carol", Status: common.UserStatusEnabled, Group: "default"}).Error)
	common.EnterpriseGovernanceDryRunEnabled = true

	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "企业请求次数",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   1,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	ctx := &EnterpriseContext{
		Enabled:      true,
		DryRun:       true,
		EnterpriseId: enterprise.Id,
		UserId:       1003,
		RuntimeGroup: "default",
		Role:         "user",
	}

	decision, reservation, err := EvaluateEnterprisePolicies(PolicyEvaluationRequest{
		EnterpriseContext: ctx,
		ModelName:         "gpt-4o",
		Ability:           "chat",
		Estimated:         UsageAmount{RequestCount: 2},
		Now:               time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
	})

	require.NoError(t, err)
	assert.Nil(t, reservation)
	assert.True(t, decision.Allowed)
	assert.True(t, decision.DryRun)
	assert.True(t, decision.WouldReject)
	assert.Equal(t, []int{policy.Id}, decision.MatchedPolicyIds)
}

func TestEnterpriseQuotaReservationConcurrentLimit(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 1011, Username: "jane", Status: common.UserStatusEnabled, Group: "default"}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "企业并发请求次数",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   5,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	ctx := &EnterpriseContext{Enabled: true, EnterpriseId: enterprise.Id, UserId: 1011}
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)

	type reservationResult struct {
		err      error
		reserved bool
	}
	const attempts = 12
	results := make(chan reservationResult, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			reservation, err := ReserveEnterpriseQuota(PolicyEvaluationRequest{
				EnterpriseContext: ctx,
				Estimated:         UsageAmount{RequestCount: 1},
				RequestId:         fmt.Sprintf("req-enterprise-concurrent-%d", index),
				Now:               now,
			}, []model.EnterpriseQuotaPolicy{policy})
			results <- reservationResult{err: err, reserved: reservation != nil}
		}(i)
	}
	wg.Wait()
	close(results)

	successCount := 0
	rejectCount := 0
	for result := range results {
		if result.err == nil && result.reserved {
			successCount++
			continue
		}
		require.Error(t, result.err)
		var quotaErr EnterpriseQuotaExceededError
		require.True(t, errors.As(result.err, &quotaErr))
		assert.Equal(t, policy.Id, quotaErr.PolicyId)
		rejectCount++
	}
	assert.Equal(t, 5, successCount)
	assert.Equal(t, attempts-5, rejectCount)

	var counter model.EnterpriseQuotaCounter
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&counter).Error)
	assert.EqualValues(t, 5, counter.ReservedValue)
	assert.EqualValues(t, 0, counter.UsedValue)
}

func TestEnterpriseReservationSettleMovesReservedToUsed(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 1004, Username: "dave", Status: common.UserStatusEnabled, Group: "default"}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "企业 quota",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   100,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	ctx := &EnterpriseContext{Enabled: true, EnterpriseId: enterprise.Id, UserId: 1004}

	reservation, err := ReserveEnterpriseQuota(PolicyEvaluationRequest{
		EnterpriseContext: ctx,
		Estimated:         UsageAmount{Quota: 10},
		Now:               time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
	}, []model.EnterpriseQuotaPolicy{policy})
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.NoError(t, SettleEnterpriseReservation(reservation, UsageAmount{Quota: 7}))

	var counter model.EnterpriseQuotaCounter
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&counter).Error)
	assert.EqualValues(t, 0, counter.ReservedValue)
	assert.EqualValues(t, 7, counter.UsedValue)
}

func TestEnterpriseReservationRefundReleasesReserved(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 1005, Username: "erin", Status: common.UserStatusEnabled, Group: "default"}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "企业 quota",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricQuota,
		Period:       model.PolicyPeriodDay,
		LimitValue:   100,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	ctx := &EnterpriseContext{Enabled: true, EnterpriseId: enterprise.Id, UserId: 1005}

	reservation, err := ReserveEnterpriseQuota(PolicyEvaluationRequest{
		EnterpriseContext: ctx,
		Estimated:         UsageAmount{Quota: 10},
		Now:               time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
	}, []model.EnterpriseQuotaPolicy{policy})
	require.NoError(t, err)
	require.NotNil(t, reservation)
	require.NoError(t, RefundEnterpriseReservation(reservation))

	var counter model.EnterpriseQuotaCounter
	require.NoError(t, model.DB.Where("policy_id = ?", policy.Id).First(&counter).Error)
	assert.EqualValues(t, 0, counter.ReservedValue)
	assert.EqualValues(t, 0, counter.UsedValue)
}

func createEnterprisePolicyServiceTestPolicy(t *testing.T, policy model.EnterpriseQuotaPolicy) model.EnterpriseQuotaPolicy {
	t.Helper()
	if policy.Timezone == "" {
		policy.Timezone = model.DefaultEnterpriseTimezone
	}
	if policy.Action == "" {
		policy.Action = model.PolicyActionReject
	}
	if policy.ConditionMode == "" {
		policy.ConditionMode = model.PolicyConditionModeStructured
	}
	require.NoError(t, NormalizeEnterpriseQuotaPolicyCondition(&policy))
	require.NoError(t, model.DB.Create(&policy).Error)
	return policy
}

func enterprisePolicyIds(policies []model.EnterpriseQuotaPolicy) []int {
	ids := make([]int, 0, len(policies))
	for _, policy := range policies {
		ids = append(ids, policy.Id)
	}
	return ids
}
