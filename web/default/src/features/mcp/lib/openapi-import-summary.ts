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
import type {
  MCPOpenAPIImportItem,
  MCPOpenAPIImportResponse,
  MCPOpenAPISchemaMetrics,
} from '../types'

type PartialImportItem = Partial<MCPOpenAPIImportItem> | null | undefined

export type PartialOpenAPIImportResponse = Partial<
  Omit<MCPOpenAPIImportResponse, 'imported' | 'updated' | 'skipped'>
> & {
  imported?: PartialImportItem[] | null
  updated?: PartialImportItem[] | null
  skipped?: Array<string | null | undefined> | null
}

export type OpenAPIImportResultSummary = {
  affectedToolCount: number
  changedFieldCount: number
  changedToolCount: number
  importedCount: number
  skippedCount: number
  skippedReasons: string[]
  updatedCount: number
}

function finiteCount(value: unknown, fallback = 0): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) return fallback
  return Math.max(0, Math.trunc(value))
}

function definedItems<T>(items: Array<T | null | undefined> | null | undefined) {
  return Array.isArray(items)
    ? items.filter((item): item is T => item != null)
    : []
}

function changeCount(item: PartialImportItem): number {
  return Array.isArray(item?.changes) ? item.changes.length : 0
}

export function normalizeOpenAPISchemaMetrics(
  value: Partial<MCPOpenAPISchemaMetrics> | null | undefined
): MCPOpenAPISchemaMetrics {
  return {
    operation_count: finiteCount(value?.operation_count),
    imported_tool_count: finiteCount(value?.imported_tool_count),
    schema_count: finiteCount(value?.schema_count),
    unique_schema_count: finiteCount(value?.unique_schema_count),
    reused_schema_count: finiteCount(value?.reused_schema_count),
  }
}

export function summarizeOpenAPIImportResult(
  value: PartialOpenAPIImportResponse | null | undefined
): OpenAPIImportResultSummary {
  const imported = definedItems(value?.imported)
  const updated = definedItems(value?.updated)
  const skippedReasons = definedItems(value?.skipped).filter(
    (item) => item.trim().length > 0
  )
  const changedFieldCount = updated.reduce(
    (total, item) => total + changeCount(item),
    0
  )
  const changedToolCount = updated.filter((item) => changeCount(item) > 0).length
  const importedCount = finiteCount(value?.imported_count, imported.length)
  const updatedCount = finiteCount(value?.updated_count, updated.length)
  const skippedCount = finiteCount(value?.skipped_count, skippedReasons.length)

  return {
    affectedToolCount: importedCount + updatedCount,
    changedFieldCount,
    changedToolCount,
    importedCount,
    skippedCount,
    skippedReasons,
    updatedCount,
  }
}
