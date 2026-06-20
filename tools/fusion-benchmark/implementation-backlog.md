# Fusion Benchmark Implementation Backlog

This backlog turns `evaluation-plan.md` into implementation work.

## Milestone 0: v2 rollout alignment

Use `v2-evaluation-rollout.md` as the concrete plan for the next run. The v2
benchmark should use public leaderboard questions only as a small authority
anchor, with private isomorphic tasks as the main scoring set.

Done when:

- Done: coding dataset rows have `source_family` and `visibility`.
- Done: reports split `public_anchor`, `private_isomorphic`, and
  `business_realistic` scores when metadata exists.
- Done: provider/network failures are separated from model capability failures.
- Done: Fusion code-run records are not truncated by ordinary
  `--overall-request-timeout-ms`.
- Done: coding dataset has 53 runnable tasks: 44 `code_exec`, 9 `patch_exec`,
  and 8 `very_hard`. The corrected v3 pilot evidence still covers the first 20.
- Done: non-code pilot has 88 objective tasks: 30 reasoning, 18 Chinese,
  18 English, 10 long-context, and 12 instruction-following.

## Milestone 1: authoritative non-code baseline

1. Done: add `module` to v2 fresh-eval dataset rows.
2. Done: `fresh-run` writes `module`, and reports aggregate `module + category`.
3. Keep cost, latency, and success-rate tables secondary.
4. Import LiveBench results as the first public authority anchor.
5. Done: build private English and Chinese objective tasks with exact/regex
   scoring.

Done when:

- `fresh-run` can produce module-level scores.
- Report shows Overall, Reasoning, English, Chinese, Long Context, and
  Instruction Following sections.

## Milestone 2: runnable coding MVP

Add a new command:

```bash
node tools/fusion-benchmark.mjs code-run \
  --dataset tools/fusion-benchmark/data/code-eval.example.jsonl \
  --models fusion-cn-budget,openai/gpt-5.5,qwen/qwen3.7-plus \
  --out tools/fusion-benchmark/runs/code-example.jsonl
```

Minimum supported task types:

| Type | Purpose | Runner |
| --- | --- | --- |
| `code_exec` | Function-level coding and hard algorithms. | Python unittest/pytest. |
| `patch_exec` | Engineering/repo tasks. | Shell test command in isolated fixture copy. |

MVP behavior:

1. Create a temp directory per task/model.
2. Write prompt and model response to artifacts.
3. Extract fenced code for `code_exec`.
4. Apply unified diff for `patch_exec`.
5. Run the configured command with timeout.
6. Write JSONL records with `score`, `passed`, `failure_type`,
   `execution_log`, `latency_ms`, token usage, and cost fields.

Done when:

- At least one Python function task is executed by unittest or pytest.
- At least one fixture patch task runs a real test command.
- Failed tests are scored as 0 and include an execution log path.

## Milestone 3: hard coding set v1

Create `tools/fusion-benchmark/data/code-eval.v1.jsonl` and hidden tests.

Target first version:

| Coding area | Count |
| --- | ---: |
| Function-level medium | 8 |
| Hard algorithm | 10 |
| Very-hard algorithm/parser | 4 |
| Engineering patch | 6 |
| Repo-level task | 2 |

Required hard/very-hard coverage:

1. Optimized dynamic programming.
2. Graph constraints.
3. Stateful data structure.
4. Parser or DSL evaluator.
5. Large-input performance.
6. Engineering cache/config/async bug.

Done when:

- At least 30 runnable coding tasks exist.
- At least 14 are hard or very-hard.
- All tests can run locally without network access.

## Milestone 4: weighted report

Add report sections:

1. Overall Capability leaderboard.
2. Coding Capability leaderboard.
3. Hard Coding leaderboard.
4. Public vs Private split.
5. English and Chinese capability leaderboards.
6. Secondary diagnostics: cost, score per USD, p50/p95 latency, success rate.

Done when:

- Main leaderboard does not include cost or latency in score.
- Secondary tables are clearly labeled as diagnostics.
- Public and private benchmark rows are separated before combined scoring.

## Milestone 5: production run

First production-quality run target:

| Module | Count |
| --- | ---: |
| Runnable coding | 53 |
| High-difficulty reasoning | 30 |
| English capability | 18 |
| Chinese capability | 18 |
| Long context | 10 |
| Instruction following | 12 |

Done when:

- At least 141 total tasks run end to end.
- Every coding result has a test log.
- Results are reproducible with pinned dataset and fixture versions.

## Milestone 6: Fusion v3 model-like architecture

Use `fusion-v3-roadmap.md` as the implementation plan. The goal is no longer
only "beat GPT-5.5 on a pilot." Fusion should become a stronger model-like
virtual model in the same cost/latency tier by using diversity, selection,
verification, and anti-degradation guards without becoming an open-ended agent.

Boundary:

1. Default Fusion is not agentic: no autonomous repo exploration, no unbounded
   repair loop, and no writes to the user's real project.
2. Verified modes may run local objective scorers or sandbox tests over
   generated candidates.
3. Reports must separate `fusion_model_mode`, `verified_fusion_mode`, and
   `agentic_mode`.

Done when:

- [done] Fusion records include `task_family`, `policy`, `selected_model`,
  `selection_reason`, `candidate_count`, `passing_candidate_count`,
  `anti_degradation_guard`, and `repair_used`.
- [done] Reports split score, cost, latency, and verifier pass-through metrics by
  policy.
- [done] `fusion-cn-fast`, `fusion-cn-budget`, and `fusion-cn-pro` presets have
  explicit cost/latency/capability targets.
- [done] Single-model comparison uses only the reliable strongest baseline and
  the per-question strongest-single oracle; dominated or unreliable single
  models stay in diagnostics only.

## Milestone 7: Objective Fusion

Implement deterministic selection for exact, regex, numeric, multiple-choice,
and schema-like rows.

Required behavior:

1. Normalize short answers before comparison.
2. Group equivalent answers and early-return a valid majority.
3. Use constrained final formatting only when necessary.
4. Record whether final changed the selected answer.

Done when:

- [done] Objective Fusion never lowers pilot exact/regex score relative to the
  best panel candidate on the same row.
- [done] Entity-format mistakes such as returning a versioned service name when
  the expected entity is unversioned are eliminated from the pilot.

## Milestone 8: Verified Coding Fusion

Implement bounded candidate verification for coding tasks without turning Fusion
into a full agent.

Required behavior:

1. Keep panel outputs as candidate implementations or patches.
2. Run each candidate through the existing `code_exec` / `patch_exec` sandbox.
3. Select a passing candidate directly when any candidate passes.
4. Do not synthesize over a passing candidate by default.
5. Optional one-step repair must be explicitly enabled and reported separately.

Done when:

- [done] Fusion does not lose a coding task where any panel candidate passes the
  local verifier.
- [done] Coding records include candidate-level verifier outcomes and the selected
  model.
- [done] The corrected 50-task pilot is rerun and reported in
  `reports/fusion-v3-merged-comparison-corrected.md`.

Latest corrected v3 evidence:

- Fresh/objective pilot: 30/30, policy `objective`.
- Coding pilot: 20/20, policy `verified_coding_select`.
- Corrected merged comparison: `fusion-cn-budget` 100.0%; GPT-5.5 valid-score
  95.6% in the capability-weighted main table; per-question strongest-single
  oracle degradation is 0/49 comparable rows; GPT-5.5 comparison is 1 win,
  0 losses, 46 ties on valid GPT-5.5 rows.
- `code-veryhard-filter-dsl-001` was a prompt ambiguity: "ids" now explicitly
  means `row["id"]`.
- `code-veryhard-sliding-topk-001` was rerun successfully; the earlier failure
  was a candidate-generation/provider reliability issue, not a verifier bug.

## Milestone 9: Rank-Then-Fuse and portfolio routing

Implement a non-objective candidate ranker and task-aware model portfolio
selection.

Required behavior:

1. Rank candidates before synthesis.
2. Return a clear winner directly when confidence is high.
3. Synthesize only when candidates are complementary.
4. Select panel models by task family using historical score, latency, timeout,
   and cost-per-solved data.

Done when:

- [done] Final synthesis no longer rewrites clear winning candidates by default
  when the heuristic ranker finds normalized agreement.
- [done] Anti-degradation guard usage is reported by policy.
- [done] `fusion-cn-budget` uses task-specific panel selection rather than one static
  panel for all task families.
- Anti-degradation failures trend downward after a full v3 pilot rerun.
