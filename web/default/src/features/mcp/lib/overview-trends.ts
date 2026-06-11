import type {
  MCPSummaryBillingAnomalies,
  MCPSummaryBridgeTrendBucket,
  MCPSummaryOpenAPIStorageBucket,
  MCPSummaryOperationsTrends,
  MCPSummaryProxyErrorTool,
} from '../types'

function finiteNumber(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function objectFromAny(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  return value as Record<string, unknown>
}

function arrayFromAny(value: unknown): unknown[] {
  return Array.isArray(value) ? value : []
}

export function normalizeMCPSummaryBillingAnomalies(
  value: unknown
): MCPSummaryBillingAnomalies {
  const object = objectFromAny(value)
  return {
    unsettled_success_calls: finiteNumber(object.unsettled_success_calls),
    failed_charged_calls: finiteNumber(object.failed_charged_calls),
    missing_debit_events: finiteNumber(object.missing_debit_events),
    refund_events: finiteNumber(object.refund_events),
    refund_quota: finiteNumber(object.refund_quota),
    net_mcp_quota_delta: finiteNumber(object.net_mcp_quota_delta),
  }
}

export function normalizeMCPSummaryOperationsTrends(
  value: unknown
): MCPSummaryOperationsTrends {
  const object = objectFromAny(value)
  return {
    start_time: finiteNumber(object.start_time),
    end_time: finiteNumber(object.end_time),
    bucket_seconds: finiteNumber(object.bucket_seconds),
    checked_at: finiteNumber(object.checked_at),
    bridge_online: arrayFromAny(object.bridge_online)
      .map(normalizeBridgeBucket)
      .filter((bucket) => bucket.bucket_start > 0),
    openapi_storage: arrayFromAny(object.openapi_storage)
      .map(normalizeOpenAPIStorageBucket)
      .filter((bucket) => bucket.bucket_start > 0),
    proxy_error_top_n: arrayFromAny(object.proxy_error_top_n)
      .map(normalizeProxyErrorTool)
      .filter((tool) => tool.tool_name || tool.downstream_tool_name),
    billing_anomalies: normalizeMCPSummaryBillingAnomalies(
      object.billing_anomalies
    ),
  }
}

function normalizeBridgeBucket(value: unknown): MCPSummaryBridgeTrendBucket {
  const object = objectFromAny(value)
  return {
    bucket_start: finiteNumber(object.bucket_start),
    online_clients: finiteNumber(object.online_clients),
    started_sessions: finiteNumber(object.started_sessions),
    closed_sessions: finiteNumber(object.closed_sessions),
  }
}

function normalizeOpenAPIStorageBucket(
  value: unknown
): MCPSummaryOpenAPIStorageBucket {
  const object = objectFromAny(value)
  return {
    bucket_start: finiteNumber(object.bucket_start),
    object_count: finiteNumber(object.object_count),
    total_bytes: finiteNumber(object.total_bytes),
    expired_count: finiteNumber(object.expired_count),
    download_count: finiteNumber(object.download_count),
  }
}

function normalizeProxyErrorTool(value: unknown): MCPSummaryProxyErrorTool {
  const object = objectFromAny(value)
  return {
    proxy_server_id: finiteNumber(object.proxy_server_id),
    proxy_tool_id: finiteNumber(object.proxy_tool_id),
    tool_id: finiteNumber(object.tool_id),
    tool_name: String(object.tool_name || ''),
    downstream_tool_name: String(object.downstream_tool_name || ''),
    total_calls: finiteNumber(object.total_calls),
    success_calls: finiteNumber(object.success_calls),
    error_calls: finiteNumber(object.error_calls),
    timeout_calls: finiteNumber(object.timeout_calls),
    success_rate: finiteNumber(object.success_rate),
    avg_duration_ms: finiteNumber(object.avg_duration_ms),
  }
}
