package bridgepolicy

import (
	"testing"
)

func TestValidateToolDefaultsDenyWrite(t *testing.T) {
	for _, tool := range []string{
		"remote_read",
		"remote_project_info",
		"remote_get_related_files",
		"remote_git_status",
		"remote_git_diff",
		"remote_git_log",
	} {
		if err := ValidateTool(Policy{}, tool); err != nil {
			t.Fatalf("%s should be allowed by default: %v", tool, err)
		}
	}
	err := ValidateTool(Policy{}, "remote_write")
	if ErrorCode(err) != ErrorCodeWriteDisabled {
		t.Fatalf("write should be disabled by default, got %v code=%s", err, ErrorCode(err))
	}
	err = ValidateTool(Policy{}, "remote_shell")
	if ErrorCode(err) != ErrorCodeToolNotAllowed {
		t.Fatalf("unknown tool should be denied by default, got %v code=%s", err, ErrorCode(err))
	}
	err = ValidateTool(Policy{}, "remote_shell_resize")
	if ErrorCode(err) != ErrorCodeToolNotAllowed {
		t.Fatalf("trusted shell resize should be denied by default, got %v code=%s", err, ErrorCode(err))
	}
}

func TestValidateToolAllowedTools(t *testing.T) {
	policy := Policy{AllowedTools: []string{"remote_read", "mcp_proxy", "remote_shell_resize"}}
	if err := ValidateTool(policy, "remote_read"); err != nil {
		t.Fatalf("remote_read should be allowed: %v", err)
	}
	if err := ValidateTool(policy, "mcp_proxy.tools_call"); err != nil {
		t.Fatalf("mcp_proxy.tools_call should be allowed by family entry: %v", err)
	}
	if err := ValidateTool(policy, "remote_shell_resize"); err != nil {
		t.Fatalf("remote_shell_resize should be allowed when explicitly listed: %v", err)
	}
	err := ValidateTool(policy, "remote_tree")
	if ErrorCode(err) != ErrorCodeToolNotAllowed {
		t.Fatalf("remote_tree should be denied, got %v code=%s", err, ErrorCode(err))
	}
}

func TestValidateToolHTTPFamily(t *testing.T) {
	policy := Policy{AllowedTools: []string{"http_tunnel"}}
	if err := ValidateTool(policy, "http_tunnel.request"); err != nil {
		t.Fatalf("http_tunnel.request should be allowed by family entry: %v", err)
	}
	err := ValidateTool(policy, "mcp_proxy.tools_call")
	if ErrorCode(err) != ErrorCodeToolNotAllowed {
		t.Fatalf("other tool families should be denied, got %v code=%s", err, ErrorCode(err))
	}
}

func TestValidateToolTCPFamily(t *testing.T) {
	policy := Policy{AllowedTools: []string{"tcp_tunnel"}}
	if err := ValidateTool(policy, "tcp_tunnel.connect"); err != nil {
		t.Fatalf("tcp_tunnel.connect should be allowed by family entry: %v", err)
	}
	err := ValidateTool(policy, "http_tunnel.request")
	if ErrorCode(err) != ErrorCodeToolNotAllowed {
		t.Fatalf("other tool families should be denied, got %v code=%s", err, ErrorCode(err))
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

func TestValidateHTTPTargetDefaultsLoopback(t *testing.T) {
	for _, target := range []string{
		"http://localhost:8080/api",
		"http://127.0.0.1:8080/api",
		"http://[::1]:8080/api",
	} {
		if err := ValidateHTTPTarget(Policy{}, target); err != nil {
			t.Fatalf("loopback HTTP target should be allowed: %s: %v", target, err)
		}
	}
	err := ValidateHTTPTarget(Policy{}, "http://192.168.0.10:8080/api")
	if ErrorCode(err) != ErrorCodeHTTPTargetForbidden {
		t.Fatalf("non-loopback HTTP target should be denied, got %v code=%s", err, ErrorCode(err))
	}
}

func TestValidateHTTPTargetAllowlist(t *testing.T) {
	policy := Policy{HTTPAllowedTargets: []string{"http://192.168.0.10:8080/api", "dev.internal:3000"}}
	if err := ValidateHTTPTarget(policy, "http://192.168.0.10:8080/api/users"); err != nil {
		t.Fatalf("HTTP URL prefix should be allowed: %v", err)
	}
	if err := ValidateHTTPTarget(policy, "https://dev.internal:3000/health"); err != nil {
		t.Fatalf("HTTP host:port allowlist should allow any scheme/path: %v", err)
	}
	err := ValidateHTTPTarget(policy, "http://192.168.0.10:8080/other")
	if ErrorCode(err) != ErrorCodeHTTPTargetForbidden {
		t.Fatalf("HTTP path outside prefix should be denied, got %v code=%s", err, ErrorCode(err))
	}
}

func TestValidateHTTPTargetDenylistTakesPrecedence(t *testing.T) {
	policy := Policy{
		HTTPAllowedTargets: []string{"*"},
		HTTPDeniedTargets:  []string{"http://127.0.0.1:8080/admin", "169.254.169.254"},
		HTTPDeniedPorts:    []int{3306, 6379},
	}
	if err := ValidateHTTPTarget(policy, "http://127.0.0.1:8080/api"); err != nil {
		t.Fatalf("non-denied target should be allowed: %v", err)
	}
	for _, target := range []string{
		"http://127.0.0.1:8080/admin/users",
		"http://169.254.169.254/latest/meta-data",
		"http://127.0.0.1:6379/",
		"http://192.168.0.10:3306/",
	} {
		err := ValidateHTTPTarget(policy, target)
		if ErrorCode(err) != ErrorCodeHTTPTargetForbidden {
			t.Fatalf("denied target should be rejected: %s got %v code=%s", target, err, ErrorCode(err))
		}
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
