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
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowRight, FileSearch, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { getRequestDiagnosticCandidates } from '../api'
import type { RequestDiagnosticCandidate } from '../types'

interface RequestDiagnosticCandidatesDialogProps {
  startTimestamp?: number
  endTimestamp?: number
  subsiteId?: number
  onSelectRequest: (requestId: string) => void
}

type CandidateFilters = {
  severity: string
  source: string
  modelName: string
  channelId: string
}

const ALL_FILTER_VALUE = 'all'

function diagnosticSeverityVariant(
  severity?: string
): StatusBadgeProps['variant'] {
  switch (severity) {
    case 'error':
      return 'danger'
    case 'warning':
      return 'warning'
    case 'ok':
      return 'success'
    default:
      return 'neutral'
  }
}

function diagnosticSeverityLabel(
  severity: string | undefined,
  t: (key: string) => string
): string {
  switch (severity) {
    case 'error':
      return t('Error')
    case 'warning':
      return t('Warning')
    case 'ok':
      return t('OK')
    default:
      return t('Info')
  }
}

function candidateSourceLabel(
  source: string,
  t: (key: string) => string
): string {
  switch (source) {
    case 'log_error':
      return t('Error Log')
    case 'trace_meta':
      return t('Conversion Trace')
    case 'channel_failover':
      return t('Channel Failover')
    case 'capture':
      return t('Request Capture')
    default:
      return source
  }
}

function formatCandidateSources(
  source: string,
  t: (key: string) => string
): string {
  return source
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => candidateSourceLabel(item, t))
    .join(' · ')
}

function CandidateRow(props: {
  candidate: RequestDiagnosticCandidate
  onSelect: (requestId: string) => void
}) {
  const { t } = useTranslation()
  const candidate = props.candidate
  return (
    <button
      type='button'
      className={cn(
        'group bg-background hover:bg-muted/60 focus-visible:ring-ring w-full min-w-0 rounded-md border p-2.5 text-left transition-colors outline-none focus-visible:ring-2',
        candidate.severity === 'error' && 'border-red-200 dark:border-red-900'
      )}
      onClick={() => props.onSelect(candidate.request_id)}
    >
      <div className='flex min-w-0 items-start justify-between gap-2'>
        <div className='min-w-0 space-y-1'>
          <div className='flex min-w-0 flex-wrap items-center gap-1.5'>
            <StatusBadge
              label={diagnosticSeverityLabel(candidate.severity, t)}
              variant={diagnosticSeverityVariant(candidate.severity)}
              size='sm'
              copyable={false}
            />
            <span className='min-w-0 font-mono text-xs break-all'>
              {candidate.request_id}
            </span>
          </div>
          <p className='text-muted-foreground text-xs break-words'>
            {candidate.summary || t('No summary')}
          </p>
        </div>
        <ArrowRight
          className='text-muted-foreground group-hover:text-foreground mt-1 size-3.5 shrink-0'
          aria-hidden='true'
        />
      </div>

      <div className='text-muted-foreground mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[11px]'>
        <span>
          {t('Last Seen')}:{' '}
          {formatTimestampToDate(candidate.last_seen_at, 'seconds')}
        </span>
        <span>
          {t('Errors')}: {candidate.error_count}
        </span>
        <span>
          {t('Consumes')}: {candidate.consume_count}
        </span>
        {candidate.model_name && (
          <span className='font-mono'>{candidate.model_name}</span>
        )}
        {candidate.channel_id && (
          <span>
            {t('Channel')}: {candidate.channel_id}
          </span>
        )}
        {candidate.subsite_id !== undefined && candidate.subsite_id > 0 && (
          <span>
            {t('Subsite')}: {candidate.subsite_id}
          </span>
        )}
        {candidate.username && (
          <span>
            {t('User')}: {candidate.username}
          </span>
        )}
        {candidate.source && (
          <span>
            {t('Source')}: {formatCandidateSources(candidate.source, t)}
          </span>
        )}
      </div>
    </button>
  )
}

export function RequestDiagnosticCandidatesDialog(
  props: RequestDiagnosticCandidatesDialogProps
) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [filters, setFilters] = useState<CandidateFilters>({
    severity: ALL_FILTER_VALUE,
    source: ALL_FILTER_VALUE,
    modelName: '',
    channelId: '',
  })
  const normalizedChannelId = Number.parseInt(filters.channelId.trim(), 10)
  const hasFilters =
    filters.severity !== ALL_FILTER_VALUE ||
    filters.source !== ALL_FILTER_VALUE ||
    filters.modelName.trim() !== '' ||
    filters.channelId.trim() !== ''
  const candidatesQuery = useQuery({
    queryKey: [
      'request-diagnostic-candidates',
      props.startTimestamp,
      props.endTimestamp,
      props.subsiteId,
      filters.severity,
      filters.source,
      filters.modelName,
      filters.channelId,
      normalizedChannelId,
    ],
    queryFn: () =>
      getRequestDiagnosticCandidates({
        limit: 20,
        start_timestamp: props.startTimestamp,
        end_timestamp: props.endTimestamp,
        subsite_id: props.subsiteId,
        severity:
          filters.severity === ALL_FILTER_VALUE ? undefined : filters.severity,
        source:
          filters.source === ALL_FILTER_VALUE ? undefined : filters.source,
        model_name: filters.modelName.trim() || undefined,
        channel_id:
          Number.isFinite(normalizedChannelId) && normalizedChannelId > 0
            ? normalizedChannelId
            : undefined,
      }),
    enabled: open,
    staleTime: 15_000,
  })
  const response = candidatesQuery.data
  const errorMessage = candidatesQuery.isError
    ? t('Diagnostic candidates unavailable')
    : response && !response.success
      ? response.message || t('Diagnostic candidates unavailable')
      : undefined
  const items = response?.success ? response.data?.items || [] : []

  const handleSelect = (requestId: string) => {
    props.onSelectRequest(requestId)
    setOpen(false)
  }

  const clearFilters = () => {
    setFilters({
      severity: ALL_FILTER_VALUE,
      source: ALL_FILTER_VALUE,
      modelName: '',
      channelId: '',
    })
  }

  return (
    <>
      <Button
        type='button'
        variant='outline'
        size='sm'
        className='h-8 gap-1.5'
        onClick={() => setOpen(true)}
      >
        <FileSearch className='size-3.5' aria-hidden='true' />
        {t('Diagnostic Candidates')}
      </Button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2 text-base'>
              <FileSearch className='size-4' aria-hidden='true' />
              {t('Diagnostic Candidates')}
            </DialogTitle>
            <DialogDescription>
              {t('Find suspicious requests in the current time range.')}
            </DialogDescription>
          </DialogHeader>

          <div className='flex items-center justify-between gap-2'>
            <p className='text-muted-foreground text-xs'>
              {t('Click a request to filter logs by Request ID.')}
            </p>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              className='h-8 gap-1.5'
              onClick={() => candidatesQuery.refetch()}
              disabled={candidatesQuery.isFetching}
            >
              <RefreshCw
                className={cn(
                  'size-3.5',
                  candidatesQuery.isFetching && 'animate-spin'
                )}
                aria-hidden='true'
              />
              {t('Refresh')}
            </Button>
          </div>

          <div className='bg-muted/30 grid gap-2 rounded-md border p-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1.2fr)_7rem_auto] sm:items-center'>
            <Select
              value={filters.severity}
              onValueChange={(value) =>
                setFilters((current) => ({
                  ...current,
                  severity: value ?? ALL_FILTER_VALUE,
                }))
              }
            >
              <SelectTrigger className='h-8 text-xs'>
                <SelectValue placeholder={t('Severity')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL_FILTER_VALUE}>
                    {t('All severities')}
                  </SelectItem>
                  <SelectItem value='error'>{t('Error')}</SelectItem>
                  <SelectItem value='warning'>{t('Warning')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
            <Select
              value={filters.source}
              onValueChange={(value) =>
                setFilters((current) => ({
                  ...current,
                  source: value ?? ALL_FILTER_VALUE,
                }))
              }
            >
              <SelectTrigger className='h-8 text-xs'>
                <SelectValue placeholder={t('Source')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL_FILTER_VALUE}>
                    {t('All sources')}
                  </SelectItem>
                  <SelectItem value='log_error'>{t('Error Log')}</SelectItem>
                  <SelectItem value='trace_meta'>
                    {t('Conversion Trace')}
                  </SelectItem>
                  <SelectItem value='channel_failover'>
                    {t('Channel Failover')}
                  </SelectItem>
                  <SelectItem value='capture'>
                    {t('Request Capture')}
                  </SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
            <Input
              value={filters.modelName}
              onChange={(event) =>
                setFilters((current) => ({
                  ...current,
                  modelName: event.target.value,
                }))
              }
              placeholder={t('Model')}
              className='h-8 text-xs'
            />
            <Input
              value={filters.channelId}
              onChange={(event) =>
                setFilters((current) => ({
                  ...current,
                  channelId: event.target.value.replace(/[^\d]/g, ''),
                }))
              }
              inputMode='numeric'
              placeholder={t('Channel ID')}
              className='h-8 text-xs'
            />
            <Button
              type='button'
              variant='ghost'
              size='sm'
              className='h-8 px-2 text-xs'
              onClick={clearFilters}
              disabled={!hasFilters}
            >
              {t('Clear')}
            </Button>
          </div>

          <ScrollArea className='max-h-[60vh] pr-3'>
            {candidatesQuery.isLoading ? (
              <div className='space-y-2'>
                {Array.from({ length: 4 }).map((_, index) => (
                  <Skeleton key={index} className='h-20 rounded-md' />
                ))}
              </div>
            ) : errorMessage ? (
              <div className='rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600 dark:border-red-900 dark:bg-red-950/20 dark:text-red-400'>
                {errorMessage}
              </div>
            ) : items.length === 0 ? (
              <div className='text-muted-foreground rounded-md border border-dashed p-6 text-center text-sm'>
                {t('No diagnostic candidates')}
              </div>
            ) : (
              <div className='space-y-2'>
                {items.map((candidate) => (
                  <CandidateRow
                    key={candidate.request_id}
                    candidate={candidate}
                    onSelect={handleSelect}
                  />
                ))}
              </div>
            )}
          </ScrollArea>
        </DialogContent>
      </Dialog>
    </>
  )
}
