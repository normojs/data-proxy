package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	"github.com/QuantumNous/new-api/pkg/mcpgateway"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTunnelBillingAuditOnlyByDefault(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelMCPApp(t, model.TunnelApp{})
	connection := seedTunnelMCPConnection(t, app, "")
	audit := seedTunnelBillingAudit(t, model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		Action:              model.TunnelAuditActionMCPToolCall,
		Decision:            mcpgateway.DecisionAllow,
		RequestId:           "tunnel-billing-audit-only",
		ToolName:            "read_file",
	})

	require.NoError(t, recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelMCP, app, connection, audit))
	require.NoError(t, recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelMCP, app, connection, audit))

	events := tunnelBillingEventsByAudit(t, db, model.BillingEventSourceTunnelMCP, audit.Id)
	require.Len(t, events, 1)
	require.Equal(t, model.BillingEventTypeAudit, events[0].EventType)
	require.Equal(t, tunnelBillingUnitPerCall, events[0].PriceUnit)
	require.Equal(t, 0, events[0].AmountQuota)
	require.Equal(t, 0, events[0].QuotaDelta)
}

func TestTunnelBillingIgnoresRateLimitConfigWithoutSettlement(t *testing.T) {
	db := setupTunnelTestDB(t)
	app := seedTunnelMCPApp(t, model.TunnelApp{
		BillingJson: `{"rate_limit":{"max_requests_per_minute":10}}`,
	})
	connection := seedTunnelMCPConnection(t, app, "")
	audit := seedTunnelBillingAudit(t, model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		Action:              model.TunnelAuditActionMCPToolCall,
		Decision:            mcpgateway.DecisionAllow,
		RequestId:           "tunnel-billing-rate-limit-only",
		ToolName:            "read_file",
	})

	require.NoError(t, recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelMCP, app, connection, audit))

	events := tunnelBillingEventsByAudit(t, db, model.BillingEventSourceTunnelMCP, audit.Id)
	require.Len(t, events, 1)
	require.Equal(t, model.BillingEventTypeAudit, events[0].EventType)
}

func TestTunnelBillingCreatesMCPPerCallDebitWhenConfigured(t *testing.T) {
	db := setupTunnelTestDB(t)
	seedTunnelBillingUser(t, 100, 1000)
	app := seedTunnelMCPApp(t, model.TunnelApp{
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25,"price_unit":"per_call"}}`,
	})
	connection := seedTunnelMCPConnection(t, app, "")
	connection.AgentTokenId = 77
	require.NoError(t, db.Model(&model.TunnelConnection{}).Where("id = ?", connection.Id).Update("agent_token_id", connection.AgentTokenId).Error)
	audit := seedTunnelBillingAudit(t, model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		SessionId:           "tmcp-billing-session",
		UserId:              app.UserId,
		Action:              model.TunnelAuditActionMCPToolCall,
		Decision:            mcpgateway.DecisionAllow,
		RequestId:           "tunnel-billing-mcp-debit",
		ToolName:            "write_file",
		BytesIn:             128,
		BytesOut:            256,
		DurationMS:          240,
	})

	require.NoError(t, recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelMCP, app, connection, audit))
	require.NoError(t, recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelMCP, app, connection, audit))

	events := tunnelBillingEventsByAudit(t, db, model.BillingEventSourceTunnelMCP, audit.Id)
	require.Len(t, events, 2)
	auditEvent := tunnelBillingEventByType(t, events, model.BillingEventTypeAudit)
	debitEvent := tunnelBillingEventByType(t, events, model.BillingEventTypeDebit)
	require.Equal(t, 0, auditEvent.AmountQuota)
	require.Equal(t, 25, debitEvent.AmountQuota)
	require.Equal(t, -25, debitEvent.QuotaDelta)
	require.Equal(t, connection.AgentTokenId, debitEvent.TokenId)
	require.Equal(t, tunnelBillingUnitPerCall, debitEvent.PriceUnit)
	require.Equal(t, fmt.Sprintf("tunnel_mcp:audit:%d:settlement", audit.Id), debitEvent.EventId)
	require.Contains(t, debitEvent.Metadata, `"settlement_amount_quota":25`)
	require.Contains(t, debitEvent.Metadata, `"quota_per_call":25`)
	require.Contains(t, debitEvent.Metadata, `"usage_kind":"tunnel"`)

	user := tunnelBillingUser(t, db, 100)
	require.Equal(t, 975, user.Quota)
	require.Equal(t, 25, user.UsedQuota)
	require.Equal(t, 1, user.RequestCount)
}

func TestTunnelBillingCreatesHTTPTrafficDebitWhenConfigured(t *testing.T) {
	db := setupTunnelTestDB(t)
	seedTunnelBillingUser(t, 100, 1000)
	app := seedTunnelHTTPApp(t, model.TunnelApp{
		BillingJson: `{"settlement":{"enabled":true,"quota_per_request":3,"quota_per_mib_in":10,"quota_per_mib_out":20,"quota_per_second":4,"price_unit":"request_traffic"}}`,
	}, bridgepolicy.Policy{
		AllowedTools: []string{"http_tunnel"},
	})
	connection := seedTunnelHTTPConnection(t, app, "")
	audit := seedTunnelBillingAudit(t, model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		SessionId:           "http-billing-session",
		UserId:              app.UserId,
		Action:              model.TunnelAuditActionProxyRequest,
		Decision:            mcpgateway.DecisionAllow,
		RequestId:           "tunnel-billing-http-debit",
		Method:              "GET",
		Path:                "/events",
		BytesIn:             tunnelBillingMiB / 2,
		BytesOut:            tunnelBillingMiB,
		DurationMS:          2500,
	})

	require.NoError(t, recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelHTTP, app, connection, audit))

	events := tunnelBillingEventsByAudit(t, db, model.BillingEventSourceTunnelHTTP, audit.Id)
	require.Len(t, events, 2)
	debitEvent := tunnelBillingEventByType(t, events, model.BillingEventTypeDebit)
	require.Equal(t, 38, debitEvent.AmountQuota)
	require.Equal(t, -38, debitEvent.QuotaDelta)
	require.Equal(t, tunnelBillingUnitTraffic, debitEvent.PriceUnit)
	require.Contains(t, debitEvent.Metadata, `"quota_per_request":3`)
	require.Contains(t, debitEvent.Metadata, `"quota_per_mib_in":10`)
	require.Contains(t, debitEvent.Metadata, `"quota_per_mib_out":20`)
	require.Contains(t, debitEvent.Metadata, `"quota_per_second":4`)

	user := tunnelBillingUser(t, db, 100)
	require.Equal(t, 962, user.Quota)
	require.Equal(t, 38, user.UsedQuota)
	require.Equal(t, 1, user.RequestCount)
}

func TestTunnelBillingDoesNotDebitDeniedEventsUnlessConfigured(t *testing.T) {
	db := setupTunnelTestDB(t)
	seedTunnelBillingUser(t, 100, 1000)
	app := seedTunnelMCPApp(t, model.TunnelApp{
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25}}`,
	})
	connection := seedTunnelMCPConnection(t, app, "")
	audit := seedTunnelBillingAudit(t, model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		Action:              model.TunnelAuditActionMCPToolCall,
		Decision:            mcpgateway.DecisionDeny,
		RequestId:           "tunnel-billing-deny",
		ToolName:            "write_file",
	})

	require.NoError(t, recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelMCP, app, connection, audit))

	events := tunnelBillingEventsByAudit(t, db, model.BillingEventSourceTunnelMCP, audit.Id)
	require.Len(t, events, 1)
	require.Equal(t, model.BillingEventTypeAudit, events[0].EventType)
	require.Equal(t, 1000, tunnelBillingUser(t, db, 100).Quota)
}

func TestTunnelBillingLedgerOnlyDoesNotDecreaseUserQuota(t *testing.T) {
	db := setupTunnelTestDB(t)
	seedTunnelBillingUser(t, 100, 1000)
	app := seedTunnelMCPApp(t, model.TunnelApp{
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25,"ledger_only":true}}`,
	})
	connection := seedTunnelMCPConnection(t, app, "")
	audit := seedTunnelBillingAudit(t, model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		Action:              model.TunnelAuditActionMCPToolCall,
		Decision:            mcpgateway.DecisionAllow,
		RequestId:           "tunnel-billing-ledger-only",
		ToolName:            "read_file",
	})

	require.NoError(t, recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelMCP, app, connection, audit))

	events := tunnelBillingEventsByAudit(t, db, model.BillingEventSourceTunnelMCP, audit.Id)
	require.Len(t, events, 2)
	debitEvent := tunnelBillingEventByType(t, events, model.BillingEventTypeDebit)
	require.Equal(t, 25, debitEvent.AmountQuota)
	require.Contains(t, debitEvent.Metadata, `"settlement_balance_debit":false`)
	user := tunnelBillingUser(t, db, 100)
	require.Equal(t, 1000, user.Quota)
	require.Equal(t, 0, user.UsedQuota)
	require.Equal(t, 0, user.RequestCount)
}

func TestTunnelBillingSettlementAllowsPostpaidNegativeBalance(t *testing.T) {
	db := setupTunnelTestDB(t)
	seedTunnelBillingUser(t, 100, 10)
	app := seedTunnelMCPApp(t, model.TunnelApp{
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25}}`,
	})
	connection := seedTunnelMCPConnection(t, app, "")
	audit := seedTunnelBillingAudit(t, model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		Action:              model.TunnelAuditActionMCPToolCall,
		Decision:            mcpgateway.DecisionAllow,
		RequestId:           "tunnel-billing-postpaid-negative",
		ToolName:            "read_file",
	})

	require.NoError(t, recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelMCP, app, connection, audit))

	user := tunnelBillingUser(t, db, 100)
	require.Equal(t, -15, user.Quota)
	require.Equal(t, 25, user.UsedQuota)
	require.Equal(t, 1, user.RequestCount)
}

func TestTunnelBillingAutoDisablesAppOnOverdue(t *testing.T) {
	db := setupTunnelTestDB(t)
	seedTunnelBillingUser(t, 100, 10)
	app := seedTunnelMCPApp(t, model.TunnelApp{
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25,"auto_disable_on_overdue":true}}`,
	})
	connection := seedTunnelMCPConnection(t, app, "")
	audit := seedTunnelBillingAudit(t, model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		SessionId:           "tmcp-overdue-session",
		Action:              model.TunnelAuditActionMCPToolCall,
		Decision:            mcpgateway.DecisionAllow,
		RequestId:           "tunnel-billing-overdue-disable",
		ToolName:            "read_file",
		Path:                "workspace/readme.md",
	})

	require.NoError(t, recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelMCP, app, connection, audit))
	require.NoError(t, recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelMCP, app, connection, audit))

	user := tunnelBillingUser(t, db, 100)
	require.Equal(t, -15, user.Quota)
	require.Equal(t, 25, user.UsedQuota)
	require.Equal(t, 1, user.RequestCount)

	var updatedApp model.TunnelApp
	require.NoError(t, db.First(&updatedApp, app.Id).Error)
	require.Equal(t, model.TunnelAppStatusDisabled, updatedApp.Status)
	require.Contains(t, updatedApp.LastError, "auto-disabled")

	var disableAudit model.TunnelAuditLog
	require.NoError(t, db.Where("app_id = ? AND action = ? AND reason = ?", app.Id, model.TunnelAuditActionUpdate, tunnelBillingReasonOverdueDisable).First(&disableAudit).Error)
	require.Equal(t, mcpgateway.DecisionAllow, disableAudit.Decision)
	require.Equal(t, connection.Id, disableAudit.ConnectionId)
	require.Equal(t, audit.RequestId, disableAudit.RequestId)
	require.Contains(t, disableAudit.MetadataJson, `"auto_disable_on_overdue":true`)
	require.Contains(t, disableAudit.MetadataJson, `"current_quota":-15`)

	var auditEvents []model.BillingEvent
	require.NoError(t, db.Where("source = ? AND source_id = ?", model.BillingEventSourceTunnelMCP, fmt.Sprintf("audit:%d", disableAudit.Id)).Find(&auditEvents).Error)
	require.Len(t, auditEvents, 1)
	require.Equal(t, model.BillingEventTypeAudit, auditEvents[0].EventType)
}

func TestTunnelBillingPositiveBalanceGuardRequiresDebitPolicy(t *testing.T) {
	_ = setupTunnelTestDB(t)
	seedTunnelBillingUser(t, 100, 0)
	seedTunnelBillingUser(t, 101, 10)

	policy, quota, err := checkTunnelBillingPositiveBalance(model.TunnelApp{
		UserId:      100,
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25,"require_positive_balance":true}}`,
	})
	require.ErrorIs(t, err, ErrTunnelBillingInsufficient)
	require.Equal(t, 0, quota)
	require.True(t, policy.RequirePositiveBalance)
	require.True(t, policy.DeductBalance)

	_, _, err = checkTunnelBillingPositiveBalance(model.TunnelApp{
		UserId:      100,
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25,"require_positive_balance":true,"ledger_only":true}}`,
	})
	require.NoError(t, err)

	_, _, err = checkTunnelBillingPositiveBalance(model.TunnelApp{
		UserId:      100,
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25,"require_positive_balance":true,"deduct_balance":false}}`,
	})
	require.NoError(t, err)

	_, _, err = checkTunnelBillingPositiveBalance(model.TunnelApp{
		UserId:      100,
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25}}`,
	})
	require.NoError(t, err)

	_, _, err = checkTunnelBillingBalance(model.TunnelApp{
		UserId:      101,
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25,"require_positive_balance":true}}`,
	}, 25)
	require.NoError(t, err)

	policy, quota, err = checkTunnelBillingBalance(model.TunnelApp{
		UserId:      101,
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25,"require_positive_balance":true,"require_sufficient_balance":true}}`,
	}, 25)
	require.ErrorIs(t, err, ErrTunnelBillingInsufficient)
	require.Equal(t, 10, quota)
	require.True(t, policy.RequireSufficientBalance)
	require.Equal(t, 25, policy.EstimatedMinQuota)

	_, _, err = checkTunnelBillingBalance(model.TunnelApp{
		UserId:      101,
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25,"require_positive_balance":true,"require_sufficient_balance":true}}`,
	}, 0)
	require.NoError(t, err)
}

func TestTunnelBillingPreflightQuotaUsesActionMinimum(t *testing.T) {
	mcpApp := model.TunnelApp{
		AppType:     model.TunnelAppTypeMCPCode,
		BillingJson: `{"settlement":{"enabled":true,"quota_per_call":25,"quota_per_request":9,"min_quota":30}}`,
	}
	require.Equal(t, 30, tunnelBillingPreflightQuota(mcpApp, model.TunnelAuditActionMCPToolCall, 0))
	require.Equal(t, 0, tunnelBillingPreflightQuota(mcpApp, model.TunnelAuditActionRateLimit, 0))

	httpApp := model.TunnelApp{
		AppType:     model.TunnelAppTypeHTTP,
		BillingJson: `{"settlement":{"enabled":true,"quota_per_request":3,"quota_per_mib_in":10,"min_quota":1}}`,
	}
	require.Equal(t, 8, tunnelBillingPreflightQuota(httpApp, model.TunnelAuditActionProxyRequest, tunnelBillingMiB/2))
}

func seedTunnelBillingUser(t *testing.T, userId int, quota int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userId,
		Username: fmt.Sprintf("tunnel_billing_user_%d", userId),
		Password: "password123",
		Group:    "default",
		AffCode:  fmt.Sprintf("tunnel-billing-aff-%d", userId),
		Quota:    quota,
	}).Error)
}

func tunnelBillingUser(t *testing.T, db *gorm.DB, userId int) model.User {
	t.Helper()
	var user model.User
	require.NoError(t, db.First(&user, "id = ?", userId).Error)
	return user
}

func seedTunnelBillingAudit(t *testing.T, audit model.TunnelAuditLog) model.TunnelAuditLog {
	t.Helper()
	require.NoError(t, model.CreateTunnelAuditLog(&audit))
	require.NotZero(t, audit.Id)
	return audit
}

func tunnelBillingEventsByAudit(t *testing.T, db *gorm.DB, source string, auditId int64) []model.BillingEvent {
	t.Helper()
	var events []model.BillingEvent
	require.NoError(t, db.Where("source = ? AND source_id = ?", source, fmt.Sprintf("audit:%d", auditId)).Order("id asc").Find(&events).Error)
	return events
}

func tunnelBillingEventByType(t *testing.T, events []model.BillingEvent, eventType string) model.BillingEvent {
	t.Helper()
	for _, event := range events {
		if event.EventType == eventType {
			return event
		}
	}
	t.Fatalf("missing tunnel billing event type %s in %#v", eventType, events)
	return model.BillingEvent{}
}
