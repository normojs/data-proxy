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
import { Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Activity, AlertTriangle, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { getServiceStatusSummary } from './api'
import { getOverallStatus, getStatusMeta } from './status-meta'
import type { ServiceStatusData, ServiceStatusKind } from './types'

const SERVICE_STATUS_WINDOW_HOURS = 24

const STATUS_KEYS: ServiceStatusKind[] = [
  'normal',
  'degraded',
  'outage',
  'low_confidence',
  'no_traffic',
  'only_probe',
  'unknown',
]

function statusCount(data: ServiceStatusData | undefined, status: string) {
  if (!data) return 0
  return Number(data.summary[status as keyof typeof data.summary] ?? 0)
}

function buildIndicatorLabel(
  data: ServiceStatusData | undefined,
  status: ServiceStatusKind,
  t: ReturnType<typeof useTranslation>['t']
) {
  if (!data) return t('Service Status')

  const { summary } = data
  if (summary.outage > 0) {
    return t('Service Status · {{count}} outage', {
      count: summary.outage,
    })
  }
  if (summary.degraded > 0) {
    return t('Service Status · {{count}} degraded', {
      count: summary.degraded,
    })
  }
  if (summary.low_confidence > 0) {
    return t('Service Status · low confidence')
  }
  if (summary.normal > 0) {
    return t('Service Status · normal')
  }
  return t('Service Status · {{status}}', {
    status: t(getStatusMeta(status).shortLabelKey),
  })
}

export function ServiceStatusIndicator() {
  const { t } = useTranslation()
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
  const meta = getStatusMeta(overallStatus)
  const StatusIcon = statusQuery.isError ? AlertTriangle : meta.icon
  const label = useMemo(
    () => buildIndicatorLabel(data, overallStatus, t),
    [data, overallStatus, t]
  )
  const activeAlerts = data?.summary.active_alerts ?? 0

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='sm'
            className='inline-flex max-w-[13.5rem] gap-1.5 px-2'
          />
        }
      >
        <span
          className={cn(
            'inline-flex size-2 rounded-full',
            statusQuery.isError ? 'bg-warning' : meta.dotClassName
          )}
          aria-hidden='true'
        />
        <StatusIcon
          className={cn(
            'size-3.5',
            statusQuery.isError ? 'text-warning' : meta.textClassName
          )}
          aria-hidden='true'
        />
        <span className='hidden truncate xl:inline'>{label}</span>
        {statusQuery.isFetching && (
          <RefreshCw
            className='text-muted-foreground size-3 animate-spin'
            aria-hidden='true'
          />
        )}
      </PopoverTrigger>
      <PopoverContent align='end' sideOffset={8} className='w-80'>
        <PopoverHeader>
          <PopoverTitle className='flex items-center gap-2'>
            <Activity className='text-muted-foreground size-4' />
            {t('Service Status')}
          </PopoverTitle>
          <PopoverDescription>
            {t(
              'Based on real traffic in the last {{hours}} hours. Active probes and connectivity checks are not connected yet.',
              { hours: SERVICE_STATUS_WINDOW_HOURS }
            )}
          </PopoverDescription>
        </PopoverHeader>

        {statusQuery.isError ? (
          <div className='bg-warning/10 text-warning rounded-lg px-3 py-2 text-xs'>
            {t('Service status is temporarily unavailable.')}
          </div>
        ) : (
          <div className='grid grid-cols-2 gap-1.5'>
            {STATUS_KEYS.map((status) => {
              const itemMeta = getStatusMeta(status)
              const ItemIcon = itemMeta.icon
              const count = statusCount(data, status)
              return (
                <div
                  key={status}
                  className='bg-muted/30 flex items-center gap-2 rounded-lg px-2.5 py-2'
                >
                  <ItemIcon className={cn('size-3.5', itemMeta.textClassName)} />
                  <span className='text-muted-foreground min-w-0 flex-1 truncate text-xs'>
                    {t(itemMeta.shortLabelKey)}
                  </span>
                  <span className='font-mono text-xs font-semibold tabular-nums'>
                    {count}
                  </span>
                </div>
              )
            })}
          </div>
        )}

        {activeAlerts > 0 && (
          <div className='bg-destructive/10 text-destructive flex items-start gap-2 rounded-lg px-3 py-2 text-xs'>
            <AlertTriangle className='mt-0.5 size-3.5 shrink-0' />
            <span>
              {t('Admin alert: {{count}} channel(s) require attention.', {
                count: activeAlerts,
              })}
            </span>
          </div>
        )}

        <div className='flex items-center justify-between gap-2 border-t pt-2'>
          <span className='text-muted-foreground text-xs'>
            {data?.updated_at
              ? t('Updated {{time}}', {
                  time: new Date(data.updated_at * 1000).toLocaleTimeString(),
                })
              : t('No update time')}
          </span>
          <Button
            variant='outline'
            size='sm'
            render={<Link to='/status' />}
            className='h-7'
          >
            {t('View Details')}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
