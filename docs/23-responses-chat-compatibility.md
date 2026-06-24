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

`settings.responses_reasoning_adapter` controls how Responses `reasoning` is
mapped when this compatibility path sends a Chat Completions request.

| Value | Behavior |
| --- | --- |
| `default` or empty | Keep the legacy `reasoning_effort` mapping. |
| `auto` | Infer a provider-specific adapter from the channel type when known. |
| `off` | Do not forward reasoning controls. |
| `openai` | Send top-level `reasoning_effort`. |
| `deepseek` | Send `thinking` plus mapped top-level `reasoning_effort`; `xhigh/max` maps to `max`, other enabled efforts map to `high`. |
| `openrouter` | Send nested `reasoning.effort`; `max/xhigh` maps to `xhigh`, and explicit `none/off/disabled` maps to `{"reasoning":{"effort":"none"}}`. |
| `qwen_enable_thinking` | Send boolean `enable_thinking`. |
| `minimax_reasoning_split` | Send boolean `reasoning_split`. |
| `low_high` | Map effort to top-level `reasoning_effort` with `minimal/low -> low`, everything else enabled -> `high`. |

## Current Conversion Scope

Supported:

- `instructions` to a leading system message.
- string and array `input` items to chat messages.
- `function` tools.
- Codex-style `custom` and `tool_search` tools as Chat function tools.
- Responses hosted tool declarations such as `web_search`,
  `web_search_preview`, `file_search`, `computer`,
  `computer_use_preview`, `image_generation`, `code_interpreter`, and hosted
  `mcp` as Chat function tools.
- tools returned by `tool_search_output`, when they can be represented as Chat
  function tools.
- non-streaming Chat responses back to Responses JSON.
- streaming Chat SSE back to basic Responses SSE events.
- native Responses SSE back to Chat Completions chunks when Chat requests are
  routed through a native Responses upstream.
- Chat tool calls restored back to Responses `function_call`,
  `custom_tool_call`, `tool_search_call`, and hosted call items such as
  `web_search_call`.
- token usage mapping for billing.

Filtered during Chat conversion:

- unknown Responses tool types that cannot be represented as Chat function
  tools.

This follows the compatibility behavior used by mature local routing proxies:
requests are allowed to continue and tool context is preserved across protocol
conversion. Chat-only upstreams still do not get native server-side execution
for hosted tools unless the selected provider/runtime has that capability.

## Operational Guidance

Use `auto` for most channels. For providers such as SiliconFlow that support
`/v1/chat/completions` but not `/v1/responses`, either leave `auto` enabled if
the channel type is recognized or set the mode explicitly to
`chat_completions`.

Use `native` for providers with real Responses support. Use `disabled` when a
channel must never receive Responses traffic.

## Logs and Troubleshooting

Text request logs include `other.request_conversion` and
`other.request_conversion_meta` for converted traffic. Useful fields include:

- `responses_protocol`: normalized channel setting.
- `upstream_protocol`: final upstream protocol, such as `responses` or
  `chat_completions`.
- `responses_protocol_decision`: whether the request stayed native or was
  converted.
- `responses_channel_capability`: the channel capability inferred by
  data-proxy, such as native Responses support, Chat Completions compatibility,
  or unknown.
- `responses_reasoning_adapter_recommended`: the provider-specific reasoning
  adapter data-proxy recommends for this channel type when it can infer one.
- `responses_reasoning_adapter`, `reasoning_params`, and
  `reasoning_effort_mapped`: provider-specific reasoning mapping details.
- `hosted_tools_filtered`: OpenAI hosted Responses tools intentionally omitted
  from Chat tools because data-proxy does not execute hosted tools on behalf of
  the upstream.
- `hosted_tools_direct_answer_hint`: data-proxy injected an additional system
  hint asking the Chat upstream to answer directly, or briefly state limitations,
  after hosted Responses tools were filtered.
- `hosted_tools_functionized`: legacy metadata for hosted Responses tools
  represented as Chat function tools.
- `unsupported_tools_filtered`: Responses tool types filtered because they
  cannot be represented safely as Chat Completions tools.
- `history_restored_count`, `history_restore_sources`, and
  `history_recorded_count`: cross-turn tool-call cache activity for
  `previous_response_id` flows, including whether restoration came from the
  previous response id or a unique call-id fallback.
- `chat_sse_fallback`: non-stream route received Chat SSE and aggregated it.
- `responses_terminal_status` and `responses_incomplete_details`: terminal
  status mapped back into Responses shape.
