# Responses to Chat Completions Compatibility

Data Proxy supports a compatibility path for clients that call `/v1/responses`
while the selected upstream channel only exposes OpenAI-compatible
`/v1/chat/completions`.

## Channel Setting

Channel extra settings store the protocol mode in `settings.responses_protocol`.

| Value | Behavior |
| --- | --- |
| `auto` or empty | Infer from channel type. Native Responses channels stay native; known Chat-only OpenAI-compatible channels use conversion. |
| `native` | Send `/v1/responses` to the upstream directly. |
| `chat_completions` | Convert Responses requests to Chat Completions, then wrap Chat JSON/SSE back into Responses shape. |
| `disabled` | Reject `/v1/responses` for this channel. |

The admin channel form exposes this as **Responses Protocol** under advanced
settings.

## Current Conversion Scope

Supported:

- `instructions` to a leading system message.
- string and array `input` items to chat messages.
- `function` tools.
- Codex-style `custom` and `tool_search` tools as Chat function tools.
- tools returned by `tool_search_output`, when they can be represented as Chat
  function tools.
- non-streaming Chat responses back to Responses JSON.
- streaming Chat SSE back to basic Responses SSE events.
- token usage mapping for billing.

Filtered during Chat conversion:

- hosted OpenAI tools such as `web_search`, `web_search_preview`,
  `file_search`, `computer`, `computer_use_preview`, `image_generation`,
  `code_interpreter`, and hosted `mcp`.
- unknown Responses tool types that cannot be represented as Chat function
  tools.

This follows the compatibility behavior used by mature local routing proxies:
requests are allowed to continue, but Chat-only upstreams do not execute those
hosted tools. Use a native Responses upstream when true hosted tool execution is
required.

## Operational Guidance

Use `auto` for most channels. For providers such as SiliconFlow that support
`/v1/chat/completions` but not `/v1/responses`, either leave `auto` enabled if
the channel type is recognized or set the mode explicitly to
`chat_completions`.

Use `native` for providers with real Responses support. Use `disabled` when a
channel must never receive Responses traffic.
