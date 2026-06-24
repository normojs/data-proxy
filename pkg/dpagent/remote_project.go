package dpagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

const (
	defaultRemoteGitTimeoutMS = 30000
	remoteHardGitTimeoutMS    = 120000
)

var remoteProjectManifestNames = []string{
	"package.json",
	"pnpm-lock.yaml",
	"yarn.lock",
	"bun.lock",
	"go.mod",
	"go.sum",
	"Cargo.toml",
	"Cargo.lock",
	"pyproject.toml",
	"requirements.txt",
	"poetry.lock",
	"pom.xml",
	"build.gradle",
	"composer.json",
	"Gemfile",
	"Makefile",
	"Dockerfile",
	"docker-compose.yml",
	"docker-compose.yaml",
}

type remoteGitOutput struct {
	Text      string
	ExitCode  int
	TimedOut  bool
	Truncated bool
}

type remoteRelatedCandidate struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	Score  int    `json:"score"`
}

func (c BridgeClient) handleRemoteGitStatus(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	workdir, err := resolveRemoteWorkdir(c.Config, args, "REMOTE_GIT_STATUS")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	output, err := c.runRemoteGit(ctx, workdir.Path, []string{"status", "--short", "--branch"}, args)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	if output.ExitCode != 0 {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GIT_STATUS_FAILED", Message: gitFailureMessage(output)}
	}
	text := output.Text
	if text == "" {
		text = "working tree clean"
	}
	if output.Truncated {
		text += remoteTruncatedMarker
	}
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: text}},
		Summary:    "git status " + workdir.Rel,
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len([]byte(text)),
		Metadata: map[string]any{
			"workdir":   workdir.Rel,
			"exit_code": output.ExitCode,
			"truncated": output.Truncated,
		},
	}, nil
}

func (c BridgeClient) handleRemoteGitDiff(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	workdir, err := resolveRemoteWorkdir(c.Config, args, "REMOTE_GIT_DIFF")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	gitArgs := []string{"diff", "--no-ext-diff", "--no-textconv"}
	cached := boolFromMap(args, "cached")
	if cached {
		gitArgs = append(gitArgs, "--cached")
	}
	if filePath := firstString(args, "file_path", "path"); filePath != "" {
		fileInfo, err := resolveExistingRemotePath(c.Config, filePath, "", "REMOTE_GIT_DIFF")
		if err != nil {
			return dto.BridgeToolCallResult{}, err
		}
		rel, err := filepath.Rel(workdir.Path, fileInfo.Path)
		if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GIT_DIFF_FORBIDDEN", Message: "file_path is outside git workdir"}
		}
		gitArgs = append(gitArgs, "--", rel)
	}
	output, err := c.runRemoteGit(ctx, workdir.Path, gitArgs, args)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	if output.ExitCode != 0 {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GIT_DIFF_FAILED", Message: gitFailureMessage(output)}
	}
	text := output.Text
	if text == "" {
		text = "no diff"
	}
	if output.Truncated {
		text += remoteTruncatedMarker
	}
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: text}},
		Summary:    "git diff " + workdir.Rel,
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len([]byte(text)),
		Metadata: map[string]any{
			"workdir":   workdir.Rel,
			"cached":    cached,
			"exit_code": output.ExitCode,
			"truncated": output.Truncated,
		},
	}, nil
}

func (c BridgeClient) handleRemoteGitLog(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	workdir, err := resolveRemoteWorkdir(c.Config, args, "REMOTE_GIT_LOG")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	limit := remotePositiveInt(args["limit"], 20, 200)
	output, err := c.runRemoteGit(ctx, workdir.Path, []string{"log", "--oneline", "--decorate=short", "--no-show-signature", "-n", fmt.Sprint(limit)}, args)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	if output.ExitCode != 0 {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GIT_LOG_FAILED", Message: gitFailureMessage(output)}
	}
	text := output.Text
	if text == "" {
		text = "no commits"
	}
	if output.Truncated {
		text += remoteTruncatedMarker
	}
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: text}},
		Summary:    "git log " + workdir.Rel,
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len([]byte(text)),
		Metadata: map[string]any{
			"workdir":   workdir.Rel,
			"limit":     limit,
			"exit_code": output.ExitCode,
			"truncated": output.Truncated,
		},
	}, nil
}

func (c BridgeClient) handleRemoteProjectInfo(_ context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	info, err := resolveRemoteDirectory(c.Config, firstString(args, "path", "workdir"), ".", "REMOTE_PROJECT_INFO")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	limits := remoteLimitsFromConfig(c.Config, args)
	languages, languageTruncated := remoteLanguageCounts(c.Config, info.Root, info.Path, limits)
	topLevel, topLevelTruncated := remoteTopLevelEntries(c.Config, info.Root, info.Path, limits)
	gitRoot := ""
	if root, ok := findRemoteGitRoot(info.Root, info.Path); ok {
		gitRoot = relativeRemotePath(info.Root, root)
	}
	payload := map[string]any{
		"path":                info.Rel,
		"name":                filepath.Base(info.Path),
		"git_root":            gitRoot,
		"manifests":           detectRemoteProjectManifests(info.Path),
		"languages":           languages,
		"top_level":           topLevel,
		"top_level_truncated": topLevelTruncated,
		"language_truncated":  languageTruncated,
	}
	text, truncated, err := encodeLimitedRemoteJSON(payload, limits.MaxResultBytes)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: text}},
		Summary:    "project info " + info.Rel,
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len([]byte(text)),
		Metadata: map[string]any{
			"path":      info.Rel,
			"git_root":  gitRoot,
			"truncated": truncated || topLevelTruncated || languageTruncated,
		},
	}, nil
}

func (c BridgeClient) handleRemoteGetRelatedFiles(_ context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	filePath := firstString(args, "file_path", "path")
	info, err := resolveExistingRemotePath(c.Config, filePath, "", "REMOTE_GET_RELATED_FILES")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	stat, err := os.Stat(info.Path)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GET_RELATED_FILES_NOT_FOUND", Message: err.Error()}
	}
	if !stat.Mode().IsRegular() {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GET_RELATED_FILES_NOT_FILE", Message: "target is not a regular file: " + info.Rel}
	}
	limits := remoteLimitsFromConfig(c.Config, args)
	limit := remotePositiveInt(args["max_results"], minPositiveInt(limits.MaxResults, 20), remoteHardMaxResults)
	walkLimit := scanCandidateLimit(limits, limit, 20)
	walk := walkRemoteWorkspace(c.Config, info.Root, info.Root, limits.WalkDepth, walkLimit, false)
	candidates := relatedRemoteFileCandidates(info.Root, info.Path, walk.Items)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	payload := map[string]any{
		"file_path": info.Rel,
		"related":   candidates,
		"truncated": walk.Truncated,
	}
	text, truncated, err := encodeLimitedRemoteJSON(payload, limits.MaxResultBytes)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: text}},
		Summary:    fmt.Sprintf("related files %s (%d)", info.Rel, len(candidates)),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len([]byte(text)),
		Metadata: map[string]any{
			"file_path": info.Rel,
			"count":     len(candidates),
			"truncated": truncated || walk.Truncated,
		},
	}, nil
}

func (c BridgeClient) runRemoteGit(ctx context.Context, workdir string, gitArgs []string, args map[string]any) (remoteGitOutput, error) {
	timeoutMS := remotePositiveInt(args["timeout_ms"], defaultRemoteGitTimeoutMS, remoteHardGitTimeoutMS)
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "git", gitArgs...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_PAGER=cat",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_EXTERNAL_DIFF=",
	)
	output := &remoteExecOutputBuffer{limit: remoteLimitsFromConfig(c.Config, args).MaxResultBytes}
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	timedOut := runCtx.Err() == context.DeadlineExceeded
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if timedOut {
			exitCode = -1
		} else {
			return remoteGitOutput{}, ToolError{Code: "REMOTE_GIT_FAILED", Message: err.Error()}
		}
	}
	return remoteGitOutput{
		Text:      output.String(),
		ExitCode:  exitCode,
		TimedOut:  timedOut,
		Truncated: output.Truncated(),
	}, nil
}

func resolveRemoteWorkdir(cfg Config, args map[string]any, codePrefix string) (remotePathInfo, error) {
	info, err := resolveExistingRemotePath(cfg, firstString(args, "workdir", "path"), ".", codePrefix)
	if err != nil {
		return remotePathInfo{}, err
	}
	stat, err := os.Stat(info.Path)
	if err != nil {
		return remotePathInfo{}, ToolError{Code: codePrefix + "_NOT_FOUND", Message: err.Error()}
	}
	if stat.IsDir() {
		return info, nil
	}
	parent := filepath.Dir(info.Path)
	return remotePathInfo{Root: info.Root, Path: parent, Rel: relativeRemotePath(info.Root, parent)}, nil
}

func resolveRemoteDirectory(cfg Config, requestedPath string, fallback string, codePrefix string) (remotePathInfo, error) {
	info, err := resolveExistingRemotePath(cfg, requestedPath, fallback, codePrefix)
	if err != nil {
		return remotePathInfo{}, err
	}
	stat, err := os.Stat(info.Path)
	if err != nil {
		return remotePathInfo{}, ToolError{Code: codePrefix + "_NOT_FOUND", Message: err.Error()}
	}
	if !stat.IsDir() {
		return remotePathInfo{}, ToolError{Code: codePrefix + "_NOT_DIRECTORY", Message: "path is not a directory: " + info.Rel}
	}
	return info, nil
}

func gitFailureMessage(output remoteGitOutput) string {
	message := strings.TrimSpace(output.Text)
	if output.TimedOut {
		if message == "" {
			return "git command timed out"
		}
		return "git command timed out: " + message
	}
	if message == "" {
		return fmt.Sprintf("git exited with code %d", output.ExitCode)
	}
	return message
}

func detectRemoteProjectManifests(projectPath string) []string {
	result := []string{}
	for _, name := range remoteProjectManifestNames {
		path := filepath.Join(projectPath, name)
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() {
			result = append(result, name)
		}
	}
	return result
}

func remoteTopLevelEntries(cfg Config, workspaceRoot string, projectPath string, limits remoteFileLimits) ([]map[string]any, bool) {
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return nil, false
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	limit := minPositiveInt(limits.MaxResults, 50)
	result := make([]map[string]any, 0, minPositiveInt(len(entries), limit))
	truncated := false
	for _, entry := range entries {
		if len(result) >= limit {
			truncated = true
			break
		}
		if entry.Type()&os.ModeSymlink != 0 || remoteDefaultIgnores[entry.Name()] {
			continue
		}
		path := filepath.Join(projectPath, entry.Name())
		if isDeniedRemotePath(cfg, workspaceRoot, path) {
			continue
		}
		itemType := "file"
		if entry.IsDir() {
			itemType = "directory"
		}
		result = append(result, map[string]any{
			"path": relativeRemotePath(workspaceRoot, path),
			"type": itemType,
		})
	}
	return result, truncated
}

func remoteLanguageCounts(cfg Config, workspaceRoot string, projectPath string, limits remoteFileLimits) (map[string]int, bool) {
	walkLimit := scanCandidateLimit(limits, minPositiveInt(limits.MaxResults, 200), 10)
	walk := walkRemoteWorkspace(cfg, workspaceRoot, projectPath, limits.WalkDepth, walkLimit, false)
	counts := map[string]int{}
	for _, item := range walk.Items {
		key := remoteLanguageKey(item.Path)
		if key == "" {
			continue
		}
		counts[key]++
	}
	return counts, walk.Truncated
}

func remoteLanguageKey(path string) string {
	name := filepath.Base(path)
	switch name {
	case "Dockerfile":
		return "dockerfile"
	case "Makefile":
		return "makefile"
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	switch ext {
	case "":
		return ""
	case "tsx":
		return "typescript"
	case "ts":
		return "typescript"
	case "jsx":
		return "javascript"
	case "js", "mjs", "cjs":
		return "javascript"
	case "md", "markdown":
		return "markdown"
	case "yml", "yaml":
		return "yaml"
	default:
		return ext
	}
}

func findRemoteGitRoot(workspaceRoot string, startPath string) (string, bool) {
	current := startPath
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		if !pathInside(workspaceRoot, current) {
			return "", false
		}
		if info, err := os.Stat(filepath.Join(current, ".git")); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return current, true
		}
		if current == workspaceRoot {
			return "", false
		}
		next := filepath.Dir(current)
		if next == current {
			return "", false
		}
		current = next
	}
}

func relatedRemoteFileCandidates(workspaceRoot string, targetPath string, items []remoteWalkItem) []remoteRelatedCandidate {
	targetRel := relativeRemotePath(workspaceRoot, targetPath)
	result := []remoteRelatedCandidate{}
	seen := map[string]bool{}
	for _, item := range items {
		if item.Type != "file" || item.Path == targetPath {
			continue
		}
		rel := relativeRemotePath(workspaceRoot, item.Path)
		score, reason := remoteRelatedScore(targetRel, rel)
		if score <= 0 || seen[rel] {
			continue
		}
		seen[rel] = true
		result = append(result, remoteRelatedCandidate{Path: rel, Reason: reason, Score: score})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		return result[i].Path < result[j].Path
	})
	return result
}

func remoteRelatedScore(targetRel string, candidateRel string) (int, string) {
	targetDir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(targetRel)))
	candidateDir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(candidateRel)))
	targetBase := filepath.Base(targetRel)
	candidateBase := filepath.Base(candidateRel)
	targetStem := strings.TrimSuffix(targetBase, filepath.Ext(targetBase))
	candidateStem := strings.TrimSuffix(candidateBase, filepath.Ext(candidateBase))
	if targetDir == candidateDir && targetStem == candidateStem {
		return 100, "same basename"
	}
	if targetDir == candidateDir && isRemoteTestCompanion(targetStem, candidateStem, candidateBase) {
		return 90, "test companion"
	}
	if strings.Contains(candidateDir, "__tests__") && strings.Contains(candidateStem, targetStem) {
		return 85, "test directory companion"
	}
	if targetDir == candidateDir {
		return 40, "same directory"
	}
	if isRemoteProjectManifest(candidateBase) {
		return 20, "project manifest"
	}
	return 0, ""
}

func isRemoteTestCompanion(targetStem string, candidateStem string, candidateBase string) bool {
	if targetStem == "" {
		return false
	}
	if candidateStem == targetStem+"_test" {
		return true
	}
	for _, marker := range []string{".test", ".spec", ".stories"} {
		if strings.HasPrefix(candidateStem, targetStem+marker) {
			return true
		}
	}
	return strings.Contains(candidateBase, targetStem) && (strings.Contains(candidateBase, "test") || strings.Contains(candidateBase, "spec"))
}

func isRemoteProjectManifest(name string) bool {
	for _, manifest := range remoteProjectManifestNames {
		if name == manifest {
			return true
		}
	}
	return false
}

func encodeLimitedRemoteJSON(value any, maxBytes int64) (string, bool, error) {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", false, ToolError{Code: "REMOTE_RESULT_ENCODE_FAILED", Message: err.Error()}
	}
	limited := enforceRemoteTextLimit(string(bytes), maxBytes)
	return limited.Text, limited.Truncated, nil
}
