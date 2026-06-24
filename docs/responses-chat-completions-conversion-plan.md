# Responses <-> Chat Completions Conversion Plan

Date: 2026-06-22

This project is based on `new-api`, so all implementation must remain compatible with the upstream license model and keep third-party license notices when code is copied or adapted. The preferred approach is to learn from mature open-source implementations, then implement project-native code inside data-proxy instead of importing a proxy-specific dependency wholesale.

## Reference Projects

### copilot2api

Repository: https://github.com/whtsky/copilot2api

License: MIT

Local revision inspected: `7368d6b` (`chore: release v0.3.1`, 2026-04-26)

Useful files:

- `proxy/convert.go`
- `proxy/stream.go`
- `proxy/convert_test.go`
- `proxy/smart_routing_test.go`

Key ideas to borrow:

- Keep four conversion directions as explicit functions:
  - Chat request -> Responses request
  - Responses request -> Chat request
  - Responses result/events -> Chat response/chunks
  - Chat response/chunks -> Responses result/events
- Keep streaming conversion state outside HTTP handlers.
- Emit complete Responses streaming lifecycle for function calls:
  `response.output_item.added`, `response.function_call_arguments.delta`, `response.function_call_arguments.done`, `response.output_item.done`, then terminal event.
- Defer terminal stream event when usage may arrive in a later usage-only Chat chunk.
- Unit-test edge cases for `tool_choice`, `previous_response_id`, reasoning, response format, usage, and terminal events.

Limitations for us:

- It is a lightweight proxy and does not know data-proxy channel policy, quota, request logs, or new-api relay context.
- Its `previous_response_id` handling is stateless and intentionally ignored in Responses -> Chat conversion, which is not enough for Codex-style tool follow-up turns.

### CLIProxyAPI

Repository: https://github.com/router-for-me/CLIProxyAPI

License: MIT

Local revision inspected: `1f2504e` (`fix(claude): bypass signature sanitizer for non-Claude models (#3946)`, 2026-06-21)

Useful files:

- `sdk/translator/pipeline.go`
- `internal/translator/openai/openai/responses/openai_openai-responses_request.go`
- `internal/translator/openai/openai/responses/openai_openai-responses_response.go`
- `internal/translator/openai/openai/responses/openai_openai-responses_tools.go`

Key ideas to borrow:

- Registry/pipeline shape: relay code calls a translator, translator owns protocol semantics.
- Use original request JSON during response conversion so the final Responses object can preserve request fields such as `previous_response_id`, `tools`, `tool_choice`, `reasoning`, `metadata`, and `service_tier`.
- Handle namespace tools by flattening names for Chat and restoring namespace/name in Responses output.
- Track stream state for text, reasoning, function calls, output indexes, usage, and terminal events.
- Build deterministic final `response.output` ordered by output index.

Limitations for us:

- Important translator packages are under `internal`, so they are not directly importable outside the module.
- It is a multi-protocol proxy with its own SDK abstractions; copying the architecture directly would be too large for our current codebase.

### CC Switch

Repository: https://github.com/farion1231/cc-switch

License: MIT

Local revision inspected: `895d7af` (`docs(readme): add Kimi sponsor call-to-action link`, 2026-06-22)

Useful files:

- `src-tauri/src/proxy/providers/transform_codex_chat.rs`
- `src-tauri/src/proxy/providers/streaming_codex_chat.rs`
- `src-tauri/src/proxy/providers/codex_chat_history.rs`
- `src-tauri/src/proxy/providers/codex_chat_common.rs`
- `docs/release-notes/v3.16.0-zh.md`
- `docs/release-notes/v3.16.1-zh.md`
- `docs/release-notes/v3.16.2-zh.md`

Key ideas to borrow:

- Treat Codex Responses -> Chat Completions routing as a first-class compatibility mode.
- Maintain a bounded cross-turn history cache keyed by response id and call id. When a later request only carries `previous_response_id + function_call_output`, restore the missing previous `function_call` item before converting to Chat messages.
- Fall back by unique `call_id` when `previous_response_id` is missing or rewritten; do not restore ambiguous call ids.
- Preserve or backfill `reasoning_content` for assistant messages with `tool_calls`, because DeepSeek, Kimi, and Moonshot-style thinking models may reject tool calls without reasoning.
- Support function, namespace, custom, and `tool_search` tools, then map Chat upstream tool calls back to the original Responses item type.
- Detect inline `<think>...</think>` blocks in Chat deltas and expose them as Responses reasoning summary events.
- Inject `stream_options.include_usage` for Chat streaming so usage arrives and billing is accurate.
- Drop `tool_choice` and `parallel_tool_calls` if final Chat tools are empty, avoiding strict upstream errors.
- Sniff mislabeled SSE bodies on non-streaming routes and aggregate them before non-stream conversion.

Limitations for us:

- It is a desktop/local proxy in Rust, not a Go server package.
- Some behavior is Codex-specific and should be guarded behind data-proxy channel/model capabilities rather than applied globally.

## Current Data-Proxy State

Relevant local files:

- `relay/responses_via_chat.go`
- `relay/chat_completions_via_responses.go`
- `service/openaicompat/responses_to_chat_request.go`
- `service/openaicompat/chat_to_responses.go`
- `service/openaicompat/chat_to_responses_response.go`
- `service/openaicompat/policy.go`

Already implemented:

- Channel-level `responses_protocol` policy: `auto`, `native`, `chat_completions`, `disabled`.
- Basic Responses -> Chat request conversion for Chat-only providers such as SiliconFlow and DeepSeek.
- Basic Chat -> Responses request conversion for channels that should use native Responses upstream.

## Progress on 2026-06-22

Implemented in the project-native Go converter:

- Added flexible reasoning extraction for `reasoning_content`, `reasoning`, and `reasoning_details` when providers return strings, objects, arrays, or JSON-encoded strings.
- Added Chat stream -> Responses stream state for inline `<think>...</think>` content, including split chunks such as `<thi`, `nk>...`, and `</think>...`.
- Changed streaming tool-call conversion to wait for a stable real tool `id` and `name` before emitting `response.output_item.added`, so argument chunks that arrive before metadata do not become a bogus `tool` call or mismatch final IDs.
- Added bounded Responses chat history restore for `previous_response_id + function_call_output` follow-up turns.
- Added strict Responses -> Chat tool-call adjacency:
  - buffer consecutive `function_call` / `custom_tool_call` / `tool_search_call` items into one assistant message
  - defer regular messages until expected tool outputs are emitted
  - keep `assistant(tool_calls)` immediately followed by matching `tool` messages when the input contains outputs
- Added JSON canonicalization for parseable tool arguments and tool outputs while preserving plain text values.
- Added namespace tool flattening with a 64-character cap and hash suffix, while keeping `ResponsesToChatContext` able to restore the original `namespace` and child `name`.
- Added non-stream Responses SSE body aggregation fallback when a route expected JSON but received SSE-like body content.
- Preserved Responses `reasoning` output summaries when converting native non-stream Responses results back to Chat Completions.
- Preserved mixed assistant text + tool calls in both non-stream directions:
  - Chat response -> Responses output emits a message item and function call items.
  - Responses output -> Chat response keeps assistant `content` and `tool_calls` on the same message.

Open-source reference status:

- `copilot2api/proxy` is a useful lightweight Go reference for conversion function boundaries and tests.
- `CLIProxyAPI` is a useful Go reference for translator/pipeline shape, strict tool adjacency, namespace tool flatten/restore, stream indexes, and final output ordering.
- `cc-switch` remains a useful mature behavior reference for history restore, reasoning backfill, and Codex-like client compatibility.
- No external package has been imported. Current code reimplements the behavior locally to fit data-proxy/new-api relay, quota, channel policy, and logging boundaries.
- Basic non-stream Chat response -> Responses response conversion.
- Streaming Chat -> Responses text conversion.
- Streaming Chat -> Responses function call lifecycle was recently added for split `tool_calls` chunks.
- Usage log now records request id and request conversion chain.

Current remaining gaps:

- Hosted Responses tools are filtered on the Chat-only conversion path by
  default. data-proxy logs `hosted_tools_filtered` and injects a direct-answer
  hint, but true hosted-tool execution still depends on a native Responses
  upstream or a future data-proxy executor bridge.
- Some domestic thinking models need non-empty `reasoning_content` on assistant tool-call messages.
- The fixture harness is in place, but more golden fixtures from real
  Codex/domestic-model traffic are still needed, especially provider-specific
  streaming quirks and production error bodies.

## Implementation Update 2026-06-22

Implemented in data-proxy:

- Moved Chat stream -> Responses stream state into `service/openaicompat/ChatToResponsesStreamConverter`; relay is now scan/write glue.
- Added bounded in-memory Responses/Chat history cache for `previous_response_id + function_call_output` follow-up turns, including unique `call_id` fallback.
- Recorded non-stream and stream Chat -> Responses tool calls into the history cache.
- Added deterministic output indexes and final `response.output` ordering for reasoning, text, and tool call items.
- Added reasoning summary stream lifecycle events and visible text fallback when a domestic Chat model emits only `reasoning_content`.
- Preserved/backfilled `reasoning_content` on assistant tool-call messages for strict thinking models.
- Completed stream handling for function calls and added custom tool call input `done` events.
- Preserved function, namespace, custom, and `tool_search` mappings across request and response conversion.
- Replaced hosted-tool functionization with safe filtering for `web_search`,
  `web_search_preview`, `file_search`, `computer`, `computer_use_preview`,
  `image_generation`, `code_interpreter`, and hosted `mcp`. Filtered hosted
  tools are logged under `hosted_tools_filtered`, and data-proxy injects a
  direct-answer hint marked by `hosted_tools_direct_answer_hint`.
- Mapped Chat `finish_reason=length` to Responses `status=incomplete` with
  `incomplete_details.reason=max_output_tokens` for non-stream and stream
  conversions. Stream end without `finish_reason` now becomes incomplete when
  output exists and `response.failed` when no substantive output exists.
- Added `response.custom_tool_call_input.delta` before
  `response.custom_tool_call_input.done` for streamed custom tool calls.
- Added non-stream `responsesViaChat` fallback that aggregates mislabeled Chat
  Completions SSE bodies into final Responses JSON.
- Added channel-level `responses_reasoning_adapter` mapping for OpenAI-style
  `reasoning_effort`, DeepSeek `thinking` + effort, OpenRouter
  `reasoning.effort`, Qwen `enable_thinking`, MiniMax `reasoning_split`,
  low/high effort adapters, and explicit `none/off/disabled` handling.
- Added regression tests for usage-only chunks, reasoning fallback, custom tool streaming, and history restoration.
- Preserved Chat assistant `reasoning_content` when converting Chat history into Responses input items.
- Added a non-stream hardening path that aggregates mislabeled Responses SSE bodies before converting them back to Chat responses.
- Added structured request-log metadata under `other.request_conversion_meta`,
  including final upstream protocol, Responses protocol decision, reasoning
  adapter/params, hosted tools filtered from Chat conversion, history
  restore/record counts, Chat SSE fallback, and terminal incomplete/failed
  status details.
- Moved native Responses stream -> Chat Completions chunk conversion into
  `service/openaicompat/ResponsesToChatStreamConverter`; the OpenAI adaptor now
  only scans upstream SSE, sends returned Chat chunks, and handles relay-specific
  formatting/billing glue.
- Added directory-based golden fixture tests under
  `service/openaicompat/testdata/responses_to_chat/` for hosted `web_search`
  filtering with DeepSeek reasoning, `tool_search_output` namespace tool
  loading, and `previous_response_id` history restoration.
- Added compact channel-form capability hints for `Responses Protocol` and
  `Responses 推理适配`, based on channel type, so admins can see whether Auto is
  expected to stay native or convert to Chat Completions.

## Implementation Update 2026-06-23

Additional observability and fixture coverage:

- Added request-id trace API and Usage Logs detail UI integration so operators
  can inspect `summary`, `diagnostics`, conversion metadata, stream status, and
  related consume/error logs from a single request id.
- Added Usage Logs table shortcuts that open request trace details or filter
  Common logs by the selected request id without losing the copy-to-clipboard
  behavior.
- Added [Request ID Trace Troubleshooting](./request-trace-troubleshooting.md)
  and linked it from the README.
- Added provider-shaped golden fixtures for OpenRouter-style
  `reasoning.effort`, Qwen `enable_thinking` plus namespace tools, explicit
  Moonshot low/high reasoning, and Moonshot/Kimi-style low/high reasoning effort
  with restored tool output context.
- Hardened non-stream Responses SSE fallback to preserve `response.failed`
  terminal events.
- Added SSE-body regression coverage for failed Responses SSE, provider error
  JSON/SSE bodies, Chat Completions empty-stop streams, and domestic
  Chat-Completions tool-call streams with reasoning deltas.
- Added stream-converter coverage for OpenRouter/Kimi-style
  `reasoning_details` arrays so provider reasoning still becomes Responses
  reasoning summary output.

Hosted tool behavior:

- cc-switch's core converter only preserves tools that can be represented in
  Chat (`function`, `custom`, `tool_search`, `namespace`) and ignores unknown
  hosted tools in the pure Chat conversion path.
- data-proxy now follows the same safety boundary and makes the behavior
  observable: hosted Responses tool declarations are filtered, logged under
  `hosted_tools_filtered`, and paired with a direct-answer hint marked by
  `hosted_tools_direct_answer_hint`.
- This conversion does not by itself perform external search, file search,
  computer control, image generation, code execution, or MCP calls. Those
  operations must still be carried by the client, by a native Responses upstream,
  or by a separate provider/runtime integration.
- More golden fixtures from real Codex traces should be added as production traffic exposes edge cases.

## Target Architecture

Keep relay code responsible for HTTP/channel concerns only:

- validate request
- choose channel and protocol mode
- apply disabled fields / param overrides
- call upstream
- write response
- record billing/logging

Move protocol semantics into `service/openaicompat`:

- request conversion
- non-stream response conversion
- stream event conversion state machines
- tool context and namespace mapping
- history restoration
- unsupported-feature warnings

Proposed internal structure:

- `service/openaicompat/types.go`
  - shared conversion context, warning types, converter capabilities
- `service/openaicompat/tools.go`
  - function/custom/tool_search/namespace tool mapping
- `service/openaicompat/responses_to_chat_request.go`
  - keep and extend current request conversion
- `service/openaicompat/chat_to_responses_request.go`
  - split from current `chat_to_responses.go`
- `service/openaicompat/chat_to_responses_response.go`
  - extend non-stream mapping
- `service/openaicompat/chat_to_responses_stream.go`
  - move stream state from `relay/responses_via_chat.go`
- `service/openaicompat/responses_to_chat_stream.go`
  - normalize native Responses SSE -> Chat chunks where needed
- `service/openaicompat/history.go`
  - bounded previous-response/tool-call cache
- `service/openaicompat/fixtures_test.go`
  - golden fixtures from real Codex and domestic-model traces

Relay integration:

- `relay/responses_via_chat.go` constructs a converter, then only scans upstream Chat SSE and writes returned Responses SSE events.
- `relay/channel/openai/chat_via_responses.go` uses
  `ResponsesToChatStreamConverter`, then only scans native Responses SSE and
  writes returned Chat chunks.
- `relay/chat_completions_via_responses.go` can keep using native OpenAI Responses handlers, while sharing conversion helpers and fixtures where possible.
- Logs now record final upstream protocol, protocol decision, inferred channel
  capability, recommended reasoning adapter, actual reasoning adapter details,
  hosted-tool functionization, unsupported tool filtering, history restore
  sources/counts, history record counts, fallback use, and terminal status
  mapping. Future log work should make these hints
  model-specific when channel capability modeling becomes more detailed.

## Development Phases

### Phase 0: Baseline and Fixture Capture

Goal: freeze current behavior and create a repeatable compatibility suite before deeper refactoring.

Tasks:

- Add golden fixtures for:
  - text-only Chat stream -> Responses stream
  - split function-call args -> Responses stream
  - tool call followed by `function_call_output`
  - usage-only final Chat stream chunk
  - request id `202606221224363261214478268d9d6PRNQoZqL` style flow
- Store sanitized fixtures under `service/openaicompat/testdata/`.
- Add assertions for event order, output indexes, final `response.output`, usage, and request conversion logs.

Acceptance:

- `go test ./service/openaicompat ./relay -count=1` passes.
- Existing production behavior remains unchanged.

### Phase 1: Extract Chat -> Responses Stream State Machine

Status: completed.

Goal: move the stream conversion logic out of `relay/responses_via_chat.go`.

Tasks:

- Create `ChatToResponsesStreamConverter`.
- Input: Chat SSE payloads or parsed `dto.ChatCompletionsStreamResponse`.
- Output: typed Responses SSE events or JSON-ready payloads.
- Preserve current behavior for text and function calls.
- Add:
  - `response.in_progress`
  - optional `sequence_number`
  - deterministic output indexes
  - completion deferral for usage-only chunks
  - incomplete mapping for `finish_reason=length`

Acceptance:

- Relay handler becomes mostly scan/write glue.
- Existing tool-call lifecycle test still passes.
- New tests cover usage-only chunk and `[DONE]` without usage.

### Phase 2: Responses -> Chat Request Compatibility Hardening

Status: mostly completed. Synthetic fixture coverage exists; remaining work is
real-world fixture coverage and provider capability hints.

Goal: make Codex requests work reliably against Chat-only domestic models.

Tasks:

- Extend tool context:
  - function tools
  - namespace tools flattened to Chat-safe names and restored later
  - custom tools represented as a function with `{input:string}`
  - `tool_search` represented as a function
- Drop `tool_choice` and `parallel_tool_calls` when converted `tools` is empty.
- Convert `reasoning.effort` to provider-compatible Chat fields where safe.
- Preserve extra Chat passthrough fields when they are meaningful.
- Canonicalize parseable JSON strings in tool arguments and tool outputs for stable cache behavior.
- Add optional placeholder `reasoning_content` for assistant messages with tool calls when the target channel/model is marked as reasoning-required.

Acceptance:

- Strict Chat-compatible upstreams no longer reject converted requests due to empty tools/tool_choice.
- DeepSeek/Kimi-style tool-call requests include compatible reasoning metadata.
- Tool names are reversible for namespace/custom/tool_search cases.

### Phase 3: Cross-Turn History Cache

Status: completed for in-memory single-instance use; Redis-backed sharing is a
future enhancement.

Goal: support `previous_response_id` follow-up turns.

Tasks:

- Implement bounded in-memory cache:
  - key: response id
  - value: call id -> Responses call item
  - max entries: default 512 responses
  - TTL: default 2 hours
- Record call items from:
  - non-stream final `response.output`
  - stream `response.output_item.done`
  - stream `response.completed`
- Before converting a Responses request to Chat:
  - if input contains tool outputs without corresponding call items, restore call items from `previous_response_id`
  - if previous id misses, restore by unique call id only
  - do not restore ambiguous call ids
- Later optional improvement: Redis-backed shared cache for multi-replica deployment.

Acceptance:

- Codex tool-call round trip works across turns against Chat-only channels.
- Restart or cache miss degrades gracefully by using reasoning placeholder, not panic.
- Logs expose `history_restored_count` and `history_restore_sources`.

### Phase 4: Reasoning and Inline Think Handling

Status: completed for common provider shapes; explicit per-model gating remains
future capability metadata work.

Goal: preserve useful thinking-model output without blocking normal answer text.

Tasks:

- Extract reasoning from common Chat fields:
  - `reasoning_content`
  - `reasoning`
  - `reasoning_details`
  - leading `<think>...</think>` content
- Emit Responses reasoning stream events:
  - `response.output_item.added` with `type=reasoning`
  - `response.reasoning_summary_part.added`
  - `response.reasoning_summary_text.delta`
  - `response.reasoning_summary_text.done`
  - `response.reasoning_summary_part.done`
  - `response.output_item.done`
- Attach reasoning to function/custom/tool_search call items where useful.
- Keep reasoning feature gated by channel/model compatibility to avoid leaking hidden reasoning where a provider forbids it.

Acceptance:

- Text answer is still emitted even when reasoning chunks arrive first.
- Tool calls preserve reasoning when upstream returns it.
- Domestic thinking model traces no longer produce blank Codex responses.

### Phase 5: Custom Tool and Tool Search Response Mapping

Status: completed for function/custom/tool_search/namespace and hosted-tool
functionization/restoration.

Goal: map Chat tool calls back into the correct Responses item type.

Tasks:

- Restore `function_call`, `custom_tool_call`, and `tool_search_call` based on original tool context.
- For custom tools, emit native custom tool stream events:
  - `response.custom_tool_call_input.delta`
  - `response.custom_tool_call_input.done`
- For namespace tools, restore `namespace` and original child `name`.
- For `tool_search_output`, collect loaded tools and make them available for later conversion.

Acceptance:

- Codex MCP/tool-search flows can continue after the first tool discovery call.
- Final Responses output contains the same item type Codex expects.

### Phase 6: Non-Stream SSE Sniffing and Aggregation

Status: completed for Chat SSE fallback on Responses-via-Chat and Responses SSE
fallback on Chat-via-Responses.

Goal: handle upstreams that return SSE even when the request is `stream:false` or set an incorrect content type.

Tasks:

- If non-stream JSON parsing fails, inspect body prefix for SSE (`data:`, `event:`).
- Aggregate Chat SSE into a final Chat response, then run non-stream Chat -> Responses conversion.
- Add diagnostic error snippets when parsing still fails.
- Preserve current error mapping for genuine upstream errors.

Acceptance:

- Non-stream Codex calls do not fail just because a provider force-streamed.
- Bad upstream bodies produce actionable errors in request logs.

### Phase 7: Observability, Billing, and UI

Status: mostly completed. Structured request conversion metadata is now logged,
including inferred channel capability, recommended reasoning adapter, and
history restore sources; the usage log detail view renders those fields plus
unsupported tool filtering, and channel-form protocol/reasoning hints are
visible. Per-provider warnings can still become more exact as the capability
model grows.

Goal: make conversion behavior visible and administrable.

Tasks:

- Usage logs:
  - request id
  - source protocol and final upstream protocol
  - conversion warnings
  - unsupported/dropped tool types
  - history restore stats
  - reasoning backfill flag
- Billing:
  - prefer upstream usage
  - inject `stream_options.include_usage` for Chat streams
  - estimate only when upstream usage is absent
  - do not double-count synthetic all-zero usage
- Channel UI:
  - keep Chinese labels for `responses_protocol`
  - add help text explaining native/auto/chat conversion/disabled
  - later add per-model capability hints: native Responses, Chat-only, requires reasoning_content, supports vision

Acceptance:

- Admin can diagnose why a request converted or failed, including unsupported
  tool filtering, from one request detail page.
- Converted streams bill from upstream usage when available.

## Hosted Tool Conversion

Protocol conversion now preserves hosted-tool intent as metadata instead of
pretending Chat-only upstreams can execute OpenAI hosted tools:

- Request side: hosted Responses tools are filtered from Chat tools by default.
- Response side: Chat-only upstreams should not receive synthetic hosted tool
  functions unless an executor bridge is explicitly configured in the future.
- History side: restored hosted call items are downgraded to assistant context
  explaining that no hosted executor is available.

Execution remains a separate capability question:

- If the model/provider has native hosted-tool execution, prefer a native
  Responses route.
- If a future data-proxy executor owns the tool loop, it can re-enable selected
  hosted tools as real Chat functions and continue the conversation after
  execution.
- If neither side can execute the operation, the safe behavior is to filter the
  hosted tool, log that decision, and ask the model to answer directly or state
  the limitation.

## Implementation Priority

1. Capture additional sanitized golden fixtures from real Codex/domestic-model traffic.
2. Run E2E against SiliconFlow/DeepSeek-style Chat-only channels for search,
   tool loop, reasoning-before-text, and `previous_response_id` turns.
3. Add richer per-provider/per-model capability hints in channel settings and logs.
4. Add optional Redis-backed history cache for multi-instance deployments.
5. Improve UI help text around Responses protocol and reasoning adapters.

This order now focuses on production hardening rather than core conversion
mechanics, which are implemented in project-native Go code.

## Test Matrix

Minimum tests before deployment:

- `go test ./service/openaicompat ./relay -count=1`
- `go test ./relay/channel/openai -count=1` if native Responses -> Chat paths are changed
- `go test ./service ./relay ./relay/channel/openai -count=1` before release
- E2E with a Chat-only domestic model:
  - "当前时间是多少"
  - "用百度查询一下，人民币和美元汇率"
  - a prompt that forces a local tool call
  - a prompt that produces reasoning before text
- E2E with native OpenAI Responses channel to ensure native behavior is not regressed.

## License Notes

- `copilot2api`, `CLIProxyAPI`, and `cc-switch` are MIT licensed.
- MIT code can be adapted into an AGPL/new-api-based project, but copied code must retain copyright and MIT license notices.
- Prefer reimplementation based on observed behavior and tests. Copy only small, necessary helpers when the benefit is clear.
