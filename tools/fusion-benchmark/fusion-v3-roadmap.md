# Fusion v3 Roadmap

目标：把 Fusion 做成一个 model-like virtual model，而不是开放式
agent。Fusion 应在同成本/延迟档位内，在明确目标任务族上超过最佳单模型，
并通过多样性、选择和验证降低严重错误率。

单模型比较只保留全局 strongest baseline 和逐题 strongest-single oracle：
弱单模型只做诊断参考、消融和路由学习信号，不作为 Fusion 的主比较目标。

## 边界

Fusion v3 默认不是 agent：

1. 不主动探索用户仓库，不做开放式计划执行。
2. 不在真实项目里落地修改，只在临时 sandbox 中验证候选。
3. 不做无限 repair loop；默认 0 轮 repair，最多 1 轮可显式开启。
4. 用户看到的是一个稳定 model id，请求内部步骤固定、有上限、可审计。
5. 评测报告必须区分 `fusion_model_mode`、`verified_fusion_mode` 和
   `agentic_mode`，避免把 agent 能力混进模型能力榜。

## 公开依据

| 来源 | 相关结论 | Fusion v3 采用方式 |
| --- | --- | --- |
| [OpenRouter Fusion Router](https://openrouter.ai/docs/guides/routing/routers/fusion-router) | OpenRouter Fusion 是 panel 并行回答、judge 结构化比较、final 使用分析写答案的 deliberation 工具。 | 保留 panel/judge/final 作为基础，但不把它作为 coding/objective 的唯一策略。 |
| [OpenRouter Fusion pricing](https://openrouter.ai/openrouter/fusion) | Fusion 成本是底层 panel/judge 调用之和，不是单模型价格。 | v3 必须有 router、early exit 和档位化产品，而不是固定全量 panel。 |
| [LLM-Blender](https://arxiv.org/abs/2306.02561) | PairRanker 先比较/排序候选，GenFuser 只融合 top candidates；不同样本的最佳模型会变化。 | v3 默认先 rank/select，再决定是否 fuse；避免 final 改坏最佳候选。 |
| [DeliberationBench: When Do More Voices Hurt?](https://arxiv.org/abs/2601.08835) | 多模型 deliberation 可能输给 best-single selection，且成本质量比更差。 | 设计 anti-degradation guard：候选足够好时直接返回，不强制 synthesis。 |
| [Mixture-of-Agents](https://arxiv.org/abs/2406.04692) | 分层多模型可提升开放生成任务质量，但会增加成本和延迟。 | 只在 `fusion-cn-pro` 或开放综合任务启用 layered refinement；budget 档默认不用多层 MoA。 |
| [FrugalGPT](https://arxiv.org/abs/2305.05176) | LLM cascade 可降低成本并保持/提升质量。 | 引入 task router 和 confidence gate，简单题不启动 full Fusion。 |
| [Self-Consistency](https://arxiv.org/abs/2203.11171) | 多路径推理后选一致答案能提升算术/常识推理。 | 对 numeric/exact/reasoning 使用 normalized answer voting，而不是自由文本 judge。 |
| [AlphaCodium](https://arxiv.org/abs/2401.08500) | 代码生成应使用 test-based flow 和迭代反馈。 | coding 模式使用 sandbox verifier；默认只选已通过候选，repair 单独计入 verified mode。 |
| [CodeT](https://arxiv.org/abs/2207.10397) | 通过生成测试和执行一致性选择代码候选。 | 后续为代码题加入 generated regression tests 和 consistency ranking。 |
| [LEVER](https://arxiv.org/abs/2302.08468) | 执行结果可作为程序候选的 verifier/reranker 信号。 | verifier score 纳入候选排序。 |
| [SWE-agent](https://arxiv.org/abs/2405.15793) | 大型 repo 修复需要 agent-computer interface、导航、编辑、执行。 | 归入 `agentic_mode`，不并入默认 Fusion model 榜；大型项目只做 bounded verification。 |

## 产品分层

| Model id | 目标 | 默认策略 |
| --- | --- | --- |
| `fusion-cn-fast` | 低延迟、低成本、减少明显错误 | 2 panel + objective early exit + no synthesis when high confidence |
| `fusion-cn-budget` | 主力性价比虚拟模型 | 3 panel + task-aware ranker + anti-degradation guard + constrained final |
| `fusion-cn-pro` | 更强能力档，可接受高成本/高延迟 | 4-5 specialist panel + ranker + verifier + optional one-step refinement |

## 核心架构

```text
request
  -> task_classifier
  -> portfolio_router
  -> panel_candidates
  -> normalizer
  -> verifier_or_ranker
  -> anti_degradation_guard
  -> constrained_final_or_select
  -> response + fusion_metrics
```

### 1. Task classifier

将请求分到下列任务族：

| Task family | Examples | Primary scoring/selection |
| --- | --- | --- |
| `objective` | exact, regex, numeric, multiple choice, schema | normalization + scorer + consistency |
| `coding` | single-file function, patch_exec, algorithm | sandbox tests + candidate selection |
| `long_context` | evidence retrieval, conflict detection | evidence coverage + answer consistency |
| `reasoning` | math, logic, scheduling, probability | self-consistency + verifier where possible |
| `open_synthesis` | writing, strategy, business decision | pairwise/rubric ranker + conservative fusion |

### 2. Portfolio router

The router should choose panel composition by task family rather than using one
static preset for all work.

Initial rule set:

| Task family | Preferred panel | Avoid / downweight |
| --- | --- | --- |
| `coding` | DeepSeek, Qwen, strongest available coder | slow models with high timeout rate |
| `objective` | Qwen, DeepSeek, MiniMax | verbose final model unless constrained |
| `chinese` | Qwen, Moonshot, DeepSeek | models with weak Chinese exact behavior |
| `long_context` | Moonshot, Qwen, MiniMax | models that omit evidence |
| `open_synthesis` | diverse panel, including one conservative critic | homogeneous same-family panel |

This starts rule-based. Later it can be learned from benchmark records:
`model x task_family -> accuracy, latency, timeout_rate, cost_per_solved`.

### 3. Candidate normalizer

Every panel answer is normalized before ranking:

1. Strip markdown and surrounding quotes for objective answers.
2. Extract exact answer spans for short-answer tasks.
3. Extract code fence or unified diff for coding tasks.
4. Record parse/extraction failure separately from model correctness.
5. Preserve the raw answer for audit.

### 4. Verifier or ranker

Use objective verifiers whenever available. Use LLM ranking only when the task is
not objectively scorable.

| Task family | First choice | Fallback |
| --- | --- | --- |
| `objective` | local scorer over normalized answers | constrained judge |
| `coding` | sandbox test result | one-step repair if enabled |
| `long_context` | evidence/span coverage checker | pairwise judge |
| `reasoning` | numeric/exact consistency | self-consistency vote |
| `open_synthesis` | pairwise/rubric ranker | judge + final synthesis |

### 5. Anti-degradation guard

Fusion must avoid changing a correct answer into a worse answer.

Return the selected candidate directly when any of these hold:

1. Objective scorer finds a valid majority.
2. One coding candidate passes all required tests.
3. Pairwise ranker has a clear winner above confidence threshold.
4. Top candidate satisfies all hard constraints and other candidates add no new
   checked evidence.

Only call the final synthesizer when candidates are complementary or the user
asked for synthesis.

## Objective Fusion

Objective Fusion handles exact, regex, numeric, multiple-choice, and schema
tasks. This is the highest-confidence path because it does not require an LLM
judge to decide correctness.

Pipeline:

```text
panel answers
  -> normalize_answer
  -> scorer / regex / schema check
  -> equivalence grouping
  -> exact or semantic majority
  -> constrained final formatting
```

Required metrics:

1. `normalized_answer`
2. `answer_group`
3. `majority_size`
4. `objective_scorer_passed`
5. `selected_model`
6. `final_changed_answer`

Immediate target: eliminate format/entity errors such as answering
`cache-warmer-v3` when the exact expected entity is `cache-warmer`.

Runner status: implemented in `fresh-run` for `exact`, `contains`, and `regex`
rows. It selects a locally passing panel candidate directly and records
`fusion_metrics.policy=objective`.

## Verified Coding Fusion

Verified Coding Fusion is bounded verification, not an agent.

Pipeline:

```text
panel candidates
  -> extract code / patch
  -> run in isolated workspace
  -> rank by verifier result
  -> return passing candidate
  -> optional one-step repair only when explicitly enabled
```

Passing standard:

1. `code_exec`: code extraction succeeds, files are written, test command exits
   0 before timeout.
2. `patch_exec`: diff extraction succeeds, `git apply` succeeds, test command
   exits 0 before timeout.
3. Candidate must not touch forbidden files, generated lockfiles, tests, or
   unrelated files unless the task explicitly permits it.
4. Timeout, syntax error, patch apply failure, and assertion failure are all
   non-passing outcomes.

Ranking among passing candidates:

1. Highest verifier stage passed.
2. Smallest patch/touched-file risk.
3. Lower execution latency.
4. Lower model cost.
5. Preferred model reliability on the task family.

Default behavior when at least one candidate passes: return the best passing
candidate directly. Do not ask final synthesis to rewrite it.

Runner status: implemented in `code-run` for Fusion presets. Each panel
candidate is executed once in an isolated workspace; passing candidates are
ranked by verifier pass, patch risk, execution latency, and estimated cost. No
repair loop is enabled.

Optional repair mode:

1. Take top 1-2 failing candidates by verifier progress.
2. Provide only the failure log, candidate code, and original task.
3. Run one repair attempt.
4. Report `repair_used=true`; keep this mode separate in reports.

## Large project handling

Large projects are where Fusion can accidentally become an agent. The default
Fusion model must stay bounded.

Allowed in `verified_fusion_mode`:

1. Use caller-provided files, stack traces, and test commands.
2. Build a static repo map from provided checkout if explicitly requested.
3. Run configured tests in a sandbox.
4. Select among supplied/generated candidate patches.

Not allowed by default:

1. Autonomous repo exploration.
2. Long-horizon planning.
3. Multi-step file editing beyond generated patch candidates.
4. Repeated repair loops.

For real SWE-bench-style tasks, create a separate `agentic_mode` benchmark and
do not compare those results directly against model-mode GPT-5.5.

## Metrics schema additions

Add these fields to `fusion_metrics`:

| Field | Meaning |
| --- | --- |
| `task_family` | classifier output |
| `policy` | `objective`, `verified_coding`, `rank_then_fuse`, `plain_fusion` |
| `selected_model` | panel/final model whose answer was returned |
| `selection_reason` | `majority`, `tests_passed`, `ranker_clear_winner`, `synthesis` |
| `candidate_count` | number of generated candidates |
| `valid_candidate_count` | candidates that parsed/applied successfully |
| `passing_candidate_count` | candidates that passed objective/coding verifier |
| `final_changed_selected_answer` | true when final rewrote selected answer |
| `anti_degradation_guard` | whether guard skipped synthesis |
| `repair_used` | true only for explicit repair mode |
| `verifier_stage` | highest verifier stage used |
| `verifier_latency_ms` | total local verification latency |

Reports should include:

1. Fusion win/loss/tie against best single model.
2. Anti-degradation failures.
3. Verifier pass-through rate.
4. Final changed-answer error rate.
5. Cost and p95 latency by policy.
6. Severe error rate by task family.

## Implementation phases

### Phase 0: evaluation hygiene

1. Keep v2 pilot corrected report as baseline:
   `reports/fusion-v2-pilot-50-corrected.md`.
2. Separate model failures, provider failures, verifier failures, and runner
   failures.
3. Ensure all local test runners inherit a stable PATH and record execution
   environment in logs.

Done when:

- `fresh-validate` and `code-validate` are 0 warning / 0 error.
- Reports include provider error, timeout, and runner/verifier failure fields.

### Phase 1: Objective Fusion

1. Add task-family detection for objective fresh-eval rows.
2. Implement answer normalization and equivalence grouping.
3. Early-return objective majority without final synthesis.
4. Add metrics for selected answer, selected model, majority size, and final
   rewrite status.

Done when:

- `fresh-v2-english-incident-001` style entity-format errors are fixed.
- Objective policy does not lower exact/regex score versus best panel on the
  pilot set.

### Phase 2: Verified Coding Fusion

1. Modify Fusion code-run path to keep panel candidates as executable
   candidates.
2. Run each candidate through existing `code_exec` / `patch_exec` verifier.
3. Select passing candidate directly.
4. Only invoke synthesis/repair if no candidate passes and repair is explicitly
   enabled.

Done when:

- Fusion does not lose a coding task where any panel candidate passes.
- Reports show `selected_model`, `passing_candidate_count`, and
  `verifier_stage`.

### Phase 3: Rank-Then-Fuse

1. Implement pairwise/rubric candidate ranker for non-objective open tasks.
2. Add clear-winner threshold.
3. Synthesize only top candidates when ranker says candidates are
   complementary.

Done when:

- `final_changed_selected_answer` errors decrease versus plain fusion.
- Open-synthesis win rate improves without p95 latency regression above the
  configured budget.

### Phase 4: Portfolio learning

1. Aggregate historical records by `model`, `task_family`, `difficulty`, and
   `source_family`.
2. Build a rule-based portfolio table first.
3. Later replace with a learned router only if records are sufficient.

Done when:

- `fusion-cn-budget` uses task-specific panel selection.
- The router can justify why a model was included or skipped.

### Phase 5: product tiers

1. Define `fusion-cn-fast`, `fusion-cn-budget`, and `fusion-cn-pro` presets.
2. Give each preset explicit cost/latency/capability targets.
3. Do not mix tiers in the same leaderboard row.

Done when:

- Each tier has its own report section and decision threshold.

## Success criteria

Short-term pilot target:

1. `fusion-cn-budget` matches or exceeds the reliable strongest single baseline
   on capability-weighted score on the 50-task v2 pilot.
2. It has zero degradation versus the per-question strongest-single oracle on
   comparable rows.
3. p95 latency does not increase versus current corrected Fusion baseline.
4. Estimated cost stays within 1.5x GPT-5.5 for `fusion-cn-budget`.
5. Severe error rate is no worse than the strongest single baseline.

Longer-term product target:

1. In at least two target task families, Fusion beats the reliable strongest
   single model in the same cost/latency tier.
2. Severe error rate is lower than the reliable strongest single model.
3. Anti-degradation failures trend toward zero.
4. Objective format errors are near zero.
5. Public claim needs 141+ objective questions plus confidence intervals that
   support the conservative claim.

## Immediate next work

Completed v3 pilot evidence:

1. `fusion-cn-budget` corrected merged comparison:
   `reports/fusion-v3-merged-comparison-corrected.md`.
2. Fresh/objective pilot: 30/30 via Objective Fusion.
3. Coding pilot: 20/20 via Verified Coding Fusion.
4. Per-question strongest-single oracle degradation: 0/49 comparable rows.
5. Valid GPT-5.5 comparison rows: 1 Fusion win, 0 losses, 46 ties.
6. The corrected pilot is for expansion, not a final public superiority claim.

Next scaling work:

1. Expand from the 50-task pilot to the 141-task production-quality run.
2. Re-run provider-error GPT-5.5 rows before making a final public claim.
3. Family-level severe-error metrics and confidence intervals are implemented
   in reports; use them to audit the 141-task run.
4. Keep any repo-editing repair loop out of `fusion_model_mode`; report it only
   as `agentic_mode` if added later.
5. Use `production-gate` on the final merged JSONL; the current corrected pilot
   is `pilot_ready_expand`, not architecture/public-claim ready.

Current inventory:

- Corrected v3 merged pilot evidence: 50 evaluated unique questions.
- Current pinned dataset assets: 141/141 unique questions, missing 0.
- Historical fresh/code/fusion run union in this workspace: 80 unique questions,
  useful for diagnostics but not a substitute for the pinned production set.
- Current pinned dataset gaps against the 141-task target after completing all
  module targets:

| Module | Current dataset | Target | Missing |
| --- | ---: | ---: | ---: |
| `coding` | 53 | 53 | 0 |
| `reasoning` | 30 | 30 | 0 |
| `english` | 18 | 18 | 0 |
| `chinese` | 18 | 18 | 0 |
| `long_context` | 10 | 10 | 0 |
| `instruction_following` | 12 | 12 | 0 |
