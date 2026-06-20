# Fusion Model Evaluation Plan

This plan defines how to compare `fusion-cn-budget` with single models and other
Fusion presets. The main leaderboard ranks capability only. Cost, latency, and
operational success rate are reported as secondary diagnostics and do not affect
the main capability rank.

## Goals

1. Produce an authoritative leaderboard that is comparable to public LLM rankings.
2. Include runnable coding evaluations, with extra weight on hard coding and real
   engineering tasks.
3. Test both English and Chinese capability instead of treating this as a
   Chinese-only benchmark.
4. Keep public benchmark results separate from private fresh tasks to reduce
   benchmark contamination risk.
5. Record cost, latency, timeout, parse failures, and execution failures for
   operational analysis, not as main-ranking inputs.

## Leaderboards

### Main capability leaderboards

These leaderboards use correctness or rubric scores only.

| Leaderboard | Purpose |
| --- | --- |
| Overall Capability | Weighted score across all capability modules. |
| Coding Capability | All runnable coding tasks, including algorithms and repo tasks. |
| Hard Coding | Hard/very-hard algorithm, engineering, and repo-level tasks only. |
| High-Difficulty Reasoning | Math, science, logic, and multi-step reasoning. |
| English Capability | English reading, writing, instruction following, extraction, and technical tasks. |
| Chinese Capability | Chinese reading, writing, instruction following, extraction, and business tasks. |
| Long Context | Long-document retrieval, synthesis, conflict detection, and evidence grounding. |
| Instruction Following | JSON/schema/format constraints and multi-condition compliance. |

### Secondary diagnostic leaderboards

These are useful for product decisions but should be labeled as secondary.

| Diagnostic | Meaning |
| --- | --- |
| Cost | Estimated and relay-actual spend per run and per 100 tasks. |
| Score per USD | Overall score divided by cost. Informational only. |
| Latency | Mean, p50, p95, timeout rate. |
| Success Rate | Non-timeout, non-error, parseable-output rate. |
| Stability | JSON validity, code extraction validity, execution setup success. |
| Token Efficiency | Tokens used per solved task. |

## Main Weighting

| Module | Weight | Scoring mode |
| --- | ---: | --- |
| Runnable coding | 40% | Program execution, tests, and patch validation. |
| High-difficulty reasoning | 20% | Exact answer, numeric answer, or strict rubric. |
| English capability | 12.5% | Objective checks plus pairwise/rubric tasks. |
| Chinese capability | 12.5% | Objective checks plus pairwise/rubric tasks. |
| Long context | 10% | Evidence-grounded exact/rubric scoring. |
| Instruction following | 5% | Parser/schema/regex/exact validation. |

The main score should be reported as:

```text
overall_score =
  0.40 * coding_score +
  0.20 * reasoning_score +
  0.125 * english_score +
  0.125 * chinese_score +
  0.10 * long_context_score +
  0.05 * instruction_score
```

## Coding Evaluation

Coding must be executed. Text-only judgment is not acceptable for the coding
leaderboards.

### Coding weight breakdown

| Submodule | Overall weight | Coding weight | Purpose |
| --- | ---: | ---: | --- |
| Function-level coding | 8% | 20% | Basic code generation and edge cases. |
| Hard algorithm coding | 10% | 25% | Differentiates strong models from competent models. |
| Very-hard coding | 6% | 15% | Deep algorithmic reasoning and implementation precision. |
| Engineering coding | 10% | 25% | Modify a small project and pass tests. |
| Repo-level tasks | 4% | 10% | Multi-file issue repair in realistic repositories. |
| Debug/code understanding | 2% | 5% | Root-cause analysis and minimal fixes. |

### Authoritative benchmark alignment

| Benchmark family | Use |
| --- | --- |
| LiveCodeBench | Primary public signal for fresh coding tasks. |
| HumanEval+ / EvalPlus | Function-level correctness with stronger hidden tests. |
| MBPP+ / EvalPlus | Basic Python programming and edge cases. |
| BigCodeBench | Function and library usage with richer APIs. |
| SWE-bench Verified | Real issue repair; public authority anchor. |
| Terminal-Bench | Terminal/tool-use style engineering tasks. |

### Private hard coding set

The private coding set should avoid copied public problems. It can follow public
benchmark structure but must use new problem statements, hidden tests, and fresh
edge cases.

Recommended categories:

| Category | Examples | Required checks |
| --- | --- | --- |
| Dynamic programming | State compression, interval DP, optimized transitions. | Correctness and large-input performance. |
| Graph algorithms | Shortest path variants, topological constraints, flow/matching style tasks. | Hidden corner cases and timeout. |
| Data structures | Segment tree/Fenwick/tree maps/heap invariants. | Large random tests and adversarial sequences. |
| String algorithms | Parsing, trie, rolling hash, KMP-like matching. | Unicode/ASCII boundaries where relevant. |
| Combinatorics | Counting under modulo, inclusion-exclusion, constructive proofs. | Exact answers and overflow/modulo handling. |
| Parsers/DSL | Query evaluator, expression parser, small config language. | Invalid input and precedence tests. |
| Systems-ish tasks | Cache invalidation, retries, idempotency, async races. | Deterministic tests and concurrency simulations. |

### Coding task formats

#### Function task

The model receives a signature, constraints, and examples. The harness extracts
the submitted code and runs hidden tests.

```json
{
  "id": "code-hard-dp-001",
  "category": "coding_algorithm",
  "difficulty": "hard",
  "language": "python",
  "entrypoint": "solve",
  "prompt": "Implement solve(events: list[tuple[int,int,int]]) -> int ...",
  "scoring": {
    "type": "code_exec",
    "runner": "python-pytest",
    "tests": "tests/code-hard-dp-001"
  },
  "timeout_ms": 10000,
  "max_tokens": 4096
}
```

#### Engineering task

The model receives a small repository snapshot, a failing test, or an issue
description. The harness applies the patch and runs tests.

```json
{
  "id": "eng-cache-ttl-001",
  "category": "coding_engineering",
  "difficulty": "hard",
  "repo": "fixtures/eng-cache-ttl-001",
  "prompt": "Fix the stale-cache behavior. Preserve the public API.",
  "scoring": {
    "type": "patch_exec",
    "test_command": "npm test",
    "review_required": true
  },
  "timeout_ms": 60000,
  "max_tokens": 8192
}
```

#### Repo-level task

Use a pinned repository fixture and issue text. The harness should run the full
test command plus task-specific regression tests.

```json
{
  "id": "repo-cli-config-001",
  "category": "coding_repo",
  "difficulty": "very_hard",
  "repo": "fixtures/repo-cli-config-001",
  "prompt": "The CLI ignores --config when the env var is present. Fix it.",
  "scoring": {
    "type": "patch_exec",
    "test_command": "npm test && npm run test:regression",
    "review_required": true
  },
  "timeout_ms": 120000,
  "max_tokens": 12000
}
```

### Coding scoring rules

1. Main coding metric is `pass@1`.
2. Public examples do not count toward score.
3. Hidden tests must include normal, edge, randomized, and performance cases.
4. Timeout, syntax error, test failure, and invalid patch all score 0.
5. Engineering and repo tasks should also record patch size and touched files.
6. Human review is used only to catch reward hacking or unacceptable patches;
   the primary signal remains test execution.

## Non-Coding Evaluation

### High-difficulty reasoning

Authority anchors:

| Benchmark family | Use |
| --- | --- |
| GPQA Diamond | Graduate-level science reasoning. |
| MMLU-Pro | Harder multi-choice knowledge and reasoning. |
| AIME / AMC / MATH-500 | Math reasoning with objective answers. |
| OlympiadBench | Olympiad-style math and science. |

Private tasks should emphasize multi-step reasoning, not trivia. Prefer exact
answers, numeric answers, or multiple choice with strict answer extraction.

### English capability

Test English as a first-class capability.

| Task type | Scoring |
| --- | --- |
| Technical document QA | Exact/rubric with evidence requirement. |
| English summarization | Rubric plus factual consistency checks. |
| Structured extraction | JSON schema validation. |
| Professional writing/editing | Pairwise judge plus human audit sample. |
| Complex instruction following | Exact/schema/regex validation. |

Authority anchors: HELM-style tasks, MT-Bench style open prompts, IFEval,
LongBench English subsets, MMLU-Pro language/domain tasks.

### Chinese capability

Chinese tasks should be realistic, not just translation.

| Task type | Scoring |
| --- | --- |
| Business document extraction | JSON schema and field matching. |
| Contract/notice/meeting-note QA | Exact/rubric with evidence. |
| Chinese summarization | Rubric plus factual consistency checks. |
| Customer-service reasoning | Exact answer or decision rubric. |
| Chinese writing/editing | Pairwise judge plus human audit sample. |

Authority anchors: C-Eval, CMMLU, SuperCLUE, OpenCompass Chinese task families.

### Long context

Long-context tasks should include both English and Chinese sources.

| Task type | Scoring |
| --- | --- |
| Needle retrieval | Exact answer and cited evidence location. |
| Multi-document synthesis | Rubric with required evidence. |
| Conflict detection | Exact conflicting fields and source IDs. |
| Log/config analysis | Exact root cause or required change. |

Authority anchors: LongBench, Needle-in-a-Haystack, RULER-style retrieval, and
HELM-style long-form tasks.

### Instruction following

This module should be mostly automatic.

| Task type | Scoring |
| --- | --- |
| Strict JSON | JSON parse and schema validation. |
| Field ordering/format | Exact or regex validation. |
| Multi-condition output | Programmatic validators. |
| Refusal boundaries | Rubric plus safety classification when relevant. |

Authority anchor: IFEval.

## Dataset Sizes

### Full benchmark

| Module | Count |
| --- | ---: |
| Function-level coding | 40 |
| Hard algorithm coding | 30 |
| Very-hard coding | 15 |
| Engineering coding | 25 |
| Repo-level tasks | 8 |
| Debug/code understanding | 12 |
| High-difficulty reasoning | 80 |
| English capability | 40 |
| Chinese capability | 40 |
| Long context | 25 |
| Instruction following | 30 |

Target total: 345 tasks.

### First production-quality version

| Module | Count |
| --- | ---: |
| Function-level coding | 15 |
| Hard algorithm coding | 15 |
| Very-hard coding | 5 |
| Engineering coding | 10 |
| Repo-level tasks | 3 |
| Debug/code understanding | 5 |
| High-difficulty reasoning | 30 |
| English capability | 18 |
| Chinese capability | 18 |
| Long context | 10 |
| Instruction following | 12 |

Target total: 141 tasks.

### Cheap smoke version

| Module | Count |
| --- | ---: |
| Runnable coding | 8 |
| High-difficulty reasoning | 8 |
| English capability | 5 |
| Chinese capability | 5 |
| Long context | 3 |
| Instruction following | 3 |

Target total: 32 tasks.

## Reporting Requirements

Each task result should include:

| Field | Meaning |
| --- | --- |
| `run_id` | Unique benchmark run ID. |
| `task_id` | Stable task ID. |
| `model` | Model or Fusion preset name. |
| `module` | Main scoring module. |
| `category` | Fine-grained category. |
| `difficulty` | `simple`, `medium`, `hard`, or `very_hard`. |
| `score` | Capability score, usually 0 or 1. |
| `passed` | Boolean pass/fail when applicable. |
| `latency_ms` | End-to-end latency. |
| `prompt_tokens` | Prompt tokens from API usage when available. |
| `completion_tokens` | Completion tokens from API usage when available. |
| `estimated_cost_usd` | Config-price estimate. |
| `actual_cost_usd` | Relay-attributed cost or configured account-balance burn when available; if vouchers are used, this may be account-credit consumption rather than cash spend. |
| `failure_type` | Timeout, API error, parse error, test failure, etc. |
| `execution_log` | Path to test output for coding tasks. |

Reports should show public and private scores separately, then a combined score.
This makes the leaderboard more credible because readers can see whether a model
is strong only on public benchmarks or also on fresh private tasks.

## Implementation Roadmap

### Phase 1: current harness hardening

1. Keep using `fresh-run` for objective non-code tasks.
2. Keep using `pairwise-run` for open-ended English/Chinese tasks.
3. Keep importing LiveBench as a public authority anchor.
4. Add module labels to datasets so reports can aggregate by main weighting.

### Phase 2: runnable coding harness

Add a new command, tentatively:

```bash
node tools/fusion-benchmark.mjs code-run \
  --dataset tools/fusion-benchmark/data/code-eval.v1.jsonl \
  --models fusion-cn-budget,openai/gpt-5.5,qwen/qwen3.7-plus \
  --out tools/fusion-benchmark/runs/code-v1.jsonl
```

Required behavior:

1. Create a temp workspace per model/task.
2. Ask the model for code or a patch.
3. Extract code or patch with strict parsing rules.
4. Run the configured tests with a timeout.
5. Capture stdout/stderr and write an execution artifact path.
6. Score pass/fail from process exit code and optional validators.

### Phase 3: engineering/repo fixtures

1. Add `tools/fusion-benchmark/fixtures/` for pinned coding projects.
2. Keep each fixture small enough for repeated runs.
3. Include hidden regression tests outside the model-visible prompt.
4. Add patch review sampling for repo-level tasks.

### Phase 4: full reporting

1. Add weighted overall score.
2. Add coding and hard-coding leaderboards.
3. Add English and Chinese capability leaderboards.
4. Add secondary cost/latency/success-rate sections.
5. Add public/private split tables.

## Initial Model Set

The first comparison should include:

| Type | Models |
| --- | --- |
| Fusion target | `fusion-cn-budget` |
| Fusion variants | `fusion-cn-cheap`, `fusion-cn-alt`, `fusion-cn-stable` |
| Chinese single baselines | `qwen/qwen3.7-plus`, `moonshotai/kimi-k2.6`, `deepseek/deepseek-v4-pro`, `minimax/minimax-m3` |
| Premium anchor | `openai/gpt-5.5` |
| Optional judge anchor | `anthropic/claude-sonnet-latest` |

## Acceptance Criteria

The benchmark is ready to drive product decisions when:

1. At least 100 capability tasks are runnable end to end.
2. At least 30 tasks are true code-execution tasks.
3. At least 10 tasks are hard or very-hard coding tasks.
4. Public benchmark scores and private fresh scores are reported separately.
5. The report includes overall, coding, hard-coding, English, Chinese, and
   secondary diagnostic leaderboards.
6. Every coding score is backed by test execution logs.
7. Cost and latency are visible but not mixed into the main capability score.
