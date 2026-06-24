package mcpgateway

import "strings"

type Policy struct {
	PermissionMode          string
	AllowedTools            []string
	DeniedTools             []string
	AllowedCategories       []string
	DeniedCategories        []string
	RequireKnownCategory    bool
	RequireExplicitToolList bool
}

func PolicyForPermissionMode(mode string) Policy {
	mode = strings.TrimSpace(strings.ToLower(mode))
	policy := Policy{
		PermissionMode:       mode,
		AllowedTools:         AllowedToolsForPermissionMode(mode),
		RequireKnownCategory: true,
	}
	switch mode {
	case PermissionWrite:
		policy.AllowedCategories = []string{ToolCategoryRead, ToolCategoryWrite}
	case PermissionExecSafe:
		policy.AllowedCategories = []string{ToolCategoryRead, ToolCategoryWrite, ToolCategoryExec}
		policy.DeniedTools = []string{"remote_exec", "remote_shell_open", "remote_shell_eval", "remote_install_package"}
	case PermissionExecTrusted:
		policy.AllowedCategories = []string{ToolCategoryRead, ToolCategoryWrite, ToolCategoryExec}
	default:
		policy.PermissionMode = PermissionReadOnly
		policy.AllowedCategories = []string{ToolCategoryRead}
	}
	return policy
}

func AllowedToolsForPermissionMode(mode string) []string {
	readTools := []string{
		"mcp_proxy",
		"remote_read",
		"remote_tree",
		"remote_glob",
		"remote_grep",
		"remote_env_info",
		"remote_project_info",
		"remote_get_related_files",
		"remote_git_status",
		"remote_git_diff",
		"remote_git_log",
	}
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case PermissionWrite:
		return append(readTools, "remote_write", "remote_edit")
	case PermissionExecSafe:
		return append(readTools, "remote_write", "remote_edit", "remote_run_tests")
	case PermissionExecTrusted:
		return append(readTools,
			"remote_write",
			"remote_edit",
			"remote_run_tests",
			"remote_exec",
			"remote_shell_open",
			"remote_shell_eval",
			"remote_install_package",
		)
	default:
		return readTools
	}
}

func FilterTools(policy Policy, tools []Tool) []Tool {
	result := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		if AuthorizeTool(policy, tool).Allowed() {
			result = append(result, tool)
		}
	}
	return result
}

func AuthorizeTool(policy Policy, tool Tool) Decision {
	name := strings.TrimSpace(tool.Name)
	if name == "" {
		return Decision{Decision: DecisionDeny, Reason: "tool name is required", Category: ToolCategoryUnknown}
	}
	if stringSetContains(policy.DeniedTools, name) {
		return Decision{Decision: DecisionDeny, Reason: "tool is explicitly denied", Category: firstCategory(ClassifyTool(tool))}
	}
	if len(policy.AllowedTools) > 0 && !stringSetContains(policy.AllowedTools, name) {
		if policy.RequireExplicitToolList {
			return Decision{Decision: DecisionDeny, Reason: "tool is not in the allowed tool list", Category: firstCategory(ClassifyTool(tool))}
		}
	}
	categories := ClassifyTool(tool)
	category := firstCategory(categories)
	if policy.RequireKnownCategory && stringSetContains(categories, ToolCategoryUnknown) {
		return Decision{Decision: DecisionDeny, Reason: "tool category is unknown and requires review", Category: category}
	}
	if categoryIntersects(categories, policy.DeniedCategories) {
		return Decision{Decision: DecisionDeny, Reason: "tool category is denied", Category: category}
	}
	if len(policy.AllowedCategories) > 0 && !categoryIntersects(categories, policy.AllowedCategories) {
		return Decision{Decision: DecisionDeny, Reason: "tool category is not allowed", Category: category}
	}
	return Decision{Decision: DecisionAllow, Reason: "tool allowed by gateway policy", Category: category}
}

func firstCategory(categories []string) string {
	if len(categories) == 0 {
		return ToolCategoryUnknown
	}
	return categories[0]
}

func stringSetContains(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func categoryIntersects(categories []string, allowed []string) bool {
	for _, category := range categories {
		if stringSetContains(allowed, category) {
			return true
		}
	}
	return false
}
