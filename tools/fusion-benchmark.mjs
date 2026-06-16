#!/usr/bin/env node

import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const defaultConfigPath = path.join(__dirname, "fusion-benchmark", "config.json");

function readJson(file) {
  return JSON.parse(fs.readFileSync(file, "utf8"));
}

function loadConfig(argv) {
  const configPath = valueOf(argv, "--config") || defaultConfigPath;
  return readJson(configPath);
}

function loadEnvFile(argv) {
  const envFile = valueOf(argv, "--env-file");
  if (!envFile) return;
  const raw = fs.readFileSync(envFile, "utf8");
  for (const line of raw.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const idx = trimmed.indexOf("=");
    if (idx <= 0) continue;
    const key = trimmed.slice(0, idx).trim();
    let value = trimmed.slice(idx + 1).trim();
    if (
      (value.startsWith("\"") && value.endsWith("\"")) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    process.env[key] = value;
  }
}

function valueOf(argv, name, fallback = undefined) {
  const index = argv.indexOf(name);
  if (index === -1 || index + 1 >= argv.length) return fallback;
  return argv[index + 1];
}

function hasFlag(argv, name) {
  return argv.includes(name);
}

function normalizeApiBase(apiBase) {
  return String(apiBase || "").replace(/\/+$/, "");
}

function chatUrl(apiBase) {
  return `${normalizeApiBase(apiBase)}/chat/completions`;
}

function authKey(config) {
  return upstreamAuthKey(config, "default");
}

function envValue(name) {
  return name ? process.env[name] : "";
}

function upstreamConfig(config, upstreamName = "default") {
  const topLevel = {
    apiBase: config.apiBase,
    apiBaseEnv: config.apiBaseEnv,
    apiKeyEnv: config.apiKeyEnv
  };
  if (upstreamName === "default") {
    return { ...topLevel, ...(config.upstreams?.default || {}) };
  }
  return { ...(config.upstreams?.[upstreamName] || {}) };
}

function upstreamAuthKey(config, upstreamName = "default") {
  const upstream = upstreamConfig(config, upstreamName);
  return (
    envValue(upstream.apiKeyEnv) ||
    (upstreamName === "default" ? process.env.OPENAI_API_KEY : "") ||
    ""
  );
}

function configuredApiBase(config, upstreamName = "default") {
  const upstream = upstreamConfig(config, upstreamName);
  return (
    envValue(upstream.apiBaseEnv) ||
    upstream.apiBase ||
    (upstreamName === "default"
      ? process.env.OPENROUTER_API_BASE || process.env.OPENAI_BASE_URL || process.env.OPENAI_API_BASE || config.apiBase
      : "")
  );
}

function configuredApiBaseSource(config, upstreamName = "default") {
  const upstream = upstreamConfig(config, upstreamName);
  if (envValue(upstream.apiBaseEnv)) return upstream.apiBaseEnv;
  if (upstream.apiBase) return `${upstreamName}.apiBase`;
  if (upstreamName !== "default") return "missing";
  if (process.env.OPENROUTER_API_BASE) return "OPENROUTER_API_BASE";
  if (process.env.OPENAI_BASE_URL) return "OPENAI_BASE_URL";
  if (process.env.OPENAI_API_BASE) return "OPENAI_API_BASE";
  return "config.apiBase";
}

function configuredApiKeySource(config, upstreamName = "default") {
  const upstream = upstreamConfig(config, upstreamName);
  if (envValue(upstream.apiKeyEnv)) return upstream.apiKeyEnv;
  if (upstreamName === "default" && process.env.OPENAI_API_KEY) return "OPENAI_API_KEY";
  return "missing";
}

function upstreamNameForModel(model, config) {
  return config.modelUpstreams?.[model] || "default";
}

function upstreamModel(model, config) {
  return config.modelAliases?.[model] || model;
}

function modelOptions(model, config) {
  return config.modelOptions?.[model] || {};
}

function resolveModelUpstream(model, config, options = {}) {
  const upstreamName = options.upstreamName || upstreamNameForModel(model, config);
  const apiBase = options.apiBaseOverride || configuredApiBase(config, upstreamName);
  const apiKey = options.apiKeyOverride || upstreamAuthKey(config, upstreamName);
  if (!apiBase) throw new Error(`Missing API base for upstream ${upstreamName} (${model})`);
  if (!apiKey) throw new Error(`Missing API key for upstream ${upstreamName} (${model})`);
  return {
    upstreamName,
    apiBase,
    apiKey,
    model: upstreamModel(model, config)
  };
}

function collectConfiguredModels(config) {
  const models = new Set([...(config.baselines || [])]);
  for (const preset of Object.values(config.fusionPresets || {})) {
    for (const model of preset.panel || []) models.add(model);
    if (preset.judge) models.add(preset.judge);
    for (const model of preset.judgeFallbacks || []) models.add(model);
    if (preset.final) models.add(preset.final);
    for (const model of preset.finalFallbacks || []) models.add(model);
  }
  if (config.pairwise?.defaultTarget) models.add(config.pairwise.defaultTarget);
  if (config.pairwise?.defaultBaseline) models.add(config.pairwise.defaultBaseline);
  if (config.pairwise?.defaultJudge) models.add(config.pairwise.defaultJudge);
  return [...models];
}

function jsonResponse(res, status, body) {
  const payload = JSON.stringify(body);
  res.writeHead(status, {
    "content-type": "application/json",
    "content-length": Buffer.byteLength(payload)
  });
  res.end(payload);
}

function nowId(prefix) {
  return `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function percentile(values, p) {
  const nums = values.filter((v) => Number.isFinite(v)).sort((a, b) => a - b);
  if (nums.length === 0) return 0;
  const idx = Math.min(nums.length - 1, Math.ceil((p / 100) * nums.length) - 1);
  return nums[idx];
}

function normalizeText(text) {
  return String(text ?? "")
    .trim()
    .replace(/\s+/g, " ")
    .replace(/^["'`]+|["'`]+$/g, "")
    .toLowerCase();
}

function extractContent(responseJson) {
  return responseJson?.choices?.[0]?.message?.content ?? "";
}

function parseJsonObjectText(content) {
  const text = String(content || "").trim();
  if (!text) throw new Error("Empty JSON content");
  try {
    return JSON.parse(text);
  } catch {}
  const fenced = text.match(/```(?:json)?\s*([\s\S]*?)```/i);
  if (fenced) {
    try {
      return JSON.parse(fenced[1].trim());
    } catch {}
  }
  const start = text.indexOf("{");
  const end = text.lastIndexOf("}");
  if (start !== -1 && end > start) {
    return JSON.parse(text.slice(start, end + 1));
  }
  throw new Error("No parseable JSON object found");
}

function tokenCost(model, usage, config) {
  const price = config.pricing?.[model];
  if (!price || !usage) return 0;
  const prompt = usage.prompt_tokens || 0;
  const completion = usage.completion_tokens || 0;
  return prompt * price.prompt + completion * price.completion;
}

function timeoutMs(config, name, fallback) {
  const value = Number(config.timeouts?.[name]);
  return Number.isFinite(value) && value > 0 ? value : fallback;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isRetryableStatus(status) {
  return status === 408 || status === 409 || status === 425 || status === 429 || status >= 500;
}

function isRetryableError(err) {
  if (err?.status) return isRetryableStatus(err.status);
  const message = String(err?.message || "").toLowerCase();
  return (
    message.includes("fetch failed") ||
    message.includes("econnreset") ||
    message.includes("etimedout") ||
    message.includes("socket") ||
    message.includes("network")
  );
}

function aggregateUsage(parts) {
  const usage = { prompt_tokens: 0, completion_tokens: 0, total_tokens: 0 };
  for (const part of parts) {
    const u = part?.usage;
    if (!u) continue;
    usage.prompt_tokens += u.prompt_tokens || 0;
    usage.completion_tokens += u.completion_tokens || 0;
    usage.total_tokens += u.total_tokens || (u.prompt_tokens || 0) + (u.completion_tokens || 0);
  }
  return usage;
}

function exactMajorityEarlyExit(preset, panelResults, config) {
  const rule = preset.earlyExit;
  if (rule?.strategy !== "exact_majority") return null;
  const minAgree = Math.max(2, Number(rule.minAgree) || 2);
  const maxAnswerChars = Math.max(1, Number(rule.maxAnswerChars) || 200);
  const groups = new Map();
  for (const result of panelResults) {
    if (!result.ok || !result.content?.trim()) continue;
    const content = result.content.trim();
    if (content.length > maxAnswerChars) continue;
    const key = normalizeText(content);
    if (!key) continue;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(result);
  }
  const winners = [...groups.values()].sort((a, b) => b.length - a.length);
  const winner = winners[0];
  if (!winner || winner.length < minAgree) return null;
  const usage = aggregateUsage(panelResults);
  const costUsd = panelResults.reduce((sum, r) => sum + (r.cost_usd || 0), 0);
  return {
    answer: winner[0].content.trim(),
    agreed_models: winner.map((r) => r.model),
    usage,
    cost_usd: costUsd
  };
}

async function callChat({ apiBase, apiKey, body, extraHeaders = {}, timeoutMs = 120000, retries = 2 }) {
  const started = Date.now();
  let lastErr;
  for (let attempt = 0; attempt <= retries; attempt += 1) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), timeoutMs);
    let resp;
    try {
      resp = await fetch(chatUrl(apiBase), {
        method: "POST",
        headers: {
          "content-type": "application/json",
          ...(apiKey ? { authorization: `Bearer ${apiKey}` } : {}),
          ...extraHeaders
        },
        body: JSON.stringify(body),
        signal: controller.signal
      });
    } catch (err) {
      if (err?.name === "AbortError") {
        lastErr = new Error(`Request timed out after ${timeoutMs}ms`);
      } else {
        lastErr = err;
      }
      clearTimeout(timeout);
      if (attempt < retries && isRetryableError(lastErr)) {
        await sleep(500 * 2 ** attempt);
        continue;
      }
      throw lastErr;
    }
    clearTimeout(timeout);
    const text = await resp.text();
    let json;
    try {
      json = JSON.parse(text);
    } catch {
      json = { error: { message: text } };
    }
    if (!resp.ok) {
      const msg = json?.error?.message || json?.message || text || `HTTP ${resp.status}`;
      const err = new Error(msg);
      err.status = resp.status;
      err.response = json;
      lastErr = err;
      if (attempt < retries && isRetryableStatus(resp.status)) {
        await sleep(500 * 2 ** attempt);
        continue;
      }
      throw err;
    }
    json._latency_ms = Date.now() - started;
    return json;
  }
  throw lastErr || new Error("Request failed");
}

function panelMessages(originalMessages, model) {
  return [
    {
      role: "system",
      content:
        "You are one model in a multi-model Fusion panel. Solve the user request independently. Be precise, avoid deference to other models, and include concise reasoning only when it helps correctness."
    },
    ...originalMessages,
    {
      role: "user",
      content: `Panel model: ${model}. Provide your best independent answer.`
    }
  ];
}

function judgeMessages(originalMessages, panelResults) {
  return [
    {
      role: "system",
      content:
        "You are the Fusion judge. Compare model answers and return strict JSON with keys: consensus, contradictions, partial_coverage, unique_insights, blind_spots, recommended_answer_strategy. Do not include markdown."
    },
    {
      role: "user",
      content: JSON.stringify(
        {
          original_messages: originalMessages,
          panel_answers: panelResults.map((r) => ({
            model: r.model,
            ok: r.ok,
            answer: r.content,
            error: r.error
          }))
        },
        null,
        2
      )
    }
  ];
}

function finalMessages(originalMessages, panelResults, judgeJsonText) {
  return [
    {
      role: "system",
      content:
        "You are the final Fusion synthesizer. Produce the best final answer for the original user. Use the panel answers and judge analysis, but do not mention internal panel mechanics unless the user asks."
    },
    {
      role: "user",
      content: JSON.stringify(
        {
          original_messages: originalMessages,
          panel_answers: panelResults.map((r) => ({
            model: r.model,
            answer: r.content
          })),
          judge_analysis: judgeJsonText
        },
        null,
        2
      )
    }
  ];
}

function pairwiseJudgeMessages(question, left, right) {
  return [
    {
      role: "system",
      content:
        "You are an impartial evaluator. Compare two assistant answers to the same user task. Return strict JSON only with keys: winner, confidence, rationale. winner must be exactly one of A, B, or tie."
    },
    {
      role: "user",
      content: JSON.stringify(
        {
          prompt: question.prompt,
          rubric: question.rubric || "Prefer the answer that is more correct, useful, specific, and concise.",
          answer_A: left.answer,
          answer_B: right.answer
        },
        null,
        2
      )
    }
  ];
}

async function runFusion(body, config, options = {}) {
  const preset = config.fusionPresets?.[body.model];
  if (!preset) throw new Error(`Unknown fusion preset: ${body.model}`);

  const originalMessages = body.messages || [];
  const maxTokens = body.max_tokens || body.max_completion_tokens || 4096;
  const temperature = body.temperature ?? 0.2;

  const panelPromises = preset.panel.map(async (model) => {
    const started = Date.now();
    try {
      const upstream = resolveModelUpstream(model, config, {
        apiBaseOverride: options.upstreamApiBase,
        apiKeyOverride: options.apiKey
      });
      const result = await callChat({
        apiBase: upstream.apiBase,
        apiKey: upstream.apiKey,
        body: {
          ...body,
          ...modelOptions(model, config),
          model: upstream.model,
          messages: panelMessages(originalMessages, model),
          max_tokens: maxTokens,
          temperature
        },
        extraHeaders: { "x-revo-fusion-depth": "1" },
        timeoutMs: timeoutMs(config, "panelMs", 30000)
      });
      return {
        model,
        ok: true,
        content: extractContent(result),
        usage: result.usage,
        latency_ms: result._latency_ms ?? Date.now() - started,
        cost_usd: tokenCost(model, result.usage, config)
      };
    } catch (err) {
      return {
        model,
        ok: false,
        content: "",
        error: err.message,
        latency_ms: Date.now() - started,
        cost_usd: 0
      };
    }
  });

  const panelResults = await Promise.all(panelPromises);
  const successfulPanels = panelResults.filter((r) => r.ok && r.content.trim());
  if (successfulPanels.length === 0) {
    const err = new Error("All Fusion panel calls failed");
    err.status = 502;
    err.panelResults = panelResults;
    throw err;
  }

  const earlyExit = exactMajorityEarlyExit(preset, panelResults, config);
  if (earlyExit) {
    return {
      id: nowId("chatcmpl-fusion"),
      object: "chat.completion",
      created: Math.floor(Date.now() / 1000),
      model: body.model,
      choices: [
        {
          index: 0,
          message: {
            role: "assistant",
            content: earlyExit.answer
          },
          finish_reason: "stop"
        }
      ],
      usage: earlyExit.usage,
      fusion_metrics: {
        preset: body.model,
        panel: panelResults,
        early_exit: {
          strategy: preset.earlyExit?.strategy,
          agreed_models: earlyExit.agreed_models
        },
        judge: {
          skipped: true,
          model: null,
          preferred_model: preset.judge,
          json_valid: null,
          usage: null,
          latency_ms: 0,
          errors: []
        },
        final: {
          skipped: true,
          model: null,
          preferred_model: preset.final,
          usage: null,
          latency_ms: 0
        },
        all_panel_failure: false,
        judge_json_valid: null,
        cost_usd: earlyExit.cost_usd
      }
    };
  }

  let judgeResult = null;
  let judgeJsonValid = false;
  let judgeContent = "";
  let judgeModel = preset.judge;
  const judgeErrors = [];
  const judgeMaxTokens = Math.max(512, Math.min(Math.max(maxTokens, 512), 2048));
  try {
    const judgeCandidates = [preset.judge, ...(preset.judgeFallbacks || [])].filter(Boolean);
    for (const candidate of judgeCandidates) {
      try {
        const judgeUpstream = resolveModelUpstream(candidate, config, {
          apiBaseOverride: options.upstreamApiBase,
          apiKeyOverride: options.apiKey
        });
        const candidateResult = await callChat({
          apiBase: judgeUpstream.apiBase,
          apiKey: judgeUpstream.apiKey,
          body: {
            ...modelOptions(candidate, config),
            model: judgeUpstream.model,
            messages: judgeMessages(originalMessages, panelResults),
            temperature: 0,
            max_tokens: judgeMaxTokens,
            response_format: { type: "json_object" }
          },
          extraHeaders: { "x-revo-fusion-depth": "1" },
          timeoutMs: timeoutMs(config, "judgeMs", 45000)
        });
        const candidateContent = extractContent(candidateResult);
        parseJsonObjectText(candidateContent);
        judgeResult = candidateResult;
        judgeContent = candidateContent;
        judgeJsonValid = true;
        judgeModel = candidate;
        break;
      } catch (err) {
        judgeErrors.push(`${candidate}: ${err.message}`);
      }
    }
    if (!judgeJsonValid) throw new Error(judgeErrors.join("; ") || "All Fusion judge candidates failed");
  } catch (err) {
    judgeContent = JSON.stringify({
      consensus: "",
      contradictions: [],
      partial_coverage: [],
      unique_insights: [],
      blind_spots: [`Judge failed: ${err.message}`],
      recommended_answer_strategy: "Synthesize directly from successful panel answers."
    });
  }

  const finalCandidates = [preset.final, ...(preset.finalFallbacks || [])].filter(Boolean);
  let finalModel = preset.final;
  let finalResult = null;
  let finalError = null;
  for (const candidate of finalCandidates) {
    try {
      const finalUpstream = resolveModelUpstream(candidate, config, {
        apiBaseOverride: options.upstreamApiBase,
        apiKeyOverride: options.apiKey
      });
      finalResult = await callChat({
        apiBase: finalUpstream.apiBase,
        apiKey: finalUpstream.apiKey,
        body: {
          ...body,
          ...modelOptions(candidate, config),
          model: finalUpstream.model,
          messages: finalMessages(originalMessages, successfulPanels, judgeContent),
          max_tokens: maxTokens,
          temperature
        },
        extraHeaders: { "x-revo-fusion-depth": "1" },
        timeoutMs: timeoutMs(config, "finalMs", 60000)
      });
      const finalContent = extractContent(finalResult);
      if (!finalContent.trim()) {
        throw new Error("Fusion final candidate returned empty content");
      }
      finalModel = candidate;
      break;
    } catch (err) {
      finalError = err;
      finalResult = null;
    }
  }
  if (!finalResult) {
    throw finalError || new Error("All Fusion final candidates failed");
  }

  const usage = aggregateUsage([...panelResults, judgeResult, finalResult]);
  const costUsd =
    panelResults.reduce((sum, r) => sum + (r.cost_usd || 0), 0) +
    tokenCost(judgeModel, judgeResult?.usage, config) +
    tokenCost(finalModel, finalResult.usage, config);

  return {
    id: nowId("chatcmpl-fusion"),
    object: "chat.completion",
    created: Math.floor(Date.now() / 1000),
    model: body.model,
    choices: [
      {
        index: 0,
        message: {
          role: "assistant",
          content: extractContent(finalResult)
        },
        finish_reason: "stop"
      }
    ],
    usage,
    fusion_metrics: {
      preset: body.model,
      panel: panelResults,
      judge: {
        model: judgeModel,
        preferred_model: preset.judge,
        json_valid: judgeJsonValid,
        usage: judgeResult?.usage || null,
        latency_ms: judgeResult?._latency_ms || 0,
        errors: judgeErrors
      },
      final: {
        model: finalModel,
        preferred_model: preset.final,
        usage: finalResult.usage || null,
        latency_ms: finalResult._latency_ms || 0
      },
      all_panel_failure: false,
      judge_json_valid: judgeJsonValid,
      cost_usd: costUsd
    }
  };
}

async function readRequestBody(req) {
  const chunks = [];
  for await (const chunk of req) chunks.push(chunk);
  const raw = Buffer.concat(chunks).toString("utf8");
  return raw ? JSON.parse(raw) : {};
}

async function serve(argv) {
  const config = loadConfig(argv);
  const port = Number(valueOf(argv, "--port", "8787"));
  const upstreamApiBase = valueOf(argv, "--upstream-api-base");
  const apiKey = upstreamApiBase ? authKey(config) : undefined;
  const logPath = valueOf(argv, "--log");
  if (logPath) fs.mkdirSync(path.dirname(logPath), { recursive: true });
  const logStream = logPath ? fs.createWriteStream(logPath, { flags: "a" }) : null;

  function writeRequestLog(record) {
    if (!logStream) return;
    logStream.write(`${JSON.stringify({ run_type: "request_log", ts: new Date().toISOString(), ...record })}\n`);
  }

  const server = http.createServer(async (req, res) => {
    const started = Date.now();
    let body = {};
    try {
      if (req.method === "GET" && req.url === "/healthz") {
        return jsonResponse(res, 200, { ok: true });
      }
      if (req.method !== "POST" || req.url !== "/v1/chat/completions") {
        return jsonResponse(res, 404, { error: { message: "Not found" } });
      }
      body = await readRequestBody(req);
      if (body.stream) {
        return jsonResponse(res, 400, {
          error: { message: "Fusion benchmark proxy does not support stream=true yet." }
        });
      }
      if (config.fusionPresets?.[body.model]) {
        const fusion = await runFusion(body, config, { upstreamApiBase, apiKey });
        writeRequestLog({
          model: body.model,
          ok: true,
          is_fusion: true,
          latency_ms: Date.now() - started,
          usage: fusion.usage || null,
          cost_usd: fusion.fusion_metrics?.cost_usd || 0,
          judge_json_valid: fusion.fusion_metrics?.judge_json_valid,
          all_panel_failure: fusion.fusion_metrics?.all_panel_failure
        });
        return jsonResponse(res, 200, fusion);
      }
      const upstream = resolveModelUpstream(body.model, config, {
        apiBaseOverride: upstreamApiBase,
        apiKeyOverride: apiKey
      });
      const passthrough = await callChat({
        apiBase: upstream.apiBase,
        apiKey: upstream.apiKey,
        body: { ...body, ...modelOptions(body.model, config), model: upstream.model },
        extraHeaders: { "x-revo-fusion-proxy": "passthrough" },
        timeoutMs: timeoutMs(config, "passthroughMs", 60000)
      });
      writeRequestLog({
        model: body.model,
        ok: true,
        is_fusion: false,
        latency_ms: Date.now() - started,
        usage: passthrough.usage || null,
        cost_usd: tokenCost(body.model, passthrough.usage, config)
      });
      return jsonResponse(res, 200, passthrough);
    } catch (err) {
      writeRequestLog({
        model: body?.model || "",
        ok: false,
        is_fusion: Boolean(config.fusionPresets?.[body?.model]),
        latency_ms: Date.now() - started,
        cost_usd: 0,
        error: err.message
      });
      return jsonResponse(res, err.status || 500, {
        error: {
          message: err.message,
          type: "fusion_benchmark_error"
        },
        fusion_panel: err.panelResults || undefined
      });
    }
  });

  server.listen(port, "127.0.0.1", () => {
    console.log(`Fusion benchmark proxy listening on http://127.0.0.1:${port}/v1`);
    console.log(`Upstream routing: ${upstreamApiBase ? "global --upstream-api-base override" : "modelUpstreams config"}`);
    if (logPath) console.log(`Request log: ${logPath}`);
  });
}

function parseJsonl(file) {
  return fs
    .readFileSync(file, "utf8")
    .split(/\r?\n/)
    .filter((line) => line.trim())
    .map((line, index) => {
      try {
        return JSON.parse(line);
      } catch (err) {
        throw new Error(`${file}:${index + 1}: ${err.message}`);
      }
    });
}

function filterQuestions(questions, argv) {
  const categoryArg = valueOf(argv, "--category");
  const categories = categoryArg
    ? new Set(categoryArg.split(",").map((c) => c.trim()).filter(Boolean))
    : null;
  const offset = Math.max(0, Number(valueOf(argv, "--offset", "0")) || 0);
  const limitRaw = valueOf(argv, "--limit");
  const limit = limitRaw === undefined ? null : Math.max(0, Number(limitRaw) || 0);
  const filtered = categories ? questions.filter((q) => categories.has(q.category)) : questions;
  return limit === null ? filtered.slice(offset) : filtered.slice(offset, offset + limit);
}

function walkFiles(root, matcher, out = []) {
  const stat = fs.statSync(root);
  if (stat.isFile()) {
    if (matcher(root)) out.push(root);
    return out;
  }
  if (!stat.isDirectory()) return out;
  for (const entry of fs.readdirSync(root)) {
    walkFiles(path.join(root, entry), matcher, out);
  }
  return out;
}

function collectInputFiles(inputs, matcher = () => true) {
  return inputs
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
    .flatMap((item) => walkFiles(item, matcher));
}

function scoreAnswer(content, scoring) {
  if (!scoring) return { score: null, passed: null, reason: "No scoring rule" };
  const actual = normalizeText(content);
  if (scoring.type === "exact") {
    const expected = normalizeText(scoring.answer);
    const passed = actual === expected;
    return { score: passed ? 1 : 0, passed, reason: `exact:${scoring.answer}` };
  }
  if (scoring.type === "contains") {
    const expected = normalizeText(scoring.answer);
    const passed = actual.includes(expected);
    return { score: passed ? 1 : 0, passed, reason: `contains:${scoring.answer}` };
  }
  if (scoring.type === "regex") {
    const re = new RegExp(scoring.pattern, scoring.flags || "");
    const passed = re.test(String(content ?? "").trim());
    return { score: passed ? 1 : 0, passed, reason: `regex:${scoring.pattern}` };
  }
  throw new Error(`Unsupported scoring type: ${scoring.type}`);
}

function parseJudgeWinner(content) {
  try {
    const parsed = parseJsonObjectText(content);
    const winner = String(parsed.winner || "").trim().toLowerCase();
    if (winner === "a") return { winner: "A", json_valid: true, parsed };
    if (winner === "b") return { winner: "B", json_valid: true, parsed };
    if (winner === "tie") return { winner: "tie", json_valid: true, parsed };
    return { winner: "invalid", json_valid: true, parsed };
  } catch {
    const text = String(content || "").toLowerCase();
    if (/\bwinner\b[^a-z0-9]+a\b/.test(text)) return { winner: "A", json_valid: false, parsed: null };
    if (/\bwinner\b[^a-z0-9]+b\b/.test(text)) return { winner: "B", json_valid: false, parsed: null };
    if (/\btie\b/.test(text)) return { winner: "tie", json_valid: false, parsed: null };
    return { winner: "invalid", json_valid: false, parsed: null };
  }
}

async function callBenchmarkModel(model, body, config, options = {}) {
  if (options.apiBaseOverride) {
    return callChat({
      apiBase: options.apiBaseOverride,
      apiKey: options.apiKeyOverride || "",
      body: { ...body, model },
      timeoutMs: timeoutMs(config, "passthroughMs", 60000)
    });
  }
  if (config.fusionPresets?.[model]) {
    return runFusion({ ...body, model }, config);
  }
  const upstream = resolveModelUpstream(model, config);
  return callChat({
    apiBase: upstream.apiBase,
    apiKey: upstream.apiKey,
    body: { ...body, ...modelOptions(model, config), model: upstream.model },
    timeoutMs: timeoutMs(config, "passthroughMs", 60000)
  });
}

async function freshRun(argv) {
  const config = loadConfig(argv);
  const dataset = valueOf(argv, "--dataset");
  if (!dataset) throw new Error("Missing --dataset");
  const out = valueOf(
    argv,
    "--out",
    path.join(__dirname, "fusion-benchmark", "runs", `fresh-${Date.now()}.jsonl`)
  );
  const apiBaseOverride = valueOf(argv, "--api-base");
  const apiKeyOverride = apiBaseOverride ? authKey(config) : undefined;
  const modelArg = valueOf(argv, "--models");
  const models = modelArg
    ? modelArg.split(",").map((m) => m.trim()).filter(Boolean)
    : [...config.baselines, ...Object.keys(config.fusionPresets)];
  const questions = filterQuestions(parseJsonl(dataset), argv);
  if (questions.length === 0) throw new Error("No questions matched --category/--offset/--limit filters");
  fs.mkdirSync(path.dirname(out), { recursive: true });
  const stream = fs.createWriteStream(out, { flags: "w" });

  for (const question of questions) {
    for (const model of models) {
      const started = Date.now();
      let record;
      try {
        const response = await callBenchmarkModel(model, {
          messages: [{ role: "user", content: question.prompt }],
          temperature: question.temperature ?? 0,
          max_tokens: question.max_tokens || 1024
        }, config, {
          apiBaseOverride,
          apiKeyOverride
        });
        const content = extractContent(response);
        const emptyContent = !content.trim();
        const scored = scoreAnswer(content, question.scoring);
        record = {
          run_type: "fresh_eval",
          question_id: question.id,
          category: question.category,
          model,
          ok: !emptyContent,
          score: emptyContent ? 0 : scored.score,
          passed: emptyContent ? false : scored.passed,
          score_reason: emptyContent ? "empty_content" : scored.reason,
          latency_ms: response._latency_ms ?? Date.now() - started,
          usage: response.usage || null,
          cost_usd: response.fusion_metrics?.cost_usd ?? tokenCost(model, response.usage, config),
          judge_json_valid: response.fusion_metrics?.judge_json_valid,
          all_panel_failure: response.fusion_metrics?.all_panel_failure,
          fusion_metrics: response.fusion_metrics
            ? {
                panel: response.fusion_metrics.panel?.map((p) => ({
                  model: p.model,
                  ok: p.ok,
                  error: p.error,
                  latency_ms: p.latency_ms,
                  cost_usd: p.cost_usd
                })),
                early_exit: response.fusion_metrics.early_exit,
                judge: response.fusion_metrics.judge,
                final: response.fusion_metrics.final,
                cost_usd: response.fusion_metrics.cost_usd
              }
            : undefined,
          answer: content,
          error: emptyContent ? "Empty model response" : undefined
        };
      } catch (err) {
        record = {
          run_type: "fresh_eval",
          question_id: question.id,
          category: question.category,
          model,
          ok: false,
          score: 0,
          passed: false,
          latency_ms: Date.now() - started,
          cost_usd: 0,
          error: err.message
        };
      }
      stream.write(`${JSON.stringify(record)}\n`);
      console.log(`${record.ok ? "ok" : "fail"} ${model} ${question.id} score=${record.score}`);
    }
  }
  await new Promise((resolve) => stream.end(resolve));
  console.log(`Wrote ${out}`);
}

async function pairwiseRun(argv) {
  const config = loadConfig(argv);
  const dataset = valueOf(argv, "--dataset");
  if (!dataset) throw new Error("Missing --dataset");
  const out = valueOf(
    argv,
    "--out",
    path.join(__dirname, "fusion-benchmark", "runs", `pairwise-${Date.now()}.jsonl`)
  );
  const apiBaseOverride = valueOf(argv, "--api-base");
  const apiKeyOverride = apiBaseOverride ? authKey(config) : undefined;
  const target = valueOf(argv, "--target", config.pairwise?.defaultTarget || "fusion-cn-budget");
  const baseline = valueOf(argv, "--baseline", config.pairwise?.defaultBaseline || "openai/gpt-5.5");
  const judge = valueOf(argv, "--judge", config.pairwise?.defaultJudge || "anthropic/claude-sonnet-latest");
  const questions = parseJsonl(dataset);
  fs.mkdirSync(path.dirname(out), { recursive: true });
  const stream = fs.createWriteStream(out, { flags: "w" });

  for (const question of questions) {
    const started = Date.now();
    const answerBody = () => ({
      messages: [{ role: "user", content: question.prompt }],
      temperature: question.temperature ?? 0.2,
      max_tokens: question.max_tokens || 2048
    });

    let record;
    try {
      const [targetResponse, baselineResponse] = await Promise.all([
        callBenchmarkModel(target, answerBody(), config, { apiBaseOverride, apiKeyOverride }),
        callBenchmarkModel(baseline, answerBody(), config, { apiBaseOverride, apiKeyOverride })
      ]);
      const targetAnswer = extractContent(targetResponse);
      const baselineAnswer = extractContent(baselineResponse);
      const targetIsA = Math.random() < 0.5;
      const left = targetIsA
        ? { model: target, answer: targetAnswer }
        : { model: baseline, answer: baselineAnswer };
      const right = targetIsA
        ? { model: baseline, answer: baselineAnswer }
        : { model: target, answer: targetAnswer };
      const judgeResponse = await callBenchmarkModel(judge, {
        messages: pairwiseJudgeMessages(question, left, right),
        temperature: 0,
        max_tokens: 768,
        response_format: { type: "json_object" }
      }, config, { apiBaseOverride, apiKeyOverride });
      const judgeContent = extractContent(judgeResponse);
      const judged = parseJudgeWinner(judgeContent);
      let winnerModel = null;
      if (judged.winner === "A") winnerModel = left.model;
      if (judged.winner === "B") winnerModel = right.model;
      if (judged.winner === "tie") winnerModel = "tie";

      const costUsd =
        (targetResponse.fusion_metrics?.cost_usd ?? tokenCost(target, targetResponse.usage, config)) +
        (baselineResponse.fusion_metrics?.cost_usd ?? tokenCost(baseline, baselineResponse.usage, config)) +
        tokenCost(judge, judgeResponse.usage, config);

      record = {
        run_type: "pairwise_eval",
        question_id: question.id,
        category: question.category,
        target_model: target,
        baseline_model: baseline,
        judge_model: judge,
        ok: judged.winner !== "invalid",
        winner: winnerModel,
        target_result: winnerModel === target ? "win" : winnerModel === baseline ? "loss" : winnerModel === "tie" ? "tie" : "invalid",
        ab_order: targetIsA ? "target=A" : "target=B",
        judge_json_valid: judged.json_valid,
        judge_rationale: judged.parsed?.rationale || "",
        confidence: judged.parsed?.confidence ?? null,
        latency_ms: Date.now() - started,
        cost_usd: costUsd,
        usage: {
          target: targetResponse.usage || null,
          baseline: baselineResponse.usage || null,
          judge: judgeResponse.usage || null
        },
        answers: {
          target: targetAnswer,
          baseline: baselineAnswer
        }
      };
    } catch (err) {
      record = {
        run_type: "pairwise_eval",
        question_id: question.id,
        category: question.category,
        target_model: target,
        baseline_model: baseline,
        judge_model: judge,
        ok: false,
        winner: "invalid",
        target_result: "invalid",
        latency_ms: Date.now() - started,
        cost_usd: 0,
        error: err.message
      };
    }
    stream.write(`${JSON.stringify(record)}\n`);
    console.log(`${record.ok ? "ok" : "fail"} ${target} vs ${baseline} ${question.id} result=${record.target_result}`);
  }
  await new Promise((resolve) => stream.end(resolve));
  console.log(`Wrote ${out}`);
}

function livebenchImport(argv) {
  const inputs = valueOf(argv, "--inputs");
  if (!inputs) throw new Error("Missing --inputs");
  const release = valueOf(argv, "--release", "unknown");
  const out = valueOf(
    argv,
    "--out",
    path.join(__dirname, "fusion-benchmark", "runs", `livebench-${release}-${Date.now()}.jsonl`)
  );
  const files = collectInputFiles(inputs, (file) => file.endsWith("ground_truth_judgment.jsonl") || file.endsWith(".jsonl"));
  const records = [];
  for (const file of files) {
    for (const row of parseJsonl(file)) {
      if (!row.model || typeof row.score !== "number") continue;
      records.push({
        run_type: "livebench",
        release,
        question_id: row.question_id,
        task: row.task || row.subtask || "",
        category: row.category || "",
        model: row.model,
        ok: true,
        score: row.score,
        source_file: file,
        eval_status: row.eval_status || ""
      });
    }
  }
  fs.mkdirSync(path.dirname(out), { recursive: true });
  fs.writeFileSync(out, records.map((r) => JSON.stringify(r)).join("\n") + (records.length ? "\n" : ""));
  console.log(`Imported ${records.length} LiveBench judgments from ${files.length} files`);
  console.log(`Wrote ${out}`);
}

function summarize(records, config) {
  const byModel = new Map();
  for (const r of records) {
    if (!byModel.has(r.model)) byModel.set(r.model, []);
    byModel.get(r.model).push(r);
  }
  const rows = [];
  for (const [model, items] of byModel.entries()) {
    const scored = items.filter((r) => typeof r.score === "number");
    const solved = scored.reduce((sum, r) => sum + r.score, 0);
    const totalCost = items.reduce((sum, r) => sum + (r.cost_usd || 0), 0);
    const failures = items.filter((r) => !r.ok).length;
    rows.push({
      model,
      n: items.length,
      score: scored.length ? solved / scored.length : 0,
      solved,
      total_cost_usd: totalCost,
      avg_cost_usd: items.length ? totalCost / items.length : 0,
      cost_per_solved_usd: solved > 0 ? totalCost / solved : 0,
      p50_latency_ms: percentile(items.map((r) => r.latency_ms), 50),
      p95_latency_ms: percentile(items.map((r) => r.latency_ms), 95),
      early_exit_rate: earlyExitRate(items),
      p50_panel_max_latency_ms: fusionStageLatency(items, "panel", 50),
      p50_judge_latency_ms: fusionStageLatency(items, "judge", 50),
      p50_final_latency_ms: fusionStageLatency(items, "final", 50),
      failure_rate: items.length ? failures / items.length : 0,
      judge_json_validity: judgeValidity(items),
      all_panel_failure_rate: panelFailureRate(items),
      is_fusion: Boolean(config.fusionPresets?.[model])
    });
  }
  rows.sort((a, b) => b.score - a.score || a.total_cost_usd - b.total_cost_usd);
  return rows;
}

function judgeValidity(items) {
  const fusionItems = items.filter((r) => typeof r.judge_json_valid === "boolean");
  if (fusionItems.length === 0) return null;
  return fusionItems.filter((r) => r.judge_json_valid).length / fusionItems.length;
}

function panelFailureRate(items) {
  const fusionItems = items.filter((r) => typeof r.all_panel_failure === "boolean");
  if (fusionItems.length === 0) return null;
  return fusionItems.filter((r) => r.all_panel_failure).length / fusionItems.length;
}

function earlyExitRate(items) {
  const fusionItems = items.filter((r) => r.fusion_metrics);
  if (fusionItems.length === 0) return null;
  return fusionItems.filter((r) => r.fusion_metrics?.early_exit).length / fusionItems.length;
}

function fusionStageLatency(items, stage, p) {
  const values = items
    .map((r) => {
      if (!r.fusion_metrics) return null;
      if (stage === "panel") {
        const latencies = (r.fusion_metrics.panel || []).map((panel) => panel.latency_ms).filter(Number.isFinite);
        return latencies.length ? Math.max(...latencies) : null;
      }
      if (r.fusion_metrics?.[stage]?.skipped) return null;
      return r.fusion_metrics?.[stage]?.latency_ms;
    })
    .filter(Number.isFinite);
  if (values.length === 0) return null;
  return percentile(values, p);
}

function isChineseSingleModel(model, config) {
  if (!model || model === "openai/gpt-5.5" || config.fusionPresets?.[model]) return false;
  const upstreamName = upstreamNameForModel(model, config);
  if (["qwen", "moonshot", "deepseek", "minimax"].includes(upstreamName)) return true;
  return /^(qwen|moonshotai|deepseek|minimax)\//.test(model);
}

function chineseSingleModelsFromRecords(records, config) {
  return [
    ...new Set(
      records
        .map((r) => r.model)
        .filter((model) => isChineseSingleModel(model, config))
    )
  ];
}

function categoryRows(records) {
  const keyMap = new Map();
  for (const r of records) {
    const key = `${r.model}\t${r.category}`;
    if (!keyMap.has(key)) keyMap.set(key, []);
    keyMap.get(key).push(r);
  }
  return [...keyMap.entries()]
    .map(([key, items]) => {
      const [model, category] = key.split("\t");
      const scored = items.filter((r) => typeof r.score === "number");
      const solved = scored.reduce((sum, r) => sum + r.score, 0);
      return {
        model,
        category,
        n: items.length,
        score: scored.length ? solved / scored.length : 0
      };
    })
    .sort((a, b) => a.category.localeCompare(b.category) || b.score - a.score);
}

function comparisonRows(records, config) {
  const baselines = chineseSingleModelsFromRecords(records, config);
  const fusions = [
    ...new Set(records.map((r) => r.model).filter((model) => config.fusionPresets?.[model]))
  ];
  const byQuestionModel = new Map();
  for (const r of records) byQuestionModel.set(`${r.question_id}\t${r.model}`, r);
  const questionIds = [...new Set(records.map((r) => r.question_id))];

  return fusions.map((fusion) => {
    let degrade = 0;
    let comparable = 0;
    let winGpt = 0;
    let loseGpt = 0;
    let tieGpt = 0;
    for (const qid of questionIds) {
      const f = byQuestionModel.get(`${qid}\t${fusion}`);
      if (!f) continue;
      const bestSingleScore = Math.max(
        ...baselines
          .map((m) => byQuestionModel.get(`${qid}\t${m}`))
          .filter((r) => r?.ok && typeof r.score === "number")
          .map((r) => r.score)
      );
      if (Number.isFinite(bestSingleScore)) {
        comparable += 1;
        if ((f.score || 0) < bestSingleScore) degrade += 1;
      }
      const g = byQuestionModel.get(`${qid}\topenai/gpt-5.5`);
      if (g?.ok && f?.ok && typeof g.score === "number" && typeof f.score === "number") {
        if (f.score > g.score) winGpt += 1;
        else if (f.score < g.score) loseGpt += 1;
        else tieGpt += 1;
      }
    }
    const gptComparable = winGpt + loseGpt + tieGpt;
    return {
      fusion,
      degradation_rate_vs_best_chinese_single: comparable ? degrade / comparable : null,
      win_rate_vs_gpt55: gptComparable ? winGpt / gptComparable : null,
      wins_vs_gpt55: winGpt,
      losses_vs_gpt55: loseGpt,
      ties_vs_gpt55: tieGpt
    };
  });
}

function pairwiseRows(records) {
  const rows = records.filter((r) => r.run_type === "pairwise_eval");
  const groups = new Map();
  for (const r of rows) {
    const key = `${r.target_model}\t${r.baseline_model}\t${r.judge_model}`;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(r);
  }
  return [...groups.entries()].map(([key, items]) => {
    const [target_model, baseline_model, judge_model] = key.split("\t");
    const valid = items.filter((r) => r.target_result !== "invalid");
    const wins = valid.filter((r) => r.target_result === "win").length;
    const losses = valid.filter((r) => r.target_result === "loss").length;
    const ties = valid.filter((r) => r.target_result === "tie").length;
    const totalCost = items.reduce((sum, r) => sum + (r.cost_usd || 0), 0);
    return {
      target_model,
      baseline_model,
      judge_model,
      n: items.length,
      win_rate: valid.length ? wins / valid.length : 0,
      wins,
      losses,
      ties,
      invalid: items.length - valid.length,
      judge_json_validity: judgeValidity(items),
      total_cost_usd: totalCost,
      avg_cost_usd: items.length ? totalCost / items.length : 0,
      p50_latency_ms: percentile(items.map((r) => r.latency_ms), 50),
      p95_latency_ms: percentile(items.map((r) => r.latency_ms), 95)
    };
  });
}

function requestLogRows(records) {
  const logs = records.filter((r) => r.run_type === "request_log");
  const groups = new Map();
  for (const r of logs) {
    if (!groups.has(r.model)) groups.set(r.model, []);
    groups.get(r.model).push(r);
  }
  return [...groups.entries()]
    .map(([model, items]) => {
      const totalCost = items.reduce((sum, r) => sum + (r.cost_usd || 0), 0);
      const failures = items.filter((r) => !r.ok).length;
      return {
        model,
        n: items.length,
        total_cost_usd: totalCost,
        avg_cost_usd: items.length ? totalCost / items.length : 0,
        p50_latency_ms: percentile(items.map((r) => r.latency_ms), 50),
        p95_latency_ms: percentile(items.map((r) => r.latency_ms), 95),
        failure_rate: items.length ? failures / items.length : 0,
        judge_json_validity: judgeValidity(items),
        all_panel_failure_rate: panelFailureRate(items)
      };
    })
    .sort((a, b) => b.total_cost_usd - a.total_cost_usd);
}

const reportColumnLabels = {
  model: "模型",
  fusion: "Fusion 组合",
  category: "类别",
  n: "样本数",
  score: "得分",
  solved: "答对数",
  total_cost_usd: "总成本 USD",
  avg_cost_usd: "平均成本 USD",
  cost_per_solved_usd: "每答对成本 USD",
  p50_latency_ms: "p50 延迟 ms",
  p95_latency_ms: "p95 延迟 ms",
  early_exit_rate: "早停率",
  p50_panel_max_latency_ms: "panel 最大 p50 ms",
  p50_judge_latency_ms: "judge p50 ms",
  p50_final_latency_ms: "final p50 ms",
  failure_rate: "失败率",
  judge_json_validity: "judge JSON 有效率",
  all_panel_failure_rate: "panel 全失败率",
  degradation_rate_vs_best_chinese_single: "相对最佳中国单模型退化率",
  win_rate_vs_gpt55: "对 GPT-5.5 胜率",
  wins_vs_gpt55: "胜 GPT-5.5",
  losses_vs_gpt55: "负 GPT-5.5",
  ties_vs_gpt55: "平 GPT-5.5",
  target_model: "目标模型",
  baseline_model: "基准模型",
  judge_model: "裁判模型",
  win_rate: "胜率",
  wins: "胜",
  losses: "负",
  ties: "平",
  invalid: "无效"
};

const reportCategoryLabels = {
  business_reasoning: "业务推理",
  coding: "代码",
  data_analysis: "数据分析",
  instruction_following: "指令遵循",
  long_context: "长上下文",
  reasoning: "推理",
  language: "语言",
  math: "数学"
};

function markdownTable(rows, columns) {
  const header = `| ${columns.map((col) => reportColumnLabels[col] || col).join(" | ")} |`;
  const sep = `| ${columns.map(() => "---").join(" | ")} |`;
  const lines = rows.map((row) =>
    `| ${columns
      .map((col) => {
        const v = row[col];
        if (typeof v === "number") {
          if (col.includes("usd")) return v.toFixed(6);
          if (col.includes("rate") || col.includes("score") || col.includes("validity")) {
            return `${(v * 100).toFixed(1)}%`;
          }
          return Number.isInteger(v) ? String(v) : v.toFixed(2);
        }
        if (v === null || v === undefined) return "";
        if (col === "category") return reportCategoryLabels[v] || String(v);
        return String(v);
      })
      .join(" | ")} |`
  );
  return [header, sep, ...lines].join("\n");
}

function report(argv) {
  const config = loadConfig(argv);
  const inputs = valueOf(argv, "--inputs");
  if (!inputs) throw new Error("Missing --inputs");
  const out = valueOf(
    argv,
    "--out",
    path.join(__dirname, "fusion-benchmark", "reports", `report-${Date.now()}.md`)
  );
  const files = inputs.split(",").map((f) => f.trim()).filter(Boolean);
  const records = files.flatMap(parseJsonl);
  const objectiveRecords = records.filter((r) => r.run_type !== "pairwise_eval" && r.run_type !== "request_log" && r.model && typeof r.score === "number");
  const modelRows = summarize(objectiveRecords, config);
  const catRows = categoryRows(objectiveRecords);
  const compRows = comparisonRows(objectiveRecords, config);
  const pairRows = pairwiseRows(records);
  const requestRows = requestLogRows(records);
  const bestChinese = modelRows.find(
    (r) => isChineseSingleModel(r.model, config) && r.failure_rate < 1
  );
  const gpt = modelRows.find((r) => r.model === "openai/gpt-5.5");
  const gptComparable = Boolean(gpt && gpt.n > 0 && gpt.failure_rate < 1);
  const objectiveQuestionCount = new Set(objectiveRecords.map((r) => r.question_id)).size;
  const minArchitectureQuestions = config.successCriteria.minObjectiveQuestionsForArchitecture || 100;
  const enoughEvidenceForArchitecture = objectiveQuestionCount >= minArchitectureQuestions;

  const verdictLines = [];
  for (const row of modelRows.filter((r) => r.is_fusion)) {
    const lift = bestChinese ? row.score / Math.max(bestChinese.score, 0.000001) - 1 : 0;
    const costRatio = gptComparable && gpt.total_cost_usd > 0 ? row.total_cost_usd / gpt.total_cost_usd : null;
    const nearGpt = gptComparable ? row.score >= gpt.score * config.successCriteria.nearGpt55ScoreRatio : false;
    const passLift = lift >= config.successCriteria.minLiftVsBestChineseSingle;
    const beatsGpt = gptComparable ? row.score > gpt.score : false;
    const passNearGptCost = nearGpt && costRatio !== null && costRatio <= config.successCriteria.maxCostRatioVsGpt55;
    const shouldProceed = passLift || beatsGpt || passNearGptCost;
    const verdict = shouldProceed ? (enoughEvidenceForArchitecture ? "可进入架构设计" : "扩大评测") : "暂缓";
    verdictLines.push(
      `- ${row.model}: ${verdict}；相对最佳中国单模型提升=${(lift * 100).toFixed(
        1
      )}%${bestChinese ? `（${bestChinese.model}）` : ""}；相对 GPT-5.5 成本=${
        costRatio === null ? "无对照" : `${(costRatio * 100).toFixed(1)}%`
      }；证据量=${objectiveQuestionCount}/${minArchitectureQuestions}；GPT-5.5 对照=${
        gptComparable ? "可用" : "不可用"
      }`
    );
  }

  const md = [
    "# Fusion 评测报告",
    "",
    `生成时间：${new Date().toISOString()}`,
    "",
    "## 结论",
    "",
    verdictLines.join("\n") || "- 没有找到 Fusion 评测记录。",
    "",
    "## 模型汇总",
    "",
    markdownTable(modelRows, [
      "model",
      "n",
      "score",
      "solved",
      "total_cost_usd",
      "avg_cost_usd",
      "cost_per_solved_usd",
      "p50_latency_ms",
      "p95_latency_ms",
      "early_exit_rate",
      "p50_panel_max_latency_ms",
      "p50_judge_latency_ms",
      "p50_final_latency_ms",
      "failure_rate",
      "judge_json_validity",
      "all_panel_failure_rate"
    ]),
    "",
    "## 分类汇总",
    "",
    markdownTable(catRows, ["category", "model", "n", "score"]),
    "",
    "## Fusion 对比",
    "",
    markdownTable(compRows, [
      "fusion",
      "degradation_rate_vs_best_chinese_single",
      "win_rate_vs_gpt55",
      "wins_vs_gpt55",
      "losses_vs_gpt55",
      "ties_vs_gpt55"
    ]),
    "",
    "## 两两对比",
    "",
    pairRows.length
      ? markdownTable(pairRows, [
          "target_model",
          "baseline_model",
          "judge_model",
          "n",
          "win_rate",
          "wins",
          "losses",
          "ties",
          "invalid",
          "judge_json_validity",
          "total_cost_usd",
          "avg_cost_usd",
          "p50_latency_ms",
          "p95_latency_ms"
        ])
      : "- 没有找到两两对比记录。",
    "",
    "## 请求成本与延迟",
    "",
    requestRows.length
      ? markdownTable(requestRows, [
          "model",
          "n",
          "total_cost_usd",
          "avg_cost_usd",
          "p50_latency_ms",
          "p95_latency_ms",
          "failure_rate",
          "judge_json_validity",
          "all_panel_failure_rate"
        ])
      : "- 没有找到请求日志记录。",
    ""
  ].join("\n");

  fs.mkdirSync(path.dirname(out), { recursive: true });
  fs.writeFileSync(out, md);
  console.log(`Wrote ${out}`);
}

function livebenchPlan(argv) {
  const config = loadConfig(argv);
  const apiBase = valueOf(argv, "--api-base", "http://127.0.0.1:8787/v1");
  const models = [...config.baselines, ...Object.keys(config.fusionPresets)];
  const releases = [...config.livebench.preferredReleases, config.livebench.publicAnchorRelease];
  console.log("# LiveBench command plan");
  console.log("");
  console.log("Start proxy first:");
  console.log("node tools/fusion-benchmark.mjs serve --port 8787 --env-file tools/fusion-benchmark/.env.local --log tools/fusion-benchmark/runs/proxy-requests.jsonl");
  console.log("");
  for (const release of releases) {
    console.log(`## Release ${release}`);
    for (const model of models) {
      console.log(
        `python run_livebench.py --model ${model} --api-base ${apiBase} --api-key benchmark-local --bench-name ${config.livebench.categories.join(
          " "
        )} --livebench-release-option ${release} --parallel-requests 2 --resume --retry-failures`
      );
    }
    console.log("");
  }
}

async function modelsList(argv) {
  const config = loadConfig(argv);
  const upstreamName = valueOf(argv, "--upstream", "default");
  const { ids } = await fetchUpstreamModels(config, upstreamName);
  const filter = valueOf(argv, "--filter", "qwen|kimi|moonshot|deepseek|minimax|gpt|openai|claude|anthropic|gemini|google");
  const re = filter ? new RegExp(filter, "i") : null;
  const filtered = re ? ids.filter((id) => re.test(id)) : ids;
  console.log(`Upstream: ${upstreamName}`);
  console.log(`API base: found from ${configuredApiBaseSource(config, upstreamName)}`);
  console.log(`models_total=${ids.length}`);
  console.log(`filtered_total=${filtered.length}`);
  for (const id of filtered) console.log(id);
}

async function fetchUpstreamModels(config, upstreamName = "default") {
  const apiBase = configuredApiBase(config, upstreamName);
  const apiKey = upstreamAuthKey(config, upstreamName);
  if (!apiBase) throw new Error("Missing API base URL");
  if (!apiKey) throw new Error(`Missing API key for upstream ${upstreamName}`);
  const resp = await fetch(`${normalizeApiBase(apiBase)}/models`, {
    headers: { authorization: `Bearer ${apiKey}` }
  });
  const text = await resp.text();
  let json;
  try {
    json = JSON.parse(text);
  } catch {
    throw new Error(`Models endpoint returned non-JSON status=${resp.status}`);
  }
  if (!resp.ok) {
    const msg = JSON.stringify(json.error || json).replace(/https?:\/\/\S+/g, "[redacted-url]").slice(0, 500);
    throw new Error(`Models endpoint failed status=${resp.status} message=${msg}`);
  }
  const models = Array.isArray(json.data) ? json.data : Array.isArray(json) ? json : [];
  const ids = models.map((m) => m.id || m.name || m.model || "").filter(Boolean).sort();
  return { ids, models };
}

async function modelsProbe(argv) {
  const config = loadConfig(argv);
  const upstreamName = valueOf(argv, "--upstream", "default");
  const { ids } = await fetchUpstreamModels(config, upstreamName);
  const filter = valueOf(argv, "--filter", "");
  const limit = Number(valueOf(argv, "--limit", "50"));
  const timeoutMs = Number(valueOf(argv, "--timeout-ms", "20000"));
  const re = filter ? new RegExp(filter, "i") : null;
  const selected = (re ? ids.filter((id) => re.test(id)) : ids).slice(0, limit);
  console.log(`Upstream: ${upstreamName}`);
  console.log(`API base: found from ${configuredApiBaseSource(config, upstreamName)}`);
  console.log(`probe_total=${selected.length}`);
  for (const model of selected) {
    try {
      const response = await callChat({
        apiBase: configuredApiBase(config, upstreamName),
        apiKey: upstreamAuthKey(config, upstreamName),
        body: {
          model,
          messages: [{ role: "user", content: "Reply ok" }],
          temperature: 0,
          max_tokens: 3
        },
        timeoutMs
      });
      const answer = extractContent(response).replace(/\s+/g, " ").slice(0, 60);
      console.log(`ok\t${model}\t${answer}`);
    } catch (err) {
      const msg = String(err.message || "").replace(/https?:\/\/\S+/g, "[redacted-url]").slice(0, 160);
      console.log(`fail\t${model}\t${msg}`);
    }
  }
}

async function latencyProbe(argv) {
  const config = loadConfig(argv);
  const modelArg = valueOf(argv, "--models");
  const rounds = Math.max(1, Number(valueOf(argv, "--rounds", "3")) || 3);
  const timeout = Math.max(1000, Number(valueOf(argv, "--timeout-ms", "60000")) || 60000);
  const models = modelArg
    ? modelArg.split(",").map((m) => m.trim()).filter(Boolean)
    : collectConfiguredModels(config).filter((model) => !config.fusionPresets?.[model]);
  const rows = [];

  for (const model of models) {
    const samples = [];
    for (let i = 0; i < rounds; i += 1) {
      const started = Date.now();
      try {
        const response = await callBenchmarkModel(model, {
          messages: [{ role: "user", content: "Reply exactly: ok" }],
          temperature: 0,
          max_tokens: 8
        }, {
          ...config,
          timeouts: {
            ...(config.timeouts || {}),
            passthroughMs: timeout,
            panelMs: timeout,
            judgeMs: timeout,
            finalMs: timeout
          }
        });
        samples.push({
          ok: true,
          latency_ms: response._latency_ms ?? Date.now() - started,
          cost_usd: response.fusion_metrics?.cost_usd ?? tokenCost(model, response.usage, config),
          answer: extractContent(response).replace(/\s+/g, " ").slice(0, 40)
        });
      } catch (err) {
        samples.push({
          ok: false,
          latency_ms: Date.now() - started,
          cost_usd: 0,
          error: String(err.message || "").replace(/https?:\/\/\S+/g, "[redacted-url]").slice(0, 160)
        });
      }
      console.log(`${samples.at(-1).ok ? "ok" : "fail"} ${model} round=${i + 1}/${rounds} latency_ms=${samples.at(-1).latency_ms}`);
    }
    const okSamples = samples.filter((s) => s.ok);
    rows.push({
      model,
      rounds,
      ok: okSamples.length,
      failures: samples.length - okSamples.length,
      failure_rate: samples.length ? (samples.length - okSamples.length) / samples.length : 0,
      p50_latency_ms: percentile(okSamples.map((s) => s.latency_ms), 50),
      p95_latency_ms: percentile(okSamples.map((s) => s.latency_ms), 95),
      avg_cost_usd: samples.length ? samples.reduce((sum, s) => sum + s.cost_usd, 0) / samples.length : 0
    });
  }

  console.log("");
  console.log(markdownTable(rows, [
    "model",
    "rounds",
    "ok",
    "failures",
    "failure_rate",
    "p50_latency_ms",
    "p95_latency_ms",
    "avg_cost_usd"
  ]));
}

function validateConfig(argv) {
  const config = loadConfig(argv);
  const errors = [];
  const warnings = [];
  const models = collectConfiguredModels(config);
  const upstreamNames = new Set(["default", ...Object.keys(config.upstreams || {})]);
  for (const model of models) {
    if (config.fusionPresets?.[model]) continue;
    upstreamNames.add(upstreamNameForModel(model, config));
  }

  if (!config.apiBase) errors.push("Missing config.apiBase");
  if (!config.apiKeyEnv) warnings.push("Missing config.apiKeyEnv; OPENAI_API_KEY fallback still works.");
  if (!config.apiBaseEnv) warnings.push("Missing config.apiBaseEnv; OPENAI_BASE_URL fallback still works.");

  for (const upstreamName of upstreamNames) {
    if (upstreamName !== "default" && !config.upstreams?.[upstreamName]) {
      errors.push(`Missing upstream config: ${upstreamName}`);
      continue;
    }
    if (!configuredApiBase(config, upstreamName)) {
      warnings.push(`${upstreamName}: API base not found.`);
    }
    if (!upstreamAuthKey(config, upstreamName)) {
      warnings.push(`${upstreamName}: API key not found.`);
    }
  }

  for (const [name, preset] of Object.entries(config.fusionPresets || {})) {
    if (!Array.isArray(preset.panel) || preset.panel.length < 2) {
      errors.push(`${name}: panel must include at least two models.`);
    }
    if (!preset.judge) errors.push(`${name}: missing judge model.`);
    if (!preset.final) errors.push(`${name}: missing final model.`);
    if (preset.panel?.includes(name)) errors.push(`${name}: preset cannot include itself in panel.`);
  }

  for (const model of models) {
    if (config.fusionPresets?.[model]) continue;
    if (!config.pricing?.[model]) warnings.push(`${model}: missing pricing, cost metrics will be zero.`);
  }

  const defaultJudge = config.pairwise?.defaultJudge;
  const comparedDefaults = [config.pairwise?.defaultTarget, config.pairwise?.defaultBaseline].filter(Boolean);
  if (defaultJudge && comparedDefaults.includes(defaultJudge)) {
    warnings.push("Pairwise default judge is also a compared model; use an external judge for cleaner GPT-5.5 comparison.");
  }
  if (defaultJudge === "openai/gpt-5.5") {
    warnings.push("Pairwise judge is openai/gpt-5.5; do not use GPT-5.5 to judge GPT-5.5 comparisons.");
  }

  console.log("Fusion benchmark config validation");
  console.log(`Config: ${valueOf(argv, "--config") || defaultConfigPath}`);
  console.log("Upstreams:");
  for (const upstreamName of [...upstreamNames].sort()) {
    const baseSource = configuredApiBase(config, upstreamName)
      ? `found from ${configuredApiBaseSource(config, upstreamName)}`
      : "missing";
    const keySource = upstreamAuthKey(config, upstreamName)
      ? `found from ${configuredApiKeySource(config, upstreamName)}`
      : "missing";
    console.log(`- ${upstreamName}: base ${baseSource}; key ${keySource}`);
  }
  console.log(`Baselines: ${(config.baselines || []).join(", ")}`);
  console.log(`Fusion presets: ${Object.keys(config.fusionPresets || {}).join(", ")}`);
  if (Object.keys(config.modelAliases || {}).length) {
    console.log(`Model aliases: ${Object.keys(config.modelAliases).length}`);
  }
  if (Object.keys(config.modelUpstreams || {}).length) {
    console.log(`Model upstream routes: ${Object.keys(config.modelUpstreams).length}`);
  }
  console.log(`Warnings: ${warnings.length}`);
  for (const warning of warnings) console.log(`- WARN ${warning}`);
  console.log(`Errors: ${errors.length}`);
  for (const error of errors) console.log(`- ERROR ${error}`);
  if (errors.length) process.exitCode = 1;
}

function usage() {
  console.log(`Usage:
  node tools/fusion-benchmark.mjs validate-config [--env-file FILE]
  node tools/fusion-benchmark.mjs models-list [--env-file FILE] [--upstream NAME] [--filter REGEX]
  node tools/fusion-benchmark.mjs models-probe [--env-file FILE] [--upstream NAME] [--filter REGEX] [--limit N] [--timeout-ms 20000]
  node tools/fusion-benchmark.mjs latency-probe [--env-file FILE] [--models a,b,c] [--rounds N] [--timeout-ms 60000]
  node tools/fusion-benchmark.mjs serve [--port 8787] [--upstream-api-base URL] [--log FILE] [--env-file FILE]
  node tools/fusion-benchmark.mjs livebench-plan [--api-base URL]
  node tools/fusion-benchmark.mjs livebench-import --inputs FILE_OR_DIR[,FILE_OR_DIR] [--release YYYY-MM-DD] [--out FILE]
  node tools/fusion-benchmark.mjs fresh-run --dataset FILE [--out FILE] [--api-base URL] [--models a,b,c] [--category a,b] [--offset N] [--limit N] [--env-file FILE]
  node tools/fusion-benchmark.mjs pairwise-run --dataset FILE [--out FILE] [--api-base URL] [--target MODEL] [--baseline MODEL] [--judge MODEL] [--env-file FILE]
  node tools/fusion-benchmark.mjs report --inputs FILE[,FILE] [--out FILE]

Global:
  --config FILE  Override tools/fusion-benchmark/config.json
  --env-file FILE  Load API keys from a local env file without printing them
`);
}

async function main() {
  const [, , cmd, ...argv] = process.argv;
  if (!cmd || cmd === "--help" || cmd === "-h" || hasFlag(argv, "--help") || cmd === "help") {
    usage();
    return;
  }
  loadEnvFile(argv);
  if (cmd === "validate-config") return validateConfig(argv);
  if (cmd === "models-list") return modelsList(argv);
  if (cmd === "models-probe") return modelsProbe(argv);
  if (cmd === "latency-probe") return latencyProbe(argv);
  if (cmd === "serve") return serve(argv);
  if (cmd === "livebench-plan") return livebenchPlan(argv);
  if (cmd === "livebench-import") return livebenchImport(argv);
  if (cmd === "fresh-run") return freshRun(argv);
  if (cmd === "pairwise-run") return pairwiseRun(argv);
  if (cmd === "report") return report(argv);
  throw new Error(`Unknown command: ${cmd}`);
}

main().catch((err) => {
  console.error(err.stack || err.message);
  process.exit(1);
});
