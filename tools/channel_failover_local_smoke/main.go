package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/router"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

const (
	smokeModel      = "gpt-4o-mini"
	adminUserID     = 1
	adminAccess     = "0123456789abcdef0123456789abcdef"
	apiTokenKey     = "localfailoversmokekey"
	badChannelID    = 101
	backupChannelID = 102
)

type smokeResult struct {
	BaseURL        string
	RequestID      string
	BadHits        int
	BackupHits     int
	EventSummary   string
	TraceTotal     int
	BundleSkipped  bool
	CandidateFound bool
}

type diagnosticCandidateItem struct {
	RequestID string `json:"request_id"`
	Source    string `json:"source"`
	ModelName string `json:"model_name"`
	ChannelID int    `json:"channel_id"`
	Summary   string `json:"summary"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "[data-proxy-local-failover-smoke] %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	gin.SetMode(gin.TestMode)
	restore, err := configureRuntime()
	if err != nil {
		return err
	}
	defer restore()

	badHits := 0
	badUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		badHits++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("OpenAI-Request-ID", "upstream-local-bad")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"local smoke synthetic 502","type":"server_error","code":"bad_gateway"}}`))
	}))
	defer badUpstream.Close()

	backupHits := 0
	backupUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backupHits++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("OpenAI-Request-ID", "upstream-local-backup")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-local-failover","object":"chat.completion","created":1710000000,"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"failover-pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	defer backupUpstream.Close()

	if err := initDatabase(); err != nil {
		return err
	}
	if err := seedData(badUpstream.URL, backupUpstream.URL); err != nil {
		return err
	}

	engine := gin.New()
	engine.Use(middleware.RequestId())
	engine.Use(middleware.I18n())
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte(common.SessionSecret))))
	router.SetApiRouter(engine)
	router.SetRelayRouter(engine)
	server := httptest.NewServer(engine)
	defer server.Close()

	requestID, err := sendChatProbe(server.URL)
	if err != nil {
		return err
	}
	traceTotal, eventSummary, err := validateTrace(server.URL, requestID)
	if err != nil {
		return err
	}
	candidateFound, err := validateCandidates(server.URL, requestID)
	if err != nil {
		return err
	}
	if badHits != 1 {
		return fmt.Errorf("expected bad upstream to be hit once, got %d", badHits)
	}
	if backupHits != 1 {
		return fmt.Errorf("expected backup upstream to be hit once, got %d", backupHits)
	}

	printSummary(smokeResult{
		BaseURL:        server.URL,
		RequestID:      requestID,
		BadHits:        badHits,
		BackupHits:     backupHits,
		EventSummary:   eventSummary,
		TraceTotal:     traceTotal,
		BundleSkipped:  true,
		CandidateFound: candidateFound,
	})
	return nil
}

func configureRuntime() (func(), error) {
	tempDir, err := os.MkdirTemp("", "data-proxy-local-failover-smoke.")
	if err != nil {
		return nil, err
	}
	originals := map[string]string{}
	for _, key := range []string{"SQL_DSN", "LOG_SQL_DSN", "REDIS_CONN_STRING", "SQLITE_PATH"} {
		originals[key] = os.Getenv(key)
	}

	originalSQLitePath := common.SQLitePath
	originalIsMaster := common.IsMasterNode
	originalRetryTimes := common.RetryTimes
	originalMemoryCache := common.MemoryCacheEnabled
	originalRedisEnabled := common.RedisEnabled
	originalRDB := common.RDB
	originalAutomaticDisable := common.AutomaticDisableChannelEnabled
	originalHealthFailureThreshold := common.ChannelHealthFailureThreshold
	originalLogConsume := common.LogConsumeEnabled
	originalDataExport := common.DataExportEnabled
	originalErrorLog := constant.ErrorLogEnabled
	originalCountToken := constant.CountToken
	originalSelfUseMode := operation_setting.SelfUseModeEnabled
	originalRetryRanges := append([]operation_setting.StatusCodeRange(nil), operation_setting.AutomaticRetryStatusCodeRanges...)
	originalTransientRanges := append([]operation_setting.StatusCodeRange(nil), operation_setting.ChannelHealthTransientStatusCodeRanges...)
	originalDisableRanges := append([]operation_setting.StatusCodeRange(nil), operation_setting.AutomaticDisableStatusCodeRanges...)

	dbPath := filepath.Join(tempDir, "local-failover-smoke.db")
	common.SQLitePath = dbPath
	common.IsMasterNode = true
	common.RetryTimes = 1
	common.MemoryCacheEnabled = true
	common.RedisEnabled = false
	common.RDB = nil
	common.AutomaticDisableChannelEnabled = true
	common.ChannelHealthFailureThreshold = 3
	common.LogConsumeEnabled = true
	common.DataExportEnabled = false
	constant.ErrorLogEnabled = true
	constant.CountToken = false
	operation_setting.SelfUseModeEnabled = true
	operation_setting.AutomaticRetryStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 429, End: 429}, {Start: 500, End: 599}}
	operation_setting.ChannelHealthTransientStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 429, End: 429}, {Start: 500, End: 599}}
	operation_setting.AutomaticDisableStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 401, End: 401}}
	service.InitHttpClient()

	_ = os.Setenv("SQL_DSN", "local")
	_ = os.Setenv("SQLITE_PATH", dbPath)
	_ = os.Unsetenv("LOG_SQL_DSN")
	_ = os.Unsetenv("REDIS_CONN_STRING")

	restore := func() {
		common.SQLitePath = originalSQLitePath
		common.IsMasterNode = originalIsMaster
		common.RetryTimes = originalRetryTimes
		common.MemoryCacheEnabled = originalMemoryCache
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRDB
		common.AutomaticDisableChannelEnabled = originalAutomaticDisable
		common.ChannelHealthFailureThreshold = originalHealthFailureThreshold
		common.LogConsumeEnabled = originalLogConsume
		common.DataExportEnabled = originalDataExport
		constant.ErrorLogEnabled = originalErrorLog
		constant.CountToken = originalCountToken
		operation_setting.SelfUseModeEnabled = originalSelfUseMode
		operation_setting.AutomaticRetryStatusCodeRanges = originalRetryRanges
		operation_setting.ChannelHealthTransientStatusCodeRanges = originalTransientRanges
		operation_setting.AutomaticDisableStatusCodeRanges = originalDisableRanges
		for key, value := range originals {
			if value == "" {
				_ = os.Unsetenv(key)
			} else {
				_ = os.Setenv(key, value)
			}
		}
		service.ClearChannelTemporaryHealth(badChannelID)
		service.ClearChannelTemporaryHealth(backupChannelID)
		_ = os.RemoveAll(tempDir)
	}
	return restore, nil
}

func initDatabase() error {
	if err := model.InitDB(); err != nil {
		return fmt.Errorf("init database: %w", err)
	}
	if err := model.InitLogDB(); err != nil {
		return fmt.Errorf("init log database: %w", err)
	}
	return nil
}

func seedData(badURL, backupURL string) error {
	adminSetting, err := common.Marshal(dto.UserSetting{BillingPreference: "wallet_only"})
	if err != nil {
		return err
	}
	access := adminAccess
	user := model.User{
		Id:          adminUserID,
		Username:    "local-failover-admin",
		Password:    "local-password",
		DisplayName: "Local Failover Admin",
		Role:        common.RoleRootUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AccessToken: &access,
		Quota:       100000000,
		Setting:     string(adminSetting),
	}
	if err := model.DB.Create(&user).Error; err != nil {
		return fmt.Errorf("create smoke user: %w", err)
	}
	token := model.Token{
		Id:             1,
		UserId:         adminUserID,
		Key:            apiTokenKey,
		Status:         common.TokenStatusEnabled,
		Name:           "local-failover-token",
		ExpiredTime:    -1,
		RemainQuota:    100000000,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := model.DB.Create(&token).Error; err != nil {
		return fmt.Errorf("create smoke token: %w", err)
	}

	if err := seedChannel(badChannelID, "local-bad-channel", badURL, 100); err != nil {
		return err
	}
	if err := seedChannel(backupChannelID, "local-backup-channel", backupURL, 90); err != nil {
		return err
	}
	model.InitChannelCache()
	return nil
}

func seedChannel(id int, name string, baseURL string, priority int64) error {
	autoBan := 1
	weight := uint(100)
	channel := model.Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Key:      fmt.Sprintf("sk-local-upstream-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     name,
		Weight:   &weight,
		BaseURL:  &baseURL,
		Models:   smokeModel,
		Group:    "default",
		Priority: &priority,
		AutoBan:  &autoBan,
	}
	if err := model.DB.Create(&channel).Error; err != nil {
		return fmt.Errorf("create channel %d: %w", id, err)
	}
	ability := model.Ability{
		Group:     "default",
		Model:     smokeModel,
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}
	if err := model.DB.Create(&ability).Error; err != nil {
		return fmt.Errorf("create ability for channel %d: %w", id, err)
	}
	return nil
}

func sendChatProbe(baseURL string) (string, error) {
	payload := []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Reply with the single word failover-pong."}],"max_tokens":16,"stream":false}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer sk-"+apiTokenKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat probe failed: status=%d body=%s", resp.StatusCode, safePreview(body))
	}
	if !bytes.Contains(body, []byte("failover-pong")) {
		return "", fmt.Errorf("chat probe did not return backup response: body=%s", safePreview(body))
	}
	requestID := strings.TrimSpace(resp.Header.Get(common.RequestIdKey))
	if requestID == "" {
		return "", errors.New("chat probe did not return request id header")
	}
	return requestID, nil
}

func validateTrace(baseURL, requestID string) (int, string, error) {
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Total       int                    `json:"total"`
			Diagnostics map[string]interface{} `json:"diagnostics"`
			Summary     struct {
				TypeCounts map[string]int `json:"type_counts"`
			} `json:"summary"`
			Logs []struct {
				Other map[string]interface{} `json:"other"`
			} `json:"logs"`
		} `json:"data"`
	}
	if err := callAdminJSON(baseURL+"/api/log/request/"+requestID, http.MethodGet, nil, &envelope); err != nil {
		return 0, "", err
	}
	if !envelope.Success {
		return 0, "", errors.New("trace API returned success=false")
	}
	if envelope.Data.Total == 0 {
		return 0, "", errors.New("trace API returned no logs")
	}
	if envelope.Data.Summary.TypeCounts["consume"] == 0 {
		return 0, "", errors.New("trace API did not include a consume log")
	}

	events := collectFailoverEvents(envelope.Data.Diagnostics, envelope.Data.Logs)
	if len(events) == 0 {
		return 0, "", errors.New("trace has no channel_failover events")
	}
	if !hasFailedRetryEvent(events, badChannelID, http.StatusBadGateway) {
		return 0, "", fmt.Errorf("trace has no failed retry event for channel %d status %d", badChannelID, http.StatusBadGateway)
	}
	if !hasRetrySelectedEvent(events, backupChannelID) {
		return 0, "", fmt.Errorf("trace has no retry selected event for backup channel %d", backupChannelID)
	}
	return envelope.Data.Total, summarizeEvents(events), nil
}

func validateCandidates(baseURL, requestID string) (bool, error) {
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Items []diagnosticCandidateItem `json:"items"`
		} `json:"data"`
	}
	url := baseURL + "/api/log/request-diagnostic-candidates?limit=20&source=failover"
	if err := callAdminJSON(url, http.MethodGet, nil, &envelope); err != nil {
		return false, err
	}
	if !envelope.Success {
		return false, errors.New("diagnostic candidates API returned success=false")
	}
	for _, item := range envelope.Data.Items {
		if item.RequestID == requestID && candidateHasSource(item.Source, "channel_failover") {
			return true, nil
		}
	}
	return false, fmt.Errorf("diagnostic candidates did not include request id %s; candidates=%s", requestID, summarizeCandidates(envelope.Data.Items))
}

func callAdminJSON(url string, method string, body io.Reader, out any) error {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminAccess)
	req.Header.Set("New-Api-User", strconv.Itoa(adminUserID))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s failed: status=%d body=%s", method, url, resp.StatusCode, safePreview(raw))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode %s: %w; body=%s", url, err, safePreview(raw))
	}
	return nil
}

func collectFailoverEvents(diagnostics map[string]interface{}, logs []struct {
	Other map[string]interface{} `json:"other"`
}) []map[string]interface{} {
	events := make([]map[string]interface{}, 0)
	events = append(events, eventsFromOther(diagnostics)...)
	for _, log := range logs {
		events = append(events, eventsFromOther(log.Other)...)
	}
	return events
}

func eventsFromOther(other map[string]interface{}) []map[string]interface{} {
	adminInfo, _ := other["admin_info"].(map[string]interface{})
	rawEvents, _ := adminInfo["channel_failover"].([]interface{})
	events := make([]map[string]interface{}, 0, len(rawEvents))
	for _, raw := range rawEvents {
		event, ok := raw.(map[string]interface{})
		if ok {
			events = append(events, event)
		}
	}
	return events
}

func hasFailedRetryEvent(events []map[string]interface{}, channelID int, status int) bool {
	for _, event := range events {
		if eventString(event, "event") != "failed" {
			continue
		}
		if eventInt(event, "channel_id") != channelID {
			continue
		}
		if eventInt(event, "status_code") != status {
			continue
		}
		if planned, ok := event["retry_planned"].(bool); !ok || !planned {
			continue
		}
		return true
	}
	return false
}

func hasRetrySelectedEvent(events []map[string]interface{}, channelID int) bool {
	for _, event := range events {
		if eventString(event, "event") != "selected" {
			continue
		}
		if eventInt(event, "channel_id") != channelID {
			continue
		}
		if eventInt(event, "retry_index") <= 0 {
			continue
		}
		return true
	}
	return false
}

func summarizeEvents(events []map[string]interface{}) string {
	parts := make([]string, 0, len(events))
	seen := map[string]bool{}
	for _, event := range events {
		part := fmt.Sprintf("%s:channel=%d,retry=%d,planned=%v,status=%d",
			eventString(event, "event"),
			eventInt(event, "channel_id"),
			eventInt(event, "retry_index"),
			event["retry_planned"],
			eventInt(event, "status_code"),
		)
		if seen[part] {
			continue
		}
		seen[part] = true
		parts = append(parts, part)
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func summarizeCandidates(items []diagnosticCandidateItem) string {
	if len(items) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("%s/source=%s/model=%s/channel=%d/summary=%s",
			item.RequestID,
			item.Source,
			item.ModelName,
			item.ChannelID,
			strings.ReplaceAll(item.Summary, "|", "/"),
		))
	}
	return strings.Join(parts, "; ")
}

func candidateHasSource(raw string, expected string) bool {
	for _, source := range strings.Split(raw, ",") {
		if strings.TrimSpace(source) == expected {
			return true
		}
	}
	return false
}

func eventString(event map[string]interface{}, key string) string {
	value, _ := event[key].(string)
	return value
}

func eventInt(event map[string]interface{}, key string) int {
	switch value := event[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		v, _ := value.Int64()
		return int(v)
	default:
		return 0
	}
}

func printSummary(result smokeResult) {
	fmt.Println("# Data Proxy Local Channel Failover Smoke")
	fmt.Println()
	fmt.Println("| Field | Value |")
	fmt.Println("| --- | --- |")
	fmt.Printf("| base_url | `%s` |\n", result.BaseURL)
	fmt.Printf("| model | `%s` |\n", smokeModel)
	fmt.Printf("| request_id | `%s` |\n", result.RequestID)
	fmt.Printf("| bad_channel_id | `%d` |\n", badChannelID)
	fmt.Printf("| backup_channel_id | `%d` |\n", backupChannelID)
	fmt.Printf("| bad_upstream_hits | `%d` |\n", result.BadHits)
	fmt.Printf("| backup_upstream_hits | `%d` |\n", result.BackupHits)
	fmt.Printf("| trace_total | `%d` |\n", result.TraceTotal)
	fmt.Printf("| diagnostic_candidate | `%t` |\n", result.CandidateFound)
	fmt.Printf("| failover_events | `%s` |\n", strings.ReplaceAll(result.EventSummary, "|", "\\|"))
	fmt.Printf("| completed_at_utc | `%s` |\n", time.Now().UTC().Format(time.RFC3339))
}

func safePreview(body []byte) string {
	text := strings.ReplaceAll(string(body), "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	if len(text) > 500 {
		return text[:500] + "..."
	}
	return text
}
