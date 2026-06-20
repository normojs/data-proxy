# Fusion Benchmark Spike

This harness validates whether Chinese-model Fusion presets are worth turning into a production API.

It provides three pieces:

- An OpenAI-compatible Fusion proxy for LiveBench and other API-based benchmarks.
- A small objective fresh-eval runner for local JSONL datasets.
- A Markdown report generator for score, cost, latency, degradation, and GPT-5.5 comparison.

Planning docs:

- `evaluation-plan.md`: long-form benchmark design.
- `v2-evaluation-rollout.md`: concrete v2 plan for public benchmark anchors,
  private isomorphic tasks, runnable coding, and Fusion-vs-GPT-5.5 decision
  thresholds.
- `fusion-v3-roadmap.md`: roadmap for turning Fusion into a stronger
  model-like virtual model with task-aware routing, ranking, verification, and
  anti-degradation guards rather than an open-ended agent.
- `implementation-backlog.md`: implementation milestones.

## Commands

Validate local config and key-file loading:

```bash
node tools/fusion-benchmark.mjs validate-config \
  --env-file tools/fusion-benchmark/.env.local
```

List and probe upstream relay models:

```bash
node tools/fusion-benchmark.mjs models-list \
  --env-file tools/fusion-benchmark/.env.local \
  --upstream default

node tools/fusion-benchmark.mjs models-probe \
  --env-file tools/fusion-benchmark/.env.local \
  --upstream default \
  --filter 'qwen|kimi|moonshot|deepseek|minimax|gpt|claude|gemini'
```

Probe whether an upstream can provide relay/provider billing signals:

```bash
node tools/fusion-benchmark.mjs billing-probe \
  --env-file tools/fusion-benchmark/.env.local \
  --upstream all
```

Measure baseline relay latency without running a full benchmark:

```bash
node tools/fusion-benchmark.mjs latency-probe \
  --env-file tools/fusion-benchmark/.env.local \
  --models qwen/qwen3.7-plus,qwen/qwen3.6-flash,moonshotai/kimi-k2.6,deepseek/deepseek-v4-pro,minimax/minimax-m3,openai/gpt-5.5 \
  --rounds 3 \
  --timeout-ms 90000
```

Start the Fusion proxy:

```bash
cp tools/fusion-benchmark/.env.example tools/fusion-benchmark/.env.local
# Edit tools/fusion-benchmark/.env.local locally and set each upstream base/key you plan to use.

node tools/fusion-benchmark.mjs serve \
  --port 8787 \
  --env-file tools/fusion-benchmark/.env.local \
  --log tools/fusion-benchmark/runs/proxy-requests.jsonl
```

Run LiveBench through the proxy:

```bash
node tools/fusion-benchmark.mjs livebench-plan --api-base http://127.0.0.1:8787/v1
```

Import LiveBench judgments into the unified report format:

```bash
node tools/fusion-benchmark.mjs livebench-import \
  --inputs /path/to/LiveBench/livebench/data \
  --release 2026-01-08 \
  --out tools/fusion-benchmark/runs/livebench-2026-01-08.jsonl
```

Run the sample fresh eval:

```bash
node tools/fusion-benchmark.mjs fresh-validate \
  --dataset tools/fusion-benchmark/data/fresh-eval.example.jsonl

node tools/fusion-benchmark.mjs fresh-run \
  --api-base http://127.0.0.1:8787/v1 \
  --env-file tools/fusion-benchmark/.env.local \
  --dataset tools/fusion-benchmark/data/fresh-eval.example.jsonl \
  --category coding,data_analysis \
  --limit 20 \
  --out tools/fusion-benchmark/runs/fresh-example.jsonl
```

Omit `--category` and `--limit` for a full run. Use `--offset` with `--limit` to run a larger fresh dataset in batches.

For a cheap balanced pilot, use one or a few questions from each category:

```bash
node tools/fusion-benchmark.mjs fresh-run \
  --env-file tools/fusion-benchmark/.env.local \
  --dataset tools/fusion-benchmark/data/fresh-eval.v1-differentiated.jsonl \
  --per-category-limit 1 \
  --models qwen/qwen3.7-plus,moonshotai/kimi-k2.6,deepseek/deepseek-v4-pro,openai/gpt-5.5,fusion-cn-budget \
  --out tools/fusion-benchmark/runs/fresh-v1-diff-balanced-1.jsonl
```

`--per-category-limit` is applied per difficulty and category when the dataset has a `difficulty` field. Use `--difficulty simple` for simple-only smoke, `--difficulty hard` for harder separation runs, or omit it for mixed simple+hard.

Run the v2 non-code pilot. `fresh-eval.v2-pilot.jsonl` contains 88 objective
hard/very-hard tasks: 30 reasoning, 18 Chinese, 18 English, 10 long-context, and
12 instruction-following rows. Each row has `module`, `source_family`, and
`visibility` metadata for authority/source splits in reports. The corrected v3
pilot evidence has only run the first 30 non-code rows; the new non-code rows
still need a paid model run before they count as evidence.

For Fusion presets, objective rows (`exact`, `contains`, `regex`) use Objective
Fusion: panel answers are scored locally, passing candidates are grouped, and a
passing candidate is returned directly instead of being rewritten by final
synthesis. The record keeps `fusion_metrics.policy=objective` and the selected
underlying model.

```bash
node tools/fusion-benchmark.mjs fresh-validate \
  --dataset tools/fusion-benchmark/data/fresh-eval.v2-pilot.jsonl

node tools/fusion-benchmark.mjs fresh-run \
  --env-file tools/fusion-benchmark/.env.local \
  --dataset tools/fusion-benchmark/data/fresh-eval.v2-pilot.jsonl \
  --models fusion-cn-budget,openai/gpt-5.5,deepseek/deepseek-v4-pro,qwen/qwen3.7-plus,moonshotai/kimi-k2.6,minimax/minimax-m3 \
  --out tools/fusion-benchmark/runs/fresh-v2-pilot.jsonl
```

Validate and run executable coding tasks:

```bash
node tools/fusion-benchmark.mjs code-validate \
  --dataset tools/fusion-benchmark/data/code-eval.example.jsonl

node tools/fusion-benchmark.mjs code-run \
  --env-file tools/fusion-benchmark/.env.local \
  --dataset tools/fusion-benchmark/data/code-eval.example.jsonl \
  --models fusion-cn-budget,openai/gpt-5.5,qwen/qwen3.7-plus \
  --request-timeout-ms 120000 \
  --out tools/fusion-benchmark/runs/code-example.jsonl
```

Run the current hard coding set. `code-eval.v1.jsonl` now has 53 runnable
tasks: 44 `code_exec` tasks, 9 `patch_exec` tasks, and 8 `very_hard` rows.
The corrected v3 pilot report has only run the first 20 of these; the new
coding tasks still need a paid model run before they count as evidence.

```bash
node tools/fusion-benchmark.mjs code-validate \
  --dataset tools/fusion-benchmark/data/code-eval.v1.jsonl

node tools/fusion-benchmark.mjs code-run \
  --env-file tools/fusion-benchmark/.env.local \
  --dataset tools/fusion-benchmark/data/code-eval.v1.jsonl \
  --models fusion-cn-budget \
  --request-timeout-ms 180000 \
  --out tools/fusion-benchmark/runs/code-v1-fusion-complete.jsonl
```

`code-run` executes model outputs instead of judging them by text. `code_exec`
tasks write the extracted implementation into a temporary workspace and run the
configured test command. `patch_exec` tasks copy a fixture repository, apply the
model's unified diff with `git apply`, and then run the fixture tests. Per-task
execution logs are written next to the JSONL output under an ignored artifacts
directory.

For Fusion presets, `code-run` uses bounded Verified Coding Fusion: it runs each
panel candidate once in an isolated workspace, selects a candidate that passes
the configured tests, and returns that candidate directly. If all primary panel
calls fail, or if no primary candidate passes the verifier, the runner may add a
single explicitly configured fallback panel candidate and verify it the same
way. If no candidate passes after that bounded step, the runner falls back to the
normal panel/judge/final synthesis path. This is verification and selection
only; it does not explore the repository, write to the user's project, use
GPT-5.5 as an internal fallback, or run a repair loop.

Use `--request-timeout-ms` as the per-upstream-call timeout. For ordinary
baseline models this is the model request timeout; for Fusion presets this is the
timeout for each panel, judge, and final synthesis call. Official Fusion scoring
runs are never truncated by `--overall-request-timeout-ms`: Fusion is a
multi-model pipeline, so the runner lets the whole task finish and records the
full `latency_ms`. Use `--fusion-overall-request-timeout-ms` only for smoke tests
or operational diagnostics where an intentionally truncated Fusion run is
acceptable. Local test timeouts are still controlled per task by `timeout_ms`.

The example dataset includes both supported executable task types:

- `code_exec`: extracts a single-file implementation, writes it to `solution.py`,
  and runs the configured Python `unittest` command.
- `patch_exec`: extracts a unified diff, applies it to a fixture repository, and
  runs `node --test`.

Generate a report:

```bash
node tools/fusion-benchmark.mjs report \
  --inputs tools/fusion-benchmark/runs/livebench-2026-01-08.jsonl,tools/fusion-benchmark/runs/proxy-requests.jsonl,tools/fusion-benchmark/runs/pairwise-example.jsonl \
  --out tools/fusion-benchmark/reports/fresh-example.md
```

Fusion reports separate two billing views:

- Estimated cost: calculated from `config.json` token pricing and response `usage`.
- Actual relay bill: extracted only from relay response body/header fields. If the relay does not return a bill field, the value stays blank and is not filled with an estimate.

Reports also include solved rate, p50/p95 latency, early-exit rate, panel max
latency, judge latency, final latency, judge JSON validity, panel failure
rates, Wilson confidence intervals, severe-error counts, and Fusion policy
splits when the input records contain Fusion metrics. Capability scores are
computed on valid samples, excluding provider/network failures and upstream
request timeouts; `raw_score` keeps those failures as zero so reliability
remains visible. Local code-test timeouts remain model failures and are not
excluded. Formal claims should rerun provider-error and request-timeout rows or
mark the affected comparison incomplete. Main Fusion verdicts and leaderboard
tables use only Fusion, the reliable strongest non-Fusion baseline in the same
run, and the per-question single-model oracle used for anti-degradation checks.
Weaker or unreliable single models remain diagnostics and router-learning
signals only.
Reports also split `public_anchor`, `private_isomorphic`, and
`business_realistic` rows when datasets include `visibility`, plus a
`source_family` split for benchmark-family diagnostics.

Current v3 corrected pilot artifacts:

- `runs/code-v3-pilot-fusion-corrected.jsonl`: 20/20 coding tasks pass after
  fixing the `filter_rows` prompt ambiguity and rerunning the `sliding_top_k`
  row.
- `runs/fusion-v3-merged-comparison-corrected.jsonl`: 50-task corrected merge.
- `reports/fusion-v3-merged-comparison-corrected.md`: corrected report showing
  `fusion-cn-budget` at 100.0%, a stronger point estimate than the reliable
  strongest single baseline, zero degradation versus the per-question
  strongest-single oracle on comparable rows, and no GPT-5.5 losses on valid
  GPT-5.5 comparison rows. The 50-task pilot is evidence to expand, not a final
  public superiority claim.

Check whether a merged JSONL is pilot-, architecture-, or public-claim-ready:

```bash
node tools/fusion-benchmark.mjs production-gate \
  --inputs tools/fusion-benchmark/runs/fusion-v3-merged-comparison-corrected.jsonl \
  --target fusion-cn-budget
```

The current corrected pilot returns `pilot_ready_expand`; it does not pass the
100-question architecture gate or the 141-question public-claim gate.

Check pinned dataset coverage separately from already-run evidence:

```bash
node tools/fusion-benchmark.mjs run-inventory \
  --inputs tools/fusion-benchmark/data/fresh-eval.v2-pilot.jsonl,tools/fusion-benchmark/data/code-eval.v1.jsonl
```

Current pinned dataset coverage is 141/141: coding 53/53, reasoning 30/30,
Chinese 18/18, English 18/18, long-context 10/10, and
instruction-following 12/12. These extra rows are inventory, not evidence,
until the paid model run is completed.

Run pairwise open-ended eval:

```bash
node tools/fusion-benchmark.mjs pairwise-run \
  --api-base http://127.0.0.1:8787/v1 \
  --env-file tools/fusion-benchmark/.env.local \
  --dataset tools/fusion-benchmark/data/pairwise-eval.example.jsonl \
  --target fusion-cn-budget \
  --baseline openai/gpt-5.5 \
  --judge anthropic/claude-sonnet-latest \
  --out tools/fusion-benchmark/runs/pairwise-example.jsonl
```

## Fresh Eval Dataset Format

Each JSONL row must contain:

```json
{
  "id": "unique-question-id",
  "module": "reasoning",
  "category": "reasoning",
  "difficulty": "hard",
  "source_family": "gpqa_style_private",
  "visibility": "private_isomorphic",
  "prompt": "Question text",
  "scoring": { "type": "exact", "answer": "42" },
  "max_tokens": 512
}
```

Supported scoring types:

- `exact`: normalized exact match.
- `contains`: normalized substring match.
- `regex`: JavaScript regular expression.

The included `fresh-eval.example.jsonl` is only a smoke dataset. The real spike should replace it with 100-300 recent, objective questions.

For v2-style reports, set `source_family` and `visibility` on every row.
Supported visibility values are `public_anchor`, `private_isomorphic`, and
`business_realistic`. Public leaderboard originals should stay a small authority
anchor; private isomorphic rows are the main anti-contamination scoring set.

Use `fresh-validate` before a paid run:

```bash
node tools/fusion-benchmark.mjs fresh-validate \
  --dataset tools/fusion-benchmark/data/fresh-eval.v1-pilot.jsonl
```

## Pairwise Dataset Format

Each JSONL row must contain:

```json
{
  "id": "open-question-id",
  "category": "architecture",
  "prompt": "Open-ended task",
  "rubric": "What a good answer should do"
}
```

The runner randomizes A/B order and asks a third-party judge for strict JSON. Do not use `openai/gpt-5.5` as the judge when GPT-5.5 is one of the compared models.

## Secret Handling

API keys and upstream base URLs belong in `tools/fusion-benchmark/.env.local`, which is ignored by git. The CLI reads it with `--env-file`; it does not print keys or base URLs, write them to request logs, or include them in reports.

Do not paste API keys or private relay base URLs into chat. Create and inspect the file only on your machine:

```bash
cp tools/fusion-benchmark/.env.example tools/fusion-benchmark/.env.local
chmod 600 tools/fusion-benchmark/.env.local
$EDITOR tools/fusion-benchmark/.env.local
```

After editing, run `validate-config` and check that each upstream you plan to use has `base found` and `key found`. It will never echo actual key or base URL values.

Expected local env shape:

```bash
OPENROUTER_API_BASE=https://your-relay.example/v1
OPENROUTER_API_KEY=your-local-key

QWEN_API_BASE=https://your-qwen-route.example/v1
QWEN_API_KEY=your-local-key

MOONSHOT_API_BASE=https://your-moonshot-route.example/v1
MOONSHOT_API_KEY=your-local-key

DEEPSEEK_API_BASE=https://your-deepseek-route.example/v1
DEEPSEEK_API_KEY=your-local-key

MINIMAX_API_BASE=https://your-minimax-route.example/v1
MINIMAX_API_KEY=your-local-key
```

The base URL can also be supplied as `OPENAI_BASE_URL` or `OPENAI_API_BASE` if your relay already uses those names.

If the relay uses model IDs such as `gpt-5.5` instead of canonical report names such as `openai/gpt-5.5`, add mappings in `config.json` under `modelAliases`. Keep the left side as the benchmark/report model name and the right side as the upstream relay model ID.

Domestic models are routed by `config.json` under `modelUpstreams`. For example, `qwen/qwen3.7-plus` uses the `qwen` upstream, which reads `QWEN_API_BASE` and `QWEN_API_KEY`. Probe each upstream separately:

```bash
node tools/fusion-benchmark.mjs models-probe --env-file tools/fusion-benchmark/.env.local --upstream qwen --filter qwen
node tools/fusion-benchmark.mjs models-probe --env-file tools/fusion-benchmark/.env.local --upstream moonshot --filter 'kimi|moonshot'
node tools/fusion-benchmark.mjs models-probe --env-file tools/fusion-benchmark/.env.local --upstream deepseek --filter deepseek
node tools/fusion-benchmark.mjs models-probe --env-file tools/fusion-benchmark/.env.local --upstream minimax --filter minimax
```

Some relay models need extra request options. Put those under `modelOptions` in `config.json`; for example, Qwen thinking models may return only `reasoning_content` unless `enable_thinking` is disabled for short-answer benchmarks.

## Billing Sources

The benchmark writes both `estimated_cost_usd` and `actual_cost_usd` to JSONL records. `cost_usd` is kept as a backward-compatible alias for estimated cost. The field name `actual_cost_usd` means "relay-attributed cost or account-balance burn"; it is not automatically the same as cash paid.

Domestic model billing should be treated as relay billing only. Do not use public provider pricing as the real bill for Qwen, Kimi, DeepSeek, or MiniMax. The tool extracts relay bills from common response fields such as `usage.cost_usd`, `billing.cost_usd`, `usage.quota`, and common cost/quota headers. If your relay uses a custom field or header, add it to `billing.actualCostSources`:

```json
{
  "billing": {
    "quotaPerUsd": 500000,
    "actualCostSources": [
      { "type": "body", "path": "usage.cost_usd", "unit": "usd" },
      { "type": "header", "name": "x-oneapi-used-quota", "unit": "quota" }
    ]
  }
}
```

Supported units are `usd`, `quota`, and `cny`. `quota` is converted through `billing.quotaPerUsd`; `cny` requires `billing.cnyPerUsd`.

SiliconFlow-specific finding: the public docs point usage bills to the console page `https://cloud.siliconflow.cn/bills`. The chat completion response only exposes token `usage`, not per-request money. The API endpoint `GET /v1/user/info` does expose account balance fields such as `data.balance`, `data.chargeBalance`, and `data.totalBalance`, so the benchmark can optionally use before/after balance delta as an account-deduction signal:

- Use `data.chargeBalance` to approximate paid/recharged balance consumption.
- Use `data.totalBalance` to measure total account credit consumption, including vouchers or promotional balance.
- If calls are paid by vouchers, `data.totalBalance` may decrease while real cash consumption is zero. In that case, do not call total-balance delta "cash cost"; keep it as voucher/account-credit burn.

```json
{
  "billing": {
    "cnyPerUsd": 7.25,
    "balanceDelta": {
      "enabled": false,
      "path": "/user/info",
      "balancePath": "data.chargeBalance",
      "unit": "cny",
      "upstreams": ["qwen", "moonshot", "deepseek", "minimax"]
    },
    "runBalanceDelta": {
      "enabled": true,
      "path": "/user/info",
      "balancePath": "data.chargeBalance",
      "unit": "cny",
      "upstreams": ["qwen", "moonshot", "deepseek", "minimax"]
    }
  }
}
```

Keep `balanceDelta.enabled` off for normal runs. When enabled, requests sharing the same upstream account are serialized so the before/after balance delta is attributable to one request.

For SiliconFlow, prefer `runBalanceDelta.enabled=true`. `fresh-run` and `pairwise-run` write `billing_summary` records at the end of the JSONL output. These records contain only the account-balance delta per upstream, not the raw before/after balance values. The report shows them in a separate "中转站账户扣减汇总" table.

Batch balance deltas are account-level account-credit burns. They are useful for checking the whole domestic run against relay/account consumption, but they cannot be directly attributed to one baseline model or one Fusion preset unless the run isolates that model/preset in a separate batch. The model summary table only shows per-request attributed cost when the relay returns attributable body/header billing fields.

Fusion presets can use conservative early exit:

```json
"earlyExit": {
  "strategy": "exact_majority",
  "minAgree": 2,
  "maxAnswerChars": 200
}
```

This skips judge/final only when enough panel models return the same short normalized answer. The report includes `early_exit_rate` and panel/judge/final latency columns so the optimization is visible.

Some upstreams need provider-specific request flags for fair benchmark behavior. Add those under `modelOptions` in `config.json`; the harness merges them into passthrough, panel, judge, final, and benchmark calls before replacing the public model name with the upstream model ID. For example, Qwen thinking mode can be disabled without changing benchmark request bodies:

```json
{
  "modelOptions": {
    "qwen/qwen3.6-flash": {
      "enable_thinking": false
    }
  }
}
```

Fusion presets can also define an `earlyExit` rule for objective short-answer runs. The `exact_majority` strategy skips the judge and final synthesizer when enough panel models return the same normalized answer under the configured length limit:

```json
{
  "fusionPresets": {
    "fusion-cn-budget": {
      "earlyExit": {
        "strategy": "exact_majority",
        "minAgree": 2,
        "maxAnswerChars": 200
      }
    }
  }
}
```

Early exits are reported in `fusion_metrics.early_exit`, and the skipped judge/final stages remain visible in the metrics object.

## What You Need To Do

1. Put the required upstream base URLs and keys into `tools/fusion-benchmark/.env.local`.
2. Run `validate-config`, then run `models-list`, `models-probe`, and `billing-probe` per upstream.
3. Start the local proxy with `serve`.
4. Clone or update LiveBench separately and run the commands printed by `livebench-plan`.
5. Import LiveBench `ground_truth_judgment.jsonl` files with `livebench-import`.
6. Run `pairwise-run` on the fresh open-ended dataset.
7. Generate the final Markdown report with `report`.

## LiveBench Release Policy

Use the newest available release first:

1. `2026-01-08`
2. `2025-11-25`
3. `2025-04-25`

Keep `2024-11-25` only as a public anchor for historical reproducibility.

LiveBench scoring writes `ground_truth_judgment.jsonl` files under its `data/` tree. Use `livebench-import` on either those files directly or the root data directory.
