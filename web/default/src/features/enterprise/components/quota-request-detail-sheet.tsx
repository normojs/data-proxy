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
import { type ReactNode } from 'react'
import {
  ClipboardList,
  FileText,
  Gauge,
  TimerReset,
  UserRound,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import type {
  EnterpriseQuotaRequest,
  EnterpriseQuotaRequestStatus,
} from '../types'

function formatNumber(value: number | undefined) {
  return new Intl.NumberFormat().format(value ?? 0)
}

function formatDateTime(value: number | undefined) {
  if (!value) return '-'
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value * 1000))
}

function formatMetric(metric: string) {
  return metric === 'request_count' ? 'Requests' : 'Quota'
}

function formatPeriod(period: string) {
  return period === 'month' ? 'Monthly' : 'Daily'
}

function formatQuotaRequestStatus(status: EnterpriseQuotaRequestStatus) {
  switch (status) {
    case 'approved':
      return 'Approved'
    case 'rejected':
      return 'Rejected'
    case 'withdrawn':
      return 'Withdrawn'
    case 'expired':
      return 'Expired'
    default:
      return 'Pending'
  }
}

function formatTarget(request: EnterpriseQuotaRequest) {
  if (request.target_type === 'enterprise') return 'Enterprise'
  return request.target_name || `${request.target_type} #${request.target_id}`
}

function formatUser(name: string, id: number) {
  if (name) return name
  return id > 0 ? `user:${id}` : '-'
}

function quotaRequestAuditActions(request: EnterpriseQuotaRequest) {
  const actions = ['quota_request.submit']
  if (request.status === 'approved') actions.push('quota_request.approve')
  if (request.status === 'rejected') actions.push('quota_request.reject')
  if (request.status === 'withdrawn') actions.push('quota_request.withdraw')
  if (request.status === 'expired') actions.push('quota_request.expire')
  return actions
}

export function QuotaRequestStatusBadge(props: {
  status: EnterpriseQuotaRequestStatus
}) {
  const { t } = useTranslation()
  const className = {
    pending: 'border-amber-500/40 text-amber-700',
    approved: 'border-emerald-500/40 text-emerald-700',
    rejected: 'border-destructive/40 text-destructive',
    withdrawn: 'text-muted-foreground',
    expired: 'text-muted-foreground',
  }[props.status]

  return (
    <Badge variant='outline' className={className}>
      {t(formatQuotaRequestStatus(props.status))}
    </Badge>
  )
}

function DetailField(props: {
  label: string
  value: ReactNode
  mono?: boolean
  className?: string
}) {
  const { t } = useTranslation()
  return (
    <div className={cn('min-w-0 space-y-1', props.className)}>
      <div className='text-muted-foreground text-xs'>{t(props.label)}</div>
      <div
        className={cn(
          'text-sm font-medium break-words',
          props.mono && 'font-mono text-xs break-all'
        )}
      >
        {props.value}
      </div>
    </div>
  )
}

function DetailTextBlock(props: {
  label: string
  value: string
  empty: string
}) {
  const { t } = useTranslation()
  const value = props.value.trim()

  return (
    <div className='space-y-2'>
      <div className='text-muted-foreground text-xs'>{t(props.label)}</div>
      <div className='bg-muted/20 min-h-16 rounded-md border px-3 py-2 text-sm leading-6 break-words whitespace-pre-wrap'>
        {value || t(props.empty)}
      </div>
    </div>
  )
}

export function QuotaRequestDetailSheet(props: {
  open: boolean
  request: EnterpriseQuotaRequest | null
  canViewAudit?: boolean
  onViewAudit?: (request: EnterpriseQuotaRequest) => void
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const request = props.request
  const actions = request ? quotaRequestAuditActions(request) : []

  return (
    <Sheet
      open={props.open && Boolean(request)}
      onOpenChange={props.onOpenChange}
    >
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[580px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <div className='flex min-w-0 items-start justify-between gap-3 pr-8'>
            <div className='min-w-0'>
              <SheetTitle>{t('Quota Request Details')}</SheetTitle>
              <SheetDescription className='mt-1 font-mono text-xs break-all'>
                {request ? `quota_request #${request.id}` : '-'}
              </SheetDescription>
            </div>
            {request ? (
              <QuotaRequestStatusBadge status={request.status} />
            ) : null}
          </div>
        </SheetHeader>

        {request ? (
          <div className={sideDrawerFormClassName('gap-5')}>
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Request Context')}
                description={t('Policy, target, applicant, and quota window.')}
                icon={<Gauge className='size-4' />}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <DetailField
                  label='Policy'
                  value={request.policy_name || `#${request.policy_id}`}
                />
                <DetailField
                  label='Policy ID'
                  value={`#${request.policy_id || '-'}`}
                  mono
                />
                <DetailField label='Target' value={formatTarget(request)} />
                <DetailField
                  label='Target ID'
                  value={`${request.target_type} #${request.target_id || '-'}`}
                  mono
                />
                <DetailField
                  label='Metric'
                  value={t(formatMetric(request.metric))}
                />
                <DetailField
                  label='Period'
                  value={t(formatPeriod(request.period))}
                />
                <DetailField
                  label='Extra Limit'
                  value={formatNumber(request.limit_delta)}
                />
                <DetailField
                  label='Applicant'
                  value={formatUser(
                    request.applicant_name,
                    request.applicant_user_id
                  )}
                />
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Reason')}
                description={t('Requester input and reviewer decision.')}
                icon={<FileText className='size-4' />}
              />
              <div className='space-y-4'>
                <DetailTextBlock
                  label='Request Reason'
                  value={request.reason}
                  empty='No reason provided'
                />
                <DetailTextBlock
                  label='Decision Reason'
                  value={request.decision_reason}
                  empty='No decision reason recorded'
                />
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Decision')}
                description={t('Reviewer and lifecycle timestamps.')}
                icon={<UserRound className='size-4' />}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <DetailField
                  label='Reviewer'
                  value={formatUser(
                    request.approver_name,
                    request.approver_user_id
                  )}
                />
                <DetailField
                  label='Decided At'
                  value={formatDateTime(request.decided_at)}
                />
                <DetailField
                  label='Effective At'
                  value={formatDateTime(request.effective_at)}
                />
                <DetailField
                  label='Expires At'
                  value={formatDateTime(request.expires_at)}
                />
                <DetailField
                  label='Created At'
                  value={formatDateTime(request.created_at)}
                />
                <DetailField
                  label='Updated At'
                  value={formatDateTime(request.updated_at)}
                />
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Audit Trail')}
                description={t(
                  'Recorded lifecycle actions for this quota request.'
                )}
                icon={<ClipboardList className='size-4' />}
              />
              <div className='space-y-3'>
                <div className='grid gap-3 sm:grid-cols-2'>
                  <DetailField
                    label='Audit Target'
                    value={`quota_request #${request.id}`}
                    mono
                  />
                  <DetailField
                    label='Current Status'
                    value={t(formatQuotaRequestStatus(request.status))}
                  />
                </div>
                <div className='space-y-2'>
                  {actions.map((action, index) => (
                    <div
                      key={action}
                      className='flex min-w-0 items-center gap-3 text-sm'
                    >
                      <span className='bg-muted text-muted-foreground flex size-6 shrink-0 items-center justify-center rounded-md text-xs tabular-nums'>
                        {index + 1}
                      </span>
                      <span className='min-w-0 font-mono text-xs break-all'>
                        {action}
                      </span>
                    </div>
                  ))}
                </div>
                {props.canViewAudit && props.onViewAudit ? (
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => props.onViewAudit?.(request)}
                  >
                    <TimerReset className='size-3.5' />
                    {t('View Audit')}
                  </Button>
                ) : null}
              </div>
            </SideDrawerSection>
          </div>
        ) : null}
      </SheetContent>
    </Sheet>
  )
}
