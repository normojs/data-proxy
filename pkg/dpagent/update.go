package dpagent

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultAgentUpdateRepo      = "normojs/data-proxy"
	DefaultAgentUpdateGitHubAPI = "https://api.github.com"
	DefaultAgentUpdateTimeout   = 2 * time.Minute
)

type AgentUpdateOptions struct {
	CurrentVersion  string
	Version         string
	Repo            string
	ManifestURL     string
	GitHubAPIBase   string
	InstallPath     string
	Platform        string
	Arch            string
	DryRun          bool
	SkipSelfTest    bool
	SkipChecksum    bool
	AllowPrerelease bool
	Timeout         time.Duration
	Out             io.Writer
	Err             io.Writer
	HTTPClient      *http.Client
}

type AgentUpdateResult struct {
	Version     string
	AssetName   string
	AssetURL    string
	InstallPath string
	BackupPath  string
	StagedPath  string
	DryRun      bool
}

type AgentUpdateCheckOptions struct {
	CurrentVersion  string
	Version         string
	Repo            string
	ManifestURL     string
	GitHubAPIBase   string
	Platform        string
	Arch            string
	AllowPrerelease bool
	Timeout         time.Duration
	HTTPClient      *http.Client
}

type AgentUpdateCheckResult struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	AssetName       string `json:"asset_name,omitempty"`
	AssetURL        string `json:"asset_url,omitempty"`
}

type agentUpdateAsset struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	SHA256URL string `json:"sha256_url,omitempty"`
}

type agentUpdateManifest struct {
	Version string             `json:"version"`
	Assets  []agentUpdateAsset `json:"assets"`
}

type githubRelease struct {
	TagName    string               `json:"tag_name"`
	Prerelease bool                 `json:"prerelease"`
	Assets     []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (c CLI) runUpdate(args []string) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	version := fs.String("version", "latest", "release version, for example latest or v1.2.3")
	repo := fs.String("repo", DefaultAgentUpdateRepo, "GitHub repo in owner/name form")
	manifestURL := fs.String("manifest-url", "", "custom update manifest URL")
	githubAPIBase := fs.String("github-api", DefaultAgentUpdateGitHubAPI, "GitHub API base URL")
	installPath := fs.String("install-path", "", "binary install path; defaults to current executable")
	dryRun := fs.Bool("dry-run", false, "resolve update but do not download or install")
	skipSelfTest := fs.Bool("skip-self-test", false, "skip running downloaded binary self-test")
	skipChecksum := fs.Bool("skip-checksum", false, "allow update without sha256")
	allowPrerelease := fs.Bool("allow-prerelease", false, "allow prerelease GitHub releases")
	timeout := fs.Duration("timeout", DefaultAgentUpdateTimeout, "download and self-test timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	result, err := UpdateAgent(context.Background(), AgentUpdateOptions{
		CurrentVersion:  c.Version,
		Version:         *version,
		Repo:            *repo,
		ManifestURL:     *manifestURL,
		GitHubAPIBase:   *githubAPIBase,
		InstallPath:     *installPath,
		DryRun:          *dryRun,
		SkipSelfTest:    *skipSelfTest,
		SkipChecksum:    *skipChecksum,
		AllowPrerelease: *allowPrerelease,
		Timeout:         *timeout,
		Out:             c.Out,
		Err:             c.Err,
	})
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	if result.DryRun {
		fmt.Fprintf(c.Out, "update_available: %s\n", result.Version)
		fmt.Fprintf(c.Out, "asset: %s\n", result.AssetName)
		fmt.Fprintf(c.Out, "install_path: %s\n", result.InstallPath)
		return 0
	}
	if result.StagedPath != "" {
		fmt.Fprintf(c.Out, "update staged: %s\n", result.StagedPath)
		fmt.Fprintf(c.Out, "replace %s after stopping %s\n", result.InstallPath, c.programName())
		return 0
	}
	fmt.Fprintf(c.Out, "updated %s to %s\n", c.programName(), result.Version)
	if result.BackupPath != "" {
		fmt.Fprintf(c.Out, "backup: %s\n", result.BackupPath)
	}
	return 0
}

func UpdateAgent(ctx context.Context, opts AgentUpdateOptions) (AgentUpdateResult, error) {
	opts = normalizeAgentUpdateOptions(opts)
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	asset, version, err := resolveAgentUpdateAsset(ctx, opts)
	if err != nil {
		return AgentUpdateResult{}, err
	}
	installPath, err := resolveAgentInstallPath(opts.InstallPath, opts.Platform)
	if err != nil {
		return AgentUpdateResult{}, err
	}
	result := AgentUpdateResult{
		Version:     version,
		AssetName:   asset.Name,
		AssetURL:    asset.URL,
		InstallPath: installPath,
		DryRun:      opts.DryRun,
	}
	if opts.DryRun {
		return result, nil
	}

	tmpDir, err := os.MkdirTemp("", "data-proxy-agent-update-*")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(tmpDir)

	checksum, err := resolveAssetChecksum(ctx, opts.HTTPClient, asset, opts.SkipChecksum)
	if err != nil {
		return result, err
	}
	archivePath := filepath.Join(tmpDir, asset.Name)
	if err := downloadFile(ctx, opts.HTTPClient, asset.URL, archivePath); err != nil {
		return result, err
	}
	if checksum != "" {
		if err := verifyFileSHA256(archivePath, checksum); err != nil {
			return result, err
		}
	}
	binaryPath, err := extractAgentBinary(archivePath, filepath.Join(tmpDir, "extract"), opts.Platform)
	if err != nil {
		return result, err
	}
	if !opts.SkipSelfTest {
		if err := runAgentSelfTest(ctx, binaryPath); err != nil {
			return result, err
		}
	}
	stagedPath, err := stageAgentBinary(binaryPath, installPath, opts.Platform)
	if err != nil {
		return result, err
	}
	result.StagedPath = stagedPath
	if opts.Platform == "windows" && isCurrentExecutablePath(installPath) {
		return result, nil
	}
	backupPath, err := replaceAgentBinary(stagedPath, installPath)
	if err != nil {
		return result, err
	}
	result.BackupPath = backupPath
	result.StagedPath = ""
	return result, nil
}

func CheckAgentUpdate(ctx context.Context, opts AgentUpdateCheckOptions) (AgentUpdateCheckResult, error) {
	updateOpts := normalizeAgentUpdateOptions(AgentUpdateOptions{
		CurrentVersion:  opts.CurrentVersion,
		Version:         opts.Version,
		Repo:            opts.Repo,
		ManifestURL:     opts.ManifestURL,
		GitHubAPIBase:   opts.GitHubAPIBase,
		Platform:        opts.Platform,
		Arch:            opts.Arch,
		AllowPrerelease: opts.AllowPrerelease,
		Timeout:         opts.Timeout,
		HTTPClient:      opts.HTTPClient,
	})
	ctx, cancel := context.WithTimeout(ctx, updateOpts.Timeout)
	defer cancel()
	asset, version, err := resolveAgentUpdateAsset(ctx, updateOpts)
	if err != nil {
		return AgentUpdateCheckResult{}, err
	}
	current := strings.TrimSpace(updateOpts.CurrentVersion)
	if current == "" {
		current = DefaultAgentVersion
	}
	return AgentUpdateCheckResult{
		CurrentVersion:  current,
		LatestVersion:   version,
		UpdateAvailable: agentVersionIsNewer(current, version),
		AssetName:       asset.Name,
		AssetURL:        asset.URL,
	}, nil
}

func normalizeAgentUpdateOptions(opts AgentUpdateOptions) AgentUpdateOptions {
	if strings.TrimSpace(opts.Version) == "" {
		opts.Version = "latest"
	}
	if strings.TrimSpace(opts.Repo) == "" {
		opts.Repo = DefaultAgentUpdateRepo
	}
	if strings.TrimSpace(opts.GitHubAPIBase) == "" {
		opts.GitHubAPIBase = DefaultAgentUpdateGitHubAPI
	}
	if strings.TrimSpace(opts.Platform) == "" {
		opts.Platform = runtime.GOOS
	}
	if strings.TrimSpace(opts.Arch) == "" {
		opts.Arch = runtime.GOARCH
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultAgentUpdateTimeout
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Err == nil {
		opts.Err = io.Discard
	}
	return opts
}

func agentVersionIsNewer(current string, candidate string) bool {
	current = strings.TrimSpace(current)
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false
	}
	if strings.EqualFold(normalizeAgentVersionTag(current), normalizeAgentVersionTag(candidate)) {
		return false
	}
	if current == "" || strings.Contains(strings.ToLower(current), "dev") {
		return true
	}
	currentParts, currentOK := parseAgentSemver(current)
	candidateParts, candidateOK := parseAgentSemver(candidate)
	if currentOK && candidateOK {
		for index := 0; index < len(currentParts); index++ {
			if candidateParts[index] > currentParts[index] {
				return true
			}
			if candidateParts[index] < currentParts[index] {
				return false
			}
		}
		return false
	}
	return !strings.EqualFold(current, candidate)
}

func normalizeAgentVersionTag(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "v")
	value = strings.TrimSuffix(value, ".0.0")
	return value
}

func parseAgentSemver(value string) ([3]int, bool) {
	var out [3]int
	value = normalizeAgentVersionTag(value)
	if cut, _, ok := strings.Cut(value, "+"); ok {
		value = cut
	}
	if cut, _, ok := strings.Cut(value, "-"); ok {
		value = cut
	}
	parts := strings.Split(value, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, false
	}
	for index, part := range parts {
		parsed, err := strconv.Atoi(part)
		if err != nil || parsed < 0 {
			return out, false
		}
		out[index] = parsed
	}
	return out, true
}

func resolveAgentUpdateAsset(ctx context.Context, opts AgentUpdateOptions) (agentUpdateAsset, string, error) {
	if strings.TrimSpace(opts.ManifestURL) != "" {
		return resolveAgentUpdateAssetFromManifest(ctx, opts)
	}
	return resolveAgentUpdateAssetFromGitHub(ctx, opts)
}

func resolveAgentUpdateAssetFromManifest(ctx context.Context, opts AgentUpdateOptions) (agentUpdateAsset, string, error) {
	var manifest agentUpdateManifest
	if err := fetchJSON(ctx, opts.HTTPClient, opts.ManifestURL, &manifest); err != nil {
		return agentUpdateAsset{}, "", err
	}
	if strings.TrimSpace(manifest.Version) == "" {
		manifest.Version = opts.Version
	}
	asset, ok := selectAgentUpdateAsset(manifest.Assets, opts.Platform, opts.Arch)
	if !ok {
		return agentUpdateAsset{}, manifest.Version, fmt.Errorf("no dpa/data-proxy-agent asset found for %s/%s", opts.Platform, opts.Arch)
	}
	return asset, manifest.Version, nil
}

func resolveAgentUpdateAssetFromGitHub(ctx context.Context, opts AgentUpdateOptions) (agentUpdateAsset, string, error) {
	releaseURL, err := githubReleaseURL(opts.GitHubAPIBase, opts.Repo, opts.Version)
	if err != nil {
		return agentUpdateAsset{}, "", err
	}
	var release githubRelease
	if err := fetchJSON(ctx, opts.HTTPClient, releaseURL, &release); err != nil {
		return agentUpdateAsset{}, "", err
	}
	if release.Prerelease && !opts.AllowPrerelease {
		return agentUpdateAsset{}, release.TagName, fmt.Errorf("release %s is prerelease; pass --allow-prerelease to use it", release.TagName)
	}
	assets := make([]agentUpdateAsset, 0, len(release.Assets))
	checksums := map[string]string{}
	for _, ghAsset := range release.Assets {
		if strings.HasSuffix(ghAsset.Name, ".sha256") {
			checksums[strings.TrimSuffix(ghAsset.Name, ".sha256")] = ghAsset.BrowserDownloadURL
			continue
		}
		assets = append(assets, agentUpdateAsset{Name: ghAsset.Name, URL: ghAsset.BrowserDownloadURL})
	}
	asset, ok := selectAgentUpdateAsset(assets, opts.Platform, opts.Arch)
	if !ok {
		return agentUpdateAsset{}, release.TagName, fmt.Errorf("no dpa/data-proxy-agent release asset found for %s/%s in %s", opts.Platform, opts.Arch, release.TagName)
	}
	asset.SHA256URL = checksums[asset.Name]
	return asset, release.TagName, nil
}

func githubReleaseURL(apiBase string, repo string, version string) (string, error) {
	if strings.Count(repo, "/") != 1 {
		return "", fmt.Errorf("repo must be owner/name, got %q", repo)
	}
	base, err := url.Parse(strings.TrimRight(apiBase, "/"))
	if err != nil {
		return "", err
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("github api base is invalid: %s", apiBase)
	}
	if strings.EqualFold(strings.TrimSpace(version), "latest") {
		base.Path = strings.TrimRight(base.Path, "/") + "/repos/" + repo + "/releases/latest"
	} else {
		base.Path = strings.TrimRight(base.Path, "/") + "/repos/" + repo + "/releases/tags/" + url.PathEscape(strings.TrimSpace(version))
	}
	return base.String(), nil
}

func selectAgentUpdateAsset(assets []agentUpdateAsset, platform string, arch string) (agentUpdateAsset, bool) {
	for _, asset := range assets {
		if assetMatchesPlatform(asset, platform, arch) {
			return asset, true
		}
	}
	return agentUpdateAsset{}, false
}

func assetMatchesPlatform(asset agentUpdateAsset, platform string, arch string) bool {
	if strings.TrimSpace(asset.URL) == "" || strings.TrimSpace(asset.Name) == "" {
		return false
	}
	if asset.OS != "" && asset.OS != platform {
		return false
	}
	if asset.Arch != "" && asset.Arch != arch {
		return false
	}
	name := strings.ToLower(asset.Name)
	if strings.HasSuffix(name, ".sha256") || strings.Contains(name, "checksums") {
		return false
	}
	if !assetNameMatchesAgent(name) {
		return false
	}
	if !strings.Contains(name, platform) || !strings.Contains(name, arch) {
		return false
	}
	if platform == "windows" {
		return strings.HasSuffix(name, ".zip")
	}
	return strings.HasSuffix(name, ".tar.gz")
}

func assetNameMatchesAgent(name string) bool {
	name = strings.ToLower(name)
	if strings.Contains(name, LegacyAgentCommandName) {
		return true
	}
	for _, marker := range []string{DefaultAgentCommandName + "-", DefaultAgentCommandName + "_", DefaultAgentCommandName + "."} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func resolveAgentInstallPath(path string, platform string) (string, error) {
	rawPath := strings.TrimSpace(path)
	if rawPath == "" {
		executable, err := os.Executable()
		if err != nil {
			return "", err
		}
		path = executable
		rawPath = path
	}
	absolute, err := filepath.Abs(expandPath(path))
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(rawPath, "/") || strings.HasSuffix(rawPath, "\\") {
		return filepath.Join(absolute, agentBinaryName(platform)), nil
	}
	if info, err := os.Stat(absolute); err == nil && info.IsDir() {
		return filepath.Join(absolute, agentBinaryName(platform)), nil
	}
	return absolute, nil
}

func fetchJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "data-proxy-agent-updater")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("GET %s failed: %s: %s", endpoint, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func downloadFile(ctx context.Context, client *http.Client, endpoint string, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "data-proxy-agent-updater")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("download %s failed: %s: %s", endpoint, resp.Status, strings.TrimSpace(string(body)))
	}
	if err := os.MkdirAll(filepath.Dir(path), DefaultConfigFolderMode); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return err
}

func resolveAssetChecksum(ctx context.Context, client *http.Client, asset agentUpdateAsset, skipChecksum bool) (string, error) {
	if strings.TrimSpace(asset.SHA256) != "" {
		return normalizeSHA256(asset.SHA256)
	}
	if strings.TrimSpace(asset.SHA256URL) != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.SHA256URL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "data-proxy-agent-updater")
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("download checksum %s failed: %s", asset.SHA256URL, resp.Status)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if err != nil {
			return "", err
		}
		return normalizeSHA256(string(body))
	}
	if skipChecksum {
		return "", nil
	}
	return "", errors.New("release asset is missing sha256; pass --skip-checksum only for trusted local testing")
}

func normalizeSHA256(value string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return "", errors.New("sha256 is empty")
	}
	checksum := strings.ToLower(fields[0])
	if len(checksum) != sha256.Size*2 {
		return "", fmt.Errorf("sha256 has invalid length: %d", len(checksum))
	}
	if _, err := hex.DecodeString(checksum); err != nil {
		return "", fmt.Errorf("sha256 is invalid: %w", err)
	}
	return checksum, nil
}

func verifyFileSHA256(path string, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("sha256 mismatch for %s: expected %s got %s", filepath.Base(path), expected, actual)
	}
	return nil
}

func extractAgentBinary(archivePath string, destDir string, platform string) (string, error) {
	if err := os.MkdirAll(destDir, DefaultConfigFolderMode); err != nil {
		return "", err
	}
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return extractAgentBinaryFromZip(archivePath, destDir, platform)
	}
	if strings.HasSuffix(strings.ToLower(archivePath), ".tar.gz") {
		return extractAgentBinaryFromTarGz(archivePath, destDir, platform)
	}
	return "", fmt.Errorf("unsupported agent archive: %s", filepath.Base(archivePath))
}

func extractAgentBinaryFromTarGz(archivePath string, destDir string, platform string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	expected := agentBinaryName(platform)
	fallback := ""
	allowed := agentBinaryNameSet(platform)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		base := filepath.Base(header.Name)
		if header.Typeflag != tar.TypeReg || !allowed[base] {
			continue
		}
		path, err := writeExtractedAgentBinary(tr, destDir, base, header.FileInfo().Mode().Perm())
		if err != nil {
			return "", err
		}
		if base == expected {
			return path, nil
		}
		fallback = path
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("%s not found in %s", strings.Join(agentBinaryNames(platform), " or "), filepath.Base(archivePath))
}

func extractAgentBinaryFromZip(archivePath string, destDir string, platform string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	for _, expected := range agentBinaryNames(platform) {
		for _, file := range reader.File {
			if file.FileInfo().IsDir() || filepath.Base(file.Name) != expected {
				continue
			}
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			path, writeErr := writeExtractedAgentBinary(rc, destDir, expected, file.FileInfo().Mode().Perm())
			closeErr := rc.Close()
			if writeErr != nil {
				return "", writeErr
			}
			if closeErr != nil {
				return "", closeErr
			}
			return path, nil
		}
	}
	return "", fmt.Errorf("%s not found in %s", strings.Join(agentBinaryNames(platform), " or "), filepath.Base(archivePath))
}

func writeExtractedAgentBinary(reader io.Reader, destDir string, binaryName string, mode os.FileMode) (string, error) {
	if mode == 0 {
		mode = 0o755
	}
	path := filepath.Join(destDir, binaryName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode|0o700)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(file, reader); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func agentBinaryName(platform string) string {
	return agentPrimaryBinaryName(platform)
}

func agentBinaryNames(platform string) []string {
	primary := agentPrimaryBinaryName(platform)
	legacy := agentLegacyBinaryName(platform)
	if primary == legacy {
		return []string{primary}
	}
	return []string{primary, legacy}
}

func agentBinaryNameSet(platform string) map[string]bool {
	result := map[string]bool{}
	for _, name := range agentBinaryNames(platform) {
		result[name] = true
	}
	return result
}

func agentPrimaryBinaryName(platform string) string {
	if platform == "windows" {
		return DefaultAgentCommandName + ".exe"
	}
	return DefaultAgentCommandName
}

func agentLegacyBinaryName(platform string) string {
	if platform == "windows" {
		return LegacyAgentCommandName + ".exe"
	}
	return LegacyAgentCommandName
}

func runAgentSelfTest(ctx context.Context, binaryPath string) error {
	cmd := exec.CommandContext(ctx, binaryPath, "self-test")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("downloaded agent self-test failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func stageAgentBinary(binaryPath string, installPath string, platform string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(installPath), 0o755); err != nil {
		return "", err
	}
	suffix := ".new"
	if platform == "windows" {
		suffix = ".new.exe"
	}
	stagedPath := installPath + suffix
	if err := copyFile(binaryPath, stagedPath, 0o755); err != nil {
		return "", err
	}
	return stagedPath, nil
}

func replaceAgentBinary(stagedPath string, installPath string) (string, error) {
	backupPath := ""
	if _, err := os.Stat(installPath); err == nil {
		backupPath = installPath + ".bak"
		_ = os.Remove(backupPath)
		if err := os.Rename(installPath, backupPath); err != nil {
			return backupPath, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Rename(stagedPath, installPath); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, installPath)
		}
		return backupPath, err
	}
	if err := os.Chmod(installPath, 0o755); err != nil {
		return backupPath, err
	}
	return backupPath, nil
}

func copyFile(src string, dst string, mode os.FileMode) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

func isCurrentExecutablePath(path string) bool {
	current, err := os.Executable()
	if err != nil {
		return false
	}
	left, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	right, err := filepath.Abs(current)
	if err != nil {
		return false
	}
	return filepath.Clean(left) == filepath.Clean(right)
}
