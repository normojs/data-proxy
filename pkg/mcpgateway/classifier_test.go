package mcpgateway

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyToolUsesAnnotationsFirst(t *testing.T) {
	tool := Tool{
		Name:        "run_command",
		Description: "execute command",
		Annotations: map[string]any{
			"readOnlyHint": true,
		},
	}

	categories := ClassifyTool(tool)
	require.Equal(t, []string{ToolCategoryRead}, categories)
	require.Equal(t, "low", RiskLevel(categories))
}

func TestClassifyToolDetectsWriteAndExec(t *testing.T) {
	writeTool := Tool{Name: "write_file", Description: "write a local file"}
	require.Contains(t, ClassifyTool(writeTool), ToolCategoryWrite)

	execTool := Tool{Name: "shell", Description: "run bash command"}
	categories := ClassifyTool(execTool)
	require.Contains(t, categories, ToolCategoryExec)
	require.Equal(t, "critical", RiskLevel(categories))
}

func TestAuthorizeToolByPermissionMode(t *testing.T) {
	readTool := Tool{Name: "custom_read", Description: "read file content"}
	writeTool := Tool{Name: "custom_write", Description: "write file content"}
	execTool := Tool{Name: "remote_exec", Description: "execute shell command"}

	readPolicy := PolicyForPermissionMode(PermissionReadOnly)
	require.True(t, AuthorizeTool(readPolicy, readTool).Allowed())
	require.False(t, AuthorizeTool(readPolicy, writeTool).Allowed())

	writePolicy := PolicyForPermissionMode(PermissionWrite)
	require.True(t, AuthorizeTool(writePolicy, writeTool).Allowed())
	require.False(t, AuthorizeTool(writePolicy, execTool).Allowed())

	execSafePolicy := PolicyForPermissionMode(PermissionExecSafe)
	require.False(t, AuthorizeTool(execSafePolicy, execTool).Allowed())
	require.NotContains(t, execSafePolicy.AllowedTools, "remote_shell_resize")

	execTrustedPolicy := PolicyForPermissionMode(PermissionExecTrusted)
	require.True(t, AuthorizeTool(execTrustedPolicy, execTool).Allowed())
	require.Contains(t, execTrustedPolicy.AllowedTools, "remote_shell_resize")
}

func TestAuthorizeToolRejectsUnknown(t *testing.T) {
	policy := PolicyForPermissionMode(PermissionExecTrusted)
	decision := AuthorizeTool(policy, Tool{Name: "do_magic", Description: "opaque custom action"})

	require.False(t, decision.Allowed())
	require.Equal(t, ToolCategoryUnknown, decision.Category)
	require.Contains(t, decision.Reason, "unknown")
}

func TestToolSnapshotDiffTracksSchemaChanges(t *testing.T) {
	first := BuildToolSnapshots([]Tool{{
		Name:        "read_file",
		InputSchema: map[string]any{"required": []any{"path"}},
	}})
	second := BuildToolSnapshots([]Tool{{
		Name:        "read_file",
		InputSchema: map[string]any{"required": []any{"path", "limit"}},
	}})

	diff := DiffToolSnapshots(first, second)
	require.Empty(t, diff.Added)
	require.Empty(t, diff.Removed)
	require.Len(t, diff.Changed, 1)
	require.NotEqual(t, diff.Changed[0].Before.SchemaHash, diff.Changed[0].After.SchemaHash)
}

func TestToolCallAuditEventHashesArguments(t *testing.T) {
	req := ToolCallRequest{
		Subject: Subject{
			UserId:    100,
			AppId:     200,
			RequestId: "req-1",
		},
		Tool: Tool{Name: "read_file", Description: "read file"},
		Arguments: map[string]any{
			"path": "/workspace/main.go",
		},
	}
	decision := AuthorizeTool(PolicyForPermissionMode(PermissionReadOnly), req.Tool)

	event := NewToolCallAuditEvent(req, decision)
	require.Equal(t, AuditActionToolCall, event.Action)
	require.Equal(t, DecisionAllow, event.Decision)
	require.Equal(t, "read_file", event.ToolName)
	require.NotEmpty(t, event.ArgumentHash)
	require.NotContains(t, event.ArgumentHash, "/workspace/main.go")
	require.Greater(t, event.BytesIn, int64(0))
}
