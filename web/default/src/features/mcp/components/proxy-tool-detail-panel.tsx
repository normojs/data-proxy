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
import { useEffect, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import {
  ArrowRight,
  Code2,
  CreditCard,
  History,
  RefreshCw,
  Wrench,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatNumber, formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/status-badge'
import { getMCPProxyTool, getMCPProxyToolHealth, mcpQueryKeys } from '../api'
import { getPriceUnitLabel } from '../constants'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type { MCPProxyTool } from '../types'
import {
  CallStatusBadge,
  ProxyToolStatusBadge,
  TimestampCell,
} from './table-cells'

const route = getRouteApi('/_authenticated/mcp/$section')
const PROXY_TOOL_HEALTH_WINDOW_SECONDS = 24 * 60 * 60

type ProxyToolDetailPanelProps = {
  proxyToolId: number
  onClose: () => void
  onViewSchema: (tool: MCPProxyTool) => void
}

function formatRate(value: number): string {
  return `${formatNumber(value)}%`
}

function formatDuration(value: number): string {
  if (!value) return '-'
  return `${formatNumber(value)} ms`
}

function formatSize(value: number): string {
  if (!value) return '-'
  if (value < 1024) return `${formatNumber(value)} B`
  if (value < 1024 * 1024) return `${formatNumber(value / 1024)} KB`
  return `${formatNumber(value / 1024 / 1024)} MB`
}

function MetricCard(props: {
  label: string
  value: string
  detail?: string
  tone?: 'default' | 'success' | 'warning' | 'danger'
}) {
  const toneClass =
    props.tone === 'success'
      ? 'text-green-600 dark:text-green-400'
      : props.tone === 'warning'
        ? 'text-yellow-600 dark:text-yellow-400'
        : props.tone === 'danger'
          ? 'text-red-600 dark:text-red-400'
          : ''

  return (
    <div className='rounded-md border px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className={`mt-1 text-xl font-semibold tabular-nums ${toneClass}`}>
        {props.value}
      </div>
      {props.detail && (
        <div className='text-muted-foreground mt-1 text-xs'>{props.detail}</div>
      )}
    </div>
  )
}

export function ProxyToolDetailPanel(props: ProxyToolDetailPanelProps) {
  const { t } = useTranslation()
  const navigate = route.useNavigate()
  const healthParams = useMemo(
    () => ({ window_seconds: PROXY_TOOL_HEALTH_WINDOW_SECONDS }),
    []
  )

  const {
    data: tool,
    error: toolError,
    isError: isToolError,
    isFetching: isToolFetching,
    refetch: refetchTool,
  } = useQuery({
    queryKey: mcpQueryKeys.proxyToolDetail(props.proxyToolId),
    queryFn: async () => {
      const result = await getMCPProxyTool(props.proxyToolId)
      if (!result.success || !result.data) {
        throw mcpQueryError(result.message, 'Failed to load MCP proxy tool')
      }
      return result.data
    },
  })

  const {
    data: health,
    error: healthError,
    isError: isHealthError,
    isFetching: isHealthFetching,
    refetch: refetchHealth,
  } = useQuery({
    queryKey: mcpQueryKeys.proxyToolHealth(props.proxyToolId, healthParams),
    queryFn: async () => {
      const result = await getMCPProxyToolHealth(
        props.proxyToolId,
        healthParams
      )
      if (!result.success || !result.data) {
        throw mcpQueryError(result.message, 'Failed to load proxy tool health')
      }
      return result.data
    },
  })

  useEffect(() => {
    if (!isToolError) return
    toast.error(
      mcpQueryErrorMessage(toolError, t('Failed to load MCP proxy tool'))
    )
  }, [isToolError, t, toolError])

  useEffect(() => {
    if (!isHealthError) return
    toast.error(
      mcpQueryErrorMessage(healthError, t('Failed to load proxy tool health'))
    )
  }, [healthError, isHealthError, t])

  const openToolCalls = (extra?: { requestId?: string; status?: string }) => {
    void navigate({
      to: '/mcp/$section',
      params: { section: 'tool-calls' },
      search: (prev) => ({
        ...prev,
        callsPage: undefined,
        callsStartTime: Date.now() - PROXY_TOOL_HEALTH_WINDOW_SECONDS * 1000,
        callsEndTime: undefined,
        callStatus: extra?.status ? [extra.status] : undefined,
        requestId: extra?.requestId,
        sessionId: undefined,
        targetClient: undefined,
        toolName:
          tool?.exposed_tool_name ?? health?.tool.exposed_tool_name ?? '',
      }),
    })
  }

  const openBillingEvents = (extra?: {
    requestId?: string
    sourceId?: number | string
  }) => {
    const toolName = tool?.exposed_tool_name ?? health?.tool.exposed_tool_name
    void navigate({
      to: '/mcp/$section',
      params: { section: 'billing-events' },
      search: (prev) => ({
        ...prev,
        billingPage: undefined,
        billingStartTime: Date.now() - PROXY_TOOL_HEALTH_WINDOW_SECONDS * 1000,
        billingEndTime: undefined,
        billingSourceKind: ['mcp_tool_call'],
        billingEventType: undefined,
        billingStatus: undefined,
        billingUsageKind: undefined,
        billingSource: undefined,
        billingSourceId:
          extra?.sourceId == null ? undefined : String(extra.sourceId),
        filter: extra?.sourceId == null ? toolName : undefined,
        requestId: extra?.requestId,
      }),
    })
  }

  const isFetching = isToolFetching || isHealthFetching
  const failureCount =
    (health?.calls.error_calls ?? 0) + (health?.calls.timeout_calls ?? 0)

  if (!tool) {
    return (
      <div className='rounded-lg border px-4 py-3'>
        <div className='flex items-center justify-between gap-3'>
          <div>
            <div className='text-sm font-medium'>{t('Proxy Tool Detail')}</div>
            <div className='text-muted-foreground mt-1 text-sm'>
              {t('Loading proxy tool detail...')}
            </div>
          </div>
          <Button variant='ghost' size='icon-sm' onClick={props.onClose}>
            <X className='size-4' />
            <span className='sr-only'>{t('Close')}</span>
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className='rounded-lg border px-4 py-3'>
      <div className='flex flex-col gap-3 md:flex-row md:items-start md:justify-between'>
        <div className='min-w-0 space-y-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <span className='min-w-0 truncate font-mono text-base font-semibold'>
              {tool.exposed_tool_name}
            </span>
            <ProxyToolStatusBadge status={tool.status} />
            <StatusBadge
              label={`${t('Server ID')} ${tool.proxy_server_id}`}
              variant='neutral'
              copyable={false}
            />
            <StatusBadge
              label={`${t('MCP Tool ID')} ${tool.mcp_tool_id}`}
              variant='neutral'
              copyable={false}
            />
          </div>
          <div className='text-muted-foreground flex flex-wrap gap-x-4 gap-y-1 text-xs'>
            <span>
              {t('Downstream')}: {tool.downstream_tool_name || '-'}
            </span>
            <span>
              {t('Schema Hash')}: {tool.schema_hash || '-'}
            </span>
            <span>
              {t('Discovered At')}:{' '}
              <TimestampCell value={tool.last_discovered_at} />
            </span>
          </div>
          {(tool.exposed_description || tool.downstream_description) && (
            <div className='text-muted-foreground max-w-4xl text-sm'>
              {tool.exposed_description || tool.downstream_description}
            </div>
          )}
          {tool.last_error && (
            <div className='text-destructive max-w-4xl text-sm'>
              {tool.last_error}
            </div>
          )}
        </div>

        <div className='flex shrink-0 flex-wrap gap-2'>
          <Button
            type='button'
            variant='outline'
            onClick={() => {
              void refetchTool()
              void refetchHealth()
            }}
            disabled={isFetching}
          >
            <RefreshCw
              className={isFetching ? 'size-4 animate-spin' : 'size-4'}
            />
            {t('Refresh')}
          </Button>
          <Button
            type='button'
            variant='outline'
            onClick={() =>
              void navigate({
                to: '/mcp/$section',
                params: { section: 'tools' },
                search: (prev) => ({
                  ...prev,
                  page: undefined,
                  filter: undefined,
                  toolStatus: undefined,
                  source: ['mcp_proxy'],
                  toolName: tool.exposed_tool_name,
                }),
              })
            }
          >
            <Wrench className='size-4' />
            {t('Open MCP Tool')}
          </Button>
          <Button
            type='button'
            variant='ghost'
            onClick={() => props.onViewSchema(tool)}
          >
            <Code2 className='size-4' />
            {t('View Schema')}
          </Button>
          <Button type='button' variant='ghost' onClick={props.onClose}>
            <X className='size-4' />
            {t('Close')}
          </Button>
        </div>
      </div>

      <div className='mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
        <MetricCard
          label={t('24h Calls')}
          value={formatNumber(health?.calls.total_calls ?? 0)}
          detail={`${t('Success')}: ${formatNumber(health?.calls.success_calls ?? 0)}`}
        />
        <MetricCard
          label={t('Call Success Rate')}
          value={formatRate(health?.calls.success_rate ?? 0)}
          detail={`${t('Errors')}: ${formatNumber(failureCount)}`}
          tone={failureCount ? 'warning' : 'success'}
        />
        <MetricCard
          label={t('Avg Duration')}
          value={formatDuration(health?.calls.avg_duration_ms ?? 0)}
          detail={`${t('Result Size')}: ${formatSize(health?.calls.result_size ?? 0)}`}
        />
        <MetricCard
          label={t('Quota Used')}
          value={formatQuota(health?.calls.quota ?? 0)}
          detail={
            health?.calls.cost
              ? `${t('Cost')} ${health.calls.cost.toFixed(4)}`
              : t('No Quota')
          }
        />
      </div>

      <div className='mt-4 grid gap-3 lg:grid-cols-2'>
        <div className='rounded-md border px-3 py-2'>
          <div className='text-sm font-medium'>{t('Proxy Mapping')}</div>
          <div className='text-muted-foreground mt-2 grid gap-2 text-xs sm:grid-cols-2'>
            <span>
              {t('Exposed Tool Name')}: {tool.exposed_tool_name}
            </span>
            <span>
              {t('Downstream')}: {tool.downstream_tool_name || '-'}
            </span>
            <span>
              {t('Price')}: {tool.price_per_call.toFixed(4)}{' '}
              {t(getPriceUnitLabel(tool.price_unit))}
            </span>
            <span>
              {t('Daily Free Quota')}: {formatNumber(tool.free_quota)}
            </span>
            <span>
              {t('Sort Order')}: {formatNumber(tool.sort_order)}
            </span>
            <span>
              {t('Updated At')}: <TimestampCell value={tool.updated_at} />
            </span>
          </div>
        </div>

        <div className='rounded-md border px-3 py-2'>
          <div className='flex items-center justify-between gap-3'>
            <div className='text-sm font-medium'>{t('Recent Errors')}</div>
            <div className='text-muted-foreground text-xs'>
              {t('24h Window')}
            </div>
          </div>
          {!health || health.recent_errors.length === 0 ? (
            <div className='text-muted-foreground mt-2 text-sm'>
              {t('No recent errors')}
            </div>
          ) : (
            <div className='mt-2 space-y-2'>
              {health.recent_errors.slice(0, 5).map((error) => (
                <div
                  key={error.id}
                  className='grid gap-1 border-t pt-2 first:border-t-0 first:pt-0'
                >
                  <div className='flex min-w-0 flex-wrap items-center gap-2'>
                    <CallStatusBadge status={error.status} />
                    <span className='max-w-[220px] truncate font-mono text-xs'>
                      {error.request_id || '-'}
                    </span>
                    <TimestampCell value={error.created_at} />
                  </div>
                  <div className='text-muted-foreground truncate text-xs'>
                    {error.error_message || error.error_code || '-'}
                  </div>
                  <div className='flex flex-wrap gap-1.5'>
                    <Button
                      type='button'
                      variant='ghost'
                      size='xs'
                      onClick={() =>
                        openToolCalls({
                          requestId: error.request_id,
                          status: error.status,
                        })
                      }
                    >
                      <History className='size-3' />
                      {t('Open Tool Calls')}
                    </Button>
                    <Button
                      type='button'
                      variant='ghost'
                      size='xs'
                      onClick={() =>
                        openBillingEvents({
                          requestId: error.request_id,
                          sourceId: error.id,
                        })
                      }
                    >
                      <CreditCard className='size-3' />
                      {t('Open Billing Events')}
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className='mt-4 flex flex-wrap gap-2'>
        <Button type='button' variant='outline' onClick={() => openToolCalls()}>
          <History className='size-4' />
          {t('Open Tool Calls')}
        </Button>
        <Button
          type='button'
          variant='outline'
          onClick={() => openBillingEvents()}
        >
          <CreditCard className='size-4' />
          {t('Open Billing Events')}
        </Button>
        <Button
          type='button'
          variant='ghost'
          onClick={() =>
            void navigate({
              to: '/mcp/$section',
              params: { section: 'proxy-servers' },
              search: (prev) => ({
                ...prev,
                proxyServerId: tool.proxy_server_id,
              }),
            })
          }
        >
          <ArrowRight className='size-4' />
          {t('Proxy Server Detail')}
        </Button>
      </div>
    </div>
  )
}
