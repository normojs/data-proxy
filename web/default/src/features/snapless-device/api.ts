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

export type SnaplessDeviceStatus =
  | 'pending'
  | 'authorized'
  | 'consumed'
  | 'expired'
  | 'denied'
  | string

export type SnaplessDeviceInfo = {
  fingerprint: string
  device_name: string
  platform: string
  app_version: string
  client: string
}

export type SnaplessTokenSummary = {
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

export type SnaplessApp = {
  id: number
  slug: string
  name: string
  trusted: boolean
  status: number
  allowed_scopes?: string[]
  default_scopes: string[]
}

export type SnaplessGrant = {
  id?: number
  status: string
  scopes: string[]
  authorized_at?: number
  last_used_at?: number
  revoked_at?: number
}

export type SnaplessModelAliases = {
  asr: string
  chat: string
  polish: string
  translate: string
  qa: string
}

export type SnaplessModelHealth = {
  model: string
  available: boolean
}

export type SnaplessActionLink = {
  label: string
  href: string
  intent?: string
}

export type SnaplessActionHints = {
  severity?: 'success' | 'info' | 'warning' | 'danger' | 'neutral' | string
  reason?: string
  primary?: SnaplessActionLink
  secondary?: SnaplessActionLink
}

export type SnaplessHealthStatus =
  | 'ok'
  | 'missing_token'
  | 'token_not_found'
  | 'not_snapless_token'
  | 'app_disabled'
  | 'token_disabled'
  | 'token_expired'
  | 'token_quota_insufficient'
  | 'user_disabled'
  | 'grant_revoked'
  | 'binding_revoked'
  | 'quota_insufficient'
  | 'models_unavailable'
  | string

export type SnaplessManagedDevice = {
  ok: boolean
  status: SnaplessHealthStatus
  actions?: SnaplessActionHints
  device: SnaplessDeviceInfo
  token: SnaplessTokenSummary
  checks: Record<string, boolean>
  last_used_at?: number
  revoked_at?: number
  created_at?: number
  updated_at?: number
}

export type SnaplessReadiness = {
  ok: boolean
  status: SnaplessHealthStatus
  actions?: SnaplessActionHints
  checks: Record<string, boolean>
}

export type SnaplessDeviceStatusResponse = {
  status: SnaplessDeviceStatus
  expires_at: number
  app: SnaplessApp
  device: SnaplessDeviceInfo
  token: SnaplessTokenSummary
  readiness?: SnaplessReadiness
}

export type SnaplessDevicesResponse = {
  ok: boolean
  status: SnaplessHealthStatus
  actions?: SnaplessActionHints
  app: SnaplessApp
  grant: SnaplessGrant
  devices: SnaplessManagedDevice[]
  models: Record<string, SnaplessModelHealth>
  aliases: SnaplessModelAliases
  base_url: string
  checks: Record<string, boolean>
}

export type SnaplessTokenResponse = {
  app: SnaplessApp
  grant: SnaplessGrant
  device: SnaplessDeviceInfo
  token: SnaplessTokenSummary
  models: SnaplessModelAliases
  endpoints: Record<string, string>
  base_url: string
  api_key?: string
  created: boolean
  rotated: boolean
  api_key_once: boolean
}

export type SnaplessRevokeResponse = {
  revoked: boolean
  token_id: number
  grant_revoked: boolean
  device: SnaplessDeviceInfo
}

function unwrap<T>(response: ApiEnvelope<T>): T {
  if (!response.success || response.data == null) {
    throw new Error(response.message || 'Request failed')
  }
  return response.data
}

function getDeviceFlowBasePath(appSlug?: string) {
  const normalized = appSlug?.trim()
  if (!normalized || normalized === 'snapless') {
    return '/api/snapless'
  }
  return `/api/connected-apps/${encodeURIComponent(normalized)}`
}

export async function getSnaplessDeviceStatus(
  userCode: string,
  appSlug?: string
): Promise<SnaplessDeviceStatusResponse> {
  const res = await api.get<ApiEnvelope<SnaplessDeviceStatusResponse>>(
    `${getDeviceFlowBasePath(appSlug)}/device/status`,
    {
      params: { user_code: userCode },
      skipBusinessError: true,
    }
  )
  return unwrap(res.data)
}

export async function authorizeSnaplessDevice(
  input: {
    user_code: string
    approve: boolean
  },
  appSlug?: string
): Promise<SnaplessDeviceStatusResponse> {
  const res = await api.post<ApiEnvelope<SnaplessDeviceStatusResponse>>(
    `${getDeviceFlowBasePath(appSlug)}/device/authorize`,
    input,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function getSnaplessDevices(): Promise<SnaplessDevicesResponse> {
  const res = await api.get<ApiEnvelope<SnaplessDevicesResponse>>(
    '/api/snapless/devices',
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function rotateSnaplessDevice(
  fingerprint: string
): Promise<SnaplessTokenResponse> {
  const res = await api.post<ApiEnvelope<SnaplessTokenResponse>>(
    `/api/snapless/devices/${encodeURIComponent(fingerprint)}/rotate`,
    undefined,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}

export async function revokeSnaplessDevice(
  fingerprint: string
): Promise<SnaplessRevokeResponse> {
  const res = await api.delete<ApiEnvelope<SnaplessRevokeResponse>>(
    `/api/snapless/devices/${encodeURIComponent(fingerprint)}`,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}
