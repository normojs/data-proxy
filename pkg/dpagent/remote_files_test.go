package dpagent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRemoteReadSlicesLinesAndGuardsWorkspace(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "notes.txt"), []byte("one\ntwo\nthree\nfour\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	client := BridgeClient{Config: remoteTestConfig(workspace)}
	result, err := client.handleRemoteFileTool(context.Background(), BridgeToolRemoteRead, map[string]any{
		"file_path": "notes.txt",
		"offset":    2,
		"limit":     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Content[0].Text; got != "two\nthree" {
		t.Fatalf("unexpected remote_read content: %q", got)
	}
	if result.Summary != "notes.txt:2-3" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.Metadata["total_lines"].(int) != 5 {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}

	_, err = client.handleRemoteFileTool(context.Background(), BridgeToolRemoteRead, map[string]any{"file_path": outside})
	if err == nil {
		t.Fatal("expected outside path to be rejected")
	}
	toolErr, ok := err.(ToolError)
	if !ok || toolErr.Code != "REMOTE_READ_FORBIDDEN" {
		t.Fatalf("unexpected outside path error: %#v", err)
	}
}

func TestRemoteReadRejectsSymlinkEscapingWorkspace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires extra privileges on some Windows runners")
	}
	workspace := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(workspace, "secret-link.txt")); err != nil {
		t.Fatal(err)
	}

	client := BridgeClient{Config: remoteTestConfig(workspace)}
	_, err := client.handleRemoteFileTool(context.Background(), BridgeToolRemoteRead, map[string]any{"file_path": "secret-link.txt"})
	if err == nil {
		t.Fatal("expected symlink escaping workspace to be rejected")
	}
	toolErr, ok := err.(ToolError)
	if !ok || toolErr.Code != "REMOTE_READ_FORBIDDEN" {
		t.Fatalf("unexpected symlink error: %#v", err)
	}
}

func TestRemoteTreeGlobAndGrep(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "README.md"), "hello docs\n")
	mustWriteFile(t, filepath.Join(workspace, "cmd", "main.go"), "package main\nfunc main() {}\n")
	mustWriteFile(t, filepath.Join(workspace, "pkg", "agent.go"), "package pkg\nconst AgentName = \"Data Proxy\"\n")
	mustWriteFile(t, filepath.Join(workspace, "node_modules", "ignored.go"), "package ignored\n")

	client := BridgeClient{Config: remoteTestConfig(workspace)}
	tree, err := client.handleRemoteFileTool(context.Background(), BridgeToolRemoteTree, map[string]any{
		"path":  ".",
		"depth": 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	treeText := tree.Content[0].Text
	if !strings.Contains(treeText, "d cmd") || !strings.Contains(treeText, "- cmd/main.go") {
		t.Fatalf("tree missing expected entries:\n%s", treeText)
	}
	if strings.Contains(treeText, "node_modules") {
		t.Fatalf("tree should ignore node_modules:\n%s", treeText)
	}

	glob, err := client.handleRemoteFileTool(context.Background(), BridgeToolRemoteGlob, map[string]any{
		"pattern": "**/*.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	globText := glob.Content[0].Text
	if !strings.Contains(globText, "cmd/main.go") || !strings.Contains(globText, "pkg/agent.go") {
		t.Fatalf("glob missing expected files:\n%s", globText)
	}
	if strings.Contains(globText, "ignored.go") {
		t.Fatalf("glob should ignore node_modules:\n%s", globText)
	}

	grep, err := client.handleRemoteFileTool(context.Background(), BridgeToolRemoteGrep, map[string]any{
		"pattern":          "data proxy",
		"glob":             "**/*.go",
		"case_insensitive": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	grepText := grep.Content[0].Text
	if !strings.Contains(grepText, "pkg/agent.go:2:") {
		t.Fatalf("grep missing expected match:\n%s", grepText)
	}
}

func TestRemoteEnvInfoAndCapabilities(t *testing.T) {
	workspace := t.TempDir()
	cfg := remoteTestConfig(workspace)
	cfg.HTTPRoutes = []HTTPRoute{{Name: "local", Target: "http://127.0.0.1:3000"}}
	cfg.MCPServers = []MCPServer{{Name: "coding", Transport: "streamable_http", Endpoint: "http://127.0.0.1:30837/mcp"}}
	client := BridgeClient{Config: cfg}

	capabilities := EffectiveCapabilities(cfg)
	for _, expected := range []string{
		BridgeToolRemoteRead,
		BridgeToolRemoteTree,
		BridgeToolRemoteGlob,
		BridgeToolRemoteGrep,
		BridgeToolRemoteEnvInfo,
		BridgeCapabilityHTTPTunnel,
		BridgeCapabilityMCPProxy,
	} {
		if !containsString(capabilities, expected) {
			t.Fatalf("capability %s missing: %#v", expected, capabilities)
		}
	}

	result, err := client.handleRemoteFileTool(context.Background(), BridgeToolRemoteEnvInfo, nil)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["platform"] != runtime.GOOS || payload["arch"] != runtime.GOARCH {
		t.Fatalf("unexpected env info: %#v", payload)
	}
	limits := payload["limits"].(map[string]any)
	if int(limits["max_results"].(float64)) != DefaultRemoteMaxResults {
		t.Fatalf("unexpected limits: %#v", limits)
	}
}

func TestRemotePolicyLimitsAndDeniedPaths(t *testing.T) {
	workspace := t.TempDir()
	mustWriteFile(t, filepath.Join(workspace, "a.txt"), strings.Repeat("a", 64))
	mustWriteFile(t, filepath.Join(workspace, "private", "secret.txt"), "secret")
	cfg := remoteTestConfig(workspace)
	cfg.Policy.DeniedPaths = []string{"private"}
	client := BridgeClient{Config: cfg}

	limited, err := client.handleRemoteFileTool(context.Background(), BridgeToolRemoteRead, map[string]any{
		"file_path": "a.txt",
		"_bridge_policy_limits": map[string]any{
			"max_result_bytes": 8,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if limited.Metadata["truncated"] != true || !strings.Contains(limited.Content[0].Text, "result truncated") {
		t.Fatalf("expected truncated read result: %#v", limited)
	}

	_, err = client.handleRemoteFileTool(context.Background(), BridgeToolRemoteRead, map[string]any{"file_path": "private/secret.txt"})
	if err == nil {
		t.Fatal("expected denied path to be rejected")
	}
	toolErr, ok := err.(ToolError)
	if !ok || toolErr.Code != "REMOTE_READ_FORBIDDEN" {
		t.Fatalf("unexpected denied path error: %#v", err)
	}
}

func TestRemoteWriteDisabledByDefault(t *testing.T) {
	workspace := t.TempDir()
	cfg := remoteTestConfig(workspace)
	client := BridgeClient{Config: cfg}

	if containsString(EffectiveCapabilities(cfg), BridgeToolRemoteWrite) {
		t.Fatalf("remote_write should not be advertised by default: %#v", EffectiveCapabilities(cfg))
	}
	_, err := client.handleRemoteFileTool(context.Background(), BridgeToolRemoteWrite, map[string]any{
		"file_path": "note.txt",
		"content":   "hello",
	})
	if err == nil {
		t.Fatal("expected remote_write to be disabled by default")
	}
	toolErr, ok := err.(ToolError)
	if !ok || toolErr.Code != "REMOTE_WRITE_DISABLED" {
		t.Fatalf("unexpected disabled write error: %#v", err)
	}
}

func TestRemoteWriteAndEditWhenAllowed(t *testing.T) {
	workspace := t.TempDir()
	cfg := remoteTestConfig(workspace)
	cfg.Policy.AllowWrite = true
	client := BridgeClient{Config: cfg}

	capabilities := EffectiveCapabilities(cfg)
	if !containsString(capabilities, BridgeToolRemoteWrite) || !containsString(capabilities, BridgeToolRemoteEdit) {
		t.Fatalf("write capabilities missing when allow_write=true: %#v", capabilities)
	}

	written, err := client.handleRemoteFileTool(context.Background(), BridgeToolRemoteWrite, map[string]any{
		"file_path": "docs/note.txt",
		"content":   "hello world\nworld\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if written.Metadata["file_path"] != "docs/note.txt" {
		t.Fatalf("unexpected write metadata: %#v", written.Metadata)
	}
	if got := readTestFile(t, filepath.Join(workspace, "docs", "note.txt")); got != "hello world\nworld\n" {
		t.Fatalf("unexpected written content: %q", got)
	}

	_, err = client.handleRemoteFileTool(context.Background(), BridgeToolRemoteEdit, map[string]any{
		"file_path":  "docs/note.txt",
		"old_string": "world",
		"new_string": "agent",
	})
	if err == nil {
		t.Fatal("expected ambiguous edit to fail")
	}
	toolErr, ok := err.(ToolError)
	if !ok || toolErr.Code != "REMOTE_EDIT_AMBIGUOUS" {
		t.Fatalf("unexpected ambiguous edit error: %#v", err)
	}

	edited, err := client.handleRemoteFileTool(context.Background(), BridgeToolRemoteEdit, map[string]any{
		"file_path":   "docs/note.txt",
		"old_string":  "world",
		"new_string":  "agent",
		"replace_all": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if edited.Metadata["replacements"].(int) != 2 {
		t.Fatalf("unexpected edit metadata: %#v", edited.Metadata)
	}
	if got := readTestFile(t, filepath.Join(workspace, "docs", "note.txt")); got != "hello agent\nagent\n" {
		t.Fatalf("unexpected edited content: %q", got)
	}
}

func TestRemoteWriteGuardsPathAndSize(t *testing.T) {
	workspace := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	cfg := remoteTestConfig(workspace)
	cfg.Policy.AllowWrite = true
	cfg.Runtime.MaxWriteBytes = 4
	client := BridgeClient{Config: cfg}

	_, err := client.handleRemoteFileTool(context.Background(), BridgeToolRemoteWrite, map[string]any{
		"file_path": outside,
		"content":   "ok",
	})
	if err == nil {
		t.Fatal("expected outside write path to be rejected")
	}
	toolErr, ok := err.(ToolError)
	if !ok || toolErr.Code != "REMOTE_WRITE_FORBIDDEN" {
		t.Fatalf("unexpected outside write error: %#v", err)
	}

	_, err = client.handleRemoteFileTool(context.Background(), BridgeToolRemoteWrite, map[string]any{
		"file_path": "large.txt",
		"content":   "too large",
	})
	if err == nil {
		t.Fatal("expected write size limit error")
	}
	toolErr, ok = err.(ToolError)
	if !ok || toolErr.Code != "REMOTE_WRITE_TOO_LARGE" {
		t.Fatalf("unexpected write size error: %#v", err)
	}
}

func TestRemoteWriteRejectsSymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires extra privileges on some Windows runners")
	}
	workspace := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(workspace, "secret-link.txt")); err != nil {
		t.Fatal(err)
	}
	cfg := remoteTestConfig(workspace)
	cfg.Policy.AllowWrite = true
	client := BridgeClient{Config: cfg}

	_, err := client.handleRemoteFileTool(context.Background(), BridgeToolRemoteWrite, map[string]any{
		"file_path": "secret-link.txt",
		"content":   "changed",
	})
	if err == nil {
		t.Fatal("expected symlink target to be rejected")
	}
	toolErr, ok := err.(ToolError)
	if !ok || toolErr.Code != "REMOTE_WRITE_FORBIDDEN" {
		t.Fatalf("unexpected symlink write error: %#v", err)
	}
	if got := readTestFile(t, outside); got != "secret" {
		t.Fatalf("outside symlink target was modified: %q", got)
	}
}

func remoteTestConfig(workspace string) Config {
	cfg := DefaultConfig()
	cfg.Server.BaseURL = "https://dp.example.com"
	cfg.Agent.Workspace = workspace
	return cfg
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes)
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
