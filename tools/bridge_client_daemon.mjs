#!/usr/bin/env node

import fs from 'fs/promises';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '..');

const DEFAULT_SERVER = 'ws://127.0.0.1:3000/bridge/ws';
const DEFAULT_CLIENT_ID = 'local-bridge-daemon';
const DEFAULT_CLIENT_NAME = 'Local Bridge Client Daemon';
const DEFAULT_VERSION = '0.1.0';
const DEFAULT_PING_INTERVAL_MS = 30_000;
const DEFAULT_RECONNECT_BASE_MS = 500;
const DEFAULT_RECONNECT_MAX_MS = 15_000;
const DEFAULT_MAX_CONCURRENCY = 16;
const DEFAULT_MAX_RESULTS = 200;
const DEFAULT_TREE_DEPTH = 3;
const DEFAULT_WALK_DEPTH = 8;
const DEFAULT_MAX_RESULT_BYTES = 512 * 1024;
const DEFAULT_MAX_SCAN_FILE_BYTES = 2 * 1024 * 1024;
const MCP_PROTOCOL_VERSION = '2025-06-18';

const DEFAULT_IGNORES = new Set([
  '.git',
  '.hg',
  '.svn',
  'node_modules',
  'vendor',
  'dist',
  'build',
  '.next',
  '.cache',
]);

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
  console.log(`Local data-proxy Bridge Client daemon

Usage:
  node tools/bridge_client_daemon.mjs --token=<data-proxy token> [options]

Options:
  --server=<url>                 Bridge WebSocket URL. Default ${DEFAULT_SERVER}
  --token=<token>                data-proxy token, or BRIDGE_DAEMON_TOKEN.
  --workspace=<path>             Local workspace root. Default repository root.
  --client-id=<id>               Bridge client id. Default ${DEFAULT_CLIENT_ID}
  --name=<name>                  Client display name.
  --version=<version>            Client version.
  --enable-write                 Advertise and execute remote_write/remote_edit.
  --advertise-disabled-write-tools
                                 Advertise write tools while keeping writes disabled; intended for smoke tests.
  --allow-absolute-path          Allow absolute paths outside workspace.
  --allow-non-loopback-mcp       Allow MCP proxy targets outside loopback.
  --max-concurrency=<n>          Concurrent tool calls. Default ${DEFAULT_MAX_CONCURRENCY}
  --max-results=<n>              Max listed/search results per tool. Default ${DEFAULT_MAX_RESULTS}
  --tree-depth=<n>               Default and maximum remote_tree depth. Default ${DEFAULT_TREE_DEPTH}
  --walk-depth=<n>               Default and maximum glob/grep walk depth. Default ${DEFAULT_WALK_DEPTH}
  --max-result-bytes=<n>         Max text result bytes. Default ${DEFAULT_MAX_RESULT_BYTES}
  --max-scan-file-bytes=<n>      Max file size scanned by remote_grep. Default ${DEFAULT_MAX_SCAN_FILE_BYTES}
  --ping-interval-ms=<ms>        Heartbeat interval. Default ${DEFAULT_PING_INTERVAL_MS}
  --no-reconnect                 Exit after first WebSocket close.
  --audit-log=<path>             Append local JSONL audit events.
  --self-test                    Run local file guard checks and exit without connecting.
  --help                         Show help.

Supported tools:
  remote_read, remote_tree, remote_glob, remote_grep, remote_env_info
  remote_write, remote_edit when --enable-write is set
  mcp_proxy.test, mcp_proxy.tools_list, mcp_proxy.tools_call
`);
}

function positiveInt(value, fallback, max = Number.MAX_SAFE_INTEGER) {
  if (value === undefined || value === null || value === '') return fallback;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) return fallback;
  return Math.min(parsed, max);
}

function buildConfig() {
  const args = parseArgs(process.argv.slice(2));
  if (args.help) {
    printHelp();
    process.exit(0);
  }
  const selfTest = args['self-test'] === true || process.env.BRIDGE_DAEMON_SELF_TEST === '1';
  const token = args.token || process.env.BRIDGE_DAEMON_TOKEN || process.env.QIDIAN_BRIDGE_TOKEN || '';
  if (!token && !selfTest) {
    console.error('missing token: pass --token=<token> or set BRIDGE_DAEMON_TOKEN');
    process.exit(1);
  }
  const workspace = path.resolve(args.workspace || process.env.BRIDGE_DAEMON_WORKSPACE || repoRoot);
  const enableWrite = args['enable-write'] === true || process.env.BRIDGE_DAEMON_ENABLE_WRITE === '1';
  const advertiseDisabledWriteTools = (
    args['advertise-disabled-write-tools'] === true
    || process.env.BRIDGE_DAEMON_ADVERTISE_DISABLED_WRITE_TOOLS === '1'
  );
  const capabilities = [
    'remote_read',
    'remote_tree',
    'remote_glob',
    'remote_grep',
    'remote_env_info',
    'mcp_proxy',
  ];
  if (enableWrite || advertiseDisabledWriteTools) {
    capabilities.push('remote_write', 'remote_edit');
  }
  return {
    server: args.server || process.env.BRIDGE_DAEMON_SERVER || DEFAULT_SERVER,
    token: token || 'self-test-token',
    workspace,
    clientId: args['client-id'] || process.env.BRIDGE_DAEMON_CLIENT_ID || DEFAULT_CLIENT_ID,
    name: args.name || process.env.BRIDGE_DAEMON_NAME || DEFAULT_CLIENT_NAME,
    version: args.version || process.env.BRIDGE_DAEMON_VERSION || DEFAULT_VERSION,
    platform: args.platform || process.env.BRIDGE_DAEMON_PLATFORM || `${os.platform()}-${os.arch()}`,
    enableWrite,
    advertiseDisabledWriteTools,
    allowAbsolutePath:
      args['allow-absolute-path'] === true || process.env.BRIDGE_DAEMON_ALLOW_ABSOLUTE_PATH === '1',
    allowNonLoopbackMCP:
      args['allow-non-loopback-mcp'] === true || process.env.BRIDGE_DAEMON_ALLOW_NON_LOOPBACK_MCP === '1',
    pingIntervalMs: positiveInt(args['ping-interval-ms'] || process.env.BRIDGE_DAEMON_PING_INTERVAL_MS, DEFAULT_PING_INTERVAL_MS),
    reconnect: args['no-reconnect'] !== true && process.env.BRIDGE_DAEMON_NO_RECONNECT !== '1',
    reconnectBaseMs: positiveInt(process.env.BRIDGE_DAEMON_RECONNECT_BASE_MS, DEFAULT_RECONNECT_BASE_MS),
    reconnectMaxMs: positiveInt(process.env.BRIDGE_DAEMON_RECONNECT_MAX_MS, DEFAULT_RECONNECT_MAX_MS),
    maxConcurrency: positiveInt(args['max-concurrency'] || process.env.BRIDGE_DAEMON_MAX_CONCURRENCY, DEFAULT_MAX_CONCURRENCY, 128),
    maxResults: positiveInt(args['max-results'] || process.env.BRIDGE_DAEMON_MAX_RESULTS, DEFAULT_MAX_RESULTS, 5000),
    treeDepth: positiveInt(args['tree-depth'] || process.env.BRIDGE_DAEMON_TREE_DEPTH, DEFAULT_TREE_DEPTH, 16),
    walkDepth: positiveInt(args['walk-depth'] || process.env.BRIDGE_DAEMON_WALK_DEPTH, DEFAULT_WALK_DEPTH, 32),
    maxResultBytes: positiveInt(args['max-result-bytes'] || process.env.BRIDGE_DAEMON_MAX_RESULT_BYTES, DEFAULT_MAX_RESULT_BYTES, 50 * 1024 * 1024),
    maxScanFileBytes: positiveInt(args['max-scan-file-bytes'] || process.env.BRIDGE_DAEMON_MAX_SCAN_FILE_BYTES, DEFAULT_MAX_SCAN_FILE_BYTES, 100 * 1024 * 1024),
    auditLog: args['audit-log'] || process.env.BRIDGE_DAEMON_AUDIT_LOG || '',
    selfTest,
    capabilities,
    mcp: new LocalMCPProxyClient(),
  };
}

function log(level, message, detail) {
  const prefix = `[${new Date().toISOString()}] [${level}]`;
  if (detail === undefined) {
    console.log(`${prefix} ${message}`);
  } else {
    console.log(`${prefix} ${message}`, detail);
  }
}

async function audit(config, event) {
  if (!config.auditLog) return;
  const line = JSON.stringify({ ts: new Date().toISOString(), ...event }) + '\n';
  await fs.mkdir(path.dirname(path.resolve(config.auditLog)), { recursive: true });
  await fs.appendFile(config.auditLog, line, 'utf8');
}

function send(ws, message) {
  ws.send(JSON.stringify(message));
}

function createWebSocket(config) {
  return new WebSocket(config.server, {
    headers: {
      Authorization: `Bearer ${config.token}`,
    },
  });
}

function sendRegister(ws, config) {
  send(ws, {
    type: 'register',
    data: {
      client_id: config.clientId,
      name: config.name,
      version: config.version,
      platform: config.platform,
      workspace: config.workspace,
      capabilities: config.capabilities,
    },
  });
}

function createToolError(code, message) {
  const err = new Error(message);
  err.code = code;
  return err;
}

function normalizeErrorCode(code, fallback = 'TOOL_CALL_FAILED') {
  if (code === undefined || code === null || code === '') return fallback;
  return String(code);
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isPathInside(basePath, targetPath) {
  const relative = path.relative(basePath, targetPath);
  return relative === '' || (!relative.startsWith('..') && !path.isAbsolute(relative));
}

async function workspaceRealPath(config) {
  return fs.realpath(config.workspace);
}

async function resolveExistingWorkspacePath(config, requestedPath, codePrefix) {
  if (typeof requestedPath !== 'string' || requestedPath.trim() === '') {
    throw createToolError(`${codePrefix}_INVALID_ARGUMENT`, 'path/file_path must be a non-empty string');
  }
  const rawPath = requestedPath.trim();
  const resolved = path.isAbsolute(rawPath) ? path.resolve(rawPath) : path.resolve(config.workspace, rawPath);
  let realPath;
  try {
    realPath = await fs.realpath(resolved);
  } catch {
    throw createToolError(`${codePrefix}_NOT_FOUND`, `path does not exist: ${requestedPath}`);
  }
  if (path.isAbsolute(rawPath) && config.allowAbsolutePath) {
    return realPath;
  }
  const root = await workspaceRealPath(config);
  if (!isPathInside(root, realPath)) {
    throw createToolError(`${codePrefix}_FORBIDDEN`, 'target path is outside workspace');
  }
  return realPath;
}

async function resolveWritableWorkspacePath(config, requestedPath, codePrefix, createDirs) {
  if (!config.enableWrite) {
    throw createToolError(`${codePrefix}_DISABLED`, 'write tools require --enable-write');
  }
  if (typeof requestedPath !== 'string' || requestedPath.trim() === '') {
    throw createToolError(`${codePrefix}_INVALID_ARGUMENT`, 'file_path must be a non-empty string');
  }
  const rawPath = requestedPath.trim();
  const resolved = path.isAbsolute(rawPath) ? path.resolve(rawPath) : path.resolve(config.workspace, rawPath);
  if (path.isAbsolute(rawPath) && config.allowAbsolutePath) {
    if (createDirs) await fs.mkdir(path.dirname(resolved), { recursive: true });
    return resolved;
  }
  const root = await workspaceRealPath(config);
  let guardPath = resolved;
  try {
    guardPath = await fs.realpath(resolved);
  } catch {
    const parent = path.dirname(resolved);
    if (createDirs) {
      await fs.mkdir(parent, { recursive: true });
    }
    try {
      guardPath = await fs.realpath(parent);
    } catch {
      throw createToolError(`${codePrefix}_NOT_FOUND`, `parent directory does not exist: ${parent}`);
    }
  }
  if (!isPathInside(root, guardPath) || !isPathInside(root, resolved)) {
    throw createToolError(`${codePrefix}_FORBIDDEN`, 'target path is outside workspace');
  }
  return resolved;
}

function relativeWorkspacePath(config, absolutePath) {
  return (path.relative(config.workspace, absolutePath) || '.').replaceAll(path.sep, '/');
}

function lineOption(value, fallback) {
  return positiveInt(value, fallback, 100_000);
}

function sliceLines(text, offset, limit) {
  const lines = text.split(/\r?\n/);
  const start = Math.max(offset - 1, 0);
  const end = limit > 0 ? start + limit : undefined;
  return {
    text: lines.slice(start, end).join('\n'),
    totalLines: lines.length,
    startLine: start + 1,
    endLine: end ? Math.min(end, lines.length) : lines.length,
  };
}

function configuredLimit(config, key, fallback, hardMax) {
  return positiveInt(config?.[key], fallback, hardMax);
}

function cappedOption(value, fallback, cap) {
  return Math.min(positiveInt(value, fallback, cap), cap);
}

function maxResultsOption(config, value) {
  const cap = configuredLimit(config, 'maxResults', DEFAULT_MAX_RESULTS, 5000);
  return cappedOption(value, cap, cap);
}

function treeDepthOption(config, value) {
  const cap = configuredLimit(config, 'treeDepth', DEFAULT_TREE_DEPTH, 16);
  return cappedOption(value, cap, cap);
}

function walkDepthOption(config, value) {
  const cap = configuredLimit(config, 'walkDepth', DEFAULT_WALK_DEPTH, 32);
  return cappedOption(value, cap, cap);
}

function scanCandidateLimit(config, outputLimit, multiplier) {
  const cap = configuredLimit(config, 'maxResults', DEFAULT_MAX_RESULTS, 5000);
  return Math.min(outputLimit * multiplier, cap * multiplier, 5000);
}

function enforceResultLimit(text, config) {
  const maxResultBytes = configuredLimit(config, 'maxResultBytes', DEFAULT_MAX_RESULT_BYTES, 50 * 1024 * 1024);
  const bytes = Buffer.byteLength(text, 'utf8');
  if (bytes <= maxResultBytes) {
    return { text, bytes, truncated: false };
  }
  const chunks = [];
  let size = 0;
  for (const char of text) {
    const charSize = Buffer.byteLength(char, 'utf8');
    if (size + charSize > maxResultBytes) break;
    chunks.push(char);
    size += charSize;
  }
  return {
    text: chunks.join('') + '\n\n[result truncated by bridge daemon]',
    bytes: size,
    truncated: true,
  };
}

function escapeRegExp(value) {
  return String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function globToRegExp(pattern) {
  const normalized = String(pattern || '').replaceAll('\\', '/');
  let source = '';
  for (let i = 0; i < normalized.length; i += 1) {
    const char = normalized[i];
    const next = normalized[i + 1];
    if (char === '*') {
      if (next === '*') {
        source += '.*';
        i += 1;
      } else {
        source += '[^/]*';
      }
      continue;
    }
    if (char === '?') {
      source += '[^/]';
      continue;
    }
    source += escapeRegExp(char);
  }
  return new RegExp(`^${source}$`);
}

async function walkWorkspace(rootPath, options = {}) {
  const maxDepth = positiveInt(options.maxDepth, DEFAULT_WALK_DEPTH, 32);
  const maxResults = positiveInt(options.maxResults, DEFAULT_MAX_RESULTS, 5000);
  const includeDirectories = Boolean(options.includeDirectories);
  const items = [];

  async function visit(currentPath, depth) {
    if (items.length >= maxResults) return;
    let entries;
    try {
      entries = await fs.readdir(currentPath, { withFileTypes: true });
    } catch {
      return;
    }
    entries.sort((a, b) => a.name.localeCompare(b.name));
    for (const entry of entries) {
      if (items.length >= maxResults) return;
      if (entry.isDirectory() && DEFAULT_IGNORES.has(entry.name)) continue;
      const absolutePath = path.join(currentPath, entry.name);
      if (entry.isDirectory()) {
        if (includeDirectories) {
          items.push({ path: absolutePath, type: 'directory', depth });
        }
        if (depth < maxDepth) {
          await visit(absolutePath, depth + 1);
        }
      } else if (entry.isFile()) {
        items.push({ path: absolutePath, type: 'file', depth });
      }
    }
  }

  await visit(rootPath, 1);
  return {
    items,
    truncated: items.length >= maxResults,
  };
}

async function handleRemoteRead(config, args) {
  const startedAt = Date.now();
  const filePath = await resolveExistingWorkspacePath(config, args?.file_path, 'REMOTE_READ');
  const stat = await fs.stat(filePath);
  if (!stat.isFile()) {
    throw createToolError('REMOTE_READ_NOT_FILE', `target is not a regular file: ${args?.file_path}`);
  }
  const raw = await fs.readFile(filePath, 'utf8');
  const offset = lineOption(args?.offset, 1);
  const limit = lineOption(args?.limit, 100);
  const sliced = sliceLines(raw, offset, limit);
  const limited = enforceResultLimit(sliced.text, config);
  const relative = relativeWorkspacePath(config, filePath);
  return {
    content: [{ type: 'text', text: limited.text }],
    summary: `${relative}:${sliced.startLine}-${sliced.endLine}`,
    duration_ms: Date.now() - startedAt,
    result_size: limited.bytes,
    metadata: {
      file_path: relative,
      offset,
      limit,
      total_lines: sliced.totalLines,
      truncated: limited.truncated,
      daemon: true,
    },
  };
}

async function handleRemoteWrite(config, args) {
  const startedAt = Date.now();
  if (typeof args?.content !== 'string') {
    throw createToolError('REMOTE_WRITE_INVALID_ARGUMENT', 'content must be a string');
  }
  const target = await resolveWritableWorkspacePath(config, args?.file_path, 'REMOTE_WRITE', args?.create_dirs !== false);
  const bytes = Buffer.byteLength(args.content, 'utf8');
  await fs.writeFile(target, args.content, 'utf8');
  const relative = relativeWorkspacePath(config, target);
  return {
    content: [{ type: 'text', text: `wrote ${bytes} bytes to ${relative}` }],
    summary: `wrote ${relative}`,
    duration_ms: Date.now() - startedAt,
    result_size: bytes,
    metadata: {
      file_path: relative,
      bytes,
      daemon: true,
    },
  };
}

async function handleRemoteEdit(config, args) {
  const startedAt = Date.now();
  if (typeof args?.old_string !== 'string' || args.old_string === '') {
    throw createToolError('REMOTE_EDIT_INVALID_ARGUMENT', 'old_string must be a non-empty string');
  }
  if (typeof args?.new_string !== 'string') {
    throw createToolError('REMOTE_EDIT_INVALID_ARGUMENT', 'new_string must be a string');
  }
  const target = await resolveWritableWorkspacePath(config, args?.file_path, 'REMOTE_EDIT', false);
  const original = await fs.readFile(target, 'utf8');
  const occurrences = original.split(args.old_string).length - 1;
  if (occurrences === 0) {
    throw createToolError('REMOTE_EDIT_NOT_FOUND', 'old_string was not found');
  }
  if (occurrences > 1 && args?.replace_all !== true) {
    throw createToolError('REMOTE_EDIT_AMBIGUOUS', 'old_string matched multiple times; set replace_all=true');
  }
  const next = args?.replace_all === true
    ? original.split(args.old_string).join(args.new_string)
    : original.replace(args.old_string, args.new_string);
  await fs.writeFile(target, next, 'utf8');
  const relative = relativeWorkspacePath(config, target);
  const bytes = Buffer.byteLength(next, 'utf8');
  return {
    content: [{ type: 'text', text: `edited ${relative}; replacements=${args?.replace_all === true ? occurrences : 1}` }],
    summary: `edited ${relative}`,
    duration_ms: Date.now() - startedAt,
    result_size: bytes,
    metadata: {
      file_path: relative,
      replacements: args?.replace_all === true ? occurrences : 1,
      bytes,
      daemon: true,
    },
  };
}

async function handleRemoteTree(config, args) {
  const startedAt = Date.now();
  const rootPath = await resolveExistingWorkspacePath(config, args?.path || '.', 'REMOTE_TREE');
  const stat = await fs.stat(rootPath);
  if (!stat.isDirectory()) {
    throw createToolError('REMOTE_TREE_NOT_DIRECTORY', `target is not a directory: ${args?.path || '.'}`);
  }
  const depth = treeDepthOption(config, args?.depth ?? args?.max_depth);
  const maxResults = maxResultsOption(config, args?.max_results);
  const walked = await walkWorkspace(rootPath, { maxDepth: depth, maxResults, includeDirectories: true });
  const rootRelative = relativeWorkspacePath(config, rootPath);
  const lines = [`${rootRelative}/`];
  for (const item of walked.items) {
    const indent = '  '.repeat(Math.max(item.depth - 1, 0));
    lines.push(`${indent}${item.type === 'directory' ? 'd' : '-'} ${relativeWorkspacePath(config, item.path)}`);
  }
  const limited = enforceResultLimit(lines.join('\n'), config);
  return {
    content: [{ type: 'text', text: limited.text }],
    summary: `${rootRelative} (${walked.items.length} entries)`,
    duration_ms: Date.now() - startedAt,
    result_size: limited.bytes,
    metadata: {
      path: rootRelative,
      depth,
      count: walked.items.length,
      truncated: walked.truncated || limited.truncated,
      daemon: true,
    },
  };
}

async function handleRemoteGlob(config, args) {
  const startedAt = Date.now();
  if (typeof args?.pattern !== 'string' || args.pattern.trim() === '') {
    throw createToolError('REMOTE_GLOB_INVALID_ARGUMENT', 'pattern must be a non-empty string');
  }
  const rootPath = await resolveExistingWorkspacePath(config, args?.path || '.', 'REMOTE_GLOB');
  const maxResults = maxResultsOption(config, args?.max_results);
  const matcher = globToRegExp(args.pattern.trim());
  const walked = await walkWorkspace(rootPath, {
    maxDepth: walkDepthOption(config, args?.max_depth),
    maxResults: scanCandidateLimit(config, maxResults, 4),
  });
  const matches = [];
  for (const item of walked.items) {
    const relative = relativeWorkspacePath(config, item.path);
    if (!matcher.test(relative) && !matcher.test(path.basename(relative))) continue;
    matches.push(relative);
    if (matches.length >= maxResults) break;
  }
  const limited = enforceResultLimit(matches.join('\n'), config);
  return {
    content: [{ type: 'text', text: limited.text }],
    summary: `${matches.length} files matched ${args.pattern}`,
    duration_ms: Date.now() - startedAt,
    result_size: limited.bytes,
    metadata: {
      pattern: args.pattern,
      path: relativeWorkspacePath(config, rootPath),
      count: matches.length,
      truncated: matches.length >= maxResults || limited.truncated,
      daemon: true,
    },
  };
}

async function handleRemoteGrep(config, args) {
  const startedAt = Date.now();
  if (typeof args?.pattern !== 'string' || args.pattern.trim() === '') {
    throw createToolError('REMOTE_GREP_INVALID_ARGUMENT', 'pattern must be a non-empty string');
  }
  const rootPath = await resolveExistingWorkspacePath(config, args?.path || '.', 'REMOTE_GREP');
  const maxResults = maxResultsOption(config, args?.max_results);
  const globMatcher = args?.glob || args?.glob_pattern ? globToRegExp(args.glob || args.glob_pattern) : null;
  let matcher;
  try {
    matcher = new RegExp(args.pattern, args?.case_insensitive ? 'i' : '');
  } catch (err) {
    throw createToolError('REMOTE_GREP_INVALID_PATTERN', err.message);
  }
  const stat = await fs.stat(rootPath);
  const candidates = [];
  if (stat.isFile()) {
    candidates.push({ path: rootPath });
  } else if (stat.isDirectory()) {
    const walked = await walkWorkspace(rootPath, {
      maxDepth: walkDepthOption(config, args?.max_depth),
      maxResults: scanCandidateLimit(config, maxResults, 8),
    });
    candidates.push(...walked.items);
  } else {
    throw createToolError('REMOTE_GREP_INVALID_PATH', 'target is not a file or directory');
  }

  const matches = [];
  for (const item of candidates) {
    if (matches.length >= maxResults) break;
    const relative = relativeWorkspacePath(config, item.path);
    if (globMatcher && !globMatcher.test(relative) && !globMatcher.test(path.basename(relative))) continue;
    let fileStat;
    try {
      fileStat = await fs.stat(item.path);
    } catch {
      continue;
    }
    if (!fileStat.isFile() || fileStat.size > configuredLimit(config, 'maxScanFileBytes', DEFAULT_MAX_SCAN_FILE_BYTES, 100 * 1024 * 1024)) continue;
    let raw;
    try {
      raw = await fs.readFile(item.path, 'utf8');
    } catch {
      continue;
    }
    const lines = raw.split(/\r?\n/);
    for (let index = 0; index < lines.length; index += 1) {
      matcher.lastIndex = 0;
      if (!matcher.test(lines[index])) continue;
      matches.push(`${relative}:${index + 1}: ${lines[index]}`);
      if (matches.length >= maxResults) break;
    }
  }
  const limited = enforceResultLimit(matches.join('\n'), config);
  return {
    content: [{ type: 'text', text: limited.text }],
    summary: `${matches.length} matches for ${args.pattern}`,
    duration_ms: Date.now() - startedAt,
    result_size: limited.bytes,
    metadata: {
      pattern: args.pattern,
      path: relativeWorkspacePath(config, rootPath),
      count: matches.length,
      truncated: matches.length >= maxResults || limited.truncated,
      daemon: true,
    },
  };
}

async function handleRemoteEnvInfo(config) {
  const startedAt = Date.now();
  const data = {
    platform: os.platform(),
    arch: os.arch(),
    release: os.release(),
    hostname: os.hostname(),
    node: process.version,
    workspace: config.workspace,
    client_id: config.clientId,
    capabilities: config.capabilities,
    limits: {
      max_results: configuredLimit(config, 'maxResults', DEFAULT_MAX_RESULTS, 5000),
      tree_depth: configuredLimit(config, 'treeDepth', DEFAULT_TREE_DEPTH, 16),
      walk_depth: configuredLimit(config, 'walkDepth', DEFAULT_WALK_DEPTH, 32),
      max_result_bytes: configuredLimit(config, 'maxResultBytes', DEFAULT_MAX_RESULT_BYTES, 50 * 1024 * 1024),
      max_scan_file_bytes: configuredLimit(config, 'maxScanFileBytes', DEFAULT_MAX_SCAN_FILE_BYTES, 100 * 1024 * 1024),
    },
    daemon: true,
  };
  const text = JSON.stringify(data, null, 2);
  const limited = enforceResultLimit(text, config);
  return {
    content: [{ type: 'text', text: limited.text }],
    summary: `${data.platform}-${data.arch} ${data.node}`,
    duration_ms: Date.now() - startedAt,
    result_size: limited.bytes,
    metadata: data,
  };
}

function bridgeEndpointTarget(args) {
  if (typeof args?.target === 'string' && args.target.trim()) {
    return args.target.trim();
  }
  const endpoint = typeof args?.endpoint === 'string' ? args.endpoint.trim() : '';
  if (!endpoint) return '';
  try {
    const parsed = new URL(endpoint);
    const queryTarget = parsed.searchParams.get('target');
    if (queryTarget) return queryTarget.trim();
    const pathTarget = decodeURIComponent(parsed.pathname.replace(/^\/+/, ''));
    return pathTarget.trim();
  } catch {
    return endpoint;
  }
}

function assertAllowedMCPTarget(config, target) {
  let parsed;
  try {
    parsed = new URL(target);
  } catch {
    throw createToolError('MCP_PROXY_INVALID_TARGET', `invalid MCP target: ${target}`);
  }
  if (!['http:', 'https:'].includes(parsed.protocol)) {
    throw createToolError('MCP_PROXY_INVALID_TARGET', 'only http/https MCP targets are supported by this daemon');
  }
  if (config.allowNonLoopbackMCP) return parsed.toString();
  const host = parsed.hostname.toLowerCase();
  if (host === 'localhost' || host === '127.0.0.1' || host === '::1') {
    return parsed.toString();
  }
  throw createToolError('MCP_PROXY_FORBIDDEN_TARGET', 'MCP proxy target must be loopback unless --allow-non-loopback-mcp is set');
}

class LocalMCPProxyClient {
  constructor() {
    this.nextId = 1;
    this.sessions = new Map();
    this.initialized = new Set();
  }

  async test(config, args) {
    const startedAt = Date.now();
    const target = assertAllowedMCPTarget(config, bridgeEndpointTarget(args));
    const result = await this.initialize(target);
    await this.initializedNotification(target).catch((err) => {
      if (!String(err.message || '').includes('Method not found')) throw err;
    });
    await this.rpc(target, 'ping', {}).catch(() => {});
    const payload = {
      protocol_version: result.protocolVersion || MCP_PROTOCOL_VERSION,
      server_name: result.serverInfo?.name || args?.server?.name || 'local-mcp',
      capabilities: result.capabilities || {},
    };
    return bridgeResult(config, payload, `MCP ${payload.server_name} ready`, Date.now() - startedAt, {
      result: payload,
      target,
    });
  }

  async listTools(config, args) {
    const startedAt = Date.now();
    const target = assertAllowedMCPTarget(config, bridgeEndpointTarget(args));
    await this.ensureInitialized(target);
    const result = await this.rpc(target, 'tools/list', {});
    const tools = Array.isArray(result?.tools) ? result.tools : [];
    return bridgeResult(config, tools, `${tools.length} tools discovered`, Date.now() - startedAt, {
      result: { tools },
      target,
    });
  }

  async callTool(config, args) {
    const startedAt = Date.now();
    const target = assertAllowedMCPTarget(config, bridgeEndpointTarget(args));
    await this.ensureInitialized(target);
    const result = await this.rpc(target, 'tools/call', {
      name: args?.name,
      arguments: args?.arguments || {},
    });
    const content = Array.isArray(result?.content) ? result.content : [{ type: 'text', text: JSON.stringify(result ?? null) }];
    const text = content.map((item) => item?.text || '').filter(Boolean).join('\n');
    const bytes = Buffer.byteLength(JSON.stringify(result ?? {}), 'utf8');
    return {
      content,
      summary: result?.summary || text.slice(0, 160) || String(args?.name || 'mcp_proxy.tools_call'),
      duration_ms: Date.now() - startedAt,
      result_size: bytes,
      metadata: {
        ...(result?.metadata && typeof result.metadata === 'object' ? result.metadata : {}),
        target,
        tool_name: args?.name,
      },
    };
  }

  async ensureInitialized(target) {
    if (this.initialized.has(target)) return;
    await this.initialize(target);
    await this.initializedNotification(target).catch((err) => {
      if (!String(err.message || '').includes('Method not found')) throw err;
    });
    this.initialized.add(target);
  }

  async initialize(target) {
    this.initialized.delete(target);
    this.sessions.delete(target);
    const result = await this.rpc(target, 'initialize', {
      protocolVersion: MCP_PROTOCOL_VERSION,
      capabilities: {},
      clientInfo: {
        name: 'data-proxy-local-bridge-daemon',
        version: DEFAULT_VERSION,
      },
    });
    this.initialized.add(target);
    return result || {};
  }

  async initializedNotification(target) {
    await this.post(target, {
      jsonrpc: '2.0',
      method: 'notifications/initialized',
      params: {},
    }, true);
  }

  async rpc(target, method, params) {
    const id = this.nextId++;
    const response = await this.post(target, {
      jsonrpc: '2.0',
      id,
      method,
      params,
    }, false);
    if (response?.error) {
      const err = new Error(response.error.message || 'MCP upstream error');
      if (response.error.code === undefined || response.error.code === null || response.error.code === '') {
        err.code = 'MCP_PROXY_UPSTREAM_ERROR';
      } else {
        err.code = `MCP_PROXY_UPSTREAM_${String(response.error.code).replace(/[^A-Za-z0-9_.:-]/g, '_')}`;
      }
      throw err;
    }
    return response?.result;
  }

  async post(target, body, notification) {
    const headers = {
      'Content-Type': 'application/json',
      Accept: 'application/json, text/event-stream',
      'MCP-Protocol-Version': MCP_PROTOCOL_VERSION,
    };
    const session = this.sessions.get(target);
    if (session) {
      headers['Mcp-Session-Id'] = session;
    }
    const response = await fetch(target, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
    });
    const nextSession = response.headers.get('Mcp-Session-Id');
    if (nextSession) {
      this.sessions.set(target, nextSession);
    }
    const text = await response.text();
    if (!response.ok) {
      if (response.status === 404 && session) {
        this.sessions.delete(target);
        this.initialized.delete(target);
      }
      throw createToolError('MCP_PROXY_HTTP_ERROR', `MCP upstream HTTP ${response.status}: ${text.slice(0, 256)}`);
    }
    if (notification || !text.trim()) {
      return {};
    }
    const trimmed = text.trim();
    if (trimmed.startsWith('data:')) {
      const line = trimmed.split(/\r?\n/).find((item) => item.startsWith('data:'));
      return JSON.parse(line.slice(5).trim());
    }
    return JSON.parse(trimmed);
  }
}

function bridgeResult(config, value, summary, durationMS, metadata) {
  const text = typeof value === 'string' ? value : JSON.stringify(value, null, 2);
  const limited = enforceResultLimit(text, config);
  return {
    content: [{ type: 'text', text: limited.text }],
    summary,
    duration_ms: durationMS,
    result_size: limited.bytes,
    metadata: metadata || {},
  };
}

async function handleMCPProxy(config, toolName, args) {
  switch (toolName) {
    case 'mcp_proxy.test':
      return config.mcp.test(config, args);
    case 'mcp_proxy.tools_list':
      return config.mcp.listTools(config, args);
    case 'mcp_proxy.tools_call':
      return config.mcp.callTool(config, args);
    default:
      throw createToolError('MCP_PROXY_TOOL_NOT_SUPPORTED', `unsupported MCP proxy bridge tool: ${toolName}`);
  }
}

async function handleToolCall(config, message) {
  const requestId = message?.data?.request_id || message?.id;
  const toolName = message?.data?.tool_name;
  const args = message?.data?.arguments || {};
  if (!requestId) {
    throw createToolError('TOOL_CALL_INVALID_MESSAGE', 'missing request_id');
  }
  if (args?.mock_error_code) {
    throw createToolError(String(args.mock_error_code), String(args.mock_error_message || args.mock_error_code));
  }
  const delayMS = positiveInt(args?.mock_delay_ms, 0, 30_000);
  if (delayMS > 0) await sleep(delayMS);

  const handlers = {
    remote_read: handleRemoteRead,
    remote_write: handleRemoteWrite,
    remote_edit: handleRemoteEdit,
    remote_tree: handleRemoteTree,
    remote_glob: handleRemoteGlob,
    remote_grep: handleRemoteGrep,
    remote_env_info: handleRemoteEnvInfo,
  };
  if (toolName?.startsWith('mcp_proxy.')) {
    return handleMCPProxy(config, toolName, args);
  }
  const handler = handlers[toolName];
  if (!handler || !config.capabilities.includes(toolName)) {
    throw createToolError('TOOL_NOT_SUPPORTED', `tool is not supported by this daemon: ${toolName || '<empty>'}`);
  }
  return handler(config, args);
}

function getRequestId(message) {
  return message?.data?.request_id || message?.id || `bridge-${Date.now()}`;
}

function createLimiter(maxConcurrency) {
  let active = 0;
  const queue = [];
  function pump() {
    while (active < maxConcurrency && queue.length > 0) {
      const item = queue.shift();
      active += 1;
      item.fn()
        .then(item.resolve, item.reject)
        .finally(() => {
          active -= 1;
          pump();
        });
    }
  }
  return function limit(fn) {
    return new Promise((resolve, reject) => {
      queue.push({ fn, resolve, reject });
      pump();
    });
  };
}

async function onMessage(ws, config, limit, raw, connectionState) {
  let message;
  try {
    message = JSON.parse(raw.toString());
  } catch {
    log('WARN', 'received non-JSON bridge message', raw.toString());
    return;
  }
  if (message.type === 'registered') {
    log('INFO', 'registered to data-proxy bridge', message.data);
    await audit(config, { type: 'registered', data: message.data });
    return;
  }
  if (message.type === 'pong') return;
  if (message.type === 'close') {
    log('WARN', 'server requested bridge close', message.data);
    const reason = typeof message.data?.reason === 'string' ? message.data.reason : 'server requested bridge close';
    connectionState.serverCloseReason = reason;
    await audit(config, {
      type: 'server_close',
      connection_attempt: connectionState.connectionAttempt,
      reason,
      data: message.data || {},
    });
    try {
      ws.close(1000, reason.slice(0, 120));
    } catch {
      // The close may race with a network disconnect; the reconnect loop handles both paths.
    }
    return;
  }
  if (message.type !== 'tool_call') {
    log('WARN', `ignored bridge message type: ${message.type}`);
    return;
  }

  const requestId = getRequestId(message);
  const toolName = message?.data?.tool_name;
  await limit(async () => {
    const startedAt = Date.now();
    log('INFO', 'tool_call received', { request_id: requestId, tool_name: toolName });
    await audit(config, { type: 'tool_call', request_id: requestId, tool_name: toolName, arguments: message?.data?.arguments || {} });
    try {
      const result = await handleToolCall(config, message);
      send(ws, { type: 'tool_result', id: requestId, data: result });
      await audit(config, {
        type: 'tool_result',
        request_id: requestId,
        tool_name: toolName,
        duration_ms: Date.now() - startedAt,
        result_size: result.result_size || 0,
      });
      log('INFO', 'tool_result sent', { request_id: requestId, tool_name: toolName, duration_ms: Date.now() - startedAt });
    } catch (err) {
      const code = normalizeErrorCode(err.code);
      send(ws, {
        type: 'tool_error',
        id: requestId,
        data: {
          code,
          message: err.message || 'tool_call failed',
        },
      });
      await audit(config, {
        type: 'tool_error',
        request_id: requestId,
        tool_name: toolName,
        code,
        message: err.message,
        duration_ms: Date.now() - startedAt,
      });
      log('ERROR', 'tool_error sent', { request_id: requestId, code, message: err.message });
    }
  });
}

function startHeartbeat(ws, intervalMs) {
  if (!Number.isFinite(intervalMs) || intervalMs <= 0) return null;
  return setInterval(() => {
    if (ws.readyState !== WebSocket.OPEN) return;
    send(ws, { type: 'ping', id: `ping-${Date.now()}` });
  }, intervalMs);
}

async function runOnce(config, connectionAttempt) {
  return new Promise((resolve) => {
    const ws = createWebSocket(config);
    const limit = createLimiter(config.maxConcurrency);
    const connectionState = {
      connectionAttempt,
      serverCloseReason: '',
    };
    let heartbeat = null;
    let opened = false;

    ws.addEventListener('open', () => {
      opened = true;
      log('INFO', 'WebSocket connected; registering bridge client', {
        server: config.server,
        client_id: config.clientId,
        workspace: config.workspace,
        capabilities: config.capabilities,
      });
      audit(config, {
        type: 'connection_open',
        connection_attempt: connectionAttempt,
        server: config.server,
        client_id: config.clientId,
      }).catch((err) => {
        log('ERROR', 'failed to write connection_open audit', err.message);
      });
      sendRegister(ws, config);
      heartbeat = startHeartbeat(ws, config.pingIntervalMs);
    });
    ws.addEventListener('message', (event) => {
      onMessage(ws, config, limit, event.data, connectionState).catch((err) => {
        log('ERROR', 'unhandled bridge message error', err.stack || err.message);
      });
    });
    ws.addEventListener('error', (event) => {
      log('ERROR', 'WebSocket error', event.message || event.error?.message || event);
    });
    ws.addEventListener('close', (event) => {
      if (heartbeat) clearInterval(heartbeat);
      const result = {
        opened,
        clean: event.wasClean,
        closeCode: event.code,
        closeReason: event.reason || '',
        serverCloseReason: connectionState.serverCloseReason,
      };
      log('WARN', 'WebSocket closed', {
        code: event.code,
        reason: event.reason,
        was_clean: event.wasClean,
      });
      audit(config, {
        type: 'connection_close',
        connection_attempt: connectionAttempt,
        opened,
        clean_close: event.wasClean,
        close_code: event.code,
        close_reason: event.reason || '',
        server_close_reason: connectionState.serverCloseReason,
      }).catch((err) => {
        log('ERROR', 'failed to write connection_close audit', err.message);
      }).finally(() => {
        resolve(result);
      });
    });
  });
}

async function expectToolError(label, expectedCode, fn) {
  try {
    await fn();
  } catch (err) {
    if (err.code === expectedCode) return;
    throw new Error(`${label} returned ${err.code || '<no code>'}, expected ${expectedCode}: ${err.message}`);
  }
  throw new Error(`${label} unexpectedly succeeded; expected ${expectedCode}`);
}

async function runSelfTest(config) {
  await fs.mkdir(config.workspace, { recursive: true });
  const docsDir = path.join(config.workspace, 'docs');
  await fs.mkdir(docsDir, { recursive: true });
  await fs.writeFile(path.join(docsDir, 'seed.txt'), 'bridge daemon self-test\n', 'utf8');

  const readOnlyConfig = {
    ...config,
    enableWrite: false,
    capabilities: ['remote_read', 'remote_tree', 'remote_glob', 'remote_grep', 'remote_env_info', 'mcp_proxy', 'remote_write', 'remote_edit'],
  };
  const writeConfig = {
    ...readOnlyConfig,
    enableWrite: true,
  };

  const readResult = await handleRemoteRead(readOnlyConfig, { file_path: 'docs/seed.txt', offset: 1, limit: 5 });
  const readText = readResult.content?.[0]?.text || '';
  if (!readText.includes('bridge daemon self-test')) {
    throw new Error(`self-test remote_read mismatch: ${readText}`);
  }
  const envResult = await handleRemoteEnvInfo(readOnlyConfig);
  const limits = envResult.metadata?.limits || {};
  if (
    limits.max_results !== config.maxResults
    || limits.tree_depth !== config.treeDepth
    || limits.walk_depth !== config.walkDepth
    || limits.max_result_bytes !== config.maxResultBytes
    || limits.max_scan_file_bytes !== config.maxScanFileBytes
  ) {
    throw new Error(`self-test remote_env_info limits mismatch: ${JSON.stringify(limits)}`);
  }

  await expectToolError('write-disabled remote_write', 'REMOTE_WRITE_DISABLED', () => (
    handleRemoteWrite(readOnlyConfig, {
      file_path: 'out/disabled.txt',
      content: 'must not be written\n',
      create_dirs: true,
    })
  ));

  await expectToolError('path traversal remote_write', 'REMOTE_WRITE_FORBIDDEN', () => (
    handleRemoteWrite(writeConfig, {
      file_path: '../outside-workspace.txt',
      content: 'must not escape workspace\n',
      create_dirs: true,
    })
  ));

  console.log(JSON.stringify({
    ok: true,
    workspace: config.workspace,
    checks: ['remote_read', 'remote_env_info_limits', 'remote_write_disabled', 'remote_write_path_guard'],
  }));
}

async function main() {
  const config = buildConfig();
  await fs.mkdir(config.workspace, { recursive: true });
  if (config.selfTest) {
    await runSelfTest(config);
    return;
  }
  let attempt = 0;
  for (;;) {
    const result = await runOnce(config, attempt + 1);
    if (!config.reconnect) {
      process.exit(result.clean ? 0 : 1);
    }
    attempt += 1;
    const delay = Math.min(config.reconnectBaseMs * 2 ** Math.min(attempt - 1, 6), config.reconnectMaxMs);
    log('INFO', 'reconnecting bridge client', { attempt, delay_ms: delay, opened: result.opened });
    await audit(config, {
      type: 'reconnect_scheduled',
      attempt,
      delay_ms: delay,
      opened: result.opened,
      clean_close: result.clean,
      close_code: result.closeCode,
      close_reason: result.closeReason,
      server_close_reason: result.serverCloseReason,
    });
    await sleep(delay);
  }
}

main().catch((err) => {
  console.error(err.stack || err.message);
  process.exit(1);
});
