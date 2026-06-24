# OpenAI Hosted Tools Support Plan

Date: 2026-06-23

## Background

Codex Desktop can show `web_search` execution when it talks to a native OpenAI
Responses model such as GPT-5.5. That does not mean data-proxy or Codex Desktop
is necessarily executing search locally. In the native Responses path, OpenAI's
Responses runtime can execute hosted tools and stream the tool-call lifecycle
and final assistant message back to the client.

When data-proxy converts `/v1/responses` to `/v1/chat/completions` for a
Chat-only upstream, the OpenAI hosted-tool runtime is not present. If hosted
tools are naively exposed as Chat function tools, a domestic model can emit a
`web_search` function call, but neither the Chat upstream nor data-proxy has an
executor to run it. The user-facing result can look like an empty response.

Current production behavior is therefore:

- preserve normal client/tool-loop tools: `function`, `custom`, `tool_search`,
  and `namespace`;
- filter OpenAI hosted/external tools from Chat tools;
- inject a direct-answer hint when hosted tools were filtered;
- record the behavior in request conversion metadata:
  `hosted_tools_filtered` and `hosted_tools_direct_answer_hint`.

## Official Tool Surface To Track

OpenAI's tools documentation currently describes built-in tools, function
calling, tool search, and remote MCP servers. The pages most relevant to
data-proxy are:

- Tools overview: https://developers.openai.com/api/docs/guides/tools
- Web search: https://developers.openai.com/api/docs/guides/tools-web-search
- File search: https://developers.openai.com/api/docs/guides/tools-file-search
- Computer use: https://developers.openai.com/api/docs/guides/tools-computer-use
- Code Interpreter: https://developers.openai.com/api/docs/guides/tools-code-interpreter
- Image generation: https://developers.openai.com/api/docs/guides/tools-image-generation

For data-proxy protocol conversion, classify tool types as follows.

| Tool type | Current classification | Conversion behavior today | Support target |
| --- | --- | --- | --- |
| `function` | Client/function tool | Convert to Chat function | Keep |
| `custom` | Client/custom tool | Convert to Chat function with preserved input | Keep |
| `tool_search` | Codex/tool discovery | Convert to Chat function and restore Responses shape | Keep |
| `namespace` | Codex/MCP namespace wrapper | Flatten child functions into Chat functions | Keep |
| `web_search`, `web_search_preview` | OpenAI hosted web search | Filter, inject direct-answer hint | Add optional executor/provider bridge |
| `file_search` | OpenAI hosted retrieval over vector stores | Filter, inject direct-answer hint | Add optional retrieval bridge |
| `computer`, `computer_use_preview` | External UI/computer-use loop | Filter, inject direct-answer hint | Support only with explicit client/executor loop |
| `image_generation` | Hosted image generation | Filter, inject direct-answer hint | Route to native Responses or image-generation executor |
| `code_interpreter` | Hosted sandbox/code execution | Filter, inject direct-answer hint | Support only with sandbox executor and policy controls |
| `mcp` | Hosted/remote MCP call | Filter, inject direct-answer hint | Bridge to configured MCP clients only after auth/policy design |
| `shell` | Hosted or local runtime shell, docs mention both shapes | Not currently parsed as hosted | Track; do not enable without sandbox policy |
| `skills` | Hosted skill bundles / platform tool surface | Not currently parsed as hosted | Track; map only after schema is stable |

## Comparison With cc-switch

cc-switch's Codex Responses -> Chat converter keeps only tools that can be
expressed as Chat function calls: `function`, `custom`, `tool_search`, and
`namespace`. Other tool types fall through and are ignored. It also removes
`tool_choice` and `parallel_tool_calls` when the converted Chat request has no
tools, avoiding strict upstream errors.

data-proxy now matches that core boundary: hosted tools are not exposed as
callable Chat functions unless an executor exists. data-proxy adds one extra
observable behavior: when hosted tools are filtered, it injects a direct-answer
system hint and records `hosted_tools_direct_answer_hint=true`. This is
intentional and should remain visible in logs because it can change the model's
answer style compared with cc-switch's silent filter.

## Why We Should Not Pretend Hosted Tools Are Local

The important boundary is who owns the tool executor:

- Native OpenAI Responses route: OpenAI can own hosted tool execution.
- Chat-only domestic upstream: the upstream only sees Chat messages and Chat
  function schemas; it does not own OpenAI hosted tools.
- Codex Desktop: can display hosted-tool lifecycle events and can execute some
  client-side/local tool loops, but an OpenAI hosted tool such as web search is
  not automatically a local Codex tool.
- data-proxy: currently has no hosted web search, file retrieval, browser,
  computer-use, image, code, shell, or remote MCP executor.

Therefore, automatic conversion must not create a fake `web_search` function
unless data-proxy also implements and wires the executor loop.

## Development Plan

### Phase 0: Observability and Safety

Status: completed for the current minimal path.

- Keep filtering hosted tools in Responses -> Chat conversion.
- Keep direct-answer hint injection for filtered hosted tools.
- Log `hosted_tools_filtered` and `hosted_tools_direct_answer_hint`.
- Show the metadata in usage log details.
- Add regression fixtures for hosted `web_search` and `web_search_preview`.

### Phase 1: Tool Capability Model

Goal: make hosted-tool behavior explicit per channel/model.

- Add channel settings for hosted tool policy:
  - `filter_and_direct_answer` (default);
  - `native_responses_required`;
  - `executor_bridge`;
  - `reject_with_clear_error`.
- Add per-channel capability metadata:
  - supports native Responses;
  - supports Chat function tools;
  - supports hosted web search;
  - supports remote MCP bridge;
  - supports code/image/computer/shell executors.
- Surface the policy in channel UI with Chinese labels.
- Record the selected policy in `request_conversion_meta`.

### Phase 2: Web Search Executor Bridge

Goal: support `web_search` for Chat-only upstreams without relying on OpenAI's
hosted runtime.

- Define a `HostedToolExecutor` interface for data-proxy.
- Add a web search provider interface:
  - query;
  - optional freshness/domain filters;
  - source title/url/snippet;
  - citation metadata.
- Implement an opt-in provider, for example:
  - Bing/SerpAPI/Tavily/Brave;
  - self-hosted search;
  - admin-configured webhook.
- Add a tool loop:
  - send converted Chat request with a real `web_search` function only when the
    executor is configured;
  - execute tool calls server-side;
  - append `tool` messages;
  - continue Chat completion until final answer or max iterations.
- Convert final output back to Responses, including synthetic
  `web_search_call` items and annotations where possible.

### Phase 3: Remote MCP Bridge

Goal: map Responses `mcp` hosted tool declarations to real data-proxy-managed
MCP clients.

- Reuse existing MCP proxy/dashboard concepts where possible.
- Add app/channel scoped MCP allowlist.
- Bind auth, audit, quota, and approval policies.
- Convert Chat tool calls back into Responses `mcp_call` lifecycle items.
- Add per-tool request ids and audit events.

### Phase 4: File Search / Retrieval Bridge

Goal: support `file_search` when data-proxy owns retrieval.

- Define vector-store abstraction:
  - OpenAI vector store passthrough when native Responses is used;
  - local/external vector DB for Chat-only providers.
- Add file-store ownership and tenant isolation rules.
- Map file search results into Chat tool outputs.
- Convert retrieval citations/annotations back into Responses-compatible output.

### Phase 5: Higher-Risk Executors

Goal: only support powerful hosted tools with explicit policy and sandboxing.

- `code_interpreter`:
  - sandbox runtime;
  - CPU/memory/time limits;
  - file IO limits;
  - audit logs and artifact retention.
- `image_generation`:
  - route to image model channel or native Responses;
  - quota and moderation handling.
- `computer` / `computer_use_preview`:
  - browser/desktop harness;
  - screenshot/action loop;
  - human approval gates for high-impact actions.
- `shell`:
  - disabled by default;
  - isolated container only;
  - admin allowlist and audit.

## Acceptance Criteria

- A request detail page can answer:
  - whether hosted tools were requested;
  - whether they were filtered, rejected, natively executed, or executor-bridged;
  - whether a direct-answer hint was injected;
  - which executor ran and what it cost.
- Domestic Chat-only channels do not emit unhandled hosted tool calls.
- Native Responses channels continue to pass hosted tools through unchanged.
- Executor-backed channels can run at least one real `web_search` loop and
  return a final Responses message with citations or source metadata.
- All executor calls have request ids, audit events, quota accounting, and
  provider/channel/user attribution.

## Immediate Next Tasks

1. Keep current filtered-hosted-tool behavior as the default safe mode.
2. Add `hosted_tools_policy` channel setting and UI.
3. Add executor interface and a no-op implementation.
4. Implement a real web search executor behind an admin-configured provider.
5. Add production smoke tests for:
   - native Responses + OpenAI hosted web search;
   - Chat-only domestic model + filtered hosted web search;
   - Chat-only domestic model + executor-backed hosted web search.
