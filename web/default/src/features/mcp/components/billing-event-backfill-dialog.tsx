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
import { useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Loader2, RotateCcw, Wrench } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota } from '@/lib/format'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { StatusBadge } from '@/components/status-badge'
import {
  backfillBillingEventReconciliationMissing,
  backfillBillingEvents,
  getBillingEventReconciliationMissing,
  getBillingEventReconciliationMismatches,
  mcpQueryKeys,
  reconcileBillingEvents,
  repairBillingEventReconciliationMismatch,
} from '../api'
import type {
  BillingEvent,
  BillingEventBackfillRequest,
  BillingEventBackfillResponse,
  BillingEventBackfillSourceResult,
  BillingEventReconciliationBackfillMissingResponse,
  BillingEventReconciliationDiff,
  BillingEventReconciliationMissingItem,
  BillingEventReconciliationMissingRequest,
  BillingEventReconciliationMissingResponse,
  BillingEventReconciliationMismatchRequest,
  BillingEventReconciliationMismatchResponse,
  BillingEventReconciliationMismatchItem,
  BillingEventReconciliationRepairResponse,
  BillingEventReconciliationRequest,
  BillingEventReconciliationResponse,
  BillingEventReconciliationSourceResult,
} from '../types'

type BillingEventBackfillDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type ReconciliationOperationResult =
  | {
      type: 'repair'
      result: BillingEventReconciliationRepairResponse
    }
  | {
      type: 'backfill_missing'
      result: BillingEventReconciliationBackfillMissingResponse
    }

type BackfillSourceOption = {
  labelKey: string
  value: string
}

const backfillSourceOptions: BackfillSourceOption[] = [
  { labelKey: 'Wallet Top-up', value: 'wallet_topup' },
  { labelKey: 'Redemption', value: 'redemption' },
  { labelKey: 'Model Request', value: 'model_request' },
  { labelKey: 'Subscription Purchase', value: 'subscription_purchase' },
  { labelKey: 'Subscription Balance', value: 'subscription_balance' },
  { labelKey: 'Subscription Admin Bind', value: 'subscription_admin_bind' },
]

const defaultBackfillSources = backfillSourceOptions.map(
  (option) => option.value
)

function normalizeLimit(value: string): number {
  const limit = Number(value)
  if (!Number.isFinite(limit) || limit <= 0) return 500
  return Math.min(5000, Math.trunc(limit))
}

function buildPayload(
  sources: string[],
  limitValue: string,
  dryRun: boolean
): BillingEventBackfillRequest {
  return {
    sources,
    limit: normalizeLimit(limitValue),
    dry_run: dryRun,
  }
}

function buildReconciliationPayload(
  sources: string[],
  limitValue: string
): BillingEventReconciliationRequest {
  return {
    sources,
    limit: normalizeLimit(limitValue),
  }
}

function buildMismatchPayload(
  sources: string[],
  limitValue: string
): BillingEventReconciliationMismatchRequest {
  return {
    sources,
    limit: normalizeLimit(limitValue),
    detail_limit: 50,
  }
}

function buildMissingPayload(
  sources: string[],
  limitValue: string
): BillingEventReconciliationMissingRequest {
  return {
    sources,
    limit: normalizeLimit(limitValue),
    detail_limit: 50,
  }
}

const operationMetadataKeys = [
  'reconciliation_repair',
  'reconciliation_backfill',
  'reason',
  'label',
  'source',
  'source_id',
  'trade_no',
  'order_id',
  'subscription_id',
  'plan_id',
  'plan_title',
  'redemption_id',
  'token_name',
  'model_name',
  'upstream_request_id',
  'payment_method',
  'payment_provider',
  'subscription_from',
  'quota_from',
  'channel',
]

function getSourceLabel(source: string, t: (key: string) => string): string {
  const option = backfillSourceOptions.find((item) => item.value === source)
  return option ? t(option.labelKey) : source
}

function parseMetadata(metadata: string | undefined): Record<string, unknown> {
  if (!metadata) return {}
  try {
    const parsed = JSON.parse(metadata)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    return {}
  }
  return {}
}

function formatOperationValue(value: unknown): string {
  if (value == null) return '-'
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  if (typeof value === 'number') return String(value)
  if (typeof value === 'string') return value || '-'
  return JSON.stringify(value)
}

function getMetadataRows(event: BillingEvent | null | undefined) {
  const metadata = parseMetadata(event?.metadata)
  return operationMetadataKeys
    .filter((key) => metadata[key] != null && metadata[key] !== '')
    .map((key) => [key, formatOperationValue(metadata[key])] as const)
}

function OperationFieldRows(props: {
  rows: ReadonlyArray<readonly [string, unknown]>
}) {
  const rows = props.rows.filter(([, value]) => value != null && value !== '')
  if (rows.length === 0) return null

  return (
    <div className='space-y-1 text-xs'>
      {rows.map(([field, value]) => (
        <div key={field} className='grid gap-1 sm:grid-cols-[136px_1fr]'>
          <span className='text-muted-foreground'>{field}</span>
          <span className='break-all'>{formatOperationValue(value)}</span>
        </div>
      ))}
    </div>
  )
}

function BillingEventSnapshot(props: {
  title: string
  event: BillingEvent | null
}) {
  const { t } = useTranslation()
  const event = props.event
  if (!event) {
    return (
      <div className='space-y-1'>
        <div className='text-sm font-medium'>{props.title}</div>
        <div className='text-muted-foreground text-xs'>{t('No event')}</div>
      </div>
    )
  }

  const rows: ReadonlyArray<readonly [string, unknown]> = [
    ['id', `#${event.id}`],
    ['event_type', event.event_type],
    ['status', event.status],
    ['source', event.source],
    ['source_id', event.source_id],
    ['price_unit', event.price_unit],
    ['currency', event.currency],
    ['amount_quota', formatQuota(event.amount_quota)],
    ['quota_delta', formatQuota(event.quota_delta)],
    ['request_id', event.request_id],
    ['billing_source', event.billing_source],
  ]
  const metadataRows = getMetadataRows(event)

  return (
    <div className='space-y-1'>
      <div className='text-sm font-medium'>{props.title}</div>
      <OperationFieldRows rows={rows} />
      {metadataRows.length > 0 && (
        <div className='pt-1'>
          <div className='text-muted-foreground mb-1 text-xs'>
            {t('Metadata')}
          </div>
          <OperationFieldRows rows={metadataRows} />
        </div>
      )}
    </div>
  )
}

function BillingEventOperationResultSummary(props: {
  result: ReconciliationOperationResult | null
}) {
  const { t } = useTranslation()
  const operation = props.result
  if (!operation) return null

  const result = operation.result
  let completed: boolean
  let diffs: BillingEventReconciliationDiff[]
  let event: BillingEvent | null
  let eventTitle: string
  let header: string

  if (operation.type === 'repair') {
    completed = operation.result.repaired
    diffs = operation.result.diffs
    event = operation.result.after
    eventTitle = t('Event After Repair')
    header = t('Last Repair Result')
  } else {
    completed = operation.result.backfilled
    diffs = []
    event = operation.result.event
    eventTitle = t('Backfilled Event')
    header = t('Last Backfill Result')
  }

  const summaryRows: ReadonlyArray<readonly [string, unknown]> = [
    ['label', result.label],
    ['source', getSourceLabel(result.source, t)],
    ['expected_source_id', result.expected.source_id],
    ['expected_phase', result.expected.phase],
    ['expected_price_unit', result.expected.price_unit],
    ['expected_currency', result.expected.currency],
  ]

  return (
    <div className='rounded-lg border p-2.5'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <Label>{header}</Label>
        <StatusBadge
          label={completed ? t('Completed') : t('No Change')}
          variant={completed ? 'success' : 'neutral'}
          copyable={false}
        />
      </div>
      <div className='mt-2 space-y-3'>
        <OperationFieldRows rows={summaryRows} />
        {diffs.length > 0 && (
          <div className='space-y-1 border-t pt-2 text-xs'>
            <div className='text-muted-foreground'>{t('Repaired Fields')}</div>
            {diffs.slice(0, 6).map((diff) => (
              <div
                key={`${result.label}-${diff.field}`}
                className='grid gap-1 sm:grid-cols-[136px_1fr]'
              >
                <span className='text-muted-foreground'>{diff.field}</span>
                <span className='break-all'>
                  {diff.actual}
                  {' -> '}
                  {diff.expected}
                </span>
              </div>
            ))}
          </div>
        )}
        <div className='grid gap-3 border-t pt-2 sm:grid-cols-2'>
          <BillingEventSnapshot title={eventTitle} event={event} />
          <BillingEventSnapshot
            title={t('Audit Event')}
            event={result.audit_event}
          />
        </div>
      </div>
    </div>
  )
}

function BackfillMetric(props: {
  label: string
  value: number
  variant?: 'quota' | 'plain'
}) {
  return (
    <div className='min-w-[96px] rounded-lg border px-2.5 py-2'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='mt-1 text-sm font-medium tabular-nums'>
        {props.variant === 'quota' ? formatQuota(props.value) : props.value}
      </div>
    </div>
  )
}

function BackfillResultRow(props: {
  result: BillingEventBackfillSourceResult
}) {
  const { t } = useTranslation()
  const result = props.result

  return (
    <div className='rounded-lg border p-2.5'>
      <div className='flex flex-wrap items-center gap-2'>
        <StatusBadge
          label={getSourceLabel(result.source, t)}
          variant='neutral'
          copyable={false}
        />
        {result.error_count > 0 && (
          <StatusBadge
            label={t('Errors {{count}}', { count: result.error_count })}
            variant='warning'
            copyable={false}
          />
        )}
      </div>
      <div className='mt-2 grid grid-cols-2 gap-2 text-sm sm:grid-cols-5'>
        <BackfillMetric label={t('Scanned')} value={result.scanned} />
        <BackfillMetric label={t('Created')} value={result.created} />
        <BackfillMetric
          label={t('Would Create')}
          value={result.would_create}
        />
        <BackfillMetric
          label={t('Skipped')}
          value={result.skipped_existing}
        />
        <BackfillMetric
          label={t('Invalid')}
          value={result.skipped_invalid}
        />
      </div>
      {result.errors.length > 0 && (
        <div className='text-muted-foreground mt-2 space-y-1 text-xs'>
          {result.errors.slice(0, 3).map((error) => (
            <div key={error} className='break-all'>
              {error}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function BackfillResultSummary(props: {
  result: BillingEventBackfillResponse | null
}) {
  const { t } = useTranslation()
  const result = props.result
  if (!result) return null

  return (
    <div className='space-y-2'>
      <div className='grid grid-cols-2 gap-2 sm:grid-cols-5'>
        <BackfillMetric label={t('Scanned')} value={result.total_scanned} />
        <BackfillMetric label={t('Created')} value={result.total_created} />
        <BackfillMetric
          label={t('Would Create')}
          value={result.total_would_create}
        />
        <BackfillMetric
          label={t('Skipped')}
          value={result.total_skipped_existing}
        />
        <BackfillMetric
          label={t('Invalid')}
          value={result.total_skipped_invalid}
        />
      </div>
      <div className='max-h-[260px] space-y-2 overflow-y-auto pr-1'>
        {result.results.map((item) => (
          <BackfillResultRow key={item.source} result={item} />
        ))}
      </div>
    </div>
  )
}

function ReconciliationResultRow(props: {
  result: BillingEventReconciliationSourceResult
}) {
  const { t } = useTranslation()
  const result = props.result
  const badgeVariant =
    result.missing > 0 ||
    result.mismatched > 0 ||
    result.invalid > 0 ||
    result.error_count > 0
      ? 'warning'
      : 'success'

  return (
    <div className='rounded-lg border p-2.5'>
      <div className='flex flex-wrap items-center gap-2'>
        <StatusBadge
          label={getSourceLabel(result.source, t)}
          variant={badgeVariant}
          copyable={false}
        />
        {result.missing > 0 && (
          <StatusBadge
            label={t('Missing {{count}}', { count: result.missing })}
            variant='warning'
            copyable={false}
          />
        )}
        {result.mismatched > 0 && (
          <StatusBadge
            label={t('Mismatched {{count}}', { count: result.mismatched })}
            variant='warning'
            copyable={false}
          />
        )}
        {result.error_count > 0 && (
          <StatusBadge
            label={t('Errors {{count}}', { count: result.error_count })}
            variant='warning'
            copyable={false}
          />
        )}
        {result.has_more && (
          <StatusBadge
            label={t('More records')}
            variant='neutral'
            copyable={false}
          />
        )}
      </div>
      <div className='mt-2 grid grid-cols-2 gap-2 text-sm sm:grid-cols-6'>
        <BackfillMetric label={t('Scanned')} value={result.scanned} />
        <BackfillMetric label={t('Expected')} value={result.expected} />
        <BackfillMetric label={t('Ledgered')} value={result.ledgered} />
        <BackfillMetric label={t('Missing')} value={result.missing} />
        <BackfillMetric label={t('Mismatched')} value={result.mismatched} />
        <BackfillMetric label={t('Invalid')} value={result.invalid} />
      </div>
      {(result.sample_missing.length > 0 ||
        result.sample_mismatched.length > 0 ||
        result.sample_invalid.length > 0 ||
        result.errors.length > 0) && (
        <div className='text-muted-foreground mt-2 space-y-1 text-xs'>
          {[
            ...result.sample_missing,
            ...result.sample_mismatched,
            ...result.sample_invalid,
            ...result.errors,
          ]
            .slice(0, 4)
            .map((sample) => (
              <div key={sample} className='break-all'>
                {sample}
              </div>
            ))}
        </div>
      )}
    </div>
  )
}

function ReconciliationResultSummary(props: {
  result: BillingEventReconciliationResponse | null
  missingDetails: BillingEventReconciliationMissingResponse | null
  details: BillingEventReconciliationMismatchResponse | null
  isLoadingMissingDetails: boolean
  isLoadingDetails: boolean
  isBackfillingMissing: boolean
  isRepairing: boolean
  onLoadMissingDetails: () => void
  onLoadDetails: () => void
  onBackfillMissing: (
    item: BillingEventReconciliationMissingItem,
    reason: string
  ) => void
  onRepair: (item: BillingEventReconciliationMismatchItem, reason: string) => void
}) {
  const { t } = useTranslation()
  const result = props.result
  if (!result) return null

  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-2'>
        <Label>{t('Reconciliation')}</Label>
        <div className='flex items-center gap-2'>
          {result.total_missing > 0 && (
            <Button
              variant='outline'
              size='sm'
              disabled={props.isLoadingMissingDetails}
              onClick={props.onLoadMissingDetails}
            >
              {props.isLoadingMissingDetails && (
                <Loader2 className='animate-spin' />
              )}
              {t('View Missing Details')}
            </Button>
          )}
          {result.total_mismatched > 0 && (
            <Button
              variant='outline'
              size='sm'
              disabled={props.isLoadingDetails}
              onClick={props.onLoadDetails}
            >
              {props.isLoadingDetails && <Loader2 className='animate-spin' />}
              {t('View Mismatch Details')}
            </Button>
          )}
          <StatusBadge
            label={
              result.has_more ||
              result.total_missing > 0 ||
              result.total_mismatched > 0 ||
              result.total_invalid > 0
                ? t('Needs Review')
                : t('Balanced')
            }
            variant={
              result.has_more ||
              result.total_missing > 0 ||
              result.total_mismatched > 0 ||
              result.total_invalid > 0
                ? 'warning'
                : 'success'
            }
            copyable={false}
          />
        </div>
      </div>
      <div className='grid grid-cols-2 gap-2 sm:grid-cols-6'>
        <BackfillMetric label={t('Scanned')} value={result.total_scanned} />
        <BackfillMetric label={t('Expected')} value={result.total_expected} />
        <BackfillMetric label={t('Ledgered')} value={result.total_ledgered} />
        <BackfillMetric label={t('Missing')} value={result.total_missing} />
        <BackfillMetric
          label={t('Mismatched')}
          value={result.total_mismatched}
        />
        <BackfillMetric label={t('Invalid')} value={result.total_invalid} />
      </div>
      {result.has_more && (
        <div className='text-muted-foreground text-xs'>
          {t('More records are pending scan')}
        </div>
      )}
      <div className='max-h-[260px] space-y-2 overflow-y-auto pr-1'>
        {result.results.map((item) => (
          <ReconciliationResultRow key={item.source} result={item} />
        ))}
      </div>
      <MissingDetailsSummary
        result={props.missingDetails}
        isBackfilling={props.isBackfillingMissing}
        onBackfill={props.onBackfillMissing}
      />
      <MismatchDetailsSummary
        result={props.details}
        isRepairing={props.isRepairing}
        onRepair={props.onRepair}
      />
    </div>
  )
}

function MismatchDetailsSummary(props: {
  result: BillingEventReconciliationMismatchResponse | null
  isRepairing: boolean
  onRepair: (item: BillingEventReconciliationMismatchItem, reason: string) => void
}) {
  const { t } = useTranslation()
  const result = props.result
  if (!result) return null

  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-2'>
        <Label>{t('Mismatch Details')}</Label>
        <StatusBadge
          label={t('Mismatched {{count}}', {
            count: result.total_mismatched,
          })}
          variant={result.total_mismatched > 0 ? 'warning' : 'success'}
          copyable={false}
        />
      </div>
      {result.items.length === 0 ? (
        <div className='text-muted-foreground rounded-lg border p-2.5 text-sm'>
          {t('No mismatch details')}
        </div>
      ) : (
        <div className='max-h-[300px] space-y-2 overflow-y-auto pr-1'>
          {result.items.map((item) => (
            <MismatchDetailsRow
              key={item.label}
              item={item}
              isRepairing={props.isRepairing}
              onRepair={props.onRepair}
            />
          ))}
        </div>
      )}
      {result.has_more && (
        <div className='text-muted-foreground text-xs'>
          {t('Only the first {{count}} mismatches are shown', {
            count: result.detail_limit,
          })}
        </div>
      )}
    </div>
  )
}

function MissingDetailsSummary(props: {
  result: BillingEventReconciliationMissingResponse | null
  isBackfilling: boolean
  onBackfill: (
    item: BillingEventReconciliationMissingItem,
    reason: string
  ) => void
}) {
  const { t } = useTranslation()
  const result = props.result
  if (!result) return null

  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-2'>
        <Label>{t('Missing Details')}</Label>
        <StatusBadge
          label={t('Missing {{count}}', {
            count: result.total_missing,
          })}
          variant={result.total_missing > 0 ? 'warning' : 'success'}
          copyable={false}
        />
      </div>
      {result.items.length === 0 ? (
        <div className='text-muted-foreground rounded-lg border p-2.5 text-sm'>
          {t('No missing details')}
        </div>
      ) : (
        <div className='max-h-[300px] space-y-2 overflow-y-auto pr-1'>
          {result.items.map((item) => (
            <MissingDetailsRow
              key={item.label}
              item={item}
              isBackfilling={props.isBackfilling}
              onBackfill={props.onBackfill}
            />
          ))}
        </div>
      )}
      {result.has_more && (
        <div className='text-muted-foreground text-xs'>
          {t('Only the first {{count}} missing events are shown', {
            count: result.detail_limit,
          })}
        </div>
      )}
    </div>
  )
}

function MissingDetailsRow(props: {
  item: BillingEventReconciliationMissingItem
  isBackfilling: boolean
  onBackfill: (
    item: BillingEventReconciliationMissingItem,
    reason: string
  ) => void
}) {
  const { t } = useTranslation()
  const item = props.item
  const [reason, setReason] = useState('')
  const trimmedReason = reason.trim()
  const expectedRows = [
    ['user_id', item.expected.user_id],
    ['event_type', item.expected.event_type],
    ['amount_quota', item.expected.amount_quota],
    ['quota_delta', item.expected.quota_delta],
    ['billing_source', item.expected.billing_source],
    ['price_unit', item.expected.price_unit],
    ['currency', item.expected.currency],
  ]

  return (
    <div className='rounded-lg border p-2.5'>
      <div className='flex flex-wrap items-center gap-2'>
        <StatusBadge
          label={getSourceLabel(item.source, t)}
          variant='warning'
          copyable={false}
        />
        <span className='break-all text-sm font-medium'>{item.label}</span>
      </div>
      <div className='mt-2 space-y-1 text-xs'>
        {expectedRows.map(([field, value]) => (
          <div
            key={`${item.label}-${field}`}
            className='grid gap-1 sm:grid-cols-[120px_1fr]'
          >
            <span className='text-muted-foreground'>{field}</span>
            <span className='break-all'>{String(value)}</span>
          </div>
        ))}
      </div>
      <div className='mt-2 grid gap-2 sm:grid-cols-[1fr_auto]'>
        <Input
          value={reason}
          disabled={props.isBackfilling}
          placeholder={t('Backfill reason')}
          onChange={(event) => setReason(event.target.value)}
        />
        <Button
          variant='outline'
          disabled={props.isBackfilling || trimmedReason.length === 0}
          onClick={() => props.onBackfill(item, trimmedReason)}
        >
          {props.isBackfilling ? (
            <Loader2 className='animate-spin' />
          ) : (
            <Wrench />
          )}
          {t('Backfill')}
        </Button>
      </div>
    </div>
  )
}

function MismatchDetailsRow(props: {
  item: BillingEventReconciliationMismatchItem
  isRepairing: boolean
  onRepair: (item: BillingEventReconciliationMismatchItem, reason: string) => void
}) {
  const { t } = useTranslation()
  const item = props.item
  const [reason, setReason] = useState('')
  const trimmedReason = reason.trim()

  return (
    <div className='rounded-lg border p-2.5'>
      <div className='flex flex-wrap items-center gap-2'>
        <StatusBadge
          label={getSourceLabel(item.source, t)}
          variant='warning'
          copyable={false}
        />
        <span className='break-all text-sm font-medium'>{item.label}</span>
        {item.actual && (
          <StatusBadge
            label={`${t('Event')} #${item.actual.id}`}
            variant='warning'
            copyable={false}
          />
        )}
      </div>
      <div className='mt-2 space-y-1 text-xs'>
        {item.diffs.map((diff) => (
          <div
            key={`${item.label}-${diff.field}`}
            className='grid gap-1 sm:grid-cols-[120px_1fr]'
          >
            <span className='text-muted-foreground'>{diff.field}</span>
            <span className='break-all'>
              {t('Expected')}: {diff.expected} / {t('Actual')}: {diff.actual}
            </span>
          </div>
        ))}
      </div>
      <div className='mt-2 grid gap-2 sm:grid-cols-[1fr_auto]'>
        <Input
          value={reason}
          disabled={props.isRepairing}
          placeholder={t('Repair reason')}
          onChange={(event) => setReason(event.target.value)}
        />
        <Button
          variant='outline'
          disabled={props.isRepairing || trimmedReason.length === 0}
          onClick={() => props.onRepair(item, trimmedReason)}
        >
          {props.isRepairing ? (
            <Loader2 className='animate-spin' />
          ) : (
            <Wrench />
          )}
          {t('Repair')}
        </Button>
      </div>
    </div>
  )
}

export function BillingEventBackfillDialog(
  props: BillingEventBackfillDialogProps
) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [sources, setSources] = useState<string[]>(defaultBackfillSources)
  const [limitValue, setLimitValue] = useState('500')
  const [preview, setPreview] = useState<BillingEventBackfillResponse | null>(
    null
  )
  const [reconciliation, setReconciliation] =
    useState<BillingEventReconciliationResponse | null>(null)
  const [missingDetails, setMissingDetails] =
    useState<BillingEventReconciliationMissingResponse | null>(null)
  const [mismatchDetails, setMismatchDetails] =
    useState<BillingEventReconciliationMismatchResponse | null>(null)
  const [lastOperationResult, setLastOperationResult] =
    useState<ReconciliationOperationResult | null>(null)

  const selectedSources = useMemo(
    () => sources.filter((source) => defaultBackfillSources.includes(source)),
    [sources]
  )
  const hasPreviewWork =
    (preview?.total_would_create ?? 0) > 0 || (preview?.total_created ?? 0) > 0

  const previewMutation = useMutation({
    mutationFn: () =>
      backfillBillingEvents(buildPayload(selectedSources, limitValue, true)),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to preview billing backfill'))
        return
      }
      setPreview(res.data)
      toast.success(t('Billing backfill preview completed'))
    },
  })

  const reconciliationMutation = useMutation({
    mutationFn: () =>
      reconcileBillingEvents(
        buildReconciliationPayload(selectedSources, limitValue)
      ),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to check billing reconciliation'))
        return
      }
      setReconciliation(res.data)
      setMissingDetails(null)
      setMismatchDetails(null)
      toast.success(t('Billing reconciliation completed'))
    },
  })

  const missingDetailsMutation = useMutation({
    mutationFn: () =>
      getBillingEventReconciliationMissing(
        buildMissingPayload(selectedSources, limitValue)
      ),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to load missing details'))
        return
      }
      setMissingDetails(res.data)
      toast.success(t('Missing details loaded'))
    },
  })

  const mismatchDetailsMutation = useMutation({
    mutationFn: () =>
      getBillingEventReconciliationMismatches(
        buildMismatchPayload(selectedSources, limitValue)
      ),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to load mismatch details'))
        return
      }
      setMismatchDetails(res.data)
      toast.success(t('Mismatch details loaded'))
    },
  })

  const repairMutation = useMutation({
    mutationFn: (input: {
      item: BillingEventReconciliationMismatchItem
      reason: string
    }) =>
      repairBillingEventReconciliationMismatch({
        source: input.item.source,
        label: input.item.label,
        limit: normalizeLimit(limitValue),
        reason: input.reason,
        actual_id: input.item.actual?.id,
        expected: input.item.expected,
      }),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to repair mismatch'))
        return
      }
      setReconciliation(null)
      setMissingDetails(null)
      setMismatchDetails(null)
      setLastOperationResult({ type: 'repair', result: res.data })
      toast.success(t('Mismatch repaired; run reconciliation again'))
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.billingEvents() })
    },
  })

  const backfillMissingMutation = useMutation({
    mutationFn: (input: {
      item: BillingEventReconciliationMissingItem
      reason: string
    }) =>
      backfillBillingEventReconciliationMissing({
        source: input.item.source,
        label: input.item.label,
        limit: normalizeLimit(limitValue),
        reason: input.reason,
        expected: input.item.expected,
      }),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to backfill missing event'))
        return
      }
      setReconciliation(null)
      setMissingDetails(null)
      setMismatchDetails(null)
      setLastOperationResult({ type: 'backfill_missing', result: res.data })
      toast.success(t('Missing event backfilled; run reconciliation again'))
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.billingEvents() })
    },
  })

  const executeMutation = useMutation({
    mutationFn: () =>
      backfillBillingEvents(buildPayload(selectedSources, limitValue, false)),
    onSuccess: (res) => {
      if (!res.success || !res.data) {
        toast.error(res.message || t('Failed to execute billing backfill'))
        return
      }
      setPreview(res.data)
      setReconciliation(null)
      setMissingDetails(null)
      setMismatchDetails(null)
      setLastOperationResult(null)
      toast.success(t('Billing backfill completed'))
      queryClient.invalidateQueries({ queryKey: mcpQueryKeys.billingEvents() })
    },
  })

  const isBusy =
    previewMutation.isPending ||
    reconciliationMutation.isPending ||
    missingDetailsMutation.isPending ||
    mismatchDetailsMutation.isPending ||
    repairMutation.isPending ||
    backfillMissingMutation.isPending ||
    executeMutation.isPending

  const toggleSource = (source: string, checked: boolean) => {
    setPreview(null)
    setReconciliation(null)
    setMissingDetails(null)
    setMismatchDetails(null)
    setLastOperationResult(null)
    setSources((current) =>
      checked
        ? Array.from(new Set([...current, source]))
        : current.filter((item) => item !== source)
    )
  }

  const handleOpenChange = (open: boolean) => {
    if (!open && isBusy) return
    props.onOpenChange(open)
  }

  return (
    <Dialog open={props.open} onOpenChange={handleOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-hidden sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Backfill Billing Events')}</DialogTitle>
        </DialogHeader>

        <div className='space-y-4 overflow-y-auto pr-1'>
          <Alert>
            <RotateCcw className='size-4' />
            <AlertTitle>{t('Historical ledger backfill')}</AlertTitle>
            <AlertDescription>
              {t(
                'Run a dry-run first, then execute only missing settled usage events. Existing ledger entries and balances are not changed.'
              )}
            </AlertDescription>
          </Alert>

          <div className='grid gap-3 sm:grid-cols-[1fr_140px]'>
            <div className='space-y-2'>
              <Label>{t('Sources')}</Label>
              <div className='grid gap-2 sm:grid-cols-2'>
                {backfillSourceOptions.map((option) => (
                  <label
                    key={option.value}
                    className='border-input flex items-center gap-2 rounded-lg border px-2.5 py-2 text-sm'
                  >
                    <Checkbox
                      checked={selectedSources.includes(option.value)}
                      onCheckedChange={(checked) =>
                        toggleSource(option.value, checked === true)
                      }
                    />
                    <span>{t(option.labelKey)}</span>
                  </label>
                ))}
              </div>
            </div>

            <div className='space-y-2'>
              <Label htmlFor='billing-backfill-limit'>{t('Batch Limit')}</Label>
              <Input
                id='billing-backfill-limit'
                type='number'
                min={1}
                max={5000}
                value={limitValue}
                onChange={(event) => {
                  setPreview(null)
                  setReconciliation(null)
                  setMissingDetails(null)
                  setMismatchDetails(null)
                  setLastOperationResult(null)
                  setLimitValue(event.target.value)
                }}
              />
            </div>
          </div>

          <ReconciliationResultSummary
            result={reconciliation}
            missingDetails={missingDetails}
            details={mismatchDetails}
            isLoadingMissingDetails={missingDetailsMutation.isPending}
            isLoadingDetails={mismatchDetailsMutation.isPending}
            isBackfillingMissing={backfillMissingMutation.isPending}
            isRepairing={repairMutation.isPending}
            onLoadMissingDetails={() => missingDetailsMutation.mutate()}
            onLoadDetails={() => mismatchDetailsMutation.mutate()}
            onBackfillMissing={(item, reason) =>
              backfillMissingMutation.mutate({ item, reason })
            }
            onRepair={(item, reason) => repairMutation.mutate({ item, reason })}
          />
          <BillingEventOperationResultSummary result={lastOperationResult} />
          <BackfillResultSummary result={preview} />
        </div>

        <DialogFooter>
          <Button
            variant='outline'
            disabled={isBusy}
            onClick={() => handleOpenChange(false)}
          >
            {t('Close')}
          </Button>
          <Button
            variant='outline'
            disabled={isBusy || selectedSources.length === 0}
            onClick={() => reconciliationMutation.mutate()}
          >
            {reconciliationMutation.isPending && (
              <Loader2 className='animate-spin' />
            )}
            {t('Check Reconciliation')}
          </Button>
          <Button
            variant='outline'
            disabled={isBusy || selectedSources.length === 0}
            onClick={() => previewMutation.mutate()}
          >
            {previewMutation.isPending && <Loader2 className='animate-spin' />}
            {t('Dry Run')}
          </Button>
          <Button
            disabled={isBusy || selectedSources.length === 0 || !hasPreviewWork}
            onClick={() => executeMutation.mutate()}
          >
            {executeMutation.isPending && <Loader2 className='animate-spin' />}
            {t('Execute Backfill')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
