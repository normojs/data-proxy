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
import z from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { MCPDashboard } from '@/features/mcp'
import {
  MCP_DEFAULT_SECTION,
  isMCPSectionId,
} from '@/features/mcp/section-registry'

const arraySearchSchema = z
  .preprocess((value) => {
    if (value == null || value === '') return undefined
    return Array.isArray(value) ? value : [value]
  }, z.array(z.string()).optional())
  .catch([])

const mcpSearchSchema = z.object({
  page: z.number().optional().catch(1),
  pageSize: z.number().optional().catch(undefined),
  clientsPage: z.number().optional().catch(1),
  clientsPageSize: z.number().optional().catch(undefined),
  callsPage: z.number().optional().catch(1),
  callsPageSize: z.number().optional().catch(undefined),
  billingPage: z.number().optional().catch(1),
  billingPageSize: z.number().optional().catch(undefined),
  auditPage: z.number().optional().catch(1),
  auditPageSize: z.number().optional().catch(undefined),
  tunnelAppsPage: z.number().optional().catch(1),
  tunnelAppsPageSize: z.number().optional().catch(undefined),
  myTunnelAppsPage: z.number().optional().catch(1),
  myTunnelAppsPageSize: z.number().optional().catch(undefined),
  tunnelConnectionsPage: z.number().optional().catch(1),
  tunnelConnectionsPageSize: z.number().optional().catch(undefined),
  tunnelConnectionAppId: z.number().optional().catch(undefined),
  tunnelSessionsPage: z.number().optional().catch(1),
  tunnelSessionsPageSize: z.number().optional().catch(undefined),
  tunnelSessionAppId: z.number().optional().catch(undefined),
  tunnelSessionConnectionId: z.number().optional().catch(undefined),
  proxyServersPage: z.number().optional().catch(1),
  proxyServersPageSize: z.number().optional().catch(undefined),
  proxyToolsPage: z.number().optional().catch(1),
  proxyToolsPageSize: z.number().optional().catch(undefined),
  filter: z.string().optional().catch(''),
  proxyServerFilter: z.string().optional().catch(''),
  proxyToolFilter: z.string().optional().catch(''),
  tunnelFilter: z.string().optional().catch(''),
  myTunnelFilter: z.string().optional().catch(''),
  tunnelConnectionFilter: z.string().optional().catch(''),
  tunnelSessionFilter: z.string().optional().catch(''),
  toolStatus: arraySearchSchema.optional(),
  clientStatus: arraySearchSchema.optional(),
  callStatus: arraySearchSchema.optional(),
  tunnelStatus: arraySearchSchema.optional(),
  myTunnelStatus: arraySearchSchema.optional(),
  tunnelType: arraySearchSchema.optional(),
  tunnelConnectionStatus: arraySearchSchema.optional(),
  tunnelSessionStatus: arraySearchSchema.optional(),
  proxyServerStatus: arraySearchSchema.optional(),
  proxyToolStatus: arraySearchSchema.optional(),
  proxyTransport: arraySearchSchema.optional(),
  billingSourceKind: arraySearchSchema.optional(),
  billingEventType: arraySearchSchema.optional(),
  billingStatus: arraySearchSchema.optional(),
  billingUsageKind: arraySearchSchema.optional(),
  auditStatus: arraySearchSchema.optional(),
  source: arraySearchSchema.optional(),
  toolName: z.string().optional().catch(''),
  requestId: z.string().optional().catch(''),
  billingSourceId: z.string().optional().catch(''),
  tokenId: z.string().optional().catch(''),
  sessionId: z.string().optional().catch(''),
  targetClient: z.string().optional().catch(''),
  clientId: z.string().optional().catch(''),
  proxyServerId: z.number().optional().catch(undefined),
  proxyToolId: z.number().optional().catch(undefined),
  proxySchemaHash: z.string().optional().catch(''),
  billingSource: arraySearchSchema.optional(),
  callsStartTime: z.number().optional(),
  callsEndTime: z.number().optional(),
  billingStartTime: z.number().optional(),
  billingEndTime: z.number().optional(),
  auditStartTime: z.number().optional(),
  auditEndTime: z.number().optional(),
})

export const Route = createFileRoute('/_authenticated/mcp/$section')({
  beforeLoad: ({ params }) => {
    const { auth } = useAuthStore.getState()

    if (!auth.user) {
      throw redirect({
        to: '/403',
      })
    }

    if (!isMCPSectionId(params.section)) {
      throw redirect({
        to: '/mcp/$section',
        params: { section: MCP_DEFAULT_SECTION },
      })
    }

    if (
      auth.user.role < ROLE.ADMIN &&
      params.section !== 'market' &&
      params.section !== 'my-tunnel-apps' &&
      params.section !== 'tunnel-connections' &&
      params.section !== 'tunnel-sessions'
    ) {
      throw redirect({
        to: '/403',
      })
    }
  },
  validateSearch: mcpSearchSchema,
  component: MCPDashboard,
})
