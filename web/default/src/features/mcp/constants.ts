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
import type { TFunction } from 'i18next'
import type { StatusVariant } from '@/components/status-badge'

export const MCP_TOOL_STATUS = {
  DISABLED: 0,
  ENABLED: 1,
} as const

export const BRIDGE_CLIENT_STATUS = {
  OFFLINE: 0,
  ONLINE: 1,
} as const

export const MCP_PROXY_SERVER_STATUSES = {
  enabled: {
    labelKey: 'Enabled',
    variant: 'success' as StatusVariant,
  },
  disabled: {
    labelKey: 'Disabled',
    variant: 'neutral' as StatusVariant,
  },
  error: {
    labelKey: 'Error',
    variant: 'danger' as StatusVariant,
  },
  archived: {
    labelKey: 'Archived',
    variant: 'neutral' as StatusVariant,
  },
} as const

export const MCP_PROXY_TOOL_STATUSES = {
  pending: {
    labelKey: 'Pending',
    variant: 'warning' as StatusVariant,
  },
  enabled: {
    labelKey: 'Enabled',
    variant: 'success' as StatusVariant,
  },
  disabled: {
    labelKey: 'Disabled',
    variant: 'neutral' as StatusVariant,
  },
  schema_changed: {
    labelKey: 'Schema Changed',
    variant: 'warning' as StatusVariant,
  },
  error: {
    labelKey: 'Error',
    variant: 'danger' as StatusVariant,
  },
} as const

export const MCP_PROXY_REVIEW_REASONS = {
  server_error: {
    labelKey: 'Server Error',
  },
  no_recent_check: {
    labelKey: 'No Recent Check',
  },
  latest_check_failed: {
    labelKey: 'Latest Check Failed',
  },
  schema_changed_tools: {
    labelKey: 'Schema Changed Tools',
  },
  proxy_tool_errors: {
    labelKey: 'Proxy Tool Errors',
  },
  recent_call_errors: {
    labelKey: 'Recent Call Errors',
  },
  transport_error: {
    labelKey: 'Transport Error',
  },
} as const

export const MCP_PROXY_TRANSPORTS = {
  http: {
    labelKey: 'HTTP',
  },
  sse: {
    labelKey: 'SSE',
  },
  streamable_http: {
    labelKey: 'Streamable HTTP',
  },
  bridge: {
    labelKey: 'Bridge',
  },
  qidian_browser: {
    labelKey: 'Qidian Browser',
  },
} as const

export const MCP_PROXY_AUTH_TYPES = {
  none: {
    labelKey: 'None',
  },
  bearer: {
    labelKey: 'Bearer',
  },
  basic: {
    labelKey: 'Basic',
  },
  header: {
    labelKey: 'Header',
  },
} as const

export const MCP_PROXY_VISIBILITIES = {
  admin: {
    labelKey: 'Admin',
  },
  group: {
    labelKey: 'Group',
  },
  public: {
    labelKey: 'Public',
  },
} as const

export const MCP_TOOL_CALL_STATUSES = {
  pending: {
    labelKey: 'Pending',
    variant: 'warning' as StatusVariant,
  },
  success: {
    labelKey: 'Success',
    variant: 'success' as StatusVariant,
  },
  error: {
    labelKey: 'Error',
    variant: 'danger' as StatusVariant,
  },
  timeout: {
    labelKey: 'Timeout',
    variant: 'warning' as StatusVariant,
  },
} as const

export const MCP_PROXY_DISCOVERY_EVENT_STATUSES = {
  success: {
    labelKey: 'Success',
    variant: 'success' as StatusVariant,
  },
  error: {
    labelKey: 'Error',
    variant: 'danger' as StatusVariant,
  },
} as const

export const MCP_TOOL_PRICE_UNITS = {
  per_call: {
    labelKey: 'Per Call',
  },
  per_1k_tokens: {
    labelKey: 'Per 1K Tokens',
  },
  per_mb: {
    labelKey: 'Per MB',
  },
} as const

export const MCP_TOOL_SOURCES = {
  builtin: {
    labelKey: 'Built-in',
  },
  custom: {
    labelKey: 'Custom',
  },
  plugin: {
    labelKey: 'Plugin',
  },
  openapi: {
    labelKey: 'OpenAPI',
  },
  mcp_proxy: {
    labelKey: 'MCP Proxy',
  },
} as const

export function getToolStatusLabel(status: number): string {
  return status === MCP_TOOL_STATUS.ENABLED ? 'Enabled' : 'Disabled'
}

export function getToolStatusVariant(status: number): StatusVariant {
  return status === MCP_TOOL_STATUS.ENABLED ? 'success' : 'neutral'
}

export function getClientStatusLabel(status: number, online: boolean): string {
  if (online) return 'Online'
  return status === BRIDGE_CLIENT_STATUS.ONLINE ? 'Online' : 'Offline'
}

export function getClientStatusVariant(
  status: number,
  online: boolean
): StatusVariant {
  if (online) return 'success'
  return status === BRIDGE_CLIENT_STATUS.ONLINE ? 'success' : 'neutral'
}

export function getCallStatusLabel(status: string): string | undefined {
  return MCP_TOOL_CALL_STATUSES[status as keyof typeof MCP_TOOL_CALL_STATUSES]
    ?.labelKey
}

export function getCallStatusVariant(status: string): StatusVariant {
  return (
    MCP_TOOL_CALL_STATUSES[status as keyof typeof MCP_TOOL_CALL_STATUSES]
      ?.variant ?? 'neutral'
  )
}

export function getProxyDiscoveryEventStatusLabel(status: string): string {
  return (
    MCP_PROXY_DISCOVERY_EVENT_STATUSES[
      status as keyof typeof MCP_PROXY_DISCOVERY_EVENT_STATUSES
    ]?.labelKey ?? status
  )
}

export function getProxyDiscoveryEventTypeLabel(type: string): string {
  if (type === 'test') return 'Test'
  if (type === 'discover') return 'Discover'
  return type || 'Unknown'
}

export function getProxyDiscoveryEventStatusVariant(
  status: string
): StatusVariant {
  return (
    MCP_PROXY_DISCOVERY_EVENT_STATUSES[
      status as keyof typeof MCP_PROXY_DISCOVERY_EVENT_STATUSES
    ]?.variant ?? 'neutral'
  )
}

export function getProxyServerStatusLabel(status: string): string {
  return (
    MCP_PROXY_SERVER_STATUSES[status as keyof typeof MCP_PROXY_SERVER_STATUSES]
      ?.labelKey ?? status
  )
}

export function getProxyServerStatusVariant(status: string): StatusVariant {
  return (
    MCP_PROXY_SERVER_STATUSES[status as keyof typeof MCP_PROXY_SERVER_STATUSES]
      ?.variant ?? 'neutral'
  )
}

export function getProxyToolStatusLabel(status: string): string {
  return (
    MCP_PROXY_TOOL_STATUSES[status as keyof typeof MCP_PROXY_TOOL_STATUSES]
      ?.labelKey ?? status
  )
}

export function getProxyToolStatusVariant(status: string): StatusVariant {
  return (
    MCP_PROXY_TOOL_STATUSES[status as keyof typeof MCP_PROXY_TOOL_STATUSES]
      ?.variant ?? 'neutral'
  )
}

export function getPriceUnitLabel(unit: string): string {
  return (
    MCP_TOOL_PRICE_UNITS[unit as keyof typeof MCP_TOOL_PRICE_UNITS]?.labelKey ??
    unit
  )
}

export function getProxyTransportLabel(transport: string): string {
  return (
    MCP_PROXY_TRANSPORTS[transport as keyof typeof MCP_PROXY_TRANSPORTS]
      ?.labelKey ?? transport
  )
}

export function getProxyAuthTypeLabel(authType: string): string {
  return (
    MCP_PROXY_AUTH_TYPES[authType as keyof typeof MCP_PROXY_AUTH_TYPES]
      ?.labelKey ?? authType
  )
}

export function getProxyVisibilityLabel(visibility: string): string {
  return (
    MCP_PROXY_VISIBILITIES[visibility as keyof typeof MCP_PROXY_VISIBILITIES]
      ?.labelKey ?? visibility
  )
}

export function getProxyReviewReasonLabel(reason: string): string {
  return (
    MCP_PROXY_REVIEW_REASONS[
      reason as keyof typeof MCP_PROXY_REVIEW_REASONS
    ]?.labelKey ?? reason
  )
}

export function getToolSourceLabel(source: string): string {
  return (
    MCP_TOOL_SOURCES[source as keyof typeof MCP_TOOL_SOURCES]?.labelKey ??
    source
  )
}

export function getToolStatusOptions(t: TFunction) {
  return [
    {
      label: t('Enabled'),
      value: String(MCP_TOOL_STATUS.ENABLED),
    },
    {
      label: t('Disabled'),
      value: String(MCP_TOOL_STATUS.DISABLED),
    },
  ]
}

export function getBridgeClientStatusOptions(t: TFunction) {
  return [
    {
      label: t('Online'),
      value: String(BRIDGE_CLIENT_STATUS.ONLINE),
    },
    {
      label: t('Offline'),
      value: String(BRIDGE_CLIENT_STATUS.OFFLINE),
    },
  ]
}

export function getCallStatusOptions(t: TFunction) {
  return Object.entries(MCP_TOOL_CALL_STATUSES).map(([value, config]) => ({
    label: t(config.labelKey),
    value,
  }))
}

export function getProxyServerStatusOptions(t: TFunction) {
  return Object.entries(MCP_PROXY_SERVER_STATUSES)
    .filter(([value]) => value !== 'archived')
    .map(([value, config]) => ({
      label: t(config.labelKey),
      value,
    }))
}

export function getProxyToolStatusOptions(t: TFunction) {
  return Object.entries(MCP_PROXY_TOOL_STATUSES).map(([value, config]) => ({
    label: t(config.labelKey),
    value,
  }))
}

export function getProxyTransportOptions(t: TFunction) {
  return Object.entries(MCP_PROXY_TRANSPORTS).map(([value, config]) => ({
    label: t(config.labelKey),
    value,
  }))
}

export function getProxyAuthTypeOptions(t: TFunction) {
  return Object.entries(MCP_PROXY_AUTH_TYPES).map(([value, config]) => ({
    label: t(config.labelKey),
    value,
  }))
}

export function getProxyVisibilityOptions(t: TFunction) {
  return Object.entries(MCP_PROXY_VISIBILITIES).map(([value, config]) => ({
    label: t(config.labelKey),
    value,
  }))
}

export function getToolSourceOptions(t: TFunction) {
  return Object.entries(MCP_TOOL_SOURCES).map(([value, config]) => ({
    label: t(config.labelKey),
    value,
  }))
}

export function getPriceUnitOptions(t: TFunction) {
  return Object.entries(MCP_TOOL_PRICE_UNITS)
    .filter(([value]) => value === 'per_call')
    .map(([value, config]) => ({
      label: t(config.labelKey),
      value,
    }))
}
