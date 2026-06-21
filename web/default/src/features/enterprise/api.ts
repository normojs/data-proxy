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
  Enterprise,
  EnterpriseAnomalyThrottleConfig,
  EnterpriseAnomalyProtection,
  EnterpriseAnomalyProtectionParams,
  EnterpriseAnomalyProtectionTrendItem,
  EnterpriseAnomalyProtectionTrendParams,
  EnterpriseAuditLog,
  EnterpriseAuditLogParams,
  EnterpriseListParams,
  EnterpriseMember,
  EnterpriseMembersParams,
  EnterpriseNotificationOutbox,
  EnterpriseNotificationOutboxParams,
  EnterpriseNotificationOutboxWorkerMetrics,
  EnterpriseNotificationPreference,
  EnterpriseNotificationPreferencePayload,
  EnterpriseOrgUnit,
  EnterpriseOrgUnitPayload,
  EnterpriseOrgSyncPayload,
  EnterpriseOrgSyncResult,
  EnterprisePolicyGroup,
  EnterprisePolicyGroupMembersParams,
  EnterprisePolicyGroupMembersPayload,
  EnterprisePolicyGroupPayload,
  EnterprisePolicyGroupShareRequest,
  EnterprisePolicyGroupShareRequestDecisionPayload,
  EnterprisePolicyGroupShareRequestPayload,
  EnterprisePolicyGroupShareRequestsParams,
  EnterpriseProject,
  EnterpriseProjectMemberPayload,
  EnterpriseProjectPayload,
  EnterpriseProjectsParams,
  EnterpriseQueueAdmission,
  EnterpriseQueueAdmissionParams,
  EnterpriseQuotaPoliciesParams,
  EnterpriseQuotaPolicy,
  EnterpriseQuotaPolicyPayload,
  EnterpriseQuotaRequest,
  EnterpriseQuotaRequestBatchDecisionPayload,
  EnterpriseQuotaRequestBatchDecisionResult,
  EnterpriseQuotaRequestDecisionPayload,
  EnterpriseQuotaRequestPayload,
  EnterpriseQuotaRequestPolicy,
  EnterpriseQuotaRequestPolicyParams,
  EnterpriseQuotaRequestsParams,
  EnterpriseSharedPool,
  EnterpriseSharedPoolBorrow,
  EnterpriseSharedPoolBorrowParams,
  EnterpriseSharedPoolConfig,
  EnterpriseSharedPoolConfigParams,
  EnterpriseSharedPoolConfigPayload,
  EnterpriseSharedPoolParams,
  EnterpriseSharedPoolTrendItem,
  EnterpriseSharedPoolTrendParams,
  EnterpriseUsageBreakdownItem,
  EnterpriseUsageBreakdownParams,
  EnterpriseUsageParams,
  EnterpriseUsageSummary,
  EnterpriseWebhook,
  EnterpriseWebhookPayload,
  EnterpriseWebhookTestResult,
  PageInfo,
} from './types'

const ENTERPRISE_API = '/api/enterprise'

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

export async function getEnterpriseCurrent() {
  const res = await api.get<ApiResponse<Enterprise>>(
    `${ENTERPRISE_API}/current`
  )
  return res.data
}

export async function updateEnterpriseCurrent(payload: {
  name: string
  timezone: string
  status: number
  anomaly_throttle_config?: EnterpriseAnomalyThrottleConfig
}) {
  const res = await api.put<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/current`,
    payload
  )
  return res.data
}

export async function getEnterpriseOrgUnits(params: EnterpriseListParams = {}) {
  const res = await api.get<ApiResponse<EnterpriseOrgUnit[]>>(
    withQuery(`${ENTERPRISE_API}/org-units`, params)
  )
  return res.data
}

export async function createEnterpriseOrgUnit(
  payload: EnterpriseOrgUnitPayload
) {
  const res = await api.post<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/org-units`,
    payload
  )
  return res.data
}

export async function updateEnterpriseOrgUnit(
  id: number,
  payload: EnterpriseOrgUnitPayload
) {
  const res = await api.put<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/org-units/${id}`,
    payload
  )
  return res.data
}

export async function disableEnterpriseOrgUnit(id: number) {
  const res = await api.delete<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/org-units/${id}`
  )
  return res.data
}

export async function getEnterpriseMembers(
  params: EnterpriseMembersParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterpriseMember>>>(
    withQuery(`${ENTERPRISE_API}/members`, params)
  )
  return res.data
}

export async function updateEnterpriseMemberOrgUnit(
  userId: number,
  orgUnitId: number
) {
  const res = await api.put<ApiResponse<{ user_id: number }>>(
    `${ENTERPRISE_API}/members/${userId}/org-unit`,
    { org_unit_id: orgUnitId }
  )
  return res.data
}

export async function previewEnterpriseOrgSync(
  payload: EnterpriseOrgSyncPayload
) {
  const res = await api.post<ApiResponse<EnterpriseOrgSyncResult>>(
    `${ENTERPRISE_API}/org-sync/preview`,
    payload
  )
  return res.data
}

export async function applyEnterpriseOrgSync(
  payload: EnterpriseOrgSyncPayload
) {
  const res = await api.post<ApiResponse<EnterpriseOrgSyncResult>>(
    `${ENTERPRISE_API}/org-sync/apply`,
    payload
  )
  return res.data
}

export async function getEnterprisePolicyGroups(
  params: EnterpriseListParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterprisePolicyGroup>>>(
    withQuery(`${ENTERPRISE_API}/policy-groups`, params)
  )
  return res.data
}

export async function createEnterprisePolicyGroup(
  payload: EnterprisePolicyGroupPayload
) {
  const res = await api.post<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/policy-groups`,
    payload
  )
  return res.data
}

export async function updateEnterprisePolicyGroup(
  id: number,
  payload: EnterprisePolicyGroupPayload
) {
  const res = await api.put<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/policy-groups/${id}`,
    payload
  )
  return res.data
}

export async function disableEnterprisePolicyGroup(id: number) {
  const res = await api.delete<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/policy-groups/${id}`
  )
  return res.data
}

export async function getEnterprisePolicyGroupMembers(
  id: number,
  params: EnterprisePolicyGroupMembersParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterpriseMember>>>(
    withQuery(`${ENTERPRISE_API}/policy-groups/${id}/members`, params)
  )
  return res.data
}

export async function addEnterprisePolicyGroupMembers(
  id: number,
  payload: EnterprisePolicyGroupMembersPayload
) {
  const res = await api.post<ApiResponse<{ id: number; user_ids: number[] }>>(
    `${ENTERPRISE_API}/policy-groups/${id}/members`,
    payload
  )
  return res.data
}

export async function deleteEnterprisePolicyGroupMember(
  id: number,
  userId: number
) {
  const res = await api.delete<ApiResponse<{ id: number; user_id: number }>>(
    `${ENTERPRISE_API}/policy-groups/${id}/members/${userId}`
  )
  return res.data
}

export async function getEnterprisePolicyGroupShareRequests(
  params: EnterprisePolicyGroupShareRequestsParams = {}
) {
  const res = await api.get<
    ApiResponse<PageInfo<EnterprisePolicyGroupShareRequest>>
  >(withQuery(`${ENTERPRISE_API}/policy-group-share-requests`, params))
  return res.data
}

export async function createEnterprisePolicyGroupShareRequest(
  id: number,
  payload: EnterprisePolicyGroupShareRequestPayload
) {
  const res = await api.post<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/policy-groups/${id}/share-requests`,
    payload
  )
  return res.data
}

export async function approveEnterprisePolicyGroupShareRequest(
  id: number,
  payload: EnterprisePolicyGroupShareRequestDecisionPayload = {}
) {
  const res = await api.post<ApiResponse<{ id: number; status: string }>>(
    `${ENTERPRISE_API}/policy-group-share-requests/${id}/approve`,
    payload
  )
  return res.data
}

export async function rejectEnterprisePolicyGroupShareRequest(
  id: number,
  payload: EnterprisePolicyGroupShareRequestDecisionPayload = {}
) {
  const res = await api.post<ApiResponse<{ id: number; status: string }>>(
    `${ENTERPRISE_API}/policy-group-share-requests/${id}/reject`,
    payload
  )
  return res.data
}

export async function getEnterpriseProjects(
  params: EnterpriseProjectsParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterpriseProject>>>(
    withQuery(`${ENTERPRISE_API}/projects`, params)
  )
  return res.data
}

export async function createEnterpriseProject(
  payload: EnterpriseProjectPayload
) {
  const res = await api.post<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/projects`,
    payload
  )
  return res.data
}

export async function updateEnterpriseProject(
  id: number,
  payload: EnterpriseProjectPayload
) {
  const res = await api.put<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/projects/${id}`,
    payload
  )
  return res.data
}

export async function disableEnterpriseProject(id: number) {
  const res = await api.delete<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/projects/${id}`
  )
  return res.data
}

export async function getEnterpriseProjectMembers(
  id: number,
  params: EnterpriseListParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterpriseMember>>>(
    withQuery(`${ENTERPRISE_API}/projects/${id}/members`, params)
  )
  return res.data
}

export async function upsertEnterpriseProjectMember(
  id: number,
  payload: EnterpriseProjectMemberPayload
) {
  const res = await api.put<
    ApiResponse<{ id: number; user_id: number; role: string }>
  >(`${ENTERPRISE_API}/projects/${id}/members`, payload)
  return res.data
}

export async function deleteEnterpriseProjectMember(
  id: number,
  userId: number
) {
  const res = await api.delete<ApiResponse<{ id: number; user_id: number }>>(
    `${ENTERPRISE_API}/projects/${id}/members/${userId}`
  )
  return res.data
}

export async function getEnterpriseQuotaPolicies(
  params: EnterpriseQuotaPoliciesParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterpriseQuotaPolicy>>>(
    withQuery(`${ENTERPRISE_API}/quota-policies`, params)
  )
  return res.data
}

export async function createEnterpriseQuotaPolicy(
  payload: EnterpriseQuotaPolicyPayload
) {
  const res = await api.post<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/quota-policies`,
    payload
  )
  return res.data
}

export async function updateEnterpriseQuotaPolicy(
  id: number,
  payload: EnterpriseQuotaPolicyPayload
) {
  const res = await api.put<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/quota-policies/${id}`,
    payload
  )
  return res.data
}

export async function disableEnterpriseQuotaPolicy(id: number) {
  const res = await api.delete<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/quota-policies/${id}`
  )
  return res.data
}

export async function getEnterpriseQuotaRequests(
  params: EnterpriseQuotaRequestsParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterpriseQuotaRequest>>>(
    withQuery(`${ENTERPRISE_API}/quota-requests`, params)
  )
  return res.data
}

export async function getEnterpriseQuotaRequestPolicies(
  params: EnterpriseQuotaRequestPolicyParams = {}
) {
  const res = await api.get<ApiResponse<EnterpriseQuotaRequestPolicy[]>>(
    withQuery(`${ENTERPRISE_API}/quota-requests/policies`, params)
  )
  return res.data
}

export async function submitEnterpriseQuotaRequest(
  payload: EnterpriseQuotaRequestPayload
) {
  const res = await api.post<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/quota-requests`,
    payload
  )
  return res.data
}

export async function approveEnterpriseQuotaRequest(
  id: number,
  payload: EnterpriseQuotaRequestDecisionPayload
) {
  const res = await api.post<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/quota-requests/${id}/approve`,
    payload
  )
  return res.data
}

export async function rejectEnterpriseQuotaRequest(
  id: number,
  payload: EnterpriseQuotaRequestDecisionPayload
) {
  const res = await api.post<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/quota-requests/${id}/reject`,
    payload
  )
  return res.data
}

export async function batchApproveEnterpriseQuotaRequests(
  payload: EnterpriseQuotaRequestBatchDecisionPayload
) {
  const res = await api.post<
    ApiResponse<EnterpriseQuotaRequestBatchDecisionResult>
  >(`${ENTERPRISE_API}/quota-requests/batch/approve`, payload)
  return res.data
}

export async function batchRejectEnterpriseQuotaRequests(
  payload: EnterpriseQuotaRequestBatchDecisionPayload
) {
  const res = await api.post<
    ApiResponse<EnterpriseQuotaRequestBatchDecisionResult>
  >(`${ENTERPRISE_API}/quota-requests/batch/reject`, payload)
  return res.data
}

export async function withdrawEnterpriseQuotaRequest(id: number) {
  const res = await api.post<ApiResponse<{ id: number }>>(
    `${ENTERPRISE_API}/quota-requests/${id}/withdraw`
  )
  return res.data
}

export async function getEnterpriseUsageSummary(
  params: EnterpriseUsageParams = {}
) {
  const res = await api.get<ApiResponse<EnterpriseUsageSummary>>(
    withQuery(`${ENTERPRISE_API}/usage/summary`, params)
  )
  return res.data
}

export async function getEnterpriseUsageBreakdown(
  params: EnterpriseUsageBreakdownParams = {}
) {
  const res = await api.get<
    ApiResponse<PageInfo<EnterpriseUsageBreakdownItem>>
  >(withQuery(`${ENTERPRISE_API}/usage/breakdown`, params))
  return res.data
}

export async function downloadEnterpriseUsageBreakdownExport(
  params: EnterpriseUsageBreakdownParams = {}
) {
  const res = await api.get<Blob>(
    withQuery(`${ENTERPRISE_API}/usage/breakdown/export`, params),
    {
      responseType: 'blob',
      disableDuplicate: true,
      skipBusinessError: true,
    }
  )
  return res.data
}

export async function getEnterpriseAuditLogs(
  params: EnterpriseAuditLogParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterpriseAuditLog>>>(
    withQuery(`${ENTERPRISE_API}/audit-logs`, params)
  )
  return res.data
}

export async function getEnterpriseAnomalyProtections(
  params: EnterpriseAnomalyProtectionParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterpriseAnomalyProtection>>>(
    withQuery(`${ENTERPRISE_API}/anomaly-protections`, params)
  )
  return res.data
}

export async function getEnterpriseAnomalyProtectionTrends(
  params: EnterpriseAnomalyProtectionTrendParams = {}
) {
  const res = await api.get<
    ApiResponse<EnterpriseAnomalyProtectionTrendItem[]>
  >(withQuery(`${ENTERPRISE_API}/anomaly-protection-trends`, params))
  return res.data
}

export async function getEnterpriseQueueAdmissions(
  params: EnterpriseQueueAdmissionParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterpriseQueueAdmission>>>(
    withQuery(`${ENTERPRISE_API}/queue-admissions`, params)
  )
  return res.data
}

export async function cancelEnterpriseQueueAdmission(id: number) {
  const res = await api.post<ApiResponse<EnterpriseQueueAdmission>>(
    `${ENTERPRISE_API}/queue-admissions/${id}/cancel`
  )
  return res.data
}

export async function getEnterpriseSharedPoolConfigs(
  params: EnterpriseSharedPoolConfigParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterpriseSharedPoolConfig>>>(
    withQuery(`${ENTERPRISE_API}/shared-pool-configs`, params)
  )
  return res.data
}

export async function upsertEnterpriseSharedPoolConfig(
  payload: EnterpriseSharedPoolConfigPayload
) {
  const res = await api.put<ApiResponse<EnterpriseSharedPoolConfig>>(
    `${ENTERPRISE_API}/shared-pool-configs`,
    payload
  )
  return res.data
}

export async function getEnterpriseSharedPoolTrends(
  params: EnterpriseSharedPoolTrendParams = {}
) {
  const res = await api.get<ApiResponse<EnterpriseSharedPoolTrendItem[]>>(
    withQuery(`${ENTERPRISE_API}/shared-pool-trends`, params)
  )
  return res.data
}

export async function getEnterpriseSharedPools(
  params: EnterpriseSharedPoolParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterpriseSharedPool>>>(
    withQuery(`${ENTERPRISE_API}/shared-pools`, params)
  )
  return res.data
}

export async function getEnterpriseSharedPoolBorrows(
  params: EnterpriseSharedPoolBorrowParams = {}
) {
  const res = await api.get<ApiResponse<PageInfo<EnterpriseSharedPoolBorrow>>>(
    withQuery(`${ENTERPRISE_API}/shared-pool-borrows`, params)
  )
  return res.data
}

export async function getEnterpriseWebhooks() {
  const res = await api.get<ApiResponse<EnterpriseWebhook[]>>(
    `${ENTERPRISE_API}/webhooks`
  )
  return res.data
}

export async function createEnterpriseWebhook(
  payload: EnterpriseWebhookPayload
) {
  const res = await api.post<ApiResponse<EnterpriseWebhook>>(
    `${ENTERPRISE_API}/webhooks`,
    payload
  )
  return res.data
}

export async function updateEnterpriseWebhook(
  id: number,
  payload: EnterpriseWebhookPayload
) {
  const res = await api.put<ApiResponse<EnterpriseWebhook>>(
    `${ENTERPRISE_API}/webhooks/${id}`,
    payload
  )
  return res.data
}

export async function disableEnterpriseWebhook(id: number) {
  const res = await api.delete<ApiResponse<EnterpriseWebhook>>(
    `${ENTERPRISE_API}/webhooks/${id}`
  )
  return res.data
}

export async function testEnterpriseWebhook(id: number) {
  const res = await api.post<ApiResponse<EnterpriseWebhookTestResult>>(
    `${ENTERPRISE_API}/webhooks/${id}/test`
  )
  return res.data
}

export async function getEnterpriseNotificationOutbox(
  params: EnterpriseNotificationOutboxParams = {}
) {
  const res = await api.get<
    ApiResponse<PageInfo<EnterpriseNotificationOutbox>>
  >(withQuery(`${ENTERPRISE_API}/notification-outbox`, params))
  return res.data
}

export async function retryEnterpriseNotificationOutbox(id: number) {
  const res = await api.post<ApiResponse<EnterpriseNotificationOutbox>>(
    `${ENTERPRISE_API}/notification-outbox/${id}/retry`
  )
  return res.data
}

export async function getEnterpriseNotificationOutboxWorkerMetrics() {
  const res = await api.get<
    ApiResponse<EnterpriseNotificationOutboxWorkerMetrics>
  >(`${ENTERPRISE_API}/notification-outbox/worker-metrics`)
  return res.data
}

export async function getEnterpriseNotificationPreferences() {
  const res = await api.get<ApiResponse<EnterpriseNotificationPreference[]>>(
    `${ENTERPRISE_API}/notification-preferences`
  )
  return res.data
}

export async function updateEnterpriseNotificationPreference(
  payload: EnterpriseNotificationPreferencePayload
) {
  const res = await api.put<ApiResponse<EnterpriseNotificationPreference>>(
    `${ENTERPRISE_API}/notification-preferences`,
    payload
  )
  return res.data
}
