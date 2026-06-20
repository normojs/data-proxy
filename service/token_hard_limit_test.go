package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	mcpexecutor "github.com/QuantumNous/new-api/pkg/mcp/executor"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPreConsumeTokenQuotaHardLimitRejectsUnlimitedToken(t *testing.T) {
	truncate(t)
	token := seedHardLimitToken(t, 9101, 9101, "sk-hard-limit-reject", 100, true, true)
	relayInfo := &relaycommon.RelayInfo{
		TokenId:                    token.Id,
		TokenKey:                   token.Key,
		TokenUnlimited:             true,
		TokenQuotaHardLimitEnabled: true,
	}

	err := PreConsumeTokenQuota(relayInfo, 150)
	require.Error(t, err)
	require.Contains(t, err.Error(), "token quota is not enough")

	var reloaded model.Token
	require.NoError(t, model.DB.First(&reloaded, token.Id).Error)
	require.Equal(t, 100, reloaded.RemainQuota)
	require.Equal(t, 0, reloaded.UsedQuota)
}

func TestPreConsumeBillingHardLimitDisablesTrustBypass(t *testing.T) {
	truncate(t)
	userId := 9102
	quota := int(30 * common.QuotaPerUnit)
	seedUser(t, userId, quota)
	token := seedHardLimitToken(t, 9102, userId, "sk-hard-limit-trust", quota, true, true)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	relayInfo := &relaycommon.RelayInfo{
		UserId:                     userId,
		TokenId:                    token.Id,
		TokenKey:                   token.Key,
		TokenUnlimited:             true,
		TokenQuotaHardLimitEnabled: true,
		UserQuota:                  quota,
	}

	apiErr := PreConsumeBilling(c, 1000, relayInfo)
	require.Nil(t, apiErr)
	require.NotNil(t, relayInfo.Billing)
	require.Equal(t, 1000, relayInfo.FinalPreConsumedQuota)
	require.Equal(t, quota-1000, currentTokenRemainQuota(t, token.Id))
	require.Equal(t, quota-1000, currentUserQuota(t, userId))
}

func TestMCPPerCallBillingRespectsUnlimitedTokenHardLimit(t *testing.T) {
	setupMCPProxyServiceTestDB(t)
	user, token := seedMCPBillingUserAndToken(t, 100000, 1000, true)
	require.NoError(t, model.DB.Model(token).Update("quota_hard_limit_enabled", true).Error)
	token.QuotaHardLimitEnabled = true

	tool := &model.MCPTool{
		Name:         mcpexecutor.BuiltinToolServerTime,
		DisplayName:  "Server Time",
		Description:  "charges once per MCP call",
		Category:     "builtin",
		Source:       model.MCPToolSourceBuiltin,
		InputSchema:  `{"type":"object","properties":{}}`,
		PriceUnit:    model.MCPToolPriceUnitPerCall,
		PricePerCall: 0.001,
		Status:       model.MCPToolStatusEnabled,
	}
	require.NoError(t, model.CreateMCPTool(tool))

	restoreExecutorRegistry := setMCPExecutorRegistryForTest(mcpexecutor.NewRegistry(mcpexecutor.NewBuiltinExecutor()))
	t.Cleanup(restoreExecutorRegistry)

	resp, err := CallMCPTool(MCPToolCallRequest{
		UserId:                     user.Id,
		TokenId:                    token.Id,
		TokenKey:                   token.Key,
		TokenUnlimited:             token.UnlimitedQuota,
		TokenQuotaHardLimitEnabled: token.QuotaHardLimitEnabled,
		TokenQuota:                 token.RemainQuota,
		UsingGroup:                 "default",
		RequestId:                  "mcp-hard-limit-per-call",
		RequestIP:                  "127.0.0.1",
		Params: dto.MCPToolCallParams{
			Name:      tool.Name,
			Arguments: map[string]any{},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Result)

	var call model.MCPToolCall
	require.NoError(t, model.DB.Where("request_id = ?", "mcp-hard-limit-per-call").First(&call).Error)
	require.Equal(t, 500, call.Quota)
	require.Equal(t, model.MCPToolCallStatusSuccess, call.Status)

	assertMCPQuotaStateWithHardLimit(t, user.Id, token.Id, 100000, 1000, 500, 1)
	assertMCPBillingEvent(t, "mcp-hard-limit-per-call", call.Id, user.Id, token.Id, 500)
}

func seedHardLimitToken(t *testing.T, id int, userId int, key string, remainQuota int, unlimited bool, hardLimit bool) *model.Token {
	t.Helper()
	token := &model.Token{
		Id:                    id,
		UserId:                userId,
		Key:                   key,
		Name:                  "hard_limit_token",
		Status:                common.TokenStatusEnabled,
		RemainQuota:           remainQuota,
		UnlimitedQuota:        unlimited,
		QuotaHardLimitEnabled: hardLimit,
		ExpiredTime:           -1,
	}
	require.NoError(t, model.DB.Create(token).Error)
	return token
}

func currentUserQuota(t *testing.T, userId int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota").Where("id = ?", userId).First(&user).Error)
	return user.Quota
}

func currentTokenRemainQuota(t *testing.T, tokenId int) int {
	t.Helper()
	var token model.Token
	require.NoError(t, model.DB.Select("remain_quota").Where("id = ?", tokenId).First(&token).Error)
	return token.RemainQuota
}

func assertMCPQuotaStateWithHardLimit(t *testing.T, userId int, tokenId int, originalUserQuota int, originalTokenQuota int, chargedQuota int, requestCount int) {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota", "used_quota", "request_count").Where("id = ?", userId).First(&user).Error)
	require.Equal(t, originalUserQuota-chargedQuota, user.Quota)
	require.Equal(t, chargedQuota, user.UsedQuota)
	require.Equal(t, requestCount, user.RequestCount)

	var token model.Token
	require.NoError(t, model.DB.Select("remain_quota", "used_quota").Where("id = ?", tokenId).First(&token).Error)
	require.Equal(t, originalTokenQuota-chargedQuota, token.RemainQuota)
	require.Equal(t, chargedQuota, token.UsedQuota)
}
