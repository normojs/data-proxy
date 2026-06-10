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
import {
  Activity,
  CreditCard,
  History,
  RefreshCw,
  Server,
  Wrench,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatNumber, formatQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge } from '@/components/status-badge'
import { getMCPProxyTrends, mcpQueryKeys } from '../api'
import { normalizeMCPProxyTrendResponse } from '../lib/proxy-trends'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type {
  MCPProxyTrendResponse,
  MCPProxyTrendServerDimension,
  MCPProxyTrendToolDimension,
} from '../types'

const PROXY_TREND_WINDOW_SECONDS = 14 * 24 * 60 * 60
const PROXY_TREND_BUCKET_SECONDS = 24 * 60 * 60

type ProxyTrendRange = {
  startTime: number
  endTime: number
}

type ProxyTrendsPanelProps = {
  onOpenServerBilling?: (
    server: MCPProxyTrendServerDimension,
    range: ProxyTrendRange
  ) => void
  onOpenServerCalls?: (
    server: MCPProxyTrendServerDimension,
    range: ProxyTrendRange
  ) => void
  onOpenServerTools?: (server: MCPProxyTrendServerDimension) => void
  onOpenToolBilling?: (
    tool: MCPProxyTrendToolDimension,
    range: ProxyTrendRange
  ) => void
  onOpenToolCalls?: (
    tool: MCPProxyTrendToolDimension,
    range: ProxyTrendRange
  ) => void
}

function formatRate(value: number): string {
  return `${formatNumber(value)}%`
}

function formatDuration(value: number): string {
  if (!value) return '-'
  return `${formatNumber(value)} ms`
}

function TrendMetric(props: {
  label: string
  value: string
  tone?: 'success' | 'warning'
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

function ServerRow(
  props: {
    server: MCPProxyTrendServerDimension
    range: ProxyTrendRange
  } & Pick<
    ProxyTrendsPanelProps,
    'onOpenServerBilling' | 'onOpenServerCalls' | 'onOpenServerTools'
  >
) {
  const { t } = useTranslation()
  const server = props.server
  const hasActions =
    props.onOpenServerBilling ||
    props.onOpenServerCalls ||
    props.onOpenServerTools
  return (
    <div className='bg-muted/20 rounded-lg border px-3 py-2'>
      <div className='flex min-w-0 items-center justify-between gap-2'>
        <div className='min-w-0'>
          <div className='truncate text-xs font-medium'>{server.name}</div>
          <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
            {server.namespace}
          </div>
        </div>
        <StatusBadge
          label={`${formatNumber(server.total_calls)} ${t('Calls')}`}
          variant='info'
          copyable={false}
        />
      </div>
      <div className='text-muted-foreground mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs tabular-nums'>
        <span>
          {t('Success')} {formatRate(server.success_rate)}
        </span>
        <span>
          {t('Quota Used')} {formatQuota(server.quota)}
        </span>
      </div>
      {hasActions && (
        <div className='mt-2 flex flex-wrap gap-1.5'>
          {props.onOpenServerTools && (
            <Button
              type='button'
              variant='outline'
              size='xs'
              onClick={() => props.onOpenServerTools?.(server)}
            >
              <Wrench className='size-3' />
              {t('Tools')}
            </Button>
          )}
          {props.onOpenServerCalls && (
            <Button
              type='button'
              variant='outline'
              size='xs'
              onClick={() => props.onOpenServerCalls?.(server, props.range)}
            >
              <History className='size-3' />
              {t('Calls')}
            </Button>
          )}
          {props.onOpenServerBilling && (
            <Button
              type='button'
              variant='outline'
              size='xs'
              onClick={() => props.onOpenServerBilling?.(server, props.range)}
            >
              <CreditCard className='size-3' />
              {t('Billing')}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}

function ToolRow(
  props: {
    tool: MCPProxyTrendToolDimension
    range: ProxyTrendRange
  } & Pick<ProxyTrendsPanelProps, 'onOpenToolBilling' | 'onOpenToolCalls'>
) {
  const { t } = useTranslation()
  const tool = props.tool
  const hasActions = props.onOpenToolBilling || props.onOpenToolCalls
  return (
    <div className='bg-muted/20 rounded-lg border px-3 py-2'>
      <div className='min-w-0'>
        <div className='truncate font-mono text-xs font-medium'>
          {tool.exposed_tool_name}
        </div>
        <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
          {tool.downstream_tool_name}
        </div>
      </div>
      <div className='text-muted-foreground mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs tabular-nums'>
        <span>
          {t('Calls')} {formatNumber(tool.total_calls)}
        </span>
        <span>
          {t('Errors')} {formatNumber(tool.error_calls + tool.timeout_calls)}
        </span>
        <span>
          {t('Avg')} {formatDuration(tool.avg_duration_ms)}
        </span>
      </div>
      {hasActions && (
        <div className='mt-2 flex flex-wrap gap-1.5'>
          {props.onOpenToolCalls && (
            <Button
              type='button'
              variant='outline'
              size='xs'
              onClick={() => props.onOpenToolCalls?.(tool, props.range)}
            >
              <History className='size-3' />
              {t('Calls')}
            </Button>
          )}
          {props.onOpenToolBilling && (
            <Button
              type='button'
              variant='outline'
              size='xs'
              onClick={() => props.onOpenToolBilling?.(tool, props.range)}
            >
              <CreditCard className='size-3' />
              {t('Billing')}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}

function ProxyTrendBars(props: { trends: MCPProxyTrendResponse }) {
  const { t } = useTranslation()
  const buckets = props.trends.buckets.slice(-14)
  const maxCalls = Math.max(1, ...buckets.map((bucket) => bucket.total_calls))

  return (
    <div className='bg-muted/20 rounded-lg border px-3 py-2'>
      <div className='flex items-center gap-2 text-xs font-medium'>
        <Activity className='text-muted-foreground size-3.5' />
        {t('Daily Trend')}
      </div>
      <div className='mt-2 flex h-20 items-end gap-1.5'>
        {buckets.length > 0 ? (
          buckets.map((bucket) => {
            const failures = bucket.error_calls + bucket.timeout_calls
            return (
              <div
                key={bucket.bucket_start}
                className={cn(
                  'min-w-2 flex-1 rounded-t transition-colors',
                  failures > 0 ? 'bg-warning/70' : 'bg-primary/70'
                )}
                style={{
                  height: `${Math.max(12, (bucket.total_calls / maxCalls) * 80)}px`,
                }}
                title={`${formatTimestampToDate(bucket.bucket_start)} · ${formatNumber(bucket.total_calls)} · ${formatRate(bucket.success_rate)}`}
              />
            )
          })
        ) : (
          <div className='text-muted-foreground flex h-full items-center text-xs'>
            {t('No trend data')}
          </div>
        )}
      </div>
    </div>
  )
}

export function ProxyTrendsPanel(props: ProxyTrendsPanelProps) {
  const { t } = useTranslation()
  const [endTime] = useState(() => Math.floor(Date.now() / 1000))
  const requestParams = useMemo(() => {
    return {
      start_time: endTime - PROXY_TREND_WINDOW_SECONDS,
      end_time: endTime,
      bucket_seconds: PROXY_TREND_BUCKET_SECONDS,
    }
  }, [endTime])

  const { data, error, isError, isLoading, isFetching, refetch } = useQuery({
    queryKey: mcpQueryKeys.proxyTrends(requestParams),
    queryFn: async () => {
      const result = await getMCPProxyTrends(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load MCP proxy trends')
      }
      return normalizeMCPProxyTrendResponse(result.data)
    },
    staleTime: 30 * 1000,
    refetchInterval: 60 * 1000,
  })

  useEffect(() => {
    if (!isError) return
    toast.error(
      mcpQueryErrorMessage(error, t('Failed to load MCP proxy trends'))
    )
  }, [error, isError, t])

  if (isLoading) {
    return <Skeleton className='h-72 w-full rounded-xl' />
  }
  if (!data) return null

  const totals = data.totals
  const failures = totals.error_calls + totals.timeout_calls
  const range = { startTime: data.start_time, endTime: data.end_time }

  return (
    <div className='bg-card rounded-xl border px-4 py-3'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='flex flex-wrap items-center gap-2'>
            <Server className='text-muted-foreground size-4' />
            <span className='text-sm font-medium'>{t('Proxy Trends')}</span>
            {isFetching && (
              <RefreshCw className='text-muted-foreground size-3.5 animate-spin' />
            )}
          </div>
          <div className='text-muted-foreground mt-1 text-xs'>
            {formatTimestampToDate(data.start_time)}
            {' - '}
            {formatTimestampToDate(data.end_time)}
          </div>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => void refetch()}
          disabled={isFetching}
        >
          <RefreshCw className={cn('size-3.5', isFetching && 'animate-spin')} />
          {t('Refresh')}
        </Button>
      </div>

      <div className='mt-3 grid grid-cols-2 gap-2 md:grid-cols-5'>
        <TrendMetric
          label={t('Calls')}
          value={formatNumber(totals.total_calls)}
        />
        <TrendMetric
          label={t('Success')}
          value={formatRate(totals.success_rate)}
          tone='success'
        />
        <TrendMetric
          label={t('Failures')}
          value={formatNumber(failures)}
          tone={failures > 0 ? 'warning' : undefined}
        />
        <TrendMetric
          label={t('Quota Used')}
          value={formatQuota(totals.quota)}
        />
        <TrendMetric
          label={t('Avg Duration')}
          value={formatDuration(totals.avg_duration_ms)}
        />
      </div>

      <div className='mt-3 grid gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(280px,0.8fr)]'>
        <ProxyTrendBars trends={data} />
        <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-1'>
          <div>
            <div className='mb-2 flex items-center gap-2 text-xs font-medium'>
              <Server className='text-muted-foreground size-3.5' />
              {t('Top Servers')}
            </div>
            <div className='space-y-2'>
              {data.servers.length > 0 ? (
                data.servers.map((server) => (
                  <ServerRow
                    key={server.proxy_server_id}
                    server={server}
                    range={range}
                    onOpenServerBilling={props.onOpenServerBilling}
                    onOpenServerCalls={props.onOpenServerCalls}
                    onOpenServerTools={props.onOpenServerTools}
                  />
                ))
              ) : (
                <div className='text-muted-foreground rounded-lg border px-3 py-2 text-xs'>
                  {t('No server traffic')}
                </div>
              )}
            </div>
          </div>
          <div>
            <div className='mb-2 flex items-center gap-2 text-xs font-medium'>
              <Wrench className='text-muted-foreground size-3.5' />
              {t('Top Tools')}
            </div>
            <div className='space-y-2'>
              {data.tools.length > 0 ? (
                data.tools.map((tool) => (
                  <ToolRow
                    key={tool.proxy_tool_id}
                    tool={tool}
                    range={range}
                    onOpenToolBilling={props.onOpenToolBilling}
                    onOpenToolCalls={props.onOpenToolCalls}
                  />
                ))
              ) : (
                <div className='text-muted-foreground rounded-lg border px-3 py-2 text-xs'>
                  {t('No tool traffic')}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
