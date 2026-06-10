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
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  CheckCircle2,
  FileText,
  History,
  MoreHorizontal,
  ReceiptText,
  RefreshCw,
  RotateCcw,
  Save,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatNumber, formatQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { DataTableColumnHeader, DataTablePage } from '@/components/data-table'
import { LongText } from '@/components/long-text'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import {
  backfillBillingEventRelations,
  cleanupBillingEventRelationOrphans,
  getBillingEventHealth,
  getBillingEventRelationHealth,
  getBillingEventRelationInspection,
  getBillingEventSourceMatrix,
  getBillingEventSummary,
  listBillingEventRelationInspectionRuns,
  listBillingEvents,
  mcpQueryKeys,
  runBillingEventRelationInspection,
  updateBillingEventRelationInspection,
} from '../api'
import { positiveIntFilterValue } from '../lib/filter-values'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type {
  BillingEvent,
  BillingEventHealth,
  BillingEventRelationHealth,
  BillingEventRelationInspectionRunItem,
  BillingEventRelationInspectionStatus,
  BillingEventSourceMatrix,
  BillingEventSummary,
  PaginatedData,
} from '../types'
import { FilterInput } from './filter-input'
import { JsonDetailDialog } from './json-detail-dialog'
import { IdCell, LongTextCell, TimestampCell, TraceCell } from './table-cells'
import { TimeRangeFilter, timestampMsToSeconds } from './time-range-filter'
import { BillingEventBackfillDialog } from './billing-event-backfill-dialog'
import { BillingEventRelationDialog } from './billing-event-relation-dialog'

const route = getRouteApi('/_authenticated/mcp/$section')

type DetailState = {
  description?: string
  title: string
  value: unknown
}

type RelationDetailState = {
  event: BillingEvent
  mode: 'audit' | 'target'
}

type RelationInspectionFormState = {
  enabled: boolean
  interval_minutes: number
  limit: number
  auto_backfill: boolean
  auto_cleanup_orphans: boolean
  max_auto_backfill: number
  max_auto_cleanup_orphans: number
}

const defaultRelationInspectionForm: RelationInspectionFormState = {
  enabled: false,
  interval_minutes: 60,
  limit: 500,
  auto_backfill: false,
  auto_cleanup_orphans: false,
  max_auto_backfill: 200,
  max_auto_cleanup_orphans: 100,
}

function relationInspectionFormFromStatus(
  status?: BillingEventRelationInspectionStatus
): RelationInspectionFormState {
  const settings = status?.settings
  if (!settings) return defaultRelationInspectionForm
  return {
    enabled: settings.enabled,
    interval_minutes: settings.interval_minutes,
    limit: settings.limit,
    auto_backfill: settings.auto_backfill,
    auto_cleanup_orphans: settings.auto_cleanup_orphans,
    max_auto_backfill: settings.max_auto_backfill,
    max_auto_cleanup_orphans: settings.max_auto_cleanup_orphans,
  }
}

function normalizePositiveInteger(value: number, fallback: number) {
  if (!Number.isFinite(value) || value <= 0) return fallback
  return Math.floor(value)
}

function relationInspectionStatusVariant(status: string): StatusVariant {
  switch (status) {
    case 'running':
      return 'info'
    case 'success':
      return 'success'
    case 'blocked':
    case 'failed':
      return 'warning'
    default:
      return 'neutral'
  }
}

const billingEventSourceOptions = [
  { labelKey: 'MCP Tool Call', value: 'mcp_tool_call' },
  { labelKey: 'Model Request', value: 'model_request' },
  { labelKey: 'Async Task', value: 'async_task' },
  { labelKey: 'Violation Fee', value: 'violation_fee' },
  { labelKey: 'Wallet Top-up', value: 'wallet_topup' },
  { labelKey: 'Wallet Adjust', value: 'wallet_adjust' },
  { labelKey: 'Subscription', value: 'subscription' },
  { labelKey: 'Billing Event Repair', value: 'billing_event_repair' },
] as const

const billingEventTypeOptions = [
  { labelKey: 'Debit', value: 'debit' },
  { labelKey: 'Credit', value: 'credit' },
  { labelKey: 'Audit', value: 'audit' },
] as const

const billingEventStatusOptions = [
  { labelKey: 'Settled', value: 'settled' },
] as const

const billingSourceOptions = [
  { labelKey: 'Wallet', value: 'wallet' },
  { labelKey: 'Subscription', value: 'subscription' },
] as const

const billingUsageKindOptions = [
  { labelKey: 'Text', value: 'text' },
  { labelKey: 'Audio', value: 'audio' },
  { labelKey: 'Realtime', value: 'realtime' },
  { labelKey: 'Midjourney', value: 'midjourney' },
] as const

function getBillingEventSourceLabel(source: string) {
  return (
    billingEventSourceOptions.find((option) => option.value === source)
      ?.labelKey ?? source
  )
}

function getBillingEventTypeLabel(type: string) {
  return (
    billingEventTypeOptions.find((option) => option.value === type)?.labelKey ??
    type
  )
}

function parseMetadataValue(metadata: string | undefined): Record<string, unknown> {
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

function parseBillingEventMetadata(event: BillingEvent): Record<string, unknown> {
  return parseMetadataValue(event.metadata)
}

function metadataString(
  metadata: Record<string, unknown>,
  key: string
): string {
  const value = metadata[key]
  if (value == null || value === '') return ''
  return String(value)
}

function relatedMCPCallMetadata(event: BillingEvent): Record<string, unknown> {
  return parseMetadataValue(event.related_mcp_tool_call?.metadata)
}

function getBillingUsageKindLabel(usageKind: string) {
  switch (usageKind) {
    case 'midjourney':
      return 'Midjourney'
    case 'realtime':
      return 'Realtime'
    case 'audio':
      return 'Audio'
    case 'text':
      return 'Text'
    default:
      return usageKind
  }
}

function BillingEventActionsCell(props: {
  event: BillingEvent
  onOpenDetail: (detail: DetailState) => void
  onOpenToolCall: (event: BillingEvent) => void
  onOpenRelation: (detail: RelationDetailState) => void
}) {
  const { t } = useTranslation()
  const auditEvents = props.event.related_audit_events ?? []
  const targetEvent = props.event.related_target_event
  const mcpToolCall = props.event.related_mcp_tool_call

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
              title: t('Billing Metadata'),
              description: props.event.event_id,
              value: props.event.metadata,
            })
          }
        >
          <ReceiptText className='size-4' />
          {t('Billing Metadata')}
        </DropdownMenuItem>
        {mcpToolCall && (
          <>
            <DropdownMenuItem
              onSelect={() =>
                props.onOpenDetail({
                  title: t('MCP Tool Call'),
                  description: props.event.event_id,
                  value: mcpToolCall,
                })
              }
            >
              <FileText className='size-4' />
              {t('MCP Tool Call')}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => props.onOpenToolCall(props.event)}>
              <History className='size-4' />
              {t('Open Tool Calls')}
            </DropdownMenuItem>
          </>
        )}
        {auditEvents.length > 0 && (
          <DropdownMenuItem
            onSelect={() =>
              props.onOpenRelation({
                event: props.event,
                mode: 'audit',
              })
            }
          >
            <History className='size-4' />
            {t('Repair Audit')}
          </DropdownMenuItem>
        )}
        {targetEvent && (
          <DropdownMenuItem
            onSelect={() =>
              props.onOpenRelation({
                event: props.event,
                mode: 'target',
              })
            }
          >
            <ReceiptText className='size-4' />
            {t('Target Event')}
          </DropdownMenuItem>
        )}
        <DropdownMenuItem
          onSelect={() =>
            props.onOpenDetail({
              title: t('Billing Event Detail'),
              description: props.event.event_id,
              value: props.event,
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

function AmountCell(props: { event: BillingEvent }) {
  const { t } = useTranslation()
  const event = props.event
  const deltaPrefix = event.quota_delta > 0 ? '+' : ''

  return (
    <div className='flex min-w-[130px] flex-col gap-1'>
      <span className='tabular-nums'>{formatQuota(event.amount_quota)}</span>
      <span className='text-muted-foreground text-xs tabular-nums'>
        {deltaPrefix}
        {formatQuota(event.quota_delta)}
      </span>
      {event.cost > 0 && (
        <span className='text-muted-foreground text-xs tabular-nums'>
          {t('Cost')} {event.cost.toFixed(4)}
        </span>
      )}
    </div>
  )
}

function BillingHealthMetric(props: {
  label: string
  value: number
  tone?: 'success' | 'warning' | 'neutral'
}) {
  return (
    <div className='bg-muted/20 rounded-lg border px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div
        className={cn(
          'mt-1 text-base font-semibold tabular-nums',
          props.tone === 'success' && 'text-success',
          props.tone === 'warning' && 'text-warning'
        )}
      >
        {formatNumber(props.value)}
      </div>
    </div>
  )
}

function formatSignedQuota(value: number): string {
  const prefix = value > 0 ? '+' : ''
  return `${prefix}${formatQuota(value)}`
}

function formatBillingCost(value: number): string {
  return value.toFixed(4)
}

function BillingSummaryMetric(props: {
  label: string
  value: string
  tone?: 'success' | 'warning' | 'neutral'
}) {
  return (
    <div className='bg-muted/20 rounded-lg border px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div
        className={cn(
          'mt-1 text-base font-semibold tabular-nums',
          props.tone === 'success' && 'text-success',
          props.tone === 'warning' && 'text-warning'
        )}
      >
        {props.value}
      </div>
    </div>
  )
}

function BillingSummaryPanel(props: {
  summary?: BillingEventSummary
  isLoading: boolean
  isFetching: boolean
}) {
  const { t } = useTranslation()
  const summary = props.summary
  if (props.isLoading) {
    return <Skeleton className='h-28 w-full rounded-xl' />
  }
  if (!summary) return null

  const totals = summary.totals
  const maxDailyEvents = Math.max(
    1,
    ...summary.daily_trend.map((bucket) => bucket.total_events)
  )
  const topSources = summary.by_source.slice(0, 4)
  const trend = summary.daily_trend.slice(-14)

  return (
    <div className='bg-card rounded-xl border px-4 py-3'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='flex flex-wrap items-center gap-2'>
            <span className='text-sm font-medium'>{t('Ledger Summary')}</span>
            {props.isFetching && (
              <RefreshCw className='text-muted-foreground size-3.5 animate-spin' />
            )}
          </div>
          <div className='text-muted-foreground mt-1 text-xs'>
            {formatTimestampToDate(summary.start_time)}
            {' - '}
            {formatTimestampToDate(summary.end_time)}
          </div>
        </div>
        <div className='flex flex-wrap gap-1.5'>
          {summary.by_type.map((item) => (
            <StatusBadge
              key={item.key}
              label={`${t(getBillingEventTypeLabel(item.key))} ${formatNumber(item.total_events)}`}
              variant={item.key === 'credit' ? 'success' : 'info'}
              copyable={false}
            />
          ))}
        </div>
      </div>
      <div className='mt-3 grid grid-cols-2 gap-2 md:grid-cols-4'>
        <BillingSummaryMetric
          label={t('Events')}
          value={formatNumber(totals.total_events)}
        />
        <BillingSummaryMetric
          label={t('Net Quota')}
          value={formatSignedQuota(totals.net_quota_delta)}
          tone={totals.net_quota_delta >= 0 ? 'success' : 'warning'}
        />
        <BillingSummaryMetric
          label={t('Debits')}
          value={formatSignedQuota(totals.debit_quota_delta)}
          tone={totals.debit_quota_delta < 0 ? 'warning' : 'neutral'}
        />
        <BillingSummaryMetric
          label={t('Cost')}
          value={formatBillingCost(totals.total_cost)}
        />
      </div>
      <div className='mt-3 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(260px,0.9fr)]'>
        <div className='grid gap-2 sm:grid-cols-2'>
          {topSources.map((item) => (
            <div key={item.key} className='bg-muted/20 rounded-lg border px-3 py-2'>
              <div className='flex min-w-0 items-center justify-between gap-2'>
                <span className='min-w-0 truncate text-xs font-medium'>
                  {t(getBillingEventSourceLabel(item.key))}
                </span>
                <span className='text-muted-foreground text-xs tabular-nums'>
                  {formatNumber(item.total_events)}
                </span>
              </div>
              <div className='text-muted-foreground mt-1 text-xs tabular-nums'>
                {formatSignedQuota(item.net_quota_delta)}
                {item.total_cost > 0 ? ` · ${formatBillingCost(item.total_cost)}` : ''}
              </div>
            </div>
          ))}
        </div>
        <div className='bg-muted/20 rounded-lg border px-3 py-2'>
          <div className='text-muted-foreground text-xs'>{t('Daily Trend')}</div>
          <div className='mt-2 flex h-12 items-end gap-1'>
            {trend.length > 0 ? (
              trend.map((bucket) => (
                <div
                  key={bucket.bucket_start}
                  className='bg-primary/70 min-w-2 flex-1 rounded-t'
                  style={{
                    height: `${Math.max(12, (bucket.total_events / maxDailyEvents) * 48)}px`,
                  }}
                  title={`${formatTimestampToDate(bucket.bucket_start)} · ${formatNumber(bucket.total_events)}`}
                />
              ))
            ) : (
              <div className='text-muted-foreground flex h-full items-center text-xs'>
                {t('No trend data')}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function sourceMatrixStatusLabel(status: string) {
  switch (status) {
    case 'ready':
      return 'Ready'
    case 'record_only':
      return 'Record Only'
    case 'planned':
      return 'Planned'
    case 'audit_only':
      return 'Audit Only'
    default:
      return status || 'Unknown'
  }
}

function sourceMatrixStatusVariant(status: string): StatusVariant {
  switch (status) {
    case 'ready':
      return 'success'
    case 'record_only':
      return 'info'
    case 'planned':
      return 'warning'
    case 'audit_only':
      return 'purple'
    default:
      return 'neutral'
  }
}

function BillingSourceMatrixPanel(props: {
  isFetching: boolean
  matrix?: BillingEventSourceMatrix
}) {
  const { t } = useTranslation()
  const matrix = props.matrix
  if (!matrix) return null

  return (
    <div className='mt-3 border-t pt-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='min-w-0'>
          <div className='flex flex-wrap items-center gap-2'>
            <span className='text-sm font-medium'>{t('Source Coverage')}</span>
            {props.isFetching && (
              <RefreshCw className='text-muted-foreground size-3.5 animate-spin' />
            )}
          </div>
          <div className='text-muted-foreground mt-1 text-xs'>
            {t('Checked At')}:{' '}
            {matrix.checked_at ? formatTimestampToDate(matrix.checked_at) : '-'}
          </div>
        </div>
        <div className='flex flex-wrap gap-1.5'>
          <StatusBadge
            label={t('Ready {{count}}', { count: matrix.ready_sources })}
            variant='success'
            copyable={false}
          />
          <StatusBadge
            label={t('Record Only {{count}}', {
              count: matrix.record_only_sources,
            })}
            variant='info'
            copyable={false}
          />
          <StatusBadge
            label={t('Planned {{count}}', { count: matrix.planned_sources })}
            variant='warning'
            copyable={false}
          />
          <StatusBadge
            label={t('Audit Only {{count}}', {
              count: matrix.audit_only_sources,
            })}
            variant='purple'
            copyable={false}
          />
        </div>
      </div>
      <div className='mt-3 grid gap-2 md:grid-cols-2 xl:grid-cols-4'>
        {matrix.items.map((item) => (
          <div key={item.source} className='bg-muted/20 rounded-lg border p-3'>
            <div className='flex min-w-0 items-start justify-between gap-2'>
              <div className='min-w-0'>
                <div className='truncate text-xs font-medium'>
                  {t(item.label || getBillingEventSourceLabel(item.source))}
                </div>
                <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
                  {item.event_source}
                </div>
              </div>
              <StatusBadge
                label={t(sourceMatrixStatusLabel(item.status))}
                variant={sourceMatrixStatusVariant(item.status)}
                copyable={false}
              />
            </div>
            <div className='mt-2 flex flex-wrap gap-1'>
              {item.supports_recording && (
                <StatusBadge
                  label={t('Recording')}
                  variant='neutral'
                  copyable={false}
                />
              )}
              {item.supports_backfill && (
                <StatusBadge
                  label={t('Backfill')}
                  variant='success'
                  copyable={false}
                />
              )}
              {item.supports_reconciliation && (
                <StatusBadge
                  label={t('Reconcile')}
                  variant='success'
                  copyable={false}
                />
              )}
              {item.supports_refund_or_delta && (
                <StatusBadge
                  label={t('Refund/Delta')}
                  variant='info'
                  copyable={false}
                />
              )}
              {item.supports_audit_relation && (
                <StatusBadge
                  label={t('Relation')}
                  variant='purple'
                  copyable={false}
                />
              )}
            </div>
            {(item.backfill_sources?.length ?? 0) > 0 && (
              <div className='text-muted-foreground mt-2 truncate font-mono text-xs'>
                {item.backfill_sources.join(', ')}
              </div>
            )}
            {(item.notes?.length ?? 0) > 0 && (
              <div className='text-muted-foreground mt-2 line-clamp-2 text-xs'>
                {t(item.notes[0])}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

function RelationInspectionSwitch(props: {
  checked: boolean
  disabled?: boolean
  label: string
  onCheckedChange: (checked: boolean) => void
}) {
  return (
    <Label className='bg-muted/20 min-h-8 rounded-lg border px-2.5 py-2 text-xs font-normal'>
      <Switch
        size='sm'
        checked={props.checked}
        disabled={props.disabled}
        onCheckedChange={(checked) => props.onCheckedChange(!!checked)}
      />
      <span>{props.label}</span>
    </Label>
  )
}

function RelationInspectionRunsList(props: {
  isFetching: boolean
  page: number
  pageSize: number
  runs: BillingEventRelationInspectionRunItem[]
  total: number
  onPageChange: (page: number) => void
  onPageSizeChange: (pageSize: number) => void
}) {
  const { t } = useTranslation()
  const totalPages = Math.max(1, Math.ceil(props.total / props.pageSize))
  const canPrevious = props.page > 1
  const canNext = props.page < totalPages

  return (
    <div className='mt-3 rounded-lg border'>
      <div className='flex flex-wrap items-center justify-between gap-2 border-b px-3 py-2'>
        <div className='flex min-w-0 items-center gap-2'>
          <span className='text-xs font-medium'>{t('Inspection History')}</span>
          {props.isFetching && (
            <RefreshCw className='text-muted-foreground size-3.5 animate-spin' />
          )}
        </div>
        <div className='flex flex-wrap items-center gap-2 text-xs'>
          <span className='text-muted-foreground tabular-nums'>
            {t('Page {{current}} of {{total}}', {
              current: props.page,
              total: totalPages,
            })}
          </span>
          <Select
            value={`${props.pageSize}`}
            onValueChange={(value) => props.onPageSizeChange(Number(value))}
          >
            <SelectTrigger size='sm' className='w-[64px]'>
              <SelectValue placeholder={props.pageSize} />
            </SelectTrigger>
            <SelectContent side='top' alignItemWithTrigger={false}>
              <SelectGroup>
                {[5, 10, 20, 50].map((pageSize) => (
                  <SelectItem key={pageSize} value={`${pageSize}`}>
                    {pageSize}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Button
            type='button'
            variant='outline'
            size='icon-sm'
            disabled={!canPrevious || props.isFetching}
            onClick={() => props.onPageChange(props.page - 1)}
          >
            <span className='sr-only'>{t('Go to previous page')}</span>
            <ChevronLeft className='size-4' />
          </Button>
          <Button
            type='button'
            variant='outline'
            size='icon-sm'
            disabled={!canNext || props.isFetching}
            onClick={() => props.onPageChange(props.page + 1)}
          >
            <span className='sr-only'>{t('Go to next page')}</span>
            <ChevronRight className='size-4' />
          </Button>
        </div>
      </div>
      {props.runs.length > 0 ? (
        <div className='divide-y'>
          {props.runs.map((run) => (
            <div
              key={run.id}
              className='grid gap-2 px-3 py-2 text-xs md:grid-cols-[120px_90px_minmax(0,1fr)_150px]'
            >
              <div className='flex flex-wrap items-center gap-1'>
                <StatusBadge
                  label={t(run.status || 'Unknown')}
                  variant={relationInspectionStatusVariant(run.status)}
                  copyable={false}
                />
                <StatusBadge
                  label={t(run.trigger || 'Unknown')}
                  variant='neutral'
                  copyable={false}
                />
              </div>
              <div className='text-muted-foreground tabular-nums'>
                {run.started_at ? formatTimestampToDate(run.started_at) : '-'}
              </div>
              <div className='min-w-0'>
                <div className='flex flex-wrap gap-x-3 gap-y-1 tabular-nums'>
                  <span>
                    {t('Scanned')}: {formatNumber(run.scanned_audit_events)}
                  </span>
                  <span>
                    {t('Missing')}: {formatNumber(run.missing_relations)}
                  </span>
                  <span>
                    {t('Backfill')}: {formatNumber(run.backfill_created)}
                    {run.backfill_blocked
                      ? ` / ${formatNumber(run.backfill_would_create)}`
                      : ''}
                  </span>
                  <span>
                    {t('Clean')}: {formatNumber(run.cleanup_deleted)}
                    {run.cleanup_blocked
                      ? ` / ${formatNumber(run.cleanup_would_delete)}`
                      : ''}
                  </span>
                </div>
                {run.message ? (
                  <div className='text-muted-foreground mt-1 truncate'>
                    {run.message}
                  </div>
                ) : null}
              </div>
              <div className='text-muted-foreground tabular-nums'>
                {t('Cursor')}: {formatNumber(run.cursor)}
                {' -> '}
                {formatNumber(run.next_cursor)}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className='text-muted-foreground px-3 py-4 text-xs'>
          {t('No inspection runs yet')}
        </div>
      )}
    </div>
  )
}

function BillingHealthPanel(props: {
  health?: BillingEventHealth
  sourceMatrix?: BillingEventSourceMatrix
  relationHealth?: BillingEventRelationHealth
  relationInspection?: BillingEventRelationInspectionStatus
  relationInspectionRuns?: PaginatedData<BillingEventRelationInspectionRunItem>
  relationInspectionForm: RelationInspectionFormState
  isLoading: boolean
  isFetching: boolean
  isSourceMatrixFetching: boolean
  isRelationFetching: boolean
  isInspectionFetching: boolean
  isInspectionRunsFetching: boolean
  isSavingInspection: boolean
  isRunningInspection: boolean
  isPreviewingRelations: boolean
  isBackfillingRelations: boolean
  isCleaningRelationOrphans: boolean
  inspectionRunsPage: number
  inspectionRunsPageSize: number
  onInspectionFormChange: (form: RelationInspectionFormState) => void
  onInspectionRunsPageChange: (page: number) => void
  onInspectionRunsPageSizeChange: (pageSize: number) => void
  onRefresh: () => void
  onPreviewRelationBackfill: () => void
  onBackfillRelations: () => void
  onCleanRelationOrphans: () => void
  onSaveInspection: () => void
  onRunInspection: () => void
  onResetInspectionCursor: () => void
  onNextRelationPage: () => void
  onResetRelationCursor: () => void
  onOpenBackfill: () => void
}) {
  const { t } = useTranslation()
  if (props.isLoading) {
    return <Skeleton className='h-28 w-full rounded-xl' />
  }
  const health = props.health
  if (!health) return null

  const relationHealth = props.relationHealth
  const relationNeedsReview = relationHealth?.needs_review ?? false
  const needsReview = health.needs_review || relationNeedsReview
  const checkedAt = health.checked_at
    ? formatTimestampToDate(health.checked_at)
    : '-'
  const relationCheckedAt = relationHealth?.checked_at
    ? formatTimestampToDate(relationHealth.checked_at)
    : '-'
  const relationOrphans = relationHealth
    ? relationHealth.orphan_source_relations + relationHealth.orphan_target_relations
    : 0
  const inspection = props.relationInspection
  const inspectionSettings = inspection?.settings
  const inspectionLastRunAt = inspection?.last_run_at
    ? formatTimestampToDate(inspection.last_run_at)
    : '-'
  const inspectionStatus = inspection?.running
    ? 'Running'
    : inspection?.last_run_status || 'Idle'
  const inspectionStatusVariant = relationInspectionStatusVariant(
    inspection?.running ? 'running' : inspection?.last_run_status || ''
  )
  const inspectionForm = props.relationInspectionForm
  const inspectionBusy =
    props.isInspectionFetching ||
    props.isSavingInspection ||
    props.isRunningInspection ||
    !!inspection?.running
  const inspectionRuns = props.relationInspectionRuns?.items ?? inspection?.recent_runs ?? []
  const inspectionRunsTotal =
    props.relationInspectionRuns?.total ?? inspection?.recent_runs?.length ?? 0
  const inspectionRunsPage =
    props.relationInspectionRuns?.page ?? props.inspectionRunsPage
  const inspectionRunsPageSize =
    props.relationInspectionRuns?.page_size ?? props.inspectionRunsPageSize

  return (
    <div
      className={cn(
        'bg-card rounded-xl border px-4 py-3',
        needsReview && 'border-warning/50 bg-warning/5'
      )}
    >
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='flex flex-wrap items-center gap-2'>
            {needsReview ? (
              <AlertTriangle className='text-warning size-4' />
            ) : (
              <CheckCircle2 className='text-success size-4' />
            )}
            <span className='text-sm font-medium'>{t('Billing Health')}</span>
            <StatusBadge
              label={t(needsReview ? 'Needs Review' : 'Balanced')}
              variant={needsReview ? 'warning' : 'success'}
              copyable={false}
            />
          </div>
          <div className='text-muted-foreground mt-1 text-xs'>
            {t('Checked At')}: {checkedAt}
          </div>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={props.isFetching || props.isRelationFetching}
            onClick={props.onRefresh}
          >
            <RefreshCw
              className={cn(
                'size-3.5',
                (props.isFetching || props.isRelationFetching) && 'animate-spin'
              )}
            />
            {t('Refresh')}
          </Button>
          {needsReview && (
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={props.onOpenBackfill}
            >
              <RotateCcw className='size-3.5' />
              {t('Review')}
            </Button>
          )}
        </div>
      </div>
      <div className='mt-3 grid grid-cols-2 gap-2 md:grid-cols-5'>
        <BillingHealthMetric
          label={t('Would Create')}
          value={health.total_would_create}
          tone={health.total_would_create > 0 ? 'warning' : 'success'}
        />
        <BillingHealthMetric
          label={t('Missing')}
          value={health.total_missing}
          tone={health.total_missing > 0 ? 'warning' : 'success'}
        />
        <BillingHealthMetric
          label={t('Mismatched')}
          value={health.total_mismatched}
          tone={health.total_mismatched > 0 ? 'warning' : 'success'}
        />
        <BillingHealthMetric
          label={t('Invalid')}
          value={health.total_invalid}
          tone={health.total_invalid > 0 ? 'warning' : 'success'}
        />
        <BillingHealthMetric
          label={t('Errors')}
          value={health.total_error_count}
          tone={health.total_error_count > 0 ? 'warning' : 'success'}
        />
      </div>
      <BillingSourceMatrixPanel
        matrix={props.sourceMatrix}
        isFetching={props.isSourceMatrixFetching}
      />
      {relationHealth && (
        <div className='mt-3 border-t pt-3'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div className='min-w-0'>
              <div className='flex flex-wrap items-center gap-2'>
                {relationNeedsReview ? (
                  <AlertTriangle className='text-warning size-4' />
                ) : (
                  <CheckCircle2 className='text-success size-4' />
                )}
                <span className='text-sm font-medium'>
                  {t('Audit Relations')}
                </span>
                <StatusBadge
                  label={t(relationNeedsReview ? 'Needs Review' : 'Balanced')}
                  variant={relationNeedsReview ? 'warning' : 'success'}
                  copyable={false}
                />
              </div>
              <div className='text-muted-foreground mt-1 text-xs'>
                {t('Checked At')}: {relationCheckedAt}
              </div>
            </div>
            <div className='flex flex-wrap items-center gap-2'>
              {relationHealth.cursor > 0 && (
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={props.isRelationFetching}
                  onClick={props.onResetRelationCursor}
                >
                  {t('Latest')}
                </Button>
              )}
              {relationHealth.has_more && (
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={props.isRelationFetching}
                  onClick={props.onNextRelationPage}
                >
                  {t('Next Page')}
                </Button>
              )}
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={
                  props.isPreviewingRelations || props.isBackfillingRelations
                }
                onClick={props.onPreviewRelationBackfill}
              >
                {props.isPreviewingRelations && (
                  <RefreshCw className='size-3.5 animate-spin' />
                )}
                {t('Preview')}
              </Button>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={
                  !relationNeedsReview ||
                  props.isPreviewingRelations ||
                  props.isBackfillingRelations
                }
                onClick={props.onBackfillRelations}
              >
                {props.isBackfillingRelations ? (
                  <RefreshCw className='size-3.5 animate-spin' />
                ) : (
                  <RotateCcw className='size-3.5' />
                )}
                {t('Backfill')}
              </Button>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={
                  relationOrphans <= 0 ||
                  props.isPreviewingRelations ||
                  props.isBackfillingRelations ||
                  props.isCleaningRelationOrphans
                }
                onClick={props.onCleanRelationOrphans}
              >
                {props.isCleaningRelationOrphans ? (
                  <RefreshCw className='size-3.5 animate-spin' />
                ) : (
                  <RotateCcw className='size-3.5' />
                )}
                {t('Clean')}
              </Button>
            </div>
          </div>
          <div className='text-muted-foreground mt-2 text-xs'>
            {t('Scanned')}: {formatNumber(relationHealth.scanned_audit_events)}
            {' / '}
            {t('Total')}: {formatNumber(relationHealth.total_audit_events)}
            {relationHealth.has_more && relationHealth.next_cursor > 0 ? (
              <span>
                {' · '}
                {t('More historical audit events available')}
              </span>
            ) : null}
          </div>
          <div className='mt-3 grid grid-cols-2 gap-2 md:grid-cols-5'>
            <BillingHealthMetric
              label={t('Audit Events')}
              value={relationHealth.total_audit_events}
              tone='neutral'
            />
            <BillingHealthMetric
              label={t('Relations')}
              value={relationHealth.total_relations}
              tone='neutral'
            />
            <BillingHealthMetric
              label={t('Missing Links')}
              value={relationHealth.missing_relations}
              tone={relationHealth.missing_relations > 0 ? 'warning' : 'success'}
            />
            <BillingHealthMetric
              label={t('Invalid Audits')}
              value={relationHealth.invalid_audit_events}
              tone={
                relationHealth.invalid_audit_events > 0 ? 'warning' : 'success'
              }
            />
            <BillingHealthMetric
              label={t('Orphans')}
              value={
                relationHealth.orphan_source_relations +
                relationHealth.orphan_target_relations
              }
              tone={
                relationHealth.orphan_source_relations +
                  relationHealth.orphan_target_relations >
                0
                  ? 'warning'
                  : 'success'
              }
            />
          </div>
          {inspection && (
            <div
              className={cn(
                'mt-3 border-t pt-3',
                inspection.last_run_status === 'blocked' &&
                  'border-warning/50 bg-warning/5 -mx-2 rounded-lg px-2 pb-2'
              )}
            >
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <div className='min-w-0'>
                  <div className='flex flex-wrap items-center gap-2'>
                    <span className='text-sm font-medium'>
                      {t('Scheduled Inspection')}
                    </span>
                    <StatusBadge
                      label={t(inspectionStatus)}
                      variant={inspectionStatusVariant}
                      copyable={false}
                    />
                    {inspectionSettings?.enabled ? (
                      <StatusBadge
                        label={t('Enabled')}
                        variant='success'
                        copyable={false}
                      />
                    ) : (
                      <StatusBadge
                        label={t('Disabled')}
                        variant='neutral'
                        copyable={false}
                      />
                    )}
                  </div>
                  <div className='text-muted-foreground mt-1 text-xs'>
                    {t('Last Run')}: {inspectionLastRunAt}
                    {inspection?.last_run_message ? (
                      <span>
                        {' · '}
                        {inspection.last_run_message}
                      </span>
                    ) : null}
                  </div>
                </div>
                <div className='flex flex-wrap items-center gap-2'>
                  {(inspectionSettings?.cursor ?? 0) > 0 && (
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      disabled={inspectionBusy}
                      onClick={props.onResetInspectionCursor}
                    >
                      {t('Reset Cursor')}
                    </Button>
                  )}
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={props.isSavingInspection}
                    onClick={props.onSaveInspection}
                  >
                    {props.isSavingInspection ? (
                      <RefreshCw className='size-3.5 animate-spin' />
                    ) : (
                      <Save className='size-3.5' />
                    )}
                    {t('Save')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={inspectionBusy}
                    onClick={props.onRunInspection}
                  >
                    {props.isRunningInspection || inspection?.running ? (
                      <RefreshCw className='size-3.5 animate-spin' />
                    ) : (
                      <RotateCcw className='size-3.5' />
                    )}
                    {t('Run Now')}
                  </Button>
                </div>
              </div>
              <div className='mt-3 grid gap-2 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto_auto_auto]'>
                <div className='grid grid-cols-2 gap-2'>
                  <Label className='flex-col items-start gap-1 text-xs font-normal'>
                    <span className='text-muted-foreground'>
                      {t('Interval Minutes')}
                    </span>
                    <Input
                      type='number'
                      min={1}
                      max={10080}
                      value={inspectionForm.interval_minutes}
                      disabled={props.isSavingInspection}
                      onChange={(event) =>
                        props.onInspectionFormChange({
                          ...inspectionForm,
                          interval_minutes: Number(event.target.value),
                        })
                      }
                    />
                  </Label>
                  <Label className='flex-col items-start gap-1 text-xs font-normal'>
                    <span className='text-muted-foreground'>{t('Batch Size')}</span>
                    <Input
                      type='number'
                      min={1}
                      max={5000}
                      value={inspectionForm.limit}
                      disabled={props.isSavingInspection}
                      onChange={(event) =>
                        props.onInspectionFormChange({
                          ...inspectionForm,
                          limit: Number(event.target.value),
                        })
                      }
                    />
                  </Label>
                </div>
                <div className='grid grid-cols-1 gap-2 sm:grid-cols-3'>
                  <RelationInspectionSwitch
                    checked={inspectionForm.enabled}
                    disabled={props.isSavingInspection}
                    label={t('Enabled')}
                    onCheckedChange={(enabled) =>
                      props.onInspectionFormChange({
                        ...inspectionForm,
                        enabled,
                      })
                    }
                  />
                  <RelationInspectionSwitch
                    checked={inspectionForm.auto_backfill}
                    disabled={props.isSavingInspection}
                    label={t('Auto Backfill')}
                    onCheckedChange={(auto_backfill) =>
                      props.onInspectionFormChange({
                        ...inspectionForm,
                        auto_backfill,
                      })
                    }
                  />
                  <RelationInspectionSwitch
                    checked={inspectionForm.auto_cleanup_orphans}
                    disabled={props.isSavingInspection}
                    label={t('Auto Clean')}
                    onCheckedChange={(auto_cleanup_orphans) =>
                      props.onInspectionFormChange({
                        ...inspectionForm,
                        auto_cleanup_orphans,
                      })
                    }
                  />
                </div>
                <div className='bg-muted/20 rounded-lg border px-3 py-2 text-xs'>
                  <div className='text-muted-foreground'>{t('Cursor')}</div>
                  <div className='mt-1 font-mono tabular-nums'>
                    {formatNumber(inspectionSettings?.cursor ?? 0)}
                  </div>
                </div>
                <div className='bg-muted/20 rounded-lg border px-3 py-2 text-xs'>
                  <div className='text-muted-foreground'>{t('Last Created')}</div>
                  <div className='mt-1 font-mono tabular-nums'>
                    {formatNumber(inspection.last_backfill?.created ?? 0)}
                  </div>
                </div>
                <div className='bg-muted/20 rounded-lg border px-3 py-2 text-xs'>
                  <div className='text-muted-foreground'>{t('Last Cleaned')}</div>
                  <div className='mt-1 font-mono tabular-nums'>
                    {formatNumber(inspection.last_cleanup?.deleted ?? 0)}
                  </div>
                </div>
              </div>
              <div className='mt-2 grid grid-cols-2 gap-2 md:max-w-[520px]'>
                <Label className='flex-col items-start gap-1 text-xs font-normal'>
                  <span className='text-muted-foreground'>
                    {t('Max Auto Backfill')}
                  </span>
                  <Input
                    type='number'
                    min={1}
                    max={5000}
                    value={inspectionForm.max_auto_backfill}
                    disabled={props.isSavingInspection}
                    onChange={(event) =>
                      props.onInspectionFormChange({
                        ...inspectionForm,
                        max_auto_backfill: Number(event.target.value),
                      })
                    }
                  />
                </Label>
                <Label className='flex-col items-start gap-1 text-xs font-normal'>
                  <span className='text-muted-foreground'>
                    {t('Max Auto Clean')}
                  </span>
                  <Input
                    type='number'
                    min={1}
                    max={5000}
                    value={inspectionForm.max_auto_cleanup_orphans}
                    disabled={props.isSavingInspection}
                    onChange={(event) =>
                      props.onInspectionFormChange({
                        ...inspectionForm,
                        max_auto_cleanup_orphans: Number(event.target.value),
                      })
                    }
                  />
                </Label>
              </div>
              <RelationInspectionRunsList
                isFetching={props.isInspectionRunsFetching}
                page={inspectionRunsPage}
                pageSize={inspectionRunsPageSize}
                runs={inspectionRuns}
                total={inspectionRunsTotal}
                onPageChange={props.onInspectionRunsPageChange}
                onPageSizeChange={props.onInspectionRunsPageSizeChange}
              />
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function useBillingEventColumns(options: {
  onOpenDetail: (detail: DetailState) => void
  onOpenToolCall: (event: BillingEvent) => void
  onOpenRelation: (detail: RelationDetailState) => void
}): ColumnDef<BillingEvent>[] {
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
        accessorKey: 'event_type',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Type')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={t(getBillingEventTypeLabel(row.original.event_type))}
            variant={row.original.event_type === 'credit' ? 'success' : 'info'}
            copyable={false}
          />
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Type'), mobileBadge: true },
      },
      {
        accessorKey: 'source',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Source')} />
        ),
        cell: ({ row }) => (
          <div className='flex min-w-[190px] flex-col gap-1'>
            <div className='flex flex-wrap items-center gap-1'>
              <StatusBadge
                label={t(getBillingEventSourceLabel(row.original.source))}
                variant='neutral'
                copyable={false}
              />
              <StatusBadge
                label={t(row.original.status || 'Unknown')}
                variant={
                  row.original.status === 'settled' ? 'success' : 'warning'
                }
                copyable={false}
              />
              {(row.original.related_audit_events?.length ?? 0) > 0 && (
                <StatusBadge
                  label={t('Repair Audit')}
                  variant='info'
                  copyable={false}
                />
              )}
              {row.original.related_target_event && (
                <StatusBadge
                  label={t('Target Event')}
                  variant='purple'
                  copyable={false}
                />
              )}
            </div>
            <LongText className='max-w-[220px] font-mono text-xs'>
              {row.original.event_id}
            </LongText>
          </div>
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Source'), mobileTitle: true },
      },
      {
        accessorKey: 'status',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Status')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={t(row.original.status || 'Unknown')}
            variant={row.original.status === 'settled' ? 'success' : 'warning'}
            copyable={false}
          />
        ),
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Status'), mobileHidden: true },
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
        meta: { label: t('Request ID') },
      },
      {
        accessorKey: 'source_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Source ID')} />
        ),
        cell: ({ row }) => (
          <LongTextCell
            value={row.original.source_id}
            className='max-w-[160px] font-mono'
          />
        ),
        meta: { label: t('Source ID'), mobileHidden: true },
      },
      {
        accessorKey: 'amount_quota',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Amount')} />
        ),
        cell: ({ row }) => <AmountCell event={row.original} />,
        meta: { label: t('Amount') },
      },
      {
        accessorKey: 'billing_source',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Billing Source')} />
        ),
        cell: ({ row }) => {
          const metadata = parseBillingEventMetadata(row.original)
          const usageKind = metadataString(metadata, 'usage_kind')
          const subscriptionId = metadataString(metadata, 'subscription_id')
          return (
            <div className='flex min-w-[150px] flex-col gap-1'>
              <div className='flex flex-wrap items-center gap-1'>
                <StatusBadge
                  label={t(row.original.billing_source || 'Unknown')}
                  variant='neutral'
                  copyable={false}
                />
                {usageKind && (
                  <StatusBadge
                    label={t(getBillingUsageKindLabel(usageKind))}
                    variant='info'
                    copyable={false}
                  />
                )}
              </div>
              <span className='text-muted-foreground text-xs'>
                {row.original.price_unit || '-'}
              </span>
              {row.original.billing_source === 'subscription' &&
                subscriptionId && (
                  <span className='text-muted-foreground text-xs tabular-nums'>
                    {t('Subscription')} #{subscriptionId}
                  </span>
                )}
            </div>
          )
        },
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Billing Source') },
      },
      {
        id: 'usage_kind',
        accessorFn: (event) =>
          metadataString(parseBillingEventMetadata(event), 'usage_kind'),
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Usage Kind')} />
        ),
        cell: ({ row }) => {
          const usageKind = String(row.getValue('usage_kind') ?? '')
          return usageKind ? (
            <StatusBadge
              label={t(getBillingUsageKindLabel(usageKind))}
              variant='info'
              copyable={false}
            />
          ) : (
            <span className='text-muted-foreground'>-</span>
          )
        },
        filterFn: (row, id, value) =>
          Array.isArray(value) && value.includes(String(row.getValue(id))),
        meta: { label: t('Usage Kind'), mobileHidden: true },
      },
      {
        accessorKey: 'token_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Trace')} />
        ),
        cell: ({ row }) => {
          const event = row.original
          const mcpCall = event.related_mcp_tool_call
          const mcpMetadata = relatedMCPCallMetadata(event)
          return (
            <TraceCell
              items={[
                { label: t('User'), value: event.user_id },
                { label: t('Token'), value: event.token_id },
                { label: t('Source ID'), value: event.source_id },
                {
                  label: t('Proxy'),
                  value: metadataString(mcpMetadata, 'proxy_server_namespace'),
                },
                {
                  label: t('Downstream'),
                  value:
                    metadataString(mcpMetadata, 'downstream_tool_name') ||
                    mcpCall?.tool_name,
                },
                {
                  label: t('Transport'),
                  value: metadataString(mcpMetadata, 'transport'),
                },
                {
                  label: t('Downstream Request'),
                  value: metadataString(mcpMetadata, 'downstream_request_id'),
                },
              ]}
            />
          )
        },
        meta: { label: t('Trace') },
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
          <BillingEventActionsCell
            event={row.original}
            onOpenDetail={options.onOpenDetail}
            onOpenToolCall={options.onOpenToolCall}
            onOpenRelation={options.onOpenRelation}
          />
        ),
        enableSorting: false,
        enableHiding: false,
        meta: { label: t('Actions') },
      },
    ],
    [options.onOpenDetail, options.onOpenRelation, options.onOpenToolCall, t]
  )
}

export function BillingEventsTable() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isAdmin = useIsAdmin()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const [detail, setDetail] = useState<DetailState | null>(null)
  const [relationDetail, setRelationDetail] =
    useState<RelationDetailState | null>(null)
  const [backfillOpen, setBackfillOpen] = useState(false)
  const [relationCursor, setRelationCursor] = useState(0)
  const [inspectionRunsPage, setInspectionRunsPage] = useState(1)
  const [inspectionRunsPageSize, setInspectionRunsPageSize] = useState(10)
  const [relationInspectionForm, setRelationInspectionForm] = useState(
    defaultRelationInspectionForm
  )
  const [relationInspectionDirty, setRelationInspectionDirty] = useState(false)
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    request_id: false,
    source_id: false,
    status: false,
    usage_kind: false,
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
      pageKey: 'billingPage',
      pageSizeKey: 'billingPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : 20,
    },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'source', searchKey: 'billingSourceKind', type: 'array' },
      { columnId: 'event_type', searchKey: 'billingEventType', type: 'array' },
      { columnId: 'status', searchKey: 'billingStatus', type: 'array' },
      { columnId: 'request_id', searchKey: 'requestId', type: 'string' },
      { columnId: 'source_id', searchKey: 'billingSourceId', type: 'string' },
      { columnId: 'token_id', searchKey: 'tokenId', type: 'string' },
      {
        columnId: 'billing_source',
        searchKey: 'billingSource',
        type: 'array',
      },
      {
        columnId: 'usage_kind',
        searchKey: 'billingUsageKind',
        type: 'array',
      },
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
  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
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
  const billingSourceFilter =
    (columnFilters.find((filter) => filter.id === 'billing_source')?.value as
      | string[]
      | undefined) ?? []
  const usageKindFilter =
    (columnFilters.find((filter) => filter.id === 'usage_kind')?.value as
      | string[]
      | undefined) ?? []

  const requestParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    scope: isAdmin ? ('all' as const) : undefined,
    keyword: globalFilter,
    source: sourceFilter[0],
    source_id: sourceIdFilter,
    event_type: eventTypeFilter[0],
    status: statusFilter[0],
    token_id: positiveIntFilterValue(tokenIdFilter),
    request_id: requestIdFilter,
    billing_source: billingSourceFilter[0],
    usage_kind: usageKindFilter[0],
    start_time: timestampMsToSeconds(search.billingStartTime),
    end_time: timestampMsToSeconds(search.billingEndTime),
  }
  const summaryParams = {
    scope: requestParams.scope,
    keyword: requestParams.keyword,
    source: requestParams.source,
    source_id: requestParams.source_id,
    event_type: requestParams.event_type,
    status: requestParams.status,
    token_id: requestParams.token_id,
    request_id: requestParams.request_id,
    billing_source: requestParams.billing_source,
    usage_kind: requestParams.usage_kind,
    start_time: requestParams.start_time,
    end_time: requestParams.end_time,
  }

  const {
    data,
    error: billingEventsError,
    isError: isBillingEventsError,
    isLoading,
    isFetching,
  } = useQuery({
    queryKey: mcpQueryKeys.billingEventsList(requestParams),
    queryFn: async () => {
      const result = await listBillingEvents(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load billing events')
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  useEffect(() => {
    if (!isBillingEventsError) return
    toast.error(
      mcpQueryErrorMessage(
        billingEventsError,
        t('Failed to load billing events')
      )
    )
  }, [billingEventsError, isBillingEventsError, t])

  const {
    data: billingSummary,
    error: billingSummaryError,
    isError: isBillingSummaryError,
    isLoading: isBillingSummaryLoading,
    isFetching: isBillingSummaryFetching,
  } = useQuery({
    queryKey: mcpQueryKeys.billingEventsSummary(summaryParams),
    queryFn: async () => {
      const result = await getBillingEventSummary(summaryParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load billing summary')
      }
      return result.data
    },
    placeholderData: (previousData) => previousData,
    staleTime: 30 * 1000,
  })

  useEffect(() => {
    if (!isBillingSummaryError) return
    toast.error(
      mcpQueryErrorMessage(
        billingSummaryError,
        t('Failed to load billing summary')
      )
    )
  }, [billingSummaryError, isBillingSummaryError, t])

  const healthParams = useMemo(
    () => ({
      sources: undefined,
      limit: 500,
    }),
    []
  )
  const {
    data: health,
    error: healthError,
    isError: isHealthError,
    isLoading: isHealthLoading,
    isFetching: isHealthFetching,
    refetch: refetchHealth,
  } = useQuery({
    queryKey: mcpQueryKeys.billingEventsHealth(healthParams),
    queryFn: async () => {
      const result = await getBillingEventHealth(healthParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load billing health')
      }
      return result.data
    },
    enabled: isAdmin,
    refetchInterval: 60 * 1000,
    staleTime: 30 * 1000,
  })

  useEffect(() => {
    if (!isHealthError) return
    toast.error(
      mcpQueryErrorMessage(healthError, t('Failed to load billing health'))
    )
  }, [healthError, isHealthError, t])

  const {
    data: sourceMatrix,
    error: sourceMatrixError,
    isError: isSourceMatrixError,
    isFetching: isSourceMatrixFetching,
    refetch: refetchSourceMatrix,
  } = useQuery({
    queryKey: mcpQueryKeys.billingEventsSourceMatrix(),
    queryFn: async () => {
      const result = await getBillingEventSourceMatrix()
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load source coverage')
      }
      return result.data
    },
    enabled: isAdmin,
    refetchInterval: 60 * 1000,
    staleTime: 30 * 1000,
  })

  useEffect(() => {
    if (!isSourceMatrixError) return
    toast.error(
      mcpQueryErrorMessage(
        sourceMatrixError,
        t('Failed to load source coverage')
      )
    )
  }, [isSourceMatrixError, sourceMatrixError, t])

  const relationHealthParams = useMemo(
    () => ({
      limit: 500,
      cursor: relationCursor > 0 ? relationCursor : undefined,
    }),
    [relationCursor]
  )
  const {
    data: relationHealth,
    error: relationHealthError,
    isError: isRelationHealthError,
    isFetching: isRelationHealthFetching,
    refetch: refetchRelationHealth,
  } = useQuery({
    queryKey: mcpQueryKeys.billingEventsRelationHealth(relationHealthParams),
    queryFn: async () => {
      const result = await getBillingEventRelationHealth(relationHealthParams)
      if (!result.success) {
        throw mcpQueryError(
          result.message,
          'Failed to load audit relation health'
        )
      }
      return result.data
    },
    enabled: isAdmin,
    refetchInterval: 60 * 1000,
    staleTime: 30 * 1000,
  })

  useEffect(() => {
    if (!isRelationHealthError) return
    toast.error(
      mcpQueryErrorMessage(
        relationHealthError,
        t('Failed to load audit relation health')
      )
    )
  }, [isRelationHealthError, relationHealthError, t])

  const {
    data: relationInspection,
    error: relationInspectionError,
    isError: isRelationInspectionError,
    isFetching: isRelationInspectionFetching,
    refetch: refetchRelationInspection,
  } = useQuery({
    queryKey: mcpQueryKeys.billingEventsRelationInspection(),
    queryFn: async () => {
      const result = await getBillingEventRelationInspection()
      if (!result.success) {
        throw mcpQueryError(
          result.message,
          'Failed to load scheduled inspection settings'
        )
      }
      return result.data
    },
    enabled: isAdmin,
    refetchInterval: 60 * 1000,
    staleTime: 30 * 1000,
  })

  useEffect(() => {
    if (!isRelationInspectionError) return
    toast.error(
      mcpQueryErrorMessage(
        relationInspectionError,
        t('Failed to load scheduled inspection settings')
      )
    )
  }, [isRelationInspectionError, relationInspectionError, t])

  const relationInspectionRunsParams = useMemo(
    () => ({
      p: inspectionRunsPage,
      page_size: inspectionRunsPageSize,
    }),
    [inspectionRunsPage, inspectionRunsPageSize]
  )
  const {
    data: relationInspectionRuns,
    error: relationInspectionRunsError,
    isError: isRelationInspectionRunsError,
    isFetching: isRelationInspectionRunsFetching,
    refetch: refetchRelationInspectionRuns,
  } = useQuery({
    queryKey: mcpQueryKeys.billingEventsRelationInspectionRuns(
      relationInspectionRunsParams
    ),
    queryFn: async () => {
      const result = await listBillingEventRelationInspectionRuns(
        relationInspectionRunsParams
      )
      if (!result.success) {
        throw mcpQueryError(
          result.message,
          'Failed to load inspection history'
        )
      }
      return result.data
    },
    enabled: isAdmin,
    placeholderData: (previousData) => previousData,
    refetchInterval: 60 * 1000,
    staleTime: 30 * 1000,
  })

  useEffect(() => {
    if (!isRelationInspectionRunsError) return
    toast.error(
      mcpQueryErrorMessage(
        relationInspectionRunsError,
        t('Failed to load inspection history')
      )
    )
  }, [isRelationInspectionRunsError, relationInspectionRunsError, t])

  useEffect(() => {
    if (relationInspectionDirty) return
    setRelationInspectionForm(
      relationInspectionFormFromStatus(relationInspection)
    )
  }, [relationInspection, relationInspectionDirty])

  const relationInspectionRunTotal = relationInspectionRuns?.total ?? 0
  const relationInspectionRunPageCount = Math.max(
    1,
    Math.ceil(relationInspectionRunTotal / inspectionRunsPageSize)
  )
  useEffect(() => {
    if (inspectionRunsPage > relationInspectionRunPageCount) {
      setInspectionRunsPage(relationInspectionRunPageCount)
    }
  }, [inspectionRunsPage, relationInspectionRunPageCount])

  const refreshBillingEventState = () => {
    void queryClient.invalidateQueries({ queryKey: mcpQueryKeys.billingEvents() })
  }

  const saveRelationInspectionSettings = (cursor?: number) =>
    updateBillingEventRelationInspection({
      ...relationInspectionForm,
      interval_minutes: normalizePositiveInteger(
        relationInspectionForm.interval_minutes,
        60
      ),
      limit: normalizePositiveInteger(relationInspectionForm.limit, 500),
      max_auto_backfill: normalizePositiveInteger(
        relationInspectionForm.max_auto_backfill,
        200
      ),
      max_auto_cleanup_orphans: normalizePositiveInteger(
        relationInspectionForm.max_auto_cleanup_orphans,
        100
      ),
      cursor,
    })

  const relationInspectionSaveMutation = useMutation({
    mutationFn: (cursor?: number) => saveRelationInspectionSettings(cursor),
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(
          result.message || t('Failed to save scheduled inspection settings')
        )
        return
      }
      setRelationInspectionDirty(false)
      setRelationInspectionForm(
        relationInspectionFormFromStatus(result.data)
      )
      toast.success(t('Scheduled inspection settings saved'))
      refreshBillingEventState()
    },
  })

  const relationInspectionRunMutation = useMutation({
    mutationFn: runBillingEventRelationInspection,
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to run scheduled inspection'))
        return
      }
      const run = result.data
      if (run?.status === 'running') {
        toast.info(t('Scheduled inspection is already running'))
        refreshBillingEventState()
        return
      }
      toast.success(
        t('Scheduled inspection completed: {{message}}', {
          message: run?.message || '-',
        })
      )
      refreshBillingEventState()
    },
  })

  const relationBackfillPreviewMutation = useMutation({
    mutationFn: () =>
      backfillBillingEventRelations({
        limit: relationHealthParams.limit,
        cursor: relationCursor,
        dry_run: true,
      }),
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to preview audit relations'))
        return
      }
      const preview = result.data
      toast.success(
        t('Audit relation preview completed: {{count}} would create', {
          count: formatNumber(preview?.would_create ?? 0),
        })
      )
    },
  })

  const relationBackfillMutation = useMutation({
    mutationFn: () =>
      backfillBillingEventRelations({
        limit: relationHealthParams.limit,
        cursor: relationCursor,
        dry_run: false,
      }),
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to backfill audit relations'))
        return
      }
      const backfill = result.data
      toast.success(
        t('Audit relations backfilled: {{count}} created', {
          count: formatNumber(backfill?.created ?? 0),
        })
      )
      refreshBillingEventState()
    },
  })

  const relationOrphanCleanupMutation = useMutation({
    mutationFn: () => cleanupBillingEventRelationOrphans({ dry_run: false }),
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to clean audit relation orphans'))
        return
      }
      const cleanup = result.data
      toast.success(
        t('Audit relation orphans cleaned: {{count}} deleted', {
          count: formatNumber(cleanup?.deleted ?? 0),
        })
      )
      refreshBillingEventState()
    },
  })

  const openToolCall = (event: BillingEvent) => {
    const mcpCall = event.related_mcp_tool_call
    if (!mcpCall) return
    void navigate({
      to: '/mcp/$section',
      params: { section: 'tool-calls' },
      search: (prev) => ({
        ...prev,
        callsPage: undefined,
        callsStartTime: undefined,
        callsEndTime: undefined,
        callStatus: mcpCall.status ? [mcpCall.status] : undefined,
        requestId: mcpCall.request_id || event.request_id,
        sessionId: mcpCall.bridge_session_id || undefined,
        targetClient: mcpCall.target_client || undefined,
        toolName: mcpCall.tool_name,
      }),
    })
  }

  const columns = useBillingEventColumns({
    onOpenDetail: setDetail,
    onOpenRelation: setRelationDetail,
    onOpenToolCall: openToolCall,
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
      <div className='mb-4'>
        <BillingSummaryPanel
          summary={billingSummary}
          isLoading={isBillingSummaryLoading}
          isFetching={isBillingSummaryFetching}
        />
      </div>
      {isAdmin && (
        <div className='mb-4'>
          <BillingHealthPanel
            health={health}
            sourceMatrix={sourceMatrix}
            relationHealth={relationHealth}
            relationInspection={relationInspection}
            relationInspectionRuns={relationInspectionRuns}
            relationInspectionForm={relationInspectionForm}
            isLoading={isHealthLoading}
            isFetching={isHealthFetching}
            isSourceMatrixFetching={isSourceMatrixFetching}
            isRelationFetching={isRelationHealthFetching}
            isInspectionFetching={isRelationInspectionFetching}
            isInspectionRunsFetching={isRelationInspectionRunsFetching}
            isSavingInspection={relationInspectionSaveMutation.isPending}
            isRunningInspection={relationInspectionRunMutation.isPending}
            isPreviewingRelations={relationBackfillPreviewMutation.isPending}
            isBackfillingRelations={relationBackfillMutation.isPending}
            isCleaningRelationOrphans={relationOrphanCleanupMutation.isPending}
            inspectionRunsPage={inspectionRunsPage}
            inspectionRunsPageSize={inspectionRunsPageSize}
            onInspectionFormChange={(form) => {
              setRelationInspectionDirty(true)
              setRelationInspectionForm(form)
            }}
            onInspectionRunsPageChange={setInspectionRunsPage}
            onInspectionRunsPageSizeChange={(pageSize) => {
              setInspectionRunsPageSize(pageSize)
              setInspectionRunsPage(1)
            }}
            onRefresh={() => {
              void refetchHealth()
              void refetchSourceMatrix()
              void refetchRelationHealth()
              void refetchRelationInspection()
              void refetchRelationInspectionRuns()
            }}
            onPreviewRelationBackfill={() =>
              relationBackfillPreviewMutation.mutate()
            }
            onBackfillRelations={() => relationBackfillMutation.mutate()}
            onCleanRelationOrphans={() => relationOrphanCleanupMutation.mutate()}
            onSaveInspection={() => relationInspectionSaveMutation.mutate(undefined)}
            onRunInspection={() => relationInspectionRunMutation.mutate()}
            onResetInspectionCursor={() =>
              relationInspectionSaveMutation.mutate(0)
            }
            onNextRelationPage={() => {
              if (relationHealth?.next_cursor) {
                setRelationCursor(relationHealth.next_cursor)
              }
            }}
            onResetRelationCursor={() => setRelationCursor(0)}
            onOpenBackfill={() => setBackfillOpen(true)}
          />
        </div>
      )}
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No Billing Events Found')}
        emptyDescription={t(
          'No billing events available. Settled usage events will appear here.'
        )}
        skeletonKeyPrefix='billing-events-skeleton'
        tableClassName='overflow-x-auto'
        toolbarProps={{
          searchPlaceholder: t('Filter by keyword...'),
          filters: [
            {
              columnId: 'source',
              title: t('Source'),
              options: billingEventSourceOptions.map((option) => ({
                label: t(option.labelKey),
                value: option.value,
              })),
              singleSelect: true,
            },
            {
              columnId: 'event_type',
              title: t('Type'),
              options: billingEventTypeOptions.map((option) => ({
                label: t(option.labelKey),
                value: option.value,
              })),
              singleSelect: true,
            },
            {
              columnId: 'status',
              title: t('Status'),
              options: billingEventStatusOptions.map((option) => ({
                label: t(option.labelKey),
                value: option.value,
              })),
              singleSelect: true,
            },
            {
              columnId: 'billing_source',
              title: t('Billing Source'),
              options: billingSourceOptions.map((option) => ({
                label: t(option.labelKey),
                value: option.value,
              })),
              singleSelect: true,
            },
            {
              columnId: 'usage_kind',
              title: t('Usage Kind'),
              options: billingUsageKindOptions.map((option) => ({
                label: t(option.labelKey),
                value: option.value,
              })),
              singleSelect: true,
            },
          ],
          additionalSearch: (
            <>
              <TimeRangeFilter
                pageKey='billingPage'
                startKey='billingStartTime'
                endKey='billingEndTime'
              />
              <FilterInput
                table={table}
                columnId='request_id'
                placeholder={t('Request ID...')}
              />
              <FilterInput
                table={table}
                columnId='source_id'
                placeholder={t('Source ID...')}
              />
              <FilterInput
                table={table}
                columnId='token_id'
                placeholder={t('Token ID...')}
              />
            </>
          ),
          hasAdditionalFilters:
            search.billingStartTime != null || search.billingEndTime != null,
          preActions: isAdmin ? (
            <Button
              variant='outline'
              size='sm'
              className='gap-1.5'
              onClick={() => setBackfillOpen(true)}
            >
              <RotateCcw className='size-4' />
              <span className='hidden sm:inline'>
                {t('Backfill Billing Events')}
              </span>
              <span className='sr-only'>{t('Backfill Billing Events')}</span>
            </Button>
          ) : null,
          onReset: () => {
            void navigate({
              search: (prev) => ({
                ...prev,
                billingPage: undefined,
                billingStartTime: undefined,
                billingEndTime: undefined,
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
      <BillingEventRelationDialog
        open={relationDetail != null}
        event={relationDetail?.event ?? null}
        mode={relationDetail?.mode ?? null}
        onOpenChange={(open) => !open && setRelationDetail(null)}
      />
      <BillingEventBackfillDialog
        open={backfillOpen}
        onOpenChange={setBackfillOpen}
      />
    </>
  )
}
