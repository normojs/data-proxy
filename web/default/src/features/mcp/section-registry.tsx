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
import { MyTunnelAppsTable } from './components/my-tunnel-apps-table'
import { OpenAPIBinaryObjectsTable } from './components/openapi-binary-objects-table'
import { MCPProxyServersTable } from './components/proxy-servers-table'
import { MCPProxyToolsTable } from './components/proxy-tools-table'
import { ToolCallsTable } from './components/tool-calls-table'
import { TunnelAppsTable } from './components/tunnel-apps-table'
import { TunnelConnectionsTable } from './components/tunnel-connections-table'
import { TunnelSessionsTable } from './components/tunnel-sessions-table'

export const MCP_SECTIONS = [
  {
    id: 'market',
    titleKey: 'MCP Market',
    descriptionKey:
      'Browse enabled MCP tools, inspect schemas, and open related billing records.',
    build: () => <MCPMarket />,
  },
  {
    id: 'overview',
    titleKey: 'Overview',
    descriptionKey:
      'Monitor MCP health, recent activity, billing risk, and items that need operator review.',
    build: () => <MCPOverview />,
  },
  {
    id: 'tools',
    titleKey: 'MCP Tools',
    descriptionKey:
      'Manage server-side, OpenAPI, proxy, and bridge-exposed tools available to users.',
    build: () => <MCPToolsTable />,
  },
  {
    id: 'openapi-objects',
    titleKey: 'OpenAPI Objects',
    descriptionKey:
      'Review stored binary OpenAPI responses and download objects created by tool calls.',
    build: () => <OpenAPIBinaryObjectsTable />,
  },
  {
    id: 'proxy-servers',
    titleKey: 'Proxy Servers',
    descriptionKey:
      'Configure remote MCP servers, test transports, and run discovery or health checks.',
    build: () => <MCPProxyServersTable />,
  },
  {
    id: 'proxy-tools',
    titleKey: 'Proxy Tools',
    descriptionKey:
      'Review discovered proxy tools, downstream schema mappings, exposure status, and health.',
    build: () => <MCPProxyToolsTable />,
  },
  {
    id: 'bridge-clients',
    titleKey: 'Bridge Clients',
    descriptionKey:
      'Inspect local Bridge clients, heartbeats, workspace exposure, and write permissions.',
    build: () => <BridgeClientsTable />,
  },
  {
    id: 'tunnel-apps',
    titleKey: 'Tunnel Apps',
    descriptionKey:
      'Review MCP code tunnels and traffic tunnels with approval, permissions, and routing state.',
    build: () => <TunnelAppsTable />,
  },
  {
    id: 'my-tunnel-apps',
    titleKey: 'My Tunnel Apps',
    descriptionKey:
      'Request MCP code tunnel or HTTP tunnel apps for local Bridge clients and track administrator review.',
    build: () => <MyTunnelAppsTable />,
  },
  {
    id: 'tunnel-connections',
    titleKey: 'Tunnel Connections',
    descriptionKey:
      'Create and revoke dedicated tunnel connection keys for approved local apps.',
    build: () => <TunnelConnectionsTable />,
  },
  {
    id: 'tunnel-sessions',
    titleKey: 'Tunnel Sessions',
    descriptionKey:
      'Inspect active and historical MCP tunnel sessions by app, connection, and gateway session id.',
    build: () => <TunnelSessionsTable />,
  },
  {
    id: 'tool-calls',
    titleKey: 'Tool Calls',
    descriptionKey:
      'Trace MCP tool invocations, durations, errors, settlement state, and downstream metadata.',
    build: () => <ToolCallsTable />,
  },
  {
    id: 'billing-events',
    titleKey: 'Billing Events',
    descriptionKey:
      'Audit MCP billing events, quota movement, ledger state, and reconciliation signals.',
    build: () => <BillingEventsTable />,
  },
  {
    id: 'audit-logs',
    titleKey: 'Bridge Audit Logs',
    descriptionKey:
      'Review Bridge request audit logs for remote client activity and policy decisions.',
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
