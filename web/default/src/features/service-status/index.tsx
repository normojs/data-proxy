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
import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  AlertTriangle,
  BarChart3,
  Clock3,
  Eye,
  Radio,
  RefreshCw,
  Server,
  ShieldAlert,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getLobeIcon } from '@/lib/lobe-icon'
import { formatNumber, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import {
  getChannelTypeIcon,
  getChannelTypeLabel,
} from '@/features/channels/lib/channel-utils'
import { getServiceStatusSummary } from './api'
import {
  getOverallStatus,
  getSignalClassName,
  getSignalIcon,
  getSignalLabelKey,
  getStatusMeta,
} from './status-meta'
import type {
  ServiceSignalState,
  ServiceStatusChannel,
  ServiceStatusData,
  ServiceStatusKind,
} from './types'

const SERVICE_STATUS_WINDOW_HOURS = 24

const SUMMARY_ITEMS: Array<{
  key: keyof ServiceStatusData['summary']
  status: ServiceStatusKind
  labelKey: string
}> = [
  { key: 'normal', status: 'normal', labelKey: 'Normal' },
  { key: 'degraded', status: 'degraded', labelKey: 'Degraded' },
  { key: 'outage', status: 'outage', labelKey: 'Outage' },
  { key: 'low_confidence', status: 'low_confidence', labelKey: 'Low confidence' },
  { key: 'no_traffic', status: 'no_traffic', labelKey: 'No traffic' },
  { key: 'active_alerts', status: 'outage', labelKey: 'Alerts' },
]

const SIGNALS: Array<{
  key: keyof ServiceStatusChannel['signals']
  labelKey: string
}> = [
  { key: 'real_traffic', labelKey: 'Real traffic' },
  { key: 'probe', labelKey: 'Active probe' },
  { key: 'connectivity', labelKey: 'Connectivity' },
]

function formatRate(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return `${formatNumber(value)}%`
}

function formatDuration(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '-'
  if (value >= 1000) return `${(value / 1000).toFixed(2)}s`
  return `${Math.round(value)}ms`
}

function statusVariant(status: ServiceStatusKind) {
  switch (status) {
    case 'normal':
      return 'success'
    case 'degraded':
    case 'low_confidence':
      return 'warning'
    case 'outage':
      return 'danger'
    case 'only_probe':
      return 'info'
    default:
      return 'neutral'
  }
}

function confidenceLabel(confidence: ServiceStatusChannel['confidence']) {
  if (confidence === 'high') return 'High confidence'
  if (confidence === 'low') return 'Low confidence'
  return 'No confidence'
}

function getSeriesTitle(point: ServiceStatusChannel['series'][number]) {
  return [
    formatTimestampToDate(point.ts),
    point.request_count,
    formatRate(point.success_rate),
    formatDuration(point.avg_latency_ms),
  ].join(' · ')
}

function SummaryCell(props: {
  label: string
  value: number
  status: ServiceStatusKind
}) {
  const meta = getStatusMeta(props.status)
  const Icon = meta.icon
  return (
    <div className='bg-card rounded-xl border px-3 py-3'>
      <div className='text-muted-foreground flex items-center gap-1.5 text-xs font-medium'>
        <Icon className={cn('size-3.5', meta.textClassName)} />
        <span className='truncate'>{props.label}</span>
      </div>
      <div className='mt-2 font-mono text-xl font-semibold tabular-nums'>
        {formatNumber(props.value)}
      </div>
    </div>
  )
}

function ChannelIdentity(props: { channel: ServiceStatusChannel }) {
  const { t } = useTranslation()
  const typeNameKey = getChannelTypeLabel(props.channel.channel_type)
  const iconName = getChannelTypeIcon(props.channel.channel_type)
  return (
    <div className='flex min-w-0 items-center gap-2'>
      <span className='flex size-7 shrink-0 items-center justify-center rounded-lg border bg-background'>
        {getLobeIcon(`${iconName}.Color`, 16)}
      </span>
      <div className='min-w-0'>
        <div className='truncate text-sm font-medium'>
          {props.channel.channel_name || `#${props.channel.channel_id}`}
        </div>
        <div className='text-muted-foreground mt-0.5 flex items-center gap-1.5 text-xs'>
          <span className='truncate'>{t(typeNameKey)}</span>
          <span aria-hidden='true'>/</span>
          <span className='truncate'>{props.channel.group || 'default'}</span>
        </div>
      </div>
    </div>
  )
}

function SeriesStrip(props: { channel: ServiceStatusChannel }) {
  const points = props.channel.series.slice(-24)
  if (points.length === 0) {
    return (
      <div className='bg-muted/30 text-muted-foreground flex h-7 w-44 items-center justify-center rounded-md text-xs'>
        -
      </div>
    )
  }

  return (
    <div className='flex h-7 w-44 items-end gap-0.5' aria-label='24h status'>
      {points.map((point) => {
        const meta = getStatusMeta(point.status)
        return (
          <span
            key={point.ts}
            className={cn('min-w-1 flex-1 rounded-sm', meta.dotClassName)}
            style={{
              height: `${Math.max(8, Math.min(28, point.request_count + 8))}px`,
            }}
            title={getSeriesTitle(point)}
          />
        )
      })}
    </div>
  )
}

function SignalBadge(props: { state: ServiceSignalState; label: string }) {
  const Icon = getSignalIcon(props.state)
  return (
    <span className='bg-muted/40 inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-xs'>
      <Icon className={cn('size-3.5', getSignalClassName(props.state))} />
      <span className='text-muted-foreground'>{props.label}</span>
    </span>
  )
}

function ServiceStatusSkeleton() {
  return (
    <div className='space-y-3'>
      <div className='grid grid-cols-2 gap-2 md:grid-cols-3 xl:grid-cols-6'>
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton key={i} className='h-24 rounded-xl' />
        ))}
      </div>
      <Skeleton className='h-[28rem] rounded-xl' />
    </div>
  )
}

function ServiceStatusEmpty() {
  const { t } = useTranslation()
  return (
    <div className='bg-card flex min-h-64 flex-col items-center justify-center rounded-xl border px-4 py-10 text-center'>
      <Server className='text-muted-foreground size-8' />
      <h3 className='mt-3 text-sm font-semibold'>{t('No channels observed')}</h3>
      <p className='text-muted-foreground mt-1 max-w-xl text-sm'>
        {t(
          'Service status appears after channels receive real traffic. Active probes and connectivity checks are not connected yet.'
        )}
      </p>
    </div>
  )
}

function ChannelDetailSheet(props: {
  channel: ServiceStatusChannel | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const channel = props.channel

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className='w-full overflow-y-auto sm:max-w-2xl'>
        <SheetHeader className='border-b'>
          <SheetTitle>
            {channel?.channel_name || t('Channel details')}
          </SheetTitle>
          <SheetDescription>
            {t(
              'Observation is based on real traffic only. Active probes and connectivity checks are not connected yet.'
            )}
          </SheetDescription>
        </SheetHeader>
        {channel && (
          <div className='space-y-4 px-4 pb-4'>
            <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
              <SummaryCell
                label={t('Requests')}
                value={channel.request_count}
                status={channel.status}
              />
              <SummaryCell
                label={t('Success rate')}
                value={channel.success_rate}
                status={channel.status}
              />
              <SummaryCell
                label={t('Average latency')}
                value={channel.avg_latency_ms}
                status={channel.status}
              />
              <SummaryCell
                label={t('Average TTFT')}
                value={channel.avg_ttft_ms}
                status={channel.status}
              />
            </div>

            <div className='rounded-xl border'>
              <div className='flex items-center gap-2 border-b px-3 py-2 text-sm font-medium'>
                <Radio className='text-muted-foreground size-4' />
                {t('Signal sources')}
              </div>
              <div className='flex flex-wrap gap-2 p-3'>
                {SIGNALS.map((signal) => (
                  <SignalBadge
                    key={signal.key}
                    label={`${t(signal.labelKey)} · ${t(
                      getSignalLabelKey(channel.signals[signal.key])
                    )}`}
                    state={channel.signals[signal.key]}
                  />
                ))}
              </div>
            </div>

            <div className='rounded-xl border'>
              <div className='flex items-center gap-2 border-b px-3 py-2 text-sm font-medium'>
                <BarChart3 className='text-muted-foreground size-4' />
                {t('24h status history')}
              </div>
              <div className='space-y-1.5 p-3'>
                {channel.series.length > 0 ? (
                  channel.series.map((point) => {
                    const meta = getStatusMeta(point.status)
                    const Icon = meta.icon
                    return (
                      <div
                        key={point.ts}
                        className='bg-muted/20 grid grid-cols-[minmax(0,1.3fr)_repeat(4,minmax(0,0.8fr))] items-center gap-2 rounded-lg px-2.5 py-2 text-xs'
                      >
                        <span className='text-muted-foreground truncate'>
                          {formatTimestampToDate(point.ts)}
                        </span>
                        <span className='inline-flex items-center gap-1.5'>
                          <Icon className={cn('size-3.5', meta.textClassName)} />
                          {t(meta.shortLabelKey)}
                        </span>
                        <span className='font-mono tabular-nums'>
                          {formatNumber(point.request_count)}
                        </span>
                        <span className='font-mono tabular-nums'>
                          {formatRate(point.success_rate)}
                        </span>
                        <span className='font-mono tabular-nums'>
                          {formatDuration(point.avg_latency_ms)}
                        </span>
                      </div>
                    )
                  })
                ) : (
                  <div className='text-muted-foreground py-4 text-center text-sm'>
                    {t('No trend data')}
                  </div>
                )}
              </div>
            </div>

            <div className='rounded-xl border'>
              <div className='flex items-center gap-2 border-b px-3 py-2 text-sm font-medium'>
                <Server className='text-muted-foreground size-4' />
                {t('Models')}
              </div>
              <div className='grid gap-3 p-3 md:grid-cols-2'>
                <div>
                  <div className='text-muted-foreground mb-2 text-xs font-medium'>
                    {t('Configured models')}
                  </div>
                  <div className='flex flex-wrap gap-1.5'>
                    {channel.configured_models.length > 0 ? (
                      channel.configured_models.map((model) => (
                        <StatusBadge
                          key={model}
                          label={model}
                          variant='neutral'
                          copyable
                        />
                      ))
                    ) : (
                      <span className='text-muted-foreground text-sm'>-</span>
                    )}
                  </div>
                </div>
                <div>
                  <div className='text-muted-foreground mb-2 text-xs font-medium'>
                    {t('Observed models')}
                  </div>
                  <div className='flex flex-wrap gap-1.5'>
                    {channel.observed_models.length > 0 ? (
                      channel.observed_models.map((model) => (
                        <StatusBadge
                          key={model}
                          label={model}
                          variant='info'
                          copyable
                        />
                      ))
                    ) : (
                      <span className='text-muted-foreground text-sm'>-</span>
                    )}
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}

export function ServiceStatus() {
  const { t } = useTranslation()
  const [selectedChannel, setSelectedChannel] =
    useState<ServiceStatusChannel | null>(null)
  const statusQuery = useQuery({
    queryKey: ['service-status-summary', SERVICE_STATUS_WINDOW_HOURS],
    queryFn: async () => {
      const result = await getServiceStatusSummary(SERVICE_STATUS_WINDOW_HOURS)
      if (!result.success) {
        throw new Error(result.message || 'Failed to load service status')
      }
      return result.data
    },
    retry: false,
    staleTime: 60 * 1000,
    refetchInterval: 60 * 1000,
  })

  const data = statusQuery.data
  const overallStatus = getOverallStatus(data?.summary)
  const overallMeta = getStatusMeta(overallStatus)
  const OverallIcon = overallMeta.icon
  const sortedChannels = useMemo(() => {
    const order: Record<ServiceStatusKind, number> = {
      outage: 0,
      degraded: 1,
      low_confidence: 2,
      normal: 3,
      no_traffic: 4,
      only_probe: 5,
      unknown: 6,
    }
    return [...(data?.channels ?? [])].sort((a, b) => {
      const statusDelta = order[a.status] - order[b.status]
      if (statusDelta !== 0) return statusDelta
      return b.request_count - a.request_count
    })
  }, [data])

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Service Status')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          size='sm'
          onClick={() => void statusQuery.refetch()}
          disabled={statusQuery.isFetching}
        >
          <RefreshCw
            className={cn('size-3.5', statusQuery.isFetching && 'animate-spin')}
          />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-3 sm:space-y-4'>
          <div className='rounded-xl border bg-card px-4 py-3'>
            <div className='flex flex-wrap items-start justify-between gap-3'>
              <div className='min-w-0'>
                <div className='flex items-center gap-2 text-sm font-semibold'>
                  <OverallIcon
                    className={cn('size-4', overallMeta.textClassName)}
                  />
                  {t(overallMeta.labelKey)}
                </div>
                <p className='text-muted-foreground mt-1 max-w-3xl text-sm'>
                  {t(
                    'Based on real traffic in the last {{hours}} hours. Active probes and connectivity checks are not connected yet.',
                    { hours: SERVICE_STATUS_WINDOW_HOURS }
                  )}
                </p>
              </div>
              <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
                <Clock3 className='size-3.5' />
                {data?.updated_at
                  ? t('Updated {{time}}', {
                      time: formatTimestampToDate(data.updated_at),
                    })
                  : t('No update time')}
              </div>
            </div>
          </div>

          {statusQuery.isLoading ? (
            <ServiceStatusSkeleton />
          ) : statusQuery.isError ? (
            <div className='bg-warning/10 text-warning flex items-start gap-2 rounded-xl border border-warning/20 px-4 py-3 text-sm'>
              <AlertTriangle className='mt-0.5 size-4 shrink-0' />
              <span>{t('Service status is temporarily unavailable.')}</span>
            </div>
          ) : !data || sortedChannels.length === 0 ? (
            <ServiceStatusEmpty />
          ) : (
            <>
              <div className='grid grid-cols-2 gap-2 md:grid-cols-3 xl:grid-cols-6'>
                {SUMMARY_ITEMS.map((item) => (
                  <SummaryCell
                    key={item.key}
                    label={t(item.labelKey)}
                    value={Number(data.summary[item.key] ?? 0)}
                    status={item.status}
                  />
                ))}
              </div>

              {data.alerts && data.alerts.length > 0 && (
                <div className='bg-destructive/10 text-destructive rounded-xl border border-destructive/20 px-4 py-3'>
                  <div className='flex items-center gap-2 text-sm font-medium'>
                    <ShieldAlert className='size-4' />
                    {t('Admin alerts')}
                  </div>
                  <div className='mt-2 grid gap-2 md:grid-cols-2 xl:grid-cols-3'>
                    {data.alerts.slice(0, 6).map((alert) => (
                      <div
                        key={alert.channel_id}
                        className='bg-background/70 rounded-lg px-3 py-2 text-xs'
                      >
                        <div className='truncate font-medium'>
                          {alert.channel_name}
                        </div>
                        <div className='mt-1 flex flex-wrap gap-x-3 gap-y-1 tabular-nums'>
                          <span>{formatRate(alert.success_rate)}</span>
                          <span>{formatDuration(alert.avg_latency_ms)}</span>
                          <span>{formatNumber(alert.request_count)}</span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <div className='overflow-hidden rounded-xl border bg-card'>
                <div className='flex flex-wrap items-center justify-between gap-2 border-b px-3 py-2.5'>
                  <div className='flex items-center gap-2 text-sm font-medium'>
                    <Activity className='text-muted-foreground size-4' />
                    {t('Channel observations')}
                  </div>
                  <div className='text-muted-foreground text-xs'>
                    {t('{{count}} channels', {
                      count: sortedChannels.length,
                    })}
                  </div>
                </div>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Channel')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead className='text-right'>{t('Requests')}</TableHead>
                      <TableHead className='text-right'>
                        {t('Success rate')}
                      </TableHead>
                      <TableHead className='text-right'>
                        {t('Average latency')}
                      </TableHead>
                      <TableHead className='text-right'>
                        {t('Average TTFT')}
                      </TableHead>
                      <TableHead>{t('Last observed')}</TableHead>
                      <TableHead>{t('24h trend')}</TableHead>
                      <TableHead className='text-right'>{t('Actions')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {sortedChannels.map((channel) => (
                      <TableRow key={channel.channel_id}>
                        <TableCell className='min-w-64'>
                          <ChannelIdentity channel={channel} />
                        </TableCell>
                        <TableCell>
                          <div className='flex flex-col items-start gap-1'>
                            <StatusBadge
                              icon={getStatusMeta(channel.status).icon}
                              variant={statusVariant(channel.status)}
                              copyable={false}
                              size='lg'
                            >
                              {t(getStatusMeta(channel.status).labelKey)}
                            </StatusBadge>
                            <span className='text-muted-foreground text-xs'>
                              {t(confidenceLabel(channel.confidence))}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell className='text-right font-mono tabular-nums'>
                          {formatNumber(channel.request_count)}
                        </TableCell>
                        <TableCell className='text-right font-mono tabular-nums'>
                          {formatRate(channel.success_rate)}
                        </TableCell>
                        <TableCell className='text-right font-mono tabular-nums'>
                          {formatDuration(channel.avg_latency_ms)}
                        </TableCell>
                        <TableCell className='text-right font-mono tabular-nums'>
                          {formatDuration(channel.avg_ttft_ms)}
                        </TableCell>
                        <TableCell className='text-muted-foreground text-xs'>
                          {formatTimestampToDate(channel.last_observed_at)}
                        </TableCell>
                        <TableCell>
                          <SeriesStrip channel={channel} />
                        </TableCell>
                        <TableCell className='text-right'>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            onClick={() => setSelectedChannel(channel)}
                          >
                            <Eye className='size-3.5' />
                            {t('Details')}
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </>
          )}
        </div>
        <ChannelDetailSheet
          channel={selectedChannel}
          open={selectedChannel != null}
          onOpenChange={(open) => {
            if (!open) setSelectedChannel(null)
          }}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
