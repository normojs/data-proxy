package mcpgateway

import "strings"

func ClassifyTool(tool Tool) []string {
	categories := annotationCategories(tool)
	if len(categories) > 0 {
		return categories
	}
	text := strings.ToLower(strings.TrimSpace(tool.Name + " " + tool.Description + " " + schemaText(tool.InputSchema)))
	result := make([]string, 0, 3)
	if containsAny(text, "write", "edit", "patch", "delete", "remove", "rename", "create_file", "save_file", "mutate") {
		result = append(result, ToolCategoryWrite)
	}
	if containsAny(text, "exec", "shell", "bash", "command", "terminal", "run_test", "run_tests", "install", "npm", "pip") {
		result = append(result, ToolCategoryExec)
	}
	if containsAny(text, "http", "fetch", "url", "web", "network", "browser", "open_url") {
		result = append(result, ToolCategoryNetwork)
	}
	if containsAny(text, "browser", "page", "click", "screenshot", "playwright") {
		result = append(result, ToolCategoryBrowser)
	}
	if containsAny(text, "computer", "mouse", "keyboard", "screen", "desktop", "window") {
		result = append(result, ToolCategoryComputer)
	}
	if len(result) == 0 && containsAny(text, "read", "list", "search", "grep", "glob", "tree", "status", "diff", "log", "info") {
		result = append(result, ToolCategoryRead)
	}
	if len(result) == 0 {
		result = append(result, ToolCategoryUnknown)
	}
	return dedupeStrings(result)
}

func RiskLevel(categories []string) string {
	for _, category := range categories {
		switch category {
		case ToolCategoryExec, ToolCategoryComputer:
			return "critical"
		}
	}
	for _, category := range categories {
		switch category {
		case ToolCategoryWrite, ToolCategoryNetwork, ToolCategoryBrowser:
			return "high"
		}
	}
	for _, category := range categories {
		if category == ToolCategoryUnknown {
			return "review"
		}
	}
	return "low"
}

func annotationCategories(tool Tool) []string {
	annotations := map[string]any{}
	for key, value := range tool.Metadata {
		annotations[strings.ToLower(key)] = value
	}
	for key, value := range tool.Annotations {
		annotations[strings.ToLower(key)] = value
	}
	result := make([]string, 0, 4)
	if boolAnnotation(annotations, "readonlyhint") || boolAnnotation(annotations, "read_only") {
		result = append(result, ToolCategoryRead)
	}
	if boolAnnotation(annotations, "destructivehint") || boolAnnotation(annotations, "write") || boolAnnotation(annotations, "mutating") {
		result = append(result, ToolCategoryWrite)
	}
	if boolAnnotation(annotations, "exechint") || boolAnnotation(annotations, "exec") || boolAnnotation(annotations, "shell") {
		result = append(result, ToolCategoryExec)
	}
	if boolAnnotation(annotations, "openworldhint") || boolAnnotation(annotations, "network") {
		result = append(result, ToolCategoryNetwork)
	}
	if boolAnnotation(annotations, "browser") {
		result = append(result, ToolCategoryBrowser)
	}
	if boolAnnotation(annotations, "computer") {
		result = append(result, ToolCategoryComputer)
	}
	if boolAnnotation(annotations, "unknown") {
		result = append(result, ToolCategoryUnknown)
	}
	return dedupeStrings(result)
}

func schemaText(schema map[string]any) string {
	body := NormalizeRawJSON(schema)
	return string(body)
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func boolAnnotation(values map[string]any, key string) bool {
	value, ok := values[strings.ToLower(key)]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		typed = strings.TrimSpace(strings.ToLower(typed))
		return typed == "true" || typed == "yes" || typed == "1"
	default:
		return false
	}
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
