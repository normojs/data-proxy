package mcpgateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

func BuildToolSnapshots(tools []Tool) []ToolSnapshot {
	snapshots := make([]ToolSnapshot, 0, len(tools))
	for _, tool := range tools {
		categories := ClassifyTool(tool)
		snapshots = append(snapshots, ToolSnapshot{
			Name:        strings.TrimSpace(tool.Name),
			Description: strings.TrimSpace(tool.Description),
			SchemaHash:  HashToolSchema(tool),
			Categories:  categories,
			RiskLevel:   RiskLevel(categories),
		})
	}
	return snapshots
}

func HashToolSchema(tool Tool) string {
	body, err := json.Marshal(struct {
		Name        string         `json:"name"`
		InputSchema map[string]any `json:"input_schema,omitempty"`
		Annotations map[string]any `json:"annotations,omitempty"`
	}{
		Name:        strings.TrimSpace(tool.Name),
		InputSchema: tool.InputSchema,
		Annotations: tool.Annotations,
	})
	if err != nil {
		body = []byte(strings.TrimSpace(tool.Name))
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func DiffToolSnapshots(previous []ToolSnapshot, current []ToolSnapshot) ToolSnapshotDiff {
	prevByName := map[string]ToolSnapshot{}
	for _, item := range previous {
		prevByName[item.Name] = item
	}
	currentByName := map[string]ToolSnapshot{}
	for _, item := range current {
		currentByName[item.Name] = item
	}
	diff := ToolSnapshotDiff{}
	for _, item := range current {
		prev, ok := prevByName[item.Name]
		if !ok {
			diff.Added = append(diff.Added, item)
			continue
		}
		if prev.SchemaHash != item.SchemaHash {
			diff.Changed = append(diff.Changed, ToolSnapshotChange{Before: prev, After: item})
		}
	}
	for _, item := range previous {
		if _, ok := currentByName[item.Name]; !ok {
			diff.Removed = append(diff.Removed, item)
		}
	}
	return diff
}

type ToolSnapshotDiff struct {
	Added   []ToolSnapshot       `json:"added,omitempty"`
	Changed []ToolSnapshotChange `json:"changed,omitempty"`
	Removed []ToolSnapshot       `json:"removed,omitempty"`
}

type ToolSnapshotChange struct {
	Before ToolSnapshot `json:"before"`
	After  ToolSnapshot `json:"after"`
}
