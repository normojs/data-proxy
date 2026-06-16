# Non-MCP Backlog Audit

Date: 2026-06-16

This audit classifies remaining `TODO`, `unsupported`, `not implemented`, and
`panic` scan findings outside the completed MCP / Bridge / OpenAPI release
scope.

## Summary

The previous recommended backend batch is complete:

- admin all-channel balance refresh now runs asynchronously
- image URL sensitive checks cover text-bearing image fields without fetching
  remote images
- Gemini/Veo task images support SSRF-safe HTTP(S) input with size and MIME
  limits
- Realtime WebSocket subprotocol compatibility is explicit and tested
- generic provider adaptor `TODO implement me` / `errors.New("not implemented")`
  stubs now return typed `channel.UnsupportedFeatureError` values

The remaining scan findings are therefore smaller and mostly fall into:

- intentionally unsupported product/API boundaries
- provider-specific compatibility polish
- startup fail-fast behavior
- low-priority routing or DTO technical debt

## Recommended Next Backend Batch

### P1 - Audit Coze non-text content handling

Source: `relay/channel/coze/relay-coze.go`

Status: completed on 2026-06-16.

Coze request conversion now handles OpenAI text, image, file, and video content
deterministically. Supported media is sent as Coze `object_string` payloads;
unsupported media is represented as stable text placeholders instead of being
silently dropped.

Completed work:

- added focused conversion tests for string text, text-only media, image media,
  file IDs, remote file/video URLs, unsupported audio, and mixed supported plus
  unsupported content
- kept the implementation scoped to request conversion; no remote fetching or
  cross-provider refactor was introduced

### P1 - Verify Cohere streaming usage behavior

Source: `relay/channel/cohere/adaptor.go`

Status: completed on 2026-06-16.

Cohere v1 chat streaming exposes final usage through
`response.meta.billed_units` when present. The stream path now copies those
values into OpenAI-compatible usage, computes `total_tokens`, fills missing
prompt/completion fields from local estimates only when necessary, and emits a
final usage chunk before `[DONE]` when downstream usage output is enabled.

Completed work:

- removed the stream-path `TODO: fix this`
- added regression tests for final stream usage metadata, no-metadata fallback,
  partial metadata fallback, and emitted usage chunks
- kept API-version work separate from this fix; Cohere endpoint/version
  normalization remains covered by the API version review item below

### P2 - Review API version normalization

Source: `middleware/distributor.go`

Status: completed on 2026-06-16.

The provider metadata setup is now isolated in `setupProviderMetadataContext`
and covered by behavior tests before any normalization change. Current behavior
is intentionally preserved: Azure/Xunfei/Gemini/Cloudflare/MokaAI populate
`api_version`, Vertex populates `region`, Ali populates `plugin`, Coze
populates `bot_id`, and OpenAI leaves provider metadata unset. `GetAPIVersion`
also remains covered with query `api-version` taking precedence over context
`api_version`.

Completed work:

- removed the vague `api_version` TODO from the request path
- added focused middleware tests for provider metadata mappings
- added relay common tests for `GetAPIVersion` precedence
- deferred any semantic normalization until a provider-by-provider migration is
  explicitly planned

### P2 - Add DTO shape compatibility tests

Source: `dto/audio.go`, `dto/gemini.go`

Status: completed on 2026-06-16.

The remaining DTO request-shape TODOs are now covered by focused compatibility
tests and replaced with explicit comments. Audio streaming remains keyed by
`stream_format=sse`; a boolean `stream` field in audio JSON is ignored because
audio streaming lifecycle is handled downstream after request conversion. Gemini
thinking config preserves `thinkingBudget` and `thinkingLevel` together, accepts
snake_case aliases inside `generationConfig`, and gives snake_case values
precedence when both naming styles are present.

Completed work:

- added audio `IsStream` tests for `sse`, non-stream formats, case sensitivity,
  and ignored boolean `stream`
- added Gemini thinking config tests for camelCase, snake_case, and
  snake-over-camel precedence
- left DTO behavior unchanged; these tests are guardrails for any future field
  normalization

## Intentional Unsupported / Do Not Batch-Implement

### OpenAI-compatible routes intentionally returning 501

Source: `router/relay-router.go`, `controller/relay.go`

Routes such as `/files`, `/fine-tunes`, `/images/variations`, model delete, and
Responses compact currently use explicit not-implemented handling. These should
stay explicit 501-style responses until product decides to proxy or implement
those APIs.

The OpenAI-compatible route subset is now regression-covered: controller tests
lock the OpenAI-style 501 response shape and router tests lock route
registration to the explicit `RelayNotImplemented` handler so these routes do
not drift to 404 or generic relay handling accidentally.

### Provider adaptor unsupported feature errors

Source: `relay/channel/*/adaptor.go`, `relay/channel/unsupported.go`

Provider adaptors now use `channel.UnsupportedFeatureError` for unsupported
conversion methods. This preserves `not implemented` wording while making the
error typed and testable. Do not implement these capabilities generically; each
provider needs upstream API contract work first.

### MCP unsupported paths

Source: `pkg/mcp/*`, `service/mcp*.go`, `tools/bridge_client_daemon.mjs`

The MCP / Bridge unsupported paths are stable error handling or deliberately
unsupported daemon tool requests. They remain covered by the MCP release and
Bridge smoke work.

## Remaining Technical Debt

- `common/gin.go`: non-JSON request model variation needs request parsing design
  before implementation.
- `model/main.go`, `common/embed-file-system.go`, `common/pprof.go`: production
  startup fail-fast `panic` calls are intentional startup guards, not request
  path panics.
- `model/main.go` and `setting/ratio_setting/model_ratio.go`: ambiguous TODO
  comments were replaced with explicit maintenance notes; the disabled legacy
  migration statement still waits for an upgrade-floor decision, and new
  billable model APIs still require pricing table review before routing traffic.
- `relay/claude_handler.go` and `relay/channel/claude/relay-claude.go`: temporary
  compatibility comments should be revisited with Claude payload tests.
- `common/redis.go`, `relay/common/override.go`, `service/file_service.go`, and
  related `unsupported ...` errors are validation boundaries, not missing
  implementation.
- Smoke helper `panic` calls in `tools/*smoke*.mjs` are test fail-fast behavior.

## Next-Step Recommendation

This non-MCP provider compatibility batch is complete. Future provider work
should start from a fresh audit rather than reopening this batch opportunistically.
