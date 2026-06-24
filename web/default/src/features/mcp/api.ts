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
import type {
  ApiResponse,
  BillingEvent,
  BillingEventBackfillRequest,
  BillingEventBackfillResponse,
  BillingEventHealth,
  BillingEventRelationBackfillRequest,
  BillingEventRelationBackfillResponse,
  BillingEventRelationHealth,
  BillingEventRelationInspectionRunResponse,
  BillingEventRelationInspectionRunItem,
  BillingEventRelationInspectionRunListParams,
  BillingEventRelationInspectionSettingsRequest,
  BillingEventRelationInspectionStatus,
  BillingEventRelationOrphanCleanupRequest,
  BillingEventRelationOrphanCleanupResponse,
  BillingEventRelationRepairRequest,
  BillingEventRelationRepairResponse,
  BillingEventReconciliationBackfillMissingRequest,
  BillingEventReconciliationBackfillMissingResponse,
  BillingEventReconciliationMissingRequest,
  BillingEventReconciliationMissingResponse,
  BillingEventReconciliationMismatchRequest,
  BillingEventReconciliationMismatchResponse,
  BillingEventReconciliationRepairRequest,
  BillingEventReconciliationRepairResponse,
  BillingEventReconciliationRequest,
  BillingEventReconciliationResponse,
  BillingEventSourceMatrix,
  BillingEventSummary,
  BillingEventListParams,
  BridgeAuditLog,
  BridgeAuditLogListParams,
  BridgeAgentSetupPayload,
  BridgeAgentSetupResponse,
  BridgeAgentSetupTokenPayload,
  BridgeAgentSetupTokenResponse,
  BridgeClient,
  BridgeClientDetail,
  BridgeClientHealth,
  BridgeClientListParams,
  BridgeClientUpdatePayload,
  BridgeSession,
  TunnelApp,
  TunnelAppCreatePayload,
  TunnelAppAdminUpdatePayload,
  TunnelAppListParams,
  TunnelConnection,
  TunnelConnectionCreatePayload,
  TunnelConnectionCreateResponse,
  TunnelConnectionListParams,
  TunnelConnectionUpdatePayload,
  TunnelSession,
  TunnelSessionListParams,
  TunnelAuditLog,
  TunnelAuditLogListParams,
  TunnelAgentSetupPayload,
  TunnelAgentSetupResponse,
  MCPSummary,
  MCPSummaryParams,
  MCPProxyDiscoveryEvent,
  MCPProxyDiscoveryResult,
  MCPProxyHeartbeatRunResponse,
  MCPProxyHeartbeatSettings,
  MCPProxyHeartbeatStatus,
  MCPProxyHealthCheckRunPayload,
  MCPProxyHealthCheckRunResponse,
  MCPProxyHealthCheckSettings,
  MCPProxyHealthCheckStatus,
  MCPProxyServerHealth,
  MCPProxyServer,
  MCPProxyServerListParams,
  MCPProxyServerPayload,
  MCPProxyServerTestResult,
  MCPProxyTool,
  MCPProxyToolHealth,
  MCPProxyToolListParams,
  MCPProxyToolUpdatePayload,
  MCPProxyTrendParams,
  MCPProxyTrendResponse,
  MCPTool,
  MCPToolCall,
  MCPToolCallListParams,
  MCPToolCreatePayload,
  MCPToolListParams,
  MCPToolUpdatePayload,
  MCPOpenAPIImportPayload,
  MCPOpenAPIImportResponse,
  MCPOpenAPIBinaryCleanupPayload,
  MCPOpenAPIBinaryCleanupResponse,
  MCPOpenAPIBinaryObject,
  MCPOpenAPIBinaryObjectListParams,
  MCPOpenAPIBinaryObjectSummary,
  MCPOpenAPILifecyclePayload,
  MCPOpenAPILifecycleResponse,
  MCPOpenAPIPreviewPayload,
  MCPOpenAPIPreviewResponse,
  PaginatedResponse,
} from './types'

function buildQueryParams(params: Record<string, unknown>): string {
  const query = new URLSearchParams()

  Object.entries(params).forEach(([key, value]) => {
    if (value == null || value === '') return
    if (Array.isArray(value)) {
      value.forEach((item) => {
        if (item == null || item === '') return
        query.append(key, String(item))
      })
      return
    }
    query.set(key, String(value))
  })

  return query.toString()
}

async function getPaginated<T>(
  endpoint: string,
  params: Record<string, unknown>
): Promise<PaginatedResponse<T>> {
  const query = buildQueryParams(params)
  const res = await api.get(query ? `${endpoint}?${query}` : endpoint)
  return res.data
}

export const mcpQueryKeys = {
  all: ['mcp'] as const,
  tools: () => [...mcpQueryKeys.all, 'tools'] as const,
  toolsList: (filters: MCPToolListParams) =>
    [...mcpQueryKeys.tools(), 'list', filters] as const,
  toolDetail: (id: number) => [...mcpQueryKeys.tools(), 'detail', id] as const,
  openAPI: () => [...mcpQueryKeys.all, 'openapi'] as const,
  openAPIBinaryObjects: () =>
    [...mcpQueryKeys.openAPI(), 'binary-objects'] as const,
  openAPIBinaryObjectsList: (filters: MCPOpenAPIBinaryObjectListParams) =>
    [...mcpQueryKeys.openAPIBinaryObjects(), 'list', filters] as const,
  openAPIBinaryObjectsSummary: (filters: MCPOpenAPIBinaryObjectListParams) =>
    [...mcpQueryKeys.openAPIBinaryObjects(), 'summary', filters] as const,
  proxyServers: () => [...mcpQueryKeys.all, 'proxy-servers'] as const,
  proxyServersList: (filters: MCPProxyServerListParams) =>
    [...mcpQueryKeys.proxyServers(), 'list', filters] as const,
  proxyTrends: (filters: MCPProxyTrendParams) =>
    [...mcpQueryKeys.proxyServers(), 'trends', filters] as const,
  proxyTools: () => [...mcpQueryKeys.all, 'proxy-tools'] as const,
  proxyToolsList: (filters: MCPProxyToolListParams) =>
    [...mcpQueryKeys.proxyTools(), 'list', filters] as const,
  proxyToolDetail: (id: number) =>
    [...mcpQueryKeys.proxyTools(), 'detail', id] as const,
  proxyToolHealth: (id: number, filters: { window_seconds?: number }) =>
    [...mcpQueryKeys.proxyTools(), 'health', id, filters] as const,
  proxyServerTools: (id: number) =>
    [...mcpQueryKeys.proxyServers(), 'tools', id] as const,
  proxyServerHealth: (id: number, filters: { window_seconds?: number }) =>
    [...mcpQueryKeys.proxyServers(), 'health', id, filters] as const,
  proxyServerDiscoveryEvents: (
    id: number,
    filters: { p?: number; page_size?: number }
  ) =>
    [...mcpQueryKeys.proxyServers(), 'discovery-events', id, filters] as const,
  proxyHealthCheck: () =>
    [...mcpQueryKeys.proxyServers(), 'health-check'] as const,
  proxyHeartbeat: () => [...mcpQueryKeys.proxyServers(), 'heartbeat'] as const,
  summary: (filters: MCPSummaryParams) =>
    [...mcpQueryKeys.all, 'summary', filters] as const,
  toolCalls: () => [...mcpQueryKeys.all, 'tool-calls'] as const,
  toolCallsList: (filters: MCPToolCallListParams) =>
    [...mcpQueryKeys.toolCalls(), 'list', filters] as const,
  bridgeClients: () => [...mcpQueryKeys.all, 'bridge-clients'] as const,
  bridgeClientsList: (filters: BridgeClientListParams) =>
    [...mcpQueryKeys.bridgeClients(), 'list', filters] as const,
  bridgeClientDetail: (clientId: string, filters: { scope?: 'all' }) =>
    [...mcpQueryKeys.bridgeClients(), 'detail', clientId, filters] as const,
  bridgeClientHealth: (
    clientId: string,
    filters: { scope?: 'all'; window_seconds?: number }
  ) => [...mcpQueryKeys.bridgeClients(), 'health', clientId, filters] as const,
  tunnelApps: () => [...mcpQueryKeys.all, 'tunnel-apps'] as const,
  tunnelAppsList: (filters: TunnelAppListParams) =>
    [...mcpQueryKeys.tunnelApps(), 'list', filters] as const,
  userTunnelAppsList: (filters: TunnelAppListParams) =>
    [...mcpQueryKeys.tunnelApps(), 'user-list', filters] as const,
  tunnelConnections: (appId: number) =>
    [...mcpQueryKeys.tunnelApps(), 'connections', appId] as const,
  tunnelConnectionsList: (appId: number, filters: TunnelConnectionListParams) =>
    [...mcpQueryKeys.tunnelConnections(appId), 'list', filters] as const,
  tunnelSessionsList: (appId: number, filters: TunnelSessionListParams) =>
    [...mcpQueryKeys.tunnelApps(), 'sessions', appId, filters] as const,
  tunnelAuditLogsList: (appId: number, filters: TunnelAuditLogListParams) =>
    [...mcpQueryKeys.tunnelApps(), 'audit-logs', appId, filters] as const,
  auditLogs: () => [...mcpQueryKeys.all, 'audit-logs'] as const,
  auditLogsList: (filters: BridgeAuditLogListParams) =>
    [...mcpQueryKeys.auditLogs(), 'list', filters] as const,
  billingEvents: () => [...mcpQueryKeys.all, 'billing-events'] as const,
  billingEventsList: (filters: BillingEventListParams) =>
    [...mcpQueryKeys.billingEvents(), 'list', filters] as const,
  billingEventsSummary: (filters: BillingEventListParams) =>
    [...mcpQueryKeys.billingEvents(), 'summary', filters] as const,
  billingEventsHealth: (filters: { sources?: string[]; limit?: number }) =>
    [...mcpQueryKeys.billingEvents(), 'health', filters] as const,
  billingEventsSourceMatrix: () =>
    [...mcpQueryKeys.billingEvents(), 'source-matrix'] as const,
  billingEventsRelationHealth: (filters: { limit?: number; cursor?: number }) =>
    [...mcpQueryKeys.billingEvents(), 'relation-health', filters] as const,
  billingEventsRelationInspection: () =>
    [...mcpQueryKeys.billingEvents(), 'relation-inspection'] as const,
  billingEventsRelationInspectionRuns: (
    filters: BillingEventRelationInspectionRunListParams
  ) =>
    [
      ...mcpQueryKeys.billingEvents(),
      'relation-inspection-runs',
      filters,
    ] as const,
  billingEventsReconciliation: (filters: BillingEventReconciliationRequest) =>
    [...mcpQueryKeys.billingEvents(), 'reconciliation', filters] as const,
  billingEventsReconciliationMismatches: (
    filters: BillingEventReconciliationMismatchRequest
  ) =>
    [
      ...mcpQueryKeys.billingEvents(),
      'reconciliation-mismatches',
      filters,
    ] as const,
}

export function listMCPTools(params: MCPToolListParams) {
  return getPaginated<MCPTool>('/api/mcp/tools', params)
}

export function listTunnelApps(params: TunnelAppListParams) {
  return getPaginated<TunnelApp>('/api/tunnel/admin/apps', params)
}

export function listUserTunnelApps(params: TunnelAppListParams) {
  return getPaginated<TunnelApp>('/api/tunnel/apps', params)
}

export async function createTunnelApp(
  payload: TunnelAppCreatePayload
): Promise<ApiResponse<TunnelApp>> {
  const res = await api.post('/api/tunnel/apps', payload)
  return res.data
}

export function listTunnelConnections(
  appId: number,
  params: TunnelConnectionListParams
) {
  return getPaginated<TunnelConnection>(
    `/api/tunnel/apps/${appId}/connections`,
    params
  )
}

export function listTunnelSessions(
  appId: number,
  params: TunnelSessionListParams
) {
  return getPaginated<TunnelSession>(
    `/api/tunnel/apps/${appId}/sessions`,
    params
  )
}

export async function createTunnelConnection(
  appId: number,
  payload: TunnelConnectionCreatePayload
): Promise<ApiResponse<TunnelConnectionCreateResponse>> {
  const res = await api.post(`/api/tunnel/apps/${appId}/connections`, payload)
  return res.data
}

export async function updateTunnelConnection(
  appId: number,
  connectionId: number,
  payload: TunnelConnectionUpdatePayload
): Promise<ApiResponse<TunnelConnection>> {
  const res = await api.patch(
    `/api/tunnel/apps/${appId}/connections/${connectionId}`,
    payload
  )
  return res.data
}

export async function revokeTunnelConnection(
  appId: number,
  connectionId: number
): Promise<ApiResponse<TunnelConnection>> {
  const res = await api.delete(
    `/api/tunnel/apps/${appId}/connections/${connectionId}`
  )
  return res.data
}

export async function ensureTunnelAgentSetup(
  appId: number,
  payload: TunnelAgentSetupPayload
): Promise<ApiResponse<TunnelAgentSetupResponse>> {
  const res = await api.post(`/api/tunnel/apps/${appId}/agent-setup`, payload)
  return res.data
}

export async function ensureBridgeAgentSetup(
  payload: BridgeAgentSetupPayload
): Promise<ApiResponse<BridgeAgentSetupResponse>> {
  const res = await api.post('/api/bridge/agent-setup', payload)
  return res.data
}

export async function createBridgeAgentSetupToken(
  payload: BridgeAgentSetupTokenPayload
): Promise<ApiResponse<BridgeAgentSetupTokenResponse>> {
  const res = await api.post('/api/bridge/agent-setup-tokens', payload)
  return res.data
}

export function listTunnelAuditLogs(
  appId: number,
  params: TunnelAuditLogListParams
) {
  return getPaginated<TunnelAuditLog>(
    `/api/tunnel/apps/${appId}/audit-logs`,
    params
  )
}

export async function updateTunnelApp(
  id: number,
  payload: TunnelAppAdminUpdatePayload
): Promise<ApiResponse<TunnelApp>> {
  const res = await api.patch(`/api/tunnel/admin/apps/${id}`, payload)
  return res.data
}

export async function getMCPTool(id: number): Promise<ApiResponse<MCPTool>> {
  const res = await api.get(`/api/mcp/tools/${id}`)
  return res.data
}

export async function updateMCPTool(
  id: number,
  payload: MCPToolUpdatePayload
): Promise<ApiResponse<MCPTool>> {
  const res = await api.patch(`/api/mcp/tools/${id}`, payload)
  return res.data
}

export async function createMCPTool(
  payload: MCPToolCreatePayload
): Promise<ApiResponse<MCPTool>> {
  const res = await api.post('/api/mcp/tools', payload)
  return res.data
}

export async function archiveMCPTool(
  id: number
): Promise<ApiResponse<MCPTool>> {
  const res = await api.post(`/api/mcp/tools/${id}/archive`)
  return res.data
}

export async function deleteMCPTool(id: number): Promise<ApiResponse<MCPTool>> {
  const res = await api.delete(`/api/mcp/tools/${id}`)
  return res.data
}

export async function previewMCPOpenAPI(
  payload: MCPOpenAPIPreviewPayload
): Promise<ApiResponse<MCPOpenAPIPreviewResponse>> {
  const res = await api.post('/api/mcp/openapi/preview', payload)
  return res.data
}

export async function importMCPOpenAPI(
  payload: MCPOpenAPIImportPayload
): Promise<ApiResponse<MCPOpenAPIImportResponse>> {
  const res = await api.post('/api/mcp/openapi/import', payload)
  return res.data
}

export async function diffMCPOpenAPI(
  payload: MCPOpenAPIImportPayload
): Promise<ApiResponse<MCPOpenAPIImportResponse>> {
  const res = await api.post('/api/mcp/openapi/diff', payload)
  return res.data
}

export async function disableMCPOpenAPI(
  payload: MCPOpenAPILifecyclePayload
): Promise<ApiResponse<MCPOpenAPILifecycleResponse>> {
  const res = await api.post('/api/mcp/openapi/disable', payload)
  return res.data
}

export async function deleteMCPOpenAPI(
  payload: MCPOpenAPILifecyclePayload
): Promise<ApiResponse<MCPOpenAPILifecycleResponse>> {
  const res = await api.delete('/api/mcp/openapi/', { data: payload })
  return res.data
}

export function listMCPOpenAPIBinaryObjects(
  params: MCPOpenAPIBinaryObjectListParams
) {
  return getPaginated<MCPOpenAPIBinaryObject>('/api/mcp/openapi/binary', params)
}

export async function getMCPOpenAPIBinaryObjectSummary(
  params: MCPOpenAPIBinaryObjectListParams
): Promise<ApiResponse<MCPOpenAPIBinaryObjectSummary>> {
  const query = buildQueryParams(params)
  const res = await api.get(
    `/api/mcp/openapi/binary/summary${query ? `?${query}` : ''}`
  )
  return res.data
}

export async function cleanupMCPOpenAPIBinaryObjects(
  payload: MCPOpenAPIBinaryCleanupPayload
): Promise<ApiResponse<MCPOpenAPIBinaryCleanupResponse>> {
  const res = await api.post('/api/mcp/openapi/binary/cleanup', payload)
  return res.data
}

export async function seedMCPTools(): Promise<
  ApiResponse<{ seeded: boolean }>
> {
  const res = await api.post('/api/mcp/tools/seed')
  return res.data
}

export function listMCPProxyServers(params: MCPProxyServerListParams) {
  return getPaginated<MCPProxyServer>('/api/mcp/proxy/servers', params)
}

export async function getMCPProxyTrends(
  params: MCPProxyTrendParams
): Promise<ApiResponse<MCPProxyTrendResponse>> {
  const query = buildQueryParams(params)
  const res = await api.get(
    `/api/mcp/proxy/servers/trends${query ? `?${query}` : ''}`
  )
  return res.data
}

export async function getMCPProxyServer(
  id: number
): Promise<ApiResponse<MCPProxyServer>> {
  const res = await api.get(`/api/mcp/proxy/servers/${id}`)
  return res.data
}

export async function createMCPProxyServer(
  payload: MCPProxyServerPayload
): Promise<ApiResponse<MCPProxyServer>> {
  const res = await api.post('/api/mcp/proxy/servers', payload)
  return res.data
}

export async function updateMCPProxyServer(
  id: number,
  payload: MCPProxyServerPayload
): Promise<ApiResponse<MCPProxyServer>> {
  const res = await api.patch(`/api/mcp/proxy/servers/${id}`, payload)
  return res.data
}

export async function deleteMCPProxyServer(
  id: number
): Promise<ApiResponse<MCPProxyServer>> {
  const res = await api.delete(`/api/mcp/proxy/servers/${id}`)
  return res.data
}

export async function testMCPProxyServer(
  id: number
): Promise<ApiResponse<MCPProxyServerTestResult>> {
  const res = await api.post(`/api/mcp/proxy/servers/${id}/test`)
  return res.data
}

export async function discoverMCPProxyServerTools(
  id: number
): Promise<ApiResponse<MCPProxyDiscoveryResult>> {
  const res = await api.post(`/api/mcp/proxy/servers/${id}/discover`)
  return res.data
}

export async function getMCPProxyHealthCheck(): Promise<
  ApiResponse<MCPProxyHealthCheckStatus>
> {
  const res = await api.get('/api/mcp/proxy/servers/health-check')
  return res.data
}

export async function updateMCPProxyHealthCheck(
  payload: MCPProxyHealthCheckSettings
): Promise<ApiResponse<MCPProxyHealthCheckStatus>> {
  const res = await api.put('/api/mcp/proxy/servers/health-check', payload)
  return res.data
}

export async function runMCPProxyHealthCheck(
  payload: MCPProxyHealthCheckRunPayload
): Promise<ApiResponse<MCPProxyHealthCheckRunResponse>> {
  const res = await api.post('/api/mcp/proxy/servers/health-check/run', payload)
  return res.data
}

export async function getMCPProxyHeartbeat(): Promise<
  ApiResponse<MCPProxyHeartbeatStatus>
> {
  const res = await api.get('/api/mcp/proxy/servers/heartbeat')
  return res.data
}

export async function updateMCPProxyHeartbeat(
  payload: MCPProxyHeartbeatSettings
): Promise<ApiResponse<MCPProxyHeartbeatStatus>> {
  const res = await api.put('/api/mcp/proxy/servers/heartbeat', payload)
  return res.data
}

export async function runMCPProxyHeartbeat(): Promise<
  ApiResponse<MCPProxyHeartbeatRunResponse>
> {
  const res = await api.post('/api/mcp/proxy/servers/heartbeat/run')
  return res.data
}

export function listMCPProxyTools(params: MCPProxyToolListParams) {
  return getPaginated<MCPProxyTool>('/api/mcp/proxy/tools', params)
}

export async function getMCPProxyTool(
  id: number
): Promise<ApiResponse<MCPProxyTool>> {
  const res = await api.get(`/api/mcp/proxy/tools/${id}`)
  return res.data
}

export async function listMCPProxyServerTools(
  id: number
): Promise<ApiResponse<MCPProxyTool[]>> {
  const res = await api.get(`/api/mcp/proxy/servers/${id}/tools`)
  return res.data
}

export function listMCPProxyServerDiscoveryEvents(
  id: number,
  params: { p?: number; page_size?: number }
) {
  return getPaginated<MCPProxyDiscoveryEvent>(
    `/api/mcp/proxy/servers/${id}/discovery-events`,
    params
  )
}

export async function getMCPProxyServerHealth(
  id: number,
  params: { window_seconds?: number }
): Promise<ApiResponse<MCPProxyServerHealth>> {
  const query = buildQueryParams(params)
  const res = await api.get(
    `/api/mcp/proxy/servers/${id}/health${query ? `?${query}` : ''}`
  )
  return res.data
}

export async function getMCPProxyToolHealth(
  id: number,
  params: { window_seconds?: number }
): Promise<ApiResponse<MCPProxyToolHealth>> {
  const query = buildQueryParams(params)
  const res = await api.get(
    `/api/mcp/proxy/tools/${id}/health${query ? `?${query}` : ''}`
  )
  return res.data
}

export async function updateMCPProxyTool(
  id: number,
  payload: MCPProxyToolUpdatePayload
): Promise<ApiResponse<MCPProxyTool>> {
  const res = await api.patch(`/api/mcp/proxy/tools/${id}`, payload)
  return res.data
}

export function listMCPToolCalls(params: MCPToolCallListParams) {
  return getPaginated<MCPToolCall>('/api/mcp/tool-calls', params)
}

export async function getMCPSummary(
  params: MCPSummaryParams
): Promise<ApiResponse<MCPSummary>> {
  const query = buildQueryParams(params)
  const res = await api.get(`/api/mcp/summary${query ? `?${query}` : ''}`)
  return res.data
}

export function listBridgeClients(params: BridgeClientListParams) {
  return getPaginated<BridgeClient>('/api/bridge/clients', params)
}

export async function getBridgeClient(
  clientId: string,
  params: { scope?: 'all' }
): Promise<ApiResponse<BridgeClientDetail>> {
  const query = buildQueryParams(params)
  const res = await api.get(
    `/api/bridge/clients/${encodeURIComponent(clientId)}${query ? `?${query}` : ''}`
  )
  return res.data
}

export async function getBridgeClientHealth(
  clientId: string,
  params: { scope?: 'all'; window_seconds?: number }
): Promise<ApiResponse<BridgeClientHealth>> {
  const query = buildQueryParams(params)
  const res = await api.get(
    `/api/bridge/clients/${encodeURIComponent(clientId)}/health${query ? `?${query}` : ''}`
  )
  return res.data
}

export async function updateBridgeClient(
  clientId: string,
  payload: BridgeClientUpdatePayload,
  params: { scope?: 'all' }
): Promise<ApiResponse<BridgeClientDetail>> {
  const query = buildQueryParams(params)
  const res = await api.patch(
    `/api/bridge/clients/${encodeURIComponent(clientId)}${query ? `?${query}` : ''}`,
    payload
  )
  return res.data
}

export async function deleteBridgeClient(
  clientId: string,
  params: { scope?: 'all' }
): Promise<ApiResponse<BridgeClient>> {
  const query = buildQueryParams(params)
  const res = await api.delete(
    `/api/bridge/clients/${encodeURIComponent(clientId)}${query ? `?${query}` : ''}`
  )
  return res.data
}

export async function closeBridgeSession(
  sessionId: string,
  payload: { reason?: string },
  params: { scope?: 'all' }
): Promise<ApiResponse<BridgeSession>> {
  const query = buildQueryParams(params)
  const res = await api.post(
    `/api/bridge/sessions/${encodeURIComponent(sessionId)}/close${query ? `?${query}` : ''}`,
    payload
  )
  return res.data
}

export function listBridgeAuditLogs(params: BridgeAuditLogListParams) {
  return getPaginated<BridgeAuditLog>('/api/bridge/audit-logs', params)
}

export function listBillingEvents(params: BillingEventListParams) {
  return getPaginated<BillingEvent>('/api/billing/events', params)
}

export async function getBillingEventSummary(
  params: BillingEventListParams
): Promise<ApiResponse<BillingEventSummary>> {
  const query = buildQueryParams(params)
  const res = await api.get(
    `/api/billing/events/summary${query ? `?${query}` : ''}`
  )
  return res.data
}

export async function getBillingEventHealth(params: {
  sources?: string[]
  limit?: number
}): Promise<ApiResponse<BillingEventHealth>> {
  const query = buildQueryParams(params)
  const res = await api.get(
    `/api/billing/events/health${query ? `?${query}` : ''}`
  )
  return res.data
}

export async function getBillingEventSourceMatrix(): Promise<
  ApiResponse<BillingEventSourceMatrix>
> {
  const res = await api.get('/api/billing/events/source-matrix')
  return res.data
}

export async function getBillingEventRelationHealth(params: {
  limit?: number
  cursor?: number
}): Promise<ApiResponse<BillingEventRelationHealth>> {
  const query = buildQueryParams(params)
  const res = await api.get(
    `/api/billing/events/relation-health${query ? `?${query}` : ''}`
  )
  return res.data
}

export async function backfillBillingEventRelations(
  payload: BillingEventRelationBackfillRequest
): Promise<ApiResponse<BillingEventRelationBackfillResponse>> {
  const res = await api.post('/api/billing/events/relation-backfill', payload)
  return res.data
}

export async function repairBillingEventRelations(
  payload: BillingEventRelationRepairRequest
): Promise<ApiResponse<BillingEventRelationRepairResponse>> {
  const res = await api.post('/api/billing/events/relation-repair', payload)
  return res.data
}

export async function cleanupBillingEventRelationOrphans(
  payload: BillingEventRelationOrphanCleanupRequest
): Promise<ApiResponse<BillingEventRelationOrphanCleanupResponse>> {
  const res = await api.post(
    '/api/billing/events/relation-orphans/cleanup',
    payload
  )
  return res.data
}

export async function getBillingEventRelationInspection(): Promise<
  ApiResponse<BillingEventRelationInspectionStatus>
> {
  const res = await api.get('/api/billing/events/relation-inspection')
  return res.data
}

export async function updateBillingEventRelationInspection(
  payload: BillingEventRelationInspectionSettingsRequest
): Promise<ApiResponse<BillingEventRelationInspectionStatus>> {
  const res = await api.put('/api/billing/events/relation-inspection', payload)
  return res.data
}

export async function runBillingEventRelationInspection(): Promise<
  ApiResponse<BillingEventRelationInspectionRunResponse>
> {
  const res = await api.post('/api/billing/events/relation-inspection/run')
  return res.data
}

export function listBillingEventRelationInspectionRuns(
  params: BillingEventRelationInspectionRunListParams
) {
  return getPaginated<BillingEventRelationInspectionRunItem>(
    '/api/billing/events/relation-inspection/runs',
    params
  )
}

export async function backfillBillingEvents(
  payload: BillingEventBackfillRequest
): Promise<ApiResponse<BillingEventBackfillResponse>> {
  const res = await api.post('/api/billing/events/backfill', payload)
  return res.data
}

export async function reconcileBillingEvents(
  payload: BillingEventReconciliationRequest
): Promise<ApiResponse<BillingEventReconciliationResponse>> {
  const res = await api.post('/api/billing/events/reconciliation', payload)
  return res.data
}

export async function getBillingEventReconciliationMismatches(
  payload: BillingEventReconciliationMismatchRequest
): Promise<ApiResponse<BillingEventReconciliationMismatchResponse>> {
  const res = await api.post(
    '/api/billing/events/reconciliation/mismatches',
    payload
  )
  return res.data
}

export async function getBillingEventReconciliationMissing(
  payload: BillingEventReconciliationMissingRequest
): Promise<ApiResponse<BillingEventReconciliationMissingResponse>> {
  const res = await api.post(
    '/api/billing/events/reconciliation/missing',
    payload
  )
  return res.data
}

export async function repairBillingEventReconciliationMismatch(
  payload: BillingEventReconciliationRepairRequest
): Promise<ApiResponse<BillingEventReconciliationRepairResponse>> {
  const res = await api.post(
    '/api/billing/events/reconciliation/repair',
    payload
  )
  return res.data
}

export async function backfillBillingEventReconciliationMissing(
  payload: BillingEventReconciliationBackfillMissingRequest
): Promise<ApiResponse<BillingEventReconciliationBackfillMissingResponse>> {
  const res = await api.post(
    '/api/billing/events/reconciliation/backfill-missing',
    payload
  )
  return res.data
}
