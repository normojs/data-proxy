package service

import (
	"hash/fnv"
	"os"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

const (
	requestCaptureLevelEnv            = "CAPTURE_LEVEL"
	requestCaptureSampleRateEnv       = "CAPTURE_SAMPLE_RATE"
	requestCaptureModelPatternsEnv    = "CAPTURE_MODEL_PATTERNS"
	requestCapturePathPrefixesEnv     = "CAPTURE_PATH_PREFIXES"
	requestCaptureProtocolChainsEnv   = "CAPTURE_PROTOCOL_CHAINS"
	requestCaptureUserIdsEnv          = "CAPTURE_USER_IDS"
	requestCaptureTokenIdsEnv         = "CAPTURE_TOKEN_IDS"
	requestCaptureChannelIdsEnv       = "CAPTURE_CHANNEL_IDS"
	requestCaptureConnectedAppIdsEnv  = "CAPTURE_CONNECTED_APP_IDS"
	requestCaptureMaxArtifactBytesEnv = "CAPTURE_MAX_ARTIFACT_BYTES"
)

type RequestCapturePolicy struct {
	Enabled          bool
	Level            string
	SampleRate       float64
	ModelPatterns    []string
	PathPrefixes     []string
	ProtocolChains   []string
	UserIds          map[int]struct{}
	TokenIds         map[int]struct{}
	ChannelIds       map[int]struct{}
	ConnectedAppIds  map[int64]struct{}
	SpoolDir         string
	TmpDir           string
	MaxArtifactBytes int64
}

type RequestCaptureDecisionInput struct {
	RequestId      string
	UserId         int
	TokenId        int
	ChannelId      int
	ConnectedAppId int64
	ModelName      string
	RequestPath    string
	ProtocolChain  string
	IsStream       bool
}

type RequestCaptureDecision struct {
	Enabled bool
	Level   string
	Reason  string
}

func LoadRequestCapturePolicyFromEnv() RequestCapturePolicy {
	storage := LoadRequestCaptureStorageConfigFromEnv()
	level := strings.TrimSpace(os.Getenv(requestCaptureLevelEnv))
	if level == "" {
		level = model.RequestCaptureLevelMetadata
	}
	policy := RequestCapturePolicy{
		Enabled:          storage.Enabled && level != model.RequestCaptureLevelOff,
		Level:            level,
		SampleRate:       requestCaptureEnvFloat(requestCaptureSampleRateEnv, 1),
		ModelPatterns:    requestCaptureEnvCSV(requestCaptureModelPatternsEnv),
		PathPrefixes:     requestCaptureEnvCSV(requestCapturePathPrefixesEnv),
		ProtocolChains:   requestCaptureEnvCSV(requestCaptureProtocolChainsEnv),
		UserIds:          requestCaptureEnvIntSet(requestCaptureUserIdsEnv),
		TokenIds:         requestCaptureEnvIntSet(requestCaptureTokenIdsEnv),
		ChannelIds:       requestCaptureEnvIntSet(requestCaptureChannelIdsEnv),
		ConnectedAppIds:  requestCaptureEnvInt64Set(requestCaptureConnectedAppIdsEnv),
		SpoolDir:         storage.SpoolDir,
		TmpDir:           storage.TmpDir,
		MaxArtifactBytes: requestCaptureEnvInt64(requestCaptureMaxArtifactBytesEnv, 0),
	}
	if policy.SampleRate < 0 {
		policy.SampleRate = 0
	}
	if policy.SampleRate > 1 {
		policy.SampleRate = 1
	}
	return policy
}

func (p RequestCapturePolicy) Decide(input RequestCaptureDecisionInput) RequestCaptureDecision {
	if !p.Enabled {
		return RequestCaptureDecision{Reason: "disabled"}
	}
	if p.Level == "" || p.Level == model.RequestCaptureLevelOff {
		return RequestCaptureDecision{Reason: "level_off"}
	}
	if !requestCaptureSetMatches(p.UserIds, input.UserId) {
		return RequestCaptureDecision{Reason: "user_not_matched"}
	}
	if !requestCaptureSetMatches(p.TokenIds, input.TokenId) {
		return RequestCaptureDecision{Reason: "token_not_matched"}
	}
	if !requestCaptureSetMatches(p.ChannelIds, input.ChannelId) {
		return RequestCaptureDecision{Reason: "channel_not_matched"}
	}
	if !requestCaptureInt64SetMatches(p.ConnectedAppIds, input.ConnectedAppId) {
		return RequestCaptureDecision{Reason: "connected_app_not_matched"}
	}
	if !requestCaptureModelMatches(p.ModelPatterns, input.ModelName) {
		return RequestCaptureDecision{Reason: "model_not_matched"}
	}
	if !requestCapturePathMatches(p.PathPrefixes, input.RequestPath) {
		return RequestCaptureDecision{Reason: "path_not_matched"}
	}
	if !requestCaptureProtocolChainMatches(p.ProtocolChains, input.ProtocolChain) {
		return RequestCaptureDecision{Reason: "protocol_chain_not_matched"}
	}
	if !requestCaptureSampleMatches(input.RequestId, p.SampleRate) {
		return RequestCaptureDecision{Reason: "sample_not_matched"}
	}
	return RequestCaptureDecision{Enabled: true, Level: p.Level, Reason: "matched"}
}

func requestCaptureEnvFloat(name string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func requestCaptureEnvInt64(name string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func requestCaptureEnvCSV(name string) []string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func requestCaptureEnvIntSet(name string) map[int]struct{} {
	parts := requestCaptureEnvCSV(name)
	if len(parts) == 0 {
		return nil
	}
	result := map[int]struct{}{}
	for _, part := range parts {
		id, err := strconv.Atoi(part)
		if err == nil {
			result[id] = struct{}{}
		}
	}
	return result
}

func requestCaptureEnvInt64Set(name string) map[int64]struct{} {
	parts := requestCaptureEnvCSV(name)
	if len(parts) == 0 {
		return nil
	}
	result := map[int64]struct{}{}
	for _, part := range parts {
		id, err := strconv.ParseInt(part, 10, 64)
		if err == nil {
			result[id] = struct{}{}
		}
	}
	return result
}

func requestCaptureSetMatches(values map[int]struct{}, value int) bool {
	if len(values) == 0 {
		return true
	}
	_, ok := values[value]
	return ok
}

func requestCaptureInt64SetMatches(values map[int64]struct{}, value int64) bool {
	if len(values) == 0 {
		return true
	}
	_, ok := values[value]
	return ok
}

func requestCaptureModelMatches(patterns []string, modelName string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if requestCaptureWildcardMatch(strings.ToLower(pattern), strings.ToLower(modelName)) {
			return true
		}
	}
	return false
}

func requestCapturePathMatches(prefixes []string, requestPath string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, prefix := range prefixes {
		if prefix == "*" || strings.HasPrefix(requestPath, prefix) {
			return true
		}
	}
	return false
}

func requestCaptureProtocolChainMatches(patterns []string, protocolChain string) bool {
	if len(patterns) == 0 {
		return true
	}
	protocolChain = strings.TrimSpace(strings.ToLower(protocolChain))
	for _, pattern := range patterns {
		if requestCaptureWildcardMatch(strings.ToLower(pattern), protocolChain) {
			return true
		}
	}
	return false
}

func requestCaptureSampleMatches(requestId string, sampleRate float64) bool {
	if sampleRate >= 1 {
		return true
	}
	if sampleRate <= 0 {
		return false
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.TrimSpace(requestId)))
	bucket := float64(hash.Sum32()%10000) / 10000
	return bucket < sampleRate
}

func requestCaptureWildcardMatch(pattern string, value string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}
	parts := strings.Split(pattern, "*")
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(value[pos:], part)
		if idx < 0 {
			return false
		}
		if i == 0 && !strings.HasPrefix(pattern, "*") && idx != 0 {
			return false
		}
		pos += idx + len(part)
	}
	last := parts[len(parts)-1]
	if last != "" && !strings.HasSuffix(pattern, "*") && !strings.HasSuffix(value, last) {
		return false
	}
	return true
}
