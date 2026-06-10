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
  Archive,
  Code2,
  FileJson,
  MoreHorizontal,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTableColumnHeader,
  DataTablePage,
} from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { StatusBadge } from '@/components/status-badge'
import {
  archiveMCPTool,
  deleteMCPTool,
  deleteMCPOpenAPI,
  disableMCPOpenAPI,
  listMCPTools,
  mcpQueryKeys,
  seedMCPTools,
} from '../api'
import {
  MCP_TOOL_STATUS,
  getPriceUnitLabel,
  getToolSourceOptions,
  getToolStatusOptions,
} from '../constants'
import { stringFilterValue } from '../lib/filter-values'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type { MCPTool } from '../types'
import { JsonDetailDialog } from './json-detail-dialog'
import { OpenAPIImportDialog } from './openapi-import-dialog'
import {
  IdCell,
  LongTextCell,
  TimestampCell,
  ToolSourceBadge,
  ToolStatusBadge,
} from './table-cells'
import { ToolEditDialog } from './tool-edit-dialog'

const route = getRouteApi('/_authenticated/mcp/$section')

function useCanSeedMCPTools() {
  return useAuthStore(
    (state) => (state.auth.user?.role ?? 0) >= ROLE.SUPER_ADMIN
  )
}

function useCanManageMCPTools() {
  return useAuthStore((state) => (state.auth.user?.role ?? 0) >= ROLE.ADMIN)
}

function ToolActionsCell(props: {
  canManage: boolean
  tool: MCPTool
  onArchive: (tool: MCPTool) => void
  onDelete: (tool: MCPTool) => void
  onDeleteOpenAPI: (tool: MCPTool) => void
  onDisableOpenAPI: (tool: MCPTool) => void
  onEdit: (tool: MCPTool) => void
  onViewSchema: (tool: MCPTool) => void
}) {
  const { t } = useTranslation()
  const canManageCustomTool = props.canManage && props.tool.source === 'custom'
  const canManageOpenAPITool =
    props.canManage && props.tool.source === 'openapi'

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
        <DropdownMenuItem onSelect={() => props.onViewSchema(props.tool)}>
          <Code2 className='size-4' />
          {t('View Schema')}
        </DropdownMenuItem>
        {canManageCustomTool && (
          <>
            <DropdownMenuSeparator />
            {props.tool.status === MCP_TOOL_STATUS.ENABLED && (
              <DropdownMenuItem onSelect={() => props.onArchive(props.tool)}>
                <Archive className='size-4' />
                {t('Archive')}
              </DropdownMenuItem>
            )}
            <DropdownMenuItem
              variant='destructive'
              onSelect={() => props.onDelete(props.tool)}
            >
              <Trash2 className='size-4' />
              {t('Delete')}
            </DropdownMenuItem>
          </>
        )}
        {canManageOpenAPITool && (
          <>
            <DropdownMenuSeparator />
            {props.tool.status === MCP_TOOL_STATUS.ENABLED && (
              <DropdownMenuItem
                onSelect={() => props.onDisableOpenAPI(props.tool)}
              >
                <Archive className='size-4' />
                {t('Disable OpenAPI Import')}
              </DropdownMenuItem>
            )}
            <DropdownMenuItem
              variant='destructive'
              onSelect={() => props.onDeleteOpenAPI(props.tool)}
            >
              <Trash2 className='size-4' />
              {t('Delete OpenAPI Import')}
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function useMCPToolColumns(options: {
  canManage: boolean
  onArchive: (tool: MCPTool) => void
  onDelete: (tool: MCPTool) => void
  onDeleteOpenAPI: (tool: MCPTool) => void
  onDisableOpenAPI: (tool: MCPTool) => void
  onEdit: (tool: MCPTool) => void
  onViewSchema: (tool: MCPTool) => void
}): ColumnDef<MCPTool>[] {
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
          <DataTableColumnHeader column={column} title={t('Tool')} />
        ),
        cell: ({ row }) => {
          const tool = row.original
          const title = tool.display_name || tool.name
          return (
            <div className='flex min-w-[200px] flex-col gap-1'>
              <div className='flex min-w-0 items-center gap-2'>
                <LongText className='max-w-[180px] font-medium'>
                  {title}
                </LongText>
                {tool.is_remote && (
                  <StatusBadge
                    label={t('Remote')}
                    variant='purple'
                    copyable={false}
                  />
                )}
              </div>
              <LongText className='text-muted-foreground max-w-[240px] font-mono text-xs'>
                {tool.name}
              </LongText>
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
        cell: ({ row }) => <ToolStatusBadge status={row.original.status} />,
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Status'), mobileBadge: true },
      },
      {
        accessorKey: 'category',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Category')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={row.original.category || '-'}
            autoColor={row.original.category || undefined}
            copyable={false}
          />
        ),
        meta: { label: t('Category') },
      },
      {
        accessorKey: 'source',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Source')} />
        ),
        cell: ({ row }) => <ToolSourceBadge source={row.original.source} />,
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Source') },
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
        accessorKey: 'description',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Description')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.description}
            className='max-w-[300px]'
          />
        ),
        meta: { label: t('Description'), mobileHidden: true },
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
          <ToolActionsCell
            canManage={options.canManage}
            tool={row.original}
            onArchive={options.onArchive}
            onDelete={options.onDelete}
            onDeleteOpenAPI={options.onDeleteOpenAPI}
            onDisableOpenAPI={options.onDisableOpenAPI}
            onEdit={options.onEdit}
            onViewSchema={options.onViewSchema}
          />
        ),
        enableSorting: false,
        enableHiding: false,
        meta: { label: t('Actions') },
      },
    ],
    [
      options.canManage,
      options.onArchive,
      options.onDelete,
      options.onDeleteOpenAPI,
      options.onDisableOpenAPI,
      options.onEdit,
      options.onViewSchema,
      t,
    ]
  )
}

export function MCPToolsTable() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const canSeed = useCanSeedMCPTools()
  const canManage = useCanManageMCPTools()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    description: false,
  })
  const [createOpen, setCreateOpen] = useState(false)
  const [openAPIImportOpen, setOpenAPIImportOpen] = useState(false)
  const [archiveTool, setArchiveTool] = useState<MCPTool | null>(null)
  const [deleteTool, setDeleteTool] = useState<MCPTool | null>(null)
  const [disableOpenAPITool, setDisableOpenAPITool] = useState<MCPTool | null>(
    null
  )
  const [deleteOpenAPITool, setDeleteOpenAPITool] = useState<MCPTool | null>(
    null
  )
  const [editTool, setEditTool] = useState<MCPTool | null>(null)
  const [schemaTool, setSchemaTool] = useState<MCPTool | null>(null)

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
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : 20,
    },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'toolStatus', type: 'array' },
      { columnId: 'source', searchKey: 'source', type: 'array' },
      {
        columnId: 'name',
        searchKey: 'toolName',
        type: 'string',
        serialize: stringFilterValue,
      },
    ],
  })

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []
  const sourceFilter =
    (columnFilters.find((filter) => filter.id === 'source')?.value as
      | string[]
      | undefined) ?? []
  const toolNameFilter = stringFilterValue(
    columnFilters.find((filter) => filter.id === 'name')?.value
  )

  const requestParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    keyword: toolNameFilter || globalFilter,
    status: statusFilter[0],
    source: sourceFilter[0],
  }

  const {
    data,
    error: toolsError,
    isError: isToolsError,
    isLoading,
    isFetching,
  } = useQuery({
    queryKey: mcpQueryKeys.toolsList(requestParams),
    queryFn: async () => {
      const result = await listMCPTools(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load MCP tools')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  useEffect(() => {
    if (!isToolsError) return
    toast.error(mcpQueryErrorMessage(toolsError, t('Failed to load MCP tools')))
  }, [isToolsError, t, toolsError])

  const seedMutation = useMutation({
    mutationFn: seedMCPTools,
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to seed MCP tools'))
        return
      }
      toast.success(t('MCP tools seeded successfully'))
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.tools() })
    },
  })

  const archiveMutation = useMutation({
    mutationFn: archiveMCPTool,
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to archive MCP tool'))
        return
      }
      toast.success(t('MCP tool archived successfully'))
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.tools() })
      setArchiveTool(null)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteMCPTool,
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to delete MCP tool'))
        return
      }
      toast.success(t('MCP tool deleted successfully'))
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.tools() })
      setDeleteTool(null)
    },
  })

  const disableOpenAPIMutation = useMutation({
    mutationFn: disableMCPOpenAPI,
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to disable OpenAPI tools'))
        return
      }
      toast.success(
        t('OpenAPI tools disabled: {{count}}', {
          count: res.data?.affected_count ?? 0,
        })
      )
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.tools() })
      setDisableOpenAPITool(null)
    },
  })

  const deleteOpenAPIMutation = useMutation({
    mutationFn: deleteMCPOpenAPI,
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to delete OpenAPI tools'))
        return
      }
      toast.success(
        t('OpenAPI tools deleted: {{count}}', {
          count: res.data?.affected_count ?? 0,
        })
      )
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.tools() })
      setDeleteOpenAPITool(null)
    },
  })

  const columns = useMCPToolColumns({
    canManage,
    onArchive: setArchiveTool,
    onDelete: setDeleteTool,
    onDeleteOpenAPI: setDeleteOpenAPITool,
    onDisableOpenAPI: setDisableOpenAPITool,
    onEdit: setEditTool,
    onViewSchema: setSchemaTool,
  })
  const tools = data?.items ?? []

  const table = useReactTable({
    data: tools,
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

  const openAPILifecyclePayload = (tool: MCPTool) =>
    tool.openapi_url
      ? { openapi_url: tool.openapi_url }
      : { tool_ids: [tool.id] }

  return (
    <>
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No MCP Tools Found')}
        emptyDescription={t(
          'No MCP tools available. Seed built-in tools or adjust your filters.'
        )}
        skeletonKeyPrefix='mcp-tools-skeleton'
        toolbarProps={{
          searchPlaceholder: t('Filter by tool name or description...'),
          filters: [
            {
              columnId: 'status',
              title: t('Status'),
              options: getToolStatusOptions(t),
              singleSelect: true,
            },
            {
              columnId: 'source',
              title: t('Source'),
              options: getToolSourceOptions(t),
              singleSelect: true,
            },
          ],
          preActions: canSeed ? (
            <div className='flex flex-wrap items-center gap-2'>
              {canManage && (
                <>
                  <Button type='button' onClick={() => setCreateOpen(true)}>
                    <Plus className='size-4' />
                    {t('Create Tool')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => setOpenAPIImportOpen(true)}
                  >
                    <FileJson className='size-4' />
                    {t('Import OpenAPI')}
                  </Button>
                </>
              )}
              <Button
                type='button'
                variant='outline'
                onClick={() => seedMutation.mutate()}
                disabled={seedMutation.isPending}
                className={cn(seedMutation.isPending && 'opacity-80')}
              >
                <RefreshCw
                  className={cn(
                    'size-4',
                    seedMutation.isPending && 'animate-spin'
                  )}
                />
                {t('Seed Tools')}
              </Button>
            </div>
          ) : canManage ? (
            <div className='flex flex-wrap items-center gap-2'>
              <Button type='button' onClick={() => setCreateOpen(true)}>
                <Plus className='size-4' />
                {t('Create Tool')}
              </Button>
              <Button
                type='button'
                variant='outline'
                onClick={() => setOpenAPIImportOpen(true)}
              >
                <FileJson className='size-4' />
                {t('Import OpenAPI')}
              </Button>
            </div>
          ) : null,
        }}
        getRowClassName={(row, ctx) =>
          row.original.status === MCP_TOOL_STATUS.DISABLED
            ? ctx.isMobile
              ? DISABLED_ROW_MOBILE
              : DISABLED_ROW_DESKTOP
            : undefined
        }
      />

      <ToolEditDialog
        open={createOpen}
        tool={null}
        onOpenChange={setCreateOpen}
      />
      <OpenAPIImportDialog
        open={openAPIImportOpen}
        onOpenChange={setOpenAPIImportOpen}
      />
      <ToolEditDialog
        open={editTool != null}
        tool={editTool}
        onOpenChange={(open) => !open && setEditTool(null)}
      />
      <JsonDetailDialog
        open={schemaTool != null}
        title={t('Input Schema')}
        description={schemaTool?.name}
        value={schemaTool?.input_schema}
        onOpenChange={(open) => !open && setSchemaTool(null)}
      />
      <ConfirmDialog
        open={archiveTool != null}
        onOpenChange={(open) => !open && setArchiveTool(null)}
        title={t('Archive MCP Tool')}
        desc={t(
          'This will disable the custom tool while keeping its configuration and call history.'
        )}
        confirmText={t('Archive')}
        isLoading={archiveMutation.isPending}
        handleConfirm={() => {
          if (archiveTool) {
            archiveMutation.mutate(archiveTool.id)
          }
        }}
      />
      <ConfirmDialog
        open={deleteTool != null}
        onOpenChange={(open) => !open && setDeleteTool(null)}
        title={t('Delete MCP Tool')}
        desc={t(
          'This will soft delete the custom tool. Existing calls and billing events remain for audit.'
        )}
        confirmText={t('Delete')}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (deleteTool) {
            deleteMutation.mutate(deleteTool.id)
          }
        }}
      />
      <ConfirmDialog
        open={disableOpenAPITool != null}
        onOpenChange={(open) => !open && setDisableOpenAPITool(null)}
        title={t('Disable OpenAPI Import')}
        desc={t(
          'This will disable all OpenAPI tools imported from the same OpenAPI URL while keeping mappings and call history.'
        )}
        confirmText={t('Disable')}
        isLoading={disableOpenAPIMutation.isPending}
        handleConfirm={() => {
          if (disableOpenAPITool) {
            disableOpenAPIMutation.mutate(
              openAPILifecyclePayload(disableOpenAPITool)
            )
          }
        }}
      />
      <ConfirmDialog
        open={deleteOpenAPITool != null}
        onOpenChange={(open) => !open && setDeleteOpenAPITool(null)}
        title={t('Delete OpenAPI Import')}
        desc={t(
          'This will soft delete all OpenAPI tools imported from the same OpenAPI URL. Existing calls and billing events remain for audit.'
        )}
        confirmText={t('Delete')}
        destructive
        isLoading={deleteOpenAPIMutation.isPending}
        handleConfirm={() => {
          if (deleteOpenAPITool) {
            deleteOpenAPIMutation.mutate(
              openAPILifecyclePayload(deleteOpenAPITool)
            )
          }
        }}
      />
    </>
  )
}
