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
  model_limits?: string
}

export type SnaplessDeviceStatusResponse = {
  status: SnaplessDeviceStatus
  expires_at: number
  app: {
    id: number
    slug: string
    name: string
    trusted: boolean
    status: number
    default_scopes: string[]
  }
  device: SnaplessDeviceInfo
  token: SnaplessTokenSummary
}

function unwrap<T>(response: ApiEnvelope<T>): T {
  if (!response.success || response.data == null) {
    throw new Error(response.message || 'Request failed')
  }
  return response.data
}

export async function getSnaplessDeviceStatus(
  userCode: string
): Promise<SnaplessDeviceStatusResponse> {
  const res = await api.get<ApiEnvelope<SnaplessDeviceStatusResponse>>(
    '/api/snapless/device/status',
    {
      params: { user_code: userCode },
      skipBusinessError: true,
    }
  )
  return unwrap(res.data)
}

export async function authorizeSnaplessDevice(input: {
  user_code: string
  approve: boolean
}): Promise<SnaplessDeviceStatusResponse> {
  const res = await api.post<ApiEnvelope<SnaplessDeviceStatusResponse>>(
    '/api/snapless/device/authorize',
    input,
    { skipBusinessError: true }
  )
  return unwrap(res.data)
}
