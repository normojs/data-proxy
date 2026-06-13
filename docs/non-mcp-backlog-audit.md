# Non-MCP Backlog Audit

Date: 2026-06-13

This audit classifies remaining `TODO`, `unsupported`, `not implemented`, and
`panic` scan findings outside the completed MCP / Bridge / OpenAPI release
scope.

## Summary

The remaining scan findings are not one category of work. They split into:

- product backlog candidates that can improve reliability or user experience
- intentionally unsupported API/provider paths
- low-priority technical debt or test-only fail-fast code

The next backend batch should focus on small, testable improvements rather than
attempting to implement every provider adaptor stub.

## Recommended Next Backend Batch

### P1 - Make channel balance refresh asynchronous

Source: `controller/channel-billing.go`

`UpdateAllChannelsBalance` currently runs synchronously and sleeps between
channels. On installations with many channels, the admin HTTP request can block
for a long time or hit proxy/client timeouts.

Recommended work:

- move the admin-triggered all-channel refresh into a background job
- prevent overlapping refresh jobs
- return an immediate accepted response with job state
- expose recent job status in logs or an admin-readable response
- keep the existing single-channel balance query synchronous

### P1 - Add conservative sensitive checks for image URL payloads

Source: `service/sensitive.go`

`CheckSensitiveMessages` skips `image_url` content entirely. Full image
moderation/OCR is a larger product decision, but the service can still scan
obvious text-bearing fields such as URL strings and known metadata without
downloading remote content.

Recommended work:

- scan URL string values and any textual image metadata already present in the
  request payload
- do not fetch remote images in this pass
- keep behavior deterministic and cheap
- add tests for mixed text/image content

### P1 - Add SSRF-safe HTTP image input support for Gemini/Veo task images

Source: `relay/channel/task/gemini/image.go`

Veo task image input currently supports data URIs/raw base64 but not HTTP image
URLs. URL support should be implemented only with the project's existing SSRF
protection and strict size/mime limits.

Recommended work:

- reuse existing safe HTTP client / SSRF protection helpers
- cap downloaded bytes
- accept only supported image MIME types
- convert to base64 for the existing `VeoImageInput` path
- add tests for data URI, raw base64, rejected hosts, oversized content, and
  unsupported MIME

### P2 - Review Realtime WebSocket subprotocol compatibility

Source: `controller/relay.go`

The WebSocket upgrader currently declares only the `realtime` subprotocol.
Before adding more protocols, confirm actual client expectations and upstream
behavior.

Recommended work:

- collect expected subprotocol names from supported clients
- add tests for accepted and rejected protocols
- avoid broad wildcard behavior

## Intentional Unsupported / Do Not Batch-Implement

### OpenAI-compatible routes intentionally returning 501

Source: `router/relay-router.go`, `controller/relay.go`

Routes such as `/files`, `/fine-tunes`, `/images/variations`, and model delete
currently use `RelayNotImplemented`. These should stay explicit 501 responses
until the product decides to proxy or implement those APIs.

### Provider adaptor stubs

Source: `relay/channel/*/adaptor.go`

Many provider adaptors return `not implemented` for relay modes they do not
support. This is expected in a multi-provider gateway. Implementing these should
be driven by a provider-specific requirement and upstream API contract, not by a
generic TODO cleanup pass.

### MCP unsupported paths

Source: `pkg/mcp/*`, `service/mcp*.go`, `tools/bridge_client_daemon.mjs`

The MCP / Bridge unsupported paths are stable error handling or deliberately
unsupported daemon tool requests. They were covered by the previous MCP release
audit and should not be reopened unless a new capability is requested.

## Lower-Priority Technical Debt

- `model/main.go`: legacy migration comment can be removed only after an
  upgrade-floor decision.
- `middleware/distributor.go`: API version normalization requires a routing
  architecture review.
- `dto/audio.go` and `dto/gemini.go`: request-shape TODOs need provider-specific
  compatibility tests.
- Task polling `context.TODO()` usage was resolved by the follow-up task
  lifecycle cleanup; future work should focus on product-level task polling
  behavior rather than placeholder contexts.
- `model/*_test.go`, `service/*_test.go`, and smoke helper `panic` calls are
  test-only fail-fast behavior, not production panic paths.

## Next-Step Recommendation

Start with asynchronous channel balance refresh. It is self-contained,
admin-facing, and does not depend on external provider API research. Then do the
conservative sensitive image URL scan. Leave provider adaptor work for explicit
provider roadmap items.
