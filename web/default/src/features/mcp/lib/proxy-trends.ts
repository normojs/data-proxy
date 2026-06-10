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
  MCPProxyServerCallHealth,
  MCPProxyTrendBucket,
  MCPProxyTrendResponse,
  MCPProxyTrendServerDimension,
  MCPProxyTrendToolDimension,
} from '../types'

type PartialTrendBucket = Partial<MCPProxyTrendBucket>
type PartialTrendServerDimension = Partial<MCPProxyTrendServerDimension>
type PartialTrendToolDimension = Partial<MCPProxyTrendToolDimension>

type PartialTrendResponse = Partial<
  Omit<MCPProxyTrendResponse, 'totals' | 'buckets' | 'servers' | 'tools'>
> & {
  totals?: Partial<MCPProxyServerCallHealth> | null
  buckets?: Array<PartialTrendBucket | null> | null
  servers?: Array<PartialTrendServerDimension | null> | null
  tools?: Array<PartialTrendToolDimension | null> | null
}

function finiteNumber(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function textValue(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function definedItems<T>(items: Array<T | null> | null | undefined): T[] {
  return Array.isArray(items)
    ? items.filter((item): item is T => item != null)
    : []
}

function successRate(
  value: Partial<MCPProxyServerCallHealth> | null | undefined
) {
  if (
    typeof value?.success_rate === 'number' &&
    Number.isFinite(value.success_rate)
  ) {
    return value.success_rate
  }
  const totalCalls = finiteNumber(value?.total_calls)
  if (totalCalls <= 0) return 0
  return (finiteNumber(value?.success_calls) / totalCalls) * 100
}

export function normalizeMCPProxyCallHealth(
  value: Partial<MCPProxyServerCallHealth> | null | undefined
): MCPProxyServerCallHealth {
  return {
    total_calls: finiteNumber(value?.total_calls),
    success_calls: finiteNumber(value?.success_calls),
    error_calls: finiteNumber(value?.error_calls),
    timeout_calls: finiteNumber(value?.timeout_calls),
    pending_calls: finiteNumber(value?.pending_calls),
    settled_calls: finiteNumber(value?.settled_calls),
    unsettled: finiteNumber(value?.unsettled),
    free_calls: finiteNumber(value?.free_calls),
    quota: finiteNumber(value?.quota),
    cost: finiteNumber(value?.cost),
    result_size: finiteNumber(value?.result_size),
    avg_duration_ms: finiteNumber(value?.avg_duration_ms),
    success_rate: successRate(value),
  }
}

export function normalizeMCPProxyTrendResponse(
  value: PartialTrendResponse | null | undefined
): MCPProxyTrendResponse {
  return {
    start_time: finiteNumber(value?.start_time),
    end_time: finiteNumber(value?.end_time),
    bucket_seconds: finiteNumber(value?.bucket_seconds),
    checked_at: finiteNumber(value?.checked_at),
    totals: normalizeMCPProxyCallHealth(value?.totals),
    buckets: definedItems(value?.buckets).map((bucket) => ({
      ...normalizeMCPProxyCallHealth(bucket),
      bucket_start: finiteNumber(bucket.bucket_start),
    })),
    servers: definedItems(value?.servers).map((server) => ({
      ...normalizeMCPProxyCallHealth(server),
      proxy_server_id: finiteNumber(server.proxy_server_id),
      name: textValue(server.name),
      namespace: textValue(server.namespace),
    })),
    tools: definedItems(value?.tools).map((tool) => ({
      ...normalizeMCPProxyCallHealth(tool),
      proxy_server_id: finiteNumber(tool.proxy_server_id),
      proxy_tool_id: finiteNumber(tool.proxy_tool_id),
      tool_id: finiteNumber(tool.tool_id),
      exposed_tool_name: textValue(tool.exposed_tool_name),
      downstream_tool_name: textValue(tool.downstream_tool_name),
    })),
  }
}
