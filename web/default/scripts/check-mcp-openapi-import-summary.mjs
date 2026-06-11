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
import {
  normalizeOpenAPISchemaMetrics,
  summarizeOpenAPIImportResult,
} from '../src/features/mcp/lib/openapi-import-summary.ts'

assert.deepEqual(normalizeOpenAPISchemaMetrics(undefined), {
  operation_count: 0,
  imported_tool_count: 0,
  schema_count: 0,
  unique_schema_count: 0,
  reused_schema_count: 0,
})

assert.deepEqual(
  normalizeOpenAPISchemaMetrics({
    operation_count: 3.8,
    imported_tool_count: 2,
    schema_count: -1,
    unique_schema_count: 4,
    reused_schema_count: Number.NaN,
  }),
  {
    operation_count: 3,
    imported_tool_count: 2,
    schema_count: 0,
    unique_schema_count: 4,
    reused_schema_count: 0,
  }
)

const summary = summarizeOpenAPIImportResult({
  imported_count: 2,
  updated_count: 2,
  skipped_count: 1,
  imported: [{ operation_key: 'GET /pets' }, { operation_key: 'POST /pets' }],
  updated: [
    {
      operation_key: 'PATCH /pets/{id}',
      changes: [
        { field: 'description', previous: 'old', current: 'new' },
        { field: 'input_schema', previous: 'sha256:old', current: 'sha256:new' },
      ],
    },
    { operation_key: 'GET /pets/{id}', changes: [] },
  ],
  skipped: ['DELETE /pets/{id}: tool name pet_api.delete already exists'],
})
assert.equal(summary.affectedToolCount, 4)
assert.equal(summary.changedToolCount, 1)
assert.equal(summary.changedFieldCount, 2)
assert.equal(summary.skippedReasons.length, 1)

const partialSummary = summarizeOpenAPIImportResult({
  imported: [{ operation_key: 'GET /status' }],
  updated: [{ operation_key: 'GET /health', changes: [{}] }],
  skipped: [null, '  ', 'GET /legacy: duplicate'],
})
assert.equal(partialSummary.importedCount, 1)
assert.equal(partialSummary.updatedCount, 1)
assert.equal(partialSummary.skippedCount, 1)
assert.equal(partialSummary.changedFieldCount, 1)

console.log('MCP OpenAPI import summary smoke passed')
