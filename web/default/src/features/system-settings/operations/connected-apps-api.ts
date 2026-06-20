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
