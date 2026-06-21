package service

import (
	"encoding/json"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const enterpriseGovernanceQueueRedactedValue = "[REDACTED]"

var enterpriseGovernanceQueueSensitiveTextPattern = regexp.MustCompile(`(?i)((?:"?(?:authorization|proxy[-_\s]?authorization|api[-_\s]?key|apikey|x[-_\s]?api[-_\s]?key|openai[-_\s]?api[-_\s]?key|anthropic[-_\s]?api[-_\s]?key|access[-_\s]?token|refresh[-_\s]?token|id[-_\s]?token|client[-_\s]?secret|webhook[-_\s]?secret|private[-_\s]?key|token|secret|password|passphrase|signature|sig|credentials?|credential)"?\s*[:=]\s*)|(?:\b(?:authorization|proxy[-_\s]?authorization|api[-_\s]?key|apikey|x[-_\s]?api[-_\s]?key|openai[-_\s]?api[-_\s]?key|anthropic[-_\s]?api[-_\s]?key|access[-_\s]?token|refresh[-_\s]?token|id[-_\s]?token|client[-_\s]?secret|webhook[-_\s]?secret|private[-_\s]?key|token|secret|password|passphrase|signature|sig|credentials?|credential)\b\s*[:=]\s*))("[^"]*"|'[^']*'|[^,\s}&]+)`)

func RedactEnterpriseGovernanceQueueAdmissionsForVisibility(rows []model.EnterpriseGovernanceQueueAdmission) []model.EnterpriseGovernanceQueueAdmission {
	if len(rows) == 0 {
		return rows
	}
	redacted := make([]model.EnterpriseGovernanceQueueAdmission, len(rows))
	for i, row := range rows {
		redacted[i] = RedactEnterpriseGovernanceQueueAdmissionForVisibility(row)
	}
	return redacted
}

func RedactEnterpriseGovernanceQueueAdmissionForVisibility(row model.EnterpriseGovernanceQueueAdmission) model.EnterpriseGovernanceQueueAdmission {
	row.RequestPayloadJson = RedactEnterpriseGovernanceQueueRequestPayloadJSONForVisibility(row.RequestPayloadJson)
	return row
}

func RedactEnterpriseGovernanceQueueRequestPayloadJSONForVisibility(payloadJson string) string {
	if strings.TrimSpace(payloadJson) == "" {
		return payloadJson
	}
	var payload EnterpriseGovernanceQueueRequestPayload
	if err := common.UnmarshalJsonStr(payloadJson, &payload); err != nil {
		return redactEnterpriseGovernanceQueueText(payloadJson)
	}
	payload = redactEnterpriseGovernanceQueueRequestPayloadForVisibility(payload)
	redacted, err := common.Marshal(payload)
	if err != nil {
		return redactEnterpriseGovernanceQueueText(payloadJson)
	}
	return string(redacted)
}

func RedactEnterpriseAuditLogsForVisibility(logs []model.EnterpriseAuditLog) []model.EnterpriseAuditLog {
	if len(logs) == 0 {
		return logs
	}
	redacted := make([]model.EnterpriseAuditLog, len(logs))
	for i, log := range logs {
		redacted[i] = RedactEnterpriseAuditLogForVisibility(log)
	}
	return redacted
}

func RedactEnterpriseAuditLogForVisibility(log model.EnterpriseAuditLog) model.EnterpriseAuditLog {
	log.BeforeJson = redactEnterpriseAuditJSONForVisibility(log.BeforeJson)
	log.AfterJson = redactEnterpriseAuditJSONForVisibility(log.AfterJson)
	return log
}

func redactEnterpriseGovernanceQueueRequestPayloadForVisibility(payload EnterpriseGovernanceQueueRequestPayload) EnterpriseGovernanceQueueRequestPayload {
	payload.RawQuery = redactEnterpriseGovernanceQueueRawQuery(payload.RawQuery)
	payload.Body = redactEnterpriseGovernanceQueueRequestBody(payload.ContentType, payload.Body)
	return payload
}

func redactEnterpriseGovernanceQueueRequestBody(contentType string, body string) string {
	if strings.TrimSpace(body) == "" {
		return body
	}
	mediaType, _ := enterpriseGovernanceQueueMediaType(contentType)
	switch {
	case mediaType == "application/x-www-form-urlencoded":
		return redactEnterpriseGovernanceQueueRawQuery(body)
	case mediaType == "application/x-ndjson":
		return redactEnterpriseGovernanceQueueNDJSONText(body)
	case mediaType == "application/json" || mediaType == "text/json" || strings.HasSuffix(mediaType, "+json"):
		return redactEnterpriseGovernanceQueueJSONText(body)
	default:
		if strings.HasPrefix(mediaType, "text/") || mediaType == "" {
			return redactEnterpriseGovernanceQueueText(body)
		}
		return body
	}
}

func redactEnterpriseGovernanceQueueJSONText(body string) string {
	value, ok := decodeEnterpriseGovernanceJSONValue(body)
	if !ok {
		return redactEnterpriseGovernanceQueueText(body)
	}
	redacted := redactEnterpriseGovernanceValue(value)
	payload, err := json.Marshal(redacted)
	if err != nil {
		return redactEnterpriseGovernanceQueueText(body)
	}
	return string(payload)
}

func redactEnterpriseGovernanceQueueNDJSONText(body string) string {
	lines := strings.SplitAfter(body, "\n")
	for i, line := range lines {
		suffix := ""
		content := line
		if strings.HasSuffix(content, "\n") {
			suffix = "\n"
			content = strings.TrimSuffix(content, "\n")
		}
		if strings.TrimSpace(content) == "" {
			continue
		}
		lines[i] = redactEnterpriseGovernanceQueueJSONText(content) + suffix
	}
	return strings.Join(lines, "")
}

func redactEnterpriseAuditJSONForVisibility(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	value, ok := decodeEnterpriseGovernanceJSONValue(raw)
	if !ok {
		return redactEnterpriseGovernanceQueueText(raw)
	}
	redacted := redactEnterpriseAuditValue(value)
	payload, err := json.Marshal(redacted)
	if err != nil {
		return redactEnterpriseGovernanceQueueText(raw)
	}
	return string(payload)
}

func decodeEnterpriseGovernanceJSONValue(raw string) (any, bool) {
	var value any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, false
	}
	return value, true
}

func redactEnterpriseAuditValue(value any) any {
	switch current := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(current))
		for key, item := range current {
			switch {
			case isEnterpriseGovernanceSensitiveKey(key):
				redacted[key] = enterpriseGovernanceQueueRedactedValue
			case strings.EqualFold(key, "request_payload_json"):
				if raw, ok := item.(string); ok {
					redacted[key] = RedactEnterpriseGovernanceQueueRequestPayloadJSONForVisibility(raw)
				} else {
					redacted[key] = redactEnterpriseAuditValue(item)
				}
			case strings.EqualFold(key, "raw_query"):
				if raw, ok := item.(string); ok {
					redacted[key] = redactEnterpriseGovernanceQueueRawQuery(raw)
				} else {
					redacted[key] = redactEnterpriseAuditValue(item)
				}
			default:
				redacted[key] = redactEnterpriseAuditValue(item)
			}
		}
		return redacted
	case []any:
		redacted := make([]any, len(current))
		for i, item := range current {
			redacted[i] = redactEnterpriseAuditValue(item)
		}
		return redacted
	default:
		return current
	}
}

func redactEnterpriseGovernanceValue(value any) any {
	switch current := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(current))
		for key, item := range current {
			if isEnterpriseGovernanceSensitiveKey(key) {
				redacted[key] = enterpriseGovernanceQueueRedactedValue
				continue
			}
			redacted[key] = redactEnterpriseGovernanceValue(item)
		}
		return redacted
	case []any:
		redacted := make([]any, len(current))
		for i, item := range current {
			redacted[i] = redactEnterpriseGovernanceValue(item)
		}
		return redacted
	default:
		return current
	}
}

func redactEnterpriseGovernanceQueueRawQuery(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	parts := strings.Split(raw, "&")
	changed := false
	for i, part := range parts {
		if part == "" {
			continue
		}
		keyPart := part
		if idx := strings.Index(part, "="); idx >= 0 {
			keyPart = part[:idx]
		}
		key, err := url.QueryUnescape(keyPart)
		if err != nil {
			key = keyPart
		}
		if !isEnterpriseGovernanceSensitiveKey(key) {
			continue
		}
		parts[i] = keyPart + "=" + enterpriseGovernanceQueueRedactedValue
		changed = true
	}
	if !changed {
		return raw
	}
	return strings.Join(parts, "&")
}

func redactEnterpriseGovernanceQueueText(value string) string {
	redacted := redactEnterpriseGovernanceQueueRawQuery(value)
	return enterpriseGovernanceQueueSensitiveTextPattern.ReplaceAllString(redacted, "${1}"+enterpriseGovernanceQueueRedactedValue)
}

func isEnterpriseGovernanceSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	normalized = strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(normalized)
	compact := strings.ReplaceAll(normalized, "_", "")
	switch normalized {
	case "authorization",
		"proxy_authorization",
		"api_key",
		"apikey",
		"key",
		"x_api_key",
		"openai_api_key",
		"anthropic_api_key",
		"token",
		"token_key",
		"access_token",
		"refresh_token",
		"id_token",
		"secret",
		"client_secret",
		"webhook_secret",
		"password",
		"passphrase",
		"signature",
		"sig",
		"credential",
		"credentials",
		"private_key":
		return true
	}
	switch compact {
	case "apikey",
		"xapikey",
		"openaiapikey",
		"anthropicapikey",
		"proxyauthorization",
		"tokenkey",
		"accesstoken",
		"refreshtoken",
		"idtoken",
		"clientsecret",
		"webhooksecret",
		"privatekey":
		return true
	default:
		return false
	}
}
