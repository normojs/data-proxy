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
import { buildQueryParams } from './lib/utils'
import type {
  GetLogsParams,
  GetLogsResponse,
  GetLogFilterOptionsParams,
  GetLogFilterOptionsResponse,
  GetLogStatsParams,
  GetLogStatsResponse,
  GetRequestDiagnosticCandidatesParams,
  GetRequestDiagnosticCandidatesResponse,
  GetMidjourneyLogsParams,
  GetRequestDiagnosticReportResponse,
  GetRequestLogTraceResponse,
  GetTaskLogsParams,
  UserInfo,
} from './types'

// ============================================================================
// Generic API Helpers
// ============================================================================

function buildApiPath(endpoint: string, isAdmin: boolean): string {
  return isAdmin ? endpoint : `${endpoint}/self`
}

async function fetchLogs<T>(
  endpoint: string,
  params: T,
  isAdmin: boolean
): Promise<GetLogsResponse> {
  const paramRecord = params as unknown as Record<string, unknown>
  const queryParams = buildQueryParams({
    p: paramRecord.p || 1,
    page_size: paramRecord.page_size || 20,
    ...params,
  })
  const path = buildApiPath(endpoint, isAdmin)
  const res = await api.get(`${path}?${queryParams}`)
  return res.data
}

async function fetchLogStats<T>(
  endpoint: string,
  params: T,
  isAdmin: boolean
): Promise<GetLogStatsResponse> {
  const queryParams = buildQueryParams(
    params as unknown as Record<string, unknown>
  )
  const path = buildApiPath(endpoint, isAdmin)
  const res = await api.get(`${path}/stat?${queryParams}`)
  return res.data
}

// ============================================================================
// Common Log APIs
// ============================================================================

export const getAllLogs = (params: GetLogsParams = {}) =>
  fetchLogs('/api/log', params, true)

export const getUserLogs = (
  params: Omit<GetLogsParams, 'username' | 'channel'> = {}
) => fetchLogs('/api/log', params, false)

export const getLogStats = (params: GetLogStatsParams = {}) =>
  fetchLogStats('/api/log', params, true)

export const getUserLogStats = (
  params: Omit<GetLogStatsParams, 'username' | 'channel'> = {}
) => fetchLogStats('/api/log', params, false)

export async function getLogFilterOptions(
  params: GetLogFilterOptionsParams = {},
  isAdmin: boolean
): Promise<GetLogFilterOptionsResponse> {
  const queryParams = buildQueryParams(params as Record<string, unknown>)
  const path = isAdmin
    ? '/api/log/filter-options'
    : '/api/log/self/filter-options'
  const res = await api.get(`${path}?${queryParams}`)
  return res.data
}

export async function getRequestLogTrace(
  requestId: string,
  isAdmin: boolean,
  subsiteId?: number
): Promise<GetRequestLogTraceResponse> {
  const path = isAdmin ? '/api/log/request' : '/api/log/self/request'
  const res = await api.get(path, {
    params: {
      request_id: requestId,
      subsite_id: subsiteId,
    },
  })
  return res.data
}

export async function getRequestDiagnosticReport(
  requestId: string,
  subsiteId?: number
): Promise<GetRequestDiagnosticReportResponse> {
  const res = await api.get(
    `/api/log/request/${encodeURIComponent(requestId)}/diagnostic`,
    { params: { subsite_id: subsiteId } }
  )
  return res.data
}

export async function generateRequestDiagnosticReport(
  requestId: string,
  subsiteId?: number
): Promise<GetRequestDiagnosticReportResponse> {
  const res = await api.post(
    `/api/log/request/${encodeURIComponent(requestId)}/diagnostic`,
    undefined,
    { params: { subsite_id: subsiteId } }
  )
  return res.data
}

export async function getRequestDiagnosticCandidates(
  params: GetRequestDiagnosticCandidatesParams = {}
): Promise<GetRequestDiagnosticCandidatesResponse> {
  const queryParams = buildQueryParams({
    limit: params.limit || 20,
    start_timestamp: params.start_timestamp,
    end_timestamp: params.end_timestamp,
    severity: params.severity,
    source: params.source,
    model_name: params.model_name,
    channel_id: params.channel_id,
    group: params.group,
    report_status: params.report_status,
    user_id: params.user_id,
    token_id: params.token_id,
    subsite_id: params.subsite_id,
  })
  const res = await api.get(
    `/api/log/request-diagnostic-candidates?${queryParams}`
  )
  return res.data
}

export function getRequestDiagnosticBundleUrl(
  requestId: string,
  subsiteId?: number
): string {
  const queryParams = buildQueryParams({ subsite_id: subsiteId })
  const query = queryParams.toString()
  const basePath = `/api/log/request/${encodeURIComponent(requestId)}/diagnostic/bundle`
  return query ? `${basePath}?${query}` : basePath
}

export async function getUserInfo(
  userId: number
): Promise<{ success: boolean; message?: string; data?: UserInfo }> {
  const res = await api.get(`/api/user/${userId}`)
  return res.data
}

// ============================================================================
// Midjourney (Drawing) Logs API
// ============================================================================

export const getAllMidjourneyLogs = (params: GetMidjourneyLogsParams) =>
  fetchLogs('/api/mj', params, true)

export const getUserMidjourneyLogs = (params: GetMidjourneyLogsParams) =>
  fetchLogs('/api/mj', params, false)

// ============================================================================
// Task Logs API
// ============================================================================

export const getAllTaskLogs = (params: GetTaskLogsParams) =>
  fetchLogs('/api/task', params, true)

export const getUserTaskLogs = (params: GetTaskLogsParams) =>
  fetchLogs('/api/task', params, false)
