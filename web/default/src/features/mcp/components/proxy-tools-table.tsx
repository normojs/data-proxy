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
import {
  Code2,
  CreditCard,
  Eye,
  History,
  MoreHorizontal,
  Pencil,
  RefreshCw,
  Wrench,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
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
import { listMCPProxyTools, mcpQueryKeys } from '../api'
import { getPriceUnitLabel, getProxyToolStatusOptions } from '../constants'
import { positiveIntFilterValue, stringFilterValue } from '../lib/filter-values'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type { MCPProxyTool } from '../types'
import { FilterInput } from './filter-input'
import { JsonDetailDialog } from './json-detail-dialog'
import { ProxyToolDetailPanel } from './proxy-tool-detail-panel'
import { ProxyToolEditDialog } from './proxy-tool-edit-dialog'
import {
  IdCell,
  LongTextCell,
  ProxyToolStatusBadge,
  TimestampCell,
} from './table-cells'

const route = getRouteApi('/_authenticated/mcp/$section')

function ProxyToolActionsCell(props: {
  tool: MCPProxyTool
  onEdit: (tool: MCPProxyTool) => void
  onOpenBilling: (tool: MCPProxyTool) => void
  onOpenCalls: (tool: MCPProxyTool) => void
  onOpenMCPTool: (tool: MCPProxyTool) => void
  onViewDetail: (tool: MCPProxyTool) => void
  onViewSchema: (tool: MCPProxyTool) => void
}) {
  const { t } = useTranslation()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant='ghost' size='icon-sm' />}>
        <MoreHorizontal className='size-4' />
        <span className='sr-only'>{t('Open menu')}</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem onSelect={() => props.onEdit(props.tool)}>
          <Pencil className='size-4' />
          {t('Edit')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onViewDetail(props.tool)}>
          <Eye className='size-4' />
          {t('View Detail')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onViewSchema(props.tool)}>
          <Code2 className='size-4' />
          {t('View Schema')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onOpenMCPTool(props.tool)}>
          <Wrench className='size-4' />
          {t('Open MCP Tool')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onOpenCalls(props.tool)}>
          <History className='size-4' />
          {t('Open Tool Calls')}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => props.onOpenBilling(props.tool)}>
          <CreditCard className='size-4' />
          {t('Open Billing Events')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function useProxyToolColumns(options: {
  onEdit: (tool: MCPProxyTool) => void
  onOpenBilling: (tool: MCPProxyTool) => void
  onOpenCalls: (tool: MCPProxyTool) => void
  onOpenMCPTool: (tool: MCPProxyTool) => void
  onViewDetail: (tool: MCPProxyTool) => void
  onViewSchema: (tool: MCPProxyTool) => void
}): ColumnDef<MCPProxyTool>[] {
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
        accessorKey: 'exposed_tool_name',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Tool')} />
        ),
        cell: ({ row }) => {
          const tool = row.original
          return (
            <div className='flex min-w-[240px] flex-col gap-1'>
              <LongText className='max-w-[220px] font-medium'>
                {tool.exposed_tool_name}
              </LongText>
              <div className='flex min-w-0 items-center gap-1.5'>
                <span className='text-muted-foreground shrink-0 text-[10px] font-medium'>
                  {t('Downstream')}
                </span>
                <LongText className='text-muted-foreground max-w-[180px] font-mono text-xs'>
                  {tool.downstream_tool_name}
                </LongText>
              </div>
            </div>
          )
        },
        enableHiding: false,
        meta: { label: t('Tool'), mobileTitle: true },
      },
      {
        accessorKey: 'status',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Status')} />
        ),
        cell: ({ row }) => (
          <ProxyToolStatusBadge status={row.original.status} />
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Status'), mobileBadge: true },
      },
      {
        accessorKey: 'proxy_server_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Server ID')} />
        ),
        cell: ({ row }) => <IdCell value={row.original.proxy_server_id} />,
        meta: { label: t('Server ID') },
      },
      {
        accessorKey: 'mcp_tool_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('MCP Tool ID')} />
        ),
        cell: ({ row }) => <IdCell value={row.original.mcp_tool_id} />,
        meta: { label: t('MCP Tool ID'), mobileHidden: true },
      },
      {
        accessorKey: 'price_per_call',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Price')} />
        ),
        cell: ({ row }) => (
          <div className='min-w-[120px] text-sm tabular-nums'>
            {row.original.price_per_call.toFixed(4)}
            <span className='text-muted-foreground ml-1 text-xs'>
              {t(getPriceUnitLabel(row.original.price_unit))}
            </span>
          </div>
        ),
        meta: { label: t('Price') },
      },
      {
        accessorKey: 'free_quota',
        header: ({ column }) => (
          <DataTableColumnHeader
            column={column}
            title={t('Daily Free Quota')}
          />
        ),
        cell: ({ row }) => (
          <span className='tabular-nums'>
            {row.original.free_quota.toLocaleString()}
          </span>
        ),
        meta: { label: t('Daily Free Quota') },
      },
      {
        accessorKey: 'schema_hash',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Schema Hash')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.schema_hash}
            className='max-w-[200px] font-mono'
          />
        ),
        meta: { label: t('Schema Hash'), mobileHidden: true },
      },
      {
        accessorKey: 'exposed_description',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Description')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={
              row.original.exposed_description ||
              row.original.downstream_description
            }
            className='max-w-[300px]'
          />
        ),
        meta: { label: t('Description'), mobileHidden: true },
      },
      {
        accessorKey: 'last_discovered_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Discovered At')} />
        ),
        cell: ({ row }) => (
          <TimestampCell value={row.original.last_discovered_at} />
        ),
        meta: { label: t('Discovered At'), mobileHidden: true },
      },
      {
        id: 'actions',
        cell: ({ row }) => (
          <ProxyToolActionsCell
            tool={row.original}
            onEdit={options.onEdit}
            onOpenBilling={options.onOpenBilling}
            onOpenCalls={options.onOpenCalls}
            onOpenMCPTool={options.onOpenMCPTool}
            onViewDetail={options.onViewDetail}
            onViewSchema={options.onViewSchema}
          />
        ),
        enableSorting: false,
        enableHiding: false,
        meta: { label: t('Actions') },
      },
    ],
    [
      options.onEdit,
      options.onOpenBilling,
      options.onOpenCalls,
      options.onOpenMCPTool,
      options.onViewDetail,
      options.onViewSchema,
      t,
    ]
  )
}

export function MCPProxyToolsTable() {
  const { t } = useTranslation()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    schema_hash: false,
  })
  const [editTool, setEditTool] = useState<MCPProxyTool | null>(null)
  const [schemaTool, setSchemaTool] = useState<MCPProxyTool | null>(null)

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
      pageKey: 'proxyToolsPage',
      pageSizeKey: 'proxyToolsPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : 20,
    },
    globalFilter: { enabled: true, key: 'proxyToolFilter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'proxyToolStatus', type: 'array' },
      {
        columnId: 'proxy_server_id',
        searchKey: 'proxyServerId',
        type: 'string',
        deserialize: (value) =>
          typeof value === 'number' ? String(value) : '',
        serialize: positiveIntFilterValue,
      },
      {
        columnId: 'schema_hash',
        searchKey: 'proxySchemaHash',
        type: 'string',
        serialize: stringFilterValue,
      },
    ],
  })

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []
  const proxyServerIdFilter = positiveIntFilterValue(
    columnFilters.find((filter) => filter.id === 'proxy_server_id')?.value
  )
  const schemaHashFilter = stringFilterValue(
    columnFilters.find((filter) => filter.id === 'schema_hash')?.value
  )

  const requestParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    keyword: globalFilter,
    status: statusFilter[0],
    proxy_server_id: proxyServerIdFilter,
    schema_hash: schemaHashFilter,
  }

  const {
    data,
    error: proxyToolsError,
    isError: isProxyToolsError,
    isLoading,
    isFetching,
    refetch,
  } = useQuery({
    queryKey: mcpQueryKeys.proxyToolsList(requestParams),
    queryFn: async () => {
      const result = await listMCPProxyTools(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load MCP proxy tools')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  useEffect(() => {
    if (!isProxyToolsError) return
    toast.error(
      mcpQueryErrorMessage(proxyToolsError, t('Failed to load MCP proxy tools'))
    )
  }, [isProxyToolsError, proxyToolsError, t])

  const columns = useProxyToolColumns({
    onEdit: setEditTool,
    onOpenBilling: (tool) => {
      void navigate({
        to: '/mcp/$section',
        params: { section: 'billing-events' },
        search: (prev) => ({
          ...prev,
          billingPage: undefined,
          filter: tool.exposed_tool_name,
          billingSourceKind: ['mcp_tool_call'],
          billingEventType: undefined,
          billingStatus: undefined,
          billingUsageKind: undefined,
          requestId: undefined,
          billingSource: undefined,
        }),
      })
    },
    onOpenCalls: (tool) => {
      void navigate({
        to: '/mcp/$section',
        params: { section: 'tool-calls' },
        search: (prev) => ({
          ...prev,
          callsPage: undefined,
          callStatus: undefined,
          requestId: undefined,
          sessionId: undefined,
          targetClient: undefined,
          toolName: tool.exposed_tool_name,
        }),
      })
    },
    onOpenMCPTool: (tool) => {
      void navigate({
        to: '/mcp/$section',
        params: { section: 'tools' },
        search: (prev) => ({
          ...prev,
          page: undefined,
          filter: undefined,
          toolStatus: undefined,
          source: ['mcp_proxy'],
          toolName: tool.exposed_tool_name,
        }),
      })
    },
    onViewDetail: (tool) => {
      void navigate({
        search: (prev) => ({
          ...prev,
          proxyToolId: tool.id,
        }),
      })
    },
    onViewSchema: setSchemaTool,
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
      {search.proxyToolId && (
        <ProxyToolDetailPanel
          proxyToolId={search.proxyToolId}
          onClose={() => {
            void navigate({
              search: (prev) => ({
                ...prev,
                proxyToolId: undefined,
              }),
            })
          }}
          onViewSchema={setSchemaTool}
        />
      )}
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No Proxy Tools Found')}
        emptyDescription={t(
          'No MCP proxy tools discovered. Run discovery on a proxy server first.'
        )}
        skeletonKeyPrefix='mcp-proxy-tools-skeleton'
        toolbarProps={{
          searchPlaceholder: t('Filter by exposed or downstream tool...'),
          filters: [
            {
              columnId: 'status',
              title: t('Status'),
              options: getProxyToolStatusOptions(t),
              singleSelect: true,
            },
          ],
          additionalSearch: (
            <>
              <FilterInput
                table={table}
                columnId='proxy_server_id'
                placeholder={t('Server ID...')}
              />
              <FilterInput
                table={table}
                columnId='schema_hash'
                placeholder={t('Schema Hash...')}
              />
            </>
          ),
          hasAdditionalFilters:
            search.proxyServerId != null || !!search.proxySchemaHash,
          preActions: (
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
          ),
        }}
      />

      {editTool && (
        <ProxyToolEditDialog
          key={editTool.id}
          open={editTool != null}
          tool={editTool}
          onOpenChange={(open) => !open && setEditTool(null)}
        />
      )}
      <JsonDetailDialog
        open={schemaTool != null}
        title={t('Downstream Input Schema')}
        description={schemaTool?.exposed_tool_name}
        value={schemaTool?.downstream_input_schema}
        onOpenChange={(open) => !open && setSchemaTool(null)}
      />
    </>
  )
}
