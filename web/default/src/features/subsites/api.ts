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
import type { RegisterPayload } from '@/features/auth/types'
import type {
  ManagedSubsiteActivityResponse,
  ManagedSubsiteChannelBalanceResponse,
  ManagedSubsiteChannelDeleteResponse,
  ManagedSubsiteChannelModelsResponse,
  ManagedSubsiteChannelPayload,
  ManagedSubsiteChannelResponse,
  ManagedSubsiteChannelsResponse,
  ManagedSubsiteChannelTestResponse,
  ManagedSubsiteMemberDeleteResponse,
  ManagedSubsiteMemberResponse,
  ManagedSubsiteMembersResponse,
  ManagedSubsiteMemberUpsertPayload,
  ManagedSubsitePayload,
  ManagedSubsiteResponse,
  ManagedSubsitesResponse,
  PublicSubsiteResponse,
  SubsiteQuotaPolicyInfo,
  SubsiteQuotaPolicyResponse,
  SubsiteDashboardResponse,
  SubsiteMemberResponse,
  SubsiteTokenActionResponse,
  SubsiteTokenResponse,
} from './types'

export async function getManagedSubsites(params: {
  p?: number
  page_size?: number
} = {}) {
  const res = await api.get<ManagedSubsitesResponse>(
    '/api/subsite-management/subsites',
    { params }
  )
  return res.data
}

export async function createManagedSubsite(payload: ManagedSubsitePayload) {
  const res = await api.post<ManagedSubsiteResponse>(
    '/api/subsite-management/subsites',
    payload
  )
  return res.data
}

export async function getManagedSubsite(id: number) {
  const res = await api.get<ManagedSubsiteResponse>(
    `/api/subsite-management/subsites/${id}`
  )
  return res.data
}

export async function updateManagedSubsite(
  id: number,
  payload: ManagedSubsitePayload
) {
  const res = await api.patch<ManagedSubsiteResponse>(
    `/api/subsite-management/subsites/${id}`,
    payload
  )
  return res.data
}

export async function updateManagedSubsiteQuotaPolicy(
  id: number,
  payload: Partial<SubsiteQuotaPolicyInfo>
) {
  const res = await api.put<SubsiteQuotaPolicyResponse>(
    `/api/subsite-management/subsites/${id}/quota-policy`,
    payload
  )
  return res.data
}

export async function getManagedSubsiteActivity(id: number) {
  const res = await api.get<ManagedSubsiteActivityResponse>(
    `/api/subsite-management/subsites/${id}/activity`
  )
  return res.data
}

export async function getManagedSubsiteChannels(id: number) {
  const res = await api.get<ManagedSubsiteChannelsResponse>(
    `/api/subsite-management/subsites/${id}/channels`
  )
  return res.data
}

export async function createManagedSubsiteChannel(
  id: number,
  payload: ManagedSubsiteChannelPayload
) {
  const res = await api.post<ManagedSubsiteChannelResponse>(
    `/api/subsite-management/subsites/${id}/channels`,
    payload
  )
  return res.data
}

export async function updateManagedSubsiteChannel(
  id: number,
  channelId: number,
  payload: ManagedSubsiteChannelPayload
) {
  const res = await api.patch<ManagedSubsiteChannelResponse>(
    `/api/subsite-management/subsites/${id}/channels/${channelId}`,
    payload
  )
  return res.data
}

export async function testManagedSubsiteChannel(id: number, channelId: number) {
  const res = await api.get<ManagedSubsiteChannelTestResponse>(
    `/api/subsite-management/subsites/${id}/channels/${channelId}/test`
  )
  return res.data
}

export async function updateManagedSubsiteChannelBalance(
  id: number,
  channelId: number
) {
  const res = await api.get<ManagedSubsiteChannelBalanceResponse>(
    `/api/subsite-management/subsites/${id}/channels/${channelId}/balance`
  )
  return res.data
}

export async function getManagedSubsiteChannelUpstreamModels(
  id: number,
  channelId: number
) {
  const res = await api.get<ManagedSubsiteChannelModelsResponse>(
    `/api/subsite-management/subsites/${id}/channels/${channelId}/upstream-models`
  )
  return res.data
}

export async function syncManagedSubsiteChannelModels(
  id: number,
  channelId: number
) {
  const res = await api.post<ManagedSubsiteChannelResponse>(
    `/api/subsite-management/subsites/${id}/channels/${channelId}/models/sync`
  )
  return res.data
}

export async function deleteManagedSubsiteChannel(
  id: number,
  channelId: number
) {
  const res = await api.delete<ManagedSubsiteChannelDeleteResponse>(
    `/api/subsite-management/subsites/${id}/channels/${channelId}`
  )
  return res.data
}

export async function getManagedSubsiteMembers(id: number) {
  const res = await api.get<ManagedSubsiteMembersResponse>(
    `/api/subsite-management/subsites/${id}/members`
  )
  return res.data
}

export async function upsertManagedSubsiteMember(
  id: number,
  payload: ManagedSubsiteMemberUpsertPayload
) {
  const res = await api.put<ManagedSubsiteMemberResponse>(
    `/api/subsite-management/subsites/${id}/members`,
    payload
  )
  return res.data
}

export async function deleteManagedSubsiteMember(id: number, userId: number) {
  const res = await api.delete<ManagedSubsiteMemberDeleteResponse>(
    `/api/subsite-management/subsites/${id}/members/${userId}`
  )
  return res.data
}

export async function getPublicSubsite(slug: string) {
  const res = await api.get<PublicSubsiteResponse>(
    `/api/subsites/${encodeURIComponent(slug)}/public`,
    {
      skipErrorHandler: true,
    }
  )
  return res.data
}

export async function registerSubsiteUser(
  slug: string,
  payload: RegisterPayload
) {
  const res = await api.post(
    `/api/subsites/${encodeURIComponent(slug)}/register`,
    payload,
    {
      params: { turnstile: payload.turnstile ?? '' },
    }
  )
  return res.data
}

export async function getSubsiteSelfMember(slug: string) {
  const res = await api.get<SubsiteMemberResponse>(
    `/api/subsites/${encodeURIComponent(slug)}/member/self`,
    {
      skipBusinessError: true,
    }
  )
  return res.data
}

export async function getSubsiteDashboard(slug: string) {
  const res = await api.get<SubsiteDashboardResponse>(
    `/api/subsites/${encodeURIComponent(slug)}/dashboard`,
    {
      skipBusinessError: true,
      skipErrorHandler: true,
    }
  )
  return res.data
}

export async function ensureSubsiteToken(slug: string) {
  const res = await api.post<SubsiteTokenActionResponse>(
    `/api/subsites/${encodeURIComponent(slug)}/token`,
    undefined,
    {
      skipBusinessError: true,
    }
  )
  return res.data
}

export async function getSubsiteTokenKey(slug: string) {
  const res = await api.post<SubsiteTokenResponse>(
    `/api/subsites/${encodeURIComponent(slug)}/token/key`,
    undefined,
    {
      skipBusinessError: true,
    }
  )
  return res.data
}

export async function rotateSubsiteToken(slug: string) {
  const res = await api.post<SubsiteTokenActionResponse>(
    `/api/subsites/${encodeURIComponent(slug)}/token/rotate`,
    undefined,
    {
      skipBusinessError: true,
    }
  )
  return res.data
}
