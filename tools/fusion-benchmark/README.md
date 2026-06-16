# Fusion Benchmark Spike

This harness validates whether Chinese-model Fusion presets are worth turning into a production API.

It provides three pieces:

- An OpenAI-compatible Fusion proxy for LiveBench and other API-based benchmarks.
- A small objective fresh-eval runner for local JSONL datasets.
- A Markdown report generator for score, cost, latency, degradation, and GPT-5.5 comparison.

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
node tools/fusion-benchmark.mjs fresh-run \
  --api-base http://127.0.0.1:8787/v1 \
  --env-file tools/fusion-benchmark/.env.local \
  --dataset tools/fusion-benchmark/data/fresh-eval.example.jsonl \
  --category coding,data_analysis \
  --limit 20 \
  --out tools/fusion-benchmark/runs/fresh-example.jsonl
```

Omit `--category` and `--limit` for a full run. Use `--offset` with `--limit` to run a larger fresh dataset in batches.

Generate a report:

```bash
node tools/fusion-benchmark.mjs report \
  --inputs tools/fusion-benchmark/runs/livebench-2026-01-08.jsonl,tools/fusion-benchmark/runs/proxy-requests.jsonl,tools/fusion-benchmark/runs/pairwise-example.jsonl \
  --out tools/fusion-benchmark/reports/fresh-example.md
```

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
  "category": "reasoning",
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

## What You Need To Do

1. Put the required upstream base URLs and keys into `tools/fusion-benchmark/.env.local`.
2. Run `validate-config`, then run `models-list` and `models-probe` per upstream.
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
