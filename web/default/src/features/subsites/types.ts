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
export type SubsiteRuntimeStatus =
  | 'draft'
  | 'enabled'
  | 'disabled'
  | 'not_started'
  | 'expired'

export type SubsiteRegistrationPolicy = 'open' | 'invite' | 'closed'

export type SubsiteAccessInfo = {
  allowed: boolean
  status: SubsiteRuntimeStatus
  code?: string
  message?: string
}

export type PublicSubsite = {
  id: number
  slug: string
  name: string
  title: string
  logo_url?: string
  favicon_url?: string
  theme_color?: string
  status: string
  runtime_status: SubsiteRuntimeStatus
  disabled_reason?: string
  announcement_icon?: string
  announcement_title?: string
  announcement_body?: string
  announcement_url?: string
  contact_url?: string
  registration_policy?: SubsiteRegistrationPolicy
  starts_at?: number
  ends_at?: number
  access: SubsiteAccessInfo
}

export type PublicSubsiteResponse = {
  success: boolean
  message?: string
  data?: PublicSubsite
}

export type SubsiteMemberRole = 'owner' | 'admin' | 'member'
export type SubsiteMemberStatus = 'active' | 'disabled'

export type SubsiteMemberInfo = {
  subsite_id: number
  user_id: number
  role: SubsiteMemberRole
  status: SubsiteMemberStatus
  can_access: boolean
  can_manage: boolean
}

export type SubsiteMemberResponse = {
  success: boolean
  message?: string
  data?: SubsiteMemberInfo
}

export type SubsiteTokenInfo = {
  id: number
  name: string
  key?: string
  masked_key: string
  status: number
  created_time: number
  accessed_time: number
  expired_time: number
  unlimited_quota: boolean
}

export type SubsiteQuotaMetric = {
  limit: number
  used: number
  remaining: number
  window_start: number
  window_end: number
  next_reset_time: number
  window_seconds: number
}

export type SubsiteQuotaSummary = {
  site_daily_quota: SubsiteQuotaMetric
  site_window_quota: SubsiteQuotaMetric
  user_daily_quota: SubsiteQuotaMetric
  user_window_quota: SubsiteQuotaMetric
  site_daily_requests: SubsiteQuotaMetric
  site_window_requests: SubsiteQuotaMetric
  user_daily_requests: SubsiteQuotaMetric
  user_window_requests: SubsiteQuotaMetric
}

export type SubsiteUsageStats = {
  window_seconds: number
  calls: number
  prompt_tokens: number
  output_tokens: number
  total_tokens: number
  quota: number
  last_request_at: number
}

export type SubsiteRecentLog = {
  id: number
  created_at: number
  type: number
  username: string
  model_name: string
  prompt_tokens: number
  completion_tokens: number
  cache_tokens: number
  reasoning_tokens: number
  total_tokens: number
  quota: number
  use_time: number
  status: 'success' | 'error' | string
}

export type SubsiteDashboard = {
  subsite: PublicSubsite
  member: SubsiteMemberInfo
  base_url: string
  token?: SubsiteTokenInfo | null
  quota: SubsiteQuotaSummary
  stats_24h: SubsiteUsageStats
  recent_logs: SubsiteRecentLog[]
}

export type SubsiteDashboardResponse = {
  success: boolean
  message?: string
  data?: SubsiteDashboard
}

export type SubsiteTokenAction = {
  created: boolean
  token: SubsiteTokenInfo
}

export type SubsiteTokenActionResponse = {
  success: boolean
  message?: string
  data?: SubsiteTokenAction
}

export type SubsiteTokenResponse = {
  success: boolean
  message?: string
  data?: SubsiteTokenInfo
}

export type SubsiteQuotaPolicyInfo = {
  site_daily_quota: number
  site_window_quota: number
  user_daily_quota: number
  user_window_quota: number
  site_daily_request_limit: number
  site_window_request_limit: number
  user_daily_request_limit: number
  user_window_request_limit: number
  site_window_seconds: number
  user_window_seconds: number
}

export type ManagedSubsite = {
  subsite: PublicSubsite
  role: string
  can_manage: boolean
  owner_user_ids: number[]
  owner_usernames: string[]
  member_count: number
  today_calls: number
  today_quota: number
  quota_policy?: SubsiteQuotaPolicyInfo
}

export type ManagedSubsitesResponse = {
  success: boolean
  message?: string
  data?: {
    page: number
    page_size: number
    total: number
    items: ManagedSubsite[]
  }
}

export type ManagedSubsiteResponse = {
  success: boolean
  message?: string
  data?: ManagedSubsite
}

export type ManagedSubsiteMember = {
  id: number
  subsite_id: number
  user_id: number
  username: string
  display_name: string
  email: string
  user_status: number
  role: SubsiteMemberRole
  status: SubsiteMemberStatus
  can_access: boolean
  can_manage: boolean
  joined_at: number
  created_at: number
  updated_at: number
}

export type ManagedSubsiteMembersResponse = {
  success: boolean
  message?: string
  data?: ManagedSubsiteMember[]
}

export type ManagedSubsiteMemberResponse = {
  success: boolean
  message?: string
  data?: ManagedSubsiteMember
}

export type ManagedSubsiteMemberDeleteResponse = {
  success: boolean
  message?: string
  data?: {
    user_id: number
  }
}

export type ManagedSubsiteMemberUpsertPayload = {
  user_id: number
  role: SubsiteMemberRole
  status: SubsiteMemberStatus
}

export type ManagedSubsiteActivity = {
  stats_24h: SubsiteUsageStats
  error_calls_24h: number
  recent_logs: SubsiteRecentLog[]
}

export type ManagedSubsiteActivityResponse = {
  success: boolean
  message?: string
  data?: ManagedSubsiteActivity
}

export type ManagedSubsiteChannel = {
  id: number
  subsite_id: number
  name: string
  type: number
  status: number
  models: string
  group: string
  base_url: string
  priority: number
  weight: number
  created_time: number
  test_time: number
  response_time: number
  used_quota: number
  balance: number
  remark: string
  has_key: boolean
  model_display_names?: Record<string, string>
}

export type ManagedSubsiteChannelsResponse = {
  success: boolean
  message?: string
  data?: ManagedSubsiteChannel[]
}

export type ManagedSubsiteChannelResponse = {
  success: boolean
  message?: string
  data?: ManagedSubsiteChannel
}

export type ManagedSubsiteChannelTestResponse = {
  success: boolean
  message?: string
  time?: number
  error_code?: string
}

export type ManagedSubsiteChannelBalanceResponse = {
  success: boolean
  message?: string
  data?: {
    balance: number
  }
}

export type ManagedSubsiteChannelModelsResponse = {
  success: boolean
  message?: string
  data?: {
    models: string[]
  }
}

export type ManagedSubsiteChannelDeleteResponse = {
  success: boolean
  message?: string
  data?: {
    channel_id: number
  }
}

export type ManagedSubsiteChannelPayload = {
  name: string
  type: number
  key?: string
  base_url?: string
  models: string
  group?: string
  status: number
  priority?: number
  weight?: number
  remark?: string
  model_display_names?: Record<string, string>
}

export type ManagedSubsitePayload = {
  slug?: string
  name?: string
  title?: string
  logo_url?: string
  favicon_url?: string
  theme_color?: string
  status?: string
  disabled_reason?: string
  announcement_icon?: string
  announcement_title?: string
  announcement_body?: string
  announcement_url?: string
  contact_url?: string
  registration_policy?: SubsiteRegistrationPolicy
  invite_code?: string
  email_domain_whitelist?: string
  starts_at?: number
  ends_at?: number
  owner_user_id?: number
  quota_policy?: Partial<SubsiteQuotaPolicyInfo>
}

export type SubsiteQuotaPolicyResponse = {
  success: boolean
  message?: string
  data?: SubsiteQuotaPolicyInfo
}
