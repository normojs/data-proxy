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
import {
  Activity,
  Box,
  CreditCard,
  Database,
  FileText,
  FlaskConical,
  Key,
  LayoutDashboard,
  ListTodo,
  MessageSquare,
  Network,
  Package,
  Puzzle,
  Radio,
  Settings,
  ShieldCheck,
  Ticket,
  TimerReset,
  User,
  Users,
  Wallet,
  Gift,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { type SidebarData } from '@/components/layout/types'

/**
 * Root navigation groups for the application sidebar.
 *
 * These are shown when the URL does not match any nested sidebar view
 * registered in `layout/lib/sidebar-view-registry.ts`.
 */
export function useSidebarData(): SidebarData {
  const { t } = useTranslation()

  return {
    navGroups: [
      {
        id: 'chat',
        title: t('Chat'),
        items: [
          {
            title: t('Playground'),
            url: '/playground',
            icon: FlaskConical,
          },
          {
            title: t('Chat'),
            icon: MessageSquare,
            type: 'chat-presets',
          },
        ],
      },
      {
        id: 'general',
        title: t('General'),
        items: [
          {
            title: t('Overview'),
            url: '/dashboard/overview',
            icon: Activity,
          },
          {
            title: t('Dashboard'),
            url: '/dashboard/models',
            icon: LayoutDashboard,
          },
          {
            title: t('API Keys'),
            url: '/keys',
            icon: Key,
          },
          {
            title: t('Usage Logs'),
            url: '/usage-logs/common',
            icon: FileText,
          },
          {
            title: t('Quota Requests'),
            url: '/quota-requests',
            icon: TimerReset,
          },
          {
            title: t('Subsites'),
            url: '/dashboard/subsites',
            icon: Network,
          },
          {
            title: t('Task Logs'),
            url: '/usage-logs/task',
            activeUrls: ['/usage-logs/drawing'],
            configUrls: ['/usage-logs/drawing', '/usage-logs/task'],
            icon: ListTodo,
          },
          {
            title: t('MCP Market'),
            url: '/mcp/market',
            icon: Puzzle,
          },
        ],
      },
      {
        id: 'tunnel',
        title: t('Tunnels'),
        items: [
          {
            title: t('My Devices'),
            url: '/mcp/bridge-clients',
            icon: Radio,
          },
          {
            title: t('My Tunnel Apps'),
            url: '/mcp/my-tunnel-apps',
            icon: Network,
          },
          {
            title: t('Connections & Links'),
            url: '/mcp/tunnel-connections',
            icon: Key,
          },
          {
            title: t('Sessions'),
            url: '/mcp/tunnel-sessions',
            icon: Activity,
          },
        ],
      },
      {
        id: 'personal',
        title: t('Personal'),
        items: [
          {
            title: t('Wallet'),
            url: '/wallet',
            icon: Wallet,
          },
          {
            title: t('Invite friends'),
            url: '/invitation',
            icon: Gift,
          },
          {
            title: t('Profile'),
            url: '/profile',
            icon: User,
          },
        ],
      },
      {
        id: 'admin',
        title: t('Admin'),
        items: [
          {
            title: t('Site Dashboard'),
            url: '/dashboard/site-models',
            activeUrls: ['/dashboard/site-models', '/dashboard/users'],
            icon: LayoutDashboard,
          },
          {
            title: t('Channels'),
            url: '/channels',
            icon: Radio,
          },
          {
            title: t('Models'),
            url: '/models/metadata',
            icon: Box,
          },
          {
            title: t('Users'),
            url: '/users',
            icon: Users,
          },
          {
            title: t('Enterprise Governance'),
            url: '/enterprise',
            icon: ShieldCheck,
          },
          {
            title: t('Training Data'),
            url: '/training-data',
            icon: Database,
          },
          {
            title: t('MCP'),
            icon: Puzzle,
            activeUrls: [
              '/mcp/tools',
              '/mcp/market',
              '/mcp/bridge-clients',
              '/mcp/tool-calls',
              '/mcp/billing-events',
              '/mcp/audit-logs',
              '/mcp/tunnel-apps',
              '/mcp/my-tunnel-apps',
              '/mcp/tunnel-connections',
              '/mcp/tunnel-sessions',
            ],
            configUrls: [
              '/mcp',
              '/mcp/tools',
              '/mcp/market',
              '/mcp/bridge-clients',
              '/mcp/tool-calls',
              '/mcp/billing-events',
              '/mcp/audit-logs',
              '/mcp/tunnel-apps',
              '/mcp/my-tunnel-apps',
              '/mcp/tunnel-connections',
              '/mcp/tunnel-sessions',
            ],
            items: [
              {
                title: t('MCP Tools'),
                url: '/mcp/tools',
              },
              {
                title: t('Bridge Clients'),
                url: '/mcp/bridge-clients',
              },
              {
                title: t('Tunnel Apps (Approve)'),
                url: '/mcp/tunnel-apps',
              },
              {
                title: t('Tool Calls'),
                url: '/mcp/tool-calls',
              },
              {
                title: t('Billing Events'),
                url: '/mcp/billing-events',
              },
              {
                title: t('Bridge Audit Logs'),
                url: '/mcp/audit-logs',
              },
            ],
          },
          {
            title: t('Redemption Codes'),
            url: '/redemption-codes',
            icon: Ticket,
          },
          {
            title: t('Package SKUs'),
            url: '/package-skus',
            icon: Package,
          },
          {
            title: t('Subscription Management'),
            url: '/subscriptions',
            icon: CreditCard,
          },
          {
            title: t('System Settings'),
            url: '/system-settings/site',
            activeUrls: ['/system-settings'],
            icon: Settings,
          },
        ],
      },
    ],
  }
}
