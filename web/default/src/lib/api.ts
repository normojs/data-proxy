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
import axios, { type AxiosRequestConfig } from 'axios'
import { t } from 'i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'

declare module 'axios' {
  export interface AxiosRequestConfig {
    skipBusinessError?: boolean
    skipErrorHandler?: boolean
    disableDuplicate?: boolean
  }
}

export type ApiRequestConfig = AxiosRequestConfig

// ============================================================================
// Axios Instance Configuration
// ============================================================================

// Base URL: empty string for same-origin API requests
const baseURL = ''

// Create axios instance with default config
export const api = axios.create({
  baseURL,
  withCredentials: true, // Include cookies in cross-origin requests
  headers: {
    'Cache-Control': 'no-store', // Prevent caching
  },
})

// ============================================================================
// Request Deduplication
// ============================================================================

// Deduplicate concurrent GET requests to the same URL
// Prevents multiple identical requests from being sent simultaneously
const inFlightGet = new Map<string, Promise<unknown>>()
const originalGet = api.get.bind(api)

api.get = ((url: string, config: ApiRequestConfig = {}) => {
  const disableDuplicate = config.disableDuplicate
  if (disableDuplicate) return originalGet(url, config)

  const params = config.params ? JSON.stringify(config.params) : '{}'
  const key = `${url}?${params}`

  // Return existing in-flight request if available
  if (inFlightGet.has(key)) return inFlightGet.get(key)!

  // Create new request and clean up after completion
  const req = originalGet(url, config).finally(() => inFlightGet.delete(key))
  inFlightGet.set(key, req)
  return req
}) as typeof api.get

type EnterpriseQuotaRequestHint = {
  available?: boolean
  policy_id?: number | string
  project_id?: number | string
  limit_delta?: number | string
  reason?: string
}

type NormalizedQuotaRequestHint = {
  policyId?: number
  projectId?: number
  limitDelta?: number
  reason?: string
}

const enterpriseQuotaExceededCodes = new Set([
  'enterprise_governance_quota_exceeded',
  'enterprise_governance_org_quota_exceeded',
  'enterprise_governance_group_quota_exceeded',
  'enterprise_governance_user_quota_exceeded',
])

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === 'object'
    ? (value as Record<string, unknown>)
    : undefined
}

function stringValue(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value : undefined
}

function positiveNumber(value: unknown): number | undefined {
  const parsed =
    typeof value === 'number'
      ? value
      : typeof value === 'string'
        ? Number(value)
        : Number.NaN
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
}

function responseErrorData(error: unknown): Record<string, unknown> | undefined {
  const response = asRecord(asRecord(error)?.response)
  return asRecord(response?.data)
}

function responseErrorMessage(error: unknown): string {
  const data = responseErrorData(error)
  const openAIError = asRecord(data?.error)
  return (
    stringValue(data?.message) ||
    stringValue(openAIError?.message) ||
    (error instanceof Error ? error.message : undefined) ||
    t('Request failed')
  )
}

function enterpriseQuotaRequestHintFromError(
  error: unknown
): NormalizedQuotaRequestHint | null {
  const data = responseErrorData(error)
  const openAIError = asRecord(data?.error)
  const code = stringValue(openAIError?.code) || stringValue(data?.error_code)
  if (!code || !enterpriseQuotaExceededCodes.has(code)) return null

  const metadata = asRecord(openAIError?.metadata)
  const rawHint = asRecord(
    metadata?.quota_request_hint
  ) as EnterpriseQuotaRequestHint | undefined
  if (rawHint?.available === false) return null

  return {
    policyId: positiveNumber(rawHint?.policy_id),
    projectId: positiveNumber(rawHint?.project_id),
    limitDelta: positiveNumber(rawHint?.limit_delta),
    reason: stringValue(rawHint?.reason),
  }
}

function quotaRequestUrl(hint: NormalizedQuotaRequestHint) {
  const params = new URLSearchParams()
  params.set('request_quota', '1')
  if (hint.policyId) params.set('policy_id', String(hint.policyId))
  if (hint.projectId) params.set('project_id', String(hint.projectId))
  if (hint.limitDelta) params.set('limit_delta', String(hint.limitDelta))
  if (hint.reason) params.set('reason', hint.reason)
  return `/quota-requests?${params.toString()}`
}

function showResponseErrorToast(error: unknown) {
  const msg = responseErrorMessage(error)
  const quotaRequestHint = enterpriseQuotaRequestHintFromError(error)
  if (!quotaRequestHint) {
    toast.error(msg)
    return
  }
  toast.error(msg, {
    action: {
      label: t('Request quota'),
      onClick: () => {
        window.location.assign(quotaRequestUrl(quotaRequestHint))
      },
    },
  })
}

// ============================================================================
// Response Interceptor
// ============================================================================

// Handle business logic errors and HTTP errors globally
api.interceptors.response.use(
  (response) => {
    const skipBusiness = response.config.skipBusinessError

    // Unified business response format: { success, message, data }
    if (
      !skipBusiness &&
      response &&
      response.data &&
      typeof response.data.success === 'boolean'
    ) {
      if (!response.data.success) {
        // Show error toast for business failures
        const msg = response.data.message || t('Request failed')
        toast.error(msg)
      }
    }
    return response
  },
  (error) => {
    const skip = error?.config?.skipErrorHandler
    const status = error?.response?.status

    if (status === 401) {
      try {
        useAuthStore.getState().auth.reset()
      } catch {
        /* empty */
      }

      if (!skip) {
        toast.error(t('Session expired!'))
      }
    } else if (!skip) {
      showResponseErrorToast(error)
    }
    return Promise.reject(error)
  }
)

// ============================================================================
// Common Headers Utility
// ============================================================================

/**
 * Get user ID from localStorage
 */
function getUserId(): string | null {
  try {
    if (typeof window !== 'undefined') {
      return window.localStorage.getItem('uid')
    }
  } catch {
    /* empty */
  }
  return null
}

/**
 * Get common request headers (for both axios and SSE requests)
 */
export function getCommonHeaders(): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }

  const uid = getUserId()
  if (uid) {
    headers['New-Api-User'] = uid
  }

  return headers
}

// ============================================================================
// Request Interceptor
// ============================================================================

// Attach user ID header for all requests
api.interceptors.request.use((config) => {
  const uid = getUserId()
  if (uid) {
    // Custom header for user identification
    ;(config.headers as Record<string, string>)['New-Api-User'] = uid
  }
  return config
})

// ============================================================================
// Common API Functions
// ============================================================================

// ----------------------------------------------------------------------------
// User APIs
// ----------------------------------------------------------------------------

// Get current user info
export async function getSelf() {
  const res = await api.get('/api/user/self', {
    // Avoid global 401 toast during guards/preloads
    skipErrorHandler: true,
  })
  return res.data
}

// Get user available models
export async function getUserModels(): Promise<{
  success: boolean
  message?: string
  data?: string[]
}> {
  const res = await api.get('/api/user/models')
  return res.data
}

// Get user groups with descriptions and ratios
export async function getUserGroups(): Promise<{
  success: boolean
  message?: string
  data?: Record<string, { desc: string; ratio: number | string }>
}> {
  const res = await api.get('/api/user/self/groups')
  return res.data
}

// ----------------------------------------------------------------------------
// System APIs
// ----------------------------------------------------------------------------

// Get system status
export async function getStatus() {
  const res = await api.get('/api/status')
  return res.data?.data as Record<string, unknown>
}

// Get system notice
export async function getNotice(): Promise<{
  success: boolean
  message?: string
  data?: string
}> {
  const res = await api.get('/api/notice')
  return res.data
}

export interface EnterpriseQuotaRequestNotification {
  key: string
  kind: 'enterprise_quota_request'
  title: string
  content: string
  title_key?: string
  content_key?: string
  content_params?: Record<string, string>
  status: 'pending' | 'approved' | 'rejected' | 'withdrawn' | 'expired' | string
  read: boolean
  quota_request_id: number
  audit_log_id: number
  policy_name: string
  applicant_name: string
  actor_name: string
  limit_delta: number
  expires_at: number
  created_at: number
}

export interface ConnectedAppRequestNotification {
  key: string
  kind: 'connected_app_request'
  title: string
  content: string
  title_key?: string
  content_key?: string
  content_params?: Record<string, string>
  status: 'pending' | 'approved' | 'rejected' | string
  read: boolean
  request_id: number
  app_id: number
  audit_log_id: number
  slug: string
  app_name: string
  applicant_name: string
  actor_name: string
  requested_scopes: string[]
  created_at: number
}

export type ApprovalNotification =
  | EnterpriseQuotaRequestNotification
  | ConnectedAppRequestNotification

export interface EnterpriseQuotaRequestNotificationQuery {
  page?: number
  page_size?: number
  limit?: number
  unread_only?: boolean
}

export async function getEnterpriseQuotaRequestNotifications(
  params?: EnterpriseQuotaRequestNotificationQuery
): Promise<{
  success: boolean
  message?: string
  data?: {
    items: EnterpriseQuotaRequestNotification[]
    unread_count: number
    page: number
    page_size: number
    has_more: boolean
  }
}> {
  const res = await api.get('/api/notifications/enterprise-quota-requests', {
    params,
    skipErrorHandler: true,
  })
  return res.data
}

export async function getConnectedAppRequestNotifications(
  params?: EnterpriseQuotaRequestNotificationQuery
): Promise<{
  success: boolean
  message?: string
  data?: {
    items: ConnectedAppRequestNotification[]
    unread_count: number
    page: number
    page_size: number
    has_more: boolean
  }
}> {
  const res = await api.get('/api/notifications/connected-app-requests', {
    params,
    skipErrorHandler: true,
  })
  return res.data
}

export async function markEnterpriseQuotaRequestNotificationsRead(
  keys: string[]
): Promise<{
  success: boolean
  message?: string
  data?: {
    enterprise_notification_keys?: string[]
    enterprise_quota_request_keys?: string[]
  }
}> {
  const res = await api.post(
    '/api/notifications/enterprise-quota-requests/read',
    {
      enterprise_notification_keys: keys,
    },
    { skipErrorHandler: true }
  )
  return res.data
}

export async function markConnectedAppRequestNotificationsRead(
  keys: string[]
): Promise<{
  success: boolean
  message?: string
  data?: {
    enterprise_notification_keys?: string[]
    connected_app_request_keys?: string[]
    connected_app_request_event_keys?: string[]
  }
}> {
  const res = await api.post(
    '/api/notifications/connected-app-requests/read',
    {
      connected_app_request_keys: keys,
    },
    { skipErrorHandler: true }
  )
  return res.data
}

// ----------------------------------------------------------------------------
// 2FA Management APIs
// ----------------------------------------------------------------------------

// Get 2FA status
export async function get2FAStatus() {
  const res = await api.get('/api/user/2fa/status')
  return res.data
}

// Setup 2FA
export async function setup2FA() {
  const res = await api.post('/api/user/2fa/setup')
  return res.data
}

// Enable 2FA with verification code
export async function enable2FA(code: string) {
  const res = await api.post('/api/user/2fa/enable', { code })
  return res.data
}

// Disable 2FA with verification code
export async function disable2FA(code: string) {
  const res = await api.post('/api/user/2fa/disable', { code })
  return res.data
}

// Regenerate 2FA backup codes
export async function regenerate2FABackupCodes(code: string) {
  const res = await api.post('/api/user/2fa/backup_codes', { code })
  return res.data
}
