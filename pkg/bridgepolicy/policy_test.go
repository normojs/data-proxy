package bridgepolicy

import (
	"testing"
)

func TestValidateToolDefaultsDenyWrite(t *testing.T) {
	if err := ValidateTool(Policy{}, "remote_read"); err != nil {
		t.Fatalf("remote_read should be allowed by default: %v", err)
	}
	err := ValidateTool(Policy{}, "remote_write")
	if ErrorCode(err) != ErrorCodeWriteDisabled {
		t.Fatalf("write should be disabled by default, got %v code=%s", err, ErrorCode(err))
	}
	err = ValidateTool(Policy{}, "remote_shell")
	if ErrorCode(err) != ErrorCodeToolNotAllowed {
		t.Fatalf("unknown tool should be denied by default, got %v code=%s", err, ErrorCode(err))
	}
}

func TestValidateToolAllowedTools(t *testing.T) {
	policy := Policy{AllowedTools: []string{"remote_read", "mcp_proxy"}}
	if err := ValidateTool(policy, "remote_read"); err != nil {
		t.Fatalf("remote_read should be allowed: %v", err)
	}
	if err := ValidateTool(policy, "mcp_proxy.tools_call"); err != nil {
		t.Fatalf("mcp_proxy.tools_call should be allowed by family entry: %v", err)
	}
	err := ValidateTool(policy, "remote_tree")
	if ErrorCode(err) != ErrorCodeToolNotAllowed {
		t.Fatalf("remote_tree should be denied, got %v code=%s", err, ErrorCode(err))
	}
}

func TestValidateMCPTargetDefaultsLoopback(t *testing.T) {
	for _, target := range []string{
		"http://localhost:3001/mcp",
		"http://127.0.0.1:3001/mcp",
		"http://[::1]:3001/mcp",
	} {
		if err := ValidateMCPTarget(Policy{}, target); err != nil {
			t.Fatalf("loopback target should be allowed: %s: %v", target, err)
		}
	}
	err := ValidateMCPTarget(Policy{}, "https://example.com/mcp")
	if ErrorCode(err) != ErrorCodeMCPTargetForbidden {
		t.Fatalf("non-loopback target should be denied, got %v code=%s", err, ErrorCode(err))
	}
}

func TestValidateMCPTargetAllowlist(t *testing.T) {
	policy := Policy{MCPAllowedTargets: []string{"https://example.com/mcp", "api.internal:9443"}}
	if err := ValidateMCPTarget(policy, "https://example.com/mcp/v1"); err != nil {
		t.Fatalf("URL prefix should be allowed: %v", err)
	}
	if err := ValidateMCPTarget(policy, "https://api.internal:9443/rpc"); err != nil {
		t.Fatalf("host:port should be allowed: %v", err)
	}
	err := ValidateMCPTarget(policy, "https://example.com/other")
	if ErrorCode(err) != ErrorCodeMCPTargetForbidden {
		t.Fatalf("path outside prefix should be denied, got %v code=%s", err, ErrorCode(err))
	}
	err = ValidateMCPTarget(policy, "https://example.com/mcp-other")
	if ErrorCode(err) != ErrorCodeMCPTargetForbidden {
		t.Fatalf("sibling path should be denied, got %v code=%s", err, ErrorCode(err))
	}
}

func TestApplyArgumentLimits(t *testing.T) {
	args := ApplyArgumentLimits(Policy{
		MaxResultBytes:   1024,
		MaxScanFileBytes: 2048,
		MaxResults:       10,
		TreeDepth:        2,
	}, "remote_tree", map[string]any{
		"max_results": 50,
		"depth":       9,
	})
	if args["max_results"] != 10 || args["depth"] != 2 {
		t.Fatalf("tree limits were not capped: %#v", args)
	}
	limits, ok := args["_bridge_policy_limits"].(map[string]any)
	if !ok {
		t.Fatalf("expected hidden policy limits, got %#v", args["_bridge_policy_limits"])
	}
	if limits["max_result_bytes"] != 1024 || limits["max_scan_file_bytes"] != 2048 {
		t.Fatalf("hidden limits mismatch: %#v", limits)
	}
}

func TestValidateResultSize(t *testing.T) {
	err := ValidateResultSize(Policy{MaxResultBytes: 5}, 6)
	if ErrorCode(err) != ErrorCodeResultTooLarge {
		t.Fatalf("expected result too large, got %v code=%s", err, ErrorCode(err))
	}
}
