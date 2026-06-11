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
import type { NavGroup } from '@/components/layout/types'
import { AuditLogsTable } from './components/audit-logs-table'
import { BillingEventsTable } from './components/billing-events-table'
import { BridgeClientsTable } from './components/bridge-clients-table'
import { MCPMarket } from './components/mcp-market'
import { MCPOverview } from './components/mcp-overview'
import { MCPToolsTable } from './components/mcp-tools-table'
import { OpenAPIBinaryObjectsTable } from './components/openapi-binary-objects-table'
import { MCPProxyServersTable } from './components/proxy-servers-table'
import { MCPProxyToolsTable } from './components/proxy-tools-table'
import { ToolCallsTable } from './components/tool-calls-table'

export const MCP_SECTIONS = [
  {
    id: 'market',
    titleKey: 'MCP Market',
    build: () => <MCPMarket />,
  },
  {
    id: 'overview',
    titleKey: 'Overview',
    build: () => <MCPOverview />,
  },
  {
    id: 'tools',
    titleKey: 'MCP Tools',
    build: () => <MCPToolsTable />,
  },
  {
    id: 'openapi-objects',
    titleKey: 'OpenAPI Objects',
    build: () => <OpenAPIBinaryObjectsTable />,
  },
  {
    id: 'proxy-servers',
    titleKey: 'Proxy Servers',
    build: () => <MCPProxyServersTable />,
  },
  {
    id: 'proxy-tools',
    titleKey: 'Proxy Tools',
    build: () => <MCPProxyToolsTable />,
  },
  {
    id: 'bridge-clients',
    titleKey: 'Bridge Clients',
    build: () => <BridgeClientsTable />,
  },
  {
    id: 'tool-calls',
    titleKey: 'Tool Calls',
    build: () => <ToolCallsTable />,
  },
  {
    id: 'billing-events',
    titleKey: 'Billing Events',
    build: () => <BillingEventsTable />,
  },
  {
    id: 'audit-logs',
    titleKey: 'Bridge Audit Logs',
    build: () => <AuditLogsTable />,
  },
] as const

export type MCPSectionId = (typeof MCP_SECTIONS)[number]['id']

export const MCP_SECTION_IDS = MCP_SECTIONS.map((section) => section.id)
export const MCP_DEFAULT_SECTION: MCPSectionId = 'overview'

export function isMCPSectionId(
  value: string | undefined
): value is MCPSectionId {
  return MCP_SECTION_IDS.includes(value as MCPSectionId)
}

export function getMCPSectionMeta(section: MCPSectionId) {
  return MCP_SECTIONS.find((item) => item.id === section) ?? MCP_SECTIONS[0]
}

export function getMCPSectionContent(section: MCPSectionId) {
  return getMCPSectionMeta(section).build()
}

export function getMCPSectionNavItems(t: (key: string) => string) {
  return MCP_SECTIONS.map((section) => ({
    title: t(section.titleKey),
    url: `/mcp/${section.id}`,
  }))
}

export function getMCPNavGroups(t: (key: string) => string): NavGroup[] {
  return [
    {
      title: t('MCP'),
      items: getMCPSectionNavItems(t),
    },
  ]
}
