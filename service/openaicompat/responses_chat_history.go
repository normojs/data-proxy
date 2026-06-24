package openaicompat

import (
	"encoding/json"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const defaultResponsesChatHistoryLimit = 512

var defaultResponsesChatHistory = NewResponsesChatHistory(defaultResponsesChatHistoryLimit)

type ResponsesChatHistory struct {
	mu        sync.Mutex
	limit     int
	responses map[string]*cachedResponsesChatResponse
	order     []string
	callIndex map[string][]string
}

type cachedResponsesChatResponse struct {
	callsByID map[string]map[string]any
	callOrder []string
}

type ResponsesChatHistoryEnrichResult struct {
	Count   int
	Sources []string
}

func NewResponsesChatHistory(limit int) *ResponsesChatHistory {
	if limit <= 0 {
		limit = defaultResponsesChatHistoryLimit
	}
	return &ResponsesChatHistory{
		limit:     limit,
		responses: map[string]*cachedResponsesChatResponse{},
		callIndex: map[string][]string{},
	}
}

func DefaultResponsesChatHistory() *ResponsesChatHistory {
	return defaultResponsesChatHistory
}

func ResetDefaultResponsesChatHistoryForTest() {
	defaultResponsesChatHistory = NewResponsesChatHistory(defaultResponsesChatHistoryLimit)
}

func (h *ResponsesChatHistory) EnrichRequest(req *dto.OpenAIResponsesRequest) int {
	return h.EnrichRequestWithMeta(req).Count
}

func (h *ResponsesChatHistory) EnrichRequestWithMeta(req *dto.OpenAIResponsesRequest) ResponsesChatHistoryEnrichResult {
	if h == nil || req == nil || len(req.Input) == 0 || common.GetJsonType(req.Input) == "string" {
		return ResponsesChatHistoryEnrichResult{}
	}

	items, wasObject, ok := decodeResponsesInputItems(req.Input)
	if !ok || len(items) == 0 {
		return ResponsesChatHistoryEnrichResult{}
	}

	outputCallIDs := map[string]bool{}
	existingCallIDs := map[string]bool{}
	for _, item := range items {
		itemType := common.Interface2String(item["type"])
		callID := responseItemCallID(item)
		if callID == "" {
			continue
		}
		if isResponsesCallOutputItemType(itemType) {
			outputCallIDs[callID] = true
		}
		if isResponsesCallItemType(itemType) {
			existingCallIDs[callID] = true
		}
	}
	if len(outputCallIDs) == 0 && len(existingCallIDs) == 0 {
		return ResponsesChatHistoryEnrichResult{}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	lookupCached := func(callID string) (map[string]any, string) {
		if req.PreviousResponseID != "" {
			if previous := h.responses[req.PreviousResponseID]; previous != nil {
				if item := previous.callsByID[callID]; item != nil {
					return cloneMap(item), "previous_response_id"
				}
			}
		}
		if item := h.uniqueCallLocked(callID); item != nil {
			return cloneMap(item), "unique_call_id"
		}
		return nil, ""
	}

	changed := 0
	newItems := make([]map[string]any, 0, len(items)+len(outputCallIDs))
	restored := map[string]bool{}
	sourceSet := map[string]bool{}
	for _, item := range items {
		itemType := common.Interface2String(item["type"])
		callID := responseItemCallID(item)
		if callID != "" && isResponsesCallItemType(itemType) {
			if cached, source := lookupCached(callID); cached != nil && enrichResponsesCallItem(item, cached) {
				if source != "" {
					sourceSet[source] = true
				}
				changed++
			}
			existingCallIDs[callID] = true
		}
		if callID != "" && isResponsesCallOutputItemType(itemType) && !existingCallIDs[callID] && !restored[callID] {
			if cached, source := lookupCached(callID); cached != nil {
				newItems = append(newItems, cached)
				existingCallIDs[callID] = true
				restored[callID] = true
				if source != "" {
					sourceSet[source] = true
				}
				changed++
			}
		}
		newItems = append(newItems, item)
	}

	if changed == 0 {
		return ResponsesChatHistoryEnrichResult{}
	}
	req.Input = encodeResponsesInputItems(newItems, wasObject)
	return ResponsesChatHistoryEnrichResult{
		Count:   changed,
		Sources: historySourceList(sourceSet),
	}
}

func historySourceList(sourceSet map[string]bool) []string {
	sources := make([]string, 0, len(sourceSet))
	for source := range sourceSet {
		sources = append(sources, source)
	}
	return uniqueSortedStrings(sources)
}

func (h *ResponsesChatHistory) RecordResponseMap(response map[string]any) int {
	if h == nil || response == nil {
		return 0
	}
	responseID := common.Interface2String(response["id"])
	if responseID == "" {
		return 0
	}
	rawOutput := responseOutputItems(response["output"])
	if len(rawOutput) == 0 {
		return 0
	}
	calls := make([]map[string]any, 0)
	for _, item := range rawOutput {
		if !isResponsesCallItemType(common.Interface2String(item["type"])) || responseItemCallID(item) == "" {
			continue
		}
		calls = append(calls, cloneMap(item))
	}
	if len(calls) == 0 {
		return 0
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	return h.insertCallsLocked(responseID, calls)
}

func responseOutputItems(value any) []map[string]any {
	switch output := value.(type) {
	case []any:
		items := make([]map[string]any, 0, len(output))
		for _, rawItem := range output {
			item, ok := rawItem.(map[string]any)
			if ok {
				items = append(items, item)
			}
		}
		return items
	case []map[string]any:
		return output
	default:
		return nil
	}
}

func (h *ResponsesChatHistory) insertCallsLocked(responseID string, calls []map[string]any) int {
	if _, exists := h.responses[responseID]; !exists {
		h.order = append(h.order, responseID)
		h.responses[responseID] = &cachedResponsesChatResponse{
			callsByID: map[string]map[string]any{},
			callOrder: []string{},
		}
	}
	cached := h.responses[responseID]
	count := 0
	for _, item := range calls {
		callID := responseItemCallID(item)
		if callID == "" {
			continue
		}
		if _, exists := cached.callsByID[callID]; !exists {
			cached.callOrder = append(cached.callOrder, callID)
		}
		cached.callsByID[callID] = cloneMap(item)
		h.indexCallLocked(callID, responseID)
		count++
	}
	h.pruneLocked()
	return count
}

func (h *ResponsesChatHistory) indexCallLocked(callID string, responseID string) {
	for _, existing := range h.callIndex[callID] {
		if existing == responseID {
			return
		}
	}
	h.callIndex[callID] = append(h.callIndex[callID], responseID)
}

func (h *ResponsesChatHistory) pruneLocked() {
	for len(h.order) > h.limit {
		responseID := h.order[0]
		h.order = h.order[1:]
		delete(h.responses, responseID)
		for callID, responseIDs := range h.callIndex {
			filtered := responseIDs[:0]
			for _, id := range responseIDs {
				if id != responseID {
					filtered = append(filtered, id)
				}
			}
			if len(filtered) == 0 {
				delete(h.callIndex, callID)
			} else {
				h.callIndex[callID] = filtered
			}
		}
	}
}

func (h *ResponsesChatHistory) uniqueCallLocked(callID string) map[string]any {
	responseIDs := h.callIndex[callID]
	if len(responseIDs) != 1 {
		return nil
	}
	response := h.responses[responseIDs[0]]
	if response == nil {
		return nil
	}
	return response.callsByID[callID]
}

func decodeResponsesInputItems(raw json.RawMessage) ([]map[string]any, bool, bool) {
	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err == nil {
		return items, false, true
	}
	var single map[string]any
	if err := common.Unmarshal(raw, &single); err == nil {
		return []map[string]any{single}, true, true
	}
	return nil, false, false
}

func encodeResponsesInputItems(items []map[string]any, wasObject bool) json.RawMessage {
	if wasObject && len(items) == 1 {
		data, _ := common.Marshal(items[0])
		return data
	}
	data, _ := common.Marshal(items)
	return data
}

func isResponsesCallItemType(itemType string) bool {
	switch itemType {
	case "function_call", "custom_tool_call", "tool_search_call",
		"web_search_call", "file_search_call", "computer_call",
		"image_generation_call", "code_interpreter_call", "mcp_call":
		return true
	default:
		return false
	}
}

func isResponsesCallOutputItemType(itemType string) bool {
	switch itemType {
	case "function_call_output", "custom_tool_call_output", "tool_search_output":
		return true
	default:
		return false
	}
}

func responseItemCallID(item map[string]any) string {
	if item == nil {
		return ""
	}
	if callID := common.Interface2String(item["call_id"]); callID != "" {
		return callID
	}
	return common.Interface2String(item["id"])
}

func enrichResponsesCallItem(item map[string]any, cached map[string]any) bool {
	changed := false
	for _, key := range []string{"name", "namespace", "arguments", "input", "status", "reasoning_content", "reasoning", "action", "queries", "results", "code", "language", "tool_name", "pending_safety_checks"} {
		if !isEmptyResponsesHistoryValue(item[key]) {
			continue
		}
		value := cached[key]
		if isEmptyResponsesHistoryValue(value) {
			continue
		}
		item[key] = value
		changed = true
	}
	return changed
}

func isEmptyResponsesHistoryValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	default:
		return false
	}
}

func cloneMap(item map[string]any) map[string]any {
	if item == nil {
		return nil
	}
	data, err := common.Marshal(item)
	if err != nil {
		out := make(map[string]any, len(item))
		for key, value := range item {
			out[key] = value
		}
		return out
	}
	var out map[string]any
	if err := common.Unmarshal(data, &out); err != nil {
		out = make(map[string]any, len(item))
		for key, value := range item {
			out[key] = value
		}
	}
	return out
}
