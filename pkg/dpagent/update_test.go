package dpagent

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIUpdateDryRunFromManifest(t *testing.T) {
	assetName := "data-proxy-agent-v1.2.3-" + runtime.GOOS + "-" + runtime.GOARCH + agentTestArchiveExt(runtime.GOOS)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/manifest.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(agentUpdateManifest{
			Version: "v1.2.3",
			Assets: []agentUpdateAsset{{
				Name:   assetName,
				URL:    serverURLFromRequest(r) + "/" + assetName,
				OS:     runtime.GOOS,
				Arch:   runtime.GOARCH,
				SHA256: strings.Repeat("a", 64),
			}},
		})
	}))
	defer server.Close()

	var out, errOut bytes.Buffer
	code := RunCLI([]string{
		"update",
		"--manifest-url", server.URL + "/manifest.json",
		"--install-path", filepath.Join(t.TempDir(), "data-proxy-agent"),
		"--dry-run",
	}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("update dry-run failed with code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "update_available: v1.2.3") || !strings.Contains(out.String(), runtime.GOOS+"-"+runtime.GOARCH) {
		t.Fatalf("unexpected update dry-run output: %s", out.String())
	}
}

func TestUpdateAgentInstallsFromManifestWithChecksum(t *testing.T) {
	archiveBytes := buildAgentTarGz(t, "new agent bytes")
	checksum := sha256Hex(archiveBytes)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			_ = json.NewEncoder(w).Encode(agentUpdateManifest{
				Version: "v1.2.3",
				Assets: []agentUpdateAsset{{
					Name:   "data-proxy-agent-v1.2.3-linux-amd64.tar.gz",
					URL:    serverURLFromRequest(r) + "/data-proxy-agent-v1.2.3-linux-amd64.tar.gz",
					OS:     "linux",
					Arch:   "amd64",
					SHA256: checksum,
				}},
			})
		case "/data-proxy-agent-v1.2.3-linux-amd64.tar.gz":
			_, _ = w.Write(archiveBytes)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	installPath := filepath.Join(t.TempDir(), "bin", "data-proxy-agent")
	if err := os.MkdirAll(filepath.Dir(installPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installPath, []byte("old agent bytes"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := UpdateAgent(t.Context(), AgentUpdateOptions{
		Version:      "latest",
		ManifestURL:  server.URL + "/manifest.json",
		InstallPath:  installPath,
		Platform:     "linux",
		Arch:         "amd64",
		SkipSelfTest: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != "v1.2.3" || result.BackupPath == "" || result.StagedPath != "" {
		t.Fatalf("unexpected update result: %#v", result)
	}
	installed, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != "new agent bytes" {
		t.Fatalf("unexpected installed binary bytes: %q", string(installed))
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != "old agent bytes" {
		t.Fatalf("unexpected backup bytes: %q", string(backup))
	}
}

func TestUpdateAgentRequiresChecksum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(agentUpdateManifest{
			Version: "v1.2.3",
			Assets: []agentUpdateAsset{{
				Name: "data-proxy-agent-v1.2.3-linux-amd64.tar.gz",
				URL:  serverURLFromRequest(r) + "/data-proxy-agent-v1.2.3-linux-amd64.tar.gz",
				OS:   "linux",
				Arch: "amd64",
			}},
		})
	}))
	defer server.Close()

	_, err := UpdateAgent(t.Context(), AgentUpdateOptions{
		ManifestURL:  server.URL,
		InstallPath:  filepath.Join(t.TempDir(), "data-proxy-agent"),
		Platform:     "linux",
		Arch:         "amd64",
		SkipSelfTest: true,
	})
	if err == nil || !strings.Contains(err.Error(), "missing sha256") {
		t.Fatalf("expected missing checksum error, got %v", err)
	}
}

func TestResolveAgentUpdateAssetFromGitHub(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/normojs/data-proxy/releases/latest" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(githubRelease{
			TagName: "v1.2.3",
			Assets: []githubReleaseAsset{
				{Name: "data-proxy-agent-v1.2.3-darwin-arm64.tar.gz", BrowserDownloadURL: serverURLFromRequest(r) + "/darwin.tar.gz"},
				{Name: "data-proxy-agent-v1.2.3-linux-amd64.tar.gz", BrowserDownloadURL: serverURLFromRequest(r) + "/linux.tar.gz"},
				{Name: "data-proxy-agent-v1.2.3-linux-amd64.tar.gz.sha256", BrowserDownloadURL: serverURLFromRequest(r) + "/linux.tar.gz.sha256"},
			},
		})
	}))
	defer server.Close()

	asset, version, err := resolveAgentUpdateAssetFromGitHub(t.Context(), AgentUpdateOptions{
		Version:       "latest",
		Repo:          DefaultAgentUpdateRepo,
		GitHubAPIBase: server.URL,
		Platform:      "linux",
		Arch:          "amd64",
		HTTPClient:    server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if version != "v1.2.3" || !strings.Contains(asset.Name, "linux-amd64") || !strings.HasSuffix(asset.SHA256URL, ".sha256") {
		t.Fatalf("unexpected github asset: version=%s asset=%#v", version, asset)
	}
}

func TestResolveAgentInstallPathAllowsDirectory(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveAgentInstallPath(dir, "linux")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "dpa") {
		t.Fatalf("unexpected install path: %s", got)
	}
	got, err = resolveAgentInstallPath(dir+string(os.PathSeparator), "windows")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "dpa.exe") {
		t.Fatalf("unexpected windows install path: %s", got)
	}
}

func TestExtractAgentBinaryAcceptsLegacyArchive(t *testing.T) {
	archiveBytes := buildAgentTarGzWithBinaryName(t, "legacy agent bytes", "data-proxy-agent")
	archivePath := filepath.Join(t.TempDir(), "data-proxy-agent-v1.2.3-linux-amd64.tar.gz")
	if err := os.WriteFile(archivePath, archiveBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	extracted, err := extractAgentBinary(archivePath, filepath.Join(t.TempDir(), "extract"), "linux")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(extracted) != "data-proxy-agent" {
		t.Fatalf("unexpected extracted binary: %s", extracted)
	}
	body, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "legacy agent bytes" {
		t.Fatalf("unexpected extracted bytes: %q", string(body))
	}
}

func buildAgentTarGz(t *testing.T, body string) []byte {
	return buildAgentTarGzWithBinaryName(t, body, "dpa")
}

func buildAgentTarGzWithBinaryName(t *testing.T, body string, binaryName string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	payload := []byte(body)
	if err := tw.WriteHeader(&tar.Header{
		Name: "data-proxy-agent-v1.2.3-linux-amd64/" + binaryName,
		Mode: 0o755,
		Size: int64(len(payload)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func agentTestArchiveExt(platform string) string {
	if platform == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}
