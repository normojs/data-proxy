/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url))
const PROJECT_ROOT = path.resolve(SCRIPT_DIR, '..')

async function readProjectFile(relativePath) {
  return fs.readFile(path.join(PROJECT_ROOT, relativePath), 'utf8')
}

function assertIncludes(text, needle, label) {
  assert.ok(text.includes(needle), `${label} is missing: ${needle}`)
}

const [routeTree, mcpIndexRoute, mcpSectionRoute, sectionRegistry] =
  await Promise.all([
    readProjectFile('src/routeTree.gen.ts'),
    readProjectFile('src/routes/_authenticated/mcp/index.tsx'),
    readProjectFile('src/routes/_authenticated/mcp/$section.tsx'),
    readProjectFile('src/features/mcp/section-registry.tsx'),
  ])

for (const [needle, label] of [
  [
    "import { Route as AuthenticatedMcpIndexRouteImport } from './routes/_authenticated/mcp/index'",
    'MCP index route import',
  ],
  [
    "import { Route as AuthenticatedMcpSectionRouteImport } from './routes/_authenticated/mcp/$section'",
    'MCP section route import',
  ],
  ["id: '/mcp/'", 'MCP index route id'],
  ["path: '/mcp/'", 'MCP index route path'],
  ["id: '/mcp/$section'", 'MCP section route id'],
  ["path: '/mcp/$section'", 'MCP section route path'],
  ["'/_authenticated/mcp/': {", 'MCP index generated route entry'],
  ["fullPath: '/mcp/'", 'MCP index generated full path'],
  ["'/_authenticated/mcp/$section': {", 'MCP section generated route entry'],
  ["fullPath: '/mcp/$section'", 'MCP section generated full path'],
]) {
  assertIncludes(routeTree, needle, label)
}

assertIncludes(
  mcpIndexRoute,
  "createFileRoute('/_authenticated/mcp/')",
  'MCP index route file'
)
assertIncludes(
  mcpSectionRoute,
  "createFileRoute('/_authenticated/mcp/$section')",
  'MCP section route file'
)

const sectionBlock = sectionRegistry.match(
  /export const MCP_SECTIONS = \[([\s\S]*?)\] as const/
)?.[1]
assert.ok(sectionBlock, 'MCP_SECTIONS block is missing')

const sectionIds = [...sectionBlock.matchAll(/\bid:\s*'([^']+)'/g)].map(
  (match) => match[1]
)
assert.ok(sectionIds.length > 0, 'MCP_SECTIONS must define at least one section')
assert.equal(
  new Set(sectionIds).size,
  sectionIds.length,
  'MCP_SECTIONS contains duplicate section ids'
)

const expectedCoreSections = [
  'market',
  'overview',
  'tools',
  'proxy-servers',
  'proxy-tools',
  'bridge-clients',
  'tool-calls',
  'billing-events',
  'audit-logs',
]
for (const sectionId of expectedCoreSections) {
  assert.ok(
    sectionIds.includes(sectionId),
    `MCP_SECTIONS is missing ${sectionId}`
  )
}
for (const sectionId of sectionIds) {
  assert.match(
    sectionId,
    /^[a-z0-9]+(?:-[a-z0-9]+)*$/,
    `MCP section id is not route-safe: ${sectionId}`
  )
}

const defaultSection = sectionRegistry.match(
  /MCP_DEFAULT_SECTION:\s*MCPSectionId\s*=\s*'([^']+)'/
)?.[1]
assert.ok(defaultSection, 'MCP_DEFAULT_SECTION is missing')
assert.ok(
  sectionIds.includes(defaultSection),
  `MCP_DEFAULT_SECTION points to unknown section: ${defaultSection}`
)
assertIncludes(
  sectionRegistry,
  'url: `/mcp/${section.id}`',
  'MCP section nav URL builder'
)

console.log(
  `MCP route smoke passed: ${sectionIds.length} sections, default=${defaultSection}`
)
