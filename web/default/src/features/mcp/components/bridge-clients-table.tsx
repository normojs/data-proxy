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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import {
  type ColumnDef,
  type VisibilityState,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useMediaQuery } from '@/hooks'
import { FileText, History, Info, MoreHorizontal, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Switch } from '@/components/ui/switch'
import { DataTableColumnHeader, DataTablePage } from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { StatusBadge, StatusBadgeList } from '@/components/status-badge'
import { listBridgeClients, mcpQueryKeys } from '../api'
import { getBridgeClientStatusOptions } from '../constants'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type { BridgeClient } from '../types'
import { BridgeClientDetailPanel } from './bridge-client-detail-panel'
import {
  ClientStatusBadge,
  IdCell,
  LongTextCell,
  TimestampCell,
} from './table-cells'

const route = getRouteApi('/_authenticated/mcp/$section')
const BRIDGE_CLIENTS_REFETCH_INTERVAL_MS = 5000

function CapabilitiesCell(props: { capabilities: string[] }) {
  return (
    <StatusBadgeList
      items={props.capabilities}
      max={2}
      renderItem={(capability) => (
        <StatusBadge
          label={capability}
          autoColor={capability}
          copyable={false}
        />
      )}
    />
  )
}

function BridgeClientActionsCell(props: {
  client: BridgeClient
  onOpenAuditLogs: (client: BridgeClient) => void
  onOpenToolCalls: (client: BridgeClient) => void
  onViewDetail: (client: BridgeClient) => void
}) {
  const { t } = useTranslation()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant='ghost' size='icon-sm' />}>
        <MoreHorizontal className='size-4' />
        <span className='sr-only'>{t('Open menu')}</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem onSelect={() => props.onViewDetail(props.client)}>
          <Info className='size-4' />
          {t('View Detail')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onOpenAuditLogs(props.client)}>
          <FileText className='size-4' />
          {t('Open Audit Logs')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onOpenToolCalls(props.client)}>
          <History className='size-4' />
          {t('Open Tool Calls')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function useBridgeClientColumns(options: {
  onOpenAuditLogs: (client: BridgeClient) => void
  onOpenToolCalls: (client: BridgeClient) => void
  onViewDetail: (client: BridgeClient) => void
}): ColumnDef<BridgeClient>[] {
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
        accessorKey: 'client_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Client')} />
        ),
        cell: ({ row }) => {
          const client = row.original
          return (
            <div className='flex min-w-[220px] flex-col gap-1'>
              <div className='flex min-w-0 items-center gap-2'>
                <LongText className='max-w-[180px] font-medium'>
                  {client.name || client.client_id}
                </LongText>
                {client.version && (
                  <StatusBadge
                    label={client.version}
                    variant='neutral'
                    copyable={false}
                  />
                )}
              </div>
              <LongText className='text-muted-foreground max-w-[240px] font-mono text-xs'>
                {client.client_id}
              </LongText>
            </div>
          )
        },
        enableHiding: false,
        meta: { label: t('Client'), mobileTitle: true },
      },
      {
        accessorKey: 'status',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Status')} />
        ),
        cell: ({ row }) => (
          <ClientStatusBadge
            status={row.original.status}
            online={row.original.online}
          />
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Status'), mobileBadge: true },
      },
      {
        accessorKey: 'platform',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Platform')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={row.original.platform || '-'}
            autoColor={row.original.platform || undefined}
            copyable={false}
          />
        ),
        meta: { label: t('Platform') },
      },
      {
        accessorKey: 'workspace',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Workspace')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.workspace}
            className='max-w-[260px]'
          />
        ),
        meta: { label: t('Workspace') },
      },
      {
        accessorKey: 'capabilities',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Capabilities')} />
        ),
        cell: ({ row }) => (
          <CapabilitiesCell capabilities={row.original.capabilities ?? []} />
        ),
        meta: { label: t('Capabilities'), mobileHidden: true },
      },
      {
        accessorKey: 'session_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Session')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.session_id}
            className='max-w-[180px] font-mono'
          />
        ),
        meta: { label: t('Session'), mobileHidden: true },
      },
      {
        accessorKey: 'last_seen_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Last Seen')} />
        ),
        cell: ({ row }) => <TimestampCell value={row.original.last_seen_at} />,
        meta: { label: t('Last Seen') },
      },
      {
        id: 'actions',
        cell: ({ row }) => (
          <BridgeClientActionsCell
            client={row.original}
            onOpenAuditLogs={options.onOpenAuditLogs}
            onOpenToolCalls={options.onOpenToolCalls}
            onViewDetail={options.onViewDetail}
          />
        ),
        enableSorting: false,
        enableHiding: false,
        meta: { label: t('Actions') },
      },
    ],
    [options.onOpenAuditLogs, options.onOpenToolCalls, options.onViewDetail, t]
  )
}

export function BridgeClientsTable() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const navigate = route.useNavigate()
  const search = route.useSearch()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})

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
      pageKey: 'clientsPage',
      pageSizeKey: 'clientsPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : 20,
    },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'clientStatus', type: 'array' },
    ],
  })

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []

  const requestParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    scope: isAdmin ? ('all' as const) : undefined,
    keyword: globalFilter,
    status: statusFilter[0],
  }

  const {
    data,
    error: bridgeClientsError,
    isError: isBridgeClientsError,
    isLoading,
    isFetching,
    refetch,
  } = useQuery({
    queryKey: mcpQueryKeys.bridgeClientsList(requestParams),
    queryFn: async () => {
      const result = await listBridgeClients(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load bridge clients')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
    refetchInterval: autoRefresh ? BRIDGE_CLIENTS_REFETCH_INTERVAL_MS : false,
  })

  useEffect(() => {
    if (!isBridgeClientsError) return
    toast.error(
      mcpQueryErrorMessage(
        bridgeClientsError,
        t('Failed to load bridge clients')
      )
    )
  }, [bridgeClientsError, isBridgeClientsError, t])

  const columns = useBridgeClientColumns({
    onOpenAuditLogs: (client) => {
      void navigate({
        to: '/mcp/$section',
        params: { section: 'audit-logs' },
        search: (prev) => ({
          ...prev,
          auditPage: undefined,
          auditStartTime: Date.now() - 24 * 60 * 60 * 1000,
          auditEndTime: undefined,
          auditStatus: undefined,
          requestId: undefined,
          clientId: client.client_id,
          sessionId: client.session_id,
        }),
      })
    },
    onOpenToolCalls: (client) => {
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
          sessionId: client.session_id,
          targetClient: client.client_id,
          toolName: '',
        }),
      })
    },
    onViewDetail: (client) => {
      void navigate({
        search: (prev) => ({
          ...prev,
          clientId: client.client_id,
        }),
      })
    },
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

  return (
    <>
      {search.clientId && (
        <BridgeClientDetailPanel
          clientId={search.clientId}
          isAdmin={isAdmin}
          scope={isAdmin ? 'all' : undefined}
          onClose={() => {
            void navigate({
              search: (prev) => ({
                ...prev,
                clientId: undefined,
              }),
            })
          }}
        />
      )}
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No Bridge Clients Found')}
        emptyDescription={t(
          'No local bridge clients are connected. QidianBrowser clients will appear here after registration.'
        )}
        skeletonKeyPrefix='bridge-clients-skeleton'
        toolbarProps={{
          searchPlaceholder: t('Filter by client, workspace or capability...'),
          filters: [
            {
              columnId: 'status',
              title: t('Status'),
              options: getBridgeClientStatusOptions(t),
              singleSelect: true,
            },
          ],
          preActions: (
            <>
              <div className='flex h-9 items-center gap-2 rounded-md border px-3 text-sm'>
                <Switch
                  size='sm'
                  checked={autoRefresh}
                  onCheckedChange={setAutoRefresh}
                  aria-label={t('Auto refresh')}
                />
                <span>{t('Auto refresh')}</span>
                <span className='text-muted-foreground text-xs'>5s</span>
              </div>
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
            </>
          ),
        }}
      />
    </>
  )
}
