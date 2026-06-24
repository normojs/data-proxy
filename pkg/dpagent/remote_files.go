package dpagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/dto"
)

const (
	BridgeToolRemoteRead    = "remote_read"
	BridgeToolRemoteTree    = "remote_tree"
	BridgeToolRemoteGlob    = "remote_glob"
	BridgeToolRemoteGrep    = "remote_grep"
	BridgeToolRemoteEnvInfo = "remote_env_info"

	DefaultRemoteMaxResults       = 200
	DefaultRemoteTreeDepth        = 3
	DefaultRemoteWalkDepth        = 8
	DefaultRemoteMaxResultBytes   = int64(512 * 1024)
	DefaultRemoteMaxScanFileBytes = int64(2 * 1024 * 1024)
	remoteHardMaxResults          = 5000
	remoteHardTreeDepth           = 16
	remoteHardWalkDepth           = 32
	remoteHardMaxResultBytes      = int64(50 * 1024 * 1024)
	remoteHardMaxScanFileBytes    = int64(100 * 1024 * 1024)
	remoteDefaultReadLineLimit    = 100
	remoteHardReadLineLimit       = 100000
	remoteTruncatedMarker         = "\n\n[result truncated by data-proxy-agent]"
)

var remoteDefaultIgnores = map[string]bool{
	".git":         true,
	".hg":          true,
	".svn":         true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".cache":       true,
}

type remoteFileLimits struct {
	MaxResults       int
	TreeDepth        int
	WalkDepth        int
	MaxResultBytes   int64
	MaxScanFileBytes int64
}

type remotePathInfo struct {
	Root string
	Path string
	Rel  string
}

type remoteWalkItem struct {
	Path  string
	Type  string
	Depth int
}

type remoteWalkResult struct {
	Items     []remoteWalkItem
	Truncated bool
}

type remoteLimitedText struct {
	Text      string
	Bytes     int
	Truncated bool
}

func (c BridgeClient) handleRemoteFileTool(ctx context.Context, toolName string, args map[string]any) (dto.BridgeToolCallResult, error) {
	switch toolName {
	case BridgeToolRemoteRead:
		return c.handleRemoteRead(ctx, args)
	case BridgeToolRemoteTree:
		return c.handleRemoteTree(ctx, args)
	case BridgeToolRemoteGlob:
		return c.handleRemoteGlob(ctx, args)
	case BridgeToolRemoteGrep:
		return c.handleRemoteGrep(ctx, args)
	case BridgeToolRemoteEnvInfo:
		return c.handleRemoteEnvInfo(ctx, args)
	default:
		return dto.BridgeToolCallResult{}, ToolError{
			Code:    "REMOTE_TOOL_NOT_SUPPORTED",
			Message: fmt.Sprintf("unsupported remote file tool: %s", toolName),
		}
	}
}

func (c BridgeClient) handleRemoteRead(_ context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	info, err := resolveExistingRemotePath(c.Config, stringFromMap(args, "file_path", ""), "", "REMOTE_READ")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	stat, err := os.Stat(info.Path)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_READ_NOT_FOUND", Message: err.Error()}
	}
	if !stat.Mode().IsRegular() {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_READ_NOT_FILE", Message: "target is not a regular file: " + info.Rel}
	}
	raw, err := os.ReadFile(info.Path)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_READ_FAILED", Message: err.Error()}
	}
	offset := remotePositiveInt(args["offset"], 1, remoteHardReadLineLimit)
	limit := remotePositiveInt(args["limit"], remoteDefaultReadLineLimit, remoteHardReadLineLimit)
	sliced := sliceRemoteLines(string(raw), offset, limit)
	limited := enforceRemoteTextLimit(sliced.Text, remoteLimitsFromConfig(c.Config, args).MaxResultBytes)
	truncated := limited.Truncated
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: limited.Text}},
		Summary:    fmt.Sprintf("%s:%d-%d", info.Rel, sliced.StartLine, sliced.EndLine),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: limited.Bytes,
		Metadata: map[string]any{
			"file_path":   info.Rel,
			"offset":      offset,
			"limit":       limit,
			"total_lines": sliced.TotalLines,
			"truncated":   truncated,
			"daemon":      true,
		},
	}, nil
}

func (c BridgeClient) handleRemoteTree(_ context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	info, err := resolveExistingRemotePath(c.Config, stringFromMap(args, "path", "."), ".", "REMOTE_TREE")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	stat, err := os.Stat(info.Path)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_TREE_NOT_FOUND", Message: err.Error()}
	}
	if !stat.IsDir() {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_TREE_NOT_DIRECTORY", Message: "target is not a directory: " + info.Rel}
	}
	limits := remoteLimitsFromConfig(c.Config, args)
	depth := remotePositiveInt(firstAny(args["depth"], args["max_depth"]), limits.TreeDepth, limits.TreeDepth)
	maxResults := remotePositiveInt(args["max_results"], limits.MaxResults, limits.MaxResults)
	walked := walkRemoteWorkspace(c.Config, info.Root, info.Path, depth, maxResults, true)

	lines := []string{info.Rel + "/"}
	for _, item := range walked.Items {
		rel := relativeRemotePath(info.Root, item.Path)
		indent := strings.Repeat("  ", maxInt(item.Depth-1, 0))
		prefix := "-"
		if item.Type == "directory" {
			prefix = "d"
		}
		lines = append(lines, fmt.Sprintf("%s%s %s", indent, prefix, rel))
	}
	limited := enforceRemoteTextLimit(strings.Join(lines, "\n"), limits.MaxResultBytes)
	truncated := walked.Truncated || limited.Truncated
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: limited.Text}},
		Summary:    fmt.Sprintf("%s (%d entries)", info.Rel, len(walked.Items)),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: limited.Bytes,
		Metadata: map[string]any{
			"path":      info.Rel,
			"depth":     depth,
			"count":     len(walked.Items),
			"truncated": truncated,
			"daemon":    true,
		},
	}, nil
}

func (c BridgeClient) handleRemoteGlob(_ context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	pattern := stringFromMap(args, "pattern", "")
	if pattern == "" {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GLOB_INVALID_ARGUMENT", Message: "pattern must be a non-empty string"}
	}
	info, err := resolveExistingRemotePath(c.Config, stringFromMap(args, "path", "."), ".", "REMOTE_GLOB")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	limits := remoteLimitsFromConfig(c.Config, args)
	maxResults := remotePositiveInt(args["max_results"], limits.MaxResults, limits.MaxResults)
	maxDepth := remotePositiveInt(args["max_depth"], limits.WalkDepth, limits.WalkDepth)
	matcher, err := globToRegexp(pattern)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GLOB_INVALID_PATTERN", Message: err.Error()}
	}
	walked := walkRemoteWorkspace(c.Config, info.Root, info.Path, maxDepth, scanCandidateLimit(limits, maxResults, 4), false)
	matches := make([]string, 0, maxResults)
	for _, item := range walked.Items {
		rel := relativeRemotePath(info.Root, item.Path)
		if !matcher.MatchString(rel) && !matcher.MatchString(filepath.Base(rel)) {
			continue
		}
		matches = append(matches, rel)
		if len(matches) >= maxResults {
			break
		}
	}
	limited := enforceRemoteTextLimit(strings.Join(matches, "\n"), limits.MaxResultBytes)
	truncated := len(matches) >= maxResults || walked.Truncated || limited.Truncated
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: limited.Text}},
		Summary:    fmt.Sprintf("%d files matched %s", len(matches), pattern),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: limited.Bytes,
		Metadata: map[string]any{
			"pattern":   pattern,
			"path":      info.Rel,
			"count":     len(matches),
			"truncated": truncated,
			"daemon":    true,
		},
	}, nil
}

func (c BridgeClient) handleRemoteGrep(_ context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	pattern := stringFromMap(args, "pattern", "")
	if pattern == "" {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GREP_INVALID_ARGUMENT", Message: "pattern must be a non-empty string"}
	}
	info, err := resolveExistingRemotePath(c.Config, stringFromMap(args, "path", "."), ".", "REMOTE_GREP")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	regexpPattern := pattern
	if boolFromMap(args, "case_insensitive") {
		regexpPattern = "(?i)" + regexpPattern
	}
	matcher, err := regexp.Compile(regexpPattern)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GREP_INVALID_PATTERN", Message: err.Error()}
	}
	var globMatcher *regexp.Regexp
	if globPattern := firstString(args, "glob", "glob_pattern"); globPattern != "" {
		globMatcher, err = globToRegexp(globPattern)
		if err != nil {
			return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GREP_INVALID_GLOB", Message: err.Error()}
		}
	}

	limits := remoteLimitsFromConfig(c.Config, args)
	maxResults := remotePositiveInt(args["max_results"], limits.MaxResults, limits.MaxResults)
	maxDepth := remotePositiveInt(args["max_depth"], limits.WalkDepth, limits.WalkDepth)
	candidates := []remoteWalkItem{{Path: info.Path, Type: "file", Depth: 0}}
	walkTruncated := false
	stat, err := os.Stat(info.Path)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GREP_NOT_FOUND", Message: err.Error()}
	}
	if stat.IsDir() {
		walked := walkRemoteWorkspace(c.Config, info.Root, info.Path, maxDepth, scanCandidateLimit(limits, maxResults, 8), false)
		candidates = walked.Items
		walkTruncated = walked.Truncated
	} else if !stat.Mode().IsRegular() {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_GREP_INVALID_PATH", Message: "target is not a file or directory"}
	}

	matches := make([]string, 0, maxResults)
	for _, item := range candidates {
		if len(matches) >= maxResults {
			break
		}
		rel := relativeRemotePath(info.Root, item.Path)
		if globMatcher != nil && !globMatcher.MatchString(rel) && !globMatcher.MatchString(filepath.Base(rel)) {
			continue
		}
		fileStat, err := os.Stat(item.Path)
		if err != nil || !fileStat.Mode().IsRegular() || fileStat.Size() > limits.MaxScanFileBytes {
			continue
		}
		raw, err := os.ReadFile(item.Path)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
		for index, line := range lines {
			if !matcher.MatchString(line) {
				continue
			}
			matches = append(matches, fmt.Sprintf("%s:%d: %s", rel, index+1, line))
			if len(matches) >= maxResults {
				break
			}
		}
	}
	limited := enforceRemoteTextLimit(strings.Join(matches, "\n"), limits.MaxResultBytes)
	truncated := len(matches) >= maxResults || walkTruncated || limited.Truncated
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: limited.Text}},
		Summary:    fmt.Sprintf("%d matches for %s", len(matches), pattern),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: limited.Bytes,
		Metadata: map[string]any{
			"pattern":   pattern,
			"path":      info.Rel,
			"count":     len(matches),
			"truncated": truncated,
			"daemon":    true,
		},
	}, nil
}

func (c BridgeClient) handleRemoteEnvInfo(_ context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	hostname, _ := os.Hostname()
	limits := remoteLimitsFromConfig(c.Config, args)
	data := map[string]any{
		"platform":     runtime.GOOS,
		"arch":         runtime.GOARCH,
		"go":           runtime.Version(),
		"hostname":     hostname,
		"workspace":    c.Config.Agent.Workspace,
		"client_id":    c.Config.Agent.ClientID,
		"capabilities": EffectiveCapabilities(c.Config),
		"limits": map[string]any{
			"max_results":         limits.MaxResults,
			"tree_depth":          limits.TreeDepth,
			"walk_depth":          limits.WalkDepth,
			"max_result_bytes":    limits.MaxResultBytes,
			"max_scan_file_bytes": limits.MaxScanFileBytes,
		},
		"daemon": true,
	}
	textBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_ENV_INFO_ENCODE_FAILED", Message: err.Error()}
	}
	limited := enforceRemoteTextLimit(string(textBytes), limits.MaxResultBytes)
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: limited.Text}},
		Summary:    fmt.Sprintf("%s-%s %s", runtime.GOOS, runtime.GOARCH, runtime.Version()),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: limited.Bytes,
		Metadata:   data,
	}, nil
}

type remoteLineSlice struct {
	Text       string
	TotalLines int
	StartLine  int
	EndLine    int
}

func sliceRemoteLines(text string, offset int, limit int) remoteLineSlice {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	total := len(lines)
	start := offset - 1
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := total
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	return remoteLineSlice{
		Text:       strings.Join(lines[start:end], "\n"),
		TotalLines: total,
		StartLine:  start + 1,
		EndLine:    end,
	}
}

func remoteLimitsFromConfig(cfg Config, args map[string]any) remoteFileLimits {
	limits := remoteFileLimits{
		MaxResults:       remoteCapInt(cfg.Runtime.MaxResults, DefaultRemoteMaxResults, remoteHardMaxResults),
		TreeDepth:        remoteCapInt(cfg.Runtime.TreeDepth, DefaultRemoteTreeDepth, remoteHardTreeDepth),
		WalkDepth:        remoteCapInt(cfg.Runtime.WalkDepth, DefaultRemoteWalkDepth, remoteHardWalkDepth),
		MaxResultBytes:   remoteCapInt64(cfg.Runtime.MaxResultBytes, DefaultRemoteMaxResultBytes, remoteHardMaxResultBytes),
		MaxScanFileBytes: remoteCapInt64(cfg.Runtime.MaxScanFileBytes, DefaultRemoteMaxScanFileBytes, remoteHardMaxScanFileBytes),
	}
	policy := mapFromAny(args["_bridge_policy_limits"])
	if len(policy) > 0 {
		limits.MaxResults = minPositiveInt(limits.MaxResults, remotePositiveInt(policy["max_results"], limits.MaxResults, remoteHardMaxResults))
		limits.TreeDepth = minPositiveInt(limits.TreeDepth, remotePositiveInt(policy["tree_depth"], limits.TreeDepth, remoteHardTreeDepth))
		limits.WalkDepth = minPositiveInt(limits.WalkDepth, remotePositiveInt(policy["walk_depth"], limits.WalkDepth, remoteHardWalkDepth))
		limits.MaxResultBytes = minPositiveInt64(limits.MaxResultBytes, remotePositiveInt64(policy["max_result_bytes"], limits.MaxResultBytes, remoteHardMaxResultBytes))
		limits.MaxScanFileBytes = minPositiveInt64(limits.MaxScanFileBytes, remotePositiveInt64(policy["max_scan_file_bytes"], limits.MaxScanFileBytes, remoteHardMaxScanFileBytes))
	}
	if args != nil {
		limits.MaxResultBytes = minPositiveInt64(limits.MaxResultBytes, remotePositiveInt64(args["max_result_bytes"], limits.MaxResultBytes, remoteHardMaxResultBytes))
		limits.MaxScanFileBytes = minPositiveInt64(limits.MaxScanFileBytes, remotePositiveInt64(args["max_scan_file_bytes"], limits.MaxScanFileBytes, remoteHardMaxScanFileBytes))
	}
	return limits
}

func resolveExistingRemotePath(cfg Config, requestedPath string, fallback string, codePrefix string) (remotePathInfo, error) {
	rawPath := strings.TrimSpace(requestedPath)
	if rawPath == "" {
		rawPath = strings.TrimSpace(fallback)
	}
	if rawPath == "" {
		return remotePathInfo{}, ToolError{Code: codePrefix + "_INVALID_ARGUMENT", Message: "path/file_path must be a non-empty string"}
	}
	root, allowedRoots, err := remoteWorkspaceRoots(cfg, codePrefix)
	if err != nil {
		return remotePathInfo{}, err
	}
	expanded := expandPath(rawPath)
	candidate := expanded
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return remotePathInfo{}, ToolError{Code: codePrefix + "_INVALID_ARGUMENT", Message: err.Error()}
	}
	realPath, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return remotePathInfo{}, ToolError{Code: codePrefix + "_NOT_FOUND", Message: "path does not exist: " + rawPath}
	}
	if !pathInside(root, realPath) {
		return remotePathInfo{}, ToolError{Code: codePrefix + "_FORBIDDEN", Message: "target path is outside workspace"}
	}
	if len(allowedRoots) > 0 && !pathInsideAny(allowedRoots, realPath) {
		return remotePathInfo{}, ToolError{Code: codePrefix + "_FORBIDDEN", Message: "target path is outside allowed workspaces"}
	}
	if isDeniedRemotePath(cfg, root, realPath) {
		return remotePathInfo{}, ToolError{Code: codePrefix + "_FORBIDDEN", Message: "target path is denied by local policy"}
	}
	return remotePathInfo{Root: root, Path: realPath, Rel: relativeRemotePath(root, realPath)}, nil
}

func remoteWorkspaceRoots(cfg Config, codePrefix string) (string, []string, error) {
	workspace := strings.TrimSpace(cfg.Agent.Workspace)
	if workspace == "" {
		return "", nil, ToolError{Code: codePrefix + "_WORKSPACE_MISSING", Message: "agent.workspace is required"}
	}
	workspace = expandPath(workspace)
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return "", nil, ToolError{Code: codePrefix + "_WORKSPACE_INVALID", Message: err.Error()}
	}
	root, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", nil, ToolError{Code: codePrefix + "_WORKSPACE_NOT_FOUND", Message: "workspace does not exist: " + workspace}
	}
	allowed := make([]string, 0, len(cfg.Policy.AllowedWorkspaces))
	for _, raw := range cfg.Policy.AllowedWorkspaces {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		pathValue := expandPath(raw)
		if !filepath.IsAbs(pathValue) {
			pathValue = filepath.Join(root, pathValue)
		}
		abs, err := filepath.Abs(pathValue)
		if err != nil {
			continue
		}
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			abs = real
		}
		allowed = append(allowed, abs)
	}
	return root, allowed, nil
}

func isDeniedRemotePath(cfg Config, root string, target string) bool {
	for _, raw := range cfg.Policy.DeniedPaths {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		pathValue := expandPath(raw)
		if !filepath.IsAbs(pathValue) {
			pathValue = filepath.Join(root, pathValue)
		}
		abs, err := filepath.Abs(pathValue)
		if err != nil {
			continue
		}
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			abs = real
		}
		if pathInside(abs, target) {
			return true
		}
	}
	return false
}

func walkRemoteWorkspace(cfg Config, workspaceRoot string, rootPath string, maxDepth int, maxResults int, includeDirectories bool) remoteWalkResult {
	result := remoteWalkResult{Items: []remoteWalkItem{}}
	if maxDepth <= 0 || maxResults <= 0 {
		return result
	}
	var visit func(string, int)
	visit = func(currentPath string, depth int) {
		if len(result.Items) >= maxResults {
			result.Truncated = true
			return
		}
		entries, err := os.ReadDir(currentPath)
		if err != nil {
			return
		}
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})
		for _, entry := range entries {
			if len(result.Items) >= maxResults {
				result.Truncated = true
				return
			}
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			absolute := filepath.Join(currentPath, entry.Name())
			if isDeniedRemotePath(cfg, workspaceRoot, absolute) {
				continue
			}
			if entry.IsDir() {
				if remoteDefaultIgnores[entry.Name()] {
					continue
				}
				if includeDirectories {
					result.Items = append(result.Items, remoteWalkItem{Path: absolute, Type: "directory", Depth: depth})
				}
				if depth < maxDepth {
					visit(absolute, depth+1)
				}
				continue
			}
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			result.Items = append(result.Items, remoteWalkItem{Path: absolute, Type: "file", Depth: depth})
		}
	}
	visit(rootPath, 1)
	return result
}

func enforceRemoteTextLimit(text string, maxBytes int64) remoteLimitedText {
	if maxBytes <= 0 {
		maxBytes = DefaultRemoteMaxResultBytes
	}
	if int64(len([]byte(text))) <= maxBytes {
		return remoteLimitedText{Text: text, Bytes: len([]byte(text)), Truncated: false}
	}
	var builder strings.Builder
	var size int64
	for _, r := range text {
		runeBytes := int64(utf8.RuneLen(r))
		if runeBytes < 0 {
			runeBytes = int64(len(string(r)))
		}
		if size+runeBytes > maxBytes {
			break
		}
		builder.WriteRune(r)
		size += runeBytes
	}
	builder.WriteString(remoteTruncatedMarker)
	return remoteLimitedText{Text: builder.String(), Bytes: int(size), Truncated: true}
}

func globToRegexp(pattern string) (*regexp.Regexp, error) {
	normalized := strings.ReplaceAll(pattern, "\\", "/")
	var source strings.Builder
	for i := 0; i < len(normalized); i++ {
		char := normalized[i]
		if char == '*' {
			if i+1 < len(normalized) && normalized[i+1] == '*' {
				source.WriteString(".*")
				i++
			} else {
				source.WriteString("[^/]*")
			}
			continue
		}
		if char == '?' {
			source.WriteString("[^/]")
			continue
		}
		source.WriteString(regexp.QuoteMeta(string(char)))
	}
	return regexp.Compile("^" + source.String() + "$")
}

func relativeRemotePath(root string, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "" {
		return "."
	}
	return filepath.ToSlash(rel)
}

func pathInside(basePath string, targetPath string) bool {
	rel, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func pathInsideAny(basePaths []string, targetPath string) bool {
	for _, basePath := range basePaths {
		if pathInside(basePath, targetPath) {
			return true
		}
	}
	return false
}

func scanCandidateLimit(limits remoteFileLimits, outputLimit int, multiplier int) int {
	if multiplier <= 0 {
		multiplier = 1
	}
	limit := outputLimit * multiplier
	capLimit := limits.MaxResults * multiplier
	if limit > capLimit {
		limit = capLimit
	}
	if limit > remoteHardMaxResults {
		limit = remoteHardMaxResults
	}
	if limit <= 0 {
		return limits.MaxResults
	}
	return limit
}

func firstAny(values ...any) any {
	for _, value := range values {
		if strings.TrimSpace(fmt.Sprint(value)) != "" && fmt.Sprint(value) != "<nil>" {
			return value
		}
	}
	return nil
}

func firstString(args map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringFromMap(args, key, ""); value != "" {
			return value
		}
	}
	return ""
}

func remotePositiveInt(value any, fallback int, hardMax int) int {
	if fallback <= 0 {
		fallback = 1
	}
	if hardMax <= 0 {
		hardMax = fallback
	}
	parsed, ok := parseRemoteInt64(value)
	if !ok || parsed <= 0 {
		parsed = int64(fallback)
	}
	if parsed > int64(hardMax) {
		parsed = int64(hardMax)
	}
	return int(parsed)
}

func remotePositiveInt64(value any, fallback int64, hardMax int64) int64 {
	if fallback <= 0 {
		fallback = 1
	}
	if hardMax <= 0 {
		hardMax = fallback
	}
	parsed, ok := parseRemoteInt64(value)
	if !ok || parsed <= 0 {
		parsed = fallback
	}
	if parsed > hardMax {
		parsed = hardMax
	}
	return parsed
}

func parseRemoteInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, false
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case uint:
		return int64(typed), true
	case uint8:
		return int64(typed), true
	case uint16:
		return int64(typed), true
	case uint32:
		return int64(typed), true
	case uint64:
		const maxInt64AsUint = uint64(1<<63 - 1)
		if typed > maxInt64AsUint {
			return 0, false
		}
		return int64(typed), true
	case float32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed, err == nil
	default:
		parsed, err := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(typed)), 10, 64)
		return parsed, err == nil
	}
}

func remoteCapInt(value int, fallback int, hardMax int) int {
	if value <= 0 {
		value = fallback
	}
	if value > hardMax {
		value = hardMax
	}
	return value
}

func remoteCapInt64(value int64, fallback int64, hardMax int64) int64 {
	if value <= 0 {
		value = fallback
	}
	if value > hardMax {
		value = hardMax
	}
	return value
}

func minPositiveInt(left int, right int) int {
	if left <= 0 {
		return right
	}
	if right <= 0 {
		return left
	}
	if left < right {
		return left
	}
	return right
}

func minPositiveInt64(left int64, right int64) int64 {
	if left <= 0 {
		return right
	}
	if right <= 0 {
		return left
	}
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
