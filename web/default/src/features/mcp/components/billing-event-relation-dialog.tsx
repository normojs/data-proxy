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
import { useMemo } from 'react'
import { Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  formatQuota,
  formatTimestampToDate,
} from '@/lib/format'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { StatusBadge } from '@/components/status-badge'
import type {
  BillingEvent,
  BillingEventAuditLink,
  BillingEventTargetLink,
} from '../types'

type BillingEventRelationDialogProps = {
  event: BillingEvent | null
  mode: 'audit' | 'target' | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

type BillingEventAuditMetadata = {
  admin_id?: number
  reason?: string
  reconciliation_source?: string
  label?: string
  target_event_id?: string
  created_event_id?: string
  diffs?: Array<{ field?: string; expected?: string; actual?: string }>
}

function parseMetadata(metadata: string | undefined): BillingEventAuditMetadata {
  if (!metadata) return {}
  try {
    const parsed = JSON.parse(metadata)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as BillingEventAuditMetadata
    }
  } catch {
    return {}
  }
  return {}
}

function formatJsonValue(value: unknown): string {
  try {
    return JSON.stringify(value ?? {}, null, 2)
  } catch {
    return String(value ?? '')
  }
}

function RelationField(props: { label: string; value: unknown }) {
  const value = props.value
  if (value == null || value === '') return null
  return (
    <div className='grid gap-1 sm:grid-cols-[140px_1fr]'>
      <span className='text-muted-foreground'>{props.label}</span>
      <span className='break-all'>{String(value)}</span>
    </div>
  )
}

function TargetEventSummary(props: { target: BillingEventTargetLink }) {
  const { t } = useTranslation()
  const target = props.target

  return (
    <div className='rounded-lg border p-3'>
      <div className='mb-2 flex flex-wrap items-center gap-2'>
        <StatusBadge label={target.source} variant='neutral' copyable={false} />
        <StatusBadge
          label={target.event_type}
          variant={target.event_type === 'credit' ? 'success' : 'info'}
          copyable={false}
        />
        <StatusBadge label={target.status} variant='success' copyable={false} />
      </div>
      <div className='space-y-1 text-xs'>
        <RelationField label='ID' value={`#${target.id}`} />
        <RelationField label={t('Event ID')} value={target.event_id} />
        <RelationField label={t('User')} value={target.user_id} />
        <RelationField label={t('Source ID')} value={target.source_id} />
        <RelationField label={t('Price Unit')} value={target.price_unit} />
        <RelationField
          label={t('Amount')}
          value={formatQuota(target.amount_quota)}
        />
        <RelationField
          label={t('Quota Delta')}
          value={formatQuota(target.quota_delta)}
        />
        <RelationField
          label={t('Created At')}
          value={formatTimestampToDate(target.created_at)}
        />
      </div>
    </div>
  )
}

function AuditLinkSummary(props: { audit: BillingEventAuditLink }) {
  const { t } = useTranslation()
  const audit = props.audit

  return (
    <div className='rounded-lg border p-3'>
      <div className='mb-2 flex flex-wrap items-center gap-2'>
        <StatusBadge label={audit.price_unit} variant='info' copyable={false} />
        <StatusBadge
          label={`Admin #${audit.admin_id}`}
          variant='neutral'
          copyable={false}
        />
      </div>
      <div className='space-y-1 text-xs'>
        <RelationField label='ID' value={`#${audit.id}`} />
        <RelationField label={t('Event ID')} value={audit.event_id} />
        <RelationField label={t('Source ID')} value={audit.source_id} />
        <RelationField label={t('Label')} value={audit.label} />
        <RelationField label={t('Reason')} value={audit.reason} />
        <RelationField
          label={t('Created At')}
          value={formatTimestampToDate(audit.created_at)}
        />
      </div>
    </div>
  )
}

function AuditDiffs(props: { metadata: BillingEventAuditMetadata }) {
  const { t } = useTranslation()
  const diffs = props.metadata.diffs ?? []
  if (diffs.length === 0) return null

  return (
    <div className='rounded-lg border p-3'>
      <div className='mb-2 text-sm font-medium'>{t('Repaired Fields')}</div>
      <div className='space-y-1 text-xs'>
        {diffs.map((diff, index) => (
          <div
            key={`${diff.field ?? 'field'}-${index}`}
            className='grid gap-1 sm:grid-cols-[140px_1fr]'
          >
            <span className='text-muted-foreground'>{diff.field ?? '-'}</span>
            <span className='break-all'>
              {diff.actual ?? '-'}
              {' -> '}
              {diff.expected ?? '-'}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

export function BillingEventRelationDialog(
  props: BillingEventRelationDialogProps
) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const event = props.event
  const metadata = useMemo(() => parseMetadata(event?.metadata), [event])
  const auditEvents = event?.related_audit_events ?? []
  const title =
    props.mode === 'target' ? t('Target Event') : t('Repair Audit')
  const rawValue =
    props.mode === 'target' ? event?.related_target_event : auditEvents
  const copyContent = formatJsonValue(rawValue)

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-3xl'>
        <DialogHeader>
          <div className='flex items-start justify-between gap-3 pr-8'>
            <div className='min-w-0'>
              <DialogTitle>{title}</DialogTitle>
              {event?.event_id && (
                <DialogDescription className='mt-1 break-all'>
                  {event.event_id}
                </DialogDescription>
              )}
            </div>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => void copyToClipboard(copyContent)}
            >
              <Copy className='size-3.5' />
              {t('Copy')}
            </Button>
          </div>
        </DialogHeader>

        <ScrollArea className='max-h-[65vh] pr-3'>
          {props.mode === 'target' && event?.related_target_event ? (
            <div className='space-y-3'>
              <div className='rounded-lg border p-3'>
                <div className='mb-2 text-sm font-medium'>
                  {t('Audit Summary')}
                </div>
                <div className='space-y-1 text-xs'>
                  <RelationField
                    label={t('Reason')}
                    value={metadata.reason}
                  />
                  <RelationField
                    label={t('Admin')}
                    value={
                      metadata.admin_id == null
                        ? undefined
                        : `#${metadata.admin_id}`
                    }
                  />
                  <RelationField
                    label={t('Label')}
                    value={metadata.label}
                  />
                  <RelationField
                    label={t('Reconciliation Source')}
                    value={metadata.reconciliation_source}
                  />
                </div>
              </div>
              <TargetEventSummary target={event.related_target_event} />
              <AuditDiffs metadata={metadata} />
            </div>
          ) : (
            <div className='space-y-3'>
              {auditEvents.map((audit) => (
                <AuditLinkSummary key={audit.id} audit={audit} />
              ))}
              {auditEvents.length === 0 && (
                <div className='text-muted-foreground rounded-lg border p-3 text-sm'>
                  {t('No related audit events')}
                </div>
              )}
            </div>
          )}
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
