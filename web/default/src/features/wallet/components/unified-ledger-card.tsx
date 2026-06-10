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
  FileText,
  History,
  MoreHorizontal,
  ReceiptText,
  RefreshCw,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatNumber, formatQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { TitledCard } from '@/components/ui/titled-card'
import { DataTableColumnHeader, DataTablePage } from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { JsonDetailDialog } from '@/features/mcp/components/json-detail-dialog'
import { getUserBillingEvents } from '../api'
import type { BillingEvent } from '../types'

const route = getRouteApi('/_authenticated/wallet/')

type DetailState = {
  description?: string
  title: string
  value: unknown
}

const ledgerEventSourceOptions = [
  { labelKey: 'MCP Tool Call', value: 'mcp_tool_call' },
  { labelKey: 'Model Request', value: 'model_request' },
  { labelKey: 'Async Task', value: 'async_task' },
  { labelKey: 'Violation Fee', value: 'violation_fee' },
  { labelKey: 'Wallet Top-up', value: 'wallet_topup' },
  { labelKey: 'Wallet Adjust', value: 'wallet_adjust' },
  { labelKey: 'Subscription', value: 'subscription' },
  { labelKey: 'Billing Event Repair', value: 'billing_event_repair' },
] as const

const ledgerEventTypeOptions = [
  { labelKey: 'Debit', value: 'debit' },
  { labelKey: 'Credit', value: 'credit' },
  { labelKey: 'Audit', value: 'audit' },
] as const

const ledgerBillingSourceOptions = [
  { labelKey: 'Wallet', value: 'wallet' },
  { labelKey: 'Subscription', value: 'subscription' },
] as const

const ledgerUsageKindOptions = [
  { labelKey: 'Text', value: 'text' },
  { labelKey: 'Audio', value: 'audio' },
  { labelKey: 'Realtime', value: 'realtime' },
  { labelKey: 'Midjourney', value: 'midjourney' },
] as const

function optionLabel(
  options: readonly { labelKey: string; value: string }[],
  value: string
) {
  return options.find((option) => option.value === value)?.labelKey ?? value
}

function parseMetadata(metadata: string | undefined): Record<string, unknown> {
  if (!metadata) return {}
  try {
    const parsed = JSON.parse(metadata)
    return parsed && typeof parsed === 'object'
      ? (parsed as Record<string, unknown>)
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

function eventTypeVariant(type: string): StatusVariant {
  switch (type) {
    case 'credit':
      return 'success'
    case 'debit':
      return 'warning'
    case 'audit':
      return 'info'
    default:
      return 'neutral'
  }
}

function statusVariant(status: string): StatusVariant {
  switch (status) {
    case 'settled':
    case 'success':
      return 'success'
    case 'failed':
    case 'error':
    case 'timeout':
      return 'warning'
    default:
      return 'neutral'
  }
}

function signedQuotaDelta(event: BillingEvent) {
  if (event.quota_delta !== 0) return event.quota_delta
  if (event.event_type === 'debit') return -Math.abs(event.amount_quota)
  if (event.event_type === 'credit') return Math.abs(event.amount_quota)
  return event.amount_quota
}

function ledgerSummary(event: BillingEvent, metadata: Record<string, unknown>) {
  if (event.related_mcp_tool_call) {
    const call = event.related_mcp_tool_call
    return {
      title: call.tool_name || event.source_id,
      detail: [
        call.request_id && `req ${call.request_id}`,
        call.duration_ms > 0 && `${formatNumber(call.duration_ms)}ms`,
        call.target_client && `client ${call.target_client}`,
      ]
        .filter(Boolean)
        .join(' · '),
    }
  }

  const planTitle = metadataString(metadata, 'subscription_plan_title')
  const subscriptionId = metadataString(metadata, 'subscription_id')
  if (event.source === 'subscription' || subscriptionId) {
    return {
      title:
        planTitle ||
        (subscriptionId ? `Subscription #${subscriptionId}` : event.source_id),
      detail: [
        metadataString(metadata, 'subscription_from'),
        metadataString(metadata, 'quota_from'),
      ]
        .filter(Boolean)
        .join(' · '),
    }
  }

  if (event.source === 'model_request') {
    return {
      title:
        metadataString(metadata, 'model') ||
        metadataString(metadata, 'request_path') ||
        event.request_id ||
        event.source_id,
      detail: [
        metadataString(metadata, 'usage_kind'),
        metadataString(metadata, 'billing_preference'),
        metadataString(metadata, 'channel_id') &&
          `channel ${metadataString(metadata, 'channel_id')}`,
      ]
        .filter(Boolean)
        .join(' · '),
    }
  }

  return {
    title: event.source_id || event.event_id,
    detail: [
      event.request_id && `req ${event.request_id}`,
      metadataString(metadata, 'payment_method'),
      metadataString(metadata, 'reason'),
    ]
      .filter(Boolean)
      .join(' · '),
  }
}

function secondsToLocalInput(value?: number) {
  if (!value) return ''
  const date = new Date(value * 1000)
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}

function localInputToSeconds(value: string) {
  if (!value) return undefined
  const ms = new Date(value).getTime()
  return Number.isFinite(ms) ? Math.floor(ms / 1000) : undefined
}

function positiveIntValue(value: string) {
  const parsed = Number.parseInt(value, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
}

function LedgerTimeRangeFilter() {
  const { t } = useTranslation()
  const search = route.useSearch()
  const navigate = route.useNavigate()

  return (
    <div className='flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:items-center'>
      <div className='grid gap-1'>
        <Label className='text-muted-foreground text-xs'>
          {t('Start Time')}
        </Label>
        <Input
          type='datetime-local'
          value={secondsToLocalInput(search.ledgerStartTime)}
          onChange={(event) => {
            const next = localInputToSeconds(event.target.value)
            void navigate({
              search: (prev) => ({
                ...prev,
                ledgerPage: undefined,
                ledgerStartTime: next,
              }),
            })
          }}
          className='w-full sm:w-[190px]'
        />
      </div>
      <div className='grid gap-1'>
        <Label className='text-muted-foreground text-xs'>{t('End Time')}</Label>
        <Input
          type='datetime-local'
          value={secondsToLocalInput(search.ledgerEndTime)}
          onChange={(event) => {
            const next = localInputToSeconds(event.target.value)
            void navigate({
              search: (prev) => ({
                ...prev,
                ledgerPage: undefined,
                ledgerEndTime: next,
              }),
            })
          }}
          className='w-full sm:w-[190px]'
        />
      </div>
    </div>
  )
}

function LedgerActionsCell(props: {
  event: BillingEvent
  onOpenMetadata: (event: BillingEvent) => void
  onOpenMCPCall: (event: BillingEvent) => void
}) {
  const { t } = useTranslation()
  const hasMCPCall = props.event.related_mcp_tool_call != null

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant='ghost'
            size='icon'
            className='size-8'
            aria-label={t('Open menu')}
          />
        }
      >
        <MoreHorizontal className='size-4' />
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem onSelect={() => props.onOpenMetadata(props.event)}>
          <FileText className='size-4' />
          {t('Metadata')}
        </DropdownMenuItem>
        {hasMCPCall && (
          <DropdownMenuItem onSelect={() => props.onOpenMCPCall(props.event)}>
            <History className='size-4' />
            {t('MCP Tool Call')}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function useLedgerColumns(options: {
  onOpenMetadata: (event: BillingEvent) => void
  onOpenMCPCall: (event: BillingEvent) => void
}): ColumnDef<BillingEvent>[] {
  const { t } = useTranslation()

  return useMemo(
    () => [
      {
        accessorKey: 'source',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Source')} />
        ),
        cell: ({ row }) => {
          const event = row.original
          return (
            <div className='min-w-0 space-y-1'>
              <div className='flex min-w-0 flex-wrap items-center gap-1.5'>
                <StatusBadge
                  label={t(optionLabel(ledgerEventSourceOptions, event.source))}
                  autoColor={event.source}
                  copyable={false}
                />
                <StatusBadge
                  label={t(
                    optionLabel(ledgerEventTypeOptions, event.event_type)
                  )}
                  variant={eventTypeVariant(event.event_type)}
                  copyable={false}
                />
              </div>
              <LongText className='max-w-[260px] font-mono text-xs'>
                {event.source_id || event.event_id}
              </LongText>
            </div>
          )
        },
        meta: { label: t('Source'), mobileTitle: true },
      },
      {
        accessorKey: 'billing_source',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Funding')} />
        ),
        cell: ({ row }) => {
          const event = row.original
          const metadata = parseMetadata(event.metadata)
          const usageKind = metadataString(metadata, 'usage_kind')
          const subscriptionId = metadataString(metadata, 'subscription_id')
          return (
            <div className='flex min-w-0 flex-wrap items-center gap-1.5'>
              <StatusBadge
                label={t(
                  optionLabel(
                    ledgerBillingSourceOptions,
                    event.billing_source || 'wallet'
                  )
                )}
                autoColor={event.billing_source || 'wallet'}
                copyable={false}
              />
              {usageKind && (
                <StatusBadge
                  label={t(optionLabel(ledgerUsageKindOptions, usageKind))}
                  autoColor={usageKind}
                  copyable={false}
                />
              )}
              {subscriptionId && (
                <StatusBadge
                  label={`${t('Subscription')} #${subscriptionId}`}
                  variant='info'
                  copyable={false}
                />
              )}
            </div>
          )
        },
        meta: { label: t('Funding'), mobileBadge: true },
      },
      {
        id: 'summary',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Summary')} />
        ),
        cell: ({ row }) => {
          const metadata = parseMetadata(row.original.metadata)
          const summary = ledgerSummary(row.original, metadata)
          return (
            <div className='min-w-0 space-y-1'>
              <LongText className='max-w-[320px] text-sm font-medium'>
                {summary.title || '-'}
              </LongText>
              {summary.detail && (
                <LongText className='text-muted-foreground max-w-[320px] text-xs'>
                  {summary.detail}
                </LongText>
              )}
            </div>
          )
        },
        meta: { label: t('Summary') },
      },
      {
        accessorKey: 'quota_delta',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Quota Delta')} />
        ),
        cell: ({ row }) => {
          const delta = signedQuotaDelta(row.original)
          return (
            <div
              className={cn(
                'text-right font-mono text-sm tabular-nums',
                delta > 0 && 'text-success',
                delta < 0 && 'text-destructive'
              )}
            >
              {delta > 0 ? '+' : ''}
              {formatQuota(delta)}
            </div>
          )
        },
        meta: { label: t('Quota Delta'), align: 'right' },
      },
      {
        accessorKey: 'status',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Status')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={t(row.original.status || 'Unknown')}
            variant={statusVariant(row.original.status)}
            copyable={false}
          />
        ),
        meta: { label: t('Status'), mobileHidden: true },
      },
      {
        accessorKey: 'request_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Request ID')} />
        ),
        cell: ({ row }) =>
          row.original.request_id ? (
            <LongText className='max-w-[180px] font-mono text-xs'>
              {row.original.request_id}
            </LongText>
          ) : (
            <span className='text-muted-foreground'>-</span>
          ),
        meta: { label: t('Request ID'), mobileHidden: true },
      },
      {
        accessorKey: 'token_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Token ID')} />
        ),
        cell: ({ row }) =>
          row.original.token_id > 0 ? (
            <span className='font-mono text-xs'>{row.original.token_id}</span>
          ) : (
            <span className='text-muted-foreground'>-</span>
          ),
        meta: { label: t('Token ID'), mobileHidden: true },
      },
      {
        accessorKey: 'created_at',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Created At')} />
        ),
        cell: ({ row }) => (
          <span className='text-muted-foreground text-xs whitespace-nowrap'>
            {formatTimestampToDate(row.original.created_at)}
          </span>
        ),
        meta: { label: t('Created At') },
      },
      {
        id: 'actions',
        cell: ({ row }) => (
          <LedgerActionsCell
            event={row.original}
            onOpenMetadata={options.onOpenMetadata}
            onOpenMCPCall={options.onOpenMCPCall}
          />
        ),
        enableSorting: false,
        enableHiding: false,
        meta: { label: t('Actions') },
      },
    ],
    [options.onOpenMCPCall, options.onOpenMetadata, t]
  )
}

export function UnifiedLedgerCard() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const [detail, setDetail] = useState<DetailState | null>(null)
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    request_id: false,
    token_id: false,
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
      pageKey: 'ledgerPage',
      pageSizeKey: 'ledgerPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : 20,
    },
    globalFilter: { enabled: true, key: 'ledgerFilter' },
    columnFilters: [
      { columnId: 'source', searchKey: 'ledgerSourceKind', type: 'array' },
      { columnId: 'event_type', searchKey: 'ledgerEventType', type: 'array' },
      {
        columnId: 'billing_source',
        searchKey: 'ledgerBillingSource',
        type: 'array',
      },
      {
        columnId: 'usage_kind',
        searchKey: 'ledgerUsageKind',
        type: 'array',
      },
      { columnId: 'request_id', searchKey: 'ledgerRequestId', type: 'string' },
      { columnId: 'source_id', searchKey: 'ledgerSourceId', type: 'string' },
      { columnId: 'token_id', searchKey: 'ledgerTokenId', type: 'string' },
    ],
  })

  const sourceFilter =
    (columnFilters.find((filter) => filter.id === 'source')?.value as
      | string[]
      | undefined) ?? []
  const eventTypeFilter =
    (columnFilters.find((filter) => filter.id === 'event_type')?.value as
      | string[]
      | undefined) ?? []
  const billingSourceFilter =
    (columnFilters.find((filter) => filter.id === 'billing_source')?.value as
      | string[]
      | undefined) ?? []
  const usageKindFilter =
    (columnFilters.find((filter) => filter.id === 'usage_kind')?.value as
      | string[]
      | undefined) ?? []
  const requestIdFilter =
    (columnFilters.find((filter) => filter.id === 'request_id')
      ?.value as string) ?? ''
  const sourceIdFilter =
    (columnFilters.find((filter) => filter.id === 'source_id')
      ?.value as string) ?? ''
  const tokenIdFilter =
    (columnFilters.find((filter) => filter.id === 'token_id')
      ?.value as string) ?? ''

  const requestParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    keyword: globalFilter,
    source: sourceFilter[0],
    source_id: sourceIdFilter,
    event_type: eventTypeFilter[0],
    request_id: requestIdFilter,
    billing_source: billingSourceFilter[0],
    usage_kind: usageKindFilter[0],
    token_id: positiveIntValue(tokenIdFilter),
    start_time: search.ledgerStartTime,
    end_time: search.ledgerEndTime,
  }

  const { data, error, isError, isLoading, isFetching, refetch } = useQuery({
    queryKey: ['wallet', 'ledger-events', requestParams],
    queryFn: async () => {
      const result = await getUserBillingEvents(requestParams)
      if (!result.success) {
        throw new Error(result.message || 'Failed to load ledger events')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  useEffect(() => {
    if (!isError) return
    toast.error(
      error instanceof Error ? error.message : t('Failed to load ledger events')
    )
  }, [error, isError, t])

  const columns = useLedgerColumns({
    onOpenMetadata: (event) => {
      setDetail({
        title: t('Metadata'),
        description: event.event_id,
        value: parseMetadata(event.metadata),
      })
    },
    onOpenMCPCall: (event) => {
      setDetail({
        title: t('MCP Tool Call'),
        description: event.related_mcp_tool_call?.tool_name,
        value: event.related_mcp_tool_call,
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
      <TitledCard
        title={t('Unified Ledger')}
        description={t('Wallet, subscription, model and MCP billing events')}
        icon={<ReceiptText className='size-4' />}
        action={
          <div className='flex items-center gap-2'>
            <StatusBadge
              label={`${t('Total')}: ${formatNumber(data?.total ?? 0)}`}
              variant='neutral'
              copyable={false}
            />
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => void refetch()}
              disabled={isFetching}
            >
              <RefreshCw
                className={cn('size-4', isFetching && 'animate-spin')}
              />
              {t('Refresh')}
            </Button>
          </div>
        }
      >
        <DataTablePage
          table={table}
          columns={columns}
          isLoading={isLoading}
          isFetching={isFetching}
          emptyTitle={t('No ledger events found')}
          emptyDescription={t(
            'Usage, top-up and subscription events will appear here.'
          )}
          skeletonKeyPrefix='wallet-ledger-skeleton'
          paginationInFooter={false}
          tableClassName='overflow-x-auto'
          toolbarProps={{
            searchPlaceholder: t('Filter by keyword...'),
            filters: [
              {
                columnId: 'source',
                title: t('Source'),
                options: ledgerEventSourceOptions.map((option) => ({
                  label: t(option.labelKey),
                  value: option.value,
                })),
                singleSelect: true,
              },
              {
                columnId: 'event_type',
                title: t('Type'),
                options: ledgerEventTypeOptions.map((option) => ({
                  label: t(option.labelKey),
                  value: option.value,
                })),
                singleSelect: true,
              },
              {
                columnId: 'billing_source',
                title: t('Funding'),
                options: ledgerBillingSourceOptions.map((option) => ({
                  label: t(option.labelKey),
                  value: option.value,
                })),
                singleSelect: true,
              },
              {
                columnId: 'usage_kind',
                title: t('Usage Kind'),
                options: ledgerUsageKindOptions.map((option) => ({
                  label: t(option.labelKey),
                  value: option.value,
                })),
                singleSelect: true,
              },
            ],
            additionalSearch: <LedgerTimeRangeFilter />,
            hasAdditionalFilters:
              search.ledgerStartTime != null || search.ledgerEndTime != null,
            onReset: () => {
              void navigate({
                search: (prev) => ({
                  ...prev,
                  ledgerPage: undefined,
                  ledgerStartTime: undefined,
                  ledgerEndTime: undefined,
                }),
              })
            },
          }}
        />
      </TitledCard>
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
