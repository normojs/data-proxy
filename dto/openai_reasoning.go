package dto

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const maxReasoningExtractDepth = 8

// ExtractReasoningText normalizes provider-specific reasoning payloads into text.
func ExtractReasoningText(values ...any) string {
	parts := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		for _, part := range collectReasoningText(value, 0) {
			part = strings.TrimSpace(part)
			if part == "" || seen[part] {
				continue
			}
			seen[part] = true
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n")
}

func collectReasoningText(value any, depth int) []string {
	if value == nil || depth > maxReasoningExtractDepth {
		return nil
	}

	switch v := value.(type) {
	case string:
		return collectReasoningString(v, depth)
	case *string:
		if v == nil {
			return nil
		}
		return collectReasoningString(*v, depth)
	case json.RawMessage:
		return collectReasoningRawJSON(v, depth)
	case []byte:
		return collectReasoningRawJSON(json.RawMessage(v), depth)
	case map[string]any:
		return collectReasoningMap(v, depth)
	case []any:
		return collectReasoningSlice(v, depth)
	case []map[string]any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, collectReasoningMap(item, depth+1)...)
		}
		return parts
	default:
		return collectReasoningString(fmt.Sprintf("%v", v), depth)
	}
}

func collectReasoningString(value string, depth int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
		var decoded any
		if err := json.Unmarshal([]byte(value), &decoded); err == nil {
			return collectReasoningText(decoded, depth+1)
		}
	}
	return []string{value}
}

func collectReasoningRawJSON(value json.RawMessage, depth int) []string {
	if len(value) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return collectReasoningString(string(value), depth+1)
	}
	return collectReasoningText(decoded, depth+1)
}

func collectReasoningSlice(values []any, depth int) []string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, collectReasoningText(value, depth+1)...)
	}
	return parts
}

func collectReasoningMap(value map[string]any, depth int) []string {
	preferredKeys := []string{
		"reasoning_content",
		"reasoning",
		"reasoning_details",
		"text",
		"content",
		"summary",
		"details",
		"parts",
		"delta",
	}
	parts := make([]string, 0, len(preferredKeys))
	used := map[string]bool{}
	for _, key := range preferredKeys {
		raw, ok := value[key]
		if !ok {
			continue
		}
		used[key] = true
		parts = append(parts, collectReasoningText(raw, depth+1)...)
	}
	if len(parts) > 0 {
		return parts
	}

	keys := make([]string, 0, len(value))
	for key := range value {
		if used[key] || isReasoningMetadataKey(key) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, collectReasoningText(value[key], depth+1)...)
	}
	return parts
}

func isReasoningMetadataKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "type", "id", "index", "status", "format", "signature", "encrypted_content":
		return true
	default:
		return false
	}
}
