package gemini

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

func TestParseImageInputDownloadsHTTPImageURL(t *testing.T) {
	allowPrivateHTTPFetchForTest(t)
	imageBytes := tinyPNGBytes(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	t.Cleanup(server.Close)

	parsed := ParseImageInput(server.URL + "/image.png")

	require.NotNil(t, parsed)
	require.Equal(t, "image/png", parsed.MimeType)
	decoded, err := base64.StdEncoding.DecodeString(parsed.BytesBase64Encoded)
	require.NoError(t, err)
	require.Equal(t, imageBytes, decoded)
}

func TestParseImageInputRejectsPrivateHTTPURLWhenSSRFBlocks(t *testing.T) {
	blockPrivateHTTPFetchForTest(t)

	parsed := ParseImageInput("http://127.0.0.1:12345/image.png")

	require.Nil(t, parsed)
}

func TestParseImageInputRejectsOversizedHTTPImage(t *testing.T) {
	allowPrivateHTTPFetchForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "20971521")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	parsed := ParseImageInput(server.URL + "/large.png")

	require.Nil(t, parsed)
}

func TestParseImageInputRejectsUnsupportedHTTPMime(t *testing.T) {
	allowPrivateHTTPFetchForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not an image"))
	}))
	t.Cleanup(server.Close)

	parsed := ParseImageInput(server.URL + "/text.txt")

	require.Nil(t, parsed)
}

func TestParseImageInputKeepsDataURIAndRawBase64Support(t *testing.T) {
	imageBytes := tinyPNGBytes(t)

	dataURI := ParseImageInput("data:image/png;base64," + tinyPNGBase64)
	require.NotNil(t, dataURI)
	require.Equal(t, "image/png", dataURI.MimeType)
	require.Equal(t, tinyPNGBase64, dataURI.BytesBase64Encoded)

	raw := ParseImageInput(tinyPNGBase64)
	require.NotNil(t, raw)
	require.Equal(t, "image/png", raw.MimeType)
	decoded, err := base64.StdEncoding.DecodeString(raw.BytesBase64Encoded)
	require.NoError(t, err)
	require.Equal(t, imageBytes, decoded)
}

func tinyPNGBytes(t *testing.T) []byte {
	t.Helper()
	imageBytes, err := base64.StdEncoding.DecodeString(tinyPNGBase64)
	require.NoError(t, err)
	return imageBytes
}

func allowPrivateHTTPFetchForTest(t *testing.T) {
	t.Helper()
	updateFetchSettingForTest(t, true)
}

func blockPrivateHTTPFetchForTest(t *testing.T) {
	t.Helper()
	updateFetchSettingForTest(t, false)
}

func updateFetchSettingForTest(t *testing.T, allowPrivateIP bool) {
	t.Helper()
	service.InitHttpClient()
	fetchSetting := system_setting.GetFetchSetting()
	original := *fetchSetting
	original.DomainList = append([]string(nil), fetchSetting.DomainList...)
	original.IpList = append([]string(nil), fetchSetting.IpList...)
	original.AllowedPorts = append([]string(nil), fetchSetting.AllowedPorts...)
	originalWorkerURL := system_setting.WorkerUrl
	fetchSetting.EnableSSRFProtection = true
	fetchSetting.AllowPrivateIp = allowPrivateIP
	fetchSetting.DomainFilterMode = false
	fetchSetting.IpFilterMode = false
	fetchSetting.AllowedPorts = nil
	fetchSetting.ApplyIPFilterForDomain = true
	system_setting.WorkerUrl = ""
	t.Cleanup(func() {
		*fetchSetting = original
		system_setting.WorkerUrl = originalWorkerURL
	})
}
