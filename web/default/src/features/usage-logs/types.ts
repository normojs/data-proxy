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
/**
 * Type definitions for usage logs
 */
import type { UsageLog } from './data/schema'

// ============================================================================
// Log Category Types
// ============================================================================

/**
 * Log category for different log types
 */
export type LogCategory = 'common' | 'drawing' | 'task'

// ============================================================================
// Filter Types
// ============================================================================

/**
 * Common filters (shared across all log types)
 */
export interface CommonFilters {
  startTime?: Date
  endTime?: Date
  channel?: string
}

/**
 * Common logs specific filters
 */
export interface CommonLogFilters extends CommonFilters {
  model?: string
  token?: string
  group?: string
  username?: string
  requestId?: string
  upstreamRequestId?: string
}

export interface LogFilterOptions {
  groups: string[]
  model_names: string[]
  token_names: string[]
}

/**
 * Drawing logs specific filters
 */
export interface DrawingLogFilters extends CommonFilters {
  mjId?: string
}

/**
 * Task logs specific filters
 */
export interface TaskLogFilters extends CommonFilters {
  taskId?: string
}

/**
 * Union type for all log filters
 */
export type LogFilters = CommonLogFilters | DrawingLogFilters | TaskLogFilters

// ============================================================================
// Common Logs Additional Types
// ============================================================================

/**
 * Parsed data from the 'other' field in usage logs
 */
export interface ChannelAffinityInfo {
  rule_name?: string
  selected_group?: string
  key_source?: string
  key_path?: string
  key_key?: string
  key_hint?: string
  key_fp?: string
  using_group?: string
}

export interface ChannelFailoverEvent {
  event?: 'selected' | 'failed' | string
  ts?: number
  retry_index?: number
  remaining_retries?: number
  retry_planned?: boolean
  token_group?: string
  selected_group?: string
  model_name?: string
  channel_id?: number
  channel_name?: string
  channel_type?: number
  excluded_channel_ids?: number[]
  auto_ban?: boolean
  is_multi_key?: boolean
  multi_key_index?: number
  error_type?: string
  error_code?: string
  status_code?: number
  reason?: string
  health_action?: string
  health_failure_count?: number
  health_cooldown_until?: number
  health_reason?: string
  runtime_status?: string
  temporarily_unavailable?: boolean
  consecutive_failures?: number
  cooldown_until?: number
  [key: string]: unknown
}

export interface RequestConversionMeta {
  responses_protocol?: string
  upstream_protocol?: string
  responses_protocol_decision?: string
  responses_channel_capability?: string
  responses_native_supported?: boolean
  responses_chat_preferred?: boolean
  responses_reasoning_adapter?: string
  responses_reasoning_adapter_recommended?: string
  responses_reasoning_adapter_source?: string
  reasoning_forwarded?: boolean
  reasoning_params?: string[]
  reasoning_effort_mapped?: string
  hosted_tools_functionized?: string[]
  hosted_tools_requested?: string[]
  hosted_tools_filtered?: string[]
  hosted_tools_policy?: string
  hosted_tools_rejected?: boolean
  hosted_tools_executor_bridge_requested?: boolean
  hosted_tools_executor_bridge_ready?: boolean
  hosted_tools_direct_answer_hint?: boolean
  hosted_web_search_executor_calls?: number
  hosted_web_search_executor_error?: string
  hosted_web_search_executor_events?: Array<{
    status?: string
    tool_name?: string
    tool_call_id?: string
    duration_ms?: number
    results_count?: number
    answer_chars?: number
    query_present?: boolean
    error?: string
  }>
  unsupported_tools_filtered?: string[]
  history_restored_count?: number
  history_restore_sources?: string[]
  history_recorded_count?: number
  input_provided_tools_count?: number
  namespace_tools_flattened?: number
  reasoning_backfilled_count?: number
  chat_sse_fallback?: boolean
  responses_terminal_status?: string
  responses_incomplete_details?: unknown
  responses_terminal_error?: unknown
  notes?: string[]
  [key: string]: unknown
}

export interface LogOtherData {
  admin_info?: {
    is_multi_key?: boolean
    multi_key_index?: number
    use_channel?: number[]
    local_count_tokens?: boolean
    channel_affinity?: ChannelAffinityInfo
    multi_key_affinity?: MultiKeyAffinityInfo
    channel_failover?: ChannelFailoverEvent[]
    // Top-up audit fields (type=1, admin only)
    payment_method?: string
    callback_payment_method?: string
    caller_ip?: string
    server_ip?: string
    version?: string
    node_name?: string
    // Manage audit fields (type=3, admin only)
    admin_username?: string
    admin_id?: number | string
  }
  request_path?: string
  request_conversion?: string[]
  request_conversion_meta?: RequestConversionMeta
  ws?: boolean
  audio?: boolean
  audio_input?: number
  audio_output?: number
  text_input?: number
  text_output?: number
  cache_tokens?: number
  cache_creation_tokens?: number
  cache_creation_tokens_5m?: number
  cache_creation_tokens_1h?: number
  claude?: boolean
  model_ratio?: number
  completion_ratio?: number
  model_price?: number
  group_ratio?: number
  user_group_ratio?: number
  cache_ratio?: number
  cache_creation_ratio?: number
  cache_creation_ratio_5m?: number
  cache_creation_ratio_1h?: number
  is_model_mapped?: boolean
  upstream_model_name?: string
  audio_ratio?: number
  audio_completion_ratio?: number
  frt?: number
  // Tiered (expression-based) billing fields, set by backend when
  // billing_mode === 'tiered_expr'. expr_b64 is the base64-encoded billing
  // expression and matched_tier is the label of the tier that fired.
  billing_mode?: string
  expr_b64?: string
  matched_tier?: string
  reasoning_effort?: string
  image?: boolean
  image_ratio?: number
  image_output?: number
  web_search?: boolean
  web_search_call_count?: number
  web_search_price?: number
  file_search?: boolean
  file_search_call_count?: number
  file_search_price?: number
  audio_input_seperate_price?: boolean
  audio_input_token_count?: number
  audio_input_price?: number
  image_generation_call?: boolean
  image_generation_call_price?: number
  is_system_prompt_overwritten?: boolean
  po?: string[]
  billing_source?: string
  group?: string
  stream_status?: {
    status?: string
    end_reason?: string
    error_count?: number
    end_error?: string
    errors?: string[]
  }
  // Violation fee fields
  violation_fee?: boolean
  violation_fee_code?: string
  violation_fee_marker?: string
  fee_quota?: number
  // Reject / intercept reason (admin)
  reject_reason?: string
  // Task-related fields (for refund logs, type=6)
  is_task?: boolean
  task_id?: string
  reason?: string
  // Subscription billing fields
  subscription_plan_id?: string
  subscription_plan_title?: string
  subscription_id?: string
  subscription_pre_consumed?: number
  subscription_post_delta?: number
  subscription_consumed?: number
  subscription_remain?: number
  subscription_total?: number
}

export interface MultiKeyAffinityInfo {
  enabled?: boolean
  mode?: string
  seed_source?: string
  seed_fp?: string
  binding_hit?: boolean
  selected_key_index?: number
  selected_key_fp?: string
  primary_key_index?: number
  load_state?: string
  key_load?: number
  avg_rpm?: number
  avg_inflight?: number
  fallback_reason?: string
}

/**
 * Log statistics data
 */
export interface LogStatistics {
  quota: number
  rpm: number
  tpm: number
  tokens: number
}

// ============================================================================
// Drawing Logs (Midjourney) Types
// ============================================================================

export interface MidjourneyLog {
  id: number
  user_id: number
  channel_id: number
  code: number
  mj_id: string
  action: string // IMAGINE, UPSCALE, VARIATION, etc. (backend field name)
  submit_time: number // milliseconds
  finish_time?: number // milliseconds
  start_time?: number // milliseconds
  fail_reason?: string
  progress: string
  prompt: string
  prompt_en?: string
  description?: string
  buttons?: string
  properties?: string
  image_url?: string
  status: string // NOT_START, SUBMITTED, IN_PROGRESS, SUCCESS, FAILURE, MODAL
  other?: string
  created_at?: number
  updated_at?: number
}

// ============================================================================
// Task Logs Types
// ============================================================================

export interface TaskLog {
  id: number
  user_id: number
  username?: string
  platform: string // suno, kling, runway, etc.
  task_id: string
  action: string // MUSIC, LYRICS, GENERATE, TEXT_GENERATE, etc.
  channel_id: number
  submit_time: number // seconds
  finish_time?: number // seconds
  progress?: string
  progress_message_en?: string
  data?: string // JSON string
  fail_reason?: string
  status: string // NOT_START, SUBMITTED, IN_PROGRESS, SUCCESS, FAILURE, QUEUED, UNKNOWN
  other?: string
  created_at?: number
  updated_at?: number
}

// ============================================================================
// Common Log Types
// ============================================================================

export interface GetLogsParams {
  p?: number
  page_size?: number
  type?: number
  username?: string
  token_name?: string
  model_name?: string
  start_timestamp?: number
  end_timestamp?: number
  channel?: number
  group?: string
  request_id?: string
  upstream_request_id?: string
}

export interface GetLogsResponse {
  success: boolean
  message?: string
  data?: {
    items: UsageLog[] | MidjourneyLog[] | TaskLog[]
    total: number
    page: number
    page_size: number
  }
}

export interface GetLogStatsParams {
  type?: number
  username?: string
  token_name?: string
  model_name?: string
  start_timestamp?: number
  end_timestamp?: number
  channel?: number
  group?: string
  request_id?: string
  upstream_request_id?: string
}

export interface GetLogStatsResponse {
  success: boolean
  message?: string
  data?: LogStatistics
}

export interface GetLogFilterOptionsParams {
  type?: number
  start_timestamp?: number
  end_timestamp?: number
  username?: string
  channel?: number
}

export interface GetLogFilterOptionsResponse {
  success: boolean
  message?: string
  data?: LogFilterOptions
}

export interface RequestLogTraceSummary {
  status: string
  type_counts: Record<string, number>
  user_id?: number
  username?: string
  token_id?: number
  token_name?: string
  model_name?: string
  channel?: number
  channel_name?: string
  group?: string
  quota: number
  prompt_tokens: number
  completion_tokens: number
  max_use_time: number
  is_stream: boolean
  created_at_start?: number
  created_at_end?: number
}

export interface RequestLogTraceDiagnostics extends Record<string, unknown> {
  request_path?: string
  request_conversion?: string[]
  request_conversion_meta?: RequestConversionMeta
  admin_info?: LogOtherData['admin_info']
  stream_status?: LogOtherData['stream_status']
  billing_source?: string
  billing_preference?: string
  upstream_model_name?: string
  reasoning_effort?: string
  error_type?: string
  error_code?: string
  status_code?: number
  log_count?: number
  contains_error?: boolean
  contains_consume?: boolean
  errors?: string[]
}

export interface RequestLogTraceItem {
  id: number
  user_id: number
  created_at: number
  type: number
  type_name: string
  content: string
  username: string
  token_id: number
  token_name: string
  model_name: string
  quota: number
  prompt_tokens: number
  completion_tokens: number
  use_time: number
  is_stream: boolean
  channel: number
  channel_name?: string
  group?: string
  ip?: string
  request_id?: string
  upstream_request_id?: string
  other?: LogOtherData
}

export interface RequestLogTrace {
  query: string
  scope: 'admin' | 'self' | string
  total: number
  request_ids: string[]
  upstream_request_ids: string[]
  summary: RequestLogTraceSummary
  diagnostics: RequestLogTraceDiagnostics
  logs: RequestLogTraceItem[]
}

export interface GetRequestLogTraceResponse {
  success: boolean
  message?: string
  data?: RequestLogTrace
}

export interface RequestDiagnosticFinding {
  level: 'error' | 'warning' | 'info' | string
  code: string
  message: string
  detail?: string
}

export interface RequestDiagnosticArtifact {
  id: number
  kind: string
  status: string
  provider: string
  bucket?: string
  storage_key?: string
  content_type: string
  compression?: string
  encryption_algorithm?: string
  encryption_key_id?: string
  sha256?: string
  size_bytes: number
  last_error?: string
  uploaded_at?: number
}

export interface RequestDiagnosticCapture {
  id: number
  request_id: string
  upstream_request_id?: string
  user_id: number
  token_id: number
  channel_id: number
  connected_app_id: number
  group?: string
  model_name: string
  request_path: string
  protocol_chain?: string
  capture_level: string
  capture_status: string
  is_stream: boolean
  has_error: boolean
  last_error?: string
  request_bytes: number
  upstream_request_bytes: number
  upstream_body_bytes: number
  downstream_body_bytes: number
  total_bytes: number
  spool_dir?: string
  started_at?: number
  finished_at?: number
  finalized_at?: number
  artifacts?: RequestDiagnosticArtifact[]
}

export interface RequestDiagnosticReportPayload {
  trace: RequestLogTrace
  capture?: RequestDiagnosticCapture
  findings: RequestDiagnosticFinding[]
}

export interface RequestDiagnosticReport {
  id?: number
  request_id: string
  status: string
  severity: 'ok' | 'warning' | 'error' | 'info' | string
  summary: string
  generated_at?: number
  report: RequestDiagnosticReportPayload
}

export interface GetRequestDiagnosticReportResponse {
  success: boolean
  message?: string
  data?: RequestDiagnosticReport
}

export interface RequestDiagnosticCandidate {
  request_id: string
  severity: 'ok' | 'warning' | 'error' | 'info' | string
  source: string
  summary: string
  last_seen_at: number
  error_count: number
  consume_count: number
  user_id?: number
  username?: string
  token_id?: number
  token_name?: string
  model_name?: string
  channel_id?: number
  group?: string
  report_status?: string
  report_severity?: string
}

export interface GetRequestDiagnosticCandidatesParams {
  limit?: number
  start_timestamp?: number
  end_timestamp?: number
  severity?: string
  source?: string
  model_name?: string
  channel_id?: number
  group?: string
  report_status?: string
  user_id?: number
  token_id?: number
}

export interface GetRequestDiagnosticCandidatesResponse {
  success: boolean
  message?: string
  data?: {
    total: number
    items: RequestDiagnosticCandidate[]
  }
}

// ============================================================================
// Drawing Log Types
// ============================================================================

export interface GetMidjourneyLogsParams {
  p?: number
  page_size?: number
  channel_id?: string
  mj_id?: string
  start_timestamp?: number
  end_timestamp?: number
}

// ============================================================================
// Task Log Types
// ============================================================================

export interface GetTaskLogsParams {
  p?: number
  page_size?: number
  channel_id?: string
  task_id?: string
  start_timestamp?: number
  end_timestamp?: number
}

// ============================================================================
// Fetch Logs Configuration
// ============================================================================

/**
 * Configuration for fetching logs by category
 */
export interface FetchLogsConfig {
  logCategory: LogCategory
  isAdmin: boolean
  page: number
  pageSize: number
  searchParams: Record<string, unknown>
  columnFilters: Array<{ id: string; value: unknown }>
}

// ============================================================================
// User Info Types
// ============================================================================

export interface UserInfo {
  id: number
  username: string
  display_name?: string
  quota: number
  used_quota: number
  request_count: number
  group?: string
  aff_code?: string
  aff_count?: number
  aff_quota?: number
  remark?: string
}
