package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

const (
	defaultStreamErrorMappingStatusCode      = http.StatusTooManyRequests
	defaultStreamErrorMappingCode            = "upstream_stream_mapped_error"
	defaultStreamErrorMappingMessage         = "upstream stream content matched an error mapping rule"
	defaultStreamErrorMappingPreFlushTimeout = 60 * time.Second
)

type streamErrorMappingMatch struct {
	Rule                    dto.StreamErrorMappingRule
	StatusCode              int
	ErrorCode               string
	Message                 string
	Retryable               bool
	ChannelFailureCandidate bool
}

type streamErrorMappingPreFlushConfig struct {
	Enabled   bool
	MaxChunks int
	Timeout   time.Duration
}

func matchStreamErrorMapping(data string, info *relaycommon.RelayInfo, chunkIndex int) (*streamErrorMappingMatch, bool) {
	if info == nil || len(info.ChannelOtherSettings.StreamErrorMapping) == 0 {
		return nil, false
	}

	for _, rule := range info.ChannelOtherSettings.StreamErrorMapping {
		if !streamErrorMappingRuleEnabled(rule) {
			continue
		}
		if rule.MaxChunks > 0 && chunkIndex > rule.MaxChunks {
			continue
		}
		if rule.MaxRawChars > 0 && utf8.RuneCountInString(data) > rule.MaxRawChars {
			continue
		}
		if strings.TrimSpace(rule.Pattern) == "" {
			continue
		}
		if streamErrorMappingRuleMatches(rule, data) {
			return buildStreamErrorMappingMatch(rule), true
		}
	}
	return nil, false
}

func streamErrorMappingPreFlushConfigFromInfo(info *relaycommon.RelayInfo) streamErrorMappingPreFlushConfig {
	if info == nil || len(info.ChannelOtherSettings.StreamErrorMapping) == 0 {
		return streamErrorMappingPreFlushConfig{}
	}

	maxChunks := 0
	timeoutMs := 0
	for _, rule := range info.ChannelOtherSettings.StreamErrorMapping {
		if !streamErrorMappingRuleEnabled(rule) || strings.TrimSpace(rule.Pattern) == "" || rule.PreFlushMaxChunks <= 0 {
			continue
		}
		if rule.PreFlushMaxChunks > maxChunks {
			maxChunks = rule.PreFlushMaxChunks
		}
		if rule.PreFlushTimeoutMs > timeoutMs {
			timeoutMs = rule.PreFlushTimeoutMs
		}
	}
	if maxChunks <= 0 {
		return streamErrorMappingPreFlushConfig{}
	}

	timeout := defaultStreamErrorMappingPreFlushTimeout
	if timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
	}
	return streamErrorMappingPreFlushConfig{
		Enabled:   true,
		MaxChunks: maxChunks,
		Timeout:   timeout,
	}
}

func streamErrorMappingRuleEnabled(rule dto.StreamErrorMappingRule) bool {
	return rule.Enabled == nil || *rule.Enabled
}

func streamErrorMappingRuleMatches(rule dto.StreamErrorMappingRule, data string) bool {
	candidates := streamErrorMappingCandidates(rule.Target, data)
	if len(candidates) == 0 {
		return false
	}
	for _, candidate := range candidates {
		if rule.MaxContentChars > 0 && utf8.RuneCountInString(candidate) > rule.MaxContentChars {
			continue
		}
		if matchStreamErrorMappingText(candidate, rule.Pattern, rule.Operator, rule.CaseSensitive) {
			return true
		}
	}
	return false
}

func streamErrorMappingCandidates(target string, data string) []string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "raw":
		return []string{data}
	case "text":
		return extractStreamTextCandidates(data)
	default:
		candidates := []string{data}
		candidates = append(candidates, extractStreamTextCandidates(data)...)
		return candidates
	}
}

func extractStreamTextCandidates(data string) []string {
	var parsed any
	if err := json.Unmarshal(common.StringToByteSlice(data), &parsed); err != nil {
		if strings.TrimSpace(data) != "" {
			return []string{data}
		}
		return nil
	}
	candidates := make([]string, 0, 8)
	collectStringValues(parsed, &candidates)
	return candidates
}

func collectStringValues(value any, candidates *[]string) {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			*candidates = append(*candidates, typed)
		}
	case []any:
		for _, item := range typed {
			collectStringValues(item, candidates)
		}
	case map[string]any:
		for _, item := range typed {
			collectStringValues(item, candidates)
		}
	}
}

func matchStreamErrorMappingText(candidate string, pattern string, operator string, caseSensitive bool) bool {
	normalizedOperator := strings.ToLower(strings.TrimSpace(operator))
	if normalizedOperator == "regex" {
		if !caseSensitive {
			pattern = "(?i)" + pattern
		}
		matched, err := regexp.MatchString(pattern, candidate)
		return err == nil && matched
	}

	if !caseSensitive {
		candidate = strings.ToLower(candidate)
		pattern = strings.ToLower(pattern)
	}

	switch normalizedOperator {
	case "", "contains":
		return strings.Contains(candidate, pattern)
	case "equals", "eq":
		return candidate == pattern
	case "prefix", "starts_with":
		return strings.HasPrefix(candidate, pattern)
	case "suffix", "ends_with":
		return strings.HasSuffix(candidate, pattern)
	default:
		return false
	}
}

func buildStreamErrorMappingMatch(rule dto.StreamErrorMappingRule) *streamErrorMappingMatch {
	statusCode := rule.StatusCode
	if statusCode < http.StatusContinue || statusCode > 599 {
		statusCode = defaultStreamErrorMappingStatusCode
	}

	errorCode := strings.TrimSpace(rule.ErrorCode)
	if errorCode == "" {
		errorCode = defaultStreamErrorMappingCode
	}

	message := strings.TrimSpace(rule.Message)
	if message == "" {
		message = defaultStreamErrorMappingMessage
	}

	retryable := true
	if rule.Retryable != nil {
		retryable = *rule.Retryable
	}

	channelFailureCandidate := true
	if rule.ChannelFailureCandidate != nil {
		channelFailureCandidate = *rule.ChannelFailureCandidate
	}

	return &streamErrorMappingMatch{
		Rule:                    rule,
		StatusCode:              statusCode,
		ErrorCode:               errorCode,
		Message:                 message,
		Retryable:               retryable,
		ChannelFailureCandidate: channelFailureCandidate,
	}
}

func (m *streamErrorMappingMatch) NewAPIError() *types.NewAPIError {
	if m == nil {
		return nil
	}
	openaiError := types.OpenAIError{
		Message: m.Message,
		Type:    "upstream_stream_error",
		Code:    m.ErrorCode,
	}
	options := make([]types.NewAPIErrorOptions, 0, 1)
	if !m.Retryable {
		options = append(options, types.ErrOptionWithSkipRetry())
	}
	return types.WithOpenAIError(openaiError, m.StatusCode, options...)
}

func (m *streamErrorMappingMatch) Error() error {
	if m == nil {
		return errors.New(defaultStreamErrorMappingMessage)
	}
	return fmt.Errorf("%s: %s", m.ErrorCode, m.Message)
}
