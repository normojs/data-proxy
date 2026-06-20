# Fusion v2 Evaluation Rollout

目标：判断 `fusion-cn-budget` 是否在真实高难任务上相对 GPT-5.5 和中国单模型有能力优势。主榜只排名能力；成本、延迟、成功率、单位成本只作为附属诊断。

## 核心判断

不要把真实公开榜单原题作为主力题库。公开原题可以提供权威锚点，但污染风险高，容易测到模型记忆、榜单适配或公开讨论痕迹。v2 应采用：

| 题源 | 比例 | 作用 |
| --- | ---: | --- |
| 公开榜单锚点题 | 10%-20% | 对齐外部权威榜单，做 sanity check。 |
| 榜单同构私有题 | 60%-70% | 主力区分能力，题型和评分方式对齐权威榜单，但内容新鲜。 |
| 真实业务任务题 | 15%-25% | 判断 Fusion 是否真的适合产品场景。 |

公开榜单锚点必须单独出表，不和私有题混在一起解释。最终报告至少展示：

1. Public-anchor score.
2. Private-isomorphic score.
3. Business-realistic score.
4. Combined capability score.

单模型对比只看 strongest baseline 或同成本/延迟档位 baseline；弱单模型
仅作诊断，不作为 Fusion 的主比较对象。

## v2 样本规模

v2 目标是先做到能区分 `fusion-cn-budget` 和 GPT-5.5，而不是一步到位做 300+ 题大榜。

| 模块 | 数量 | 主评分方式 | 权威锚点 |
| --- | ---: | --- | --- |
| Runnable coding | 50 | 代码执行、隐藏测试、patch 测试 | LiveCodeBench, EvalPlus, SWE-bench Verified, Terminal-Bench |
| High-difficulty reasoning | 30 | exact/numeric/multiple-choice | GPQA Diamond, MMLU-Pro, AIME, MATH-500 |
| Chinese capability | 25 | exact/schema/rubric | C-Eval, CMMLU, SuperCLUE, OpenCompass |
| English capability | 20 | exact/schema/rubric | MMLU-Pro, HELM, IFEval, LongBench |
| Long context | 15 | evidence-grounded exact/rubric | LongBench, RULER, Needle |
| Instruction following | 15 | parser/schema/regex/exact | IFEval |

Total: 155 tasks.

Cheap v2 pilot: 50 tasks.

| 模块 | 数量 |
| --- | ---: |
| Runnable coding | 20 |
| High-difficulty reasoning | 10 |
| Chinese capability | 8 |
| English capability | 6 |
| Long context | 3 |
| Instruction following | 3 |

## Coding v2

高难编程是 v2 的第一优先级，因为它最少依赖主观 judge，也最容易测出真实差距。

### Coding v2 组成

| 子模块 | 数量 | 要求 |
| --- | ---: | --- |
| LiveCodeBench-style hard algorithms | 14 | 新题，隐藏边界和大输入性能测试。 |
| Very-hard algorithms/parsers | 6 | 需要复杂状态、解析器、图/DP/数据结构组合。 |
| EvalPlus-style function tasks | 8 | 函数级，强边界测试。 |
| SWE-bench-style engineering patches | 14 | 小型 fixture repo，应用 diff 后跑测试。 |
| Repo/task debugging | 5 | 根因定位 + 最小修复。 |
| Terminal/tool-use style tasks | 3 | 命令行、配置、日志、文件处理。 |

### Coding v2 规则

1. 所有 coding 题必须运行代码。
2. 公开样例不计分，只用于理解题意。
3. 隐藏测试包含正常、边界、随机、性能和对抗 case。
4. patch 题要记录 touched files、patch size、测试日志。
5. Fusion 官方跑分不使用整体请求超时截断；完整跑完并记录 `latency_ms`。
6. Provider/network 错误要和模型失败分开，必要时允许同模型同题重试一次并标注 retry。

## 能测出 Fusion 差距的题型

Fusion 是多模型流水线，不能只测普通单答案题。v2 要增加这些题：

| 题型 | 为什么适合测 Fusion |
| --- | --- |
| 多约束代码修复 | 单模型容易漏一个约束，Fusion 可能从 panel 互补中获益。 |
| 长中文资料抽取 + 冲突检测 | 多模型可交叉校验证据。 |
| 中英混合技术任务 | 测中文指令、英文资料、代码语境的组合能力。 |
| 反直觉边界题 | 测是否能发现常见错误答案。 |
| 多方案比较决策 | 测 judge/final synthesis 是否优于单模型直答。 |
| 需要自检的算法题 | 测 final 是否能修正 panel 的局部错误。 |

## 公开榜单锚点使用规则

### 可以使用

1. 少量公开题或公开数据集样本，用于校准外部可比性。
2. 公开榜单的题型、难度、评分协议。
3. 公开 benchmark 的指标名和模块名。

### 不作为主力使用

1. 已被大量讨论的经典原题。
2. 排行榜长期公开的完整测试集。
3. 只靠 LLM judge、没有可复核答案的开放题。

### 私有同构题生成要求

1. 题型对齐公开榜单，但换事实、数据、约束和边界。
2. 每题有人工或程序校验的标准答案。
3. 编程题必须有隐藏测试；非编程题优先 exact/schema/numeric。
4. 每题记录 `source_family`，例如 `livecodebench_style_private`、`swebench_style_private`、`gpqa_style_private`。
5. 每题记录 `visibility`: `public_anchor`、`private_isomorphic`、`business_realistic`。

## Ranking 口径

主榜：

```text
overall_score =
  0.40 * coding_score +
  0.20 * reasoning_score +
  0.15 * chinese_score +
  0.10 * english_score +
  0.10 * long_context_score +
  0.05 * instruction_score
```

附属诊断：

1. Estimated cost total / per task / per solved task.
2. Relay-attributed cost or account-balance burn when the relay can attribute it.
3. p50/p95 latency.
4. Provider error rate.
5. Timeout rate.
6. Parse/extraction failure rate.
7. Score per USD, marked as informational only.

如果硅基流动调用使用代金券，余额差额要分清口径：`data.totalBalance`
更像总账户权益消耗，可能包含代金券；`data.chargeBalance` 更接近充值余额
/现金余额消耗。没有 provider 返回的分项扣款时，不能把代金券消耗说成真实现金支出。

Tie 规则：

1. 主分差小于 2pp 视为 statistical tie。
2. 优先看 private-isomorphic score。
3. 再看 hard-coding score。
4. 成本和延迟不打破能力平局，只进入产品建议。

## Immediate next steps

### Step 1: 修正 v1 平局解释

1. 重跑 GPT-5.5 的 `code-hard-window-subseq-001`，把 `fetch failed` 和模型失败分开。
2. 分析 Fusion 同题失败 artifact，判断是 panel、judge 还是 final synthesis 失误。
3. 在报告里新增 provider error 与 model failure 分表。

### Step 2: 扩 coding v2 pilot

状态：已扩展。`data/code-eval.v1.jsonl` 当前为 53 道 runnable coding tasks；
corrected v3 pilot 已跑其中前 20 道：

1. 44 道 `code_exec`，整体题库中 8 道 `very_hard`。
2. 9 道 `patch_exec` SWE-bench-style fixture。
3. 所有行已标注 `source_family` 和 `visibility`。
4. `code-validate` 已通过，0 warning / 0 error。

### Step 3: 建非编程 pilot

状态：已扩展。`data/fresh-eval.v2-pilot.jsonl` 当前为 88 道非编程 pilot；
corrected v3 pilot 已跑其中前 30 道：

1. 30 道高难 reasoning，含调度、逻辑、概率、图、计数和整数优化。
2. 18 道中文专项，含合同、公告、会议纪要、政策、客服和财务。
3. 18 道英文专项，含技术文档 QA、事故分析、API、权限、发布说明和科学推理。
4. 10 道长上下文，要求跨段证据定位。
5. 12 道 IFEval-style 指令遵循。
6. 全部为 objective scoring：84 道 `exact`，4 道 `regex`。
7. 所有行已标注 `module`、`source_family` 和 `visibility`。
8. `fresh-validate` 已通过，0 warning / 0 error。

### Step 4: 跑 v2 pilot

模型集合：

1. `fusion-cn-budget`
2. `openai/gpt-5.5`
3. `deepseek/deepseek-v4-pro`
4. `qwen/qwen3.7-plus`
5. `moonshotai/kimi-k2.6`
6. `minimax/minimax-m3`

输出：

1. `reports/code-v2-pilot.md`
2. `reports/fusion-v2-pilot.md`
3. Public/private/business split.
4. Main capability + secondary diagnostics.

### Step 5: 决策门槛

可以认为 Fusion 拉开能力差距的条件：

1. Overall capability 高于 GPT-5.5 至少 3pp，或
2. Private-isomorphic score 高于 GPT-5.5 至少 5pp，或
3. Hard coding 高于 GPT-5.5 至少 2 题，并且不是 provider error 导致。

如果能力持平：

1. Fusion 更慢、更贵，则不应作为默认模型。
2. 可以定位到 Fusion 擅长模块时，作为特定场景路由模型。
3. 继续优化 Fusion panel/judge/final，而不是扩大同质题库。
