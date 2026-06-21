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
import { api } from '@/lib/api'

type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data?: T
}

type ApiPage<T> = {
  items: T[]
  total: number
  page: number
  page_size: number
}

function buildQuery(params: Record<string, unknown> = {}) {
  const query = new URLSearchParams()

  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return
    query.set(key, String(value))
  })

  return query.toString()
}

function withQuery(path: string, params?: Record<string, unknown>) {
  const query = buildQuery(params)
  return query ? `${path}?${query}` : path
}

export const CONNECTED_APP_STATUS_ENABLED = 1
export const CONNECTED_APP_STATUS_DISABLED = 2

export type ConnectedAppStatus =
  | typeof CONNECTED_APP_STATUS_ENABLED
  | typeof CONNECTED_APP_STATUS_DISABLED

export type ConnectedApp = {
  id: number
  slug: string
  name: string
  description: string
  allowed_scopes: string[]
  default_scopes: string[]
  trusted: boolean
  status: ConnectedAppStatus
  authorization_flow: 'device_code' | string
  grant_count: number
  device_count: number
  active_device_count: number
  created_at: number
  updated_at: number
}

export type ConnectedAppPayload = {
  slug?: string
  name: string
  description: string
  allowed_scopes: string[]
  default_scopes: string[]
  authorization_flow: 'device_code' | string
  trusted: boolean
  status: ConnectedAppStatus
}

export type ConnectedAppRequestStatus = 'pending' | 'approved' | 'rejected'

export type ConnectedAppRequest = {
  id: number
  applicant_user_id: number
  applicant_name: string
  app_id: number
  slug: string
  name: string
  description: string
  requested_scopes: string[]
  default_scopes: string[]
  authorization_flow: 'device_code' | string
  homepage_url: string
  callback_url: string
  reason: string
  status: ConnectedAppRequestStatus | string
  reviewer_user_id: number
  reviewer_name: string
  review_note: string
  reviewed_at: number
  created_at: number
  updated_at: number
}

export type ConnectedAppRequestListParams = {
  status?: ConnectedAppRequestStatus | string
  p?: number
  page_size?: number
}

export type ConnectedAppReviewDecision = 'approved' | 'rejected'

export type ConnectedAppReviewPayload = {
  decision: ConnectedAppReviewDecision
  review_note?: string
  name?: string
  description?: string
  allowed_scopes?: string[]
  default_scopes?: string[]
  authorization_flow?: 'device_code' | string
  homepage_url?: string
  callback_url?: string
}

export type ConnectedAppReviewResponse = {
  request: ConnectedAppRequest
  app?: ConnectedApp
  audit: ConnectedAppAuditLog
}

export type ConnectedAppAuditLog = {
  id: number
  actor_user_id: number
  actor_name: string
  action: string
  target_type: string
  target_id: number
  before_json: string
  after_json: string
  request_id: string
  created_at: number
}

export type ConnectedAppAuditLogListParams = {
  action?: string
  target_type?: string
  target_id?: number
  actor_user_id?: number
  request_id?: string
  p?: number
  page_size?: number
}

export type ConnectedAppNotificationRecipientScope = {
  applicant: boolean
  authorizing_user: boolean
  app_developers: boolean
  explicit_emails: string[]
}

export type ConnectedAppNotificationPreference = {
  id: number
  app_id: number
  channel: string
  event_type: string
  enabled: boolean
  recipient_scope: ConnectedAppNotificationRecipientScope
  recipient_scope_json: string
  created_at: number
  updated_at: number
}

export type ConnectedAppNotificationPreferencePayload = {
  app_id: number
  channel: string
  event_type: string
  enabled: boolean
  recipient_scope: ConnectedAppNotificationRecipientScope
}

export type ConnectedAppWebhook = {
  id: number
  app_id: number
  name: string
  url: string
  has_secret: boolean
  event_types: string[]
  event_types_json: string
  status: number
  created_at: number
  updated_at: number
}

export type ConnectedAppWebhookPayload = {
  app_id: number
  name: string
  url: string
  secret?: string
  event_types: string[]
  status: number
}

export type ConnectedAppWebhookTestResult = {
  success: boolean
  status_code: number
  duration_ms: number
  error: string
  signature_header: string
}

export type ConnectedAppNotificationOutbox = {
  id: number
  event_key: string
  event_type: string
  app_id: number
  recipient_user_id: number
  recipient_email: string
  channel: string
  target_type: string
  target_id: number
  status: string
  retry_count: number
  next_retry_at: number
  last_error: string
  created_at: number
  updated_at: number
}

export type ConnectedAppNotificationOutboxParams = {
  app_id?: number
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

export type ConnectedAppNotificationOutboxBatchStats = {
  claimed: number
  sent: number
  failed: number
  permanent_failed: number
  duration_ms: number
  started_at: number
  finished_at: number
}

export type ConnectedAppNotificationOutboxWorkerMetrics = {
  last_run: ConnectedAppNotificationOutboxBatchStats
  total_runs: number
  total_claimed: number
  total_sent: number
  total_failed: number
  total_permanent_failed: number
}

export type ConnectedAppDeveloperApp = {
  id: number
  slug: string
  name: string
  trusted: boolean
  status: number
  allowed_scopes?: string[]
  default_scopes: string[]
}

export type ConnectedAppDeveloperGrant = {
  id?: number
  status: string
  scopes: string[]
  authorized_at?: number
  last_used_at?: number
  revoked_at?: number
}

export type ConnectedAppDeveloperDevice = {
  fingerprint: string
  device_name: string
  platform: string
  app_version: string
  client?: string
}

export type ConnectedAppDeveloperToken = {
  id?: number
  name?: string
  status?: number
  masked_key?: string
  expired_time?: number
  unlimited_quota?: boolean
  quota_hard_limit_enabled?: boolean
  model_limits_enabled?: boolean
  model_limits?: string
  binding_status?: string
  last_used_at?: number
}

export type ConnectedAppDeveloperSDKConfig = {
  app: ConnectedAppDeveloperApp
  owner: boolean
  base_url: string
  api_endpoints: Record<string, string>
  device_flow: Record<string, string>
  developer_endpoints: Record<string, string>
  scopes: string[]
  permissions: {
    can_create_key: boolean
    can_read_usage: boolean
  }
  openapi_url: string
  sdk: {
    openai_compatible: boolean
    base_url: string
    api_key_env: string
    api_key_prefix: string
    authorization: string
  }
}

export type ConnectedAppDeveloperKeyPayload = {
  device_id?: string
  device_name?: string
  platform?: string
  app_version?: string
  client?: string
  rotate?: boolean
}

export type ConnectedAppDeveloperKeyResponse = {
  app: ConnectedAppDeveloperApp
  grant: ConnectedAppDeveloperGrant
  device: ConnectedAppDeveloperDevice
  token: ConnectedAppDeveloperToken
  endpoints: Record<string, string>
  base_url: string
  api_key?: string
  created: boolean
  rotated: boolean
  api_key_once: boolean
}

export type ConnectedAppDeveloperUsageTotals = {
  request_count: number
  quota: number
  prompt_tokens: number
  completion_tokens: number
}

export type ConnectedAppDeveloperUsageByModel =
  ConnectedAppDeveloperUsageTotals & {
    model_name: string
  }

export type ConnectedAppDeveloperUsageByToken =
  ConnectedAppDeveloperUsageTotals & {
    token_id: number
    token_name: string
    user_id: number
    status: string
    device: ConnectedAppDeveloperDevice
  }

export type ConnectedAppDeveloperUsageParams = {
  start_time?: number
  end_time?: number
  token_id?: number
  user_id?: number
  model_name?: string
}

export type ConnectedAppDeveloperUsage = {
  app: ConnectedAppDeveloperApp
  start_time: number
  end_time: number
  token_count: number
  total: ConnectedAppDeveloperUsageTotals
  by_model: ConnectedAppDeveloperUsageByModel[]
  by_token: ConnectedAppDeveloperUsageByToken[]
}

function unwrap<T>(response: ApiEnvelope<T>): T {
  if (!response.success || response.data == null) {
    throw new Error(response.message || 'Request failed')
  }
  return response.data
}

export async function listConnectedApps(): Promise<ConnectedApp[]> {
  const res = await api.get<ApiEnvelope<ConnectedApp[]>>(
    '/api/connected-apps',
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function createConnectedApp(
  payload: ConnectedAppPayload
): Promise<ConnectedApp> {
  const res = await api.post<ApiEnvelope<ConnectedApp>>(
    '/api/connected-apps',
    payload,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function updateConnectedApp(
  id: number,
  payload: ConnectedAppPayload
): Promise<ConnectedApp> {
  const res = await api.put<ApiEnvelope<ConnectedApp>>(
    `/api/connected-apps/${id}`,
    payload,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function listConnectedAppRequests(
  params: ConnectedAppRequestListParams = {}
): Promise<ApiPage<ConnectedAppRequest>> {
  const res = await api.get<ApiEnvelope<ApiPage<ConnectedAppRequest>>>(
    '/api/connected-apps/requests',
    { params, skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function listSelfConnectedAppRequests(
  params: ConnectedAppRequestListParams = {}
): Promise<ApiPage<ConnectedAppRequest>> {
  const res = await api.get<ApiEnvelope<ApiPage<ConnectedAppRequest>>>(
    '/api/connected-app-requests/self',
    { params, skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function reviewConnectedAppRequest(
  id: number,
  payload: ConnectedAppReviewPayload
): Promise<ConnectedAppReviewResponse> {
  const res = await api.post<ApiEnvelope<ConnectedAppReviewResponse>>(
    `/api/connected-apps/requests/${id}/review`,
    payload,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function listConnectedAppAuditLogs(
  params: ConnectedAppAuditLogListParams = {}
): Promise<ApiPage<ConnectedAppAuditLog>> {
  const res = await api.get<ApiEnvelope<ApiPage<ConnectedAppAuditLog>>>(
    '/api/connected-apps/audit-logs',
    { params, skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function listConnectedAppNotificationPreferences(
  appId = 0
): Promise<ConnectedAppNotificationPreference[]> {
  const res = await api.get<ApiEnvelope<ConnectedAppNotificationPreference[]>>(
    withQuery('/api/connected-apps/notification-preferences', {
      app_id: appId,
    }),
    {
      skipBusinessError: true,
    }
  )
  return unwrap(res.data)
}

export async function updateConnectedAppNotificationPreference(
  payload: ConnectedAppNotificationPreferencePayload
): Promise<ConnectedAppNotificationPreference> {
  const res = await api.put<ApiEnvelope<ConnectedAppNotificationPreference>>(
    '/api/connected-apps/notification-preferences',
    payload,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function listConnectedAppWebhooks(
  appId = 0
): Promise<ConnectedAppWebhook[]> {
  const res = await api.get<ApiEnvelope<ConnectedAppWebhook[]>>(
    withQuery('/api/connected-apps/webhooks', { app_id: appId }),
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function createConnectedAppWebhook(
  payload: ConnectedAppWebhookPayload
): Promise<ConnectedAppWebhook> {
  const res = await api.post<ApiEnvelope<ConnectedAppWebhook>>(
    '/api/connected-apps/webhooks',
    payload,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function updateConnectedAppWebhook(
  id: number,
  payload: ConnectedAppWebhookPayload
): Promise<ConnectedAppWebhook> {
  const res = await api.put<ApiEnvelope<ConnectedAppWebhook>>(
    `/api/connected-apps/webhooks/${id}`,
    payload,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function disableConnectedAppWebhook(
  id: number
): Promise<ConnectedAppWebhook> {
  const res = await api.delete<ApiEnvelope<ConnectedAppWebhook>>(
    `/api/connected-apps/webhooks/${id}`,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function testConnectedAppWebhook(
  id: number
): Promise<ConnectedAppWebhookTestResult> {
  const res = await api.post<ApiEnvelope<ConnectedAppWebhookTestResult>>(
    `/api/connected-apps/webhooks/${id}/test`,
    undefined,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function listConnectedAppNotificationOutbox(
  params: ConnectedAppNotificationOutboxParams = {}
): Promise<ApiPage<ConnectedAppNotificationOutbox>> {
  const res = await api.get<
    ApiEnvelope<ApiPage<ConnectedAppNotificationOutbox>>
  >(withQuery('/api/connected-apps/notification-outbox', params), {
    skipBusinessError: true,
  })
  return unwrap(res.data)
}

export async function retryConnectedAppNotificationOutbox(
  id: number
): Promise<ConnectedAppNotificationOutbox> {
  const res = await api.post<ApiEnvelope<ConnectedAppNotificationOutbox>>(
    `/api/connected-apps/notification-outbox/${id}/retry`,
    undefined,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function getConnectedAppNotificationOutboxWorkerMetrics(): Promise<ConnectedAppNotificationOutboxWorkerMetrics> {
  const res = await api.get<
    ApiEnvelope<ConnectedAppNotificationOutboxWorkerMetrics>
  >('/api/connected-apps/notification-outbox/worker-metrics', {
    skipBusinessError: true,
  })
  return unwrap(res.data)
}

function connectedAppDeveloperPath(appSlug: string, suffix: string) {
  return `/api/connected-apps/${encodeURIComponent(appSlug)}/developer${suffix}`
}

export async function listConnectedAppDeveloperNotificationPreferences(
  appSlug: string
): Promise<ConnectedAppNotificationPreference[]> {
  const res = await api.get<ApiEnvelope<ConnectedAppNotificationPreference[]>>(
    connectedAppDeveloperPath(appSlug, '/notification-preferences'),
    {
      skipBusinessError: true,
    }
  )
  return unwrap(res.data)
}

export async function updateConnectedAppDeveloperNotificationPreference(
  appSlug: string,
  payload: ConnectedAppNotificationPreferencePayload
): Promise<ConnectedAppNotificationPreference> {
  const res = await api.patch<ApiEnvelope<ConnectedAppNotificationPreference>>(
    connectedAppDeveloperPath(appSlug, '/notification-preferences'),
    payload,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function listConnectedAppDeveloperWebhooks(
  appSlug: string
): Promise<ConnectedAppWebhook[]> {
  const res = await api.get<ApiEnvelope<ConnectedAppWebhook[]>>(
    connectedAppDeveloperPath(appSlug, '/webhooks'),
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function createConnectedAppDeveloperWebhook(
  appSlug: string,
  payload: ConnectedAppWebhookPayload
): Promise<ConnectedAppWebhook> {
  const res = await api.post<ApiEnvelope<ConnectedAppWebhook>>(
    connectedAppDeveloperPath(appSlug, '/webhooks'),
    payload,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function updateConnectedAppDeveloperWebhook(
  appSlug: string,
  id: number,
  payload: ConnectedAppWebhookPayload
): Promise<ConnectedAppWebhook> {
  const res = await api.patch<ApiEnvelope<ConnectedAppWebhook>>(
    connectedAppDeveloperPath(appSlug, `/webhooks/${id}`),
    payload,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function disableConnectedAppDeveloperWebhook(
  appSlug: string,
  id: number
): Promise<ConnectedAppWebhook> {
  const res = await api.delete<ApiEnvelope<ConnectedAppWebhook>>(
    connectedAppDeveloperPath(appSlug, `/webhooks/${id}`),
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function testConnectedAppDeveloperWebhook(
  appSlug: string,
  id: number
): Promise<ConnectedAppWebhookTestResult> {
  const res = await api.post<ApiEnvelope<ConnectedAppWebhookTestResult>>(
    connectedAppDeveloperPath(appSlug, `/webhooks/${id}/test`),
    undefined,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function listConnectedAppDeveloperNotificationOutbox(
  appSlug: string,
  params: ConnectedAppNotificationOutboxParams = {}
): Promise<ApiPage<ConnectedAppNotificationOutbox>> {
  const res = await api.get<
    ApiEnvelope<ApiPage<ConnectedAppNotificationOutbox>>
  >(
    withQuery(
      connectedAppDeveloperPath(appSlug, '/notification-outbox'),
      params
    ),
    {
      skipBusinessError: true,
    }
  )
  return unwrap(res.data)
}

export async function getConnectedAppDeveloperSDKConfig(
  appSlug: string
): Promise<ConnectedAppDeveloperSDKConfig> {
  const res = await api.get<ApiEnvelope<ConnectedAppDeveloperSDKConfig>>(
    connectedAppDeveloperPath(appSlug, '/sdk-config'),
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function getConnectedAppDeveloperOpenAPI(
  appSlug: string
): Promise<Record<string, unknown>> {
  const res = await api.get<ApiEnvelope<Record<string, unknown>>>(
    connectedAppDeveloperPath(appSlug, '/openapi'),
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function createConnectedAppDeveloperKey(
  appSlug: string,
  payload: ConnectedAppDeveloperKeyPayload
): Promise<ConnectedAppDeveloperKeyResponse> {
  const res = await api.post<ApiEnvelope<ConnectedAppDeveloperKeyResponse>>(
    connectedAppDeveloperPath(appSlug, '/keys'),
    payload,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function getConnectedAppDeveloperUsage(
  appSlug: string,
  params: ConnectedAppDeveloperUsageParams = {}
): Promise<ConnectedAppDeveloperUsage> {
  const res = await api.get<ApiEnvelope<ConnectedAppDeveloperUsage>>(
    withQuery(connectedAppDeveloperPath(appSlug, '/usage'), params),
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}
