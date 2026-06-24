package bridgepolicy

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	ErrorCodeToolNotAllowed      = "BRIDGE_POLICY_TOOL_NOT_ALLOWED"
	ErrorCodeWriteDisabled       = "BRIDGE_POLICY_WRITE_DISABLED"
	ErrorCodeMCPTargetForbidden  = "BRIDGE_POLICY_MCP_TARGET_FORBIDDEN"
	ErrorCodeHTTPTargetForbidden = "BRIDGE_POLICY_HTTP_TARGET_FORBIDDEN"
	ErrorCodeResultTooLarge      = "BRIDGE_POLICY_RESULT_TOO_LARGE"
)

type Policy struct {
	AllowedTools       []string `json:"allowed_tools,omitempty"`
	AllowWrite         bool     `json:"allow_write"`
	MaxResultBytes     int      `json:"max_result_bytes,omitempty"`
	MaxScanFileBytes   int      `json:"max_scan_file_bytes,omitempty"`
	MaxResults         int      `json:"max_results,omitempty"`
	TreeDepth          int      `json:"tree_depth,omitempty"`
	WalkDepth          int      `json:"walk_depth,omitempty"`
	MCPAllowedTargets  []string `json:"mcp_allowed_targets,omitempty"`
	HTTPAllowedTargets []string `json:"http_allowed_targets,omitempty"`
	HTTPDeniedTargets  []string `json:"http_denied_targets,omitempty"`
	HTTPDeniedPorts    []int    `json:"http_denied_ports,omitempty"`
}

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return "bridge policy denied the request"
}

func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var policyErr *Error
	if errors.As(err, &policyErr) && policyErr.Code != "" {
		return policyErr.Code
	}
	return ""
}

func Parse(raw string) (Policy, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Policy{}, nil
	}
	var policy Policy
	if err := common.UnmarshalJsonStr(raw, &policy); err != nil {
		return Policy{}, err
	}
	return Normalize(policy), nil
}

func Marshal(policy Policy) (string, error) {
	normalized := Normalize(policy)
	bytes, err := common.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func Normalize(policy Policy) Policy {
	policy.AllowedTools = normalizeList(policy.AllowedTools, 128)
	policy.MCPAllowedTargets = normalizeList(policy.MCPAllowedTargets, 512)
	policy.HTTPAllowedTargets = normalizeList(policy.HTTPAllowedTargets, 512)
	policy.HTTPDeniedTargets = normalizeList(policy.HTTPDeniedTargets, 512)
	policy.HTTPDeniedPorts = normalizePositiveInts(policy.HTTPDeniedPorts)
	policy.MaxResultBytes = nonNegative(policy.MaxResultBytes)
	policy.MaxScanFileBytes = nonNegative(policy.MaxScanFileBytes)
	policy.MaxResults = nonNegative(policy.MaxResults)
	policy.TreeDepth = nonNegative(policy.TreeDepth)
	policy.WalkDepth = nonNegative(policy.WalkDepth)
	return policy
}

func ValidateTool(policy Policy, toolName string) error {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return &Error{Code: ErrorCodeToolNotAllowed, Message: "bridge tool name is required"}
	}
	if isWriteTool(toolName) && !policy.AllowWrite {
		return &Error{Code: ErrorCodeWriteDisabled, Message: "bridge write tools are disabled by server policy"}
	}
	if len(policy.AllowedTools) == 0 {
		if isDefaultAllowedTool(toolName) {
			return nil
		}
		return &Error{
			Code:    ErrorCodeToolNotAllowed,
			Message: fmt.Sprintf("bridge tool %s is not allowed by default server policy", toolName),
		}
	}
	for _, allowed := range policy.AllowedTools {
		if allowed == "*" || allowed == toolName {
			return nil
		}
		if allowed == "mcp_proxy" && strings.HasPrefix(toolName, "mcp_proxy.") {
			return nil
		}
		if allowed == "http_tunnel" && strings.HasPrefix(toolName, "http_tunnel.") {
			return nil
		}
		if strings.HasSuffix(allowed, ".*") && strings.HasPrefix(toolName, strings.TrimSuffix(allowed, "*")) {
			return nil
		}
	}
	return &Error{
		Code:    ErrorCodeToolNotAllowed,
		Message: fmt.Sprintf("bridge tool %s is not allowed by server policy", toolName),
	}
}

func ApplyArgumentLimits(policy Policy, toolName string, args map[string]any) map[string]any {
	next := copyArgs(args)
	limits := map[string]any{}
	if policy.MaxResultBytes > 0 {
		limits["max_result_bytes"] = policy.MaxResultBytes
	}
	if policy.MaxScanFileBytes > 0 {
		limits["max_scan_file_bytes"] = policy.MaxScanFileBytes
	}
	if policy.MaxResults > 0 {
		limits["max_results"] = policy.MaxResults
		setCappedInt(next, "max_results", policy.MaxResults)
	}
	switch strings.TrimSpace(toolName) {
	case "remote_tree":
		if policy.TreeDepth > 0 {
			limits["tree_depth"] = policy.TreeDepth
			setCappedDepth(next, policy.TreeDepth)
		}
	case "remote_glob", "remote_grep":
		if policy.WalkDepth > 0 {
			limits["walk_depth"] = policy.WalkDepth
			setCappedInt(next, "max_depth", policy.WalkDepth)
		}
	}
	if len(limits) > 0 {
		next["_bridge_policy_limits"] = limits
	}
	return next
}

func ValidateResultSize(policy Policy, resultSize int) error {
	if policy.MaxResultBytes <= 0 || resultSize <= policy.MaxResultBytes {
		return nil
	}
	return &Error{
		Code:    ErrorCodeResultTooLarge,
		Message: fmt.Sprintf("bridge result size %d exceeds server policy max_result_bytes %d", resultSize, policy.MaxResultBytes),
	}
}

func ValidateMCPTarget(policy Policy, rawTarget string) error {
	return validateURLTarget(policy.MCPAllowedTargets, nil, nil, rawTarget, ErrorCodeMCPTargetForbidden, "MCP")
}

func ValidateHTTPTarget(policy Policy, rawTarget string) error {
	return validateURLTarget(policy.HTTPAllowedTargets, policy.HTTPDeniedTargets, policy.HTTPDeniedPorts, rawTarget, ErrorCodeHTTPTargetForbidden, "HTTP")
}

func validateURLTarget(allowedTargets []string, deniedTargets []string, deniedPorts []int, rawTarget string, errorCode string, label string) error {
	rawTarget = strings.TrimSpace(rawTarget)
	if rawTarget == "" {
		return nil
	}
	parsed, err := url.Parse(rawTarget)
	if err != nil || parsed.Hostname() == "" {
		return &Error{Code: errorCode, Message: "invalid " + label + " target URL"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return &Error{Code: errorCode, Message: label + " target must use http or https"}
	}
	if targetMatchesDenied(parsed, deniedTargets, deniedPorts) {
		return &Error{Code: errorCode, Message: label + " target is denied by server policy"}
	}
	if len(allowedTargets) == 0 {
		if isDefaultLoopbackHost(parsed.Hostname()) {
			return nil
		}
		return &Error{Code: errorCode, Message: label + " target must be loopback unless allowed by server policy"}
	}
	for _, allowed := range allowedTargets {
		if targetMatchesAllowed(parsed, allowed) {
			return nil
		}
	}
	return &Error{Code: errorCode, Message: label + " target is not allowed by server policy"}
}

func copyArgs(args map[string]any) map[string]any {
	next := map[string]any{}
	for key, value := range args {
		next[key] = value
	}
	return next
}

func setCappedDepth(args map[string]any, cap int) {
	if _, ok := args["depth"]; ok {
		setCappedInt(args, "depth", cap)
		return
	}
	setCappedInt(args, "max_depth", cap)
}

func setCappedInt(args map[string]any, key string, cap int) {
	if cap <= 0 {
		return
	}
	current, ok := positiveInt(args[key])
	if ok && current < cap {
		args[key] = current
		return
	}
	args[key] = cap
}

func positiveInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, typed > 0
	case int64:
		if typed > int64(^uint(0)>>1) {
			return 0, false
		}
		return int(typed), typed > 0
	case float64:
		converted := int(typed)
		return converted, typed > 0 && float64(converted) == typed
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil && parsed > 0
	default:
		return 0, false
	}
}

func targetMatchesAllowed(target *url.URL, allowed string) bool {
	allowed = strings.TrimSpace(allowed)
	if allowed == "" {
		return false
	}
	if allowed == "*" {
		return true
	}
	if strings.Contains(allowed, "://") {
		parsedAllowed, err := url.Parse(allowed)
		if err != nil || parsedAllowed.Hostname() == "" {
			return false
		}
		if parsedAllowed.Scheme != "" && parsedAllowed.Scheme != target.Scheme {
			return false
		}
		if !hostMatches(parsedAllowed.Hostname(), target.Hostname()) {
			return false
		}
		if parsedAllowed.Port() != "" && parsedAllowed.Port() != target.Port() {
			return false
		}
		allowedPath := strings.TrimRight(parsedAllowed.EscapedPath(), "/")
		if allowedPath != "" && !pathMatchesPrefix(strings.TrimRight(target.EscapedPath(), "/"), allowedPath) {
			return false
		}
		return true
	}
	allowedHost, allowedPort := splitAllowedHostPort(allowed)
	if !hostMatches(allowedHost, target.Hostname()) {
		return false
	}
	return allowedPort == "" || allowedPort == target.Port()
}

func targetMatchesDenied(target *url.URL, deniedTargets []string, deniedPorts []int) bool {
	for _, port := range deniedPorts {
		if port > 0 && strconv.Itoa(port) == target.Port() {
			return true
		}
	}
	for _, denied := range deniedTargets {
		if targetMatchesAllowed(target, denied) {
			return true
		}
	}
	return false
}

func splitAllowedHostPort(value string) (string, string) {
	value = strings.TrimSpace(value)
	if host, port, err := net.SplitHostPort(value); err == nil {
		return host, port
	}
	if strings.Count(value, ":") == 1 {
		host, port, ok := strings.Cut(value, ":")
		if ok {
			return host, port
		}
	}
	return strings.Trim(value, "[]"), ""
}

func hostMatches(allowed string, target string) bool {
	return strings.EqualFold(strings.Trim(allowed, "[]"), strings.Trim(target, "[]"))
}

func pathMatchesPrefix(targetPath string, allowedPath string) bool {
	return targetPath == allowedPath || strings.HasPrefix(targetPath, allowedPath+"/")
}

func isDefaultLoopbackHost(host string) bool {
	host = strings.ToLower(strings.Trim(host, "[]"))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func isWriteTool(toolName string) bool {
	return toolName == "remote_write" || toolName == "remote_edit"
}

func isDefaultAllowedTool(toolName string) bool {
	switch toolName {
	case "remote_read", "remote_tree", "remote_glob", "remote_grep", "remote_env_info":
		return true
	default:
		return strings.HasPrefix(toolName, "mcp_proxy.") || strings.HasPrefix(toolName, "http_tunnel.")
	}
}

func normalizeList(values []string, maxLen int) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if maxLen > 0 && len(value) > maxLen {
			value = value[:maxLen]
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func normalizePositiveInts(values []int) []int {
	seen := map[int]bool{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func nonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
