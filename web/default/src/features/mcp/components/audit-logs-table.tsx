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
import { Code2, FileText, MoreHorizontal } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useIsAdmin } from '@/hooks/use-admin'
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
import { listBridgeAuditLogs, mcpQueryKeys } from '../api'
import { getCallStatusOptions } from '../constants'
import { positiveIntFilterValue } from '../lib/filter-values'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type { BridgeAuditLog } from '../types'
import { FilterInput } from './filter-input'
import { JsonDetailDialog } from './json-detail-dialog'
import {
  CallStatusBadge,
  DurationCell,
  IdCell,
  LongTextCell,
  SizeCell,
  TimestampCell,
  TraceCell,
} from './table-cells'
import { TimeRangeFilter, timestampMsToSeconds } from './time-range-filter'

const route = getRouteApi('/_authenticated/mcp/$section')

type DetailState = {
  description?: string
  title: string
  value: unknown
}

function AuditLogActionsCell(props: {
  log: BridgeAuditLog
  onOpenDetail: (detail: DetailState) => void
}) {
  const { t } = useTranslation()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant='ghost' size='icon-sm' />}>
        <MoreHorizontal className='size-4' />
        <span className='sr-only'>{t('Open menu')}</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem
          onSelect={() =>
            props.onOpenDetail({
              title: t('Request Body'),
              description: props.log.request_id,
              value: props.log.request_body,
            })
          }
        >
          <Code2 className='size-4' />
          {t('Request Body')}
        </DropdownMenuItem>
        <DropdownMenuItem
          onSelect={() =>
            props.onOpenDetail({
              title: t('Audit Log Detail'),
              description: props.log.request_id,
              value: props.log,
            })
          }
        >
          <FileText className='size-4' />
          {t('Details')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function useAuditLogColumns(options: {
  onOpenDetail: (detail: DetailState) => void
}): ColumnDef<BridgeAuditLog>[] {
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
        accessorKey: 'status',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Status')} />
        ),
        cell: ({ row }) => <CallStatusBadge status={row.original.status} />,
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Status'), mobileBadge: true },
      },
      {
        accessorKey: 'tool_name',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Tool')} />
        ),
        cell: ({ row }) => {
          const log = row.original
          return (
            <div className='flex min-w-[180px] flex-col gap-1'>
              <LongText className='max-w-[180px] font-medium'>
                {log.tool_name}
              </LongText>
              <LongText className='text-muted-foreground max-w-[220px] font-mono text-xs'>
                {log.request_id}
              </LongText>
            </div>
          )
        },
        meta: { label: t('Tool'), mobileTitle: true },
      },
      {
        accessorKey: 'client_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Trace')} />
        ),
        cell: ({ row }) => (
          <TraceCell
            items={[
              { label: t('User'), value: row.original.user_id },
              { label: t('Token'), value: row.original.token_id },
              { label: t('Client'), value: row.original.client_id },
              { label: t('Session'), value: row.original.session_id },
            ]}
          />
        ),
        meta: { label: t('Trace') },
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
        accessorKey: 'request_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Request ID')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.request_id}
            className='max-w-[180px] font-mono'
          />
        ),
        meta: { label: t('Request ID'), mobileHidden: true },
      },
      {
        accessorKey: 'token_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Token')} />
        ),
        cell: ({ row }) => <IdCell value={row.original.token_id} />,
        meta: { label: t('Token'), mobileHidden: true },
      },
      {
        accessorKey: 'duration_ms',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Duration')} />
        ),
        cell: ({ row }) => <DurationCell value={row.original.duration_ms} />,
        meta: { label: t('Duration') },
      },
      {
        accessorKey: 'result_size',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Result Size')} />
        ),
        cell: ({ row }) => <SizeCell value={row.original.result_size} />,
        meta: { label: t('Result Size'), mobileHidden: true },
      },
      {
        accessorKey: 'error_message',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Error')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.error_message}
            className='max-w-[260px]'
          />
        ),
        meta: { label: t('Error'), mobileHidden: true },
      },
      {
        accessorKey: 'created_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Created At')} />
        ),
        cell: ({ row }) => <TimestampCell value={row.original.created_at} />,
        meta: { label: t('Created At') },
      },
      {
        id: 'actions',
        cell: ({ row }) => (
          <AuditLogActionsCell
            log={row.original}
            onOpenDetail={options.onOpenDetail}
          />
        ),
        enableSorting: false,
        enableHiding: false,
        meta: { label: t('Actions') },
      },
    ],
    [options.onOpenDetail, t]
  )
}

export function AuditLogsTable() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const [detail, setDetail] = useState<DetailState | null>(null)
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    request_id: false,
    token_id: false,
    session_id: false,
    error_message: false,
  })

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
      pageKey: 'auditPage',
      pageSizeKey: 'auditPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : 20,
    },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'auditStatus', type: 'array' },
      { columnId: 'tool_name', searchKey: 'toolName', type: 'string' },
      { columnId: 'request_id', searchKey: 'requestId', type: 'string' },
      { columnId: 'token_id', searchKey: 'tokenId', type: 'string' },
      { columnId: 'client_id', searchKey: 'clientId', type: 'string' },
      { columnId: 'session_id', searchKey: 'sessionId', type: 'string' },
    ],
  })

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []
  const toolNameFilter =
    (columnFilters.find((filter) => filter.id === 'tool_name')
      ?.value as string) ?? ''
  const requestIdFilter =
    (columnFilters.find((filter) => filter.id === 'request_id')
      ?.value as string) ?? ''
  const tokenIdFilter =
    (columnFilters.find((filter) => filter.id === 'token_id')
      ?.value as string) ?? ''
  const clientIdFilter =
    (columnFilters.find((filter) => filter.id === 'client_id')
      ?.value as string) ?? ''
  const sessionIdFilter =
    (columnFilters.find((filter) => filter.id === 'session_id')
      ?.value as string) ?? ''

  const requestParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    scope: isAdmin ? ('all' as const) : undefined,
    keyword: globalFilter,
    status: statusFilter[0],
    token_id: positiveIntFilterValue(tokenIdFilter),
    tool_name: toolNameFilter,
    request_id: requestIdFilter,
    client_id: clientIdFilter,
    session_id: sessionIdFilter,
    start_time: timestampMsToSeconds(search.auditStartTime),
    end_time: timestampMsToSeconds(search.auditEndTime),
  }

  const {
    data,
    error: auditLogsError,
    isError: isAuditLogsError,
    isLoading,
    isFetching,
  } = useQuery({
    queryKey: mcpQueryKeys.auditLogsList(requestParams),
    queryFn: async () => {
      const result = await listBridgeAuditLogs(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load bridge audit logs')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  useEffect(() => {
    if (!isAuditLogsError) return
    toast.error(
      mcpQueryErrorMessage(
        auditLogsError,
        t('Failed to load bridge audit logs')
      )
    )
  }, [auditLogsError, isAuditLogsError, t])

  const columns = useAuditLogColumns({ onOpenDetail: setDetail })
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
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No Bridge Audit Logs Found')}
        emptyDescription={t(
          'No bridge audit logs available. Remote tool requests will appear here after execution.'
        )}
        skeletonKeyPrefix='bridge-audit-logs-skeleton'
        tableClassName='overflow-x-auto'
        toolbarProps={{
          searchPlaceholder: t('Filter by keyword...'),
          filters: [
            {
              columnId: 'status',
              title: t('Status'),
              options: getCallStatusOptions(t),
              singleSelect: true,
            },
          ],
          additionalSearch: (
            <>
              <TimeRangeFilter
                pageKey='auditPage'
                startKey='auditStartTime'
                endKey='auditEndTime'
              />
              <FilterInput
                table={table}
                columnId='tool_name'
                placeholder={t('Tool name...')}
              />
              <FilterInput
                table={table}
                columnId='request_id'
                placeholder={t('Request ID...')}
              />
              <FilterInput
                table={table}
                columnId='token_id'
                placeholder={t('Token ID...')}
              />
              <FilterInput
                table={table}
                columnId='client_id'
                placeholder={t('Client ID...')}
              />
              <FilterInput
                table={table}
                columnId='session_id'
                placeholder={t('Session ID...')}
              />
            </>
          ),
          hasAdditionalFilters:
            search.auditStartTime != null || search.auditEndTime != null,
          onReset: () => {
            void navigate({
              search: (prev) => ({
                ...prev,
                auditPage: undefined,
                auditStartTime: undefined,
                auditEndTime: undefined,
              }),
            })
          },
        }}
      />
      <JsonDetailDialog
        open={detail != null}
        title={detail?.title ?? ''}
        description={detail?.description}
        value={detail?.value}
        onOpenChange={(open) => !open && setDetail(null)}
      />
    </>
  )
}
