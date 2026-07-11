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
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { DataTableColumnHeader, DataTablePage } from '@/components/data-table'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import {
  listTunnelConnections,
  listTunnelSessions,
  listUserTunnelApps,
  mcpQueryKeys,
} from '../api'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type {
  TunnelApp,
  TunnelConnection,
  TunnelSession,
  TunnelSessionListParams,
} from '../types'
import {
  IdCell,
  LongTextCell,
  SizeCell,
  TimestampCell,
  TraceCell,
} from './table-cells'

const route = getRouteApi('/_authenticated/mcp/$section')

function tunnelSessionStatusVariant(
  status: string
): StatusBadgeProps['variant'] {
  if (status === 'online') return 'green'
  if (status === 'offline') return 'grey'
  return 'neutral'
}

function tunnelSessionStatusLabel(
  status: string,
  t: (key: string) => string
): string {
  const labels: Record<string, string> = {
    online: t('Online'),
    offline: t('Offline'),
  }
  return labels[status] || status || t('Unknown')
}

function OptionalTimestampCell(props: { value: number }) {
  if (!props.value) return <span className='text-muted-foreground'>-</span>
  return <TimestampCell value={props.value} />
}

function TunnelAppSelector(props: {
  apps: TunnelApp[]
  loading: boolean
  selectedAppId?: number
  onChange: (appId: number) => void
}) {
  const { t } = useTranslation()

  return (
    <div className='flex min-w-[220px] items-center gap-2'>
      <Label
        htmlFor='mcp-tunnel-session-app-select'
        className='shrink-0 text-xs'
      >
        {t('Tunnel App')}
      </Label>
      <NativeSelect
        id='mcp-tunnel-session-app-select'
        size='sm'
        value={props.selectedAppId ? String(props.selectedAppId) : ''}
        disabled={props.loading || props.apps.length === 0}
        onChange={(event) => props.onChange(Number(event.target.value))}
        className='w-[240px] max-w-full'
      >
        {props.apps.length === 0 ? (
          <NativeSelectOption value=''>
            {props.loading ? t('Loading...') : t('No Approved Tunnel Apps')}
          </NativeSelectOption>
        ) : (
          props.apps.map((app) => (
            <NativeSelectOption key={app.id} value={String(app.id)}>
              {app.name}
            </NativeSelectOption>
          ))
        )}
      </NativeSelect>
    </div>
  )
}

function TunnelConnectionSelector(props: {
  connections: TunnelConnection[]
  disabled: boolean
  loading: boolean
  selectedConnectionId?: number
  onChange: (connectionId?: number) => void
}) {
  const { t } = useTranslation()

  return (
    <div className='flex min-w-[220px] items-center gap-2'>
      <Label
        htmlFor='mcp-tunnel-session-connection-select'
        className='shrink-0 text-xs'
      >
        {t('Connection')}
      </Label>
      <NativeSelect
        id='mcp-tunnel-session-connection-select'
        size='sm'
        value={
          props.selectedConnectionId ? String(props.selectedConnectionId) : ''
        }
        disabled={props.disabled || props.loading}
        onChange={(event) => {
          const parsed = Number(event.target.value)
          props.onChange(
            Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
          )
        }}
        className='w-[240px] max-w-full'
      >
        <NativeSelectOption value=''>{t('All Connections')}</NativeSelectOption>
        {props.connections.map((connection) => (
          <NativeSelectOption key={connection.id} value={String(connection.id)}>
            {connection.name || connection.key_prefix || `#${connection.id}`}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    </div>
  )
}

function TrafficCell(props: { bytesIn: number; bytesOut: number }) {
  const { t } = useTranslation()
  return (
    <div className='flex min-w-[120px] flex-col gap-1'>
      <div className='flex items-center gap-1.5'>
        <span className='text-muted-foreground shrink-0 text-[10px] font-medium'>
          {t('In')}
        </span>
        <SizeCell value={props.bytesIn} />
      </div>
      <div className='flex items-center gap-1.5'>
        <span className='text-muted-foreground shrink-0 text-[10px] font-medium'>
          {t('Out')}
        </span>
        <SizeCell value={props.bytesOut} />
      </div>
    </div>
  )
}

function useTunnelSessionColumns(): ColumnDef<TunnelSession>[] {
  const { t } = useTranslation()

  return useMemo(
    () => [
      {
        accessorKey: 'id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('ID')} />
        ),
        cell: ({ row }) => <IdCell value={row.original.id} />,
        meta: { label: t('ID') },
      },
      {
        accessorKey: 'status',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Status')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={tunnelSessionStatusLabel(row.original.status, t)}
            variant={tunnelSessionStatusVariant(row.original.status)}
            pulse={row.original.status === 'online'}
            copyable={false}
          />
        ),
        meta: { label: t('Status') },
      },
      {
        accessorKey: 'session_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Gateway Session')} />
        ),
        cell: ({ row }) => (
          <TraceCell
            className='min-w-[220px]'
            items={[
              { label: t('Gateway'), value: row.original.session_id },
              {
                label: t('Bridge Client'),
                value: row.original.bridge_client_id,
              },
            ]}
          />
        ),
        meta: { label: t('Gateway Session') },
      },
      {
        accessorKey: 'connection_name',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Connection')} />
        ),
        cell: ({ row }) => (
          <TraceCell
            className='min-w-[180px]'
            items={[
              { label: t('Name'), value: row.original.connection_name },
              { label: t('Key'), value: row.original.key_prefix },
              { label: t('ID'), value: row.original.connection_id },
            ]}
          />
        ),
        meta: { label: t('Connection') },
      },
      {
        accessorKey: 'client_version',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Client')} />
        ),
        cell: ({ row }) => (
          <TraceCell
            className='min-w-[210px]'
            items={[
              { label: t('Version'), value: row.original.client_version },
              { label: t('IP'), value: row.original.client_ip },
              { label: t('UA'), value: row.original.user_agent },
            ]}
          />
        ),
        meta: { label: t('Client'), mobileHidden: true },
      },
      {
        id: 'traffic',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Traffic')} />
        ),
        cell: ({ row }) => (
          <TrafficCell
            bytesIn={row.original.bytes_in}
            bytesOut={row.original.bytes_out}
          />
        ),
        meta: { label: t('Traffic'), mobileHidden: true },
      },
      {
        accessorKey: 'connected_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Connected')} />
        ),
        cell: ({ row }) => (
          <OptionalTimestampCell value={row.original.connected_at} />
        ),
        meta: { label: t('Connected'), mobileHidden: true },
      },
      {
        accessorKey: 'last_seen_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Last Seen At')} />
        ),
        cell: ({ row }) => (
          <OptionalTimestampCell value={row.original.last_seen_at} />
        ),
        meta: { label: t('Last Seen At'), mobileHidden: true },
      },
      {
        accessorKey: 'disconnected_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Closed')} />
        ),
        cell: ({ row }) => (
          <div className='flex min-w-[180px] flex-col gap-1'>
            <OptionalTimestampCell value={row.original.disconnected_at} />
            <div className='flex min-w-0 items-center gap-1.5'>
              <span className='text-muted-foreground shrink-0 text-[10px] font-medium'>
                {t('Reason')}
              </span>
              <LongTextCell
                className='max-w-[150px] font-mono text-xs'
                value={row.original.close_reason}
              />
            </div>
          </div>
        ),
        meta: { label: t('Closed'), mobileHidden: true },
      },
    ],
    [t]
  )
}

export function TunnelSessionsTable() {
  const { t } = useTranslation()
  const navigate = route.useNavigate()
  const search = route.useSearch()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const [selectedAppId, setSelectedAppId] = useState<number | undefined>()

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
      pageKey: 'tunnelSessionsPage',
      pageSizeKey: 'tunnelSessionsPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : 20,
    },
    globalFilter: { enabled: true, key: 'tunnelSessionFilter' },
    columnFilters: [
      {
        columnId: 'status',
        searchKey: 'tunnelSessionStatus',
        type: 'array',
      },
    ],
  })

  const appsParams = {
    p: 1,
    page_size: 100,
    status: 'approved',
    app_type: 'mcp_code',
  }
  const {
    data: appsData,
    error: appsError,
    isError: isAppsError,
    isLoading: isAppsLoading,
    isFetching: isAppsFetching,
    refetch: refetchApps,
  } = useQuery({
    queryKey: mcpQueryKeys.userTunnelAppsList(appsParams),
    queryFn: async () => {
      const result = await listUserTunnelApps(appsParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load tunnel apps')
      }
      return result.data?.items ?? []
    },
    placeholderData: (previousData) => previousData,
  })

  const apps = useMemo(() => appsData ?? [], [appsData])
  useEffect(() => {
    let cancelled = false
    queueMicrotask(() => {
      if (cancelled) return
      const requestedAppId = search.tunnelSessionAppId
      if (requestedAppId && apps.some((app) => app.id === requestedAppId)) {
        if (selectedAppId !== requestedAppId) setSelectedAppId(requestedAppId)
        return
      }
      if (selectedAppId && apps.some((app) => app.id === selectedAppId)) return
      setSelectedAppId(apps[0]?.id)
    })
    return () => {
      cancelled = true
    }
  }, [apps, search.tunnelSessionAppId, selectedAppId])

  const selectedApp =
    apps.find((app) => app.id === selectedAppId) ?? apps[0] ?? null

  const handleSelectApp = (appId: number) => {
    setSelectedAppId(appId)
    void navigate({
      search: (prev) => ({
        ...prev,
        tunnelSessionAppId: appId,
        tunnelSessionConnectionId: undefined,
        tunnelSessionsPage: undefined,
      }),
    })
  }

  const connectionParams = {
    p: 1,
    page_size: 200,
  }
  const {
    data: connectionsData,
    error: connectionsError,
    isError: isConnectionsError,
    isLoading: isConnectionsLoading,
    isFetching: isConnectionsFetching,
    refetch: refetchConnections,
  } = useQuery({
    queryKey: selectedApp
      ? mcpQueryKeys.tunnelConnectionsList(selectedApp.id, connectionParams)
      : [...mcpQueryKeys.tunnelApps(), 'session-connections', 'none'],
    queryFn: async () => {
      if (!selectedApp) return []
      const result = await listTunnelConnections(
        selectedApp.id,
        connectionParams
      )
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load tunnel connections')
      }
      return result.data?.items ?? []
    },
    enabled: !!selectedApp,
    placeholderData: (previousData) => previousData,
  })

  const connections = useMemo(() => connectionsData ?? [], [connectionsData])
  const selectedConnectionId =
    typeof search.tunnelSessionConnectionId === 'number'
      ? search.tunnelSessionConnectionId
      : undefined

  useEffect(() => {
    if (!selectedConnectionId) return
    if (
      connections.some((connection) => connection.id === selectedConnectionId)
    ) {
      return
    }
    void navigate({
      replace: true,
      search: (prev) => ({
        ...prev,
        tunnelSessionConnectionId: undefined,
        tunnelSessionsPage: undefined,
      }),
    })
  }, [connections, navigate, selectedConnectionId])

  const handleSelectConnection = (connectionId?: number) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        tunnelSessionConnectionId: connectionId,
        tunnelSessionsPage: undefined,
      }),
    })
  }

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []

  const requestParams: TunnelSessionListParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    keyword: globalFilter,
    status: statusFilter[0],
    connection_id: selectedConnectionId,
  }

  const {
    data,
    error: sessionsError,
    isError: isSessionsError,
    isLoading: isSessionsLoading,
    isFetching: isSessionsFetching,
    refetch: refetchSessions,
  } = useQuery({
    queryKey: selectedApp
      ? mcpQueryKeys.tunnelSessionsList(selectedApp.id, requestParams)
      : [...mcpQueryKeys.tunnelApps(), 'sessions', 'none', requestParams],
    queryFn: async () => {
      if (!selectedApp) return { items: [], total: 0 }
      const result = await listTunnelSessions(selectedApp.id, requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load tunnel sessions')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    enabled: !!selectedApp,
    placeholderData: (previousData) => previousData,
  })

  useEffect(() => {
    if (!isAppsError) return
    toast.error(
      mcpQueryErrorMessage(appsError, t('Failed to load tunnel apps'))
    )
  }, [appsError, isAppsError, t])

  useEffect(() => {
    if (!isConnectionsError) return
    toast.error(
      mcpQueryErrorMessage(
        connectionsError,
        t('Failed to load tunnel connections')
      )
    )
  }, [connectionsError, isConnectionsError, t])

  useEffect(() => {
    if (!isSessionsError) return
    toast.error(
      mcpQueryErrorMessage(sessionsError, t('Failed to load tunnel sessions'))
    )
  }, [isSessionsError, sessionsError, t])

  const columns = useTunnelSessionColumns()
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

  const refreshAll = () => {
    void refetchApps()
    void refetchConnections()
    void refetchSessions()
  }

  const isLoading = isAppsLoading || isConnectionsLoading || isSessionsLoading
  const isFetching =
    isAppsFetching || isConnectionsFetching || isSessionsFetching

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t(
        apps.length === 0
          ? 'No Approved Tunnel Apps'
          : 'No Tunnel Sessions Found'
      )}
      emptyDescription={t(
        apps.length === 0
          ? 'Choose an approved MCP code tunnel app to inspect session state.'
          : 'Tunnel sessions will appear after a client initializes or opens an MCP stream.'
      )}
      skeletonKeyPrefix='tunnel-sessions-skeleton'
      toolbarProps={{
        searchPlaceholder: t('Filter sessions...'),
        additionalSearch: (
          <div className='flex flex-wrap items-center gap-2'>
            <TunnelAppSelector
              apps={apps}
              loading={isAppsLoading}
              selectedAppId={selectedApp?.id}
              onChange={handleSelectApp}
            />
            <TunnelConnectionSelector
              connections={connections}
              disabled={!selectedApp}
              loading={isConnectionsLoading}
              selectedConnectionId={selectedConnectionId}
              onChange={handleSelectConnection}
            />
          </div>
        ),
        filters: [
          {
            columnId: 'status',
            title: t('Status'),
            options: [
              { label: t('Online'), value: 'online' },
              { label: t('Offline'), value: 'offline' },
            ],
            singleSelect: true,
          },
        ],
        preActions: (
          <Button
            type='button'
            variant='outline'
            onClick={refreshAll}
            disabled={isFetching}
            className={cn(isFetching && 'opacity-80')}
          >
            <RefreshCw className={cn('size-4', isFetching && 'animate-spin')} />
            {t('Refresh')}
          </Button>
        ),
      }}
      afterTable={
        selectedApp ? (
          <div className='text-muted-foreground flex flex-wrap items-center gap-2 text-xs'>
            <span>{t('Selected Tunnel App')}</span>
            <StatusBadge
              label={selectedApp.name}
              autoColor={selectedApp.public_slug}
              copyable={false}
            />
            <span className='font-mono'>{selectedApp.public_slug}</span>
          </div>
        ) : null
      }
    />
  )
}
