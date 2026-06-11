package executor

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRemoteBridgePolicyTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.BridgeClient{}, &model.BridgeAuditLog{}))
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

func createBridgePolicyClient(t *testing.T, clientId string, policy bridgepolicy.Policy) {
	t.Helper()
	rawPolicy, err := bridgepolicy.Marshal(policy)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.BridgeClient{
		ClientId: clientId,
		UserId:   1,
		Name:     clientId,
		Policy:   rawPolicy,
		Status:   model.BridgeClientStatusOnline,
	}).Error)
}

func TestRemoteBridgeExecutorDeniesWriteByServerPolicy(t *testing.T) {
	setupRemoteBridgePolicyTestDB(t)
	createBridgePolicyClient(t, "bridge-policy-deny-write", bridgepolicy.Policy{})

	hub := bridge.NewHub()
	outbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "session-policy-deny-write",
		ClientId:     "bridge-policy-deny-write",
		UserId:       1,
		Capabilities: []string{"remote_write"},
		Send:         outbound,
	})
	executor := &RemoteBridgeExecutor{Hub: hub}

	result, err := executor.Execute(context.Background(), Request{
		UserId:    1,
		TokenId:   2,
		RequestId: "policy-deny-write",
		Tool: model.MCPTool{
			Name:     "remote_write",
			IsRemote: true,
		},
		Arguments: map[string]any{"file_path": "README.md", "content": "nope"},
	})
	require.Error(t, err)
	require.Equal(t, bridgepolicy.ErrorCodeWriteDisabled, ErrorCode(err))
	require.Equal(t, "session-policy-deny-write", result.BridgeSessionId)
	require.Equal(t, "bridge-policy-deny-write", result.TargetClient)
	select {
	case msg := <-outbound:
		t.Fatalf("policy denial should not forward to bridge daemon: %#v", msg)
	default:
	}

	var audit model.BridgeAuditLog
	require.NoError(t, model.DB.Where("request_id = ?", "policy-deny-write").First(&audit).Error)
	require.Equal(t, model.BridgeAuditStatusError, audit.Status)
	require.Equal(t, bridgepolicy.ErrorCodeWriteDisabled, audit.ErrorCode)
}

func TestRemoteBridgeExecutorAppliesServerPolicyLimits(t *testing.T) {
	setupRemoteBridgePolicyTestDB(t)
	createBridgePolicyClient(t, "bridge-policy-limits", bridgepolicy.Policy{
		MaxResultBytes:   1024,
		MaxScanFileBytes: 2048,
		MaxResults:       5,
		TreeDepth:        2,
	})

	hub := bridge.NewHub()
	outbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "session-policy-limits",
		ClientId:     "bridge-policy-limits",
		UserId:       1,
		Capabilities: []string{"remote_tree"},
		Send:         outbound,
	})
	executor := &RemoteBridgeExecutor{Hub: hub}

	done := make(chan error, 1)
	go func() {
		_, err := executor.Execute(context.Background(), Request{
			UserId:    1,
			TokenId:   2,
			RequestId: "policy-limits",
			Tool: model.MCPTool{
				Name:     "remote_tree",
				IsRemote: true,
			},
			Arguments: map[string]any{"max_results": 50, "depth": 9},
		})
		done <- err
	}()

	msg := <-outbound
	call := msg.Data.(dto.BridgeToolCallRequest)
	require.Equal(t, "remote_tree", call.ToolName)
	require.Equal(t, 5, call.Arguments["max_results"])
	require.Equal(t, 2, call.Arguments["depth"])
	limits := call.Arguments["_bridge_policy_limits"].(map[string]any)
	require.Equal(t, 1024, limits["max_result_bytes"])
	require.Equal(t, 2048, limits["max_scan_file_bytes"])
	require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: "ok"}},
		ResultSize: 2,
	}))
	require.NoError(t, <-done)
}
