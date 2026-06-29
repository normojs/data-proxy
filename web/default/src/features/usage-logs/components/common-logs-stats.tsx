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
import type { ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { formatLogQuota, formatTokenVolume } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { Skeleton } from '@/components/ui/skeleton'
import { getLogStats, getUserLogStats } from '../api'
import { DEFAULT_LOG_STATS } from '../constants'
import { buildApiParams } from '../lib/utils'
import { useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

function StatBadge(props: {
  label: string
  value: string | number
  detail?: ReactNode
  accent: string
}) {
  return (
    <span className='border-border/60 bg-muted/25 inline-flex min-h-7 flex-wrap items-center gap-x-2 gap-y-0.5 rounded-md border px-2.5 py-1 text-xs shadow-xs'>
      <span className={cn('h-3.5 w-0.5 rounded-full', props.accent)} />
      <span className='text-muted-foreground'>{props.label}</span>
      <span className='text-foreground/85 font-mono font-semibold tabular-nums'>
        {props.value}
      </span>
      {props.detail && (
        <span className='text-muted-foreground/70 inline-flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5 font-mono tabular-nums'>
          {props.detail}
        </span>
      )}
    </span>
  )
}

function formatTokenUnitCosts(quota: number, tokens: number): string | null {
  if (tokens <= 0 || quota <= 0) return null
  const costFor = (tokenVolume: number) =>
    formatLogQuota((quota / tokens) * tokenVolume)
  return `1M: ${costFor(1_000_000)} · 100M: ${costFor(100_000_000)}`
}

export function CommonLogsStats() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const searchParams = route.useSearch()
  const { sensitiveVisible } = useUsageLogsContext()

  const { data: stats, isLoading } = useQuery({
    queryKey: ['usage-logs-stats', isAdmin, searchParams],
    queryFn: async () => {
      const params = buildApiParams({
        page: 1,
        pageSize: 1,
        searchParams,
        columnFilters: [],
        isAdmin,
      })

      const result = isAdmin
        ? await getLogStats(params)
        : await getUserLogStats(params)

      return result.success
        ? result.data || DEFAULT_LOG_STATS
        : DEFAULT_LOG_STATS
    },
    placeholderData: (previousData) => previousData,
  })

  if (isLoading) {
    return (
      <div className='flex items-center gap-2'>
        <Skeleton className='h-7 w-[150px] rounded-md' />
        <Skeleton className='h-7 w-[100px] rounded-md' />
        <Skeleton className='h-7 w-[120px] rounded-md' />
      </div>
    )
  }

  const quota = stats?.quota || 0
  const tokens = stats?.tokens || 0
  const tokenUnitCosts = formatTokenUnitCosts(quota, tokens)
  const usageDetail =
    sensitiveVisible && tokenUnitCosts ? (
      <>
        <span>
          {formatTokenVolume(tokens)} {t('tokens')}
        </span>
        <span className='text-muted-foreground/45'>·</span>
        <span className='text-muted-foreground'>{t('Token Unit Cost')}</span>
        <span>{tokenUnitCosts}</span>
      </>
    ) : sensitiveVisible ? (
      `${formatTokenVolume(tokens)} ${t('tokens')}`
    ) : undefined

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <StatBadge
        label={t('Usage')}
        value={sensitiveVisible ? formatLogQuota(quota) : '••••'}
        detail={usageDetail}
        accent='bg-sky-500/70'
      />
      <StatBadge
        label={t('RPM')}
        value={stats?.rpm || 0}
        accent='bg-rose-500/65'
      />
      <StatBadge
        label={t('TPM')}
        value={formatTokenVolume(stats?.tpm || 0)}
        accent='bg-slate-400/70'
      />
    </div>
  )
}
