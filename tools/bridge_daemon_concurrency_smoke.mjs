#!/usr/bin/env node

import fs from 'fs/promises';
import http from 'http';
import net from 'net';
import path from 'path';
import { spawn } from 'child_process';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '..');
const daemonPath = path.join(__dirname, 'bridge_client_daemon.mjs');

const DEFAULT_DSN =
  'root:my-secret-pw@tcp(127.0.0.1:3306)/data-proxy?charset=utf8mb4&parseTime=true&loc=Local';
const DEFAULT_TIMEOUT_MS = 240_000;
const DEFAULT_CONCURRENCY = 12;
const DEFAULT_ITERATIONS = 4;
const GO_CACHE_ROOT = '/Volumes/fushilu/.caches/gocache';

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
  console.log(`Bridge daemon concurrency smoke

Usage:
  node tools/bridge_daemon_concurrency_smoke.mjs [options]

Options:
  --dsn=<dsn>             SQL_DSN for local data-proxy. Default local Docker MySQL.
  --port=<port>           new-api port. Defaults to a free local port.
  --base-url=<url>        Use an already running data-proxy instead of starting one.
  --concurrency=<n>       Concurrent MCP requests. Default ${DEFAULT_CONCURRENCY}
  --iterations=<n>        Batches per tool family. Default ${DEFAULT_ITERATIONS}
  --timeout=<ms>          Overall wait timeout. Default ${DEFAULT_TIMEOUT_MS}
  --keep-data             Keep smoke DB rows and temporary workspace.
  --help                  Show help.

The smoke starts a real local bridge daemon, enables write tools against a
temporary workspace, starts a loopback MCP HTTP server, configures a
qidian_browser MCP Proxy server, then concurrently calls remote read/write/tree
glob/grep/edit and proxied MCP tools through /mcp/v1.
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
  baseUrl: args['base-url'] || process.env.BRIDGE_DAEMON_SMOKE_BASE_URL || '',
  concurrency: numberArg(args.concurrency || process.env.BRIDGE_DAEMON_SMOKE_CONCURRENCY, DEFAULT_CONCURRENCY),
  iterations: numberArg(args.iterations || process.env.BRIDGE_DAEMON_SMOKE_ITERATIONS, DEFAULT_ITERATIONS),
  timeoutMs: numberArg(args.timeout || process.env.BRIDGE_DAEMON_SMOKE_TIMEOUT_MS, DEFAULT_TIMEOUT_MS),
  keepData: args['keep-data'] === true || process.env.BRIDGE_DAEMON_SMOKE_KEEP_DATA === '1',
};

const children = [];
let prepared = null;
let workspace = '';
let localMCP = null;
let cleaned = false;

function numberArg(value, fallback) {
  const parsed = Number(value || fallback);
  if (!Number.isFinite(parsed) || parsed <= 0) return fallback;
  return Math.floor(parsed);
}

function log(message, detail) {
  if (detail === undefined) {
    console.log(`[bridge-smoke] ${message}`);
  } else {
    console.log(`[bridge-smoke] ${message}`, detail);
  }
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
    SESSION_SECRET: process.env.SESSION_SECRET || 'bridge-daemon-smoke-session-secret',
    NODE_TYPE: process.env.NODE_TYPE || 'master',
    MEMORY_CACHE_ENABLED: 'false',
    REDIS_CONN_STRING: '',
    BATCH_UPDATE_ENABLED: 'false',
    CHANNEL_UPDATE_FREQUENCY: '',
    ...extra,
  };
}

async function ensureDevEmbedFiles() {
  const html = '<!doctype html><html><head><title>bridge daemon smoke</title></head><body>bridge daemon smoke</body></html>\n';
  for (const rel of ['web/default/dist/index.html', 'web/classic/dist/index.html']) {
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
  return await new Promise((resolve, reject) => {
    const server = net.createServer();
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      server.close(() => resolve(address.port));
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
    if (!options.quietExit) log(`${name} exited`, { code, signal });
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
  const helperPath = path.join(repoRoot, '.test', 'bridge_daemon_smoke_helper.go');
  await fs.writeFile(helperPath, helperSource(), 'utf8');
  const env = makeEnv({
    BRIDGE_DAEMON_SMOKE_ACTION: action,
    BRIDGE_DAEMON_SMOKE_PAYLOAD: JSON.stringify(payload),
  });
  try {
    const result = await runCommand('go', ['run', './.test/bridge_daemon_smoke_helper.go'], {
      cwd: repoRoot,
      env,
      prefix: `go:${action}`,
    });
    const output = `${result.stdout}\n${result.stderr}`;
    const line = output.split(/\r?\n/).find((item) => item.startsWith('SMOKE_JSON:'));
    if (!line) throw new Error(`helper did not return SMOKE_JSON for action ${action}`);
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
	Suffix string ` + "`json:\"suffix\"`" + `
	InitialQuota int ` + "`json:\"initial_quota\"`" + `
}

type cleanupPayload struct {
	UserId int ` + "`json:\"user_id\"`" + `
	TokenId int ` + "`json:\"token_id\"`" + `
	Suffix string ` + "`json:\"suffix\"`" + `
	Namespace string ` + "`json:\"namespace\"`" + `
}

func main() {
	action := os.Getenv("BRIDGE_DAEMON_SMOKE_ACTION")
	payload := []byte(os.Getenv("BRIDGE_DAEMON_SMOKE_PAYLOAD"))
	if action == "" {
		panic("BRIDGE_DAEMON_SMOKE_ACTION is required")
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
	defer func() { _ = model.CloseDB() }()

	var result any
	var err error
	switch action {
	case "prepare":
		var p preparePayload
		mustUnmarshal(payload, &p)
		result, err = prepare(p)
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
		p.InitialQuota = 1000000
	}
	if err := model.SeedBuiltinMCPTools(); err != nil {
		return nil, err
	}
	remoteTools := []string{
		"remote_read",
		"remote_write",
		"remote_edit",
		"remote_tree",
		"remote_glob",
		"remote_grep",
		"remote_env_info",
	}
	for _, name := range remoteTools {
		tool, err := model.GetMCPToolByName(name)
		if err != nil {
			return nil, err
		}
		if _, err := model.UpdateMCPToolFields(tool.Id, map[string]any{
			"price_per_call": 0,
			"price_unit": model.MCPToolPriceUnitPerCall,
			"is_remote": true,
			"status": model.MCPToolStatusEnabled,
		}); err != nil {
			return nil, err
		}
	}
	if err := model.UpdateOption("performance_setting.monitor_disk_threshold", "100"); err != nil {
		return nil, err
	}

	accessToken := "bdsmoke" + p.Suffix + "access000000000000"
	if len(accessToken) > 32 {
		accessToken = accessToken[:32]
	}
	hashedPassword, err := common.Password2Hash("bridge-daemon-smoke-password")
	if err != nil {
		return nil, err
	}
	user := model.User{
		Username: "bdsmoke" + p.Suffix,
		Password: hashedPassword,
		DisplayName: "Bridge Daemon Smoke",
		Role: common.RoleRootUser,
		Status: common.UserStatusEnabled,
		Quota: p.InitialQuota,
		Group: "default",
		AffCode: common.GetRandomString(8),
	}
	user.SetAccessToken(accessToken)
	if err := model.DB.Create(&user).Error; err != nil {
		return nil, err
	}

	tokenKey := strings.ReplaceAll("bdsmoke" + p.Suffix, "-", "")
	token := model.Token{
		UserId: user.Id,
		Key: tokenKey,
		Status: common.TokenStatusEnabled,
		Name: "Bridge Daemon Smoke Token",
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
		"initial_quota": p.InitialQuota,
	}, nil
}

func cleanup(p cleanupPayload) (map[string]any, error) {
	if p.Suffix != "" {
		pattern := "%bridge-daemon-smoke-" + p.Suffix + "%"
		_ = model.DB.Where("request_id LIKE ?", pattern).Delete(&model.BillingEvent{}).Error
		_ = model.DB.Where("request_id LIKE ?", pattern).Delete(&model.BridgeAuditLog{}).Error
		_ = model.DB.Where("request_id LIKE ?", pattern).Delete(&model.MCPToolCall{}).Error
		_ = model.DB.Where("client_id = ?", "bridge-daemon-" + p.Suffix).Delete(&model.BridgeSession{}).Error
		_ = model.DB.Unscoped().Where("client_id = ?", "bridge-daemon-" + p.Suffix).Delete(&model.BridgeClient{}).Error
	}
	if p.Namespace != "" {
		var server model.MCPProxyServer
		if err := model.DB.Unscoped().Where("namespace = ?", p.Namespace).First(&server).Error; err == nil {
			_ = model.DB.Unscoped().Where("proxy_server_id = ?", server.Id).Delete(&model.MCPProxyTool{}).Error
			_ = model.DB.Unscoped().Where("proxy_server_id = ?", server.Id).Delete(&model.MCPProxyDiscoveryEvent{}).Error
			_ = model.DB.Unscoped().Delete(&server).Error
		}
		_ = model.DB.Unscoped().Where("name LIKE ?", p.Namespace + ".%").Delete(&model.MCPTool{}).Error
	}
	if p.TokenId > 0 {
		_ = model.DB.Unscoped().Where("id = ?", p.TokenId).Delete(&model.Token{}).Error
	}
	if p.UserId > 0 {
		_ = model.DB.Unscoped().Where("id = ?", p.UserId).Delete(&model.User{}).Error
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

async function apiGet(url, headers) {
  const response = await fetch(url, { headers });
  const json = await response.json().catch(() => ({}));
  if (!response.ok || json.success !== true) {
    throw new Error(`GET ${url} failed: HTTP ${response.status} ${JSON.stringify(json)}`);
  }
  return json;
}

async function apiSend(method, url, headers, body) {
  const response = await fetch(url, {
    method,
    headers: { ...headers, 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const json = await response.json().catch(() => ({}));
  if (!response.ok || json.success !== true) {
    throw new Error(`${method} ${url} failed: HTTP ${response.status} ${JSON.stringify(json)}`);
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

async function waitForBridgeClient(baseUrl, headers, clientId, timeoutMs) {
  const started = Date.now();
  let last = '';
  while (Date.now() - started < timeoutMs) {
    try {
      const json = await apiGet(`${baseUrl}/api/bridge/clients?scope=all&page_size=50`, headers);
      const found = pageItems(json).find((item) => item.client_id === clientId && item.online === true);
      if (found) return found;
      last = `clients=${pageItems(json).map((item) => `${item.client_id}:${item.online}`).join(',')}`;
    } catch (err) {
      last = err.message;
    }
    await sleep(500);
  }
  throw new Error(`timeout waiting for bridge client ${clientId}; ${last}`);
}

async function waitForBridgeClientReconnect(baseUrl, headers, clientId, previousSessionId, timeoutMs) {
  const started = Date.now();
  let last = '';
  while (Date.now() - started < timeoutMs) {
    try {
      const json = await apiGet(`${baseUrl}/api/bridge/clients?scope=all&page_size=50`, headers);
      const found = pageItems(json).find((item) => (
        item.client_id === clientId
        && item.online === true
        && item.session_id
        && item.session_id !== previousSessionId
      ));
      if (found) return found;
      last = `clients=${pageItems(json).map((item) => `${item.client_id}:${item.online}:${item.session_id || '-'}`).join(',')}`;
    } catch (err) {
      last = err.message;
    }
    await sleep(500);
  }
  throw new Error(`timeout waiting for bridge client ${clientId} to reconnect; ${last}`);
}

function startLocalMCPServer() {
  const sessions = new Set();
  const server = http.createServer(async (req, res) => {
    if (req.method !== 'POST' || req.url !== '/mcp') {
      res.writeHead(404);
      res.end('not found');
      return;
    }
    const raw = await readBody(req);
    let rpc;
    try {
      rpc = JSON.parse(raw || '{}');
    } catch {
      writeJSON(res, 400, { jsonrpc: '2.0', error: { code: -32700, message: 'Parse error' }, id: null });
      return;
    }
    const session = req.headers['mcp-session-id'] || `smoke-session-${Date.now()}`;
    sessions.add(session);
    res.setHeader('Mcp-Session-Id', session);
    if (!rpc.id && rpc.method === 'notifications/initialized') {
      res.writeHead(204);
      res.end();
      return;
    }
    if (rpc.method === 'initialize') {
      writeJSON(res, 200, {
        jsonrpc: '2.0',
        id: rpc.id,
        result: {
          protocolVersion: '2025-06-18',
          capabilities: { tools: { listChanged: false } },
          serverInfo: { name: 'bridge-daemon-smoke-mcp', version: '0.1.0' },
        },
      });
      return;
    }
    if (rpc.method === 'ping') {
      writeJSON(res, 200, { jsonrpc: '2.0', id: rpc.id, result: {} });
      return;
    }
    if (rpc.method === 'tools/list') {
      writeJSON(res, 200, {
        jsonrpc: '2.0',
        id: rpc.id,
        result: {
          tools: [
            {
              name: 'echo',
              description: 'Echo a message through the local MCP server.',
              inputSchema: {
                type: 'object',
                required: ['message'],
                properties: { message: { type: 'string' } },
              },
            },
            {
              name: 'sum',
              description: 'Sum numeric values through the local MCP server.',
              inputSchema: {
                type: 'object',
                required: ['values'],
                properties: { values: { type: 'array', items: { type: 'number' } } },
              },
            },
            {
              name: 'fail',
              description: 'Return a deterministic JSON-RPC error for Bridge proxy failure verification.',
              inputSchema: {
                type: 'object',
                properties: {
                  message: { type: 'string' },
                },
              },
            },
          ],
        },
      });
      return;
    }
    if (rpc.method === 'tools/call') {
      const name = rpc.params?.name;
      const params = rpc.params?.arguments || {};
      if (name === 'echo') {
        const message = String(params.message || '');
        writeJSON(res, 200, {
          jsonrpc: '2.0',
          id: rpc.id,
          result: {
            content: [{ type: 'text', text: `echo:${message}` }],
            metadata: { echo: message, source: 'local-mcp' },
          },
        });
        return;
      }
      if (name === 'sum') {
        const values = Array.isArray(params.values) ? params.values.map(Number) : [];
        const sum = values.reduce((acc, value) => acc + value, 0);
        writeJSON(res, 200, {
          jsonrpc: '2.0',
          id: rpc.id,
          result: {
            content: [{ type: 'text', text: String(sum) }],
            metadata: { sum, count: values.length, source: 'local-mcp' },
          },
        });
        return;
      }
      if (name === 'fail') {
        writeJSON(res, 200, {
          jsonrpc: '2.0',
          id: rpc.id,
          error: {
            code: -32099,
            message: String(params.message || 'smoke downstream failure'),
          },
        });
        return;
      }
    }
    writeJSON(res, 200, {
      jsonrpc: '2.0',
      id: rpc.id,
      error: { code: -32601, message: `Method not found: ${rpc.method}` },
    });
  });
  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const port = server.address().port;
      resolve({
        url: `http://127.0.0.1:${port}/mcp`,
        close: () => new Promise((done) => server.close(done)),
      });
    });
  });
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    let body = '';
    req.setEncoding('utf8');
    req.on('data', (chunk) => {
      body += chunk;
    });
    req.on('end', () => resolve(body));
    req.on('error', reject);
  });
}

function writeJSON(res, status, value) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(value));
}

async function setupWorkspace(suffix) {
  const root = path.join(repoRoot, '.test', `bridge-daemon-workspace-${suffix}`);
  await fs.rm(root, { recursive: true, force: true });
  await fs.mkdir(path.join(root, 'docs'), { recursive: true });
  await fs.writeFile(
    path.join(root, 'docs', 'seed.txt'),
    [
      'alpha bridge daemon smoke',
      'beta data-proxy concurrency',
      'gamma local mcp proxy',
      '',
    ].join('\n'),
    'utf8',
  );
  const editFixtures = Math.max(config.iterations * concurrentCallFamilies('fixture').length, 32);
  await Promise.all(Array.from({ length: editFixtures }, (_, index) => fs.writeFile(
    path.join(root, 'docs', `edit-${index}.txt`),
    `pending-${index}\nbridge daemon edit fixture\n`,
    'utf8',
  )));
  return root;
}

async function configureProxy(baseUrl, headers, clientId, mcpUrl, namespace) {
  const create = await apiSend('POST', `${baseUrl}/api/mcp/proxy/servers`, headers, {
    name: 'Bridge Daemon Smoke MCP',
    namespace,
    transport: 'qidian_browser',
    endpoint: `bridge://${clientId}?target=${encodeURIComponent(mcpUrl)}`,
    auth_type: 'none',
    timeout_ms: 10000,
    max_result_size: 1048576,
    max_metadata_size: 65536,
    visibility: 'admin',
    status: 'enabled',
  });
  const server = create.data;
  await apiSend('POST', `${baseUrl}/api/mcp/proxy/servers/${server.id}/test`, headers);
  const discovered = await apiSend('POST', `${baseUrl}/api/mcp/proxy/servers/${server.id}/discover`, headers);
  const tools = discovered.data?.tools || [];
  if (tools.length < 1) throw new Error(`MCP proxy discovery returned no tools: ${JSON.stringify(discovered)}`);
  for (const tool of tools) {
    await apiSend('PATCH', `${baseUrl}/api/mcp/proxy/tools/${tool.id}`, headers, {
      status: 'enabled',
      price_per_call: 0,
      free_quota: 0,
    });
  }
  return { server, tools };
}

function concurrentCallFamilies(namespace) {
  return [
    (i) => ({
      name: 'remote_write',
      arguments: {
        file_path: `out/write-${i}.txt`,
        content: `daemon write ${i}\nalpha ${i}\n`,
        create_dirs: true,
      },
    }),
    (i) => ({
      name: 'remote_edit',
      arguments: {
        file_path: `docs/edit-${i}.txt`,
        old_string: `pending-${i}`,
        new_string: `edited-${i}`,
      },
    }),
    () => ({
      name: 'remote_read',
      arguments: { file_path: 'docs/seed.txt', offset: 1, limit: 10 },
    }),
    () => ({
      name: 'remote_glob',
      arguments: { path: '.', pattern: 'docs/*.txt', max_results: 100 },
    }),
    () => ({
      name: 'remote_grep',
      arguments: { path: '.', pattern: 'alpha|concurrency', max_results: 20 },
    }),
    () => ({
      name: 'remote_tree',
      arguments: { path: '.', depth: 4, max_results: 100 },
    }),
    (i) => ({
      name: `${namespace}.echo`,
      arguments: { message: `proxy-${i}` },
    }),
  ];
}

function validateConcurrentResult(call, text) {
  if (call.name === 'remote_edit' && !text.includes(`edited ${call.arguments.file_path}`)) {
    throw new Error(`${call.name} ${call.requestId} did not report edited file: ${text}`);
  }
  if (call.name === 'remote_glob' && !text.includes('docs/seed.txt')) {
    throw new Error(`${call.name} ${call.requestId} did not include docs/seed.txt: ${text}`);
  }
}

function buildConcurrentCalls(baseUrl, apiToken, suffix, namespace) {
  const calls = [];
  const families = concurrentCallFamilies(namespace);
  let index = 0;
  for (let iteration = 0; iteration < config.iterations; iteration += 1) {
    for (const family of families) {
      const requestId = `bridge-daemon-smoke-${suffix}-${String(index).padStart(4, '0')}`;
      calls.push({ requestId, ...family(index) });
      index += 1;
    }
  }
  return calls.map((call) => async () => {
    const response = await postMCP(baseUrl, apiToken, {
      jsonrpc: '2.0',
      id: call.requestId,
      method: 'tools/call',
      params: {
        name: call.name,
        arguments: call.arguments,
      },
    });
    if (response.error) {
      throw new Error(`${call.name} ${call.requestId} returned ${JSON.stringify(response.error)}`);
    }
    const text = response?.result?.content?.[0]?.text || '';
    if (!text.trim()) {
      throw new Error(`${call.name} ${call.requestId} returned empty content`);
    }
    validateConcurrentResult(call, text);
    return { ...call, text };
  });
}

function bridgeWSURL(baseUrl) {
  const parsed = new URL(baseUrl);
  parsed.protocol = parsed.protocol === 'https:' ? 'wss:' : 'ws:';
  parsed.pathname = '/bridge/ws';
  parsed.search = '';
  parsed.hash = '';
  return parsed.toString();
}

async function runPool(tasks, concurrency) {
  const results = [];
  const errors = [];
  let next = 0;
  async function worker() {
    for (;;) {
      const index = next;
      next += 1;
      if (index >= tasks.length) return;
      try {
        results[index] = await tasks[index]();
      } catch (err) {
        errors.push(err);
      }
    }
  }
  await Promise.all(Array.from({ length: Math.min(concurrency, tasks.length) }, () => worker()));
  if (errors.length > 0) {
    throw new Error(`${errors.length} concurrent calls failed:\n${errors.map((err) => err.stack || err.message).join('\n')}`);
  }
  return results;
}

function expectedBridgeAuditToolName(toolName, namespace) {
  if (toolName.startsWith(`${namespace}.`)) {
    return 'mcp_proxy.tools_call';
  }
  return toolName;
}

async function getSingleItemByRequestId(baseUrl, headers, apiPath, requestId) {
  const json = await apiGet(
    `${baseUrl}${apiPath}?scope=all&page_size=10&request_id=${encodeURIComponent(requestId)}`,
    headers,
  );
  const items = pageItems(json);
  if (items.length < 1) {
    throw new Error(`no record found for ${apiPath} request_id=${requestId}: ${JSON.stringify(json)}`);
  }
  return items[0];
}

async function assertMCPCallRecord(baseUrl, headers, requestId, expected) {
  const call = await getSingleItemByRequestId(baseUrl, headers, '/api/mcp/tool-calls', requestId);
  if (call.status !== expected.status) {
    throw new Error(`MCP call ${requestId} status mismatch: got ${call.status}, expected ${expected.status}`);
  }
  if (expected.toolName && call.tool_name !== expected.toolName) {
    throw new Error(`MCP call ${requestId} tool mismatch: got ${call.tool_name}, expected ${expected.toolName}`);
  }
  if (expected.errorCode && call.error_code !== expected.errorCode) {
    throw new Error(`MCP call ${requestId} error_code mismatch: got ${call.error_code}, expected ${expected.errorCode}`);
  }
  if (expected.clientId && call.target_client !== expected.clientId) {
    throw new Error(`MCP call ${requestId} target_client mismatch: got ${call.target_client}, expected ${expected.clientId}`);
  }
  if (expected.clientId && !call.bridge_session_id) {
    throw new Error(`MCP call ${requestId} did not persist bridge_session_id`);
  }
  return call;
}

async function assertBridgeAuditRecord(baseUrl, headers, requestId, expected) {
  const audit = await getSingleItemByRequestId(baseUrl, headers, '/api/bridge/audit-logs', requestId);
  if (audit.status !== expected.status) {
    throw new Error(`bridge audit ${requestId} status mismatch: got ${audit.status}, expected ${expected.status}`);
  }
  if (expected.toolName && audit.tool_name !== expected.toolName) {
    throw new Error(`bridge audit ${requestId} tool mismatch: got ${audit.tool_name}, expected ${expected.toolName}`);
  }
  if (expected.errorCode && audit.error_code !== expected.errorCode) {
    throw new Error(`bridge audit ${requestId} error_code mismatch: got ${audit.error_code}, expected ${expected.errorCode}`);
  }
  if (expected.clientId && audit.client_id !== expected.clientId) {
    throw new Error(`bridge audit ${requestId} client_id mismatch: got ${audit.client_id}, expected ${expected.clientId}`);
  }
  return audit;
}

async function assertConcurrentRecords(baseUrl, headers, results, namespace, clientId) {
  for (const result of results) {
    await assertMCPCallRecord(baseUrl, headers, result.requestId, {
      status: 'success',
      toolName: result.name,
      clientId,
    });
    await assertBridgeAuditRecord(baseUrl, headers, result.requestId, {
      status: 'success',
      toolName: expectedBridgeAuditToolName(result.name, namespace),
      clientId,
    });
  }
}

async function expectMCPError(baseUrl, apiToken, request, expectedCode) {
  const response = await postMCP(baseUrl, apiToken, request);
  if (!response.error) {
    throw new Error(`expected MCP error for ${request.id}, got ${JSON.stringify(response)}`);
  }
  if (expectedCode !== undefined && response.error.code !== expectedCode) {
    throw new Error(`MCP error code mismatch for ${request.id}: got ${response.error.code}, expected ${expectedCode}`);
  }
  return response.error;
}

async function assertFailureScenarios(baseUrl, apiToken, headers, suffix, namespace, clientId) {
  const failures = [];

  const forbiddenWriteId = `bridge-daemon-smoke-${suffix}-forbidden-write`;
  await expectMCPError(baseUrl, apiToken, {
    jsonrpc: '2.0',
    id: forbiddenWriteId,
    method: 'tools/call',
    params: {
      name: 'remote_write',
      arguments: {
        file_path: '../outside-workspace.txt',
        content: 'this write must be rejected\n',
      },
    },
  }, -32003);
  await assertMCPCallRecord(baseUrl, headers, forbiddenWriteId, {
    status: 'error',
    toolName: 'remote_write',
    errorCode: 'REMOTE_WRITE_FORBIDDEN',
    clientId,
  });
  await assertBridgeAuditRecord(baseUrl, headers, forbiddenWriteId, {
    status: 'error',
    toolName: 'remote_write',
    errorCode: 'REMOTE_WRITE_FORBIDDEN',
    clientId,
  });
  failures.push({ requestId: forbiddenWriteId, toolName: 'remote_write' });

  const proxyFailureId = `bridge-daemon-smoke-${suffix}-proxy-failure`;
  const proxyErrorCode = 'MCP_PROXY_UPSTREAM_-32099';
  await expectMCPError(baseUrl, apiToken, {
    jsonrpc: '2.0',
    id: proxyFailureId,
    method: 'tools/call',
    params: {
      name: `${namespace}.fail`,
      arguments: {
        message: 'intentional downstream MCP failure',
      },
    },
  }, -32003);
  await assertMCPCallRecord(baseUrl, headers, proxyFailureId, {
    status: 'error',
    toolName: `${namespace}.fail`,
    errorCode: proxyErrorCode,
    clientId,
  });
  await assertBridgeAuditRecord(baseUrl, headers, proxyFailureId, {
    status: 'error',
    toolName: 'mcp_proxy.tools_call',
    errorCode: proxyErrorCode,
    clientId,
  });
  failures.push({ requestId: proxyFailureId, toolName: `${namespace}.fail` });

  return failures;
}

async function assertReconnectScenario(baseUrl, apiToken, headers, suffix, clientId, currentSessionId) {
  if (!currentSessionId) {
    throw new Error('bridge client did not expose session_id for reconnect verification');
  }
  log('verifying bridge daemon reconnect', { clientId, previous_session_id: currentSessionId });
  await apiSend('POST', `${baseUrl}/api/bridge/sessions/${encodeURIComponent(currentSessionId)}/close`, headers, {
    reason: 'bridge daemon reconnect smoke',
  });
  const reconnected = await waitForBridgeClientReconnect(baseUrl, headers, clientId, currentSessionId, config.timeoutMs);
  const requestId = `bridge-daemon-smoke-${suffix}-reconnect-env`;
  const response = await postMCP(baseUrl, apiToken, {
    jsonrpc: '2.0',
    id: requestId,
    method: 'tools/call',
    params: {
      name: 'remote_env_info',
      arguments: {},
    },
  });
  if (response.error) {
    throw new Error(`remote_env_info after reconnect returned ${JSON.stringify(response.error)}`);
  }
  await assertMCPCallRecord(baseUrl, headers, requestId, {
    status: 'success',
    toolName: 'remote_env_info',
    clientId,
  });
  await assertBridgeAuditRecord(baseUrl, headers, requestId, {
    status: 'success',
    toolName: 'remote_env_info',
    clientId,
  });
  return {
    requestId,
    toolName: 'remote_env_info',
    previousSessionId: currentSessionId,
    sessionId: reconnected.session_id,
  };
}

async function readLocalAuditEvents(auditLogPath) {
  let raw = '';
  try {
    raw = await fs.readFile(auditLogPath, 'utf8');
  } catch {
    return [];
  }
  return raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => JSON.parse(line));
}

async function waitForLocalAuditEvents(auditLogPath, expectations, timeoutMs) {
  const started = Date.now();
  let lastMissing = [];
  while (Date.now() - started < timeoutMs) {
    const events = await readLocalAuditEvents(auditLogPath);
    lastMissing = expectations.filter((expectation) => {
      const hasToolCall = events.some((event) => event.type === 'tool_call' && event.request_id === expectation.requestId);
      const hasFinal = events.some((event) => event.type === expectation.finalType && event.request_id === expectation.requestId);
      return !hasToolCall || !hasFinal;
    });
    if (events.some((event) => event.type === 'registered') && lastMissing.length === 0) {
      return events;
    }
    await sleep(200);
  }
  throw new Error(`local bridge daemon audit log missing events: ${JSON.stringify(lastMissing)}`);
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
      // already gone
    }
  }
}

async function cleanup() {
  if (cleaned) return;
  cleaned = true;
  if (localMCP) {
    await localMCP.close().catch(() => {});
  }
  await stopChildren();
  if (prepared && !config.keepData) {
    await runSmokeHelper('cleanup', {
      user_id: prepared.user_id,
      token_id: prepared.token_id,
      suffix: prepared.suffix,
      namespace: prepared.namespace,
    }).catch((err) => {
      console.error(`[bridge-smoke] cleanup failed: ${err.message}`);
    });
  }
  if (workspace && !config.keepData) {
    await fs.rm(workspace, { recursive: true, force: true }).catch(() => {});
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
  await ensureDevEmbedFiles();
  await ensureGoCacheDirs();
  await fs.access(daemonPath);

  const suffix = Date.now().toString(36).replace(/[^a-z0-9]/g, '');
  const namespace = `daemon_${suffix}`;
  prepared = await runSmokeHelper('prepare', { suffix, initial_quota: 1000000 });
  prepared.namespace = namespace;

  workspace = await setupWorkspace(suffix);
  localMCP = await startLocalMCPServer();
  log('local MCP server started', { url: localMCP.url });

  const port = config.baseUrl ? 0 : (config.port > 0 ? config.port : await findFreePort());
  const baseUrl = config.baseUrl || `http://127.0.0.1:${port}`;
  const clientId = `bridge-daemon-${suffix}`;
  const auditLogPath = path.join(workspace, 'bridge-daemon-audit.jsonl');
  const dashboardHeaders = {
    Authorization: `Bearer ${prepared.access_token}`,
    'New-Api-User': String(prepared.user_id),
  };

  if (!config.baseUrl) {
    log('starting new-api', { baseUrl });
    spawnLogged('new-api', 'go', ['run', '.'], {
      cwd: repoRoot,
      env: makeEnv({
        PORT: String(port),
        GIN_MODE: 'release',
        MCP_REMOTE_BRIDGE_TIMEOUT_MS: '10000',
      }),
    });
    await waitForHTTP(`${baseUrl}/api/setup`, config.timeoutMs);
  }

  log('starting local bridge daemon', { clientId, workspace });
  spawnLogged(
    'bridge-daemon',
    'node',
    [
      daemonPath,
      `--server=${bridgeWSURL(baseUrl)}`,
      `--token=${prepared.api_token}`,
      `--workspace=${workspace}`,
      `--client-id=${clientId}`,
      '--name=Local Bridge Daemon Smoke',
      '--enable-write',
      '--ping-interval-ms=5000',
      `--max-concurrency=${Math.max(config.concurrency, 8)}`,
      `--audit-log=${auditLogPath}`,
    ],
    { cwd: repoRoot },
  );
  const bridgeClient = await waitForBridgeClient(baseUrl, dashboardHeaders, clientId, config.timeoutMs);
  for (const capability of ['remote_read', 'remote_write', 'remote_edit', 'remote_tree', 'remote_glob', 'remote_grep', 'remote_env_info', 'mcp_proxy']) {
    if (!bridgeClient.capabilities?.includes(capability)) {
      throw new Error(`bridge daemon did not advertise ${capability}: ${JSON.stringify(bridgeClient.capabilities)}`);
    }
  }
  const reconnectResult = await assertReconnectScenario(
    baseUrl,
    prepared.api_token,
    dashboardHeaders,
    suffix,
    clientId,
    bridgeClient.session_id,
  );

  const proxy = await configureProxy(baseUrl, dashboardHeaders, clientId, localMCP.url, namespace);
  const exposedNames = proxy.tools.map((tool) => tool.exposed_tool_name);
  if (!exposedNames.includes(`${namespace}.echo`)) {
    throw new Error(`expected discovered echo proxy tool, got ${JSON.stringify(exposedNames)}`);
  }
  if (!exposedNames.includes(`${namespace}.fail`)) {
    throw new Error(`expected discovered fail proxy tool, got ${JSON.stringify(exposedNames)}`);
  }

  const tasks = buildConcurrentCalls(baseUrl, prepared.api_token, suffix, namespace);
  log('running concurrent MCP calls', { calls: tasks.length, concurrency: config.concurrency });
  const startedAt = Date.now();
  const results = await runPool(tasks, config.concurrency);
  const durationMS = Date.now() - startedAt;

  log('verifying concurrent call persistence', { calls: results.length });
  await assertConcurrentRecords(baseUrl, dashboardHeaders, results, namespace, clientId);

  log('verifying expected failure paths');
  const failureResults = await assertFailureScenarios(baseUrl, prepared.api_token, dashboardHeaders, suffix, namespace, clientId);

  const localAuditExpectations = [
    { requestId: reconnectResult.requestId, finalType: 'tool_result' },
    ...results.map((result) => ({ requestId: result.requestId, finalType: 'tool_result' })),
    ...failureResults.map((result) => ({ requestId: result.requestId, finalType: 'tool_error' })),
  ];
  const localAuditEvents = await waitForLocalAuditEvents(
    auditLogPath,
    localAuditExpectations,
    Math.min(config.timeoutMs, 30_000),
  );

  log('bridge daemon concurrency smoke passed', {
    calls: results.length,
    expected_failures: failureResults.length,
    local_audit_events: localAuditEvents.length,
    concurrency: config.concurrency,
    duration_ms: durationMS,
    client_id: clientId,
    reconnect_session_id: reconnectResult.sessionId,
    proxy_server_id: proxy.server.id,
    audit_log: auditLogPath,
  });
}

main()
  .then(cleanup)
  .catch(async (err) => {
    console.error(err.stack || err.message);
    await cleanup();
    process.exit(1);
  });
