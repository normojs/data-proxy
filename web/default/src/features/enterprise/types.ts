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

export type PageInfo<T> = {
  page: number
  page_size: number
  total: number
  items: T[]
}

export type EnterpriseAnomalyThrottleConfig = {
  enabled: boolean
  current_window_seconds: number
  baseline_window_seconds: number
  cooldown_seconds: number
  min_current_requests: number
  min_baseline_requests: number
  request_spike_ratio: number
  min_current_quota: number
  min_baseline_quota: number
  cost_spike_ratio: number
  min_failure_requests: number
  min_failures: number
  failure_rate: number
}

export type Enterprise = {
  id: number
  name: string
  slug: string
  status: number
  timezone: string
  anomaly_throttle_config: EnterpriseAnomalyThrottleConfig
  created_at: number
  updated_at: number
}

export type EnterpriseOrgUnit = {
  id: number
  enterprise_id: number
  parent_id: number
  name: string
  slug: string
  description: string
  path: string
  depth: number
  sort: number
  status: number
  created_at: number
  updated_at: number
}

export type EnterpriseMember = {
  user_id: number
  username: string
  display_name: string
  email: string
  status: number
  org_unit_id: number
  org_unit_name: string
  role?: string
  policy_group_count: number
}

export type EnterpriseOrgSyncOrgUnitPayload = {
  external_id: string
  parent_external_id?: string
  name: string
  slug: string
  description?: string
  sort?: number
  status?: number
}

export type EnterpriseOrgSyncMemberPayload = {
  user_id?: number
  username?: string
  email?: string
  provider_user_id?: string
  org_unit_external_id?: string
  org_unit_slug?: string
  role?: string
}

export type EnterpriseOrgSyncPayload = {
  provider: string
  snapshot_at?: number
  org_units: EnterpriseOrgSyncOrgUnitPayload[]
  members: EnterpriseOrgSyncMemberPayload[]
  allow_conflicts?: boolean
}

export type EnterpriseOrgSyncSummary = {
  org_units_total: number
  members_total: number
  create_org_units: number
  update_org_units: number
  unchanged_org_units: number
  assign_members: number
  unchanged_members: number
  conflicts: number
}

export type EnterpriseOrgSyncConflict = {
  type: string
  external_id?: string
  user_id?: number
  username?: string
  email?: string
  field?: string
  message: string
}

export type EnterpriseOrgSyncOperation = {
  type: string
  action: string
  external_id?: string
  slug?: string
  user_id?: number
  target_id?: number
  target_name?: string
  before?: Record<string, unknown>
  after?: Record<string, unknown>
}

export type EnterpriseOrgSyncResult = {
  provider: string
  snapshot_at: number
  dry_run: boolean
  applied_at?: number
  summary: EnterpriseOrgSyncSummary
  conflicts: EnterpriseOrgSyncConflict[]
  operations: EnterpriseOrgSyncOperation[]
}

export type EnterprisePolicyGroup = {
  id: number
  enterprise_id: number
  org_unit_id: number
  name: string
  slug: string
  description: string
  shared_org_unit_ids: number[]
  shared_org_unit_names: string[]
  shared_expires_at: number
  can_manage: boolean
  status: number
  created_at: number
  updated_at: number
  member_count: number
  policy_count: number
}

export type EnterprisePolicyGroupShareRequestStatus =
  | 'pending'
  | 'approved'
  | 'rejected'
  | 'withdrawn'

export type EnterprisePolicyGroupShareRequest = {
  id: number
  enterprise_id: number
  policy_group_id: number
  requester_user_id: number
  requester_org_unit_id: number
  target_org_unit_id: number
  shared_expires_at: number
  reason: string
  status: EnterprisePolicyGroupShareRequestStatus
  approver_user_id: number
  decision_reason: string
  decided_at: number
  created_at: number
  updated_at: number
  policy_group_name: string
  requester_org_unit_name: string
  target_org_unit_name: string
  requester_name: string
  approver_name: string
  can_decide: boolean
}

export type EnterpriseProject = {
  id: number
  enterprise_id: number
  name: string
  slug: string
  description: string
  owner_user_id: number
  owner_name: string
  org_unit_ids: number[]
  org_unit_names: string[]
  member_role?: string
  can_manage: boolean
  member_count: number
  policy_count: number
  status: number
  created_at: number
  updated_at: number
}

export type PolicyTargetType =
  | 'enterprise'
  | 'org_unit'
  | 'project'
  | 'policy_group'
  | 'user'

export type PolicyMetric = 'request_count' | 'quota'

export type PolicyPeriod = 'day' | 'month'

export type PolicyModelScope = 'all' | 'specific'

export type PolicyConditionMode = 'structured' | 'cel'

export type PolicyAction =
  | 'reject'
  | 'alert'
  | 'fallback_model'
  | 'queue'
  | 'shared_pool'

export type EnterpriseQuotaPolicy = {
  id: number
  enterprise_id: number
  name: string
  description: string
  target_type: PolicyTargetType
  target_id: number
  target_name: string
  metric: PolicyMetric
  period: PolicyPeriod
  limit_value: number
  timezone: string
  model_scope: PolicyModelScope
  models_json: string
  condition_mode: PolicyConditionMode
  condition_json: string
  condition_expr: string
  condition_hash: string
  action: PolicyAction
  priority: number
  status: number
  effective_at: number
  expires_at: number
  created_at: number
  updated_at: number
  used_value: number
}

export type EnterpriseQuotaRequestStatus =
  | 'pending'
  | 'approved'
  | 'rejected'
  | 'withdrawn'
  | 'expired'

export type EnterpriseQuotaRequest = {
  id: number
  enterprise_id: number
  applicant_user_id: number
  approver_user_id: number
  policy_id: number
  policy_name: string
  project_id: number
  policy_limit_value: number
  policy_used_value: number
  stacked_limit_value: number
  recent_policy_hits: number
  recent_dry_run_hits: number
  target_type: PolicyTargetType
  target_id: number
  target_name: string
  metric: PolicyMetric
  period: PolicyPeriod
  limit_delta: number
  reason: string
  decision_reason: string
  status: EnterpriseQuotaRequestStatus
  effective_at: number
  expires_at: number
  decided_at: number
  created_at: number
  updated_at: number
  applicant_name: string
  approver_name: string
}

export type EnterpriseQuotaRequestPolicy = EnterpriseQuotaPolicy

export type EnterpriseUsageTotal = {
  request_count: number
  quota: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
}

export type EnterpriseUsageBreakdownItem = EnterpriseUsageTotal & {
  dimension: string
  target_id: number
  target_name: string
  model_name?: string
  status?: string
  time_bucket?: string
}

export type EnterpriseUsageSummary = {
  start_time: number
  end_time: number
  total: EnterpriseUsageTotal
  by_model: EnterpriseUsageBreakdownItem[]
  by_status: EnterpriseUsageBreakdownItem[]
}

export type EnterpriseAuditLog = {
  id: number
  enterprise_id: number
  actor_user_id: number
  action: string
  target_type: string
  target_id: number
  before_json: string
  after_json: string
  request_id: string
  created_at: number
}

export type EnterpriseQueueAdmission = {
  id: number
  request_id: string
  enterprise_id: number
  user_id: number
  token_id: number
  org_unit_id: number
  project_id: number
  policy_id: number
  policy_ids_json: string
  policy_group_ids_json: string
  model_name: string
  channel_id: number
  relay_mode: number
  queue_key: string
  status: 'admitted' | 'timeout' | 'canceled' | string
  wait_ms: number
  timeout_ms: number
  dry_run: boolean
  policy_actions_json: string
  user_message_key: string
  created_at: number
}

export type EnterpriseWebhook = {
  id: number
  enterprise_id: number
  name: string
  url: string
  has_secret: boolean
  event_types: string[]
  event_types_json: string
  status: number
  created_at: number
  updated_at: number
}

export type EnterpriseWebhookPayload = {
  name: string
  url: string
  secret?: string
  event_types: string[]
  status: number
}

export type EnterpriseWebhookTestResult = {
  success: boolean
  status_code: number
  duration_ms: number
  error: string
  signature_header: string
}

export type EnterpriseNotificationOutboxStatus =
  | 'pending'
  | 'processing'
  | 'sent'
  | 'failed'
  | 'permanent_failed'

export type EnterpriseNotificationOutboxChannel = 'in_app' | 'email' | 'webhook'

export type EnterpriseNotificationOutbox = {
  id: number
  event_key: string
  event_type: string
  enterprise_id: number
  recipient_user_id: number
  recipient_email: string
  channel: EnterpriseNotificationOutboxChannel | string
  target_type: string
  target_id: number
  status: EnterpriseNotificationOutboxStatus | string
  retry_count: number
  next_retry_at: number
  last_error: string
  created_at: number
  updated_at: number
}

export type EnterpriseNotificationOutboxBatchStats = {
  claimed: number
  sent: number
  failed: number
  permanent_failed: number
  duration_ms: number
  started_at: number
  finished_at: number
}

export type EnterpriseNotificationOutboxWorkerMetrics = {
  last_run: EnterpriseNotificationOutboxBatchStats
  total_runs: number
  total_claimed: number
  total_sent: number
  total_failed: number
  total_permanent_failed: number
}

export type EnterpriseNotificationRecipientScope = {
  applicant: boolean
  enterprise_admins: boolean
  explicit_emails: string[]
}

export type EnterpriseNotificationPreference = {
  id: number
  enterprise_id: number
  channel: string
  event_type: string
  enabled: boolean
  recipient_scope: EnterpriseNotificationRecipientScope
  recipient_scope_json: string
  created_at: number
  updated_at: number
}

export type EnterpriseNotificationPreferencePayload = {
  channel: string
  event_type: string
  enabled: boolean
  recipient_scope: EnterpriseNotificationRecipientScope
}

export type EnterpriseOrgUnitPayload = {
  parent_id: number
  name: string
  slug: string
  description: string
  status: number
  sort: number
}

export type EnterprisePolicyGroupPayload = {
  org_unit_id?: number
  shared_org_unit_ids?: number[]
  shared_expires_at?: number
  name: string
  slug: string
  description: string
  status: number
}

export type EnterprisePolicyGroupMembersPayload = {
  user_ids: number[]
  role?: string
}

export type EnterprisePolicyGroupShareRequestPayload = {
  org_unit_id: number
  shared_expires_at?: number
  reason?: string
}

export type EnterprisePolicyGroupShareRequestDecisionPayload = {
  decision_reason?: string
}

export type EnterpriseProjectPayload = {
  name: string
  slug: string
  description: string
  owner_user_id: number
  org_unit_ids: number[]
  status: number
}

export type EnterpriseProjectMemberPayload = {
  user_id: number
  role: string
}

export type EnterpriseQuotaPolicyPayload = {
  name: string
  description: string
  target_type: PolicyTargetType
  target_id: number
  metric: PolicyMetric
  period: PolicyPeriod
  limit_value: number
  timezone: string
  model_scope: PolicyModelScope
  models: string[]
  condition_mode: PolicyConditionMode
  condition_json: string
  condition_expr: string
  action: string
  priority: number
  status: number
  effective_at: number
  expires_at: number
}

export type EnterpriseQuotaRequestPayload = {
  policy_id: number
  project_id?: number
  limit_delta: number
  reason: string
  expires_at: number
}

export type EnterpriseQuotaRequestPolicyParams = {
  project_id?: number
}

export type EnterpriseQuotaRequestDecisionPayload = {
  decision_reason: string
  expires_at?: number
}

export type EnterpriseQuotaRequestBatchDecisionPayload =
  EnterpriseQuotaRequestDecisionPayload & {
    ids: number[]
  }

export type EnterpriseQuotaRequestBatchDecisionItem = {
  id: number
  success: boolean
  status: EnterpriseQuotaRequestStatus | string
  message?: string
}

export type EnterpriseQuotaRequestBatchDecisionResult = {
  items: EnterpriseQuotaRequestBatchDecisionItem[]
  success_count: number
  failure_count: number
}

export type EnterpriseListParams = {
  p?: number
  page_size?: number
  keyword?: string
  status?: number | string
}

export type EnterpriseMembersParams = EnterpriseListParams & {
  org_unit_id?: number
  unassigned?: boolean
}

export type EnterprisePolicyGroupMembersParams = {
  p?: number
  page_size?: number
  keyword?: string
}

export type EnterprisePolicyGroupShareRequestsParams = EnterpriseListParams & {
  policy_group_id?: number
  org_unit_id?: number
  status?: EnterprisePolicyGroupShareRequestStatus | string
}

export type EnterpriseQuotaPoliciesParams = EnterpriseListParams & {
  target_type?: string
  metric?: string
}

export type EnterpriseQuotaRequestsParams = {
  p?: number
  page_size?: number
  id?: number
  status?: EnterpriseQuotaRequestStatus | string
  policy_id?: number
  project_id?: number
  target_type?: string
  target_id?: number
  applicant_user_id?: number
}

export type EnterpriseProjectsParams = EnterpriseListParams & {
  owner_user_id?: number
  org_unit_id?: number
}

export type EnterpriseUsageParams = {
  start_time?: number
  end_time?: number
  user_id?: number
  org_unit_id?: number
  project_id?: number
  policy_group_id?: number
  channel_id?: number
  token_id?: number
  model_name?: string
  status?: string
  granularity?: 'day' | 'month'
}

export type EnterpriseUsageBreakdownParams = EnterpriseUsageParams & {
  p?: number
  page_size?: number
  dimension?:
    | 'org_unit'
    | 'project'
    | 'policy_group'
    | 'user'
    | 'model'
    | 'status'
    | 'channel'
    | 'api_key'
    | 'time'
  sort_by?: 'quota' | 'request_count' | 'tokens'
  sort_order?: 'asc' | 'desc'
}

export type EnterpriseAuditLogParams = {
  p?: number
  page_size?: number
  action?: string
  target_type?: string
  target_id?: number
  actor_user_id?: number
  request_id?: string
  start_time?: number
  end_time?: number
}

export type EnterpriseQueueAdmissionParams = {
  p?: number
  page_size?: number
  status?: string
  request_id?: string
  model_name?: string
  user_id?: number
  token_id?: number
  org_unit_id?: number
  project_id?: number
  policy_id?: number
  channel_id?: number
  start_time?: number
  end_time?: number
}

export type EnterpriseNotificationOutboxParams = {
  p?: number
  page_size?: number
  channel?: string
  event_type?: string
  status?: string
  target_type?: string
  target_id?: number
  webhook_id?: number
  start_time?: number
  end_time?: number
}
