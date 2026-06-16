#!/usr/bin/env node

import fs from 'fs/promises';
import net from 'net';
import path from 'path';
import { spawn } from 'child_process';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '..');
const dataProxyRoot = path.resolve(repoRoot, '..', '..');
const qidianBrowserRoot = path.resolve(repoRoot, '..', '..', '..', 'QidianBrowser');

const DEFAULT_DSN =
  'root:my-secret-pw@tcp(127.0.0.1:3306)/data-proxy?charset=utf8mb4&parseTime=true&loc=Local';
const DEFAULT_QIDIAN_MOCK = path.join(qidianBrowserRoot, 'tools', 'mock-bridge-client.mjs');
const DEFAULT_WORKSPACE = dataProxyRoot;
const DEFAULT_PRICE_PER_CALL = 0.001;
const DEFAULT_INITIAL_QUOTA = 100000;
const DEFAULT_TIMEOUT_MS = 180000;
const DEFAULT_REMOTE_BRIDGE_TIMEOUT_MS = 500;
const GO_CACHE_ROOT = '/Volumes/fushilu/.caches/gocache';
const MCP_ERROR_EXECUTOR_FAILED = -32003;
const MCP_ERROR_EXECUTOR_TIMEOUT = -32004;
const MCP_ERROR_BRIDGE_UNAVAILABLE = -32005;

function parseArgs(argv) {
  const args = {};
  for (const item of argv) {
    if (!item.startsWith('--')) continue;
    const eq = item.indexOf('=');
    if (eq === -1) {
      args[item.slice(2)] = true;
    } else {
      args[item.slice(2, eq)] = item.slice(eq + 1);
    }
  }
  return args;
}

function printHelp() {
  console.log(`MCP Bridge end-to-end smoke

Usage:
  node tools/mcp_bridge_smoke.mjs [options]

Options:
  --dsn=<dsn>              MySQL DSN, defaults to local Docker data-proxy.
  --port=<port>            new-api port. Defaults to a free local port.
  --workspace=<path>       Workspace exposed by QidianBrowser mock.
  --mock=<path>            QidianBrowser mock bridge client path.
  --file=<path>            File path passed to remote_read. Default README.md.
  --price=<number>         Temporary remote_read price_per_call. Default 0.001.
  --quota=<number>         Initial user/token quota. Default 100000.
  --bridge-timeout-ms=<n>  new-api remote bridge timeout. Default ${DEFAULT_REMOTE_BRIDGE_TIMEOUT_MS}.
  --keep-data              Keep smoke DB rows for inspection.
  --help                   Show this help.

The script starts new-api and QidianBrowser mock locally, calls /mcp/v1,
then verifies MCP call logs, bridge audit logs, and user/token quota settlement.
`);
}

const args = parseArgs(process.argv.slice(2));
if (args.help) {
  printHelp();
  process.exit(0);
}

const config = {
  dsn: args.dsn || process.env.SQL_DSN || DEFAULT_DSN,
  port: args.port ? Number(args.port) : 0,
  workspace: path.resolve(args.workspace || process.env.MCP_BRIDGE_SMOKE_WORKSPACE || DEFAULT_WORKSPACE),
  mockClientPath: path.resolve(args.mock || process.env.QIDIAN_BRIDGE_MOCK || DEFAULT_QIDIAN_MOCK),
  filePath: args.file || 'README.md',
  pricePerCall: Number(args.price || process.env.MCP_BRIDGE_SMOKE_PRICE || DEFAULT_PRICE_PER_CALL),
  initialQuota: Number(args.quota || process.env.MCP_BRIDGE_SMOKE_QUOTA || DEFAULT_INITIAL_QUOTA),
  keepData: args['keep-data'] === true || process.env.MCP_BRIDGE_SMOKE_KEEP_DATA === '1',
  timeoutMs: Number(args.timeout || process.env.MCP_BRIDGE_SMOKE_TIMEOUT_MS || DEFAULT_TIMEOUT_MS),
  remoteBridgeTimeoutMs: Number(
    args['bridge-timeout-ms'] ||
      process.env.MCP_REMOTE_BRIDGE_TIMEOUT_MS ||
      DEFAULT_REMOTE_BRIDGE_TIMEOUT_MS,
  ),
};

if (!Number.isFinite(config.pricePerCall) || config.pricePerCall < 0) {
  fail(`invalid --price: ${args.price}`);
}
if (!Number.isInteger(config.initialQuota) || config.initialQuota <= 0) {
  fail(`invalid --quota: ${args.quota}`);
}
if (!Number.isInteger(config.remoteBridgeTimeoutMs) || config.remoteBridgeTimeoutMs <= 0) {
  fail(`invalid --bridge-timeout-ms: ${args['bridge-timeout-ms']}`);
}

const children = [];
const stepState = {
  prepared: null,
  cleaned: false,
};

function log(message, detail) {
  if (detail === undefined) {
    console.log(`[smoke] ${message}`);
  } else {
    console.log(`[smoke] ${message}`, detail);
  }
}

function fail(message) {
  console.error(`[smoke] ${message}`);
  process.exit(1);
}

function makeEnv(extra = {}) {
  const cacheRoot = process.env.MCP_GO_CACHE_ROOT || GO_CACHE_ROOT;
  return {
    ...process.env,
    GOPATH: process.env.GOPATH || path.join(cacheRoot, 'gopath'),
    GOMODCACHE: process.env.GOMODCACHE || path.join(cacheRoot, 'pkg', 'mod'),
    GOCACHE: process.env.GOCACHE || path.join(cacheRoot, 'build'),
    GOTMPDIR: process.env.GOTMPDIR || path.join(cacheRoot, 'tmp'),
    GOTOOLCHAIN: process.env.GOTOOLCHAIN || 'auto',
    SQL_DSN: config.dsn,
    SESSION_SECRET: process.env.SESSION_SECRET || 'mcp-bridge-smoke-session-secret',
    NODE_TYPE: process.env.NODE_TYPE || 'master',
    MEMORY_CACHE_ENABLED: 'false',
    REDIS_CONN_STRING: '',
    BATCH_UPDATE_ENABLED: 'false',
    CHANNEL_UPDATE_FREQUENCY: '',
    ...extra,
  };
}

async function ensureDevEmbedFiles() {
  const html = '<!doctype html><html><head><title>mcp smoke</title></head><body>mcp smoke</body></html>\n';
  for (const rel of ['web/default/dist/index.html']) {
    const abs = path.join(repoRoot, rel);
    try {
      await fs.access(abs);
    } catch {
      await fs.mkdir(path.dirname(abs), { recursive: true });
      await fs.writeFile(abs, html, 'utf8');
      log(`created minimal dev embed file: ${rel}`);
    }
  }
}

async function ensureGoCacheDirs() {
  const env = makeEnv();
  await Promise.all([
    fs.mkdir(env.GOPATH, { recursive: true }),
    fs.mkdir(env.GOMODCACHE, { recursive: true }),
    fs.mkdir(env.GOCACHE, { recursive: true }),
    fs.mkdir(env.GOTMPDIR, { recursive: true }),
  ]);
}

async function findFreePort() {
  if (config.port > 0) return config.port;
  return await new Promise((resolve, reject) => {
    const server = net.createServer();
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      const port = address.port;
      server.close(() => resolve(port));
    });
  });
}

function spawnLogged(name, command, spawnArgs, options = {}) {
  const child = spawn(command, spawnArgs, {
    cwd: options.cwd || repoRoot,
    env: options.env || process.env,
    detached: options.detached !== false,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  child.stdout.setEncoding('utf8');
  child.stderr.setEncoding('utf8');
  child.stdout.on('data', (chunk) => writePrefixed(name, chunk));
  child.stderr.on('data', (chunk) => writePrefixed(name, chunk));
  child.on('exit', (code, signal) => {
    if (!options.quietExit) {
      log(`${name} exited`, { code, signal });
    }
  });
  children.push(child);
  return child;
}

function writePrefixed(name, chunk) {
  for (const line of chunk.split(/\r?\n/)) {
    if (line.trim() === '') continue;
    console.log(`[${name}] ${line}`);
  }
}

function runCommand(command, spawnArgs, options = {}) {
  return new Promise((resolve, reject) => {
    let stdout = '';
    let stderr = '';
    const child = spawn(command, spawnArgs, {
      cwd: options.cwd || repoRoot,
      env: options.env || process.env,
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    child.stdout.setEncoding('utf8');
    child.stderr.setEncoding('utf8');
    child.stdout.on('data', (chunk) => {
      stdout += chunk;
      if (options.prefix) writePrefixed(options.prefix, chunk);
    });
    child.stderr.on('data', (chunk) => {
      stderr += chunk;
      if (options.prefix) writePrefixed(options.prefix, chunk);
    });
    child.on('error', reject);
    child.on('close', (code) => {
      if (code === 0) {
        resolve({ stdout, stderr });
        return;
      }
      const err = new Error(`${command} ${spawnArgs.join(' ')} exited with ${code}`);
      err.stdout = stdout;
      err.stderr = stderr;
      reject(err);
    });
  });
}

async function runSmokeHelper(action, payload = {}) {
  await fs.mkdir(path.join(repoRoot, '.test'), { recursive: true });
  const helperPath = path.join(repoRoot, '.test', 'mcp_bridge_smoke_helper.go');
  await fs.writeFile(helperPath, helperSource(), 'utf8');
  const env = makeEnv({
    MCP_BRIDGE_SMOKE_ACTION: action,
    MCP_BRIDGE_SMOKE_PAYLOAD: JSON.stringify(payload),
  });
  try {
    const result = await runCommand('go', ['run', './.test/mcp_bridge_smoke_helper.go'], {
      cwd: repoRoot,
      env,
      prefix: `go:${action}`,
    });
    const output = `${result.stdout}\n${result.stderr}`;
    const line = output
      .split(/\r?\n/)
      .find((item) => item.startsWith('SMOKE_JSON:'));
    if (!line) {
      throw new Error(`helper did not return SMOKE_JSON for action ${action}`);
    }
    return JSON.parse(line.slice('SMOKE_JSON:'.length));
  } finally {
    await fs.rm(helperPath, { force: true });
  }
}

function helperSource() {
  return String.raw`package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type preparePayload struct {
	Suffix       string  ` + "`json:\"suffix\"`" + `
	PricePerCall float64 ` + "`json:\"price_per_call\"`" + `
	InitialQuota int     ` + "`json:\"initial_quota\"`" + `
}

type inspectPayload struct {
	UserId int ` + "`json:\"user_id\"`" + `
	TokenId int ` + "`json:\"token_id\"`" + `
	CallId int64 ` + "`json:\"call_id\"`" + `
}

type cleanupPayload struct {
	UserId int ` + "`json:\"user_id\"`" + `
	TokenId int ` + "`json:\"token_id\"`" + `
	Suffix string ` + "`json:\"suffix\"`" + `
	OriginalPrice float64 ` + "`json:\"original_price\"`" + `
	OriginalDiskThreshold int ` + "`json:\"original_disk_threshold\"`" + `
}

func main() {
	action := os.Getenv("MCP_BRIDGE_SMOKE_ACTION")
	payload := []byte(os.Getenv("MCP_BRIDGE_SMOKE_PAYLOAD"))
	if action == "" {
		panic("MCP_BRIDGE_SMOKE_ACTION is required")
	}

	common.InitEnv()
	logger.SetupLogger()
	ratio_setting.InitRatioSettings()
	if err := model.InitDB(); err != nil {
		panic(err)
	}
	if err := model.InitLogDB(); err != nil {
		panic(err)
	}
	model.InitOptionMap()
	defer func() {
		_ = model.CloseDB()
	}()

	var result any
	var err error
	switch action {
	case "prepare":
		var p preparePayload
		mustUnmarshal(payload, &p)
		result, err = prepare(p)
	case "inspect":
		var p inspectPayload
		mustUnmarshal(payload, &p)
		result, err = inspect(p)
	case "cleanup":
		var p cleanupPayload
		mustUnmarshal(payload, &p)
		result, err = cleanup(p)
	default:
		err = fmt.Errorf("unknown action %s", action)
	}
	if err != nil {
		panic(err)
	}
	bytes, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	fmt.Println("SMOKE_JSON:" + string(bytes))
}

func mustUnmarshal(payload []byte, target any) {
	if len(payload) == 0 {
		return
	}
	if err := json.Unmarshal(payload, target); err != nil {
		panic(err)
	}
}

func prepare(p preparePayload) (map[string]any, error) {
	if p.Suffix == "" {
		p.Suffix = common.GetRandomString(12)
	}
	if p.InitialQuota <= 0 {
		p.InitialQuota = 100000
	}
	if err := model.SeedBuiltinMCPTools(); err != nil {
		return nil, err
	}
	originalDiskThreshold := common.GetPerformanceMonitorConfig().DiskThreshold
	if err := model.UpdateOption("performance_setting.monitor_disk_threshold", "100"); err != nil {
		return nil, err
	}
	tool, err := model.GetEnabledMCPToolByName("remote_read")
	if err != nil {
		return nil, err
	}
	originalPrice := tool.PricePerCall
	if _, err := model.UpdateMCPToolFields(tool.Id, map[string]any{
		"price_per_call": p.PricePerCall,
		"price_unit": model.MCPToolPriceUnitPerCall,
		"is_remote": true,
		"status": model.MCPToolStatusEnabled,
	}); err != nil {
		return nil, err
	}

	accessToken := "mcpbridge" + p.Suffix + "access000000000000"
	if len(accessToken) > 32 {
		accessToken = accessToken[:32]
	}
	hashedPassword, err := common.Password2Hash("mcp-bridge-smoke-password")
	if err != nil {
		return nil, err
	}
	user := model.User{
		Username: "mcpb" + p.Suffix,
		Password: hashedPassword,
		DisplayName: "MCP Bridge Smoke",
		Role: common.RoleRootUser,
		Status: common.UserStatusEnabled,
		Quota: p.InitialQuota,
		Group: "default",
	}
	user.SetAccessToken(accessToken)
	if err := model.DB.Create(&user).Error; err != nil {
		return nil, err
	}

	tokenKey := "mcpbridge" + p.Suffix
	tokenKey = strings.ReplaceAll(tokenKey, "-", "")
	token := model.Token{
		UserId: user.Id,
		Key: tokenKey,
		Status: common.TokenStatusEnabled,
		Name: "MCP Bridge Smoke Token",
		RemainQuota: p.InitialQuota,
		UnlimitedQuota: false,
		ExpiredTime: -1,
		Group: "default",
	}
	if err := model.DB.Create(&token).Error; err != nil {
		return nil, err
	}

	return map[string]any{
		"suffix": p.Suffix,
		"user_id": user.Id,
		"token_id": token.Id,
		"api_token": "sk-" + token.Key,
		"access_token": accessToken,
		"initial_user_quota": user.Quota,
		"initial_token_quota": token.RemainQuota,
		"original_price": originalPrice,
		"original_disk_threshold": originalDiskThreshold,
		"price_per_call": p.PricePerCall,
		"quota_per_unit": common.QuotaPerUnit,
		"group_ratio": ratio_setting.GetGroupRatio("default"),
		"expected_quota": int(p.PricePerCall * common.QuotaPerUnit * ratio_setting.GetGroupRatio("default") + 0.5),
	}, nil
}

func inspect(p inspectPayload) (map[string]any, error) {
	var user model.User
	if err := model.DB.Select("id", "quota", "used_quota", "request_count").Where("id = ?", p.UserId).First(&user).Error; err != nil {
		return nil, err
	}
	var token model.Token
	if err := model.DB.Select("id", "remain_quota", "used_quota").Where("id = ?", p.TokenId).First(&token).Error; err != nil {
		return nil, err
	}
	var call model.MCPToolCall
	if p.CallId > 0 {
		if err := model.DB.Where("id = ?", p.CallId).First(&call).Error; err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"user_quota": user.Quota,
		"user_used_quota": user.UsedQuota,
		"user_request_count": user.RequestCount,
		"token_remain_quota": token.RemainQuota,
		"token_used_quota": token.UsedQuota,
		"call_quota": call.Quota,
		"call_status": call.Status,
		"call_settled_at": call.SettledAt,
		"call_target_client": call.TargetClient,
		"call_bridge_session_id": call.BridgeSessionId,
	}, nil
}

func cleanup(p cleanupPayload) (map[string]any, error) {
	tool, err := model.GetEnabledMCPToolByName("remote_read")
	if err != nil {
		return nil, err
	}
	if _, err := model.UpdateMCPToolFields(tool.Id, map[string]any{"price_per_call": p.OriginalPrice}); err != nil {
		return nil, err
	}
	if p.Suffix != "" {
		pattern := "%mcp-bridge-smoke-" + p.Suffix + "%"
		_ = model.DB.Where("request_id LIKE ?", pattern).Delete(&model.BillingEvent{}).Error
		_ = model.DB.Where("request_id LIKE ?", pattern).Delete(&model.BridgeAuditLog{}).Error
		_ = model.DB.Where("request_id LIKE ?", pattern).Delete(&model.MCPToolCall{}).Error
		_ = model.DB.Where("client_id = ?", "qidian-smoke-" + p.Suffix).Delete(&model.BridgeSession{}).Error
		_ = model.DB.Unscoped().Where("client_id = ?", "qidian-smoke-" + p.Suffix).Delete(&model.BridgeClient{}).Error
	}
	if p.TokenId > 0 {
		_ = model.DB.Unscoped().Where("id = ?", p.TokenId).Delete(&model.Token{}).Error
	}
	if p.UserId > 0 {
		_ = model.DB.Unscoped().Where("id = ?", p.UserId).Delete(&model.User{}).Error
	}
	if p.OriginalDiskThreshold > 0 {
		if err := model.UpdateOption("performance_setting.monitor_disk_threshold", fmt.Sprint(p.OriginalDiskThreshold)); err != nil {
			return nil, err
		}
	}
	return map[string]any{"cleaned": true}, nil
}
`;
}

async function waitForHTTP(url, timeoutMs) {
  const started = Date.now();
  let lastError;
  while (Date.now() - started < timeoutMs) {
    try {
      const response = await fetch(url);
      if (response.ok) return;
      lastError = new Error(`HTTP ${response.status}`);
    } catch (err) {
      lastError = err;
    }
    await sleep(500);
  }
  throw new Error(`timeout waiting for ${url}: ${lastError?.message || 'unknown error'}`);
}

async function waitForBridgeClient(baseUrl, authHeaders, clientId, timeoutMs) {
  const started = Date.now();
  let last;
  while (Date.now() - started < timeoutMs) {
    try {
      const json = await apiGet(`${baseUrl}/api/bridge/clients?scope=all&page_size=20`, authHeaders);
      const items = pageItems(json);
      const found = items.find((item) => item.client_id === clientId && item.online === true);
      if (found) return found;
      last = `registered clients: ${items.map((item) => `${item.client_id}:${item.online}`).join(', ')}`;
    } catch (err) {
      last = err.message;
    }
    await sleep(500);
  }
  throw new Error(`timeout waiting for bridge client ${clientId}; ${last || ''}`);
}

async function apiGet(url, headers) {
  const response = await fetch(url, { headers });
  const json = await response.json().catch(() => ({}));
  if (!response.ok || json.success !== true) {
    throw new Error(`GET ${url} failed: HTTP ${response.status} ${JSON.stringify(json)}`);
  }
  return json;
}

async function postMCP(baseUrl, apiToken, request) {
  const response = await fetch(`${baseUrl}/mcp/v1`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${apiToken}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(request),
  });
  const json = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(`MCP HTTP failed: ${response.status} ${JSON.stringify(json)}`);
  }
  return json;
}

function pageItems(json) {
  return json?.data?.items || [];
}

function firstPageItem(json, label) {
  const items = pageItems(json);
  if (items.length < 1) {
    throw new Error(`${label} returned no items: ${JSON.stringify(json)}`);
  }
  return items[0];
}

async function callMCPToolAndVerifySuccess(baseUrl, apiToken, dashboardHeaders, clientId, requestId, name, toolArgs) {
  log(`calling MCP ${name}`, { requestId });
  const response = await postMCP(baseUrl, apiToken, {
    jsonrpc: '2.0',
    id: requestId,
    method: 'tools/call',
    params: {
      name,
      arguments: toolArgs || {},
    },
  });
  if (response.error) {
    throw new Error(`${name} returned error: ${JSON.stringify(response.error)}`);
  }
  const text = response?.result?.content?.[0]?.text || '';
  if (!text.trim()) {
    throw new Error(`${name} result did not include text content: ${JSON.stringify(response)}`);
  }

  const callJson = await apiGet(
    `${baseUrl}/api/mcp/tool-calls?scope=all&page_size=10&request_id=${encodeURIComponent(requestId)}`,
    dashboardHeaders,
  );
  const call = firstPageItem(callJson, `${name} MCP tool-calls`);
  if (call.status !== 'success') {
    throw new Error(`${name} MCP call status mismatch: ${JSON.stringify(call)}`);
  }
  if (call.target_client !== clientId) {
    throw new Error(`${name} target_client mismatch, got ${call.target_client}, want ${clientId}`);
  }

  const auditJson = await apiGet(
    `${baseUrl}/api/bridge/audit-logs?scope=all&page_size=10&request_id=${encodeURIComponent(requestId)}`,
    dashboardHeaders,
  );
  const audit = firstPageItem(auditJson, `${name} bridge audit-logs`);
  if (audit.status !== 'success') {
    throw new Error(`${name} bridge audit status mismatch: ${JSON.stringify(audit)}`);
  }
  if (audit.client_id !== clientId) {
    throw new Error(`${name} bridge audit client mismatch, got ${audit.client_id}, want ${clientId}`);
  }
  return { response, call, audit, text };
}

async function listBillingEventsForRequest(baseUrl, dashboardHeaders, requestId) {
  const json = await apiGet(
    `${baseUrl}/api/billing/events?scope=all&page_size=10&source=mcp_tool_call&request_id=${encodeURIComponent(requestId)}`,
    dashboardHeaders,
  );
  return pageItems(json);
}

async function verifyMCPToolFailure(baseUrl, apiToken, dashboardHeaders, options) {
  const {
    requestId,
    name = 'remote_read',
    arguments: toolArgs = {},
    expectedMCPErrorCode,
    expectedCallStatus,
    expectedExecutorErrorCode,
    expectedAuditStatus,
    expectedAuditErrorCode,
    expectedQuota,
    expectAudit = true,
    clientId = '',
  } = options;

  log(`calling MCP ${name} failure scenario`, {
    requestId,
    expected: expectedExecutorErrorCode,
  });
  const response = await postMCP(baseUrl, apiToken, {
    jsonrpc: '2.0',
    id: requestId,
    method: 'tools/call',
    params: {
      name,
      arguments: toolArgs,
    },
  });
  if (!response.error) {
    throw new Error(`${requestId} expected MCP error, got ${JSON.stringify(response)}`);
  }
  if (response.error.code !== expectedMCPErrorCode) {
    throw new Error(`${requestId} MCP error code mismatch, got ${response.error.code}, want ${expectedMCPErrorCode}`);
  }

  const callJson = await apiGet(
    `${baseUrl}/api/mcp/tool-calls?scope=all&page_size=10&request_id=${encodeURIComponent(requestId)}`,
    dashboardHeaders,
  );
  const call = firstPageItem(callJson, `${requestId} MCP tool-calls`);
  if (call.status !== expectedCallStatus || call.error_code !== expectedExecutorErrorCode) {
    throw new Error(`${requestId} call state mismatch: ${JSON.stringify(call)}`);
  }
  if (call.quota !== 0 || call.cost !== 0 || !call.settled_at || call.settled_at <= 0) {
    throw new Error(`${requestId} refunded call net billing mismatch: ${JSON.stringify(call)}`);
  }
  if (clientId && call.target_client !== clientId) {
    throw new Error(`${requestId} target_client mismatch: ${call.target_client} != ${clientId}`);
  }

  const billingEvents = await listBillingEventsForRequest(baseUrl, dashboardHeaders, requestId);
  const settlement = billingEvents.find((item) => item.event_type === 'debit');
  const refund = billingEvents.find((item) => item.event_type === 'credit');
  if (!settlement || !refund) {
    throw new Error(`${requestId} expected settlement and refund billing events: ${JSON.stringify(billingEvents)}`);
  }
  if (settlement.amount_quota !== expectedQuota || settlement.quota_delta !== -expectedQuota) {
    throw new Error(`${requestId} settlement billing event mismatch: ${JSON.stringify(settlement)}`);
  }
  if (refund.amount_quota !== expectedQuota || refund.quota_delta !== expectedQuota) {
    throw new Error(`${requestId} refund billing event mismatch: ${JSON.stringify(refund)}`);
  }
  if (refund.source_id !== String(call.id) || settlement.source_id !== String(call.id)) {
    throw new Error(`${requestId} billing source_id mismatch: ${JSON.stringify(billingEvents)}`);
  }

  const auditJson = await apiGet(
    `${baseUrl}/api/bridge/audit-logs?scope=all&page_size=10&request_id=${encodeURIComponent(requestId)}`,
    dashboardHeaders,
  );
  const auditItems = pageItems(auditJson);
  if (!expectAudit) {
    if (auditItems.length !== 0) {
      throw new Error(`${requestId} expected no bridge audit logs, got ${JSON.stringify(auditItems)}`);
    }
    return { response, call, billingEvents, audit: null };
  }
  const audit = firstPageItem(auditJson, `${requestId} bridge audit-logs`);
  if (audit.status !== expectedAuditStatus || audit.error_code !== expectedAuditErrorCode) {
    throw new Error(`${requestId} bridge audit state mismatch: ${JSON.stringify(audit)}`);
  }
  if (clientId && audit.client_id !== clientId) {
    throw new Error(`${requestId} bridge audit client mismatch: ${audit.client_id} != ${clientId}`);
  }
  return { response, call, billingEvents, audit };
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function stopChildren() {
  for (const child of [...children].reverse()) {
    if (!child || child.killed || child.exitCode !== null) continue;
    killChildProcessGroup(child, 'SIGTERM');
  }
  await sleep(800);
  for (const child of [...children].reverse()) {
    if (!child || child.killed || child.exitCode !== null) continue;
    killChildProcessGroup(child, 'SIGKILL');
  }
}

function killChildProcessGroup(child, signal) {
  try {
    process.kill(-child.pid, signal);
  } catch {
    try {
      child.kill(signal);
    } catch {
      // Process already exited.
    }
  }
}

async function cleanup() {
  if (stepState.cleaned) return;
  stepState.cleaned = true;
  await stopChildren();
  if (stepState.prepared && !config.keepData) {
    try {
      await runSmokeHelper('cleanup', {
        user_id: stepState.prepared.user_id,
        token_id: stepState.prepared.token_id,
        suffix: stepState.prepared.suffix,
        original_price: stepState.prepared.original_price,
        original_disk_threshold: stepState.prepared.original_disk_threshold,
      });
    } catch (err) {
      console.error(`[smoke] cleanup failed: ${err.message}`);
    }
  }
}

process.on('SIGINT', async () => {
  await cleanup();
  process.exit(130);
});
process.on('SIGTERM', async () => {
  await cleanup();
  process.exit(143);
});

async function main() {
  log('preparing dev embed files and Go cache');
  await ensureDevEmbedFiles();
  await ensureGoCacheDirs();
  await fs.access(config.mockClientPath);

  const suffix = Date.now().toString(36);
  log('seeding smoke user/token/tool price');
  const prepared = await runSmokeHelper('prepare', {
    suffix,
    price_per_call: config.pricePerCall,
    initial_quota: config.initialQuota,
  });
  stepState.prepared = prepared;

  const port = await findFreePort();
  const baseUrl = `http://127.0.0.1:${port}`;
  const requestId = `mcp-bridge-smoke-${prepared.suffix}`;
  const clientId = `qidian-smoke-${prepared.suffix}`;
  const dashboardHeaders = {
    Authorization: `Bearer ${prepared.access_token}`,
    'New-Api-User': String(prepared.user_id),
  };

  log('starting new-api', { baseUrl });
  spawnLogged('new-api', 'go', ['run', '.'], {
    cwd: repoRoot,
    env: makeEnv({
      PORT: String(port),
      GIN_MODE: 'release',
      MCP_REMOTE_BRIDGE_TIMEOUT_MS: String(config.remoteBridgeTimeoutMs),
    }),
  });
  await waitForHTTP(`${baseUrl}/api/setup`, config.timeoutMs);

  const clientNotFound = await verifyMCPToolFailure(baseUrl, prepared.api_token, dashboardHeaders, {
    requestId: `${requestId}-client-not-found`,
    arguments: { file_path: config.filePath, offset: 1, limit: 1 },
    expectedMCPErrorCode: MCP_ERROR_BRIDGE_UNAVAILABLE,
    expectedCallStatus: 'error',
    expectedExecutorErrorCode: 'BRIDGE_CLIENT_NOT_FOUND',
    expectedQuota: prepared.expected_quota,
    expectAudit: false,
  });

  log('starting QidianBrowser mock bridge client', { clientId, workspace: config.workspace });
  spawnLogged(
    'qidian-mock',
    'node',
    [
      config.mockClientPath,
      `--server=ws://127.0.0.1:${port}/bridge/ws`,
      `--token=${prepared.api_token}`,
      `--workspace=${config.workspace}`,
      `--client-id=${clientId}`,
      '--name=Qidian Browser Smoke Mock',
      '--ping-interval-ms=5000',
    ],
    { cwd: qidianBrowserRoot },
  );
  const bridgeClient = await waitForBridgeClient(baseUrl, dashboardHeaders, clientId, config.timeoutMs);
  for (const capability of ['remote_read', 'remote_tree', 'remote_glob', 'remote_grep', 'remote_env_info']) {
    if (!bridgeClient.capabilities?.includes(capability)) {
      throw new Error(`bridge client did not advertise ${capability}: ${JSON.stringify(bridgeClient.capabilities)}`);
    }
  }

  log('calling MCP remote_read', { requestId, file: config.filePath });
  const mcpResponse = await postMCP(baseUrl, prepared.api_token, {
    jsonrpc: '2.0',
    id: requestId,
    method: 'tools/call',
    params: {
      name: 'remote_read',
      arguments: {
        file_path: config.filePath,
        offset: 1,
        limit: 5,
      },
    },
  });
  if (mcpResponse.error) {
    throw new Error(`MCP returned error: ${JSON.stringify(mcpResponse.error)}`);
  }
  const text = mcpResponse?.result?.content?.[0]?.text || '';
  if (!text.trim()) {
    throw new Error(`MCP result did not include text content: ${JSON.stringify(mcpResponse)}`);
  }

  const callJson = await apiGet(
    `${baseUrl}/api/mcp/tool-calls?scope=all&page_size=10&request_id=${encodeURIComponent(requestId)}`,
    dashboardHeaders,
  );
  const call = firstPageItem(callJson, 'MCP tool-calls');
  if (call.status !== 'success') {
    throw new Error(`MCP call status mismatch: ${JSON.stringify(call)}`);
  }
  if (call.target_client !== clientId) {
    throw new Error(`target_client mismatch, got ${call.target_client}, want ${clientId}`);
  }
  if (call.quota !== prepared.expected_quota) {
    throw new Error(`quota mismatch, got ${call.quota}, want ${prepared.expected_quota}`);
  }
  if (!call.settled_at || call.settled_at <= 0) {
    throw new Error(`settled_at was not set: ${JSON.stringify(call)}`);
  }

  const auditJson = await apiGet(
    `${baseUrl}/api/bridge/audit-logs?scope=all&page_size=10&request_id=${encodeURIComponent(requestId)}`,
    dashboardHeaders,
  );
  const audit = firstPageItem(auditJson, 'bridge audit-logs');
  if (audit.status !== 'success') {
    throw new Error(`bridge audit status mismatch: ${JSON.stringify(audit)}`);
  }
  if (audit.client_id !== clientId) {
    throw new Error(`bridge audit client mismatch, got ${audit.client_id}, want ${clientId}`);
  }

  const inspected = await runSmokeHelper('inspect', {
    user_id: prepared.user_id,
    token_id: prepared.token_id,
    call_id: call.id,
  });
  const expectedQuotaAfter = prepared.initial_user_quota - prepared.expected_quota;
  const expectedTokenAfter = prepared.initial_token_quota - prepared.expected_quota;
  if (inspected.user_quota !== expectedQuotaAfter) {
    throw new Error(`user quota mismatch: got ${inspected.user_quota}, want ${expectedQuotaAfter}`);
  }
  if (inspected.token_remain_quota !== expectedTokenAfter) {
    throw new Error(`token quota mismatch: got ${inspected.token_remain_quota}, want ${expectedTokenAfter}`);
  }
  if (inspected.user_used_quota !== prepared.expected_quota || inspected.token_used_quota !== prepared.expected_quota) {
    throw new Error(`used quota mismatch: ${JSON.stringify(inspected)}`);
  }

  const smokeFileDir = path.dirname(config.filePath) === '.' ? '.' : path.dirname(config.filePath);
  const smokeFileName = path.basename(config.filePath);
  const extraCalls = [
    {
      name: 'remote_tree',
      requestId: `${requestId}-tree`,
      arguments: { path: smokeFileDir, depth: 2, max_results: 80 },
    },
    {
      name: 'remote_glob',
      requestId: `${requestId}-glob`,
      arguments: { path: smokeFileDir, pattern: smokeFileName, max_results: 50 },
    },
    {
      name: 'remote_grep',
      requestId: `${requestId}-grep`,
      arguments: { path: config.filePath, pattern: '.', max_results: 30 },
    },
    {
      name: 'remote_env_info',
      requestId: `${requestId}-env`,
      arguments: {},
    },
  ];
  const extraResults = [];
  for (const extra of extraCalls) {
    extraResults.push(await callMCPToolAndVerifySuccess(
      baseUrl,
      prepared.api_token,
      dashboardHeaders,
      clientId,
      extra.requestId,
      extra.name,
      extra.arguments,
    ));
  }

  const toolError = await verifyMCPToolFailure(baseUrl, prepared.api_token, dashboardHeaders, {
    requestId: `${requestId}-tool-error`,
    arguments: {
      file_path: config.filePath,
      mock_error_code: 'REMOTE_PERMISSION_DENIED',
      mock_error_message: 'mock bridge denied remote_read',
    },
    expectedMCPErrorCode: MCP_ERROR_EXECUTOR_FAILED,
    expectedCallStatus: 'error',
    expectedExecutorErrorCode: 'REMOTE_PERMISSION_DENIED',
    expectedAuditStatus: 'error',
    expectedAuditErrorCode: 'REMOTE_PERMISSION_DENIED',
    expectedQuota: prepared.expected_quota,
    clientId,
  });

  const timeoutDelayMs = config.remoteBridgeTimeoutMs + 500;
  const timeoutFailure = await verifyMCPToolFailure(baseUrl, prepared.api_token, dashboardHeaders, {
    requestId: `${requestId}-timeout`,
    arguments: {
      file_path: config.filePath,
      mock_delay_ms: timeoutDelayMs,
    },
    expectedMCPErrorCode: MCP_ERROR_EXECUTOR_TIMEOUT,
    expectedCallStatus: 'timeout',
    expectedExecutorErrorCode: 'EXECUTOR_TIMEOUT',
    expectedAuditStatus: 'timeout',
    expectedAuditErrorCode: 'EXECUTOR_TIMEOUT',
    expectedQuota: prepared.expected_quota,
    clientId,
  });

  log('smoke passed', {
    request_id: requestId,
    client_id: bridgeClient.client_id,
    session_id: bridgeClient.session_id,
    call_id: call.id,
    audit_id: audit.id,
    charged_quota: call.quota,
    user_quota: `${prepared.initial_user_quota} -> ${inspected.user_quota}`,
    token_quota: `${prepared.initial_token_quota} -> ${inspected.token_remain_quota}`,
    extra_mock_tools: extraResults.map((item) => item.call.tool_name),
    failure_scenarios: [
      clientNotFound.call.error_code,
      toolError.call.error_code,
      timeoutFailure.call.error_code,
    ],
    result_preview: text.split(/\r?\n/).slice(0, 2).join(' / '),
  });
}

try {
  await main();
  await cleanup();
} catch (err) {
  console.error(`[smoke] failed: ${err.stack || err.message}`);
  await cleanup();
  process.exit(1);
}
