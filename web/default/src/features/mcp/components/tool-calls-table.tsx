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
import { Code2, CreditCard, FileText, MoreHorizontal } from 'lucide-react'
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
import { listMCPToolCalls, mcpQueryKeys } from '../api'
import { getCallStatusOptions } from '../constants'
import { positiveIntFilterValue } from '../lib/filter-values'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type { MCPToolCall } from '../types'
import { FilterInput } from './filter-input'
import { JsonDetailDialog } from './json-detail-dialog'
import {
  CallStatusBadge,
  DurationCell,
  IdCell,
  LongTextCell,
  SettlementCell,
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

function parseCallMetadata(call: MCPToolCall): Record<string, unknown> {
  if (!call.metadata) return {}
  try {
    const parsed = JSON.parse(call.metadata)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? parsed
      : {}
  } catch {
    return {}
  }
}

function metadataString(
  metadata: Record<string, unknown>,
  key: string
): string {
  const value = metadata[key]
  if (value == null || value === '') return ''
  return String(value)
}

function ToolCallTraceCell(props: { call: MCPToolCall }) {
  const { t } = useTranslation()
  const metadata = parseCallMetadata(props.call)

  return (
    <TraceCell
      items={[
        { label: t('User'), value: props.call.user_id },
        { label: t('Token'), value: props.call.token_id },
        { label: t('Client'), value: props.call.target_client },
        { label: t('Session'), value: props.call.bridge_session_id },
        {
          label: t('Proxy'),
          value: metadataString(metadata, 'proxy_server_namespace'),
        },
        {
          label: t('Downstream'),
          value: metadataString(metadata, 'downstream_tool_name'),
        },
        {
          label: t('Transport'),
          value: metadataString(metadata, 'transport'),
        },
        {
          label: t('Downstream Request'),
          value: metadataString(metadata, 'downstream_request_id'),
        },
      ]}
    />
  )
}

function ToolCallActionsCell(props: {
  call: MCPToolCall
  onOpenBilling: (call: MCPToolCall) => void
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
              title: t('Request Params'),
              description: props.call.request_id,
              value: props.call.request_params,
            })
          }
        >
          <Code2 className='size-4' />
          {t('Request Params')}
        </DropdownMenuItem>
        <DropdownMenuItem
          onSelect={() =>
            props.onOpenDetail({
              title: t('Call Metadata'),
              description: props.call.request_id,
              value: props.call.metadata || '{}',
            })
          }
        >
          <Code2 className='size-4' />
          {t('Call Metadata')}
        </DropdownMenuItem>
        <DropdownMenuItem
          onSelect={() =>
            props.onOpenDetail({
              title: t('Tool Call Detail'),
              description: props.call.request_id,
              value: props.call,
            })
          }
        >
          <FileText className='size-4' />
          {t('Details')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onOpenBilling(props.call)}>
          <CreditCard className='size-4' />
          {t('Open Billing Events')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function useToolCallColumns(options: {
  onOpenBilling: (call: MCPToolCall) => void
  onOpenDetail: (detail: DetailState) => void
}): ColumnDef<MCPToolCall>[] {
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
        cell: ({ row }) => (
          <div className='flex min-w-[180px] flex-col gap-1'>
            <LongText className='max-w-[180px] font-medium'>
              {row.original.tool_name}
            </LongText>
            <LongText className='text-muted-foreground max-w-[220px] font-mono text-xs'>
              {row.original.request_id}
            </LongText>
          </div>
        ),
        meta: { label: t('Tool'), mobileTitle: true },
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
        accessorKey: 'bridge_session_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Session')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.bridge_session_id}
            className='max-w-[180px] font-mono'
          />
        ),
        meta: { label: t('Session'), mobileHidden: true },
      },
      {
        accessorKey: 'target_client',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Trace')} />
        ),
        cell: ({ row }) => <ToolCallTraceCell call={row.original} />,
        meta: { label: t('Trace') },
      },
      {
        accessorKey: 'quota',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Billing')} />
        ),
        cell: ({ row }) => (
          <SettlementCell
            cost={row.original.cost}
            freeUsed={row.original.free_used}
            quota={row.original.quota}
            settledAt={row.original.settled_at}
          />
        ),
        meta: { label: t('Billing') },
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
          <DataTableColumnHeader column={column} title={t('Result')} />
        ),
        cell: ({ row }) => {
          const message =
            row.original.error_message || row.original.result_summary || ''
          return <LongTextCell value={message} className='max-w-[260px]' />
        },
        meta: { label: t('Result'), mobileHidden: true },
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
          <ToolCallActionsCell
            call={row.original}
            onOpenBilling={options.onOpenBilling}
            onOpenDetail={options.onOpenDetail}
          />
        ),
        enableSorting: false,
        enableHiding: false,
        meta: { label: t('Actions') },
      },
    ],
    [options.onOpenBilling, options.onOpenDetail, t]
  )
}

export function ToolCallsTable() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const [detail, setDetail] = useState<DetailState | null>(null)
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    request_id: false,
    token_id: false,
    bridge_session_id: false,
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
      pageKey: 'callsPage',
      pageSizeKey: 'callsPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : 20,
    },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'callStatus', type: 'array' },
      { columnId: 'tool_name', searchKey: 'toolName', type: 'string' },
      { columnId: 'request_id', searchKey: 'requestId', type: 'string' },
      { columnId: 'token_id', searchKey: 'tokenId', type: 'string' },
      { columnId: 'bridge_session_id', searchKey: 'sessionId', type: 'string' },
      { columnId: 'target_client', searchKey: 'targetClient', type: 'string' },
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
  const sessionIdFilter =
    (columnFilters.find((filter) => filter.id === 'bridge_session_id')
      ?.value as string) ?? ''
  const targetClientFilter =
    (columnFilters.find((filter) => filter.id === 'target_client')
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
    bridge_session_id: sessionIdFilter,
    target_client: targetClientFilter,
    start_time: timestampMsToSeconds(search.callsStartTime),
    end_time: timestampMsToSeconds(search.callsEndTime),
  }

  const {
    data,
    error: toolCallsError,
    isError: isToolCallsError,
    isLoading,
    isFetching,
  } = useQuery({
    queryKey: mcpQueryKeys.toolCallsList(requestParams),
    queryFn: async () => {
      const result = await listMCPToolCalls(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load tool calls')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  useEffect(() => {
    if (!isToolCallsError) return
    toast.error(
      mcpQueryErrorMessage(toolCallsError, t('Failed to load tool calls'))
    )
  }, [isToolCallsError, t, toolCallsError])

  const openBillingEvents = (call: MCPToolCall) => {
    void navigate({
      to: '/mcp/$section',
      params: { section: 'billing-events' },
      search: (prev) => ({
        ...prev,
        billingPage: undefined,
        billingStartTime: undefined,
        billingEndTime: undefined,
        billingSourceKind: ['mcp_tool_call'],
        billingEventType: undefined,
        billingStatus: undefined,
        billingUsageKind: undefined,
        billingSource: undefined,
        billingSourceId: String(call.id),
        filter: undefined,
        requestId: call.request_id,
      }),
    })
  }

  const columns = useToolCallColumns({
    onOpenBilling: openBillingEvents,
    onOpenDetail: setDetail,
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
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No Tool Calls Found')}
        emptyDescription={t(
          'No MCP tool calls available. Calls will appear here once clients invoke tools.'
        )}
        skeletonKeyPrefix='mcp-tool-calls-skeleton'
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
                pageKey='callsPage'
                startKey='callsStartTime'
                endKey='callsEndTime'
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
                columnId='bridge_session_id'
                placeholder={t('Session ID...')}
              />
              <FilterInput
                table={table}
                columnId='target_client'
                placeholder={t('Client ID...')}
              />
            </>
          ),
          hasAdditionalFilters:
            search.callsStartTime != null || search.callsEndTime != null,
          onReset: () => {
            void navigate({
              search: (prev) => ({
                ...prev,
                callsPage: undefined,
                callsStartTime: undefined,
                callsEndTime: undefined,
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
