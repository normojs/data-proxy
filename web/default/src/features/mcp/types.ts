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

export type ApiResponse<T> = {
  success: boolean
  message?: string
  data?: T
}

export type PaginatedData<T> = {
  items: T[]
  page: number
  page_size: number
  total: number
}

export type PaginatedResponse<T> = ApiResponse<PaginatedData<T>>

export type MCPTool = {
  id: number
  name: string
  display_name: string
  description: string
  category: string
  source: string
  plugin_id?: number | null
  openapi_url: string
  input_schema: unknown
  price_per_call: number
  price_unit: string
  free_quota: number
  is_remote: boolean
  status: number
  sort_order: number
  created_at: number
  updated_at: number
}

export type MCPToolUpdatePayload = Partial<{
  display_name: string
  description: string
  category: string
  price_per_call: number
  price_unit: string
  free_quota: number
  status: number
  sort_order: number
}>

export type MCPToolCreatePayload = {
  name: string
  display_name: string
  description: string
  category: string
  input_schema: unknown
  price_per_call: number
  price_unit: string
  free_quota: number
  status: number
  sort_order: number
}

export type MCPOpenAPIPreviewOperation = {
  key: string
  operation_id: string
  method: string
  path: string
  summary: string
  description: string
  tool_name: string
  input_schema: Record<string, unknown>
  has_request_body: boolean
}

export type MCPOpenAPIPreviewPayload = Partial<{
  openapi_url: string
  document: unknown
  namespace: string
  category: string
}>

export type MCPOpenAPISchemaMetrics = {
  operation_count: number
  imported_tool_count: number
  schema_count: number
  unique_schema_count: number
  reused_schema_count: number
}

export type MCPOpenAPIPreviewResponse = {
  openapi_url: string
  title: string
  version: string
  server_url: string
  namespace: string
  category: string
  schema_metrics: MCPOpenAPISchemaMetrics
  operations: MCPOpenAPIPreviewOperation[]
}

export type MCPOpenAPIImportPayload = MCPOpenAPIPreviewPayload &
  Partial<{
    selected_operations: string[]
    update_existing: boolean
    auth_type: string
    auth_ref: string
    auth_header_name: string
    price_per_call: number
    price_unit: string
    free_quota: number
    status: number
    sort_order: number
  }>

export type MCPOpenAPILifecyclePayload = Partial<{
  openapi_url: string
  tool_ids: number[]
}>

export type MCPOpenAPIImportItem = {
  operation_key: string
  tool: MCPTool
  changes?: MCPOpenAPIImportChange[]
}

export type MCPOpenAPIImportChange = {
  field: string
  previous: string
  current: string
}

export type MCPOpenAPIImportResponse = {
  openapi_url: string
  imported_count: number
  updated_count: number
  skipped_count: number
  imported: MCPOpenAPIImportItem[]
  updated: MCPOpenAPIImportItem[]
  skipped: string[]
}

export type MCPOpenAPIBinaryObject = {
  id: number
  object_id: string
  provider: string
  content_type: string
  content_family: string
  sha256: string
  size: number
  filename: string
  disposition: string
  mcp_tool_call_id: number
  mcp_tool_id: number
  openapi_tool_id: number
  user_id: number
  token_id: number
  request_id: string
  operation_key: string
  expires_at: number
  expiry_status: string
  download_count: number
  last_downloaded_at: number
  download_url: string
  created_at: number
  updated_at: number
}

export type MCPOpenAPIBinaryObjectSummary = {
  total_count: number
  total_bytes: number
  active_count: number
  expired_count: number
  no_expiry_count: number
  downloaded_count: number
  download_count: number
  default_ttl_seconds: number
  default_cleanup_limit: number
  checked_at: number
}

export type MCPOpenAPIBinaryCleanupPayload = Partial<{
  ttl_seconds: number
  limit: number
  dry_run: boolean
}>

export type MCPOpenAPIBinaryCleanupResponse = {
  provider: string
  ttl_seconds: number
  cutoff_time: number
  dry_run: boolean
  scanned: number
  deleted: number
  deleted_bytes: number
  deleted_object_ids?: string[]
  registry_deleted: number
  errors?: string[]
}

export type MCPOpenAPILifecycleResponse = {
  openapi_url: string
  affected_count: number
  tools: MCPTool[]
}

export type MCPProxyServer = {
  id: number
  name: string
  namespace: string
  transport: string
  endpoint: string
  command?: string
  auth_type: string
  auth_ref?: string
  timeout_ms: number
  max_result_size: number
  max_metadata_size: number
  visibility: string
  allowed_groups: string[]
  status: string
  last_error: string
  last_discovered_at: number
  created_at: number
  updated_at: number
  archived: boolean
  health?: MCPProxyServerListHealth
}

export type MCPProxyServerPayload = Partial<{
  name: string
  namespace: string
  transport: string
  endpoint: string
  command: string
  auth_type: string
  auth_ref: string
  timeout_ms: number
  max_result_size: number
  max_metadata_size: number
  visibility: string
  allowed_groups: string[]
  status: string
}>

export type MCPProxyServerTestResult = {
  proxy_server_id: number
  protocol_version: string
  server_name: string
  capabilities: Record<string, unknown>
}

export type MCPProxyDiscoveryResult = {
  proxy_server_id: number
  discovered_count: number
  created_count: number
  updated_count: number
  disabled_count: number
  schema_changed: number
  tools: MCPProxyTool[]
}

export type MCPProxyDiscoveryEvent = {
  id: number
  proxy_server_id: number
  event_type: string
  status: string
  message: string
  protocol_version: string
  server_name: string
  capabilities: Record<string, unknown>
  discovered_count: number
  created_count: number
  updated_count: number
  disabled_count: number
  schema_changed: number
  duration_ms: number
  started_at: number
  finished_at: number
  created_at: number
}

export type MCPProxyHealthCheckSettings = {
  enabled: boolean
  interval_minutes: number
  limit: number
  stale_seconds: number
  discover: boolean
  max_discover_tools: number
}

export type MCPProxyHealthCheckRunPayload = Partial<{
  dry_run: boolean
  force: boolean
  discover: boolean
  limit: number
  stale_seconds: number
  max_discover_tools: number
}>

export type MCPProxyHealthCheckRunItem = {
  proxy_server_id: number
  name: string
  namespace: string
  status: string
  action: string
  skipped_reason?: string
  previous_check?: MCPProxyDiscoveryEvent
  latest_check?: MCPProxyDiscoveryEvent
  error?: string
  discovered: number
  schema_changed: number
}

export type MCPProxyHealthCheckRunResponse = {
  manual: boolean
  dry_run: boolean
  status: string
  message: string
  settings: MCPProxyHealthCheckSettings
  checked_at: number
  scanned_count: number
  checked_count: number
  skipped_count: number
  success_count: number
  error_count: number
  discover_count: number
  blocked_count: number
  items: MCPProxyHealthCheckRunItem[]
  latest_event_by_id?: Record<string, MCPProxyDiscoveryEvent>
}

export type MCPProxyHealthCheckStatus = {
  settings: MCPProxyHealthCheckSettings
  running: boolean
  last_run_at: number
  last_run_status: string
  last_run_message: string
  last_run?: MCPProxyHealthCheckRunResponse
}

export type MCPProxyHeartbeatSettings = {
  enabled: boolean
  interval_seconds: number
  limit: number
  active_window_seconds: number
  timeout_seconds: number
}

export type MCPProxyHeartbeatRunItem = {
  proxy_server_id: number
  name: string
  namespace: string
  transport: string
  action: string
  skipped_reason?: string
  error?: string
  last_activity_at?: number
}

export type MCPProxyHeartbeatRunResponse = {
  manual: boolean
  status: string
  message: string
  settings: MCPProxyHeartbeatSettings
  checked_at: number
  scanned_count: number
  pinged_count: number
  skipped_count: number
  success_count: number
  error_count: number
  items: MCPProxyHeartbeatRunItem[]
}

export type MCPProxyHeartbeatStatus = {
  settings: MCPProxyHeartbeatSettings
  running: boolean
  last_run_at: number
  last_run_status: string
  last_run_message: string
  last_run?: MCPProxyHeartbeatRunResponse
}

export type MCPProxyServerHealth = {
  proxy_server_id: number
  window_seconds: number
  generated_at: number
  needs_review: boolean
  review_reasons: string[]
  calls: MCPProxyServerCallHealth
  discovery: MCPProxyServerDiscoveryInfo
  transport: MCPProxyTransportHealth
  top_tools: MCPProxyServerToolHealth[]
  recent_errors: MCPProxyServerRecentError[]
  latest_check?: MCPProxyDiscoveryEvent
}

export type MCPProxyServerListHealth = {
  window_seconds: number
  generated_at: number
  needs_review: boolean
  review_reasons: string[]
  calls: MCPProxyServerCallHealth
  discovery: MCPProxyServerDiscoveryInfo
  top_tool?: MCPProxyServerToolHealth
  latest_error?: MCPProxyServerRecentError
  latest_check?: MCPProxyDiscoveryEvent
}

export type MCPProxyServerCallHealth = {
  total_calls: number
  success_calls: number
  error_calls: number
  timeout_calls: number
  pending_calls: number
  settled_calls: number
  unsettled: number
  free_calls: number
  quota: number
  cost: number
  result_size: number
  avg_duration_ms: number
  success_rate: number
}

export type MCPProxyTrendBucket = MCPProxyServerCallHealth & {
  bucket_start: number
}

export type MCPProxyTrendServerDimension = MCPProxyServerCallHealth & {
  proxy_server_id: number
  name: string
  namespace: string
}

export type MCPProxyTrendToolDimension = MCPProxyServerCallHealth & {
  proxy_server_id: number
  proxy_tool_id: number
  tool_id: number
  exposed_tool_name: string
  downstream_tool_name: string
}

export type MCPProxyTrendResponse = {
  start_time: number
  end_time: number
  bucket_seconds: number
  checked_at: number
  totals: MCPProxyServerCallHealth
  buckets: MCPProxyTrendBucket[]
  servers: MCPProxyTrendServerDimension[]
  tools: MCPProxyTrendToolDimension[]
}

export type MCPProxyTrendParams = Partial<{
  proxy_server_id: number
  proxy_tool_id: number
  status: string
  start_time: number
  end_time: number
  bucket_seconds: number
}>

export type MCPProxyServerDiscoveryInfo = {
  total_tools: number
  enabled_tools: number
  pending_tools: number
  disabled_tools: number
  schema_changed_tools: number
  error_tools: number
  last_discovered_at: number
  last_tool_updated_at: number
}

export type MCPProxyTransportHealth = {
  transport: string
  has_session: boolean
  initialized: boolean
  message_endpoint?: string
  last_error?: string
  streamable_session: boolean
  sse_connected: boolean
  active_sessions: number
  pending_requests: number
  last_activity_at?: number
  observable: boolean
}

export type MCPProxyServerToolHealth = {
  tool_id: number
  proxy_tool_id: number
  exposed_tool_name: string
  downstream_tool_name: string
  status: string
  calls: number
  success_calls: number
  error_calls: number
  timeout_calls: number
  quota: number
  cost: number
  avg_duration_ms: number
  success_rate: number
}

export type MCPProxyServerRecentError = {
  id: number
  request_id: string
  tool_name: string
  status: string
  error_code: string
  error_message: string
  duration_ms: number
  created_at: number
}

export type MCPProxyTool = {
  id: number
  proxy_server_id: number
  mcp_tool_id: number
  downstream_tool_name: string
  downstream_title: string
  downstream_description: string
  downstream_input_schema: unknown
  exposed_tool_name: string
  exposed_description: string
  schema_hash: string
  status: string
  price_per_call: number
  price_unit: string
  free_quota: number
  sort_order: number
  last_error: string
  last_discovered_at: number
  created_at: number
  updated_at: number
}

export type MCPProxyToolHealth = {
  proxy_tool_id: number
  proxy_server_id: number
  mcp_tool_id: number
  window_seconds: number
  generated_at: number
  calls: MCPProxyServerCallHealth
  tool: MCPProxyServerToolHealth
  recent_errors: MCPProxyServerRecentError[]
}

export type MCPProxyToolUpdatePayload = Partial<{
  exposed_tool_name: string
  exposed_description: string
  status: string
  price_per_call: number
  price_unit: string
  free_quota: number
  sort_order: number
}>

export type MCPToolCall = {
  id: number
  user_id: number
  token_id: number
  tool_id: number
  tool_name: string
  request_id: string
  request_params: string
  request_ip: string
  status: string
  result_summary: string
  error_code: string
  error_message: string
  metadata: string
  duration_ms: number
  result_size: number
  bridge_session_id: string
  target_client: string
  cost: number
  quota: number
  free_used: boolean
  settled_at: number
  created_at: number
}

export type BridgeClient = {
  id: number
  client_id: string
  user_id: number
  token_id: number
  name: string
  version: string
  platform: string
  workspace: string
  capabilities: string[]
  policy: BridgeClientPolicy
  status: number
  online: boolean
  session_id?: string
  last_seen_at: number
  created_at: number
  updated_at: number
}

export type BridgeClientPolicy = {
  allowed_tools?: string[]
  allow_write: boolean
  max_result_bytes?: number
  max_scan_file_bytes?: number
  max_results?: number
  tree_depth?: number
  walk_depth?: number
  mcp_allowed_targets?: string[]
  http_allowed_targets?: string[]
  http_denied_targets?: string[]
  http_denied_ports?: number[]
}

export type BridgeClientUpdatePayload = Partial<{
  name: string
  version: string
  platform: string
  workspace: string
  capabilities: string[]
  status: number
  policy: BridgeClientPolicy
}>

export type BridgeAgentSetupPayload = Partial<{
  client_id: string
  rotate: boolean
  client_name: string
  version: string
  platform: string
  workspace: string
}>

export type BridgeAgentSetupResponse = {
  client: BridgeClient
  base_url: string
  bridge_ws_url: string
  client_id: string
  api_key?: string
  api_key_once: boolean
  token_id: number
  token_name: string
  token_masked_key: string
  created: boolean
  rotated: boolean
  headers: Record<string, string>
  register: Record<string, unknown>
  environment: Record<string, string>
  config: Record<string, unknown>
}

export type BridgeAgentSetupTokenPayload = Partial<{
  client_id: string
  rotate: boolean
  client_name: string
  version: string
  platform: string
  workspace: string
  ttl_seconds: number
}>

export type BridgeAgentSetupTokenResponse = {
  setup_token: string
  expires_at: number
  expires_in_seconds: number
  client_id?: string
  enroll_command: string
  install_command: string
  full_command: string
}

export type TunnelApp = {
  id: number
  user_id: number
  name: string
  description: string
  app_type: 'mcp_code' | 'http_tunnel' | 'tcp_tunnel' | string
  permission_mode: string
  status: string
  public_slug: string
  bridge_client_id: string
  target_host: string
  target_port: number
  target_path: string
  policy?: Record<string, unknown>
  route?: Record<string, unknown>
  billing?: Record<string, unknown>
  approved_by: number
  approved_at: number
  review_note: string
  last_error: string
  last_seen_at: number
  created_at: number
  updated_at: number
}

export type TunnelAppListParams = Partial<{
  p: number
  page_size: number
  user_id: number
  app_type: string
  status: string
  keyword: string
}>

export type TunnelAppCreatePayload = Partial<{
  name: string
  description: string
  app_type: string
  permission_mode: string
  bridge_client_id: string
  target_host: string
  target_port: number
  target_path: string
  policy: Record<string, unknown>
  route: Record<string, unknown>
  billing: Record<string, unknown>
}>

export type TunnelAppAdminUpdatePayload = Partial<{
  name: string
  description: string
  permission_mode: string
  status: string
  bridge_client_id: string
  target_host: string
  target_port: number
  target_path: string
  policy: Record<string, unknown>
  route: Record<string, unknown>
  billing: Record<string, unknown>
  review_note: string
}>

export type TunnelConnection = {
  id: number
  app_id: number
  user_id: number
  agent_token_id: number
  name: string
  key_prefix: string
  permission_mode: string
  status: string
  endpoint_path: string
  config?: Record<string, unknown>
  expires_at: number
  last_used_at: number
  last_request_id: string
  revoked_at: number
  created_at: number
  updated_at: number
}

export type TunnelConnectionListParams = Partial<{
  p: number
  page_size: number
  status: string
  keyword: string
}>

export type TunnelSession = {
  id: number
  app_id: number
  user_id: number
  connection_id: number
  connection_name: string
  key_prefix: string
  session_id: string
  bridge_client_id: string
  status: string
  client_version: string
  client_ip: string
  user_agent: string
  bytes_in: number
  bytes_out: number
  connected_at: number
  last_seen_at: number
  disconnected_at: number
  close_reason: string
  created_at: number
  updated_at: number
}

export type TunnelSessionListParams = Partial<{
  p: number
  page_size: number
  connection_id: number
  status: string
  session_id: string
  keyword: string
  start_time: number
  end_time: number
}>

export type TunnelConnectionCreatePayload = Partial<{
  name: string
  permission_mode: string
  expires_at: number
  config: Record<string, unknown>
}>

export type TunnelConnectionUpdatePayload = Partial<{
  name: string
  expires_at: number
  config: Record<string, unknown>
}>

export type TunnelConnectionCreateResponse = {
  connection: TunnelConnection
  connection_key: string
  endpoint_path: string
}

export type TunnelAgentSetupPayload = Partial<{
  connection_id: number
  rotate: boolean
  client_name: string
  platform: string
  workspace: string
  version: string
}>

export type TunnelAgentSetupResponse = {
  app: TunnelApp
  connection: TunnelConnection
  base_url: string
  bridge_ws_url: string
  mcp_url: string
  client_id: string
  api_key?: string
  api_key_once: boolean
  token_id: number
  token_name: string
  token_masked_key: string
  created: boolean
  rotated: boolean
  headers: Record<string, string>
  register: Record<string, unknown>
  environment: Record<string, string>
  config: Record<string, unknown>
}

export type TunnelAuditLog = {
  id: number
  app_id: number
  connection_id: number
  connection_key_prefix: string
  session_id: string
  user_id: number
  actor_user_id: number
  action: string
  decision: string
  reason: string
  request_id: string
  tool_name: string
  method: string
  path: string
  bytes_in: number
  bytes_out: number
  duration_ms: number
  metadata?: Record<string, unknown>
  created_at: number
}

export type TunnelAuditLogListParams = Partial<{
  p: number
  page_size: number
  connection_id: number
  action: string
  decision: string
  request_id: string
  tool_name: string
  session_id: string
  keyword: string
  start_time: number
  end_time: number
}>

export type BridgeSessionSnapshot = {
  session_id: string
  client_id: string
  user_id: number
  token_id: number
  name: string
  version: string
  platform: string
  workspace: string
  capabilities: string[]
  connected_at: number
  last_seen_at: number
}

export type BridgeSession = {
  id: number
  session_id: string
  client_id: string
  user_id: number
  token_id: number
  request_ip: string
  user_agent: string
  status: string
  connected_at: number
  last_ping_at: number
  closed_at: number
  close_reason: string
  created_at: number
  updated_at: number
}

export type BridgeClientDetail = {
  client: BridgeClient
  online_session?: BridgeSessionSnapshot
  recent_sessions: BridgeSession[]
}

export type BridgeAuditHealth = {
  total_requests: number
  success: number
  error: number
  timeout: number
  pending: number
  result_size: number
  avg_duration_ms: number
  success_rate: number
}

export type BridgeRecentError = {
  id: number
  request_id: string
  session_id: string
  client_id: string
  tool_name: string
  status: string
  error_code: string
  error_message: string
  duration_ms: number
  created_at: number
}

export type BridgeClientHealth = {
  client_id: string
  window_seconds: number
  generated_at: number
  online: boolean
  online_session?: BridgeSessionSnapshot
  calls: BridgeAuditHealth
  recent_errors: BridgeRecentError[]
  recent_sessions: BridgeSession[]
}

export type BridgeAuditLog = {
  id: number
  request_id: string
  session_id: string
  client_id: string
  user_id: number
  token_id: number
  tool_name: string
  request_body: string
  status: string
  error_code: string
  error_message: string
  duration_ms: number
  result_size: number
  created_at: number
  updated_at: number
}

export type BillingEvent = {
  id: number
  event_id: string
  user_id: number
  token_id: number
  source: string
  source_id: string
  event_type: string
  status: string
  request_id: string
  group: string
  billing_source: string
  price_unit: string
  currency: string
  amount_quota: number
  quota_delta: number
  cost: number
  metadata: string
  created_at: number
  related_audit_events?: BillingEventAuditLink[]
  related_target_event?: BillingEventTargetLink
  related_mcp_tool_call?: BillingEventMCPToolCallLink
}

export type BillingEventAuditLink = {
  id: number
  event_id: string
  source_id: string
  price_unit: string
  reason: string
  label: string
  admin_id: number
  created_at: number
}

export type BillingEventTargetLink = {
  id: number
  event_id: string
  user_id: number
  source: string
  source_id: string
  event_type: string
  status: string
  price_unit: string
  amount_quota: number
  quota_delta: number
  created_at: number
}

export type BillingEventMCPToolCallLink = {
  id: number
  tool_id: number
  tool_name: string
  request_id: string
  status: string
  error_code: string
  error_message: string
  metadata: string
  bridge_session_id: string
  target_client: string
  duration_ms: number
  result_size: number
  created_at: number
}

export type BillingEventBackfillRequest = {
  sources: string[]
  limit: number
  dry_run: boolean
}

export type BillingEventBackfillSourceResult = {
  source: string
  scanned: number
  created: number
  would_create: number
  skipped_existing: number
  skipped_invalid: number
  error_count: number
  errors: string[]
}

export type BillingEventBackfillResponse = {
  dry_run: boolean
  limit: number
  sources: string[]
  results: BillingEventBackfillSourceResult[]
  total_scanned: number
  total_created: number
  total_would_create: number
  total_skipped_existing: number
  total_skipped_invalid: number
  total_error_count: number
}

export type BillingEventSourceCapabilityItem = {
  source: string
  event_source: string
  label: string
  status: string
  backfill_sources: string[]
  supports_recording: boolean
  supports_backfill: boolean
  supports_reconciliation: boolean
  supports_missing_backfill: boolean
  supports_refund_or_delta: boolean
  supports_repair_audit: boolean
  supports_audit_relation: boolean
  requires_durable_source_log: boolean
  notes: string[]
}

export type BillingEventSourceMatrix = {
  checked_at: number
  items: BillingEventSourceCapabilityItem[]
  total_sources: number
  ready_sources: number
  record_only_sources: number
  planned_sources: number
  audit_only_sources: number
}

export type BillingEventHealth = {
  limit: number
  sources: string[]
  checked_at: number
  needs_review: boolean
  total_would_create: number
  total_missing: number
  total_mismatched: number
  total_invalid: number
  total_error_count: number
  backfill: BillingEventBackfillResponse
  reconciliation: BillingEventReconciliationResponse
}

export type BillingEventSummaryAggregate = {
  total_events: number
  credit_events: number
  debit_events: number
  audit_events: number
  amount_quota: number
  net_quota_delta: number
  credit_quota_delta: number
  debit_quota_delta: number
  total_cost: number
}

export type BillingEventSummaryDimension = BillingEventSummaryAggregate & {
  key: string
}

export type BillingEventSummaryBucket = BillingEventSummaryAggregate & {
  bucket_start: number
}

export type BillingEventSummary = {
  start_time: number
  end_time: number
  bucket_seconds: number
  checked_at: number
  totals: BillingEventSummaryAggregate
  by_source: BillingEventSummaryDimension[]
  by_type: BillingEventSummaryDimension[]
  daily_trend: BillingEventSummaryBucket[]
}

export type BillingEventRelationMaintenanceItem = {
  audit_event_id: number
  audit_event: string
  target_event_id: number
  relation_type: string
  reason: string
  label: string
  admin_id: number
  error?: string
}

export type BillingEventRelationHealth = {
  limit: number
  cursor: number
  checked_at: number
  total_audit_events: number
  total_relations: number
  scanned_audit_events: number
  missing_relations: number
  invalid_audit_events: number
  orphan_source_relations: number
  orphan_target_relations: number
  needs_review: boolean
  has_more: boolean
  scan_complete: boolean
  next_cursor: number
  sample_missing_relations: BillingEventRelationMaintenanceItem[]
  sample_invalid_audits: BillingEventRelationMaintenanceItem[]
}

export type BillingEventRelationBackfillRequest = {
  limit: number
  cursor: number
  dry_run: boolean
}

export type BillingEventRelationBackfillResponse = {
  dry_run: boolean
  limit: number
  cursor: number
  scanned_audit_events: number
  created: number
  would_create: number
  skipped_existing: number
  skipped_invalid: number
  error_count: number
  has_more: boolean
  scan_complete: boolean
  next_cursor: number
  items: BillingEventRelationMaintenanceItem[]
  errors: string[]
}

export type BillingEventRelationRepairRequest = {
  dry_run: boolean
  items: BillingEventRelationMaintenanceItem[]
}

export type BillingEventRelationRepairResponse = {
  dry_run: boolean
  selected: number
  created: number
  would_create: number
  skipped_existing: number
  skipped_invalid: number
  error_count: number
  items: BillingEventRelationMaintenanceItem[]
  errors: string[]
}

export type BillingEventRelationOrphanCleanupRequest = {
  dry_run: boolean
}

export type BillingEventRelationOrphanCleanupResponse = {
  dry_run: boolean
  source_orphans: number
  target_orphans: number
  would_delete: number
  deleted: number
}

export type BillingEventRelationInspectionSettings = {
  enabled: boolean
  interval_minutes: number
  limit: number
  auto_backfill: boolean
  auto_cleanup_orphans: boolean
  max_auto_backfill: number
  max_auto_cleanup_orphans: number
  cursor: number
}

export type BillingEventRelationInspectionSettingsRequest = {
  enabled: boolean
  interval_minutes: number
  limit: number
  auto_backfill: boolean
  auto_cleanup_orphans: boolean
  max_auto_backfill: number
  max_auto_cleanup_orphans: number
  cursor?: number
}

export type BillingEventRelationInspectionStatus = {
  settings: BillingEventRelationInspectionSettings
  running: boolean
  last_run_at: number
  last_run_status: string
  last_run_message: string
  last_health?: BillingEventRelationHealth
  last_backfill?: BillingEventRelationBackfillResponse
  last_cleanup?: BillingEventRelationOrphanCleanupResponse
  recent_runs: BillingEventRelationInspectionRunItem[]
}

export type BillingEventRelationInspectionRunResponse = {
  manual: boolean
  status: string
  message: string
  settings: BillingEventRelationInspectionSettings
  health: BillingEventRelationHealth
  backfill?: BillingEventRelationBackfillResponse
  cleanup?: BillingEventRelationOrphanCleanupResponse
  run?: BillingEventRelationInspectionRunItem
}

export type BillingEventRelationInspectionRunListParams = {
  p?: number
  page_size?: number
}

export type BillingEventRelationInspectionRunItem = {
  id: number
  trigger: string
  status: string
  message: string
  limit: number
  cursor: number
  next_cursor: number
  auto_backfill: boolean
  auto_cleanup_orphans: boolean
  max_auto_backfill: number
  max_auto_cleanup_orphans: number
  scanned_audit_events: number
  missing_relations: number
  invalid_audit_events: number
  orphan_source_relations: number
  orphan_target_relations: number
  backfill_created: number
  backfill_would_create: number
  backfill_skipped_invalid: number
  backfill_error_count: number
  backfill_blocked: boolean
  cleanup_deleted: number
  cleanup_would_delete: number
  cleanup_blocked: boolean
  started_at: number
  finished_at: number
  created_at: number
}

export type BillingEventReconciliationRequest = {
  sources: string[]
  limit: number
}

export type BillingEventReconciliationMismatchRequest = {
  sources: string[]
  limit: number
  detail_limit: number
}

export type BillingEventReconciliationMissingRequest = {
  sources: string[]
  limit: number
  detail_limit: number
}

export type BillingEventReconciliationExpectedEvent = {
  label: string
  source: string
  source_id: string
  phase: string
  user_id: number
  token_id: number
  event_type: string
  status: string
  amount_quota: number
  quota_delta: number
  request_id: string
  group: string
  billing_source: string
  price_unit: string
  currency: string
}

export type BillingEventReconciliationDiff = {
  field: string
  expected: string
  actual: string
}

export type BillingEventReconciliationMismatchItem = {
  source: string
  label: string
  expected: BillingEventReconciliationExpectedEvent
  actual: BillingEvent | null
  diffs: BillingEventReconciliationDiff[]
}

export type BillingEventReconciliationMissingItem = {
  source: string
  label: string
  expected: BillingEventReconciliationExpectedEvent
}

export type BillingEventReconciliationMismatchResponse = {
  limit: number
  detail_limit: number
  sources: string[]
  items: BillingEventReconciliationMismatchItem[]
  total_scanned: number
  total_expected: number
  total_mismatched: number
  total_missing: number
  total_invalid: number
  total_error_count: number
  has_more: boolean
  scan_complete: boolean
}

export type BillingEventReconciliationMissingResponse = {
  limit: number
  detail_limit: number
  sources: string[]
  items: BillingEventReconciliationMissingItem[]
  total_scanned: number
  total_expected: number
  total_missing: number
  total_mismatched: number
  total_invalid: number
  total_error_count: number
  has_more: boolean
  scan_complete: boolean
}

export type BillingEventReconciliationRepairRequest = {
  source: string
  label: string
  limit: number
  reason: string
  actual_id?: number
  expected?: BillingEventReconciliationExpectedEvent
}

export type BillingEventReconciliationBackfillMissingRequest = {
  source: string
  label: string
  limit: number
  reason: string
  expected?: BillingEventReconciliationExpectedEvent
}

export type BillingEventReconciliationRepairResponse = {
  repaired: boolean
  label: string
  source: string
  expected: BillingEventReconciliationExpectedEvent
  diffs: BillingEventReconciliationDiff[]
  before: BillingEvent | null
  after: BillingEvent | null
  audit_event: BillingEvent | null
}

export type BillingEventReconciliationBackfillMissingResponse = {
  backfilled: boolean
  label: string
  source: string
  expected: BillingEventReconciliationExpectedEvent
  event: BillingEvent | null
  audit_event: BillingEvent | null
}

export type BillingEventReconciliationSourceResult = {
  source: string
  scanned: number
  expected: number
  ledgered: number
  missing: number
  mismatched: number
  invalid: number
  has_more: boolean
  scan_complete: boolean
  error_count: number
  sample_missing: string[]
  sample_mismatched: string[]
  sample_invalid: string[]
  errors: string[]
}

export type BillingEventReconciliationResponse = {
  limit: number
  sources: string[]
  results: BillingEventReconciliationSourceResult[]
  total_scanned: number
  total_expected: number
  total_ledgered: number
  total_missing: number
  total_mismatched: number
  total_invalid: number
  total_error_count: number
  has_more: boolean
  scan_complete: boolean
}

export type MCPSummaryToolStats = {
  total: number
  enabled: number
  disabled: number
  remote: number
}

export type MCPSummaryBridgeStats = {
  total_clients: number
  online_clients: number
  offline_clients: number
  active_clients: number
  online_sessions: number
}

export type MCPSummaryCallStats = {
  total_calls: number
  success_calls: number
  error_calls: number
  timeout_calls: number
  pending_calls: number
  settled_calls: number
  unsettled: number
  free_calls: number
  quota: number
  cost: number
  result_size: number
  avg_duration_ms: number
  success_rate: number
}

export type MCPSummaryAuditStats = {
  total_requests: number
  success: number
  error: number
  timeout: number
  pending: number
  result_size: number
  avg_duration_ms: number
  success_rate: number
}

export type MCPSummaryTopTool = {
  tool_name: string
  calls: number
  success_calls: number
  quota: number
  cost: number
  avg_duration_ms: number
  success_rate: number
}

export type MCPSummaryRecentError = {
  source: string
  request_id: string
  tool_name: string
  client_id?: string
  session_id?: string
  error_code: string
  error_message: string
  created_at: number
}

export type MCPReviewItem = {
  category: string
  severity: string
  target_type: string
  target_id: string
  target_name: string
  reasons: string[]
  detail: string
  created_at?: number
}

export type MCPReviewScanScope = {
  scanned: number
  total: number
  limit: number
  capped: boolean
}

export type MCPReviewScanLimits = {
  proxy_servers?: MCPReviewScanScope
  bridge_clients?: MCPReviewScanScope
  tools?: MCPReviewScanScope
}

export type MCPReviewQueue = {
  total: number
  critical_count: number
  warning_count: number
  visible_count?: number
  max_items?: number
  truncated?: boolean
  scan_limits?: MCPReviewScanLimits
  items: MCPReviewItem[]
}

export type MCPSummaryBridgeTrendBucket = {
  bucket_start: number
  online_clients: number
  started_sessions: number
  closed_sessions: number
}

export type MCPSummaryOpenAPIStorageBucket = {
  bucket_start: number
  object_count: number
  total_bytes: number
  expired_count: number
  download_count: number
}

export type MCPSummaryProxyErrorTool = {
  proxy_server_id: number
  proxy_tool_id: number
  tool_id: number
  tool_name: string
  downstream_tool_name: string
  total_calls: number
  success_calls: number
  error_calls: number
  timeout_calls: number
  success_rate: number
  avg_duration_ms: number
}

export type MCPSummaryBillingAnomalies = {
  unsettled_success_calls: number
  failed_charged_calls: number
  missing_debit_events: number
  refund_events: number
  refund_quota: number
  net_mcp_quota_delta: number
}

export type MCPSummaryOperationsTrends = {
  start_time: number
  end_time: number
  bucket_seconds: number
  checked_at: number
  bridge_online: MCPSummaryBridgeTrendBucket[]
  openapi_storage: MCPSummaryOpenAPIStorageBucket[]
  proxy_error_top_n: MCPSummaryProxyErrorTool[]
  billing_anomalies: MCPSummaryBillingAnomalies
}

export type MCPSummary = {
  window_seconds: number
  generated_at: number
  tools: MCPSummaryToolStats
  bridge: MCPSummaryBridgeStats
  calls: MCPSummaryCallStats
  audit: MCPSummaryAuditStats
  top_tools: MCPSummaryTopTool[]
  recent_errors: MCPSummaryRecentError[]
  operations_trends?: MCPSummaryOperationsTrends
  review_queue?: MCPReviewQueue
}

export type MCPToolListParams = Partial<{
  p: number
  page_size: number
  keyword: string
  category: string
  source: string
  status: number | string
}>

export type MCPOpenAPIBinaryObjectListParams = Partial<{
  p: number
  page_size: number
  keyword: string
  provider: string
  content_family: string
  expiry_status: string
  user_id: number
  mcp_tool_id: number
  start_time: number
  end_time: number
}>

export type MCPProxyServerListParams = Partial<{
  p: number
  page_size: number
  keyword: string
  transport: string
  status: string
}>

export type MCPProxyToolListParams = Partial<{
  p: number
  page_size: number
  keyword: string
  proxy_server_id: number
  status: string
  schema_hash: string
}>

export type MCPToolCallListParams = Partial<{
  p: number
  page_size: number
  scope: 'all'
  keyword: string
  token_id: number
  tool_name: string
  status: string
  request_id: string
  bridge_session_id: string
  target_client: string
  start_time: number
  end_time: number
}>

export type BridgeClientListParams = Partial<{
  p: number
  page_size: number
  scope: 'all'
  keyword: string
  status: number | string
}>

export type BridgeAuditLogListParams = Partial<{
  p: number
  page_size: number
  scope: 'all'
  keyword: string
  token_id: number
  client_id: string
  session_id: string
  tool_name: string
  status: string
  request_id: string
  start_time: number
  end_time: number
}>

export type BillingEventListParams = Partial<{
  p: number
  page_size: number
  scope: 'all'
  keyword: string
  token_id: number
  source: string
  source_id: string
  event_type: string
  status: string
  request_id: string
  billing_source: string
  usage_kind: string
  start_time: number
  end_time: number
}>

export type MCPSummaryParams = Partial<{
  scope: 'all'
  window_seconds: number
}>
