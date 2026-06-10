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
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import {
  type ColumnDef,
  type VisibilityState,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useMediaQuery } from '@/hooks'
import {
  Code2,
  CreditCard,
  History,
  Info,
  MoreHorizontal,
  Pencil,
  Plus,
  Radio,
  RefreshCw,
  SearchCheck,
  Trash2,
  Wrench,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatNumber, formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTableColumnHeader, DataTablePage } from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { StatusBadge, StatusBadgeList } from '@/components/status-badge'
import {
  deleteMCPProxyServer,
  discoverMCPProxyServerTools,
  listMCPProxyServers,
  mcpQueryKeys,
  runMCPProxyHealthCheck,
  testMCPProxyServer,
} from '../api'
import {
  getProxyAuthTypeLabel,
  getProxyDiscoveryEventStatusLabel,
  getProxyDiscoveryEventTypeLabel,
  getProxyReviewReasonLabel,
  getProxyServerStatusOptions,
  getProxyTransportLabel,
  getProxyTransportOptions,
  getProxyVisibilityLabel,
} from '../constants'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type {
  MCPProxyDiscoveryResult,
  MCPProxyHeartbeatRunResponse,
  MCPProxyHealthCheckRunResponse,
  MCPProxyServer,
  MCPProxyServerTestResult,
} from '../types'
import { JsonDetailDialog } from './json-detail-dialog'
import { ProxyHeartbeatDialog } from './proxy-heartbeat-dialog'
import { ProxyServerDetailPanel } from './proxy-server-detail-panel'
import { ProxyServerEditDialog } from './proxy-server-edit-dialog'
import { ProxyTrendsPanel } from './proxy-trends-panel'
import {
  IdCell,
  LongTextCell,
  ProxyServerStatusBadge,
  TimestampCell,
} from './table-cells'

const route = getRouteApi('/_authenticated/mcp/$section')

type ProxyServerActionState = {
  title: string
  description: string
  value:
    | MCPProxyDiscoveryResult
    | MCPProxyHeartbeatRunResponse
    | MCPProxyHealthCheckRunResponse
    | MCPProxyServerTestResult
} | null

function GroupsCell(props: { groups: string[] }) {
  if (!props.groups.length) {
    return <span className='text-muted-foreground'>-</span>
  }

  return (
    <StatusBadgeList
      items={props.groups}
      max={2}
      renderItem={(group) => (
        <StatusBadge label={group} autoColor={group} copyable={false} />
      )}
    />
  )
}

function formatRate(value: number): string {
  return `${formatNumber(value)}%`
}

function ProxyServerHealthCell(props: { server: MCPProxyServer }) {
  const { t } = useTranslation()
  const health = props.server.health
  if (!health) {
    return <span className='text-muted-foreground'>-</span>
  }
  const failures = health.calls.error_calls + health.calls.timeout_calls

  return (
    <div className='flex min-w-[220px] flex-col gap-1 text-xs'>
      <div className='flex flex-wrap items-center gap-1.5 tabular-nums'>
        <span>
          {t('Calls')}: {formatNumber(health.calls.total_calls)}
        </span>
        <span className='text-muted-foreground'>
          {t('Success')}: {formatRate(health.calls.success_rate)}
        </span>
        {failures > 0 && (
          <span className='text-destructive'>
            {t('Errors')}: {formatNumber(failures)}
          </span>
        )}
      </div>
      <div className='text-muted-foreground flex flex-wrap gap-x-2 gap-y-1 tabular-nums'>
        <span>
          {t('Tools')}: {formatNumber(health.discovery.total_tools)}
        </span>
        <span>
          {t('Quota Used')}: {formatQuota(health.calls.quota)}
        </span>
      </div>
      {health.needs_review && (
        <div className='flex flex-wrap gap-1'>
          <StatusBadge
            label={t('Needs Review')}
            variant='warning'
            copyable={false}
          />
          {health.review_reasons.slice(0, 2).map((reason) => (
            <StatusBadge
              key={reason}
              label={t(getProxyReviewReasonLabel(reason))}
              autoColor={reason}
              copyable={false}
            />
          ))}
        </div>
      )}
      {health.latest_check && (
        <div className='text-muted-foreground flex flex-wrap gap-x-2 gap-y-1 text-xs'>
          <span>
            {t('Latest Check')}:{' '}
            {t(getProxyDiscoveryEventTypeLabel(health.latest_check.event_type))}
          </span>
          <span>
            {t(getProxyDiscoveryEventStatusLabel(health.latest_check.status))}
          </span>
        </div>
      )}
      {health.top_tool && (
        <LongText className='text-muted-foreground max-w-[260px] font-mono text-xs'>
          {health.top_tool.exposed_tool_name}
        </LongText>
      )}
      {health.latest_error && (
        <LongText className='text-destructive max-w-[260px] text-xs'>
          {health.latest_error.error_message ||
            health.latest_error.error_code ||
            health.latest_error.request_id}
        </LongText>
      )}
    </div>
  )
}

function ProxyServerActionsCell(props: {
  server: MCPProxyServer
  onDelete: (server: MCPProxyServer) => void
  onDiscover: (server: MCPProxyServer) => void
  onEdit: (server: MCPProxyServer) => void
  onOpenBilling: (server: MCPProxyServer) => void
  onOpenCalls: (server: MCPProxyServer) => void
  onOpenTools: (server: MCPProxyServer) => void
  onTest: (server: MCPProxyServer) => void
  onViewDetail: (server: MCPProxyServer) => void
  onViewConfig: (server: MCPProxyServer) => void
}) {
  const { t } = useTranslation()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant='ghost' size='icon-sm' />}>
        <MoreHorizontal className='size-4' />
        <span className='sr-only'>{t('Open menu')}</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem onSelect={() => props.onViewDetail(props.server)}>
          <Info className='size-4' />
          {t('View Detail')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onEdit(props.server)}>
          <Pencil className='size-4' />
          {t('Edit')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onTest(props.server)}>
          <SearchCheck className='size-4' />
          {t('Test Connection')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onDiscover(props.server)}>
          <RefreshCw className='size-4' />
          {t('Discover Tools')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onOpenTools(props.server)}>
          <Wrench className='size-4' />
          {t('Tools')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onOpenCalls(props.server)}>
          <History className='size-4' />
          {t('Open Tool Calls')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onOpenBilling(props.server)}>
          <CreditCard className='size-4' />
          {t('Open Billing Events')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onViewConfig(props.server)}>
          <Code2 className='size-4' />
          {t('View Config')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onDelete(props.server)}>
          <Trash2 className='size-4' />
          {t('Delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function useProxyServerColumns(options: {
  onDelete: (server: MCPProxyServer) => void
  onDiscover: (server: MCPProxyServer) => void
  onEdit: (server: MCPProxyServer) => void
  onOpenBilling: (server: MCPProxyServer) => void
  onOpenCalls: (server: MCPProxyServer) => void
  onOpenTools: (server: MCPProxyServer) => void
  onTest: (server: MCPProxyServer) => void
  onViewDetail: (server: MCPProxyServer) => void
  onViewConfig: (server: MCPProxyServer) => void
}): ColumnDef<MCPProxyServer>[] {
  const { t } = useTranslation()

  return useMemo(
    () => [
      {
        accessorKey: 'id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title='ID' />
        ),
        cell: ({ row }) => <IdCell value={row.original.id} />,
        meta: { label: t('ID'), mobileHidden: true },
      },
      {
        accessorKey: 'name',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Server')} />
        ),
        cell: ({ row }) => {
          const server = row.original
          return (
            <div className='flex min-w-[220px] flex-col gap-1'>
              <div className='flex min-w-0 items-center gap-2'>
                <LongText className='max-w-[180px] font-medium'>
                  {server.name}
                </LongText>
                <StatusBadge
                  label={t(getProxyTransportLabel(server.transport))}
                  variant='info'
                  copyable={false}
                />
              </div>
              <LongText className='text-muted-foreground max-w-[240px] font-mono text-xs'>
                {server.namespace}
              </LongText>
            </div>
          )
        },
        enableHiding: false,
        meta: { label: t('Server'), mobileTitle: true },
      },
      {
        accessorKey: 'status',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Status')} />
        ),
        cell: ({ row }) => (
          <ProxyServerStatusBadge status={row.original.status} />
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Status'), mobileBadge: true },
      },
      {
        accessorKey: 'transport',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Transport')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={t(getProxyTransportLabel(row.original.transport))}
            variant='info'
            copyable={false}
          />
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Transport'), mobileHidden: true },
      },
      {
        accessorKey: 'endpoint',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Endpoint')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.endpoint}
            className='max-w-[320px] font-mono'
          />
        ),
        meta: { label: t('Endpoint') },
      },
      {
        accessorKey: 'auth_type',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Auth')} />
        ),
        cell: ({ row }) => (
          <div className='flex min-w-[120px] flex-wrap gap-1'>
            <StatusBadge
              label={t(getProxyAuthTypeLabel(row.original.auth_type))}
              variant='neutral'
              copyable={false}
            />
            {row.original.auth_ref && (
              <StatusBadge
                label={t('Configured')}
                variant='success'
                copyable={false}
              />
            )}
          </div>
        ),
        meta: { label: t('Auth'), mobileHidden: true },
      },
      {
        accessorKey: 'visibility',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Visibility')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={t(getProxyVisibilityLabel(row.original.visibility))}
            variant='neutral'
            copyable={false}
          />
        ),
        meta: { label: t('Visibility'), mobileHidden: true },
      },
      {
        accessorKey: 'allowed_groups',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Groups')} />
        ),
        cell: ({ row }) => (
          <GroupsCell groups={row.original.allowed_groups ?? []} />
        ),
        meta: { label: t('Groups'), mobileHidden: true },
      },
      {
        id: 'health',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Health')} />
        ),
        cell: ({ row }) => <ProxyServerHealthCell server={row.original} />,
        meta: { label: t('Health') },
      },
      {
        accessorKey: 'last_discovered_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Discovered At')} />
        ),
        cell: ({ row }) => (
          <TimestampCell value={row.original.last_discovered_at} />
        ),
        meta: { label: t('Discovered At') },
      },
      {
        accessorKey: 'last_error',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Last Error')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.last_error}
            className='max-w-[260px]'
          />
        ),
        meta: { label: t('Last Error'), mobileHidden: true },
      },
      {
        id: 'actions',
        cell: ({ row }) => (
          <ProxyServerActionsCell
            server={row.original}
            onDelete={options.onDelete}
            onDiscover={options.onDiscover}
            onEdit={options.onEdit}
            onOpenBilling={options.onOpenBilling}
            onOpenCalls={options.onOpenCalls}
            onOpenTools={options.onOpenTools}
            onTest={options.onTest}
            onViewDetail={options.onViewDetail}
            onViewConfig={options.onViewConfig}
          />
        ),
        enableSorting: false,
        enableHiding: false,
        meta: { label: t('Actions') },
      },
    ],
    [
      options.onDelete,
      options.onDiscover,
      options.onEdit,
      options.onOpenBilling,
      options.onOpenCalls,
      options.onOpenTools,
      options.onTest,
      options.onViewDetail,
      options.onViewConfig,
      t,
    ]
  )
}

export function MCPProxyServersTable() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = route.useNavigate()
  const search = route.useSearch()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    last_error: false,
    transport: false,
  })
  const [heartbeatOpen, setHeartbeatOpen] = useState(false)
  const [editorOpen, setEditorOpen] = useState(false)
  const [editServer, setEditServer] = useState<MCPProxyServer | null>(null)
  const [configServer, setConfigServer] = useState<MCPProxyServer | null>(null)
  const [deleteServer, setDeleteServer] = useState<MCPProxyServer | null>(null)
  const [actionResult, setActionResult] = useState<ProxyServerActionState>(null)

  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search,
    navigate,
    pagination: {
      pageKey: 'proxyServersPage',
      pageSizeKey: 'proxyServersPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : 20,
    },
    globalFilter: { enabled: true, key: 'proxyServerFilter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'proxyServerStatus', type: 'array' },
      { columnId: 'transport', searchKey: 'proxyTransport', type: 'array' },
    ],
  })

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []
  const transportFilter =
    (columnFilters.find((filter) => filter.id === 'transport')?.value as
      | string[]
      | undefined) ?? []

  const requestParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    keyword: globalFilter,
    status: statusFilter[0],
    transport: transportFilter[0],
  }

  const {
    data,
    error: serversError,
    isError: isServersError,
    isLoading,
    isFetching,
    refetch,
  } = useQuery({
    queryKey: mcpQueryKeys.proxyServersList(requestParams),
    queryFn: async () => {
      const result = await listMCPProxyServers(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load MCP proxy servers')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  useEffect(() => {
    if (!isServersError) return
    toast.error(
      mcpQueryErrorMessage(serversError, t('Failed to load MCP proxy servers'))
    )
  }, [isServersError, serversError, t])

  const testMutation = useMutation({
    mutationFn: testMCPProxyServer,
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to test MCP proxy server'))
        return
      }
      toast.success(t('MCP proxy server test succeeded'))
      setActionResult({
        title: t('Test Result'),
        description: res.data.server_name,
        value: res.data,
      })
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.proxyServers() })
    },
  })

  const discoverMutation = useMutation({
    mutationFn: discoverMCPProxyServerTools,
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to discover MCP proxy tools'))
        return
      }
      toast.success(t('MCP proxy tools discovered successfully'))
      setActionResult({
        title: t('Discovery Result'),
        description: `${res.data.discovered_count}`,
        value: res.data,
      })
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.proxyServers() })
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.proxyTools() })
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.tools() })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteMCPProxyServer,
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to delete MCP proxy server'))
        return
      }
      toast.success(t('MCP proxy server deleted successfully'))
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.proxyServers() })
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.proxyTools() })
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.tools() })
    },
  })

  const healthCheckMutation = useMutation({
    mutationFn: () =>
      runMCPProxyHealthCheck({
        discover: false,
        force: true,
      }),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to run MCP proxy health check'))
        return
      }
      toast.success(t('MCP proxy health check completed'))
      setActionResult({
        title: t('Health Check Result'),
        description: res.data.message,
        value: res.data,
      })
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.proxyServers() })
    },
  })

  const columns = useProxyServerColumns({
    onDelete: setDeleteServer,
    onDiscover: (server) => discoverMutation.mutate(server.id),
    onEdit: (server) => {
      setEditServer(server)
      setEditorOpen(true)
    },
    onOpenTools: (server) => {
      void navigate({
        to: '/mcp/$section',
        params: { section: 'proxy-tools' },
        search: (prev) => ({
          ...prev,
          proxyToolsPage: undefined,
          proxyToolFilter: undefined,
          proxyToolStatus: undefined,
          proxySchemaHash: undefined,
          proxyServerId: server.id,
        }),
      })
    },
    onOpenCalls: (server) => {
      void navigate({
        to: '/mcp/$section',
        params: { section: 'tool-calls' },
        search: (prev) => ({
          ...prev,
          callsPage: undefined,
          callsStartTime: Date.now() - 24 * 60 * 60 * 1000,
          callsEndTime: undefined,
          callStatus: undefined,
          requestId: undefined,
          sessionId: undefined,
          targetClient: undefined,
          toolName: server.health?.top_tool?.exposed_tool_name ?? '',
          filter: server.health?.top_tool ? undefined : server.namespace,
        }),
      })
    },
    onOpenBilling: (server) => {
      void navigate({
        to: '/mcp/$section',
        params: { section: 'billing-events' },
        search: (prev) => ({
          ...prev,
          billingPage: undefined,
          billingStartTime: Date.now() - 24 * 60 * 60 * 1000,
          billingEndTime: undefined,
          billingSourceKind: ['mcp_tool_call'],
          billingEventType: undefined,
          billingStatus: undefined,
          billingUsageKind: undefined,
          billingSource: undefined,
          billingSourceId: undefined,
          requestId: undefined,
          filter:
            server.health?.top_tool?.exposed_tool_name ?? server.namespace,
        }),
      })
    },
    onTest: (server) => testMutation.mutate(server.id),
    onViewDetail: (server) => {
      void navigate({
        search: (prev) => ({
          ...prev,
          proxyServerId: server.id,
        }),
      })
    },
    onViewConfig: setConfigServer,
  })

  const table = useReactTable({
    data: data?.items ?? [],
    columns,
    pageCount: Math.ceil((data?.total ?? 0) / pagination.pageSize),
    state: {
      columnFilters,
      columnVisibility,
      globalFilter,
      pagination,
    },
    onColumnFiltersChange,
    onColumnVisibilityChange: setColumnVisibility,
    onGlobalFilterChange,
    onPaginationChange,
    getCoreRowModel: getCoreRowModel(),
    manualFiltering: true,
    manualPagination: true,
  })

  const pageCount = table.getPageCount()
  useEffect(() => {
    ensurePageInRange(pageCount)
  }, [ensurePageInRange, pageCount])

  const isMutating =
    testMutation.isPending ||
    discoverMutation.isPending ||
    healthCheckMutation.isPending ||
    deleteMutation.isPending

  return (
    <>
      {search.proxyServerId && (
        <ProxyServerDetailPanel
          serverId={search.proxyServerId}
          onClose={() => {
            void navigate({
              search: (prev) => ({
                ...prev,
                proxyServerId: undefined,
              }),
            })
          }}
          onOpenTools={() => {
            void navigate({
              to: '/mcp/$section',
              params: { section: 'proxy-tools' },
              search: (prev) => ({
                ...prev,
                proxyToolsPage: undefined,
                proxyToolFilter: undefined,
                proxyToolStatus: undefined,
                proxySchemaHash: undefined,
                proxyServerId: search.proxyServerId,
              }),
            })
          }}
        />
      )}
      <ProxyTrendsPanel
        onOpenServerTools={(server) => {
          void navigate({
            to: '/mcp/$section',
            params: { section: 'proxy-tools' },
            search: (prev) => ({
              ...prev,
              proxyToolsPage: undefined,
              proxyToolFilter: undefined,
              proxyToolStatus: undefined,
              proxySchemaHash: undefined,
              proxyToolId: undefined,
              proxyServerId: server.proxy_server_id,
            }),
          })
        }}
        onOpenServerCalls={(server, range) => {
          void navigate({
            to: '/mcp/$section',
            params: { section: 'tool-calls' },
            search: (prev) => ({
              ...prev,
              callsPage: undefined,
              callsStartTime: range.startTime * 1000,
              callsEndTime: range.endTime * 1000,
              callStatus: undefined,
              requestId: undefined,
              sessionId: undefined,
              targetClient: undefined,
              toolName: undefined,
              filter: server.namespace,
            }),
          })
        }}
        onOpenServerBilling={(server, range) => {
          void navigate({
            to: '/mcp/$section',
            params: { section: 'billing-events' },
            search: (prev) => ({
              ...prev,
              billingPage: undefined,
              billingStartTime: range.startTime * 1000,
              billingEndTime: range.endTime * 1000,
              billingSourceKind: ['mcp_tool_call'],
              billingEventType: undefined,
              billingStatus: undefined,
              billingUsageKind: undefined,
              billingSource: undefined,
              billingSourceId: undefined,
              requestId: undefined,
              filter: server.namespace,
            }),
          })
        }}
        onOpenToolCalls={(tool, range) => {
          void navigate({
            to: '/mcp/$section',
            params: { section: 'tool-calls' },
            search: (prev) => ({
              ...prev,
              callsPage: undefined,
              callsStartTime: range.startTime * 1000,
              callsEndTime: range.endTime * 1000,
              callStatus: undefined,
              requestId: undefined,
              sessionId: undefined,
              targetClient: undefined,
              toolName: tool.exposed_tool_name,
              filter: undefined,
            }),
          })
        }}
        onOpenToolBilling={(tool, range) => {
          void navigate({
            to: '/mcp/$section',
            params: { section: 'billing-events' },
            search: (prev) => ({
              ...prev,
              billingPage: undefined,
              billingStartTime: range.startTime * 1000,
              billingEndTime: range.endTime * 1000,
              billingSourceKind: ['mcp_tool_call'],
              billingEventType: undefined,
              billingStatus: undefined,
              billingUsageKind: undefined,
              billingSource: undefined,
              billingSourceId: undefined,
              requestId: undefined,
              filter: tool.exposed_tool_name,
            }),
          })
        }}
      />
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching || isMutating}
        emptyTitle={t('No Proxy Servers Found')}
        emptyDescription={t(
          'No MCP proxy servers configured. Add a remote MCP server to discover tools.'
        )}
        skeletonKeyPrefix='mcp-proxy-servers-skeleton'
        toolbarProps={{
          searchPlaceholder: t('Filter by server, namespace or endpoint...'),
          filters: [
            {
              columnId: 'status',
              title: t('Status'),
              options: getProxyServerStatusOptions(t),
              singleSelect: true,
            },
            {
              columnId: 'transport',
              title: t('Transport'),
              options: getProxyTransportOptions(t),
              singleSelect: true,
            },
          ],
          preActions: (
            <>
              <Button
                type='button'
                variant='outline'
                onClick={() => void refetch()}
                disabled={isFetching}
                className={cn(isFetching && 'opacity-80')}
              >
                <RefreshCw
                  className={cn('size-4', isFetching && 'animate-spin')}
                />
                {t('Refresh')}
              </Button>
              <Button
                type='button'
                variant='outline'
                onClick={() => setHeartbeatOpen(true)}
              >
                <Radio className='size-4' />
                {t('Heartbeat')}
              </Button>
              <Button
                type='button'
                variant='outline'
                onClick={() => healthCheckMutation.mutate()}
                disabled={healthCheckMutation.isPending}
                className={cn(healthCheckMutation.isPending && 'opacity-80')}
              >
                <SearchCheck
                  className={cn(
                    'size-4',
                    healthCheckMutation.isPending && 'animate-pulse'
                  )}
                />
                {t('Run Health Check')}
              </Button>
              <Button
                type='button'
                onClick={() => {
                  setEditServer(null)
                  setEditorOpen(true)
                }}
              >
                <Plus className='size-4' />
                {t('New Proxy Server')}
              </Button>
            </>
          ),
        }}
      />

      <ProxyHeartbeatDialog
        open={heartbeatOpen}
        onOpenChange={setHeartbeatOpen}
        onViewRun={(run) =>
          setActionResult({
            title: t('Heartbeat Result'),
            description: run.message,
            value: run,
          })
        }
      />
      {editorOpen && (
        <ProxyServerEditDialog
          key={editServer?.id ?? 'new'}
          open={editorOpen}
          server={editServer}
          onOpenChange={(open) => {
            setEditorOpen(open)
            if (!open) setEditServer(null)
          }}
        />
      )}
      <JsonDetailDialog
        open={configServer != null}
        title={t('Proxy Server Config')}
        description={configServer?.name}
        value={configServer}
        onOpenChange={(open) => !open && setConfigServer(null)}
      />
      <JsonDetailDialog
        open={actionResult != null}
        title={actionResult?.title ?? ''}
        description={actionResult?.description}
        value={actionResult?.value}
        onOpenChange={(open) => !open && setActionResult(null)}
      />
      <ConfirmDialog
        open={deleteServer != null}
        onOpenChange={(open) => !open && setDeleteServer(null)}
        title={t('Delete Proxy Server')}
        desc={
          deleteServer
            ? t(
                'This will archive the proxy server and disable discovered tools.'
              )
            : ''
        }
        confirmText={t('Delete')}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (deleteServer) {
            deleteMutation.mutate(deleteServer.id, {
              onSuccess: () => setDeleteServer(null),
            })
          }
        }}
      />
    </>
  )
}
