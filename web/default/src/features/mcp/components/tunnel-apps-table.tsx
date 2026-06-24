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
import { useMutation, useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import {
  type ColumnDef,
  type VisibilityState,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { Check, MoreHorizontal, RefreshCw, ShieldX, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { DataTableColumnHeader, DataTablePage } from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { listTunnelApps, mcpQueryKeys, updateTunnelApp } from '../api'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type { TunnelApp } from '../types'
import { IdCell, LongTextCell, TimestampCell } from './table-cells'

const route = getRouteApi('/_authenticated/mcp/$section')

function tunnelStatusVariant(status: string): StatusBadgeProps['variant'] {
  if (status === 'approved') return 'green'
  if (status === 'pending') return 'yellow'
  if (status === 'rejected') return 'red'
  if (status === 'disabled' || status === 'archived') return 'grey'
  return 'neutral'
}

function tunnelLabel(value: string, t: (key: string) => string): string {
  const labels: Record<string, string> = {
    mcp_code: t('MCP Code Tunnel'),
    http_tunnel: t('HTTP Tunnel'),
    tcp_tunnel: t('TCP Tunnel'),
    read_only: t('Read Only'),
    write: t('Write'),
    exec_safe: t('Exec Safe'),
    exec_trusted: t('Exec Trusted'),
    traffic: t('Traffic'),
    pending: t('Pending'),
    approved: t('Approved'),
    rejected: t('Rejected'),
    disabled: t('Disabled'),
    archived: t('Archived'),
  }
  return labels[value] || value
}

function TunnelAppActionsCell(props: {
  app: TunnelApp
  onSetStatus: (app: TunnelApp, status: string) => void
}) {
  const { t } = useTranslation()
  const { app, onSetStatus } = props
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant='ghost' size='icon-sm' />}>
        <MoreHorizontal className='size-4' />
        <span className='sr-only'>{t('Open menu')}</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem onSelect={() => onSetStatus(app, 'approved')}>
          <Check className='size-4' />
          {t('Approve')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onSetStatus(app, 'rejected')}>
          <X className='size-4' />
          {t('Reject')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onSetStatus(app, 'disabled')}>
          <ShieldX className='size-4' />
          {t('Disable')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function useTunnelAppColumns(options: {
  onSetStatus: (app: TunnelApp, status: string) => void
}): ColumnDef<TunnelApp>[] {
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
          <DataTableColumnHeader column={column} title={t('App')} />
        ),
        cell: ({ row }) => {
          const app = row.original
          return (
            <div className='flex min-w-[220px] flex-col gap-1'>
              <div className='flex min-w-0 items-center gap-2'>
                <LongText className='max-w-[180px] font-medium'>
                  {app.name}
                </LongText>
                <StatusBadge
                  label={tunnelLabel(app.app_type, t)}
                  autoColor={app.app_type}
                  copyable={false}
                />
              </div>
              <LongText className='text-muted-foreground max-w-[240px] font-mono text-xs'>
                {app.public_slug}
              </LongText>
            </div>
          )
        },
        enableHiding: false,
        meta: { label: t('App'), mobileTitle: true },
      },
      {
        accessorKey: 'app_type',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Type')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={tunnelLabel(row.original.app_type, t)}
            autoColor={row.original.app_type}
            copyable={false}
          />
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Type'), mobileHidden: true },
      },
      {
        accessorKey: 'status',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Status')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={tunnelLabel(row.original.status, t)}
            variant={tunnelStatusVariant(row.original.status)}
            copyable={false}
          />
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Status'), mobileBadge: true },
      },
      {
        accessorKey: 'permission_mode',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Permission')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={tunnelLabel(row.original.permission_mode, t)}
            autoColor={row.original.permission_mode}
            copyable={false}
          />
        ),
        meta: { label: t('Permission') },
      },
      {
        accessorKey: 'target',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Target')} />
        ),
        cell: ({ row }) => {
          const app = row.original
          const target =
            app.app_type === 'mcp_code'
              ? app.target_path || '/mcp'
              : `${app.target_host || '127.0.0.1'}:${app.target_port}${app.target_path || '/'}`
          return <LongTextCell value={target} className='max-w-[260px]' />
        },
        meta: { label: t('Target') },
      },
      {
        accessorKey: 'bridge_client_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Bridge Client')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.bridge_client_id}
            className='max-w-[200px] font-mono'
          />
        ),
        meta: { label: t('Bridge Client'), mobileHidden: true },
      },
      {
        accessorKey: 'updated_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Updated At')} />
        ),
        cell: ({ row }) => <TimestampCell value={row.original.updated_at} />,
        meta: { label: t('Updated At'), mobileHidden: true },
      },
      {
        id: 'actions',
        cell: ({ row }) => (
          <TunnelAppActionsCell
            app={row.original}
            onSetStatus={options.onSetStatus}
          />
        ),
        enableSorting: false,
        enableHiding: false,
        meta: { label: t('Actions') },
      },
    ],
    [options.onSetStatus, t]
  )
}

export function TunnelAppsTable() {
  const { t } = useTranslation()
  const navigate = route.useNavigate()
  const search = route.useSearch()
  const isMobile = useMediaQuery('(max-width: 640px)')
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
      pageKey: 'tunnelAppsPage',
      pageSizeKey: 'tunnelAppsPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : 20,
    },
    globalFilter: { enabled: true, key: 'tunnelFilter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'tunnelStatus', type: 'array' },
      { columnId: 'app_type', searchKey: 'tunnelType', type: 'array' },
    ],
  })

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []
  const typeFilter =
    (columnFilters.find((filter) => filter.id === 'app_type')?.value as
      | string[]
      | undefined) ?? []

  const requestParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    keyword: globalFilter,
    status: statusFilter[0],
    app_type: typeFilter[0],
  }

  const {
    data,
    error: tunnelAppsError,
    isError: isTunnelAppsError,
    isLoading,
    isFetching,
    refetch,
  } = useQuery({
    queryKey: mcpQueryKeys.tunnelAppsList(requestParams),
    queryFn: async () => {
      const result = await listTunnelApps(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load tunnel apps')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const reviewMutation = useMutation({
    mutationFn: async ({ app, status }: { app: TunnelApp; status: string }) =>
      updateTunnelApp(app.id, {
        status,
        review_note: status,
      }),
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to update tunnel app'))
        return
      }
      toast.success(t('Tunnel app updated'))
      void refetch()
    },
    onError: (error) => {
      toast.error(
        mcpQueryErrorMessage(error, t('Failed to update tunnel app'))
      )
    },
  })

  useEffect(() => {
    if (!isTunnelAppsError) return
    toast.error(
      mcpQueryErrorMessage(tunnelAppsError, t('Failed to load tunnel apps'))
    )
  }, [isTunnelAppsError, t, tunnelAppsError])

  const columns = useTunnelAppColumns({
    onSetStatus: (app, status) => reviewMutation.mutate({ app, status }),
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
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching || reviewMutation.isPending}
      emptyTitle={t('No Tunnel Apps Found')}
      emptyDescription={t(
        'Tunnel apps will appear here after users request MCP code or traffic tunnels.'
      )}
      skeletonKeyPrefix='tunnel-apps-skeleton'
      toolbarProps={{
        searchPlaceholder: t('Filter by tunnel name, slug or bridge client...'),
        filters: [
          {
            columnId: 'app_type',
            title: t('Type'),
            options: [
              { label: t('MCP Code Tunnel'), value: 'mcp_code' },
              { label: t('HTTP Tunnel'), value: 'http_tunnel' },
              { label: t('TCP Tunnel'), value: 'tcp_tunnel' },
            ],
            singleSelect: true,
          },
          {
            columnId: 'status',
            title: t('Status'),
            options: [
              { label: t('Pending'), value: 'pending' },
              { label: t('Approved'), value: 'approved' },
              { label: t('Rejected'), value: 'rejected' },
              { label: t('Disabled'), value: 'disabled' },
              { label: t('Archived'), value: 'archived' },
            ],
            singleSelect: true,
          },
        ],
        preActions: (
          <Button
            type='button'
            variant='outline'
            onClick={() => void refetch()}
            disabled={isFetching}
            className={cn(isFetching && 'opacity-80')}
          >
            <RefreshCw className={cn('size-4', isFetching && 'animate-spin')} />
            {t('Refresh')}
          </Button>
        ),
      }}
    />
  )
}
