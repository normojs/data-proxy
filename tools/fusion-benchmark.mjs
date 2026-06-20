#!/usr/bin/env node

import fs from "node:fs";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";
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

function wilsonInterval(successes, n, z = 1.96) {
  if (!Number.isFinite(successes) || !Number.isFinite(n) || n <= 0) {
    return { low: null, high: null };
  }
  const p = successes / n;
  const z2 = z * z;
  const denom = 1 + z2 / n;
  const centre = p + z2 / (2 * n);
  const margin = z * Math.sqrt((p * (1 - p) + z2 / (4 * n)) / n);
  return {
    low: Math.max(0, (centre - margin) / denom),
    high: Math.min(1, (centre + margin) / denom)
  };
}

function binaryScoreInterval(items) {
  if (!items.length || items.some((r) => r.score !== 0 && r.score !== 1)) {
    return { low: null, high: null };
  }
  const successes = items.filter((r) => r.score === 1).length;
  return wilsonInterval(successes, items.length);
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

function numberOrNull(value) {
  if (value === null || value === undefined || value === "") return null;
  const n = Number(value);
  return Number.isFinite(n) ? n : null;
}

function getPathValue(obj, pathText) {
  if (!obj || !pathText) return undefined;
  return String(pathText)
    .split(".")
    .reduce((current, part) => (current && Object.prototype.hasOwnProperty.call(current, part) ? current[part] : undefined), obj);
}

function costValueToUsd(value, unit, config) {
  const n = numberOrNull(value);
  if (n === null) return null;
  const normalizedUnit = String(unit || "usd").toLowerCase();
  if (normalizedUnit === "usd") return n;
  if (normalizedUnit === "quota") {
    const quotaPerUsd = Number(config.billing?.quotaPerUsd ?? 500000);
    return quotaPerUsd > 0 ? n / quotaPerUsd : null;
  }
  if (normalizedUnit === "cny") {
    const cnyPerUsd = Number(config.billing?.cnyPerUsd);
    return cnyPerUsd > 0 ? n / cnyPerUsd : null;
  }
  return null;
}

function defaultActualCostSources() {
  return [
    { type: "body", path: "usage.cost_usd", unit: "usd" },
    { type: "body", path: "usage.total_cost_usd", unit: "usd" },
    { type: "body", path: "usage.actual_cost_usd", unit: "usd" },
    { type: "body", path: "billing.cost_usd", unit: "usd" },
    { type: "body", path: "billing.total_cost_usd", unit: "usd" },
    { type: "body", path: "cost_usd", unit: "usd" },
    { type: "body", path: "total_cost_usd", unit: "usd" },
    { type: "body", path: "usage.quota", unit: "quota" },
    { type: "body", path: "billing.quota", unit: "quota" },
    { type: "body", path: "quota", unit: "quota" },
    { type: "header", name: "x-revo-actual-cost-usd", unit: "usd" },
    { type: "header", name: "x-openrouter-generation-cost", unit: "usd" },
    { type: "header", name: "x-usage-cost-usd", unit: "usd" },
    { type: "header", name: "x-cost-usd", unit: "usd" },
    { type: "header", name: "x-oneapi-used-quota", unit: "quota" },
    { type: "header", name: "x-oneapi-used-amount", unit: "quota" },
    { type: "header", name: "x-new-api-used-quota", unit: "quota" },
    { type: "header", name: "x-new-api-used-amount", unit: "quota" }
  ];
}

function extractActualCostUsd(json, headers, config) {
  const sources = [
    ...defaultActualCostSources(),
    ...(Array.isArray(config.billing?.actualCostSources) ? config.billing.actualCostSources : [])
  ];
  for (const source of sources) {
    const type = String(source.type || "body").toLowerCase();
    const raw =
      type === "header"
        ? headers?.get?.(String(source.name || "").toLowerCase())
        : getPathValue(json, source.path);
    const usd = costValueToUsd(raw, source.unit, config);
    if (usd !== null) {
      return {
        actual_cost_usd: usd,
        actual_cost_source: type === "header" ? `header:${String(source.name || "").toLowerCase()}` : `body:${source.path}`,
        actual_cost_unit: source.unit || "usd"
      };
    }
  }
  return {
    actual_cost_usd: null,
    actual_cost_source: null,
    actual_cost_unit: null
  };
}

const billingLocks = new Map();

async function withBillingLock(key, fn) {
  const previous = billingLocks.get(key) || Promise.resolve();
  let release;
  const next = new Promise((resolve) => {
    release = resolve;
  });
  const chained = previous.then(() => next, () => next);
  billingLocks.set(key, chained);
  await previous.catch(() => {});
  try {
    return await fn();
  } finally {
    release();
    if (billingLocks.get(key) === chained) billingLocks.delete(key);
  }
}

function balanceDeltaConfig(config, upstreamName) {
  const cfg = config.billing?.balanceDelta;
  if (!cfg?.enabled) return null;
  const upstreams = Array.isArray(cfg.upstreams) ? cfg.upstreams : [];
  if (upstreams.length && !upstreams.includes(upstreamName)) return null;
  return {
    path: cfg.path || "/user/info",
    balancePath: cfg.balancePath || "data.chargeBalance",
    unit: cfg.unit || "cny",
    prefer: Boolean(cfg.prefer),
    timeoutMs: Math.max(1000, Number(cfg.timeoutMs || 10000))
  };
}

function runBalanceDeltaConfig(config) {
  const cfg = config.billing?.runBalanceDelta;
  const fallback = config.billing?.balanceDelta || {};
  if (cfg?.enabled === false) return null;
  if (cfg?.enabled !== true && fallback?.runEnabled !== true) return null;
  return {
    path: cfg?.path || fallback.path || "/user/info",
    balancePath: cfg?.balancePath || fallback.balancePath || "data.chargeBalance",
    unit: cfg?.unit || fallback.unit || "cny",
    upstreams: Array.isArray(cfg?.upstreams)
      ? cfg.upstreams
      : Array.isArray(fallback.upstreams)
        ? fallback.upstreams
        : [],
    timeoutMs: Math.max(1000, Number(cfg?.timeoutMs || fallback.timeoutMs || 10000))
  };
}

async function fetchBillingBalance(apiBase, apiKey, cfg) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), cfg.timeoutMs);
  try {
    const resp = await fetch(`${normalizeApiBase(apiBase)}${cfg.path.startsWith("/") ? cfg.path : `/${cfg.path}`}`, {
      headers: { authorization: `Bearer ${apiKey}` },
      signal: controller.signal
    });
    const text = await resp.text();
    let json = null;
    try {
      json = JSON.parse(text);
    } catch {}
    if (!resp.ok || !json) {
      return { ok: false, status: resp.status, balance: null, keys: json && typeof json === "object" ? Object.keys(json) : [] };
    }
    return {
      ok: true,
      status: resp.status,
      balance: numberOrNull(getPathValue(json, cfg.balancePath)),
      keys: Object.keys(json || {}),
      dataKeys: json?.data && typeof json.data === "object" ? Object.keys(json.data) : []
    };
  } finally {
    clearTimeout(timeout);
  }
}

async function takeRunBillingSnapshot(config, cfg) {
  if (!cfg) return [];
  const upstreamNames = cfg.upstreams.length ? cfg.upstreams : Object.keys(config.upstreams || {});
  const accountGroups = new Map();
  const snapshots = [];
  for (const upstreamName of upstreamNames) {
    const apiBase = configuredApiBase(config, upstreamName);
    const apiKey = upstreamAuthKey(config, upstreamName);
    if (!apiBase || !apiKey) {
      snapshots.push({
        accountKey: `missing:${upstreamName}`,
        upstreamName,
        upstreams: [upstreamName],
        ok: false,
        status: null,
        balance: null,
        error: "missing_config"
      });
      continue;
    }
    const accountKey = `${normalizeApiBase(apiBase)}:${apiKey}`;
    const existing = accountGroups.get(accountKey);
    if (existing) {
      existing.upstreams.push(upstreamName);
      existing.upstreamName = existing.upstreams.join(",");
      continue;
    }
    accountGroups.set(accountKey, { accountKey, upstreamName, upstreams: [upstreamName], apiBase, apiKey });
  }

  for (const group of accountGroups.values()) {
    try {
      const result = await fetchBillingBalance(group.apiBase, group.apiKey, cfg);
      snapshots.push({
        accountKey: group.accountKey,
        upstreamName: group.upstreamName,
        upstreams: group.upstreams,
        ok: result.ok && numberOrNull(result.balance) !== null,
        status: result.status,
        balance: numberOrNull(result.balance),
        error: result.ok ? null : "balance_not_found"
      });
    } catch (err) {
      snapshots.push({
        accountKey: group.accountKey,
        upstreamName: group.upstreamName,
        upstreams: group.upstreams,
        ok: false,
        status: null,
        balance: null,
        error: String(err.message || err).slice(0, 120)
      });
    }
  }
  return snapshots;
}

function buildRunBillingRecords({ runId, runType, cfg, before, after, config }) {
  if (!cfg) return [];
  const afterByAccount = new Map(after.map((item) => [item.accountKey || item.upstreamName, item]));
  return before.map((start) => {
    const end = afterByAccount.get(start.accountKey || start.upstreamName) || {};
    const beforeBalance = numberOrNull(start.balance);
    const afterBalance = numberOrNull(end.balance);
    const nativeDelta =
      beforeBalance !== null && afterBalance !== null ? Math.max(0, beforeBalance - afterBalance) : null;
    const actualCostUsd = costValueToUsd(nativeDelta, cfg.unit, config);
    return {
      run_type: "billing_summary",
      source_run_type: runType,
      run_id: runId,
      upstream: start.upstreamName,
      upstreams: start.upstreams || [start.upstreamName],
      billing_source: `balance-delta:${cfg.path}:${cfg.balancePath}`,
      ok: nativeDelta !== null && actualCostUsd !== null,
      unit: cfg.unit,
      actual_cost_native: nativeDelta,
      actual_cost_usd: actualCostUsd,
      before_status: start.status ?? null,
      after_status: end.status ?? null,
      error: start.error || end.error || undefined
    };
  });
}

function costFields(model, response, config) {
  const estimated = tokenCost(model, response?.usage, config);
  const actual = numberOrNull(response?._actual_cost_usd);
  return {
    cost_usd: estimated,
    estimated_cost_usd: estimated,
    actual_cost_usd: actual,
    actual_cost_source: response?._actual_cost_source || null,
    actual_cost_unit: response?._actual_cost_unit || null
  };
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

function sumEstimatedCost(parts) {
  return parts.reduce((sum, part) => sum + (part?.estimated_cost_usd ?? part?.cost_usd ?? 0), 0);
}

function sumActualCost(parts) {
  const billableParts = parts.filter((part) => {
    if (!part || part.ok === false) return false;
    const estimated = numberOrNull(part.estimated_cost_usd ?? part.cost_usd);
    return Boolean(part.usage) || (estimated !== null && estimated > 0) || numberOrNull(part.actual_cost_usd) !== null;
  });
  if (billableParts.length === 0) return null;
  if (billableParts.some((part) => numberOrNull(part.actual_cost_usd) === null)) return null;
  return billableParts.reduce((sum, part) => sum + part.actual_cost_usd, 0);
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
  const estimatedCostUsd = sumEstimatedCost(panelResults);
  const actualCostUsd = sumActualCost(panelResults);
  return {
    answer: winner[0].content.trim(),
    agreed_models: winner.map((r) => r.model),
    usage,
    cost_usd: estimatedCostUsd,
    estimated_cost_usd: estimatedCostUsd,
    actual_cost_usd: actualCostUsd
  };
}

async function callChat({
  apiBase,
  apiKey,
  body,
  extraHeaders = {},
  timeoutMs = 120000,
  retries = 2,
  config,
  upstreamName = "default",
  skipBalanceDelta = false,
  signal
}) {
  const balanceCfg = config && !skipBalanceDelta ? balanceDeltaConfig(config, upstreamName) : null;
  if (balanceCfg) {
    const lockKey = `${normalizeApiBase(apiBase)}:${apiKey}`;
    return withBillingLock(lockKey, async () => {
      const before = await fetchBillingBalance(apiBase, apiKey, balanceCfg).catch(() => ({ ok: false, balance: null }));
      const json = await callChat({
        apiBase,
        apiKey,
        body,
        extraHeaders,
        timeoutMs,
        retries,
        config,
        upstreamName,
        skipBalanceDelta: true,
        signal
      });
      const after = await fetchBillingBalance(apiBase, apiKey, balanceCfg).catch(() => ({ ok: false, balance: null }));
      const beforeBalance = numberOrNull(before.balance);
      const afterBalance = numberOrNull(after.balance);
      const nativeDelta =
        beforeBalance !== null && afterBalance !== null ? Math.max(0, beforeBalance - afterBalance) : null;
      const actualCostUsd = costValueToUsd(nativeDelta, balanceCfg.unit, config);
      if (actualCostUsd !== null && (balanceCfg.prefer || numberOrNull(json._actual_cost_usd) === null)) {
        json._actual_cost_usd = actualCostUsd;
        json._actual_cost_source = `balance-delta:${balanceCfg.path}:${balanceCfg.balancePath}`;
        json._actual_cost_unit = balanceCfg.unit;
      }
      return json;
    });
  }

  const started = Date.now();
  let lastErr;
  for (let attempt = 0; attempt <= retries; attempt += 1) {
    if (signal?.aborted) {
      throw signal.reason instanceof Error ? signal.reason : new Error("Request aborted");
    }
    const controller = new AbortController();
    const abortFromParent = () => controller.abort(signal.reason);
    signal?.addEventListener("abort", abortFromParent, { once: true });
    const timeout = setTimeout(() => controller.abort(), timeoutMs);
    let resp;
    let text;
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
      text = await resp.text();
    } catch (err) {
      if (err?.name === "AbortError") {
        lastErr = signal?.aborted && signal.reason instanceof Error
          ? signal.reason
          : new Error(`Request timed out after ${timeoutMs}ms`);
      } else {
        lastErr = err;
      }
      clearTimeout(timeout);
      signal?.removeEventListener("abort", abortFromParent);
      if (attempt < retries && isRetryableError(lastErr)) {
        await sleep(500 * 2 ** attempt);
        continue;
      }
      throw lastErr;
    } finally {
      clearTimeout(timeout);
      signal?.removeEventListener("abort", abortFromParent);
    }
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
    const actualCost = config ? extractActualCostUsd(json, resp.headers, config) : {};
    json._latency_ms = Date.now() - started;
    json._actual_cost_usd = actualCost.actual_cost_usd ?? null;
    json._actual_cost_source = actualCost.actual_cost_source ?? null;
    json._actual_cost_unit = actualCost.actual_cost_unit ?? null;
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

function taskFamilyForContext(context = {}) {
  const scoringType = context.scoring?.type || context.scoring_type;
  if (["code_exec", "patch_exec"].includes(scoringType)) return "coding";
  if (["exact", "contains", "regex"].includes(scoringType)) return "objective";
  const module = String(context.module || "").toLowerCase();
  if (module === "coding") return "coding";
  if (module === "long_context") return "long_context";
  if (module === "instruction_following") return "instruction_following";
  if (module === "reasoning") return "reasoning";
  if (module === "chinese" || module === "english") return "language";
  return "open_synthesis";
}

function dedupeModels(models) {
  const seen = new Set();
  const out = [];
  for (const model of models || []) {
    if (!model || seen.has(model)) continue;
    seen.add(model);
    out.push(model);
  }
  return out;
}

function tierProfileForPreset(presetName, preset = {}) {
  const explicitTier = preset.tier || preset.profile;
  if (explicitTier) return String(explicitTier);
  if (presetName.endsWith("-fast") || presetName.endsWith("-cheap") || presetName.endsWith("-qwen-dual")) return "fast";
  if (presetName.endsWith("-pro")) return "pro";
  return "budget";
}

function preferredPortfolioModels(taskFamily, preset, config, presetName) {
  const profile = tierProfileForPreset(presetName, preset);
  const rule = config.fusionPortfolio?.[taskFamily] || config.fusionPortfolio?.default || {};
  const preferred = rule[profile] || rule.models || [];
  const fallback = preset.panel || [];
  const maxPanel = Number(rule.maxPanelByTier?.[profile] ?? preset.maxPanel ?? fallback.length);
  const ordered = dedupeModels([...preferred, ...fallback]);
  return ordered.slice(0, Math.max(2, maxPanel || fallback.length));
}

function routeFusionPortfolio({ presetName, preset, config, context = {} }) {
  const taskFamily = taskFamilyForContext(context);
  const preferred = preferredPortfolioModels(taskFamily, preset, config, presetName);
  const available = new Set(preset.panel || []);
  const included = preferred.filter((model) => available.has(model));
  const fallback = (preset.panel || []).filter((model) => !included.includes(model));
  const minPanel = Math.min(Math.max(2, Number(preset.minPanel || 2)), (preset.panel || []).length);
  const panel = dedupeModels([...included, ...fallback]).slice(0, Math.max(minPanel, included.length || minPanel));
  return {
    task_family: taskFamily,
    profile: tierProfileForPreset(presetName, preset),
    panel,
    included: panel.map((model) => ({
      model,
      reason: included.includes(model)
        ? `preferred_for_${taskFamily}`
        : panel.length <= minPanel
          ? "needed_for_min_panel"
          : "preset_fallback"
    })),
    skipped: (preset.panel || [])
      .filter((model) => !panel.includes(model))
      .map((model) => ({
        model,
        reason: `not_selected_for_${taskFamily}_${tierProfileForPreset(presetName, preset)}`
      }))
  };
}

function defaultPolicyForTaskFamily(taskFamily) {
  if (taskFamily === "objective") return "objective";
  if (taskFamily === "coding") return "verified_coding";
  return "panel_judge_final";
}

function fusionModeForPolicy(policy) {
  if (String(policy || "").startsWith("verified_coding")) return "verified_fusion_mode";
  return "fusion_model_mode";
}

function panelSummary(panelResults) {
  return (panelResults || []).map((p) => ({
    model: p.model,
    ok: p.ok,
    error: p.error,
    attempt: p.attempt,
    latency_ms: p.latency_ms,
    cost_usd: p.cost_usd,
    estimated_cost_usd: p.estimated_cost_usd,
    actual_cost_usd: p.actual_cost_usd,
    actual_cost_source: p.actual_cost_source
  }));
}

function compactFusionMetrics(metrics) {
  if (!metrics) return undefined;
  return {
    ...metrics,
    panel: panelSummary(metrics.panel || []),
    candidate_verifications: Array.isArray(metrics.candidate_verifications)
      ? metrics.candidate_verifications.map((candidate) => ({
          model: candidate.model,
          attempt: candidate.attempt,
          ok: candidate.ok,
          passed: candidate.passed,
          score: candidate.score,
          score_reason: candidate.score_reason,
          failure_type: candidate.failure_type,
          execution_log: candidate.execution_log,
          execution_latency_ms: candidate.execution_latency_ms,
          workspace: candidate.workspace,
          estimated_cost_usd: candidate.estimated_cost_usd,
          actual_cost_usd: candidate.actual_cost_usd,
          patch_risk: candidate.patch_risk,
          content_chars: candidate.content_chars,
          error: candidate.error
        }))
      : metrics.candidate_verifications
  };
}

function buildFusionMetrics({
  presetName,
  preset,
  panelResults,
  context = {},
  policy,
  selectedModel = null,
  selectionReason = null,
  passingCandidateCount = 0,
  antiDegradationGuard = false,
  verifierStage = null,
  verifierLatencyMs = 0,
  finalChangedSelectedAnswer = null,
  repairUsed = false,
  extras = {}
}) {
  const taskFamily = taskFamilyForContext(context);
  const selectedPolicy = policy || defaultPolicyForTaskFamily(taskFamily);
  return {
    preset: presetName,
    mode: fusionModeForPolicy(selectedPolicy),
    task_family: taskFamily,
    policy: selectedPolicy,
    selected_model: selectedModel,
    selection_reason: selectionReason,
    candidate_count: panelResults.length,
    valid_candidate_count: panelResults.filter((r) => r.ok && r.content?.trim()).length,
    passing_candidate_count: passingCandidateCount,
    anti_degradation_guard: Boolean(antiDegradationGuard),
    repair_used: Boolean(repairUsed),
    verifier_stage: verifierStage,
    verifier_latency_ms: verifierLatencyMs,
    final_changed_selected_answer: finalChangedSelectedAnswer,
    panel: panelResults,
    all_panel_failure: false,
    preferred_judge_model: preset?.judge || null,
    preferred_final_model: preset?.final || null,
    ...extras
  };
}

function chatCompletionFromContent({ model, content, usage, fusionMetrics, latencyMs }) {
  return {
    id: nowId("chatcmpl-fusion"),
    object: "chat.completion",
    created: Math.floor(Date.now() / 1000),
    model,
    choices: [
      {
        index: 0,
        message: {
          role: "assistant",
          content
        },
        finish_reason: "stop"
      }
    ],
    usage,
    _latency_ms: latencyMs,
    fusion_metrics: fusionMetrics
  };
}

function objectiveFusionSelection(panelResults, scoring) {
  if (!scoring || !["exact", "contains", "regex"].includes(scoring.type)) return null;
  const started = Date.now();
  const candidates = panelResults.map((result, index) => {
    const normalized = normalizeText(result.content);
    if (!result.ok || !result.content?.trim()) {
      return {
        model: result.model,
        ok: false,
        normalized_answer: normalized,
        answer_group: normalized,
        score: 0,
        passed: false,
        score_reason: result.error || "candidate_failed",
        panel_index: index
      };
    }
    try {
      const scored = scoreAnswer(result.content, scoring);
      return {
        model: result.model,
        ok: true,
        normalized_answer: normalized,
        answer_group: normalized,
        score: scored.score,
        passed: Boolean(scored.passed),
        score_reason: scored.reason,
        panel_index: index
      };
    } catch (err) {
      return {
        model: result.model,
        ok: true,
        normalized_answer: normalized,
        answer_group: normalized,
        score: 0,
        passed: false,
        score_reason: err.message,
        panel_index: index
      };
    }
  });
  const passing = candidates.filter((candidate) => candidate.passed);
  if (passing.length === 0) {
    return {
      selected: null,
      candidates,
      passing_count: 0,
      verifier_latency_ms: Date.now() - started
    };
  }
  const groups = new Map();
  for (const candidate of passing) {
    const key = candidate.answer_group || candidate.normalized_answer;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(candidate);
  }
  const winningGroup = [...groups.entries()]
    .sort((a, b) => {
      if (b[1].length !== a[1].length) return b[1].length - a[1].length;
      if (a[0].length !== b[0].length) return a[0].length - b[0].length;
      return a[1][0].panel_index - b[1][0].panel_index;
    })[0];
  const winner = winningGroup[1]
    .slice()
    .sort((a, b) => {
      const contentA = String(panelResults[a.panel_index]?.content || "").trim();
      const contentB = String(panelResults[b.panel_index]?.content || "").trim();
      if (contentA.length !== contentB.length) return contentA.length - contentB.length;
      return a.panel_index - b.panel_index;
    })[0];
  const selectedPanel = panelResults[winner.panel_index];
  return {
    selected: {
      ...winner,
      content: String(selectedPanel.content || "").trim(),
      majority_size: winningGroup[1].length,
      agreed_models: winningGroup[1].map((candidate) => candidate.model)
    },
    candidates,
    passing_count: passing.length,
    verifier_latency_ms: Date.now() - started
  };
}

function contentSignal(content) {
  const text = String(content || "").trim();
  const normalized = normalizeText(text);
  return {
    text,
    normalized,
    length: text.length,
    has_code_fence: /```/.test(text),
    has_numbers: /\d/.test(text),
    has_citations: /\[[^\]]+\]|\b(source|evidence|because|therefore)\b/i.test(text)
  };
}

function heuristicCandidateRank(panelResults, context = {}, config = {}) {
  const started = Date.now();
  const successful = panelResults
    .map((result, index) => {
      const signal = contentSignal(result.content);
      return {
        model: result.model,
        ok: result.ok && Boolean(signal.text),
        panel_index: index,
        normalized_answer: signal.normalized,
        content_chars: signal.length,
        score: 0,
        reasons: [],
        agreement_count: 0
      };
    })
    .filter((candidate) => candidate.ok);
  if (successful.length === 0) {
    return {
      clear_winner: null,
      candidates: [],
      verifier_latency_ms: Date.now() - started
    };
  }
  const groups = new Map();
  for (const candidate of successful) {
    const key = candidate.normalized_answer;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(candidate);
  }
  for (const candidate of successful) {
    const groupSize = groups.get(candidate.normalized_answer)?.length || 1;
    candidate.agreement_count = groupSize;
    if (groupSize >= 2) {
      candidate.score += groupSize * 10;
      candidate.reasons.push(`normalized_agreement_${groupSize}`);
    }
    const raw = panelResults[candidate.panel_index];
    const signal = contentSignal(raw.content);
    if (signal.length > 0 && signal.length <= Number(config.fusionRanker?.maxDirectAnswerChars || 6000)) {
      candidate.score += 2;
      candidate.reasons.push("bounded_length");
    }
    if (taskFamilyForContext(context) === "long_context" && signal.has_citations) {
      candidate.score += 2;
      candidate.reasons.push("evidence_signal");
    }
    if (taskFamilyForContext(context) === "reasoning" && signal.has_numbers) {
      candidate.score += 1;
      candidate.reasons.push("numeric_signal");
    }
    if (signal.has_code_fence) {
      candidate.score -= 1;
      candidate.reasons.push("code_fence_in_non_code");
    }
    candidate.score -= Math.min(3, Math.floor(signal.length / 4000));
  }
  const ranked = successful.slice().sort((a, b) => {
    if (b.score !== a.score) return b.score - a.score;
    if (b.agreement_count !== a.agreement_count) return b.agreement_count - a.agreement_count;
    if (a.content_chars !== b.content_chars) return a.content_chars - b.content_chars;
    return a.panel_index - b.panel_index;
  });
  const top = ranked[0];
  const second = ranked[1] || null;
  const scoreMargin = top.score - (second?.score ?? -Infinity);
  const minAgreement = Number(config.fusionRanker?.minAgreement || 2);
  const minMargin = Number(config.fusionRanker?.clearWinnerMargin || 8);
  const sameAnswerAsRunnerUp =
    second && second.normalized_answer === top.normalized_answer && second.agreement_count === top.agreement_count;
  const clearWinner =
    top.agreement_count >= minAgreement && (sameAnswerAsRunnerUp || !second || scoreMargin >= minMargin)
      ? {
          ...top,
          content: String(panelResults[top.panel_index].content || "").trim(),
          agreed_models: ranked
            .filter((candidate) => candidate.normalized_answer === top.normalized_answer)
            .map((candidate) => candidate.model),
          score_margin: Number.isFinite(scoreMargin) ? scoreMargin : top.score,
          clear_winner_reason: sameAnswerAsRunnerUp ? "normalized_agreement" : Number.isFinite(scoreMargin) ? "score_margin" : "agreement_only"
        }
      : null;
  return {
    clear_winner: clearWinner,
    candidates: ranked,
    verifier_latency_ms: Date.now() - started
  };
}

async function runFusionPanelCandidates(body, preset, config, options = {}) {
  const originalMessages = body.messages || [];
  const maxTokens = body.max_tokens || body.max_completion_tokens || 4096;
  const temperature = body.temperature ?? 0.2;
  const panelModels = options.panelModels || preset.panel || [];
  const attempt = options.attempt || "primary";

  const panelPromises = panelModels.map(async (model) => {
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
        timeoutMs: timeoutMs(config, "panelMs", 30000),
        config,
        upstreamName: upstream.upstreamName,
        signal: options.signal
      });
      const costs = costFields(model, result, config);
      return {
        model,
        ok: true,
        attempt,
        content: extractContent(result),
        usage: result.usage,
        latency_ms: result._latency_ms ?? Date.now() - started,
        ...costs
      };
    } catch (err) {
      return {
        model,
        ok: false,
        attempt,
        content: "",
        error: err.message,
        latency_ms: Date.now() - started,
        cost_usd: 0,
        estimated_cost_usd: 0,
        actual_cost_usd: null
      };
    }
  });

  return Promise.all(panelPromises);
}

function fallbackPanelModels(primaryPanel, preset, config, router) {
  const configured = config.fusionPanelFallbacks || {};
  const profile = router?.profile || tierProfileForPreset("", preset);
  const fallbackLimit = Number(configured.maxFallbackPanelByTier?.[profile] ?? configured.maxFallbackPanel ?? 2);
  if (!fallbackLimit) return [];
  const candidates = dedupeModels([
    ...(configured[profile] || []),
    ...(configured.default || []),
    ...(preset.panelFallbacks || []),
    ...(preset.judgeFallbacks || []),
    ...(preset.finalFallbacks || [])
  ]);
  const primary = new Set(primaryPanel || []);
  return candidates.filter((model) => !primary.has(model)).slice(0, Math.max(0, fallbackLimit));
}

async function ensurePanelCandidates(body, preset, config, options = {}) {
  const router = options.router || routeFusionPortfolio({
    presetName: body.model,
    preset,
    config,
    context: options.fusionContext || {}
  });
  let panelResults =
    options.panelResults || (await runFusionPanelCandidates(body, preset, config, {
      ...options,
      panelModels: router.panel,
      attempt: "primary"
    }));
  if (options.signal?.aborted) {
    throw options.signal.reason instanceof Error ? options.signal.reason : new Error("Fusion request aborted");
  }
  if (panelResults.some((r) => r.ok && r.content.trim())) {
    return { panelResults, fallbackPanel: [] };
  }

  const fallbackPanel = fallbackPanelModels(router.panel, preset, config, router);
  if (fallbackPanel.length === 0) return { panelResults, fallbackPanel };
  const fallbackResults = await runFusionPanelCandidates(body, preset, config, {
    ...options,
    panelModels: fallbackPanel,
    attempt: "fallback"
  });
  panelResults = [...panelResults, ...fallbackResults];
  return { panelResults, fallbackPanel };
}

function writePanelFailureArtifacts(panelResults, artifactDir) {
  if (!artifactDir) return;
  fs.mkdirSync(artifactDir, { recursive: true });
  fs.writeFileSync(
    path.join(artifactDir, "panel-failures.json"),
    JSON.stringify(panelSummary(panelResults), null, 2)
  );
  for (const panel of panelResults || []) {
    if (panel.ok && panel.content?.trim()) continue;
    const name = `${sanitizePathPart(panel.attempt || "primary")}-${sanitizePathPart(panel.model)}`;
    const dir = path.join(artifactDir, "panel-failures", name);
    fs.mkdirSync(dir, { recursive: true });
    fs.writeFileSync(path.join(dir, "error.txt"), panel.error || "empty_content");
  }
}

async function runFusion(body, config, options = {}) {
  const preset = config.fusionPresets?.[body.model];
  if (!preset) throw new Error(`Unknown fusion preset: ${body.model}`);

  const fusionStarted = Date.now();
  const originalMessages = body.messages || [];
  const maxTokens = body.max_tokens || body.max_completion_tokens || 4096;
  const temperature = body.temperature ?? 0.2;
  const fusionContext = options.fusionContext || {};
  const router = options.router || routeFusionPortfolio({
    presetName: body.model,
    preset,
    config,
    context: fusionContext
  });

  const { panelResults, fallbackPanel } = await ensurePanelCandidates(body, preset, config, {
    ...options,
    router
  });
  const successfulPanels = panelResults.filter((r) => r.ok && r.content.trim());
  if (successfulPanels.length === 0) {
    const err = new Error("All Fusion panel calls failed");
    err.status = 502;
    err.panelResults = panelResults;
    throw err;
  }

  const objectiveSelection = objectiveFusionSelection(panelResults, fusionContext.scoring);
  if (objectiveSelection?.selected) {
    const usage = aggregateUsage(panelResults);
    const estimatedCostUsd = sumEstimatedCost(panelResults);
    const actualCostUsd = sumActualCost(panelResults);
    const selected = objectiveSelection.selected;
    const fusionMetrics = buildFusionMetrics({
      presetName: body.model,
      preset,
      panelResults,
      context: fusionContext,
      policy: "objective",
      selectedModel: selected.model,
      selectionReason:
        selected.majority_size >= 2 ? "objective_scorer_majority_passed" : "objective_scorer_single_passed",
      passingCandidateCount: objectiveSelection.passing_count,
      antiDegradationGuard: true,
      verifierStage: "objective_scorer",
      verifierLatencyMs: objectiveSelection.verifier_latency_ms,
      finalChangedSelectedAnswer: false,
      extras: {
        objective_candidates: objectiveSelection.candidates,
        normalized_answer: selected.normalized_answer,
        answer_group: selected.answer_group,
        majority_size: selected.majority_size,
        objective_scorer_passed: true,
        early_exit: {
          strategy: "objective_scorer",
          agreed_models: selected.agreed_models
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
        judge_json_valid: null,
        cost_usd: estimatedCostUsd,
        estimated_cost_usd: estimatedCostUsd,
        actual_cost_usd: actualCostUsd,
        router
      }
    });
    return chatCompletionFromContent({
      model: body.model,
      content: selected.content,
      usage,
      fusionMetrics,
      latencyMs: Date.now() - fusionStarted
    });
  }

  const earlyExit = exactMajorityEarlyExit(preset, panelResults, config);
  if (earlyExit) {
    const fusionMetrics = buildFusionMetrics({
      presetName: body.model,
      preset,
      panelResults,
      context: fusionContext,
      policy: "exact_majority",
      selectedModel: earlyExit.agreed_models[0] || null,
      selectionReason: "exact_majority",
      passingCandidateCount: earlyExit.agreed_models.length,
      antiDegradationGuard: true,
      verifierStage: "normalized_vote",
      verifierLatencyMs: 0,
      finalChangedSelectedAnswer: false,
      extras: {
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
        cost_usd: earlyExit.estimated_cost_usd,
        estimated_cost_usd: earlyExit.estimated_cost_usd,
        actual_cost_usd: earlyExit.actual_cost_usd,
        router,
        fallback_panel: fallbackPanel
      }
    });
    return chatCompletionFromContent({
      model: body.model,
      content: earlyExit.answer,
      usage: earlyExit.usage,
      fusionMetrics,
      latencyMs: Date.now() - fusionStarted
    });
  }

  const taskFamily = taskFamilyForContext(fusionContext);
  if (!["coding", "objective"].includes(taskFamily)) {
    const ranked = heuristicCandidateRank(panelResults, fusionContext, config);
    if (ranked.clear_winner) {
      const usage = aggregateUsage(panelResults);
      const estimatedCostUsd = sumEstimatedCost(panelResults);
      const actualCostUsd = sumActualCost(panelResults);
      const selected = ranked.clear_winner;
      const fusionMetrics = buildFusionMetrics({
        presetName: body.model,
        preset,
        panelResults,
        context: fusionContext,
        policy: "rank_then_select",
        selectedModel: selected.model,
        selectionReason: "ranker_clear_winner",
        passingCandidateCount: selected.agreement_count,
        antiDegradationGuard: true,
        verifierStage: "heuristic_ranker",
        verifierLatencyMs: ranked.verifier_latency_ms,
        finalChangedSelectedAnswer: false,
        extras: {
          router,
          ranker_candidates: ranked.candidates,
          early_exit: {
            strategy: "rank_then_select",
            agreed_models: selected.agreed_models
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
          judge_json_valid: null,
          cost_usd: estimatedCostUsd,
          estimated_cost_usd: estimatedCostUsd,
          actual_cost_usd: actualCostUsd,
          fallback_panel: fallbackPanel
        }
      });
      return chatCompletionFromContent({
        model: body.model,
        content: selected.content,
        usage,
        fusionMetrics,
        latencyMs: Date.now() - fusionStarted
      });
    }
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
      if (options.signal?.aborted) {
        throw options.signal.reason instanceof Error ? options.signal.reason : new Error("Fusion request aborted");
      }
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
          timeoutMs: timeoutMs(config, "judgeMs", 45000),
          config,
          upstreamName: judgeUpstream.upstreamName,
          signal: options.signal
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
    if (options.signal?.aborted) {
      throw options.signal.reason instanceof Error ? options.signal.reason : new Error("Fusion request aborted");
    }
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
        timeoutMs: timeoutMs(config, "finalMs", 60000),
        config,
        upstreamName: finalUpstream.upstreamName,
        signal: options.signal
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
  const judgeCosts = costFields(judgeModel, judgeResult, config);
  const finalCosts = costFields(finalModel, finalResult, config);
  const estimatedCostUsd = sumEstimatedCost(panelResults) + judgeCosts.estimated_cost_usd + finalCosts.estimated_cost_usd;
  const actualCostUsd = sumActualCost([...panelResults, judgeCosts, finalCosts]);

  const finalContent = extractContent(finalResult);
  const fusionMetrics = buildFusionMetrics({
    presetName: body.model,
    preset,
    panelResults,
    context: fusionContext,
    policy: "panel_judge_final",
    selectedModel: finalModel,
    selectionReason: judgeJsonValid ? "judge_guided_synthesis" : "fallback_synthesis",
    passingCandidateCount: 0,
    antiDegradationGuard: false,
    verifierStage: judgeJsonValid ? "judge" : "judge_fallback",
    verifierLatencyMs: judgeResult?._latency_ms || 0,
    finalChangedSelectedAnswer: null,
    extras: {
      router,
      fallback_panel: fallbackPanel,
      judge: {
        model: judgeModel,
        preferred_model: preset.judge,
        json_valid: judgeJsonValid,
        usage: judgeResult?.usage || null,
        latency_ms: judgeResult?._latency_ms || 0,
        errors: judgeErrors,
        estimated_cost_usd: judgeCosts.estimated_cost_usd,
        actual_cost_usd: judgeCosts.actual_cost_usd,
        actual_cost_source: judgeCosts.actual_cost_source
      },
      final: {
        model: finalModel,
        preferred_model: preset.final,
        usage: finalResult.usage || null,
        latency_ms: finalResult._latency_ms || 0,
        estimated_cost_usd: finalCosts.estimated_cost_usd,
        actual_cost_usd: finalCosts.actual_cost_usd,
        actual_cost_source: finalCosts.actual_cost_source
      },
      all_panel_failure: false,
      judge_json_valid: judgeJsonValid,
      cost_usd: estimatedCostUsd,
      estimated_cost_usd: estimatedCostUsd,
      actual_cost_usd: actualCostUsd
    }
  });

  return chatCompletionFromContent({
    model: body.model,
    content: finalContent,
    usage,
    fusionMetrics,
    latencyMs: Date.now() - fusionStarted
  });
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
          cost_usd: fusion.fusion_metrics?.estimated_cost_usd || fusion.fusion_metrics?.cost_usd || 0,
          estimated_cost_usd: fusion.fusion_metrics?.estimated_cost_usd || fusion.fusion_metrics?.cost_usd || 0,
          actual_cost_usd: fusion.fusion_metrics?.actual_cost_usd ?? null,
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
        timeoutMs: timeoutMs(config, "passthroughMs", 60000),
        config,
        upstreamName: upstream.upstreamName
      });
      const costs = costFields(body.model, passthrough, config);
      writeRequestLog({
        model: body.model,
        ok: true,
        is_fusion: false,
        latency_ms: Date.now() - started,
        usage: passthrough.usage || null,
        ...costs
      });
      return jsonResponse(res, 200, passthrough);
    } catch (err) {
      writeRequestLog({
        model: body?.model || "",
        ok: false,
        is_fusion: Boolean(config.fusionPresets?.[body?.model]),
        latency_ms: Date.now() - started,
        cost_usd: 0,
        estimated_cost_usd: 0,
        actual_cost_usd: null,
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
  const difficultyArg = valueOf(argv, "--difficulty");
  const difficulties = difficultyArg
    ? new Set(difficultyArg.split(",").map((d) => d.trim()).filter(Boolean))
    : null;
  const perCategoryLimitRaw = valueOf(argv, "--per-category-limit");
  const perCategoryLimit =
    perCategoryLimitRaw === undefined ? null : Math.max(0, Number(perCategoryLimitRaw) || 0);
  const offset = Math.max(0, Number(valueOf(argv, "--offset", "0")) || 0);
  const limitRaw = valueOf(argv, "--limit");
  const limit = limitRaw === undefined ? null : Math.max(0, Number(limitRaw) || 0);
  const filtered = questions.filter((q) => {
    if (categories && !categories.has(q.category)) return false;
    if (difficulties && !difficulties.has(q.difficulty || "unspecified")) return false;
    return true;
  });
  const balanced =
    perCategoryLimit === null
      ? filtered
      : (() => {
          const counts = new Map();
          return filtered.filter((question) => {
            const key = `${question.difficulty || "unspecified"}\t${question.category || ""}`;
            const count = counts.get(key) || 0;
            if (count >= perCategoryLimit) return false;
            counts.set(key, count + 1);
            return true;
          });
        })();
  return limit === null ? balanced.slice(offset) : balanced.slice(offset, offset + limit);
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

function validateFreshQuestions(questions, source = "dataset") {
  const errors = [];
  const warnings = [];
  const ids = new Set();
  const categories = new Map();
  const difficulties = new Map();
  const scoringTypes = new Map();

  questions.forEach((question, index) => {
    const where = `${source}:${index + 1}`;
    if (!question || typeof question !== "object") {
      errors.push(`${where}: row must be a JSON object.`);
      return;
    }
    if (!question.id || typeof question.id !== "string") {
      errors.push(`${where}: missing string id.`);
    } else if (ids.has(question.id)) {
      errors.push(`${where}: duplicate id ${question.id}.`);
    } else {
      ids.add(question.id);
    }
    if (!question.category || typeof question.category !== "string") {
      errors.push(`${where}: missing string category.`);
    } else {
      categories.set(question.category, (categories.get(question.category) || 0) + 1);
    }
    validateQuestionMetadata(question, where, warnings);
    if (question.difficulty !== undefined) {
      if (typeof question.difficulty !== "string" || !question.difficulty.trim()) {
        errors.push(`${where}: difficulty must be a non-empty string when present.`);
      } else {
        difficulties.set(question.difficulty, (difficulties.get(question.difficulty) || 0) + 1);
      }
    } else {
      difficulties.set("unspecified", (difficulties.get("unspecified") || 0) + 1);
    }
    if (!question.prompt || typeof question.prompt !== "string") {
      errors.push(`${where}: missing string prompt.`);
    } else if (question.prompt.length < 20) {
      warnings.push(`${where}: prompt is very short.`);
    }
    if (question.max_tokens !== undefined && (!Number.isFinite(Number(question.max_tokens)) || Number(question.max_tokens) <= 0)) {
      errors.push(`${where}: max_tokens must be a positive number when present.`);
    }

    const scoring = question.scoring;
    if (!scoring || typeof scoring !== "object") {
      errors.push(`${where}: missing scoring object.`);
      return;
    }
    scoringTypes.set(scoring.type, (scoringTypes.get(scoring.type) || 0) + 1);
    if (!["exact", "contains", "regex"].includes(scoring.type)) {
      errors.push(`${where}: unsupported scoring.type ${scoring.type}.`);
    }
    if ((scoring.type === "exact" || scoring.type === "contains") && typeof scoring.answer !== "string") {
      errors.push(`${where}: ${scoring.type} scoring requires string answer.`);
    }
    if (scoring.type === "regex") {
      if (typeof scoring.pattern !== "string") {
        errors.push(`${where}: regex scoring requires string pattern.`);
      } else {
        try {
          new RegExp(scoring.pattern, scoring.flags || "");
        } catch (err) {
          errors.push(`${where}: invalid regex pattern: ${err.message}`);
        }
      }
    }
  });

  return {
    ok: errors.length === 0,
    errors,
    warnings,
    count: questions.length,
    categories: [...categories.entries()].sort((a, b) => a[0].localeCompare(b[0])),
    difficulties: [...difficulties.entries()].sort((a, b) => a[0].localeCompare(b[0])),
    scoringTypes: [...scoringTypes.entries()].sort((a, b) => a[0].localeCompare(b[0]))
  };
}

const knownVisibilityValues = new Set(["public_anchor", "private_isomorphic", "business_realistic"]);
const knownFreshModuleValues = new Set(["reasoning", "chinese", "english", "long_context", "instruction_following"]);
const knownCodeModuleValues = new Set(["coding"]);

function validateQuestionMetadata(question, where, warnings, allowedModules = knownFreshModuleValues) {
  if (!question.module || typeof question.module !== "string") {
    warnings.push(`${where}: missing module; v2 reports will infer a module from category when possible.`);
  } else if (!allowedModules.has(question.module)) {
    warnings.push(`${where}: unusual module ${question.module}; expected ${[...allowedModules].join("/")}.`);
  }
  if (!question.source_family || typeof question.source_family !== "string") {
    warnings.push(`${where}: missing source_family; v2 reports will group this under unspecified.`);
  }
  if (!question.visibility || typeof question.visibility !== "string") {
    warnings.push(`${where}: missing visibility; v2 reports will group this under unspecified.`);
  } else if (!knownVisibilityValues.has(question.visibility)) {
    warnings.push(`${where}: unusual visibility ${question.visibility}; expected public_anchor/private_isomorphic/business_realistic.`);
  }
}

function freshValidate(argv) {
  const dataset = valueOf(argv, "--dataset");
  if (!dataset) throw new Error("Missing --dataset");
  const questions = parseJsonl(dataset);
  const result = validateFreshQuestions(questions, dataset);
  console.log("Fresh eval dataset validation");
  console.log(`Dataset: ${dataset}`);
  console.log(`Questions: ${result.count}`);
  console.log(`Categories: ${result.categories.map(([name, count]) => `${name}=${count}`).join(", ") || "none"}`);
  console.log(`Difficulties: ${result.difficulties.map(([name, count]) => `${name}=${count}`).join(", ") || "none"}`);
  console.log(`Scoring: ${result.scoringTypes.map(([name, count]) => `${name}=${count}`).join(", ") || "none"}`);
  console.log(`Warnings: ${result.warnings.length}`);
  for (const warning of result.warnings) console.log(`- WARN ${warning}`);
  console.log(`Errors: ${result.errors.length}`);
  for (const error of result.errors) console.log(`- ERROR ${error}`);
  if (!result.ok) process.exitCode = 1;
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
  if (config.fusionPresets?.[model]) {
    return runFusion({ ...body, model }, config, {
      upstreamApiBase: options.apiBaseOverride,
      apiKey: options.apiKeyOverride,
      signal: options.signal,
      fusionContext: options.fusionContext
    });
  }
  if (options.apiBaseOverride) {
    return callChat({
      apiBase: options.apiBaseOverride,
      apiKey: options.apiKeyOverride || "",
      body: { ...body, model },
      timeoutMs: timeoutMs(config, "passthroughMs", 60000),
      config,
      upstreamName: "default",
      signal: options.signal
    });
  }
  const upstream = resolveModelUpstream(model, config);
  return callChat({
    apiBase: upstream.apiBase,
    apiKey: upstream.apiKey,
    body: { ...body, ...modelOptions(model, config), model: upstream.model },
    timeoutMs: timeoutMs(config, "passthroughMs", 60000),
    config,
    upstreamName: upstream.upstreamName,
    signal: options.signal
  });
}

async function freshRun(argv) {
  const config = loadConfig(argv);
  const dataset = valueOf(argv, "--dataset");
  if (!dataset) throw new Error("Missing --dataset");
  const runId = nowId("fresh-run");
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
  const allQuestions = parseJsonl(dataset);
  const validation = validateFreshQuestions(allQuestions, dataset);
  if (!validation.ok) {
    throw new Error(`Fresh eval dataset failed validation with ${validation.errors.length} error(s). Run fresh-validate for details.`);
  }
  const questions = filterQuestions(allQuestions, argv);
  if (questions.length === 0) throw new Error("No questions matched --category/--offset/--limit filters");
  fs.mkdirSync(path.dirname(out), { recursive: true });
  const stream = fs.createWriteStream(out, { flags: "w" });
  const runBillingCfg = runBalanceDeltaConfig(config);
  const billingBefore = await takeRunBillingSnapshot(config, runBillingCfg);

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
          apiKeyOverride,
          fusionContext: {
            question_id: question.id,
            category: question.category,
            module: question.module || inferCapabilityModule(question),
            difficulty: question.difficulty || "unspecified",
            visibility: question.visibility || "unspecified",
            source_family: question.source_family || "unspecified",
            scoring: question.scoring,
            task_kind: "fresh_eval"
          }
        });
        const content = extractContent(response);
        const emptyContent = !content.trim();
        const scored = scoreAnswer(content, question.scoring);
        const costs = response.fusion_metrics
          ? {
              cost_usd: response.fusion_metrics.estimated_cost_usd ?? response.fusion_metrics.cost_usd ?? 0,
              estimated_cost_usd: response.fusion_metrics.estimated_cost_usd ?? response.fusion_metrics.cost_usd ?? 0,
              actual_cost_usd: response.fusion_metrics.actual_cost_usd ?? null
            }
          : costFields(model, response, config);
        record = {
          run_type: "fresh_eval",
          run_id: runId,
          question_id: question.id,
          module: question.module || inferCapabilityModule(question),
          category: question.category,
          difficulty: question.difficulty || "unspecified",
          source_family: question.source_family || "unspecified",
          visibility: question.visibility || "unspecified",
          model,
          ok: !emptyContent,
          score: emptyContent ? 0 : scored.score,
          passed: emptyContent ? false : scored.passed,
          score_reason: emptyContent ? "empty_content" : scored.reason,
          latency_ms: response._latency_ms ?? Date.now() - started,
          usage: response.usage || null,
          ...costs,
          judge_json_valid: response.fusion_metrics?.judge_json_valid,
          all_panel_failure: response.fusion_metrics?.all_panel_failure,
          fusion_metrics: compactFusionMetrics(response.fusion_metrics),
          answer: content,
          error: emptyContent ? "Empty model response" : undefined
        };
      } catch (err) {
        const failure = classifyRequestFailure(err.message);
        record = {
          run_type: "fresh_eval",
          run_id: runId,
          question_id: question.id,
          module: question.module || inferCapabilityModule(question),
          category: question.category,
          difficulty: question.difficulty || "unspecified",
          source_family: question.source_family || "unspecified",
          visibility: question.visibility || "unspecified",
          model,
          ok: false,
          score: 0,
          passed: false,
          latency_ms: Date.now() - started,
          cost_usd: 0,
          estimated_cost_usd: 0,
          actual_cost_usd: null,
          score_reason: failure.score_reason,
          failure_type: failure.failure_type,
          error: err.message
        };
      }
      stream.write(`${JSON.stringify(record)}\n`);
      console.log(`${record.ok ? "ok" : "fail"} ${model} ${question.id} score=${record.score}`);
    }
  }
  const billingAfter = await takeRunBillingSnapshot(config, runBillingCfg);
  for (const record of buildRunBillingRecords({
    runId,
    runType: "fresh_eval",
    cfg: runBillingCfg,
    before: billingBefore,
    after: billingAfter,
    config
  })) {
    stream.write(`${JSON.stringify(record)}\n`);
  }
  await new Promise((resolve) => stream.end(resolve));
  console.log(`Wrote ${out}`);
}

function sanitizePathPart(value) {
  return String(value || "unknown").replace(/[^a-zA-Z0-9._-]+/g, "_").slice(0, 120);
}

function benchmarkRoot() {
  return path.join(__dirname, "fusion-benchmark");
}

function resolveBenchmarkPath(value) {
  if (!value) return "";
  return path.isAbsolute(value) ? value : path.resolve(benchmarkRoot(), value);
}

function shellCommand() {
  return process.platform === "win32" ? "cmd.exe" : "/bin/sh";
}

function shellArgs(command) {
  return process.platform === "win32" ? ["/d", "/s", "/c", command] : ["-c", command];
}

function childProcessEnv(extraEnv = {}) {
  const env = { ...process.env, ...extraEnv };
  const pathKey = Object.keys(env).find((key) => key.toLowerCase() === "path") || "PATH";
  const nodeBin = path.dirname(process.execPath);
  const segments = String(env[pathKey] || "")
    .split(path.delimiter)
    .filter(Boolean);
  if (!segments.includes(nodeBin)) segments.unshift(nodeBin);
  env[pathKey] = segments.join(path.delimiter);
  return env;
}

function runProcess(command, args, options = {}) {
  const started = Date.now();
  const timeoutMs = Math.max(1000, Number(options.timeoutMs || 60000));
  return new Promise((resolve) => {
    let stdout = "";
    let stderr = "";
    let timedOut = false;
    const child = spawn(command, args, {
      cwd: options.cwd,
      env: childProcessEnv(options.env),
      stdio: ["ignore", "pipe", "pipe"]
    });
    const timer = setTimeout(() => {
      timedOut = true;
      child.kill("SIGKILL");
    }, timeoutMs);
    child.stdout.on("data", (chunk) => {
      stdout += chunk.toString();
    });
    child.stderr.on("data", (chunk) => {
      stderr += chunk.toString();
    });
    child.on("error", (err) => {
      clearTimeout(timer);
      resolve({
        ok: false,
        code: null,
        signal: null,
        timed_out: false,
        stdout,
        stderr: `${stderr}${stderr ? "\n" : ""}${err.message}`,
        latency_ms: Date.now() - started
      });
    });
    child.on("close", (code, signal) => {
      clearTimeout(timer);
      resolve({
        ok: code === 0 && !timedOut,
        code,
        signal,
        timed_out: timedOut,
        stdout,
        stderr,
        latency_ms: Date.now() - started
      });
    });
  });
}

function runShell(command, options = {}) {
  return runProcess(shellCommand(), shellArgs(command), options);
}

function firstFence(content, languagePattern = "[a-zA-Z0-9_-]*") {
  const re = new RegExp("```(?:" + languagePattern + ")?\\s*\\n([\\s\\S]*?)```", "i");
  const match = String(content || "").match(re);
  return match ? match[1].trim() : "";
}

function extractCode(content, language) {
  const lang = String(language || "").toLowerCase();
  const languagePattern = lang ? `${lang}|python|py|javascript|js|typescript|ts|go|java|rust|cpp|c\\+\\+` : "[a-zA-Z0-9_-]*";
  const fenced = firstFence(content, languagePattern);
  if (fenced) return fenced;
  return String(content || "")
    .replace(/^```[a-zA-Z0-9_-]*\s*/g, "")
    .replace(/```\s*$/g, "")
    .trim();
}

function extractUnifiedDiff(content) {
  const fenced = firstFence(content, "diff|patch");
  if (fenced) return fenced;
  const text = String(content || "").trim();
  const diffIndex = text.indexOf("diff --git ");
  if (diffIndex >= 0) return text.slice(diffIndex).trim();
  const fileDiffIndex = text.search(/^---\s+/m);
  if (fileDiffIndex >= 0) return text.slice(fileDiffIndex).trim();
  return text;
}

function commandForCodeTask(task) {
  const scoring = task.scoring || {};
  if (scoring.test_command) return scoring.test_command;
  if (scoring.runner === "python-pytest") return "python3 -m pytest -q";
  if (scoring.runner === "python-unittest") return "python3 -m unittest discover -s tests -p 'test_*.py'";
  throw new Error(`${task.id}: unsupported code runner; set scoring.test_command`);
}

function copyTaskFiles(task, workspace) {
  const scoring = task.scoring || {};
  const files = scoring.files || {};
  for (const [target, source] of Object.entries(files)) {
    const sourcePath = resolveBenchmarkPath(source);
    const targetPath = path.join(workspace, target);
    fs.mkdirSync(path.dirname(targetPath), { recursive: true });
    fs.copyFileSync(sourcePath, targetPath);
  }
  if (scoring.tests) {
    const testsPath = resolveBenchmarkPath(scoring.tests);
    const targetPath = path.join(workspace, "tests");
    fs.cpSync(testsPath, targetPath, { recursive: true });
  }
}

function readPromptFiles(task) {
  const files = task.visible_files || task.prompt_files || [];
  if (!Array.isArray(files) || !task.repo) return [];
  const repoRoot = resolveBenchmarkPath(task.repo);
  return files.map((file) => {
    const fullPath = path.resolve(repoRoot, file);
    if (!fullPath.startsWith(repoRoot + path.sep)) {
      throw new Error(`${task.id}: prompt file escapes repo: ${file}`);
    }
    return {
      file,
      content: fs.readFileSync(fullPath, "utf8")
    };
  });
}

function buildCodePrompt(task) {
  const scoringType = task.scoring?.type;
  const parts = [task.prompt.trim()];
  if (scoringType === "code_exec") {
    parts.push("");
    parts.push("Return only the implementation code. Prefer a single fenced code block. Do not include explanations.");
    if (task.language) parts.push(`Language: ${task.language}.`);
    if (task.entrypoint) parts.push(`Required entrypoint: ${task.entrypoint}.`);
  } else if (scoringType === "patch_exec") {
    const promptFiles = readPromptFiles(task);
    if (promptFiles.length) {
      parts.push("");
      parts.push("Repository files:");
      for (const file of promptFiles) {
        parts.push("");
        parts.push(`File: ${file.file}`);
        parts.push("```");
        parts.push(file.content);
        parts.push("```");
      }
    }
    parts.push("");
    parts.push("Return only a unified diff patch that can be applied with git apply. Do not include explanations.");
  }
  return parts.join("\n");
}

function codeCandidatePatchRisk(task, content) {
  if (task.scoring?.type !== "patch_exec") return null;
  const patch = extractUnifiedDiff(content);
  const touched = [];
  const forbidden = [];
  for (const line of patch.split(/\r?\n/)) {
    const match = line.match(/^(?:diff --git a\/(.+?) b\/(.+)|---\s+(?:a\/)?(.+)|\+\+\+\s+(?:b\/)?(.+))/);
    if (!match) continue;
    const files = [match[1], match[2], match[3], match[4]].filter((file) => file && file !== "/dev/null");
    for (const file of files) {
      touched.push(file);
      const normalized = file.replace(/\\/g, "/");
      if (
        /(^|\/)(test|tests|__tests__)\//i.test(normalized) ||
        /(^|\/)(package-lock\.json|pnpm-lock\.yaml|yarn\.lock|go\.sum|Cargo\.lock)$/i.test(normalized)
      ) {
        forbidden.push(file);
      }
    }
  }
  return {
    touched_files: [...new Set(touched)].sort(),
    forbidden_files: [...new Set(forbidden)].sort(),
    forbidden_touch: forbidden.length > 0
  };
}

function rankVerifiedCodeCandidates(candidates) {
  return candidates
    .filter((candidate) => candidate.passed && !candidate.patch_risk?.forbidden_touch)
    .slice()
    .sort((a, b) => {
      if ((b.score || 0) !== (a.score || 0)) return (b.score || 0) - (a.score || 0);
      const aTouched = a.patch_risk?.touched_files?.length ?? 0;
      const bTouched = b.patch_risk?.touched_files?.length ?? 0;
      if (aTouched !== bTouched) return aTouched - bTouched;
      if ((a.execution_latency_ms || 0) !== (b.execution_latency_ms || 0)) {
        return (a.execution_latency_ms || 0) - (b.execution_latency_ms || 0);
      }
      return (a.estimated_cost_usd || 0) - (b.estimated_cost_usd || 0);
    });
}

async function verifyCodeCandidate({ task, model, virtualModel, artifactRoot, candidate, attempt }) {
  const candidateWorkspace = fs.mkdtempSync(
    path.join(os.tmpdir(), `fusion-code-${sanitizePathPart(task.id)}-${sanitizePathPart(candidate.model)}-`)
  );
  const candidateArtifactDir = path.join(
    artifactRoot,
    sanitizePathPart(virtualModel),
    sanitizePathPart(task.id),
    "candidates",
    `${sanitizePathPart(attempt || candidate.attempt || "primary")}-${sanitizePathPart(candidate.model)}`
  );
  let verification;
  if (!candidate.ok || !candidate.content.trim()) {
    fs.mkdirSync(candidateArtifactDir, { recursive: true });
    fs.writeFileSync(path.join(candidateArtifactDir, "response.txt"), candidate.content || "");
    verification = {
      ok: false,
      passed: false,
      score: 0,
      score_reason: candidate.error || "empty_content",
      failure_type: candidate.error ? "provider_error" : "empty_content",
      execution_log: null,
      execution_latency_ms: 0
    };
  } else {
    verification = await executeCodeTask({
      task,
      content: candidate.content,
      workspace: candidateWorkspace,
      artifactDir: candidateArtifactDir
    });
  }
  return {
    model: candidate.model,
    attempt: attempt || candidate.attempt || "primary",
    content: candidate.content,
    content_chars: String(candidate.content || "").length,
    ok: verification.ok,
    passed: verification.passed,
    score: verification.score,
    score_reason: verification.score_reason,
    failure_type: verification.failure_type,
    execution_log: verification.execution_log,
    execution_latency_ms: verification.execution_latency_ms,
    workspace: candidateWorkspace,
    estimated_cost_usd: candidate.estimated_cost_usd,
    actual_cost_usd: candidate.actual_cost_usd,
    patch_risk: codeCandidatePatchRisk(task, candidate.content),
    error: candidate.error
  };
}

async function runVerifiedCodingFusion({
  model,
  body,
  config,
  task,
  prompt,
  artifactRoot,
  apiBaseOverride,
  apiKeyOverride,
  signal
}) {
  const preset = config.fusionPresets?.[model];
  if (!preset) return null;
  const started = Date.now();
  const fusionContext = {
    question_id: task.id,
    task_id: task.id,
    category: task.category,
    module: task.module || "coding",
    difficulty: task.difficulty || "unspecified",
    visibility: task.visibility || "unspecified",
    source_family: task.source_family || "unspecified",
    scoring: task.scoring,
    task_kind: "code_eval"
  };
  const router = routeFusionPortfolio({
    presetName: model,
    preset,
    config,
    context: fusionContext
  });
  const fusionBody = { ...body, model };
  let { panelResults, fallbackPanel } = await ensurePanelCandidates(fusionBody, preset, config, {
    upstreamApiBase: apiBaseOverride,
    apiKey: apiKeyOverride,
    router,
    fusionContext,
    signal
  });
  const successfulPanels = panelResults.filter((r) => r.ok && r.content.trim());
  if (successfulPanels.length === 0) {
    writePanelFailureArtifacts(panelResults, path.join(artifactRoot, sanitizePathPart(model), sanitizePathPart(task.id)));
    const err = new Error("All Fusion panel calls failed");
    err.status = 502;
    err.panelResults = panelResults;
    throw err;
  }

  const verifierStarted = Date.now();
  const candidateVerifications = [];
  for (const candidate of panelResults) {
    candidateVerifications.push(await verifyCodeCandidate({
      task,
      virtualModel: model,
      artifactRoot,
      candidate
    }));
  }
  let passing = rankVerifiedCodeCandidates(candidateVerifications);
  if (passing.length === 0) {
    const verifierFallbackPanel = fallbackPanelModels(panelResults.map((candidate) => candidate.model), preset, config, router);
    if (verifierFallbackPanel.length > 0) {
      const verifierFallbackResults = await runFusionPanelCandidates(fusionBody, preset, config, {
        upstreamApiBase: apiBaseOverride,
        apiKey: apiKeyOverride,
        panelModels: verifierFallbackPanel,
        attempt: "verifier_fallback",
        signal
      });
      fallbackPanel = dedupeModels([...fallbackPanel, ...verifierFallbackPanel]);
      panelResults = [...panelResults, ...verifierFallbackResults];
      for (const candidate of verifierFallbackResults) {
        candidateVerifications.push(await verifyCodeCandidate({
          task,
          virtualModel: model,
          artifactRoot,
          candidate,
          attempt: "verifier_fallback"
        }));
      }
      passing = rankVerifiedCodeCandidates(candidateVerifications);
    }
  }
  const verifierLatencyMs = Date.now() - verifierStarted;
  if (passing.length > 0) {
    const selected = passing[0];
    const selectedPanel = panelResults.find((candidate) => candidate.model === selected.model);
    const usage = aggregateUsage(panelResults);
    const estimatedCostUsd = sumEstimatedCost(panelResults);
    const actualCostUsd = sumActualCost(panelResults);
    const fusionMetrics = buildFusionMetrics({
      presetName: model,
      preset,
      panelResults,
      context: fusionContext,
      policy: "verified_coding_select",
      selectedModel: selected.model,
      selectionReason: "sandbox_tests_passed",
      passingCandidateCount: passing.length,
      antiDegradationGuard: true,
      verifierStage: task.scoring?.type || "code_verifier",
      verifierLatencyMs,
      finalChangedSelectedAnswer: false,
      extras: {
        router,
        candidate_verifications: candidateVerifications,
        fallback_panel: fallbackPanel,
        early_exit: {
          strategy: "verified_coding",
          agreed_models: passing.map((candidate) => candidate.model)
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
        judge_json_valid: null,
        cost_usd: estimatedCostUsd,
        estimated_cost_usd: estimatedCostUsd,
        actual_cost_usd: actualCostUsd
      }
    });
    return chatCompletionFromContent({
      model,
      content: selectedPanel.content,
      usage,
      fusionMetrics,
      latencyMs: Date.now() - started
    });
  }

  const fallback = await runFusion(fusionBody, config, {
    panelResults,
    router,
    fusionContext,
    upstreamApiBase: apiBaseOverride,
    apiKey: apiKeyOverride,
    signal
  });
  fallback._latency_ms = Date.now() - started;
  fallback.fusion_metrics = {
    ...fallback.fusion_metrics,
    mode: "verified_fusion_mode",
    policy: "verified_coding_fallback_synthesis",
    selection_reason: fallback.fusion_metrics?.selection_reason || "no_candidate_passed_fallback_synthesis",
    passing_candidate_count: 0,
    anti_degradation_guard: false,
    verifier_stage: task.scoring?.type || "code_verifier",
    verifier_latency_ms: verifierLatencyMs,
    repair_used: false,
    candidate_verifications: candidateVerifications,
    router,
    fallback_panel: fallbackPanel
  };
  return fallback;
}

function validateCodeQuestions(questions, source = "dataset") {
  const errors = [];
  const warnings = [];
  const ids = new Set();
  const categories = new Map();
  const difficulties = new Map();
  const scoringTypes = new Map();

  questions.forEach((question, index) => {
    const where = `${source}:${index + 1}`;
    if (!question || typeof question !== "object") {
      errors.push(`${where}: row must be a JSON object.`);
      return;
    }
    if (!question.id || typeof question.id !== "string") {
      errors.push(`${where}: missing string id.`);
    } else if (ids.has(question.id)) {
      errors.push(`${where}: duplicate id ${question.id}.`);
    } else {
      ids.add(question.id);
    }
    if (!question.category || typeof question.category !== "string") {
      errors.push(`${where}: missing string category.`);
    } else {
      categories.set(question.category, (categories.get(question.category) || 0) + 1);
    }
    validateQuestionMetadata(question, where, warnings, knownCodeModuleValues);
    const difficulty = question.difficulty || "unspecified";
    difficulties.set(difficulty, (difficulties.get(difficulty) || 0) + 1);
    if (!question.prompt || typeof question.prompt !== "string") {
      errors.push(`${where}: missing string prompt.`);
    }
    const scoring = question.scoring;
    if (!scoring || typeof scoring !== "object") {
      errors.push(`${where}: missing scoring object.`);
      return;
    }
    scoringTypes.set(scoring.type, (scoringTypes.get(scoring.type) || 0) + 1);
    if (!["code_exec", "patch_exec"].includes(scoring.type)) {
      errors.push(`${where}: unsupported scoring.type ${scoring.type}.`);
      return;
    }
    if (scoring.type === "code_exec") {
      if (!question.language) errors.push(`${where}: code_exec requires language.`);
      if (!scoring.runner && !scoring.test_command) {
        errors.push(`${where}: code_exec requires scoring.runner or scoring.test_command.`);
      }
      if (scoring.tests && !fs.existsSync(resolveBenchmarkPath(scoring.tests))) {
        errors.push(`${where}: tests path not found: ${scoring.tests}.`);
      }
      if (scoring.files) {
        for (const [target, sourceFile] of Object.entries(scoring.files)) {
          if (path.isAbsolute(target) || target.includes("..")) {
            errors.push(`${where}: scoring.files target must be relative and safe: ${target}.`);
          }
          if (!fs.existsSync(resolveBenchmarkPath(sourceFile))) {
            errors.push(`${where}: scoring.files source not found: ${sourceFile}.`);
          }
        }
      }
    }
    if (scoring.type === "patch_exec") {
      if (!question.repo) errors.push(`${where}: patch_exec requires repo.`);
      if (question.repo && !fs.existsSync(resolveBenchmarkPath(question.repo))) {
        errors.push(`${where}: repo path not found: ${question.repo}.`);
      }
      if (!scoring.test_command) errors.push(`${where}: patch_exec requires scoring.test_command.`);
      for (const file of question.visible_files || question.prompt_files || []) {
        const fullPath = question.repo ? path.join(resolveBenchmarkPath(question.repo), file) : "";
        if (!fullPath || !fs.existsSync(fullPath)) errors.push(`${where}: prompt file not found: ${file}.`);
      }
    }
    if (question.max_tokens !== undefined && (!Number.isFinite(Number(question.max_tokens)) || Number(question.max_tokens) <= 0)) {
      errors.push(`${where}: max_tokens must be a positive number when present.`);
    }
    if (question.timeout_ms !== undefined && (!Number.isFinite(Number(question.timeout_ms)) || Number(question.timeout_ms) <= 0)) {
      errors.push(`${where}: timeout_ms must be a positive number when present.`);
    }
    if ((question.difficulty || "").toLowerCase() === "simple") {
      warnings.push(`${where}: coding benchmark task is marked simple; reserve code-run for medium+ tasks where possible.`);
    }
  });

  return {
    ok: errors.length === 0,
    errors,
    warnings,
    count: questions.length,
    categories: [...categories.entries()].sort((a, b) => a[0].localeCompare(b[0])),
    difficulties: [...difficulties.entries()].sort((a, b) => a[0].localeCompare(b[0])),
    scoringTypes: [...scoringTypes.entries()].sort((a, b) => a[0].localeCompare(b[0]))
  };
}

function selfTest() {
  const config = readJson(defaultConfigPath);
  const objectiveRouter = routeFusionPortfolio({
    presetName: "fusion-cn-budget",
    preset: config.fusionPresets["fusion-cn-budget"],
    config,
    context: { scoring: { type: "exact" }, module: "english" }
  });
  const codingRouter = routeFusionPortfolio({
    presetName: "fusion-cn-budget",
    preset: config.fusionPresets["fusion-cn-budget"],
    config,
    context: { scoring: { type: "patch_exec" }, module: "coding" }
  });
  const objectiveRank = objectiveFusionSelection(
    [
      { model: "a", ok: true, content: "cache-warmer" },
      { model: "b", ok: true, content: "cache-warmer-v3" },
      { model: "c", ok: true, content: "cache-warmer" }
    ],
    { type: "exact", answer: "cache-warmer" }
  );
  const rank = heuristicCandidateRank(
    [
      { model: "a", ok: true, content: "short answer" },
      { model: "b", ok: true, content: "short answer" },
      { model: "c", ok: true, content: "longer but weaker" }
    ],
    { module: "reasoning" },
    config
  );
  const fallbackCandidates = fallbackPanelModels(
    ["qwen/qwen3.7-plus", "deepseek/deepseek-v4-pro", "moonshotai/kimi-k2.6"],
    config.fusionPresets["fusion-cn-budget"],
    config,
    { profile: "budget", task_family: "coding" }
  );
  const checks = [
    objectiveRouter.panel.length >= 2,
    codingRouter.panel.length >= 2,
    objectiveRank?.selected?.content === "cache-warmer",
    rank?.clear_winner?.content === "short answer",
    fallbackCandidates.length === 1 && fallbackCandidates[0] === "minimax/minimax-m3"
  ];
  if (checks.every(Boolean)) {
    console.log("Fusion benchmark self-test passed");
    return;
  }
  throw new Error("Fusion benchmark self-test failed");
}

function codeValidate(argv) {
  const dataset = valueOf(argv, "--dataset");
  if (!dataset) throw new Error("Missing --dataset");
  const questions = parseJsonl(dataset);
  const result = validateCodeQuestions(questions, dataset);
  console.log("Code eval dataset validation");
  console.log(`Dataset: ${dataset}`);
  console.log(`Tasks: ${result.count}`);
  console.log(`Categories: ${result.categories.map(([name, count]) => `${name}=${count}`).join(", ") || "none"}`);
  console.log(`Difficulties: ${result.difficulties.map(([name, count]) => `${name}=${count}`).join(", ") || "none"}`);
  console.log(`Scoring: ${result.scoringTypes.map(([name, count]) => `${name}=${count}`).join(", ") || "none"}`);
  console.log(`Warnings: ${result.warnings.length}`);
  for (const warning of result.warnings) console.log(`- WARN ${warning}`);
  console.log(`Errors: ${result.errors.length}`);
  for (const error of result.errors) console.log(`- ERROR ${error}`);
  if (!result.ok) process.exitCode = 1;
}

function timeoutConfig(config, timeoutMs) {
  if (!timeoutMs) return config;
  return {
    ...config,
    timeouts: {
      ...(config.timeouts || {}),
      passthroughMs: timeoutMs,
      panelMs: timeoutMs,
      judgeMs: timeoutMs,
      finalMs: timeoutMs
    }
  };
}

async function withOverallTimeout(run, timeoutMs, label = "Operation") {
  const ms = Number(timeoutMs);
  if (!Number.isFinite(ms) || ms <= 0) return run();
  const controller = new AbortController();
  let timer;
  try {
    timer = setTimeout(() => controller.abort(new Error(`${label} timed out after ${ms}ms`)), ms);
    return await run(controller.signal);
  } finally {
    clearTimeout(timer);
  }
}

function classifyRequestFailure(message) {
  const text = String(message || "").toLowerCase();
  if (/timed out|timeout|aborted/.test(text)) {
    return { score_reason: "request_timeout", failure_type: "timeout" };
  }
  if (
    /all fusion panel calls failed|fetch failed|network|socket|connection|connect|econnreset|etimedout|enotfound|eai_again|rate limit|429|502|503|504|upstream|provider|distributor|no available channel|无可用渠道|渠道/.test(
      text
    )
  ) {
    return { score_reason: "provider_error", failure_type: "provider_error" };
  }
  return { score_reason: "runner_error", failure_type: "runner_error" };
}

function isProviderErrorRecord(record) {
  return (
    record?.failure_type === "provider_error" ||
    record?.score_reason === "provider_error" ||
    classifyRequestFailure(record?.error).failure_type === "provider_error"
  );
}

function isRequestTimeoutRecord(record) {
  return (
    record?.score_reason === "request_timeout" ||
    (!record?.ok && record?.failure_type === "timeout" && record?.score_reason !== "test_timeout") ||
    (!record?.ok && classifyRequestFailure(record?.error).failure_type === "timeout")
  );
}

function isTimeoutRecord(record) {
  return (
    record?.failure_type === "timeout" ||
    record?.score_reason === "request_timeout" ||
    record?.score_reason === "test_timeout" ||
    classifyRequestFailure(record?.error).failure_type === "timeout"
  );
}

function isInfrastructureErrorRecord(record) {
  return isProviderErrorRecord(record) || isRequestTimeoutRecord(record);
}

async function executeCodeTask({ task, content, workspace, artifactDir }) {
  fs.mkdirSync(artifactDir, { recursive: true });
  fs.writeFileSync(path.join(artifactDir, "response.txt"), content);
  const scoring = task.scoring || {};

  if (scoring.type === "code_exec") {
    copyTaskFiles(task, workspace);
    const solutionPath = scoring.solution_path || (String(task.language).toLowerCase() === "python" ? "solution.py" : "solution.txt");
    const code = extractCode(content, task.language);
    fs.writeFileSync(path.join(workspace, solutionPath), `${code}\n`);
    fs.writeFileSync(path.join(artifactDir, solutionPath.replace(/[\\/]/g, "_")), `${code}\n`);
    const command = commandForCodeTask(task);
    const result = await runShell(command, {
      cwd: workspace,
      timeoutMs: task.timeout_ms || scoring.timeout_ms || 30000
    });
    const executionLog = path.join(artifactDir, "execution.log");
    fs.writeFileSync(
      executionLog,
      [
        `$ ${command}`,
        `cwd=${workspace}`,
        `exit_code=${result.code}`,
        `signal=${result.signal || ""}`,
        `timed_out=${result.timed_out}`,
        "",
        "STDOUT:",
        result.stdout,
        "",
        "STDERR:",
        result.stderr
      ].join("\n")
    );
    return {
      ok: true,
      passed: result.ok,
      score: result.ok ? 1 : 0,
      score_reason: result.ok ? "tests_passed" : result.timed_out ? "test_timeout" : `test_exit_${result.code}`,
      failure_type: result.ok ? null : result.timed_out ? "timeout" : "test_failure",
      execution_log: executionLog,
      execution_latency_ms: result.latency_ms
    };
  }

  if (scoring.type === "patch_exec") {
    const repoRoot = resolveBenchmarkPath(task.repo);
    fs.cpSync(repoRoot, workspace, { recursive: true });
    await runProcess("git", ["init", "-q"], { cwd: workspace, timeoutMs: 10000 });
    await runProcess("git", ["add", "."], { cwd: workspace, timeoutMs: 10000 });
    const patch = extractUnifiedDiff(content);
    const patchPath = path.join(artifactDir, "model.patch");
    fs.writeFileSync(patchPath, `${patch}\n`);
    const applyResult = await runProcess("git", ["apply", "--whitespace=nowarn", "--recount", patchPath], {
      cwd: workspace,
      timeoutMs: 15000
    });
    if (!applyResult.ok) {
      const executionLog = path.join(artifactDir, "execution.log");
      fs.writeFileSync(
        executionLog,
        [
          `$ git apply --whitespace=nowarn --recount ${patchPath}`,
          `cwd=${workspace}`,
          `exit_code=${applyResult.code}`,
          `signal=${applyResult.signal || ""}`,
          `timed_out=${applyResult.timed_out}`,
          "",
          "STDOUT:",
          applyResult.stdout,
          "",
          "STDERR:",
          applyResult.stderr
        ].join("\n")
      );
      return {
        ok: true,
        passed: false,
        score: 0,
        score_reason: applyResult.timed_out ? "patch_apply_timeout" : "patch_apply_failed",
        failure_type: applyResult.timed_out ? "timeout" : "patch_apply_failure",
        execution_log: executionLog,
        execution_latency_ms: applyResult.latency_ms
      };
    }
    const result = await runShell(scoring.test_command, {
      cwd: workspace,
      timeoutMs: task.timeout_ms || scoring.timeout_ms || 60000
    });
    const diffResult = await runProcess("git", ["diff", "--", "."], {
      cwd: workspace,
      timeoutMs: 10000
    });
    const executionLog = path.join(artifactDir, "execution.log");
    fs.writeFileSync(
      executionLog,
      [
        `$ git apply --whitespace=nowarn --recount ${patchPath}`,
        `apply_exit_code=${applyResult.code}`,
        "",
        `$ ${scoring.test_command}`,
        `cwd=${workspace}`,
        `exit_code=${result.code}`,
        `signal=${result.signal || ""}`,
        `timed_out=${result.timed_out}`,
        "",
        "STDOUT:",
        result.stdout,
        "",
        "STDERR:",
        result.stderr,
        "",
        "PATCHED DIFF:",
        diffResult.stdout || diffResult.stderr
      ].join("\n")
    );
    return {
      ok: true,
      passed: result.ok,
      score: result.ok ? 1 : 0,
      score_reason: result.ok ? "tests_passed" : result.timed_out ? "test_timeout" : `test_exit_${result.code}`,
      failure_type: result.ok ? null : result.timed_out ? "timeout" : "test_failure",
      execution_log: executionLog,
      execution_latency_ms: applyResult.latency_ms + result.latency_ms
    };
  }

  throw new Error(`${task.id}: unsupported scoring.type ${scoring.type}`);
}

async function codeRun(argv) {
  const config = loadConfig(argv);
  const dataset = valueOf(argv, "--dataset");
  if (!dataset) throw new Error("Missing --dataset");
  const runId = nowId("code-run");
  const out = valueOf(
    argv,
    "--out",
    path.join(__dirname, "fusion-benchmark", "runs", `code-${Date.now()}.jsonl`)
  );
  const outPath = path.resolve(out);
  const apiBaseOverride = valueOf(argv, "--api-base");
  const apiKeyOverride = apiBaseOverride ? authKey(config) : undefined;
  const requestTimeoutArg = valueOf(argv, "--request-timeout-ms");
  const requestTimeoutMs = requestTimeoutArg === undefined ? null : Math.max(1000, Number(requestTimeoutArg) || 0);
  const overallRequestTimeoutArg = valueOf(argv, "--overall-request-timeout-ms");
  const overallRequestTimeoutMs =
    overallRequestTimeoutArg === undefined ? null : Math.max(1000, Number(overallRequestTimeoutArg) || 0);
  const fusionOverallRequestTimeoutArg = valueOf(argv, "--fusion-overall-request-timeout-ms");
  const fusionOverallRequestTimeoutMs =
    fusionOverallRequestTimeoutArg === undefined ? null : Math.max(1000, Number(fusionOverallRequestTimeoutArg) || 0);
  const modelArg = valueOf(argv, "--models");
  const models = modelArg
    ? modelArg.split(",").map((m) => m.trim()).filter(Boolean)
    : [...config.baselines, ...Object.keys(config.fusionPresets)];
  const allTasks = parseJsonl(dataset);
  const validation = validateCodeQuestions(allTasks, dataset);
  if (!validation.ok) {
    throw new Error(`Code eval dataset failed validation with ${validation.errors.length} error(s). Run code-validate for details.`);
  }
  const tasks = filterQuestions(allTasks, argv);
  if (tasks.length === 0) throw new Error("No tasks matched --category/--offset/--limit filters");
  fs.mkdirSync(path.dirname(outPath), { recursive: true });
  const artifactRoot = path.join(path.dirname(outPath), `${path.basename(outPath, ".jsonl")}-artifacts`);
  const stream = fs.createWriteStream(outPath, { flags: "w" });
  const runBillingCfg = runBalanceDeltaConfig(config);
  const billingBefore = await takeRunBillingSnapshot(config, runBillingCfg);
  let warnedFusionOverallTimeout = false;

  for (const task of tasks) {
    for (const model of models) {
      const started = Date.now();
      const workspace = fs.mkdtempSync(path.join(os.tmpdir(), `fusion-code-${sanitizePathPart(task.id)}-`));
      const artifactDir = path.join(artifactRoot, sanitizePathPart(model), sanitizePathPart(task.id));
      let record;
      try {
        const prompt = buildCodePrompt(task);
        const effectiveRequestTimeoutMs = task.request_timeout_ms || requestTimeoutMs;
        const isFusionModel = Boolean(config.fusionPresets?.[model]);
        let effectiveOverallRequestTimeoutMs = task.overall_request_timeout_ms || overallRequestTimeoutMs;
        if (isFusionModel) {
          effectiveOverallRequestTimeoutMs = task.fusion_overall_request_timeout_ms || fusionOverallRequestTimeoutMs;
          if (!effectiveOverallRequestTimeoutMs && (task.overall_request_timeout_ms || overallRequestTimeoutMs) && !warnedFusionOverallTimeout) {
            console.warn(
              "Ignoring --overall-request-timeout-ms for Fusion code-run records. Use --fusion-overall-request-timeout-ms only for deliberate diagnostic truncation."
            );
            warnedFusionOverallTimeout = true;
          }
        }
        const requestConfig = timeoutConfig(config, effectiveRequestTimeoutMs);
        fs.mkdirSync(artifactDir, { recursive: true });
        fs.writeFileSync(path.join(artifactDir, "prompt.md"), prompt);
        const response = await withOverallTimeout(
          (signal) => {
            const requestBody = {
              messages: [{ role: "user", content: prompt }],
              temperature: task.temperature ?? 0,
              max_tokens: task.max_tokens || 4096
            };
            if (isFusionModel) {
              return runVerifiedCodingFusion({
                model,
                body: requestBody,
                config: requestConfig,
                task,
                prompt,
                artifactRoot,
                apiBaseOverride,
                apiKeyOverride,
                signal
              });
            }
            return callBenchmarkModel(model, requestBody, requestConfig, {
              apiBaseOverride,
              apiKeyOverride,
              signal
            });
          },
          effectiveOverallRequestTimeoutMs,
          "Model request"
        );
        const content = extractContent(response);
        const emptyContent = !content.trim();
        const costs = response.fusion_metrics
          ? {
              cost_usd: response.fusion_metrics.estimated_cost_usd ?? response.fusion_metrics.cost_usd ?? 0,
              estimated_cost_usd: response.fusion_metrics.estimated_cost_usd ?? response.fusion_metrics.cost_usd ?? 0,
              actual_cost_usd: response.fusion_metrics.actual_cost_usd ?? null
            }
          : costFields(model, response, config);
        const execution = emptyContent
          ? {
              ok: false,
              passed: false,
              score: 0,
              score_reason: "empty_content",
              failure_type: "empty_content",
              execution_log: null,
              execution_latency_ms: 0
            }
          : await executeCodeTask({ task, content, workspace, artifactDir });
        record = {
          run_type: "code_eval",
          run_id: runId,
          question_id: task.id,
          task_id: task.id,
          module: task.module || "coding",
          category: task.category,
          difficulty: task.difficulty || "unspecified",
          source_family: task.source_family || "unspecified",
          visibility: task.visibility || "unspecified",
          scoring_type: task.scoring?.type,
          model,
          ok: !emptyContent && execution.ok,
          score: execution.score,
          passed: execution.passed,
          score_reason: execution.score_reason,
          failure_type: execution.failure_type,
          latency_ms: response._latency_ms ?? Date.now() - started,
          execution_latency_ms: execution.execution_latency_ms,
          usage: response.usage || null,
          ...costs,
          judge_json_valid: response.fusion_metrics?.judge_json_valid,
          all_panel_failure: response.fusion_metrics?.all_panel_failure,
          fusion_metrics: compactFusionMetrics(response.fusion_metrics),
          execution_log: execution.execution_log,
          workspace,
          answer: content,
          error: emptyContent ? "Empty model response" : undefined
        };
      } catch (err) {
        const errorMessage = String(err.message || "");
        const failure = classifyRequestFailure(errorMessage);
        fs.mkdirSync(artifactDir, { recursive: true });
        fs.writeFileSync(path.join(artifactDir, "error.txt"), `${err.stack || err.message}\n`);
        record = {
          run_type: "code_eval",
          run_id: runId,
          question_id: task.id,
          task_id: task.id,
          module: task.module || "coding",
          category: task.category,
          difficulty: task.difficulty || "unspecified",
          source_family: task.source_family || "unspecified",
          visibility: task.visibility || "unspecified",
          scoring_type: task.scoring?.type,
          model,
          ok: false,
          score: 0,
          passed: false,
          score_reason: failure.score_reason,
          failure_type: failure.failure_type,
          latency_ms: Date.now() - started,
          execution_latency_ms: 0,
          cost_usd: 0,
          estimated_cost_usd: 0,
          actual_cost_usd: null,
          all_panel_failure: Boolean(err.panelResults),
          execution_log: path.join(artifactDir, "error.txt"),
          workspace,
          error: errorMessage
        };
      }
      stream.write(`${JSON.stringify(record)}\n`);
      console.log(`${record.passed ? "pass" : "fail"} ${model} ${task.id} score=${record.score} reason=${record.score_reason}`);
    }
  }
  const billingAfter = await takeRunBillingSnapshot(config, runBillingCfg);
  for (const record of buildRunBillingRecords({
    runId,
    runType: "code_eval",
    cfg: runBillingCfg,
    before: billingBefore,
    after: billingAfter,
    config
  })) {
    stream.write(`${JSON.stringify(record)}\n`);
  }
  await new Promise((resolve) => stream.end(resolve));
  console.log(`Wrote ${outPath}`);
  console.log(`Artifacts: ${artifactRoot}`);
}

async function pairwiseRun(argv) {
  const config = loadConfig(argv);
  const dataset = valueOf(argv, "--dataset");
  if (!dataset) throw new Error("Missing --dataset");
  const runId = nowId("pairwise-run");
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
  const runBillingCfg = runBalanceDeltaConfig(config);
  const billingBefore = await takeRunBillingSnapshot(config, runBillingCfg);

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

      const targetCosts = targetResponse.fusion_metrics
        ? {
            estimated_cost_usd: targetResponse.fusion_metrics.estimated_cost_usd ?? targetResponse.fusion_metrics.cost_usd ?? 0,
            actual_cost_usd: targetResponse.fusion_metrics.actual_cost_usd ?? null
          }
        : costFields(target, targetResponse, config);
      const baselineCosts = baselineResponse.fusion_metrics
        ? {
            estimated_cost_usd: baselineResponse.fusion_metrics.estimated_cost_usd ?? baselineResponse.fusion_metrics.cost_usd ?? 0,
            actual_cost_usd: baselineResponse.fusion_metrics.actual_cost_usd ?? null
          }
        : costFields(baseline, baselineResponse, config);
      const judgeCosts = costFields(judge, judgeResponse, config);
      const estimatedCostUsd =
        targetCosts.estimated_cost_usd + baselineCosts.estimated_cost_usd + judgeCosts.estimated_cost_usd;
      const actualCostUsd = sumActualCost([targetCosts, baselineCosts, judgeCosts]);

      record = {
        run_type: "pairwise_eval",
        run_id: runId,
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
        cost_usd: estimatedCostUsd,
        estimated_cost_usd: estimatedCostUsd,
        actual_cost_usd: actualCostUsd,
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
        run_id: runId,
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
        estimated_cost_usd: 0,
        actual_cost_usd: null,
        error: err.message
      };
    }
    stream.write(`${JSON.stringify(record)}\n`);
    console.log(`${record.ok ? "ok" : "fail"} ${target} vs ${baseline} ${question.id} result=${record.target_result}`);
  }
  const billingAfter = await takeRunBillingSnapshot(config, runBillingCfg);
  for (const record of buildRunBillingRecords({
    runId,
    runType: "pairwise_eval",
    cfg: runBillingCfg,
    before: billingBefore,
    after: billingAfter,
    config
  })) {
    stream.write(`${JSON.stringify(record)}\n`);
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
    const rawScored = items.filter((r) => typeof r.score === "number");
    const scored = rawScored.filter((r) => !isInfrastructureErrorRecord(r));
    const solved = scored.reduce((sum, r) => sum + r.score, 0);
    const rawSolved = rawScored.reduce((sum, r) => sum + r.score, 0);
    const totalEstimatedCost = items.reduce((sum, r) => sum + (r.estimated_cost_usd ?? r.cost_usd ?? 0), 0);
    const actualCostItems = items.filter((r) => numberOrNull(r.actual_cost_usd) !== null);
    const totalActualCost = actualCostItems.reduce((sum, r) => sum + r.actual_cost_usd, 0);
    const failures = items.filter((r) => !r.ok).length;
    const providerErrors = items.filter(isProviderErrorRecord).length;
    const timeouts = items.filter(isTimeoutRecord).length;
    const scoreCi = binaryScoreInterval(scored);
    const rawScoreCi = binaryScoreInterval(rawScored);
    const binaryScored = scored.filter((r) => r.score === 0 || r.score === 1);
    const severeErrors = binaryScored.filter((r) => r.score === 0).length;
    rows.push({
      model,
      n: items.length,
      valid_n: scored.length,
      valid_coverage: items.length ? scored.length / items.length : 0,
      score: scored.length ? solved / scored.length : 0,
      score_ci_low: scoreCi.low,
      score_ci_high: scoreCi.high,
      raw_score: rawScored.length ? rawSolved / rawScored.length : 0,
      raw_score_ci_low: rawScoreCi.low,
      raw_score_ci_high: rawScoreCi.high,
      solved,
      raw_solved: rawSolved,
      total_cost_usd: totalEstimatedCost,
      avg_cost_usd: items.length ? totalEstimatedCost / items.length : 0,
      cost_per_solved_usd: solved > 0 ? totalEstimatedCost / solved : 0,
      total_estimated_cost_usd: totalEstimatedCost,
      avg_estimated_cost_usd: items.length ? totalEstimatedCost / items.length : 0,
      cost_per_solved_estimated_usd: solved > 0 ? totalEstimatedCost / solved : 0,
      total_actual_cost_usd: actualCostItems.length ? totalActualCost : null,
      avg_actual_cost_usd: actualCostItems.length ? totalActualCost / actualCostItems.length : null,
      cost_per_solved_actual_usd: solved > 0 && actualCostItems.length ? totalActualCost / solved : null,
      actual_cost_coverage: items.length ? actualCostItems.length / items.length : 0,
      p50_latency_ms: percentile(items.map((r) => r.latency_ms), 50),
      p95_latency_ms: percentile(items.map((r) => r.latency_ms), 95),
      early_exit_rate: earlyExitRate(items),
      p50_panel_max_latency_ms: fusionStageLatency(items, "panel", 50),
      p50_judge_latency_ms: fusionStageLatency(items, "judge", 50),
      p50_final_latency_ms: fusionStageLatency(items, "final", 50),
      failure_rate: items.length ? failures / items.length : 0,
      timeout_rate: items.length ? timeouts / items.length : 0,
      provider_error_rate: items.length ? providerErrors / items.length : 0,
      timeout_count: timeouts,
      provider_error_count: providerErrors,
      severe_error_rate: binaryScored.length ? severeErrors / binaryScored.length : null,
      severe_error_count: severeErrors,
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

const capabilityModuleWeights = {
  coding: 0.4,
  reasoning: 0.2,
  english: 0.125,
  chinese: 0.125,
  long_context: 0.1,
  instruction_following: 0.05
};

function inferCapabilityModule(record) {
  if (record.module && capabilityModuleWeights[record.module]) return record.module;
  const category = String(record.category || "").toLowerCase();
  if (record.run_type === "code_eval" || category.includes("coding") || category.includes("code")) return "coding";
  if (category.includes("instruction")) return "instruction_following";
  if (category.includes("long") || category.includes("needle")) return "long_context";
  if (category.includes("chinese") || category.includes("zh") || category.includes("cmmlu") || category.includes("c-eval")) return "chinese";
  if (category.includes("english") || category.includes("en")) return "english";
  if (category.includes("reasoning") || category.includes("math") || category.includes("gpqa") || category.includes("mmlu")) return "reasoning";
  return record.module || "uncategorized";
}

function aggregateScoreRows(records, groupNames, keyFn) {
  const keyMap = new Map();
  for (const r of records) {
    const keyParts = keyFn(r);
    if (!keyParts) continue;
    const key = keyParts.join("\t");
    if (!keyMap.has(key)) keyMap.set(key, { keyParts, items: [] });
    keyMap.get(key).items.push(r);
  }
  return [...keyMap.values()].map(({ keyParts, items }) => {
    const rawScored = items.filter((r) => typeof r.score === "number");
    const scored = rawScored.filter((r) => !isInfrastructureErrorRecord(r));
    const solved = scored.reduce((sum, r) => sum + r.score, 0);
    const rawSolved = rawScored.reduce((sum, r) => sum + r.score, 0);
    const scoreCi = binaryScoreInterval(scored);
    const rawScoreCi = binaryScoreInterval(rawScored);
    const providerErrors = items.filter(isProviderErrorRecord).length;
    const timeouts = items.filter(isTimeoutRecord).length;
    const binaryScored = scored.filter((r) => r.score === 0 || r.score === 1);
    const severeErrors = binaryScored.filter((r) => r.score === 0).length;
    const row = {
      n: items.length,
      valid_n: scored.length,
      valid_coverage: items.length ? scored.length / items.length : 0,
      score: scored.length ? solved / scored.length : 0,
      score_ci_low: scoreCi.low,
      score_ci_high: scoreCi.high,
      raw_score: rawScored.length ? rawSolved / rawScored.length : 0,
      raw_score_ci_low: rawScoreCi.low,
      raw_score_ci_high: rawScoreCi.high,
      solved,
      provider_error_count: providerErrors,
      timeout_count: timeouts,
      provider_error_rate: items.length ? providerErrors / items.length : 0,
      timeout_rate: items.length ? timeouts / items.length : 0,
      severe_error_count: severeErrors,
      severe_error_rate: binaryScored.length ? severeErrors / binaryScored.length : null
    };
    groupNames.forEach((name, index) => {
      row[name] = keyParts[index];
    });
    return row;
  });
}

function capabilityRows(records) {
  const byModel = new Map();
  for (const record of records) {
    if (!record.model || typeof record.score !== "number") continue;
    if (!byModel.has(record.model)) byModel.set(record.model, []);
    byModel.get(record.model).push(record);
  }
  return [...byModel.entries()]
    .map(([model, items]) => {
      const validItems = items.filter((r) => !isInfrastructureErrorRecord(r));
      const moduleScores = aggregateScoreRows(validItems, ["module"], (r) => [inferCapabilityModule(r)]);
      let weightedScore = 0;
      let availableWeight = 0;
      const modules = [];
      for (const row of moduleScores) {
        const weight = capabilityModuleWeights[row.module] || 0;
        if (!weight) continue;
        weightedScore += row.score * weight;
        availableWeight += weight;
        modules.push(row.module);
      }
      const scored = validItems.filter((r) => typeof r.score === "number");
      const solved = scored.reduce((sum, r) => sum + r.score, 0);
      return {
        model,
        n: items.length,
        valid_n: scored.length,
        score: availableWeight ? weightedScore / availableWeight : scored.length ? solved / scored.length : 0,
        modules: modules.sort().join(","),
        module_coverage: Object.keys(capabilityModuleWeights).length
          ? modules.length / Object.keys(capabilityModuleWeights).length
          : 0
      };
    })
    .sort((a, b) => b.score - a.score || b.module_coverage - a.module_coverage);
}

function moduleRows(records) {
  return aggregateScoreRows(records, ["module", "model"], (r) => [inferCapabilityModule(r), r.model])
    .sort((a, b) => a.module.localeCompare(b.module) || b.score - a.score || a.model.localeCompare(b.model));
}

function hardCodingRows(records) {
  const hardDifficulties = new Set(["hard", "very_hard"]);
  return aggregateScoreRows(
    records.filter((r) => inferCapabilityModule(r) === "coding" && hardDifficulties.has(String(r.difficulty || "").toLowerCase())),
    ["model"],
    (r) => [r.model]
  ).sort((a, b) => b.score - a.score || b.solved - a.solved || a.model.localeCompare(b.model));
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
      const rawScored = items.filter((r) => typeof r.score === "number");
      const scored = rawScored.filter((r) => !isInfrastructureErrorRecord(r));
      const solved = scored.reduce((sum, r) => sum + r.score, 0);
      const rawSolved = rawScored.reduce((sum, r) => sum + r.score, 0);
      return {
        model,
        category,
        n: items.length,
        valid_n: scored.length,
        score: scored.length ? solved / scored.length : 0,
        raw_score: rawScored.length ? rawSolved / rawScored.length : 0
      };
    })
    .sort((a, b) => a.category.localeCompare(b.category) || b.score - a.score);
}

function difficultyRows(records) {
  const keyMap = new Map();
  for (const r of records) {
    const key = `${r.model}\t${r.difficulty || "unspecified"}`;
    if (!keyMap.has(key)) keyMap.set(key, []);
    keyMap.get(key).push(r);
  }
  return [...keyMap.entries()]
    .map(([key, items]) => {
      const [model, difficulty] = key.split("\t");
      const rawScored = items.filter((r) => typeof r.score === "number");
      const scored = rawScored.filter((r) => !isInfrastructureErrorRecord(r));
      const solved = scored.reduce((sum, r) => sum + r.score, 0);
      const rawSolved = rawScored.reduce((sum, r) => sum + r.score, 0);
      const totalEstimatedCost = items.reduce((sum, r) => sum + (r.estimated_cost_usd ?? r.cost_usd ?? 0), 0);
      return {
        difficulty,
        model,
        n: items.length,
        valid_n: scored.length,
        score: scored.length ? solved / scored.length : 0,
        raw_score: rawScored.length ? rawSolved / rawScored.length : 0,
        solved,
        total_estimated_cost_usd: totalEstimatedCost,
        p50_latency_ms: percentile(items.map((r) => r.latency_ms), 50),
        p95_latency_ms: percentile(items.map((r) => r.latency_ms), 95)
      };
    })
    .sort((a, b) => a.difficulty.localeCompare(b.difficulty) || b.score - a.score || a.total_estimated_cost_usd - b.total_estimated_cost_usd);
}

function visibilityRows(records) {
  return aggregateScoreRows(records, ["visibility", "model"], (r) => [r.visibility || "unspecified", r.model])
    .sort((a, b) => a.visibility.localeCompare(b.visibility) || b.score - a.score || a.model.localeCompare(b.model));
}

function sourceFamilyRows(records) {
  return aggregateScoreRows(records, ["source_family", "model"], (r) => [r.source_family || "unspecified", r.model])
    .sort((a, b) => a.source_family.localeCompare(b.source_family) || b.score - a.score || a.model.localeCompare(b.model));
}

function singleBaselineModelsFromRecords(records, config) {
  return [
    ...new Set(
      records
        .map((r) => r.model)
        .filter((model) => model && !config.fusionPresets?.[model])
    )
  ];
}

function comparisonRows(records, config) {
  const baselines = singleBaselineModelsFromRecords(records, config);
  const fusions = [
    ...new Set(records.map((r) => r.model).filter((model) => config.fusionPresets?.[model]))
  ];
  const byQuestionModel = new Map();
  for (const r of records) byQuestionModel.set(`${r.question_id}\t${r.model}`, r);
  const questionIds = [...new Set(records.map((r) => r.question_id))];

  return fusions.map((fusion) => {
    let degrade = 0;
    let comparable = 0;
    let winStrongest = 0;
    let loseStrongest = 0;
    let tieStrongest = 0;
    let winGpt = 0;
    let loseGpt = 0;
    let tieGpt = 0;
    for (const qid of questionIds) {
      const f = byQuestionModel.get(`${qid}\t${fusion}`);
      if (!f) continue;
      const bestSingleScore = Math.max(
        ...baselines
          .map((m) => byQuestionModel.get(`${qid}\t${m}`))
          .filter((r) => r && typeof r.score === "number" && !isInfrastructureErrorRecord(r))
          .map((r) => r.score)
      );
      if (Number.isFinite(bestSingleScore)) {
        comparable += 1;
        const fusionScore = f.score || 0;
        if (fusionScore < bestSingleScore) {
          degrade += 1;
          loseStrongest += 1;
        } else if (fusionScore > bestSingleScore) {
          winStrongest += 1;
        } else {
          tieStrongest += 1;
        }
      }
      const g = byQuestionModel.get(`${qid}\topenai/gpt-5.5`);
      if (
        g &&
        f &&
        typeof g.score === "number" &&
        typeof f.score === "number" &&
        !isInfrastructureErrorRecord(g) &&
        !isInfrastructureErrorRecord(f)
      ) {
        if (f.score > g.score) winGpt += 1;
        else if (f.score < g.score) loseGpt += 1;
        else tieGpt += 1;
      }
    }
    const gptComparable = winGpt + loseGpt + tieGpt;
    const oracleDegradationCi = wilsonInterval(degrade, comparable);
    const gptLossCi = wilsonInterval(loseGpt, gptComparable);
    return {
      fusion,
      comparable_vs_oracle_single: comparable,
      degradation_rate_vs_oracle_single: comparable ? degrade / comparable : null,
      degradation_rate_ci_high_vs_oracle_single: oracleDegradationCi.high,
      win_rate_vs_oracle_single: comparable ? winStrongest / comparable : null,
      wins_vs_oracle_single: winStrongest,
      losses_vs_oracle_single: loseStrongest,
      ties_vs_oracle_single: tieStrongest,
      comparable_vs_gpt55: gptComparable,
      win_rate_vs_gpt55: gptComparable ? winGpt / gptComparable : null,
      loss_rate_vs_gpt55: gptComparable ? loseGpt / gptComparable : null,
      loss_rate_ci_high_vs_gpt55: gptLossCi.high,
      wins_vs_gpt55: winGpt,
      losses_vs_gpt55: loseGpt,
      ties_vs_gpt55: tieGpt
    };
  });
}

function fusionPolicyRows(records) {
  const groups = new Map();
  for (const r of records) {
    const metrics = r.fusion_metrics;
    if (!metrics || typeof r.score !== "number") continue;
    const key = [
      r.model,
      metrics.mode || "fusion_model_mode",
      metrics.task_family || inferCapabilityModule(r),
      metrics.policy || "unknown"
    ].join("\t");
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(r);
  }
  return [...groups.entries()]
    .map(([key, items]) => {
      const [model, mode, task_family, policy] = key.split("\t");
      const rawScored = items.filter((r) => typeof r.score === "number");
      const scored = rawScored.filter((r) => !isInfrastructureErrorRecord(r));
      const solved = scored.reduce((sum, r) => sum + r.score, 0);
      const totalEstimatedCost = items.reduce((sum, r) => sum + (r.estimated_cost_usd ?? r.cost_usd ?? 0), 0);
      const selectedModelCounts = new Map();
      for (const item of items) {
        const selectedModel = item.fusion_metrics?.selected_model || "none";
        selectedModelCounts.set(selectedModel, (selectedModelCounts.get(selectedModel) || 0) + 1);
      }
      const selected_models = [...selectedModelCounts.entries()]
        .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
        .map(([selectedModel, count]) => `${selectedModel}:${count}`)
        .join(",");
      const verifierItems = items.filter((r) => Number.isFinite(Number(r.fusion_metrics?.passing_candidate_count)));
      const guardItems = items.filter((r) => r.fusion_metrics?.anti_degradation_guard === true);
      return {
        model,
        mode,
        task_family,
        policy,
        n: items.length,
        valid_n: scored.length,
        score: scored.length ? solved / scored.length : 0,
        solved,
        total_estimated_cost_usd: totalEstimatedCost,
        avg_estimated_cost_usd: items.length ? totalEstimatedCost / items.length : 0,
        p50_latency_ms: percentile(items.map((r) => r.latency_ms), 50),
        p95_latency_ms: percentile(items.map((r) => r.latency_ms), 95),
        verifier_pass_through_rate: verifierItems.length
          ? verifierItems.filter((r) => Number(r.fusion_metrics?.passing_candidate_count || 0) > 0).length / verifierItems.length
          : null,
        anti_degradation_guard_rate: items.length ? guardItems.length / items.length : null,
        selected_models
      };
    })
    .sort(
      (a, b) =>
        a.model.localeCompare(b.model) ||
        a.task_family.localeCompare(b.task_family) ||
        a.policy.localeCompare(b.policy)
    );
}

function fusionRouterRows(records) {
  const rows = [];
  for (const r of records) {
    const metrics = r.fusion_metrics;
    if (!metrics?.router) continue;
    rows.push({
      model: r.model,
      question_id: r.question_id,
      mode: metrics.mode || "fusion_model_mode",
      task_family: metrics.task_family || inferCapabilityModule(r),
      policy: metrics.policy || "unknown",
      panel: (metrics.router.panel || []).join(","),
      included: (metrics.router.included || []).map((item) => item.model).join(","),
      skipped: (metrics.router.skipped || []).map((item) => item.model).join(",")
    });
  }
  return rows;
}

function fusionRankerRows(records) {
  const rows = [];
  for (const r of records) {
    const metrics = r.fusion_metrics;
    if (!metrics) continue;
    const clearWinner = metrics.selection_reason === "ranker_clear_winner" || metrics.policy === "rank_then_select";
    rows.push({
      model: r.model,
      question_id: r.question_id,
      policy: metrics.policy || "unknown",
      task_family: metrics.task_family || inferCapabilityModule(r),
      selected_model: metrics.selected_model || "",
      selection_reason: metrics.selection_reason || "",
      clear_winner: clearWinner ? 1 : 0,
      passing_candidate_count: Number(metrics.passing_candidate_count || 0)
    });
  }
  return rows;
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
    const totalEstimatedCost = items.reduce((sum, r) => sum + (r.estimated_cost_usd ?? r.cost_usd ?? 0), 0);
    const actualCostItems = items.filter((r) => numberOrNull(r.actual_cost_usd) !== null);
    const totalActualCost = actualCostItems.reduce((sum, r) => sum + r.actual_cost_usd, 0);
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
      total_cost_usd: totalEstimatedCost,
      avg_cost_usd: items.length ? totalEstimatedCost / items.length : 0,
      total_estimated_cost_usd: totalEstimatedCost,
      avg_estimated_cost_usd: items.length ? totalEstimatedCost / items.length : 0,
      total_actual_cost_usd: actualCostItems.length ? totalActualCost : null,
      avg_actual_cost_usd: actualCostItems.length ? totalActualCost / actualCostItems.length : null,
      actual_cost_coverage: items.length ? actualCostItems.length / items.length : 0,
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
      const totalEstimatedCost = items.reduce((sum, r) => sum + (r.estimated_cost_usd ?? r.cost_usd ?? 0), 0);
      const actualCostItems = items.filter((r) => numberOrNull(r.actual_cost_usd) !== null);
      const totalActualCost = actualCostItems.reduce((sum, r) => sum + r.actual_cost_usd, 0);
      const failures = items.filter((r) => !r.ok).length;
      const providerErrors = items.filter(
        (r) =>
          r.failure_type === "provider_error" ||
          r.score_reason === "provider_error" ||
          classifyRequestFailure(r.error).failure_type === "provider_error"
      ).length;
      return {
        model,
        n: items.length,
        total_cost_usd: totalEstimatedCost,
        avg_cost_usd: items.length ? totalEstimatedCost / items.length : 0,
        total_estimated_cost_usd: totalEstimatedCost,
        avg_estimated_cost_usd: items.length ? totalEstimatedCost / items.length : 0,
        total_actual_cost_usd: actualCostItems.length ? totalActualCost : null,
        avg_actual_cost_usd: actualCostItems.length ? totalActualCost / actualCostItems.length : null,
        actual_cost_coverage: items.length ? actualCostItems.length / items.length : 0,
        p50_latency_ms: percentile(items.map((r) => r.latency_ms), 50),
        p95_latency_ms: percentile(items.map((r) => r.latency_ms), 95),
        failure_rate: items.length ? failures / items.length : 0,
        provider_error_rate: items.length ? providerErrors / items.length : 0,
        judge_json_validity: judgeValidity(items),
        all_panel_failure_rate: panelFailureRate(items)
      };
    })
    .sort((a, b) => b.total_cost_usd - a.total_cost_usd);
}

function billingSummaryRows(records) {
  const summaries = records.filter((r) => r.run_type === "billing_summary");
  const groups = new Map();
  for (const r of summaries) {
    const key = `${r.source_run_type || ""}\t${r.upstream || ""}\t${r.billing_source || ""}\t${r.unit || ""}`;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(r);
  }
  return [...groups.entries()]
    .map(([key, items]) => {
      const [source_run_type, upstream, billing_source, unit] = key.split("\t");
      const okItems = items.filter((r) => r.ok && numberOrNull(r.actual_cost_usd) !== null);
      const nativeItems = okItems.filter((r) => numberOrNull(r.actual_cost_native) !== null);
      return {
        source_run_type,
        upstream,
        billing_source,
        unit,
        n: items.length,
        ok: okItems.length,
        failures: items.length - okItems.length,
        total_actual_cost_native: nativeItems.length
          ? nativeItems.reduce((sum, r) => sum + r.actual_cost_native, 0)
          : null,
        total_actual_cost_usd: okItems.length
          ? okItems.reduce((sum, r) => sum + r.actual_cost_usd, 0)
          : null
      };
    })
    .sort((a, b) => (b.total_actual_cost_usd || 0) - (a.total_actual_cost_usd || 0));
}

const reportColumnLabels = {
  model: "模型",
  fusion: "Fusion 组合",
  mode: "模式",
  task_family: "任务族",
  policy: "策略",
  selected_models: "选中模型分布",
  included: "选入模型",
  skipped: "跳过模型",
  panel: "panel 模型",
  clear_winner: "清晰赢家",
  selection_reason: "选择原因",
  router_panel: "路由 panel",
  category: "类别",
  module: "模块",
  modules: "覆盖模块",
  module_coverage: "模块覆盖率",
  difficulty: "难度",
  visibility: "题源可见性",
  source_family: "题源家族",
  source_run_type: "来源评测",
  upstream: "上游",
  billing_source: "扣减来源",
  unit: "原始单位",
  n: "样本数",
  valid_n: "有效样本数",
  valid_coverage: "有效覆盖率",
  total_actual_cost_native: "账户扣减 原始单位",
  score: "得分",
  score_ci_low: "得分 Wilson95 下界",
  score_ci_high: "得分 Wilson95 上界",
  raw_score: "原始得分",
  raw_score_ci_low: "原始得分 Wilson95 下界",
  raw_score_ci_high: "原始得分 Wilson95 上界",
  solved: "答对数",
  raw_solved: "原始答对数",
  target: "目标",
  missing: "缺口",
  is_fusion: "Fusion",
  total_cost_usd: "估算总成本 USD",
  avg_cost_usd: "估算平均成本 USD",
  cost_per_solved_usd: "估算每答对成本 USD",
  total_estimated_cost_usd: "估算总成本 USD",
  avg_estimated_cost_usd: "估算平均成本 USD",
  cost_per_solved_estimated_usd: "估算每答对成本 USD",
  total_actual_cost_usd: "可归因扣减 USD",
  avg_actual_cost_usd: "平均可归因扣减 USD",
  cost_per_solved_actual_usd: "每答对可归因扣减 USD",
  actual_cost_coverage: "可归因扣减覆盖率",
  p50_latency_ms: "p50 延迟 ms",
  p95_latency_ms: "p95 延迟 ms",
  early_exit_rate: "早停率",
  p50_panel_max_latency_ms: "panel 最大 p50 ms",
  p50_judge_latency_ms: "judge p50 ms",
  p50_final_latency_ms: "final p50 ms",
  failure_rate: "失败率",
  timeout_rate: "超时率",
  provider_error_rate: "Provider 错误率",
  timeout_count: "超时数",
  provider_error_count: "Provider 错误数",
  severe_error_rate: "严重错误率",
  severe_error_count: "严重错误数",
  judge_json_validity: "judge JSON 有效率",
  all_panel_failure_rate: "panel 全失败率",
  comparable_vs_oracle_single: "逐题最强单模型可比题数",
  degradation_rate_vs_oracle_single: "相对逐题最强单模型退化率",
  degradation_rate_ci_high_vs_oracle_single: "退化率 Wilson95 上界",
  win_rate_vs_oracle_single: "对逐题最强单模型胜率",
  wins_vs_oracle_single: "胜逐题最强单模型",
  losses_vs_oracle_single: "负逐题最强单模型",
  ties_vs_oracle_single: "平逐题最强单模型",
  comparable_vs_gpt55: "GPT-5.5 可比题数",
  win_rate_vs_gpt55: "对 GPT-5.5 胜率",
  loss_rate_vs_gpt55: "负 GPT-5.5 率",
  loss_rate_ci_high_vs_gpt55: "负 GPT-5.5 率 Wilson95 上界",
  wins_vs_gpt55: "胜 GPT-5.5",
  losses_vs_gpt55: "负 GPT-5.5",
  ties_vs_gpt55: "平 GPT-5.5",
  verifier_pass_through_rate: "验证通过直出率",
  anti_degradation_guard_rate: "防退化 guard 率",
  target_model: "目标模型",
  baseline_model: "基准模型",
  judge_model: "裁判模型",
  win_rate: "胜率",
  wins: "胜",
  losses: "负",
  ties: "平",
  invalid: "无效",
  rounds: "轮次",
  ok: "成功数",
  failures: "失败数"
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
          if (col.includes("rate") || col.includes("score") || col.includes("validity") || col.includes("coverage")) {
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

function formatPct(value, digits = 1) {
  return typeof value === "number" && Number.isFinite(value) ? `${(value * 100).toFixed(digits)}%` : "无数据";
}

function formatRatio(value, digits = 1) {
  return typeof value === "number" && Number.isFinite(value) ? `${(value * 100).toFixed(digits)}%` : "无对照";
}

function rowPassesReliabilityGate(row, criteria = {}) {
  if (!row || row.valid_n <= 0) return false;
  const minValidCoverage = criteria.minValidCoverageForStrongBaseline ?? criteria.minValidCoverageForClaim ?? 0.95;
  const maxProviderErrorRate = criteria.maxProviderErrorRateForStrongBaseline ?? criteria.maxProviderErrorRate ?? 0.01;
  const maxTimeoutRate = criteria.maxTimeoutRateForStrongBaseline ?? criteria.maxTimeoutRate ?? 0.02;
  return (
    row.valid_coverage >= minValidCoverage &&
    row.provider_error_rate <= maxProviderErrorRate &&
    row.timeout_rate <= maxTimeoutRate
  );
}

function rowPassesStrongBaselineGate(row, capabilityRow, criteria = {}, questionCount = 0) {
  if (!rowPassesReliabilityGate(row, criteria)) return false;
  const minQuestionCoverage =
    criteria.minQuestionCoverageForStrongBaseline ??
    criteria.minValidCoverageForStrongBaseline ??
    criteria.minValidCoverageForClaim ??
    0.95;
  const minModuleCoverage = criteria.minModuleCoverageForStrongBaseline ?? 1;
  const questionCoverage = questionCount > 0 ? row.n / questionCount : 1;
  return (
    questionCoverage >= minQuestionCoverage &&
    (!capabilityRow || capabilityRow.module_coverage >= minModuleCoverage)
  );
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
  const capabilityRankRows = capabilityRows(objectiveRecords);
  const modelRows = summarize(objectiveRecords, config);
  const modRows = moduleRows(objectiveRecords);
  const hardCodeRows = hardCodingRows(objectiveRecords);
  const catRows = categoryRows(objectiveRecords);
  const diffRows = difficultyRows(objectiveRecords);
  const visibilitySplitRows = visibilityRows(objectiveRecords);
  const sourceFamilySplitRows = sourceFamilyRows(objectiveRecords);
  const compRows = comparisonRows(objectiveRecords, config);
  const policyRows = fusionPolicyRows(objectiveRecords);
  const routerRows = fusionRouterRows(objectiveRecords);
  const rankerRows = fusionRankerRows(objectiveRecords);
  const pairRows = pairwiseRows(records);
  const requestRows = requestLogRows(records);
  const billingRows = billingSummaryRows(records);
  const summaryByModel = new Map(modelRows.map((r) => [r.model, r]));
  const capabilityByModel = new Map(capabilityRankRows.map((r) => [r.model, r]));
  const criteria = config.successCriteria || {};
  const objectiveQuestionCount = new Set(objectiveRecords.map((r) => r.question_id)).size;
  const bestSingleCapability = capabilityRankRows.find(
    (r) =>
      !config.fusionPresets?.[r.model] &&
      rowPassesStrongBaselineGate(summaryByModel.get(r.model), r, criteria, objectiveQuestionCount)
  );
  const bestSingle = bestSingleCapability ? summaryByModel.get(bestSingleCapability.model) : null;
  const mainComparisonModels = new Set([
    ...modelRows.filter((r) => r.is_fusion).map((r) => r.model),
    ...(bestSingleCapability ? [bestSingleCapability.model] : [])
  ]);
  const mainCapabilityRankRows = capabilityRankRows.filter((r) => mainComparisonModels.has(r.model));
  const mainModuleRows = modRows.filter((r) => mainComparisonModels.has(r.model));
  const mainHardCodeRows = hardCodeRows.filter((r) => mainComparisonModels.has(r.model));
  const mainVisibilityRows = visibilitySplitRows.filter((r) => mainComparisonModels.has(r.model));
  const mainSourceFamilyRows = sourceFamilySplitRows.filter((r) => mainComparisonModels.has(r.model));
  const mainCategoryRows = catRows.filter((r) => mainComparisonModels.has(r.model));
  const mainDifficultyRows = diffRows.filter((r) => mainComparisonModels.has(r.model));
  const statisticalRows = modelRows.filter((r) => mainComparisonModels.has(r.model));
  const gpt = modelRows.find((r) => r.model === "openai/gpt-5.5");
  const gptComparable = rowPassesReliabilityGate(gpt, criteria);
  const minArchitectureQuestions = criteria.minObjectiveQuestionsForArchitecture || 100;
  const minPublicClaimQuestions = criteria.minObjectiveQuestionsForPublicClaim || 141;
  const minValidCoverage = criteria.minValidCoverageForClaim ?? 0.99;
  const maxProviderErrorRate = criteria.maxProviderErrorRate ?? 0.01;
  const maxTimeoutRate = criteria.maxTimeoutRate ?? 0.02;
  const maxAllPanelFailureRate = criteria.maxAllPanelFailureRate ?? 0.01;
  const maxOracleDegradationRate = criteria.maxOracleDegradationRate ?? 0;
  const maxOracleDegradationWilsonHigh =
    criteria.maxOracleDegradationWilsonHighForClaim ?? 0.03;
  const enoughEvidenceForArchitecture = objectiveQuestionCount >= minArchitectureQuestions;

  const verdictLines = [];
  for (const row of modelRows.filter((r) => r.is_fusion)) {
    const fusionCapability = capabilityByModel.get(row.model);
    const capabilityScore = fusionCapability?.score ?? row.score;
    const strongestScore = bestSingleCapability?.score ?? bestSingle?.score ?? null;
    const capabilityDelta = strongestScore === null ? null : capabilityScore - strongestScore;
    const comp = compRows.find((r) => r.fusion === row.model);
    const estimatedCostRatio =
      gptComparable && gpt.total_estimated_cost_usd > 0
        ? row.total_estimated_cost_usd / gpt.total_estimated_cost_usd
        : null;
    const estimatedCostRatioVsStrongest =
      bestSingle && bestSingle.total_estimated_cost_usd > 0
        ? row.total_estimated_cost_usd / bestSingle.total_estimated_cost_usd
        : null;
    const p95LatencyRatioVsStrongest =
      bestSingle && bestSingle.p95_latency_ms > 0 ? row.p95_latency_ms / bestSingle.p95_latency_ms : null;
    const hasFullActualCost =
      gptComparable &&
      row.actual_cost_coverage >= 1 &&
      gpt.actual_cost_coverage >= 1 &&
      row.total_actual_cost_usd !== null &&
      gpt.total_actual_cost_usd > 0;
    const actualCostRatio = hasFullActualCost ? row.total_actual_cost_usd / gpt.total_actual_cost_usd : null;
    const oracleComparable = comp?.comparable_vs_oracle_single || 0;
    const oracleLosses = comp?.losses_vs_oracle_single || 0;
    const oracleDegradationRate = comp?.degradation_rate_vs_oracle_single;
    const oracleDegradationCiHigh = comp?.degradation_rate_ci_high_vs_oracle_single;
    const noOracleRegression =
      oracleComparable > 0 &&
      oracleLosses === 0 &&
      (oracleDegradationRate ?? 1) <= maxOracleDegradationRate;
    const noStrongSinglePointRegression = strongestScore === null ? false : capabilityScore >= strongestScore;
    const reliabilityReady =
      row.valid_coverage >= minValidCoverage &&
      row.provider_error_rate <= maxProviderErrorRate &&
      row.timeout_rate <= maxTimeoutRate &&
      (row.all_panel_failure_rate === null || row.all_panel_failure_rate <= maxAllPanelFailureRate);
    const severeErrorReady =
      bestSingle?.severe_error_rate === null ||
      bestSingle?.severe_error_rate === undefined ||
      row.severe_error_rate === null ||
      row.severe_error_rate <= bestSingle.severe_error_rate;
    const publicClaimReady =
      objectiveQuestionCount >= minPublicClaimQuestions &&
      reliabilityReady &&
      noOracleRegression &&
      noStrongSinglePointRegression &&
      severeErrorReady &&
      row.score_ci_low !== null &&
      bestSingle?.score_ci_high !== null &&
      row.score_ci_low > bestSingle.score_ci_high &&
      (oracleDegradationCiHigh ?? 1) <= maxOracleDegradationWilsonHigh;
    const architectureReady =
      enoughEvidenceForArchitecture &&
      reliabilityReady &&
      noOracleRegression &&
      noStrongSinglePointRegression &&
      severeErrorReady;
    const pilotReady = noOracleRegression && noStrongSinglePointRegression && severeErrorReady;
    const verdict = publicClaimReady ? "可公开宣称" : architectureReady ? "可进入架构设计" : pilotReady ? "扩大评测" : "暂缓";
    verdictLines.push(
      `- ${row.model}: ${verdict}；强单模型=${bestSingleCapability?.model || "无"}；能力加权差=${
        capabilityDelta === null ? "无对照" : `${(capabilityDelta * 100).toFixed(1)}pp`
      }；逐题 strongest-single oracle 退化=${oracleLosses}/${oracleComparable}（Wilson95 上界 ${formatPct(
        oracleDegradationCiHigh
      )}）；严重错误=${row.severe_error_count}${bestSingle ? ` vs ${bestSingle.severe_error_count}` : ""}；有效覆盖率=${formatPct(
        row.valid_coverage
      )}；相对强单模型估算成本=${formatRatio(estimatedCostRatioVsStrongest)}；p95 延迟比=${formatRatio(
        p95LatencyRatioVsStrongest
      )}；相对 GPT-5.5 可归因扣减=${
        actualCostRatio === null ? "无完整对照" : `${(actualCostRatio * 100).toFixed(1)}%`
      }；相对 GPT-5.5 估算成本=${
        estimatedCostRatio === null ? "无对照" : `${(estimatedCostRatio * 100).toFixed(1)}%`
      }；证据量=${objectiveQuestionCount}/${minArchitectureQuestions}/${minPublicClaimQuestions}；GPT-5.5 对照=${
        gptComparable ? "可用" : "不可用"
      }`
    );
  }

  const md = [
    "# Fusion 评测报告",
    "",
    `生成时间：${new Date().toISOString()}`,
    "",
    "## 成本口径",
    "",
    "- 估算成本：按 `config.json` 的 token 单价和模型返回的 usage 计算。",
    "- 可归因扣减：只来自中转站响应 body/header 中可提取的账单字段，或配置的账号余额前后差；未提取到时留空，不用估算值填充。",
    "- 硅基流动等上游如果使用代金券，`totalBalance` 下降代表账户权益/代金券消耗，不等于现金支出；`chargeBalance` 更接近充值余额/现金余额口径。",
    "- 中转站账户扣减汇总：对不返回单题费用的上游，使用评测开始/结束的余额差额单独汇总。",
    "- batch 余额差额是上游账号级扣减，不能直接拆成某个单模型或某个 Fusion preset 的成本，除非该批次隔离了单模型/单 preset；模型表只展示可逐请求归因的字段。",
    "- Fusion 官方能力跑分不被整体请求超时截断；Fusion 是多模型流水线，应完整跑完并记录总 `latency_ms`。Fusion 整体超时只用于 smoke/诊断运行。",
    "- 能力得分按有效样本计算，排除 provider/network 错误和上游请求超时；被排除的请求保留在总样本数、原始得分、失败率、超时率和 Provider 错误率中，正式结论前应重跑补齐。代码本地测试超时仍是能力失败，不会被排除。",
    "",
    "## 结论",
    "",
    verdictLines.join("\n") || "- 没有找到 Fusion 评测记录。",
    "",
    "主比较口径：Fusion 的正式结论和主榜只看 Fusion 与同批次能力加权最强单模型，以及逐题最强单模型 oracle；弱单模型仅用于诊断、消融和路由学习，不作为超越目标。",
    "",
    "通过标准：pilot 通过要求能力主榜不低于最强单模型、逐题 strongest-single oracle 零退化、严重错误率不高于最强单模型；进入架构阶段还要求样本量和可靠性达标；公开宣称还要求更大样本以及 Wilson95 区间能支持保守结论。",
    "",
    "## 中转站账户扣减汇总",
    "",
    billingRows.length
      ? markdownTable(billingRows, [
          "source_run_type",
          "upstream",
          "billing_source",
          "unit",
          "n",
          "ok",
          "failures",
          "total_actual_cost_native",
          "total_actual_cost_usd"
        ])
      : "- 没有找到 batch 级中转站账单记录。",
    "",
    "## 能力主榜",
    "",
    "主榜只保留 Fusion 与同批次能力加权最强单模型；其余单模型不作为超越目标。",
    "",
    mainCapabilityRankRows.length
      ? markdownTable(mainCapabilityRankRows, ["model", "n", "valid_n", "score", "modules", "module_coverage"])
      : "- 没有找到能力评分记录。",
    "",
    "## 统计置信与可靠性",
    "",
    "这里仅展示 Fusion 与能力加权最强单模型。Wilson 区间只对二值 0/1 评分行计算；Provider 错误和上游请求超时会降低有效覆盖率，正式结论前必须重跑补齐。",
    "",
    statisticalRows.length
      ? markdownTable(statisticalRows, [
          "model",
          "n",
          "valid_n",
          "valid_coverage",
          "score",
          "score_ci_low",
          "score_ci_high",
          "raw_score",
          "severe_error_count",
          "severe_error_rate",
          "provider_error_count",
          "timeout_count",
          "provider_error_rate",
          "timeout_rate",
          "all_panel_failure_rate"
        ])
      : "- 没有找到统计置信记录。",
    "",
    "## 任务族统计置信",
    "",
    "此表按任务族展示 Fusion 与强单模型的有效覆盖、Wilson 区间和严重错误。任务族样本很少时只能作为风险定位，不能单独支撑公开结论。",
    "",
    mainModuleRows.length
      ? markdownTable(mainModuleRows, [
          "module",
          "model",
          "n",
          "valid_n",
          "valid_coverage",
          "score",
          "score_ci_low",
          "score_ci_high",
          "raw_score",
          "severe_error_count",
          "severe_error_rate",
          "provider_error_count",
          "timeout_count"
        ])
      : "- 没有找到任务族统计记录。",
    "",
    "## 模块主对照",
    "",
    mainModuleRows.length
      ? markdownTable(mainModuleRows, ["module", "model", "n", "valid_n", "score", "raw_score", "solved"])
      : "- 没有找到模块评分记录。",
    "",
    "## 高难编程榜",
    "",
    mainHardCodeRows.length
      ? markdownTable(mainHardCodeRows, ["model", "n", "valid_n", "score", "raw_score", "solved"])
      : "- 没有找到 hard/very_hard 编程记录。",
    "",
    "## 题源可见性主对照",
    "",
    mainVisibilityRows.length
      ? markdownTable(mainVisibilityRows, ["visibility", "model", "n", "valid_n", "score", "raw_score", "solved"])
      : "- 没有找到题源可见性字段。",
    "",
    "## 题源家族主对照",
    "",
    mainSourceFamilyRows.length
      ? markdownTable(mainSourceFamilyRows, ["source_family", "model", "n", "valid_n", "score", "raw_score", "solved"])
      : "- 没有找到题源家族字段。",
    "",
    "## 模型诊断汇总",
    "",
    "此表展示所有模型的成本、延迟和可靠性诊断。Fusion 是否通过不按弱单模型判定，只按结论区的能力加权最强单模型和下方逐题最强单模型 oracle 判定。",
    "",
    markdownTable(modelRows, [
      "model",
      "n",
      "valid_n",
      "valid_coverage",
      "score",
      "score_ci_low",
      "score_ci_high",
      "raw_score",
      "solved",
      "total_estimated_cost_usd",
      "avg_estimated_cost_usd",
      "cost_per_solved_estimated_usd",
      "total_actual_cost_usd",
      "avg_actual_cost_usd",
      "cost_per_solved_actual_usd",
      "actual_cost_coverage",
      "p50_latency_ms",
      "p95_latency_ms",
      "early_exit_rate",
      "p50_panel_max_latency_ms",
      "p50_judge_latency_ms",
      "p50_final_latency_ms",
      "failure_rate",
      "timeout_rate",
      "provider_error_rate",
      "timeout_count",
      "provider_error_count",
      "severe_error_rate",
      "severe_error_count",
      "judge_json_validity",
      "all_panel_failure_rate"
    ]),
    "",
    "## Fusion Policy 汇总",
    "",
    policyRows.length
      ? markdownTable(policyRows, [
          "model",
          "mode",
          "task_family",
          "policy",
          "n",
          "valid_n",
          "score",
          "solved",
          "total_estimated_cost_usd",
          "avg_estimated_cost_usd",
          "p50_latency_ms",
          "p95_latency_ms",
          "verifier_pass_through_rate",
          "anti_degradation_guard_rate",
          "selected_models"
        ])
      : "- 没有找到 Fusion policy 记录。",
    "",
    "## Fusion Router 汇总",
    "",
    routerRows.length
      ? markdownTable(routerRows, [
          "model",
          "question_id",
          "mode",
          "task_family",
          "policy",
          "panel",
          "included",
          "skipped"
        ])
      : "- 没有找到 Fusion router 记录。",
    "",
    "## Fusion Ranker 汇总",
    "",
    rankerRows.length
      ? markdownTable(rankerRows, [
          "model",
          "question_id",
          "policy",
          "task_family",
          "selected_model",
          "selection_reason",
          "clear_winner",
          "passing_candidate_count"
        ])
      : "- 没有找到 Fusion ranker 记录。",
    "",
    "## 分类汇总",
    "",
    markdownTable(mainCategoryRows, ["category", "model", "n", "valid_n", "score", "raw_score"]),
    "",
    "## 难度汇总",
    "",
    mainDifficultyRows.length
      ? markdownTable(mainDifficultyRows, [
          "difficulty",
          "model",
          "n",
          "valid_n",
          "score",
          "raw_score",
          "solved",
          "total_estimated_cost_usd",
          "p50_latency_ms",
          "p95_latency_ms"
        ])
      : "- 没有找到难度字段。",
    "",
    "## Fusion 对比",
    "",
    "这里的单模型基线是逐题 oracle：每道题只取同批次非 Fusion 模型里的最高分，因此不会因为弱单模型拖低比较门槛。",
    "",
    markdownTable(compRows, [
      "fusion",
      "comparable_vs_oracle_single",
      "degradation_rate_vs_oracle_single",
      "degradation_rate_ci_high_vs_oracle_single",
      "win_rate_vs_oracle_single",
      "wins_vs_oracle_single",
      "losses_vs_oracle_single",
      "ties_vs_oracle_single",
      "comparable_vs_gpt55",
      "win_rate_vs_gpt55",
      "loss_rate_vs_gpt55",
      "loss_rate_ci_high_vs_gpt55",
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
          "total_estimated_cost_usd",
          "avg_estimated_cost_usd",
          "total_actual_cost_usd",
          "avg_actual_cost_usd",
          "actual_cost_coverage",
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
          "total_estimated_cost_usd",
          "avg_estimated_cost_usd",
          "total_actual_cost_usd",
          "avg_actual_cost_usd",
          "actual_cost_coverage",
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

function isObjectiveRunRecord(record) {
  return (
    record &&
    record.run_type !== "pairwise_eval" &&
    record.run_type !== "request_log" &&
    record.model &&
    record.question_id &&
    typeof record.score === "number"
  );
}

function preferCanonicalRunRecord(existing, candidate) {
  if (!existing) return candidate;
  const existingInfra = isInfrastructureErrorRecord(existing);
  const candidateInfra = isInfrastructureErrorRecord(candidate);
  if (existingInfra !== candidateInfra) return existingInfra ? candidate : existing;
  if (Boolean(existing.ok) !== Boolean(candidate.ok)) return candidate.ok ? candidate : existing;
  return candidate;
}

function mergeRuns(argv) {
  const inputs = valueOf(argv, "--inputs");
  if (!inputs) throw new Error("Missing --inputs");
  const out = valueOf(
    argv,
    "--out",
    path.join(__dirname, "fusion-benchmark", "runs", `merged-${Date.now()}.jsonl`)
  );
  const files = collectInputFiles(inputs, (file) => file.endsWith(".jsonl"));
  const byQuestionModel = new Map();
  let objectiveRecords = 0;
  let duplicateRecords = 0;
  for (const file of files) {
    for (const record of parseJsonl(file)) {
      if (!isObjectiveRunRecord(record)) continue;
      objectiveRecords += 1;
      const key = `${record.question_id}\t${record.model}`;
      if (byQuestionModel.has(key)) duplicateRecords += 1;
      byQuestionModel.set(key, preferCanonicalRunRecord(byQuestionModel.get(key), record));
    }
  }
  const records = [...byQuestionModel.values()].sort(
    (a, b) =>
      String(a.question_id).localeCompare(String(b.question_id)) ||
      String(a.model).localeCompare(String(b.model))
  );
  fs.mkdirSync(path.dirname(out), { recursive: true });
  fs.writeFileSync(out, records.map((record) => JSON.stringify(record)).join("\n") + (records.length ? "\n" : ""));
  console.log(`Merged ${objectiveRecords} objective records from ${files.length} file(s).`);
  console.log(`Canonical records: ${records.length}; duplicates replaced: ${duplicateRecords}.`);
  console.log(`Wrote ${out}`);
}

function evaluateProductionGate(records, config, targetModel) {
  const objectiveRecords = records.filter(
    (r) => r.run_type !== "pairwise_eval" && r.run_type !== "request_log" && r.model && typeof r.score === "number"
  );
  const capabilityRankRows = capabilityRows(objectiveRecords);
  const modelRows = summarize(objectiveRecords, config);
  const compRows = comparisonRows(objectiveRecords, config);
  const summaryByModel = new Map(modelRows.map((r) => [r.model, r]));
  const capabilityByModel = new Map(capabilityRankRows.map((r) => [r.model, r]));
  const criteria = config.successCriteria || {};
  const target = summaryByModel.get(targetModel);
  const targetCapability = capabilityByModel.get(targetModel);
  const questionCount = new Set(objectiveRecords.map((r) => r.question_id)).size;
  const bestSingleCapability = capabilityRankRows.find(
    (r) =>
      r.model !== targetModel &&
      !config.fusionPresets?.[r.model] &&
      rowPassesStrongBaselineGate(summaryByModel.get(r.model), r, criteria, questionCount)
  );
  const bestSingle = bestSingleCapability ? summaryByModel.get(bestSingleCapability.model) : null;
  const comp = compRows.find((r) => r.fusion === targetModel);
  const minArchitectureQuestions = criteria.minObjectiveQuestionsForArchitecture || 100;
  const minPublicClaimQuestions = criteria.minObjectiveQuestionsForPublicClaim || 141;
  const maxAllPanelFailureRate = criteria.maxAllPanelFailureRate ?? 0.01;
  const maxOracleDegradationRate = criteria.maxOracleDegradationRate ?? 0;
  const maxOracleDegradationWilsonHigh = criteria.maxOracleDegradationWilsonHighForClaim ?? 0.03;
  const minValidCoverage = criteria.minValidCoverageForClaim ?? 0.99;
  const maxProviderErrorRate = criteria.maxProviderErrorRate ?? 0.01;
  const maxTimeoutRate = criteria.maxTimeoutRate ?? 0.02;
  const targetScore = targetCapability?.score ?? target?.score ?? 0;
  const bestSingleScore = bestSingleCapability?.score ?? bestSingle?.score ?? 0;
  const reasons = [];

  if (!target) reasons.push(`Missing target model rows: ${targetModel}`);
  if (!bestSingle) reasons.push("Missing reliable strongest single-model baseline.");
  if (!comp) reasons.push("Missing Fusion comparison/oracle row.");
  if (questionCount < minArchitectureQuestions) {
    reasons.push(`Need at least ${minArchitectureQuestions} questions for architecture gate; found ${questionCount}.`);
  }
  if (questionCount < minPublicClaimQuestions) {
    reasons.push(`Need at least ${minPublicClaimQuestions} questions for public-claim gate; found ${questionCount}.`);
  }
  if (target && target.valid_coverage < minValidCoverage) {
    reasons.push(`Target valid coverage ${formatPct(target.valid_coverage)} below ${formatPct(minValidCoverage)}.`);
  }
  if (target && target.provider_error_rate > maxProviderErrorRate) {
    reasons.push(`Target provider error rate ${formatPct(target.provider_error_rate)} above ${formatPct(maxProviderErrorRate)}.`);
  }
  if (target && target.timeout_rate > maxTimeoutRate) {
    reasons.push(`Target timeout rate ${formatPct(target.timeout_rate)} above ${formatPct(maxTimeoutRate)}.`);
  }
  if (target?.all_panel_failure_rate !== null && target?.all_panel_failure_rate > maxAllPanelFailureRate) {
    reasons.push(`Target all-panel failure rate ${formatPct(target.all_panel_failure_rate)} above ${formatPct(maxAllPanelFailureRate)}.`);
  }
  if (bestSingle && targetScore < bestSingleScore) {
    reasons.push(
      `Target capability score ${formatPct(targetScore)} below reliable strongest single ${bestSingle.model} ${formatPct(bestSingleScore)}.`
    );
  }
  if (target && bestSingle && target.severe_error_rate !== null && bestSingle.severe_error_rate !== null && target.severe_error_rate > bestSingle.severe_error_rate) {
    reasons.push(
      `Target severe error rate ${formatPct(target.severe_error_rate)} above ${bestSingle.model} ${formatPct(bestSingle.severe_error_rate)}.`
    );
  }
  if (comp && (comp.degradation_rate_vs_oracle_single ?? 1) > maxOracleDegradationRate) {
    reasons.push(`Oracle degradation rate ${formatPct(comp.degradation_rate_vs_oracle_single)} above ${formatPct(maxOracleDegradationRate)}.`);
  }
  if (comp && (comp.degradation_rate_ci_high_vs_oracle_single ?? 1) > maxOracleDegradationWilsonHigh) {
    reasons.push(
      `Oracle degradation Wilson95 upper bound ${formatPct(comp.degradation_rate_ci_high_vs_oracle_single)} above ${formatPct(maxOracleDegradationWilsonHigh)}.`
    );
  }
  if (target && bestSingle && target.score_ci_low !== null && bestSingle.score_ci_high !== null && target.score_ci_low <= bestSingle.score_ci_high) {
    reasons.push(
      `Target Wilson95 lower bound ${formatPct(target.score_ci_low)} does not exceed strongest-single upper bound ${formatPct(bestSingle.score_ci_high)}.`
    );
  }

  const pilotReady = Boolean(
    target &&
      bestSingle &&
      comp &&
      targetScore >= bestSingleScore &&
      (comp.degradation_rate_vs_oracle_single ?? 1) <= maxOracleDegradationRate &&
      (target.severe_error_rate === null || bestSingle.severe_error_rate === null || target.severe_error_rate <= bestSingle.severe_error_rate)
  );
  const architectureReady = pilotReady && questionCount >= minArchitectureQuestions && target && rowPassesReliabilityGate(target, criteria);
  const publicClaimReady = architectureReady && reasons.length === 0;
  const stage = publicClaimReady ? "public_claim_ready" : architectureReady ? "architecture_ready" : pilotReady ? "pilot_ready_expand" : "not_ready";
  return {
    stage,
    target_model: targetModel,
    reliable_strongest_single: bestSingle?.model || null,
    question_count: questionCount,
    min_architecture_questions: minArchitectureQuestions,
    min_public_claim_questions: minPublicClaimQuestions,
    target_score: targetScore,
    strongest_single_score: bestSingleScore,
    target_valid_coverage: target?.valid_coverage ?? null,
    oracle_comparable: comp?.comparable_vs_oracle_single ?? 0,
    oracle_losses: comp?.losses_vs_oracle_single ?? null,
    oracle_degradation_ci_high: comp?.degradation_rate_ci_high_vs_oracle_single ?? null,
    target_score_ci_low: target?.score_ci_low ?? null,
    strongest_single_score_ci_high: bestSingle?.score_ci_high ?? null,
    reasons
  };
}

function productionGate(argv) {
  const config = loadConfig(argv);
  const inputs = valueOf(argv, "--inputs");
  if (!inputs) throw new Error("Missing --inputs");
  const target = valueOf(argv, "--target", config.pairwise?.defaultTarget || Object.keys(config.fusionPresets || {})[0]);
  if (!target) throw new Error("Missing --target and no fusion preset configured");
  const files = inputs.split(",").map((f) => f.trim()).filter(Boolean);
  const records = files.flatMap(parseJsonl);
  const gate = evaluateProductionGate(records, config, target);
  console.log(`Production gate: ${gate.stage}`);
  console.log(`Target: ${gate.target_model}`);
  console.log(`Reliable strongest single: ${gate.reliable_strongest_single || "none"}`);
  console.log(`Questions: ${gate.question_count}/${gate.min_architecture_questions}/${gate.min_public_claim_questions}`);
  console.log(`Capability: target=${formatPct(gate.target_score)} strongest=${formatPct(gate.strongest_single_score)}`);
  console.log(`Oracle: losses=${gate.oracle_losses ?? "n/a"}/${gate.oracle_comparable} degradation_ci_high=${formatPct(gate.oracle_degradation_ci_high)}`);
  console.log(`Wilson: target_low=${formatPct(gate.target_score_ci_low)} strongest_high=${formatPct(gate.strongest_single_score_ci_high)}`);
  if (gate.reasons.length) {
    console.log("Reasons:");
    for (const reason of gate.reasons) console.log(`- ${reason}`);
  }
  if (hasFlag(argv, "--json")) console.log(JSON.stringify(gate, null, 2));
  if (gate.stage !== "public_claim_ready") process.exitCode = 1;
}

function runInventory(argv) {
  const config = loadConfig(argv);
  const inputs = valueOf(argv, "--inputs", path.join(__dirname, "fusion-benchmark", "runs"));
  const files = collectInputFiles(inputs, (file) => file.endsWith(".jsonl"));
  const records = files.flatMap(parseJsonl);
  const scored = records.filter((r) => r.question_id && r.model && typeof r.score === "number");
  const questionIds = new Set(scored.map((r) => r.question_id));
  const datasetQuestions = records.filter((r) => r.id && r.scoring && !r.question_id);
  const datasetIds = new Set(datasetQuestions.map((r) => r.id));
  const byModel = new Map();
  const byModule = new Map();
  for (const record of scored) {
    if (!byModel.has(record.model)) byModel.set(record.model, new Set());
    byModel.get(record.model).add(record.question_id);
    const moduleName = inferCapabilityModule(record);
    if (!byModule.has(moduleName)) byModule.set(moduleName, new Set());
    byModule.get(moduleName).add(record.question_id);
  }
  const modelRows = [...byModel.entries()]
    .map(([model, questions]) => ({
      model,
      n: questions.size,
      is_fusion: Boolean(config.fusionPresets?.[model])
    }))
    .sort((a, b) => b.n - a.n || a.model.localeCompare(b.model));
  const moduleRowsForInventory = [...byModule.entries()]
    .map(([module, questions]) => {
      const target = Number(config.productionTargets?.modules?.[module] || 0);
      return {
        module,
        n: questions.size,
        target: target || null,
        missing: target ? Math.max(0, target - questions.size) : null
      };
    })
    .sort((a, b) => b.n - a.n || a.module.localeCompare(b.module));
  const byDatasetModule = new Map();
  for (const question of datasetQuestions) {
    const moduleName = inferCapabilityModule(question);
    if (!byDatasetModule.has(moduleName)) byDatasetModule.set(moduleName, new Set());
    byDatasetModule.get(moduleName).add(question.id);
  }
  const datasetModuleRows = [...byDatasetModule.entries()]
    .map(([module, questions]) => {
      const target = Number(config.productionTargets?.modules?.[module] || 0);
      return {
        module,
        n: questions.size,
        target: target || null,
        missing: target ? Math.max(0, target - questions.size) : null
      };
    })
    .sort((a, b) => b.n - a.n || a.module.localeCompare(b.module));
  const configuredTarget = Number(config.productionTargets?.total || config.successCriteria?.minObjectiveQuestionsForPublicClaim || 141);
  console.log(`Inventory: files=${files.length}`);
  console.log("");
  console.log("Evaluated run inventory:");
  console.log(`Scored records: ${scored.length}`);
  if (scored.length) {
    console.log(`Public-claim target: ${questionIds.size}/${configuredTarget}; missing=${Math.max(0, configuredTarget - questionIds.size)}`);
  } else {
    console.log("No scored run rows in input.");
  }
  if (modelRows.length) {
    console.log("");
    console.log("By model:");
    console.log(markdownTable(modelRows, ["model", "n", "is_fusion"]));
  }
  if (moduleRowsForInventory.length) {
    console.log("");
    console.log("By module:");
    console.log(markdownTable(moduleRowsForInventory, ["module", "n", "target", "missing"]));
  }
  console.log("");
  console.log("Pinned dataset inventory:");
  if (!datasetQuestions.length) {
    console.log("No dataset rows in input.");
    return;
  }
  console.log(`Dataset questions: ${datasetIds.size}`);
  console.log(`Public-claim target: ${datasetIds.size}/${configuredTarget}; missing=${Math.max(0, configuredTarget - datasetIds.size)}`);
  if (datasetModuleRows.length) {
    console.log("");
    console.log("By module:");
    console.log(markdownTable(datasetModuleRows, ["module", "n", "target", "missing"]));
  }
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
        timeoutMs,
        config,
        upstreamName
      });
      const answer = extractContent(response).replace(/\s+/g, " ").slice(0, 60);
      console.log(`ok\t${model}\t${answer}`);
    } catch (err) {
      const msg = String(err.message || "").replace(/https?:\/\/\S+/g, "[redacted-url]").slice(0, 160);
      console.log(`fail\t${model}\t${msg}`);
    }
  }
}

async function billingProbe(argv) {
  const config = loadConfig(argv);
  const requested = valueOf(argv, "--upstream", "all");
  const upstreamNames =
    requested === "all" ? [...new Set(["default", ...Object.keys(config.upstreams || {})])] : [requested];
  const probeCfg = {
    path: valueOf(argv, "--path", config.billing?.balanceDelta?.path || "/user/info"),
    balancePath: valueOf(argv, "--balance-path", config.billing?.balanceDelta?.balancePath || "data.chargeBalance"),
    unit: valueOf(argv, "--unit", config.billing?.balanceDelta?.unit || "cny"),
    timeoutMs: Math.max(1000, Number(valueOf(argv, "--timeout-ms", String(config.billing?.balanceDelta?.timeoutMs || 10000))))
  };

  console.log("Billing probe");
  console.log(`endpoint=${probeCfg.path}`);
  console.log(`balance_path=${probeCfg.balancePath}`);
  console.log(`unit=${probeCfg.unit}`);
  for (const upstreamName of upstreamNames) {
    const apiBase = configuredApiBase(config, upstreamName);
    const apiKey = upstreamAuthKey(config, upstreamName);
    if (!apiBase || !apiKey) {
      console.log(`${upstreamName}\tmissing_config`);
      continue;
    }
    try {
      const result = await fetchBillingBalance(apiBase, apiKey, probeCfg);
      console.log(
        `${upstreamName}\tstatus=${result.status}\tok=${result.ok}\tbalance_found=${
          numberOrNull(result.balance) !== null
        }\ttop_keys=${(result.keys || []).join(",")}\tdata_keys=${(result.dataKeys || []).join(",")}`
      );
    } catch (err) {
      console.log(`${upstreamName}\terror=${String(err.message || err).slice(0, 120)}`);
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
        const costs = response.fusion_metrics
          ? {
              estimated_cost_usd: response.fusion_metrics.estimated_cost_usd ?? response.fusion_metrics.cost_usd ?? 0,
              actual_cost_usd: response.fusion_metrics.actual_cost_usd ?? null
            }
          : costFields(model, response, config);
        samples.push({
          ok: true,
          latency_ms: response._latency_ms ?? Date.now() - started,
          cost_usd: costs.estimated_cost_usd,
          estimated_cost_usd: costs.estimated_cost_usd,
          actual_cost_usd: costs.actual_cost_usd,
          answer: extractContent(response).replace(/\s+/g, " ").slice(0, 40)
        });
      } catch (err) {
        samples.push({
          ok: false,
          latency_ms: Date.now() - started,
          cost_usd: 0,
          estimated_cost_usd: 0,
          actual_cost_usd: null,
          error: String(err.message || "").replace(/https?:\/\/\S+/g, "[redacted-url]").slice(0, 160)
        });
      }
      console.log(`${samples.at(-1).ok ? "ok" : "fail"} ${model} round=${i + 1}/${rounds} latency_ms=${samples.at(-1).latency_ms}`);
    }
    const okSamples = samples.filter((s) => s.ok);
    const actualCostSamples = samples.filter((s) => numberOrNull(s.actual_cost_usd) !== null);
    rows.push({
      model,
      rounds,
      ok: okSamples.length,
      failures: samples.length - okSamples.length,
      failure_rate: samples.length ? (samples.length - okSamples.length) / samples.length : 0,
      p50_latency_ms: percentile(okSamples.map((s) => s.latency_ms), 50),
      p95_latency_ms: percentile(okSamples.map((s) => s.latency_ms), 95),
      avg_estimated_cost_usd: samples.length ? samples.reduce((sum, s) => sum + (s.estimated_cost_usd || 0), 0) / samples.length : 0,
      avg_actual_cost_usd: actualCostSamples.length
        ? actualCostSamples.reduce((sum, s) => sum + s.actual_cost_usd, 0) / actualCostSamples.length
        : null,
      actual_cost_coverage: samples.length ? actualCostSamples.length / samples.length : 0
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
    "avg_estimated_cost_usd",
    "avg_actual_cost_usd",
    "actual_cost_coverage"
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
  const balanceCfg = config.billing?.balanceDelta;
  if (balanceCfg?.enabled && String(balanceCfg.unit || "cny").toLowerCase() === "cny" && !Number(config.billing?.cnyPerUsd)) {
    warnings.push("billing.balanceDelta uses CNY but billing.cnyPerUsd is missing; actual_cost_usd will be blank.");
  }
  const runBalanceCfg = config.billing?.runBalanceDelta;
  if (runBalanceCfg?.enabled && String(runBalanceCfg.unit || "cny").toLowerCase() === "cny" && !Number(config.billing?.cnyPerUsd)) {
    warnings.push("billing.runBalanceDelta uses CNY but billing.cnyPerUsd is missing; total_actual_cost_usd will be blank.");
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
  node tools/fusion-benchmark.mjs billing-probe [--env-file FILE] [--upstream NAME|all] [--path /user/info] [--balance-path data.chargeBalance]
  node tools/fusion-benchmark.mjs latency-probe [--env-file FILE] [--models a,b,c] [--rounds N] [--timeout-ms 60000]
  node tools/fusion-benchmark.mjs fresh-validate --dataset FILE
  node tools/fusion-benchmark.mjs serve [--port 8787] [--upstream-api-base URL] [--log FILE] [--env-file FILE]
  node tools/fusion-benchmark.mjs livebench-plan [--api-base URL]
  node tools/fusion-benchmark.mjs livebench-import --inputs FILE_OR_DIR[,FILE_OR_DIR] [--release YYYY-MM-DD] [--out FILE]
  node tools/fusion-benchmark.mjs fresh-run --dataset FILE [--out FILE] [--api-base URL] [--models a,b,c] [--category a,b] [--difficulty simple,hard] [--per-category-limit N] [--offset N] [--limit N] [--env-file FILE]
  node tools/fusion-benchmark.mjs code-validate --dataset FILE
  node tools/fusion-benchmark.mjs code-run --dataset FILE [--out FILE] [--api-base URL] [--models a,b,c] [--category a,b] [--difficulty hard,very_hard] [--per-category-limit N] [--offset N] [--limit N] [--request-timeout-ms 120000] [--overall-request-timeout-ms 300000] [--fusion-overall-request-timeout-ms 600000] [--env-file FILE]
  node tools/fusion-benchmark.mjs self-test
  node tools/fusion-benchmark.mjs run-inventory [--inputs FILE_OR_DIR[,FILE_OR_DIR]]
  node tools/fusion-benchmark.mjs merge-runs --inputs FILE_OR_DIR[,FILE_OR_DIR] [--out FILE]
  node tools/fusion-benchmark.mjs production-gate --inputs FILE[,FILE] [--target MODEL] [--json]
  node tools/fusion-benchmark.mjs pairwise-run --dataset FILE [--out FILE] [--api-base URL] [--target MODEL] [--baseline MODEL] [--judge MODEL] [--env-file FILE]
  node tools/fusion-benchmark.mjs report --inputs FILE[,FILE] [--out FILE]

Global:
  --config FILE  Override tools/fusion-benchmark/config.json
  --env-file FILE  Load API keys from a local env file without printing them

Code-run timeout notes:
  --request-timeout-ms is per upstream call. For Fusion, it applies to each panel,
  judge, and final call. --overall-request-timeout-ms applies to baseline models,
  but Fusion code-run records ignore it by default. Use
  --fusion-overall-request-timeout-ms only for deliberate diagnostic truncation.
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
  if (cmd === "billing-probe") return billingProbe(argv);
  if (cmd === "latency-probe") return latencyProbe(argv);
  if (cmd === "fresh-validate") return freshValidate(argv);
  if (cmd === "serve") return serve(argv);
  if (cmd === "livebench-plan") return livebenchPlan(argv);
  if (cmd === "livebench-import") return livebenchImport(argv);
  if (cmd === "fresh-run") return freshRun(argv);
  if (cmd === "code-validate") return codeValidate(argv);
  if (cmd === "code-run") return codeRun(argv);
  if (cmd === "pairwise-run") return pairwiseRun(argv);
  if (cmd === "run-inventory") return runInventory(argv);
  if (cmd === "merge-runs") return mergeRuns(argv);
  if (cmd === "production-gate") return productionGate(argv);
  if (cmd === "report") return report(argv);
  if (cmd === "self-test") return selfTest();
  throw new Error(`Unknown command: ${cmd}`);
}

main().catch((err) => {
  console.error(err.stack || err.message);
  process.exit(1);
});
