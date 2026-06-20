package service

import (
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListEnterpriseQuotaRequestNotificationsBuildsPendingDecisionAndReadState(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	defaultOptions := EnterpriseQuotaRequestNotificationListOptions{}
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 8101, Username: "applicant", DisplayName: "Applicant", Role: common.RoleCommonUser, AffCode: "eg-fu-8101"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 8199, Username: "approver", DisplayName: "Approver", Role: common.RoleAdminUser, AffCode: "eg-fu-8199"}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "notification policy",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	now := common.GetTimestamp()
	request := model.EnterpriseQuotaRequest{
		EnterpriseId:    enterprise.Id,
		ApplicantUserId: 8101,
		ApproverUserId:  8199,
		PolicyId:        policy.Id,
		TargetType:      model.PolicyTargetEnterprise,
		TargetId:        enterprise.Id,
		Metric:          model.PolicyMetricRequestCount,
		Period:          model.PolicyPeriodDay,
		LimitDelta:      3,
		Reason:          "need room",
		Status:          model.EnterpriseQuotaRequestStatusApproved,
		EffectiveAt:     now - 3600,
		ExpiresAt:       now + int64(72*time.Hour/time.Second),
		DecidedAt:       now + 120,
		CreatedAt:       now,
		UpdatedAt:       now + 120,
	}
	require.NoError(t, model.DB.Create(&request).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseAuditLog{
		EnterpriseId: enterprise.Id,
		ActorUserId:  8101,
		Action:       "quota_request.submit",
		TargetType:   "quota_request",
		TargetId:     request.Id,
		BeforeJson:   "{}",
		AfterJson:    "{}",
		CreatedAt:    now + 10,
	}).Error)
	approveAudit := model.EnterpriseAuditLog{
		EnterpriseId: enterprise.Id,
		ActorUserId:  8199,
		Action:       "quota_request.approve",
		TargetType:   "quota_request",
		TargetId:     request.Id,
		BeforeJson:   "{}",
		AfterJson:    "{}",
		CreatedAt:    now + 120,
	}
	require.NoError(t, model.DB.Create(&approveAudit).Error)

	adminList, err := ListEnterpriseQuotaRequestNotifications(enterprise.Id, 8199, true, nil, defaultOptions)
	require.NoError(t, err)
	assert.Empty(t, adminList.Items)
	assert.Equal(t, 0, adminList.UnreadCount)

	userList, err := ListEnterpriseQuotaRequestNotifications(enterprise.Id, 8101, false, nil, defaultOptions)
	require.NoError(t, err)
	require.Len(t, userList.Items, 1)
	assert.Equal(t, "quota_request:audit:", userList.Items[0].Key[:20])
	assert.Equal(t, model.EnterpriseQuotaRequestStatusApproved, userList.Items[0].Status)
	assert.Equal(t, request.Id, userList.Items[0].QuotaRequestId)
	assert.Equal(t, approveAudit.Id, userList.Items[0].AuditLogId)
	assert.Equal(t, "notification policy", userList.Items[0].PolicyName)
	assert.Equal(t, "Applicant", userList.Items[0].ApplicantName)
	assert.Equal(t, "Approver", userList.Items[0].ActorName)
	assert.Equal(t, "notification.enterprise_quota_request.title.approved", userList.Items[0].TitleKey)
	assert.Equal(t, "notification.enterprise_quota_request.content.approved", userList.Items[0].ContentKey)
	assert.Equal(t, "notification policy", userList.Items[0].ContentParams["policyName"])
	assert.Equal(t, "Approver", userList.Items[0].ContentParams["actorName"])
	assert.False(t, userList.Items[0].Read)
	assert.Equal(t, 1, userList.UnreadCount)

	readList, err := ListEnterpriseQuotaRequestNotifications(enterprise.Id, 8101, false, []string{userList.Items[0].Key}, defaultOptions)
	require.NoError(t, err)
	require.Len(t, readList.Items, 1)
	assert.True(t, readList.Items[0].Read)
	assert.Equal(t, 0, readList.UnreadCount)
}

func TestListEnterpriseQuotaRequestNotificationsShowsExpiringSoonToApplicantAndAdmin(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	defaultOptions := EnterpriseQuotaRequestNotificationListOptions{}
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 8301, Username: "expiring-user", DisplayName: "Expiring User", Role: common.RoleCommonUser, AffCode: "eg-fu-8301"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 8399, Username: "expiring-admin", DisplayName: "Expiring Admin", Role: common.RoleAdminUser, AffCode: "eg-fu-8399"}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "expiring policy",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	now := common.GetTimestamp()
	request := model.EnterpriseQuotaRequest{
		EnterpriseId:    enterprise.Id,
		ApplicantUserId: 8301,
		ApproverUserId:  8399,
		PolicyId:        policy.Id,
		TargetType:      model.PolicyTargetEnterprise,
		TargetId:        enterprise.Id,
		Metric:          model.PolicyMetricRequestCount,
		Period:          model.PolicyPeriodDay,
		LimitDelta:      7,
		Reason:          "expiring room",
		Status:          model.EnterpriseQuotaRequestStatusApproved,
		EffectiveAt:     now - 3600,
		ExpiresAt:       now + 3600,
		DecidedAt:       now - 1800,
		CreatedAt:       now - 7200,
		UpdatedAt:       now - 1800,
	}
	require.NoError(t, model.DB.Create(&request).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseQuotaRequest{
		EnterpriseId:    enterprise.Id,
		ApplicantUserId: 8301,
		ApproverUserId:  8399,
		PolicyId:        policy.Id,
		TargetType:      model.PolicyTargetEnterprise,
		TargetId:        enterprise.Id,
		Metric:          model.PolicyMetricRequestCount,
		Period:          model.PolicyPeriodDay,
		LimitDelta:      2,
		Reason:          "not soon",
		Status:          model.EnterpriseQuotaRequestStatusApproved,
		EffectiveAt:     now - 3600,
		ExpiresAt:       now + int64(72*time.Hour/time.Second),
		DecidedAt:       now - 1800,
		CreatedAt:       now - 7200,
		UpdatedAt:       now - 1800,
	}).Error)

	userList, err := ListEnterpriseQuotaRequestNotifications(enterprise.Id, 8301, false, nil, defaultOptions)
	require.NoError(t, err)
	require.Len(t, userList.Items, 1)
	assert.Equal(t, "expiring_soon", userList.Items[0].Status)
	assert.Equal(t, request.Id, userList.Items[0].QuotaRequestId)
	assert.Equal(t, "quota_request:expiring_soon:"+strconv.Itoa(request.Id)+":"+strconv.FormatInt(request.ExpiresAt, 10), userList.Items[0].Key)
	assert.Equal(t, "expiring policy", userList.Items[0].PolicyName)
	assert.Equal(t, "Expiring User", userList.Items[0].ApplicantName)
	assert.Equal(t, "Expiring Admin", userList.Items[0].ActorName)
	assert.Equal(t, "notification.enterprise_quota_request.title.expiring_soon", userList.Items[0].TitleKey)
	assert.Equal(t, "notification.enterprise_quota_request.content.expiring_soon", userList.Items[0].ContentKey)
	assert.Equal(t, "expiring policy", userList.Items[0].ContentParams["policyName"])
	assert.Equal(t, 1, userList.UnreadCount)

	readList, err := ListEnterpriseQuotaRequestNotifications(enterprise.Id, 8301, false, []string{userList.Items[0].Key}, defaultOptions)
	require.NoError(t, err)
	require.Len(t, readList.Items, 1)
	assert.True(t, readList.Items[0].Read)
	assert.Equal(t, 0, readList.UnreadCount)

	adminList, err := ListEnterpriseQuotaRequestNotifications(enterprise.Id, 8399, true, nil, defaultOptions)
	require.NoError(t, err)
	require.Len(t, adminList.Items, 1)
	assert.Equal(t, "expiring_soon", adminList.Items[0].Status)
	assert.Equal(t, request.Id, adminList.Items[0].QuotaRequestId)
}

func TestExpireDueEnterpriseQuotaRequestsExpiresPendingAndApprovedWithAudit(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 8401, Username: "expiry-user", Role: common.RoleCommonUser, AffCode: "eg-fu-8401"}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "expiry policy",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	now := common.GetTimestamp()
	requests := []model.EnterpriseQuotaRequest{
		{
			EnterpriseId: enterprise.Id, ApplicantUserId: 8401, PolicyId: policy.Id, TargetType: model.PolicyTargetEnterprise,
			TargetId: enterprise.Id, Metric: model.PolicyMetricRequestCount, Period: model.PolicyPeriodDay, LimitDelta: 3,
			Status: model.EnterpriseQuotaRequestStatusPending, ExpiresAt: now - 10, CreatedAt: now - 100, UpdatedAt: now - 100,
		},
		{
			EnterpriseId: enterprise.Id, ApplicantUserId: 8401, ApproverUserId: 8499, PolicyId: policy.Id, TargetType: model.PolicyTargetEnterprise,
			TargetId: enterprise.Id, Metric: model.PolicyMetricRequestCount, Period: model.PolicyPeriodDay, LimitDelta: 5,
			Status: model.EnterpriseQuotaRequestStatusApproved, EffectiveAt: now - 100, ExpiresAt: now - 5, CreatedAt: now - 100, UpdatedAt: now - 50,
		},
		{
			EnterpriseId: enterprise.Id, ApplicantUserId: 8401, PolicyId: policy.Id, TargetType: model.PolicyTargetEnterprise,
			TargetId: enterprise.Id, Metric: model.PolicyMetricRequestCount, Period: model.PolicyPeriodDay, LimitDelta: 1,
			Status: model.EnterpriseQuotaRequestStatusRejected, ExpiresAt: now - 5, CreatedAt: now - 100, UpdatedAt: now - 50,
		},
		{
			EnterpriseId: enterprise.Id, ApplicantUserId: 8401, ApproverUserId: 8499, PolicyId: policy.Id, TargetType: model.PolicyTargetEnterprise,
			TargetId: enterprise.Id, Metric: model.PolicyMetricRequestCount, Period: model.PolicyPeriodDay, LimitDelta: 2,
			Status: model.EnterpriseQuotaRequestStatusApproved, EffectiveAt: now - 100, ExpiresAt: now + 3600, CreatedAt: now - 100, UpdatedAt: now - 50,
		},
	}
	for index := range requests {
		require.NoError(t, model.DB.Create(&requests[index]).Error)
	}

	expired, err := ExpireDueEnterpriseQuotaRequests(now, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, expired)

	var reloaded []model.EnterpriseQuotaRequest
	require.NoError(t, model.DB.Order("id asc").Find(&reloaded).Error)
	require.Len(t, reloaded, 4)
	assert.Equal(t, model.EnterpriseQuotaRequestStatusExpired, reloaded[0].Status)
	assert.Equal(t, model.EnterpriseQuotaRequestStatusExpired, reloaded[1].Status)
	assert.Equal(t, model.EnterpriseQuotaRequestStatusRejected, reloaded[2].Status)
	assert.Equal(t, model.EnterpriseQuotaRequestStatusApproved, reloaded[3].Status)
	assert.Equal(t, now, reloaded[0].DecidedAt)
	assert.Equal(t, now, reloaded[1].DecidedAt)

	var audits []model.EnterpriseAuditLog
	require.NoError(t, model.DB.Where("action = ?", "quota_request.expire").Order("target_id asc").Find(&audits).Error)
	require.Len(t, audits, 2)
	assert.Equal(t, requests[0].Id, audits[0].TargetId)
	assert.Equal(t, requests[1].Id, audits[1].TargetId)
	assert.Contains(t, audits[0].BeforeJson, model.EnterpriseQuotaRequestStatusPending)
	assert.Contains(t, audits[0].AfterJson, model.EnterpriseQuotaRequestStatusExpired)

	expiredAgain, err := ExpireDueEnterpriseQuotaRequests(now, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, expiredAgain)
	var auditCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseAuditLog{}).Where("action = ?", "quota_request.expire").Count(&auditCount).Error)
	assert.Equal(t, int64(2), auditCount)
	var outboxCount int64
	require.NoError(t, model.DB.Model(&model.EnterpriseNotificationOutbox{}).Where("event_type = ?", "quota_request.expire").Count(&outboxCount).Error)
	assert.Equal(t, int64(2), outboxCount)
	var outboxRows []model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.Where("event_type = ?", "quota_request.expire").Order("target_id asc").Find(&outboxRows).Error)
	require.Len(t, outboxRows, 2)
	assert.Equal(t, requests[0].Id, outboxRows[0].TargetId)
	assert.Equal(t, 8401, outboxRows[0].RecipientUserId)
	assert.Contains(t, outboxRows[0].PayloadJson, model.EnterpriseQuotaRequestStatusExpired)
}

func TestListEnterpriseQuotaRequestNotificationsPaginatesAndFiltersUnread(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 8501, Username: "paged-user", DisplayName: "Paged User", Role: common.RoleCommonUser, AffCode: "eg-fu-8501"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 8599, Username: "paged-admin", DisplayName: "Paged Admin", Role: common.RoleAdminUser, AffCode: "eg-fu-8599"}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "paged policy",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	now := common.GetTimestamp()
	readKeys := make([]string, 0, 2)
	for index := 0; index < 5; index++ {
		request := model.EnterpriseQuotaRequest{
			EnterpriseId:    enterprise.Id,
			ApplicantUserId: 8501,
			ApproverUserId:  8599,
			PolicyId:        policy.Id,
			TargetType:      model.PolicyTargetEnterprise,
			TargetId:        enterprise.Id,
			Metric:          model.PolicyMetricRequestCount,
			Period:          model.PolicyPeriodDay,
			LimitDelta:      int64(index + 1),
			Reason:          "paged room",
			Status:          model.EnterpriseQuotaRequestStatusApproved,
			EffectiveAt:     now - 3600,
			ExpiresAt:       now + int64(72*time.Hour/time.Second),
			DecidedAt:       now + int64(index),
			CreatedAt:       now - 7200 + int64(index),
			UpdatedAt:       now + int64(index),
		}
		require.NoError(t, model.DB.Create(&request).Error)
		audit := model.EnterpriseAuditLog{
			EnterpriseId: enterprise.Id,
			ActorUserId:  8599,
			Action:       "quota_request.approve",
			TargetType:   "quota_request",
			TargetId:     request.Id,
			BeforeJson:   "{}",
			AfterJson:    "{}",
			CreatedAt:    now + int64(index),
		}
		require.NoError(t, model.DB.Create(&audit).Error)
		if index >= 3 {
			readKeys = append(readKeys, "quota_request:audit:"+strconv.FormatInt(audit.Id, 10))
		}
	}

	firstPage, err := ListEnterpriseQuotaRequestNotifications(enterprise.Id, 8501, false, readKeys, EnterpriseQuotaRequestNotificationListOptions{Page: 1, Limit: 2})
	require.NoError(t, err)
	require.Len(t, firstPage.Items, 2)
	assert.Equal(t, 1, firstPage.Page)
	assert.Equal(t, 2, firstPage.PageSize)
	assert.True(t, firstPage.HasMore)
	assert.Equal(t, 3, firstPage.UnreadCount)
	assert.True(t, firstPage.Items[0].Read)
	assert.True(t, firstPage.Items[1].Read)

	secondPage, err := ListEnterpriseQuotaRequestNotifications(enterprise.Id, 8501, false, readKeys, EnterpriseQuotaRequestNotificationListOptions{Page: 2, Limit: 2})
	require.NoError(t, err)
	require.Len(t, secondPage.Items, 2)
	assert.True(t, secondPage.HasMore)
	assert.False(t, secondPage.Items[0].Read)
	assert.False(t, secondPage.Items[1].Read)

	unreadOnly, err := ListEnterpriseQuotaRequestNotifications(enterprise.Id, 8501, false, readKeys, EnterpriseQuotaRequestNotificationListOptions{Page: 1, Limit: 2, UnreadOnly: true})
	require.NoError(t, err)
	require.Len(t, unreadOnly.Items, 2)
	assert.True(t, unreadOnly.HasMore)
	assert.Equal(t, 3, unreadOnly.UnreadCount)
	assert.False(t, unreadOnly.Items[0].Read)
	assert.False(t, unreadOnly.Items[1].Read)

	unreadOnlySecondPage, err := ListEnterpriseQuotaRequestNotifications(enterprise.Id, 8501, false, readKeys, EnterpriseQuotaRequestNotificationListOptions{Page: 2, Limit: 2, UnreadOnly: true})
	require.NoError(t, err)
	require.Len(t, unreadOnlySecondPage.Items, 1)
	assert.False(t, unreadOnlySecondPage.HasMore)
	assert.False(t, unreadOnlySecondPage.Items[0].Read)
}

func TestListEnterpriseQuotaRequestNotificationsShowsPendingOnlyToAdmins(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	defaultOptions := EnterpriseQuotaRequestNotificationListOptions{}
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 8201, Username: "pending-user", DisplayName: "Pending User", Role: common.RoleCommonUser, AffCode: "eg-fu-8201"}).Error)
	policy := createEnterprisePolicyServiceTestPolicy(t, model.EnterpriseQuotaPolicy{
		EnterpriseId: enterprise.Id,
		Name:         "pending policy",
		TargetType:   model.PolicyTargetEnterprise,
		TargetId:     enterprise.Id,
		Metric:       model.PolicyMetricRequestCount,
		Period:       model.PolicyPeriodDay,
		LimitValue:   10,
		ModelScope:   model.PolicyModelScopeAll,
		Status:       model.QuotaPolicyStatusEnabled,
	})
	now := time.Date(2026, 6, 20, 11, 0, 0, 0, time.UTC).Unix()
	request := model.EnterpriseQuotaRequest{
		EnterpriseId:    enterprise.Id,
		ApplicantUserId: 8201,
		PolicyId:        policy.Id,
		TargetType:      model.PolicyTargetEnterprise,
		TargetId:        enterprise.Id,
		Metric:          model.PolicyMetricRequestCount,
		Period:          model.PolicyPeriodDay,
		LimitDelta:      5,
		Reason:          "pending room",
		Status:          model.EnterpriseQuotaRequestStatusPending,
		ExpiresAt:       now + 3600,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	require.NoError(t, model.DB.Create(&request).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseAuditLog{
		EnterpriseId: enterprise.Id,
		ActorUserId:  8201,
		Action:       "quota_request.submit",
		TargetType:   "quota_request",
		TargetId:     request.Id,
		BeforeJson:   "{}",
		AfterJson:    "{}",
		CreatedAt:    now + 10,
	}).Error)

	userList, err := ListEnterpriseQuotaRequestNotifications(enterprise.Id, 8201, false, nil, defaultOptions)
	require.NoError(t, err)
	assert.Empty(t, userList.Items)

	adminList, err := ListEnterpriseQuotaRequestNotifications(enterprise.Id, 8299, true, nil, defaultOptions)
	require.NoError(t, err)
	require.Len(t, adminList.Items, 1)
	assert.Equal(t, "quota_request:pending:", adminList.Items[0].Key[:22])
	assert.Equal(t, model.EnterpriseQuotaRequestStatusPending, adminList.Items[0].Status)
	assert.Equal(t, request.Id, adminList.Items[0].QuotaRequestId)
	assert.NotZero(t, adminList.Items[0].AuditLogId)
	assert.Equal(t, 1, adminList.UnreadCount)
}
