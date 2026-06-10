package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetBillingEventsScopeIsolation(t *testing.T) {
	setupBillingEventControllerTestDB(t)
	seedBillingEventControllerUser(t, 6101, common.RoleCommonUser)
	seedBillingEventControllerUser(t, 6102, common.RoleCommonUser)
	seedBillingEventControllerUser(t, 6199, common.RoleAdminUser)
	seedBillingEventControllerEvent(t, 6101, "user-one", 1001)
	seedBillingEventControllerEvent(t, 6102, "user-two", 1002)
	seedBillingEventControllerEvent(t, 6199, "admin-own", 1003)

	userResp := requestBillingEventsForTest(t, 6101, "/api/billing/events/?scope=all&page_size=10")
	require.True(t, userResp.Success)
	require.Equal(t, 1, userResp.Data.Total)
	require.Equal(t, []string{"user-one"}, billingEventSourceIds(userResp.Data.Items))

	adminOwnResp := requestBillingEventsForTest(t, 6199, "/api/billing/events/?page_size=10")
	require.True(t, adminOwnResp.Success)
	require.Equal(t, 1, adminOwnResp.Data.Total)
	require.Equal(t, []string{"admin-own"}, billingEventSourceIds(adminOwnResp.Data.Items))

	adminAllResp := requestBillingEventsForTest(t, 6199, "/api/billing/events/?scope=all&page_size=10")
	require.True(t, adminAllResp.Success)
	require.Equal(t, 3, adminAllResp.Data.Total)
	require.ElementsMatch(t, []string{"user-one", "user-two", "admin-own"}, billingEventSourceIds(adminAllResp.Data.Items))
}

func TestGetBillingEventSummaryScopeIsolation(t *testing.T) {
	setupBillingEventControllerTestDB(t)
	seedBillingEventControllerUser(t, 6201, common.RoleCommonUser)
	seedBillingEventControllerUser(t, 6202, common.RoleCommonUser)
	seedBillingEventControllerUser(t, 6299, common.RoleAdminUser)
	seedBillingEventControllerEvent(t, 6201, "summary-user-one", 2001)
	seedBillingEventControllerEvent(t, 6202, "summary-user-two", 2002)
	seedBillingEventControllerEvent(t, 6299, "summary-admin-own", 2003)

	userResp := requestBillingEventSummaryForTest(t, 6201, "/api/billing/events/summary?scope=all&start_time=2000&end_time=3000")
	require.True(t, userResp.Success)
	require.EqualValues(t, 1, userResp.Data.Totals.TotalEvents)
	require.EqualValues(t, 100, userResp.Data.Totals.NetQuotaDelta)

	adminOwnResp := requestBillingEventSummaryForTest(t, 6299, "/api/billing/events/summary?start_time=2000&end_time=3000")
	require.True(t, adminOwnResp.Success)
	require.EqualValues(t, 1, adminOwnResp.Data.Totals.TotalEvents)

	adminAllResp := requestBillingEventSummaryForTest(t, 6299, "/api/billing/events/summary?scope=all&start_time=2000&end_time=3000")
	require.True(t, adminAllResp.Success)
	require.EqualValues(t, 3, adminAllResp.Data.Totals.TotalEvents)
	require.EqualValues(t, 300, adminAllResp.Data.Totals.NetQuotaDelta)
}

type billingEventControllerResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Total int                    `json:"total"`
		Items []dto.BillingEventItem `json:"items"`
	} `json:"data"`
}

type billingEventSummaryControllerResponse struct {
	Success bool                            `json:"success"`
	Message string                          `json:"message"`
	Data    dto.BillingEventSummaryResponse `json:"data"`
}

func setupBillingEventControllerTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.BillingEvent{},
		&model.BillingEventRelation{},
		&model.MCPToolCall{},
	))

	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func requestBillingEventsForTest(t *testing.T, userId int, target string) billingEventControllerResponse {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	ctx.Set("id", userId)

	GetBillingEvents(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response billingEventControllerResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Empty(t, response.Message)
	return response
}

func requestBillingEventSummaryForTest(t *testing.T, userId int, target string) billingEventSummaryControllerResponse {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	ctx.Set("id", userId)

	GetBillingEventSummary(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response billingEventSummaryControllerResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Empty(t, response.Message)
	return response
}

func seedBillingEventControllerUser(t *testing.T, userId int, role int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userId,
		Username: "billing_scope_user_" + strconv.Itoa(userId),
		AffCode:  "billing-scope-" + strconv.Itoa(userId),
		Status:   common.UserStatusEnabled,
		Role:     role,
	}).Error)
}

func seedBillingEventControllerEvent(t *testing.T, userId int, sourceId string, createdAt int64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.BillingEvent{
		EventId:     "billing-scope-" + sourceId,
		UserId:      userId,
		Source:      model.BillingEventSourceWalletTopUp,
		SourceId:    sourceId,
		EventType:   model.BillingEventTypeCredit,
		Status:      model.BillingEventStatusSettled,
		AmountQuota: 100,
		QuotaDelta:  100,
		CreatedAt:   createdAt,
	}).Error)
}

func billingEventSourceIds(items []dto.BillingEventItem) []string {
	sourceIds := make([]string, 0, len(items))
	for _, item := range items {
		sourceIds = append(sourceIds, item.SourceId)
	}
	return sourceIds
}
