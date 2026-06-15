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

Coze response conversion still has a `TODO` for supporting more content types.
This should be handled as provider compatibility work, not a generic cleanup.

Recommended work:

- inspect current Coze response DTOs and upstream payload shapes used by the
  relay
- add tests for known text, image/file, and unknown content blocks
- keep unknown content deterministic instead of silently panicking or dropping
  useful error context

### P1 - Verify Cohere streaming usage behavior

Source: `relay/channel/cohere/adaptor.go`

The stream path still carries a `TODO: fix this` comment around stream usage
handling. This is a narrow relay accounting/response-shape task.

Recommended work:

- compare non-stream and stream usage extraction
- add regression tests for stream chunks with and without usage metadata
- keep current behavior if upstream does not expose usage, but document the
  fallback clearly

### P2 - Review API version normalization

Source: `middleware/distributor.go`

The `api_version` normalization comment requires a routing-level review. It
should not be changed opportunistically because it may affect channel selection,
request URL mapping, and provider compatibility.

Recommended work:

- trace where `api_version` enters relay info and provider request URLs
- add tests before changing normalization
- decide whether normalization belongs in middleware, channel metadata, or
  provider adaptors

### P2 - Add DTO shape compatibility tests

Source: `dto/audio.go`, `dto/gemini.go`

Remaining DTO TODOs are request-shape concerns. They should be covered with
provider-specific compatibility tests before changing request parsing.

Recommended work:

- add minimal parsing tests around audio stream lifecycle assumptions
- add Gemini thinking-budget conflict tests before changing fields or defaults

## Intentional Unsupported / Do Not Batch-Implement

### OpenAI-compatible routes intentionally returning 501

Source: `router/relay-router.go`, `controller/relay.go`

Routes such as `/files`, `/fine-tunes`, `/images/variations`, model delete, and
Responses compact currently use explicit not-implemented handling. These should
stay explicit 501-style responses until product decides to proxy or implement
those APIs.

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
- `model/main.go`: legacy migration comment can be removed only after an
  upgrade-floor decision.
- `relay/claude_handler.go` and `relay/channel/claude/relay-claude.go`: temporary
  compatibility comments should be revisited with Claude payload tests.
- `common/redis.go`, `relay/common/override.go`, `service/file_service.go`, and
  related `unsupported ...` errors are validation boundaries, not missing
  implementation.
- Smoke helper `panic` calls in `tools/*smoke*.mjs` are test fail-fast behavior.

## Next-Step Recommendation

Start with Coze non-text content handling, then Cohere streaming usage behavior.
Both are narrow, provider-specific tasks with clear test boundaries and no need
to broaden the core MCP/OpenAPI release surface.
