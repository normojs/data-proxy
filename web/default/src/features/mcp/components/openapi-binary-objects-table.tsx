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
import {
  Code2,
  Download,
  HardDrive,
  MoreHorizontal,
  RefreshCw,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTableColumnHeader, DataTablePage } from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import {
  cleanupMCPOpenAPIBinaryObjects,
  getMCPOpenAPIBinaryObjectSummary,
  listMCPOpenAPIBinaryObjects,
  mcpQueryKeys,
} from '../api'
import { stringFilterValue } from '../lib/filter-values'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type {
  MCPOpenAPIBinaryCleanupResponse,
  MCPOpenAPIBinaryObject,
  MCPOpenAPIBinaryObjectListParams,
  MCPOpenAPIBinaryObjectSummary,
} from '../types'
import {
  IdCell,
  LongTextCell,
  SizeCell,
  TimestampCell,
  TraceCell,
} from './table-cells'
import { JsonDetailDialog } from './json-detail-dialog'

const route = getRouteApi('/_authenticated/mcp/$section')

type DetailState = {
  description?: string
  title: string
  value: unknown
}

const expiryStatusOptions = [
  { labelKey: 'Active', value: 'active' },
  { labelKey: 'Expired', value: 'expired' },
  { labelKey: 'No Expiry', value: 'no_expiry' },
] as const

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }
  const digits = unitIndex === 0 ? 0 : 2
  return `${size.toFixed(digits)} ${units[unitIndex]}`
}

function parsePositiveInteger(value: string): number | undefined {
  const parsed = Number(value)
  if (!Number.isFinite(parsed) || parsed <= 0) return undefined
  return Math.trunc(parsed)
}

function expiryStatusLabel(status: string): string {
  switch (status) {
    case 'active':
      return 'Active'
    case 'expired':
      return 'Expired'
    case 'no_expiry':
      return 'No Expiry'
    default:
      return 'Unknown'
  }
}

function expiryStatusVariant(status: string): StatusVariant {
  switch (status) {
    case 'active':
      return 'success'
    case 'expired':
      return 'warning'
    case 'no_expiry':
      return 'neutral'
    default:
      return 'neutral'
  }
}

function ExpiryStatusBadge(props: { status: string }) {
  const { t } = useTranslation()
  return (
    <StatusBadge
      label={t(expiryStatusLabel(props.status))}
      variant={expiryStatusVariant(props.status)}
      copyable={false}
    />
  )
}

function SummaryCell(props: { label: string; value: string }) {
  return (
    <div className='border-input bg-muted/20 min-w-0 rounded-md border px-3 py-2'>
      <div className='text-muted-foreground truncate text-xs'>
        {props.label}
      </div>
      <div className='truncate text-sm font-semibold'>{props.value}</div>
    </div>
  )
}

function BinaryObjectSummary(props: {
  isFetching: boolean
  summary?: MCPOpenAPIBinaryObjectSummary
}) {
  const { t } = useTranslation()
  const summary = props.summary

  return (
    <div className='grid gap-2 sm:grid-cols-3 lg:grid-cols-6'>
      <SummaryCell
        label={t('Objects')}
        value={(summary?.total_count ?? 0).toLocaleString()}
      />
      <SummaryCell
        label={t('Total Bytes')}
        value={formatBytes(summary?.total_bytes ?? 0)}
      />
      <SummaryCell
        label={t('Active')}
        value={(summary?.active_count ?? 0).toLocaleString()}
      />
      <SummaryCell
        label={t('Expired')}
        value={(summary?.expired_count ?? 0).toLocaleString()}
      />
      <SummaryCell
        label={t('Downloads')}
        value={(summary?.download_count ?? 0).toLocaleString()}
      />
      <SummaryCell
        label={t('Checked At')}
        value={
          props.isFetching ? t('Refreshing') : formatTimestampToDate(summary?.checked_at)
        }
      />
    </div>
  )
}

function BinaryObjectTraceCell(props: { object: MCPOpenAPIBinaryObject }) {
  const { t } = useTranslation()

  return (
    <TraceCell
      items={[
        { label: t('User'), value: props.object.user_id },
        { label: t('Token'), value: props.object.token_id },
        { label: t('Tool'), value: props.object.mcp_tool_id },
        { label: t('Call'), value: props.object.mcp_tool_call_id },
        { label: t('Request'), value: props.object.request_id },
        { label: t('Operation'), value: props.object.operation_key },
      ]}
    />
  )
}

function BinaryObjectDownloadCell(props: { object: MCPOpenAPIBinaryObject }) {
  const { t } = useTranslation()
  const lastDownloaded = formatTimestampToDate(props.object.last_downloaded_at)

  return (
    <div className='flex min-w-[120px] flex-col gap-1'>
      <span className='tabular-nums'>
        {props.object.download_count.toLocaleString()}
      </span>
      <span className='text-muted-foreground text-xs'>
        {t('Last')}: {lastDownloaded}
      </span>
    </div>
  )
}

function BinaryObjectActionsCell(props: {
  object: MCPOpenAPIBinaryObject
  onOpenDetail: (detail: DetailState) => void
}) {
  const { t } = useTranslation()

  const handleDownload = () => {
    if (!props.object.download_url) return
    window.open(props.object.download_url, '_blank', 'noopener,noreferrer')
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant='ghost' size='icon-sm' />}>
        <MoreHorizontal className='size-4' />
        <span className='sr-only'>{t('Open menu')}</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem onSelect={handleDownload}>
          <Download className='size-4' />
          {t('Download')}
        </DropdownMenuItem>
        <DropdownMenuItem
          onSelect={() =>
            props.onOpenDetail({
              title: t('Binary Object Detail'),
              description: props.object.object_id,
              value: props.object,
            })
          }
        >
          <Code2 className='size-4' />
          {t('Details')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function useBinaryObjectColumns(options: {
  onOpenDetail: (detail: DetailState) => void
}) {
  const { t } = useTranslation()

  return useMemo<ColumnDef<MCPOpenAPIBinaryObject>[]>(
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
        accessorKey: 'object_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Object ID')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.object_id}
            className='max-w-[220px] font-mono'
          />
        ),
        meta: { label: t('Object ID') },
      },
      {
        accessorKey: 'expiry_status',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Expiry')} />
        ),
        cell: ({ row }) => (
          <div className='flex min-w-[130px] flex-col gap-1'>
            <ExpiryStatusBadge status={row.original.expiry_status} />
            <span className='text-muted-foreground text-xs'>
              {formatTimestampToDate(row.original.expires_at)}
            </span>
          </div>
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Expiry') },
      },
      {
        accessorKey: 'provider',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Provider')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={row.original.provider || '-'}
            variant='info'
            copyable={false}
          />
        ),
        meta: { label: t('Provider') },
      },
      {
        accessorKey: 'content_type',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Content Type')} />
        ),
        cell: ({ row }) => (
          <div className='flex min-w-[180px] flex-col gap-1'>
            <LongText className='max-w-[220px] font-mono text-xs'>
              {row.original.content_type || '-'}
            </LongText>
            <StatusBadge
              label={row.original.content_family || '-'}
              variant='neutral'
              copyable={false}
            />
          </div>
        ),
        meta: { label: t('Content Type') },
      },
      {
        accessorKey: 'size',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Size')} />
        ),
        cell: ({ row }) => <SizeCell value={row.original.size} />,
        meta: { label: t('Size') },
      },
      {
        accessorKey: 'filename',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Filename')} />
        ),
        cell: ({ row }) => (
          <LongTextCell value={row.original.filename} className='max-w-[240px]' />
        ),
        meta: { label: t('Filename'), mobileHidden: true },
      },
      {
        id: 'audit',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Audit Context')} />
        ),
        cell: ({ row }) => <BinaryObjectTraceCell object={row.original} />,
        meta: { label: t('Audit Context') },
      },
      {
        accessorKey: 'download_count',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Downloads')} />
        ),
        cell: ({ row }) => <BinaryObjectDownloadCell object={row.original} />,
        meta: { label: t('Downloads'), mobileHidden: true },
      },
      {
        accessorKey: 'created_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Created At')} />
        ),
        cell: ({ row }) => <TimestampCell value={row.original.created_at} />,
        meta: { label: t('Created At'), mobileHidden: true },
      },
      {
        id: 'actions',
        cell: ({ row }) => (
          <BinaryObjectActionsCell
            object={row.original}
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

function CleanupResultSummary(props: {
  result: MCPOpenAPIBinaryCleanupResponse
}) {
  const { t } = useTranslation()

  return (
    <div className='border-input bg-muted/20 grid gap-2 rounded-md border p-3 sm:grid-cols-5'>
      <SummaryCell label={t('Scanned')} value={props.result.scanned.toString()} />
      <SummaryCell label={t('Deleted')} value={props.result.deleted.toString()} />
      <SummaryCell
        label={t('Deleted Bytes')}
        value={formatBytes(props.result.deleted_bytes)}
      />
      <SummaryCell
        label={t('Registry Deleted')}
        value={props.result.registry_deleted.toString()}
      />
      <SummaryCell
        label={t('Errors')}
        value={(props.result.errors?.length ?? 0).toString()}
      />
    </div>
  )
}

function cleanupPayload(ttlSeconds: string, limit: string, dryRun: boolean) {
  return {
    dry_run: dryRun,
    limit: parsePositiveInteger(limit),
    ttl_seconds: parsePositiveInteger(ttlSeconds),
  }
}

export function OpenAPIBinaryObjectsTable() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    filename: false,
  })
  const [detail, setDetail] = useState<DetailState | null>(null)
  const [cleanupResult, setCleanupResult] =
    useState<MCPOpenAPIBinaryCleanupResponse | null>(null)
  const [confirmCleanup, setConfirmCleanup] = useState(false)
  const [ttlSeconds, setTTLSeconds] = useState('')
  const [cleanupLimit, setCleanupLimit] = useState('')

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
      {
        columnId: 'expiry_status',
        searchKey: 'binaryExpiry',
        type: 'array',
      },
    ],
  })

  const expiryFilter =
    (columnFilters.find((filter) => filter.id === 'expiry_status')?.value as
      | string[]
      | undefined) ?? []

  const requestParams: MCPOpenAPIBinaryObjectListParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    keyword: stringFilterValue(globalFilter),
    expiry_status: expiryFilter[0],
  }
  const summaryParams: MCPOpenAPIBinaryObjectListParams = {
    keyword: requestParams.keyword,
    expiry_status: requestParams.expiry_status,
  }

  const {
    data,
    error: objectsError,
    isError: isObjectsError,
    isLoading,
    isFetching,
  } = useQuery({
    queryKey: mcpQueryKeys.openAPIBinaryObjectsList(requestParams),
    queryFn: async () => {
      const result = await listMCPOpenAPIBinaryObjects(requestParams)
      if (!result.success) {
        throw mcpQueryError(
          result.message,
          'Failed to load OpenAPI binary objects'
        )
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const {
    data: summary,
    error: summaryError,
    isError: isSummaryError,
    isFetching: isSummaryFetching,
  } = useQuery({
    queryKey: mcpQueryKeys.openAPIBinaryObjectsSummary(summaryParams),
    queryFn: async () => {
      const result = await getMCPOpenAPIBinaryObjectSummary(summaryParams)
      if (!result.success) {
        throw mcpQueryError(
          result.message,
          'Failed to load OpenAPI binary object summary'
        )
      }
      return result.data
    },
  })

  useEffect(() => {
    if (!isObjectsError) return
    toast.error(
      mcpQueryErrorMessage(
        objectsError,
        t('Failed to load OpenAPI binary objects')
      )
    )
  }, [isObjectsError, objectsError, t])

  useEffect(() => {
    if (!isSummaryError) return
    toast.error(
      mcpQueryErrorMessage(
        summaryError,
        t('Failed to load OpenAPI binary object summary')
      )
    )
  }, [isSummaryError, summaryError, t])

  useEffect(() => {
    if (!summary) return
    setTTLSeconds((current) => current || String(summary.default_ttl_seconds || ''))
    setCleanupLimit((current) =>
      current || String(summary.default_cleanup_limit || '')
    )
  }, [summary])

  const cleanupMutation = useMutation({
    mutationFn: (dryRun: boolean) =>
      cleanupMCPOpenAPIBinaryObjects(
        cleanupPayload(ttlSeconds, cleanupLimit, dryRun)
      ),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to clean OpenAPI binary objects'))
        return
      }
      setCleanupResult(res.data)
      const toastKey = res.data.dry_run
        ? 'OpenAPI binary cleanup dry-run completed'
        : 'OpenAPI binary cleanup completed'
      toast.success(t(toastKey))
      if (!res.data.dry_run) {
        queryClient.invalidateQueries({
          queryKey: mcpQueryKeys.openAPIBinaryObjects(),
        })
      }
      setConfirmCleanup(false)
    },
  })

  const columns = useBinaryObjectColumns({ onOpenDetail: setDetail })
  const objects = data?.items ?? []

  const table = useReactTable({
    data: objects,
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
      <div className='space-y-3'>
        <BinaryObjectSummary
          summary={summary}
          isFetching={isSummaryFetching}
        />
        {cleanupResult && <CleanupResultSummary result={cleanupResult} />}
        <DataTablePage
          table={table}
          columns={columns}
          isLoading={isLoading}
          isFetching={isFetching}
          emptyTitle={t('No OpenAPI Binary Objects Found')}
          emptyDescription={t(
            'No OpenAPI binary objects are currently retained.'
          )}
          emptyIcon={<HardDrive className='size-6' />}
          skeletonKeyPrefix='mcp-openapi-binary-objects-skeleton'
          toolbarProps={{
            searchPlaceholder: t('Filter by object, request, operation...'),
            filters: [
              {
                columnId: 'expiry_status',
                title: t('Expiry'),
                options: expiryStatusOptions.map((option) => ({
                  label: t(option.labelKey),
                  value: option.value,
                })),
                singleSelect: true,
              },
            ],
            leftActions: (
              <div className='flex flex-wrap items-center gap-2'>
                <Input
                  type='number'
                  min={1}
                  value={ttlSeconds}
                  onChange={(event) => setTTLSeconds(event.target.value)}
                  placeholder={t('TTL Seconds')}
                  className='w-[130px]'
                />
                <Input
                  type='number'
                  min={1}
                  value={cleanupLimit}
                  onChange={(event) => setCleanupLimit(event.target.value)}
                  placeholder={t('Cleanup Limit')}
                  className='w-[130px]'
                />
                <Button
                  type='button'
                  variant='outline'
                  onClick={() => cleanupMutation.mutate(true)}
                  disabled={cleanupMutation.isPending}
                  className={cn(cleanupMutation.isPending && 'opacity-80')}
                >
                  <RefreshCw
                    className={cn(
                      'size-4',
                      cleanupMutation.isPending && 'animate-spin'
                    )}
                  />
                  {t('Dry Run')}
                </Button>
                <Button
                  type='button'
                  variant='destructive'
                  onClick={() => setConfirmCleanup(true)}
                  disabled={cleanupMutation.isPending}
                >
                  <Trash2 className='size-4' />
                  {t('Cleanup')}
                </Button>
              </div>
            ),
          }}
          getRowClassName={(row) => {
            if (row.original.expiry_status === 'expired') {
              return 'opacity-70'
            }
            return undefined
          }}
        />
      </div>

      <JsonDetailDialog
        open={detail != null}
        title={detail?.title ?? t('Details')}
        description={detail?.description}
        value={detail?.value}
        onOpenChange={(open) => !open && setDetail(null)}
      />
      <ConfirmDialog
        open={confirmCleanup}
        onOpenChange={setConfirmCleanup}
        title={t('Cleanup OpenAPI Binary Objects')}
        desc={t(
          'This will remove expired OpenAPI binary objects from object storage and hide matching registry rows from the dashboard.'
        )}
        confirmText={t('Cleanup')}
        destructive
        isLoading={cleanupMutation.isPending}
        handleConfirm={() => cleanupMutation.mutate(false)}
      />
    </>
  )
}
