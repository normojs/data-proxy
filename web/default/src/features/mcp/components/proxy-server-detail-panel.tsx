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
import { ArrowRight, CreditCard, History, RefreshCw, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatNumber, formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/status-badge'
import {
  getMCPProxyServer,
  getMCPProxyServerHealth,
  listMCPProxyServerDiscoveryEvents,
  listMCPProxyServerTools,
  mcpQueryKeys,
} from '../api'
import {
  getProxyAuthTypeLabel,
  getProxyDiscoveryEventStatusLabel,
  getProxyDiscoveryEventStatusVariant,
  getProxyDiscoveryEventTypeLabel,
  getProxyReviewReasonLabel,
  getProxyTransportLabel,
  getProxyVisibilityLabel,
} from '../constants'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type { MCPProxyDiscoveryEvent, MCPProxyTool } from '../types'
import {
  CallStatusBadge,
  ProxyServerStatusBadge,
  ProxyToolStatusBadge,
  TimestampCell,
} from './table-cells'

const route = getRouteApi('/_authenticated/mcp/$section')

type ProxyServerDetailPanelProps = {
  serverId: number
  onClose: () => void
  onOpenTools: () => void
}

const EMPTY_PROXY_TOOLS: MCPProxyTool[] = []
const EMPTY_DISCOVERY_EVENTS: MCPProxyDiscoveryEvent[] = []
const PROXY_HEALTH_WINDOW_SECONDS = 24 * 60 * 60
const PROXY_DISCOVERY_EVENTS_PAGE_SIZE = 5

function countToolsByStatus(tools: MCPProxyTool[]) {
  return tools.reduce<Record<string, number>>((acc, tool) => {
    acc[tool.status] = (acc[tool.status] ?? 0) + 1
    return acc
  }, {})
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
        <div className='text-muted-foreground mt-1 text-xs'>
          {props.detail}
        </div>
      )}
    </div>
  )
}

export function ProxyServerDetailPanel(props: ProxyServerDetailPanelProps) {
  const { t } = useTranslation()
  const navigate = route.useNavigate()
  const {
    data: serverRes,
    error: serverError,
    isError: isServerError,
    isFetching: isServerFetching,
    refetch: refetchServer,
  } = useQuery({
    queryKey: [...mcpQueryKeys.proxyServers(), 'detail', props.serverId],
    queryFn: async () => {
      const result = await getMCPProxyServer(props.serverId)
      if (!result.success || !result.data) {
        throw mcpQueryError(result.message, 'Failed to load proxy server')
      }
      return result.data
    },
  })

  const healthParams = useMemo(
    () => ({ window_seconds: PROXY_HEALTH_WINDOW_SECONDS }),
    []
  )
  const {
    data: healthRes,
    error: healthError,
    isError: isHealthError,
    isFetching: isHealthFetching,
    refetch: refetchHealth,
  } = useQuery({
    queryKey: mcpQueryKeys.proxyServerHealth(props.serverId, healthParams),
    queryFn: async () => {
      const result = await getMCPProxyServerHealth(
        props.serverId,
        healthParams
      )
      if (!result.success || !result.data) {
        throw mcpQueryError(
          result.message,
          'Failed to load proxy server health'
        )
      }
      return result.data
    },
  })

  const {
    data: toolsRes,
    error: toolsError,
    isError: isToolsError,
    isFetching: isToolsFetching,
    refetch: refetchTools,
  } = useQuery({
    queryKey: mcpQueryKeys.proxyServerTools(props.serverId),
    queryFn: async () => {
      const result = await listMCPProxyServerTools(props.serverId)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load proxy server tools')
      }
      return result.data ?? []
    },
  })

  const discoveryEventParams = useMemo(
    () => ({ p: 1, page_size: PROXY_DISCOVERY_EVENTS_PAGE_SIZE }),
    []
  )
  const {
    data: discoveryEventsRes,
    error: discoveryEventsError,
    isError: isDiscoveryEventsError,
    isFetching: isDiscoveryEventsFetching,
    refetch: refetchDiscoveryEvents,
  } = useQuery({
    queryKey: mcpQueryKeys.proxyServerDiscoveryEvents(
      props.serverId,
      discoveryEventParams
    ),
    queryFn: async () => {
      const result = await listMCPProxyServerDiscoveryEvents(
        props.serverId,
        discoveryEventParams
      )
      if (!result.success) {
        throw mcpQueryError(
          result.message,
          'Failed to load proxy server discovery events'
        )
      }
      return result.data
    },
  })

  useEffect(() => {
    if (isServerError) {
      toast.error(
        mcpQueryErrorMessage(serverError, t('Failed to load proxy server'))
      )
    }
  }, [isServerError, serverError, t])

  useEffect(() => {
    if (isToolsError) {
      toast.error(
        mcpQueryErrorMessage(toolsError, t('Failed to load proxy server tools'))
      )
    }
  }, [isToolsError, toolsError, t])

  useEffect(() => {
    if (isHealthError) {
      toast.error(
        mcpQueryErrorMessage(
          healthError,
          t('Failed to load proxy server health')
        )
      )
    }
  }, [healthError, isHealthError, t])

  useEffect(() => {
    if (isDiscoveryEventsError) {
      toast.error(
        mcpQueryErrorMessage(
          discoveryEventsError,
          t('Failed to load proxy server discovery events')
        )
      )
    }
  }, [discoveryEventsError, isDiscoveryEventsError, t])

  const tools = toolsRes ?? EMPTY_PROXY_TOOLS
  const discoveryEvents =
    discoveryEventsRes?.items ?? EMPTY_DISCOVERY_EVENTS
  const statusCounts = useMemo(() => countToolsByStatus(tools), [tools])
  const server = serverRes
  const health = healthRes
  const isFetching =
    isServerFetching ||
    isToolsFetching ||
    isHealthFetching ||
    isDiscoveryEventsFetching

  const openToolCalls = (
    toolName: string,
    extra?: { requestId?: string; status?: string }
  ) => {
    void navigate({
      to: '/mcp/$section',
      params: { section: 'tool-calls' },
      search: (prev) => ({
        ...prev,
        callsPage: undefined,
        callsStartTime: Date.now() - PROXY_HEALTH_WINDOW_SECONDS * 1000,
        callsEndTime: undefined,
        callStatus: extra?.status ? [extra.status] : undefined,
        requestId: extra?.requestId,
        sessionId: undefined,
        targetClient: undefined,
        toolName,
      }),
    })
  }

  const openBillingEvents = (
    toolName: string,
    extra?: { requestId?: string; sourceId?: number | string }
  ) => {
    void navigate({
      to: '/mcp/$section',
      params: { section: 'billing-events' },
      search: (prev) => ({
        ...prev,
        billingPage: undefined,
        billingStartTime: Date.now() - PROXY_HEALTH_WINDOW_SECONDS * 1000,
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

  if (!server) {
    return (
      <div className='rounded-lg border px-4 py-3'>
        <div className='flex items-center justify-between gap-3'>
          <div>
            <div className='text-sm font-medium'>
              {t('Proxy Server Detail')}
            </div>
            <div className='text-muted-foreground mt-1 text-sm'>
              {t('Loading proxy server detail...')}
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
            <span className='text-base font-semibold'>{server.name}</span>
            <ProxyServerStatusBadge status={server.status} />
            <StatusBadge
              label={t(getProxyTransportLabel(server.transport))}
              variant='info'
              copyable={false}
            />
            <StatusBadge
              label={t(getProxyAuthTypeLabel(server.auth_type))}
              variant='neutral'
              copyable={false}
            />
            <StatusBadge
              label={t(getProxyVisibilityLabel(server.visibility))}
              variant='neutral'
              copyable={false}
            />
          </div>
          <div className='text-muted-foreground flex flex-wrap gap-x-4 gap-y-1 text-xs'>
            <span className='font-mono'>{server.namespace}</span>
            <span className='font-mono'>{server.endpoint || '-'}</span>
            <span>
              {t('Discovered At')}:{' '}
              <TimestampCell value={server.last_discovered_at} />
            </span>
          </div>
          {server.last_error && (
            <div className='text-destructive max-w-4xl text-sm'>
              {server.last_error}
            </div>
          )}
        </div>

        <div className='flex shrink-0 flex-wrap gap-2'>
          <Button
            type='button'
            variant='outline'
            onClick={() => {
              void refetchServer()
              void refetchTools()
              void refetchHealth()
              void refetchDiscoveryEvents()
            }}
            disabled={isFetching}
          >
            <RefreshCw
              className={isFetching ? 'size-4 animate-spin' : 'size-4'}
            />
            {t('Refresh')}
          </Button>
          <Button type='button' variant='outline' onClick={props.onOpenTools}>
            <ArrowRight className='size-4' />
            {t('Open Proxy Tools')}
          </Button>
          <Button type='button' variant='ghost' onClick={props.onClose}>
            <X className='size-4' />
            {t('Close')}
          </Button>
        </div>
      </div>

      <div className='mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
        <MetricCard
          label={t('Tools')}
          value={formatNumber(health?.discovery.total_tools ?? tools.length)}
          detail={`${t('Enabled')}: ${formatNumber(health?.discovery.enabled_tools ?? statusCounts.enabled ?? 0)}`}
        />
        <MetricCard
          label={t('24h Calls')}
          value={formatNumber(health?.calls.total_calls ?? 0)}
          detail={`${t('Success')}: ${formatNumber(health?.calls.success_calls ?? 0)}`}
        />
        <MetricCard
          label={t('Call Success Rate')}
          value={formatRate(health?.calls.success_rate ?? 0)}
          detail={`${t('Errors')}: ${formatNumber((health?.calls.error_calls ?? 0) + (health?.calls.timeout_calls ?? 0))}`}
          tone={
            (health?.calls.error_calls ?? 0) + (health?.calls.timeout_calls ?? 0)
              ? 'warning'
              : 'success'
          }
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
        {Object.entries(statusCounts).map(([status, count]) => (
          <div key={status} className='rounded-md border px-3 py-2'>
            <div className='flex items-center gap-2'>
              <ProxyToolStatusBadge status={status} />
            </div>
            <div className='mt-1 text-xl font-semibold tabular-nums'>
              {count.toLocaleString()}
            </div>
          </div>
        ))}
      </div>

      {health && (
        <div className='mt-4 rounded-md border px-3 py-2'>
          <div className='flex items-center justify-between gap-3'>
            <div className='text-sm font-medium'>{t('Top Tools')}</div>
            <div className='text-muted-foreground text-xs'>
              {t('24h Window')}
            </div>
          </div>
          {health.top_tools.length === 0 ? (
            <div className='text-muted-foreground mt-2 text-sm'>
              {t('No tool calls in this window')}
            </div>
          ) : (
            <div className='mt-2 divide-y'>
              {health.top_tools.map((tool) => (
                <div
                  key={`${tool.proxy_tool_id}-${tool.tool_id}`}
                  className='grid gap-2 py-2 text-xs first:pt-0 last:pb-0 lg:grid-cols-[minmax(0,1fr)_auto]'
                >
                  <div className='min-w-0'>
                    <div className='flex min-w-0 flex-wrap items-center gap-2'>
                      <ProxyToolStatusBadge status={tool.status} />
                      <span className='min-w-0 truncate font-mono text-sm font-medium'>
                        {tool.exposed_tool_name}
                      </span>
                    </div>
                    <div className='text-muted-foreground mt-1 flex flex-wrap gap-x-3 gap-y-1 tabular-nums'>
                      <span>
                        {t('Downstream')}: {tool.downstream_tool_name || '-'}
                      </span>
                      <span>
                        {t('Calls')}: {formatNumber(tool.calls)}
                      </span>
                      <span>
                        {t('Success')}: {formatRate(tool.success_rate)}
                      </span>
                      <span>
                        {t('Errors')}:{' '}
                        {formatNumber(tool.error_calls + tool.timeout_calls)}
                      </span>
                      <span>
                        {t('Avg Duration')}:{' '}
                        {formatDuration(tool.avg_duration_ms)}
                      </span>
                    </div>
                  </div>
                  <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 tabular-nums lg:justify-end lg:text-right'>
                    <span>
                      {t('Quota Used')}: {formatQuota(tool.quota)}
                    </span>
                    <span>
                      {t('Cost')} {tool.cost.toFixed(4)}
                    </span>
                    <div className='flex w-full flex-wrap gap-1.5 lg:justify-end'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='xs'
                        onClick={() => openToolCalls(tool.exposed_tool_name)}
                      >
                        <History className='size-3' />
                        {t('Open Tool Calls')}
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='xs'
                        onClick={() => openBillingEvents(tool.exposed_tool_name)}
                      >
                        <CreditCard className='size-3' />
                        {t('Open Billing Events')}
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {health && (
        <div className='mt-4 grid gap-3 lg:grid-cols-2'>
          <div className='rounded-md border px-3 py-2'>
            <div className='flex items-center justify-between gap-3'>
              <div className='text-sm font-medium'>{t('Review Summary')}</div>
              <StatusBadge
                label={t(health.needs_review ? 'Needs Review' : 'Balanced')}
                variant={health.needs_review ? 'warning' : 'success'}
                copyable={false}
              />
            </div>
            <div className='mt-2 flex flex-wrap gap-1.5'>
              {health.review_reasons.length === 0 ? (
                <span className='text-muted-foreground text-xs'>
                  {t('No review reasons')}
                </span>
              ) : (
                health.review_reasons.map((reason) => (
                  <StatusBadge
                    key={reason}
                    label={t(getProxyReviewReasonLabel(reason))}
                    autoColor={reason}
                    copyable={false}
                  />
                ))
              )}
            </div>
            {health.latest_check && (
              <div className='text-muted-foreground mt-2 grid gap-2 text-xs sm:grid-cols-2'>
                <span>
                  {t('Latest Check')}:{' '}
                  {t(
                    getProxyDiscoveryEventTypeLabel(
                      health.latest_check.event_type
                    )
                  )}
                </span>
                <span>
                  {t('Status')}:{' '}
                  {t(
                    getProxyDiscoveryEventStatusLabel(
                      health.latest_check.status
                    )
                  )}
                </span>
                <span>
                  {t('Duration')}:{' '}
                  {formatDuration(health.latest_check.duration_ms)}
                </span>
                <span>
                  {t('Checked At')}:{' '}
                  <TimestampCell value={health.latest_check.started_at} />
                </span>
                {health.latest_check.message && (
                  <span className='text-destructive min-w-0 truncate sm:col-span-2'>
                    {health.latest_check.message}
                  </span>
                )}
              </div>
            )}
          </div>

          <div className='rounded-md border px-3 py-2'>
            <div className='text-sm font-medium'>{t('Transport Health')}</div>
            <div className='text-muted-foreground mt-2 grid gap-2 text-xs sm:grid-cols-2'>
              <span>
                {t('Transport')}: {t(getProxyTransportLabel(health.transport.transport))}
              </span>
              <span>
                {t('Observable')}:{' '}
                {t(health.transport.observable ? 'Yes' : 'No')}
              </span>
              <span>
                {t('Initialized')}:{' '}
                {t(health.transport.initialized ? 'Yes' : 'No')}
              </span>
              <span>
                {t('Session')}: {t(health.transport.has_session ? 'Yes' : 'No')}
              </span>
              <span>
                {t('Streamable Session')}:{' '}
                {t(health.transport.streamable_session ? 'Yes' : 'No')}
              </span>
              <span>
                {t('SSE Connected')}:{' '}
                {t(health.transport.sse_connected ? 'Yes' : 'No')}
              </span>
              <span>
                {t('Active Sessions')}:{' '}
                {formatNumber(health.transport.active_sessions)}
              </span>
              <span>
                {t('Pending Requests')}:{' '}
                {formatNumber(health.transport.pending_requests)}
              </span>
              {health.transport.last_activity_at ? (
                <span>
                  {t('Last Activity')}:{' '}
                  <TimestampCell value={health.transport.last_activity_at} />
                </span>
              ) : null}
              {health.transport.message_endpoint && (
                <span className='min-w-0 truncate sm:col-span-2'>
                  {t('Message Endpoint')}: {health.transport.message_endpoint}
                </span>
              )}
              {health.transport.last_error && (
                <span className='text-destructive min-w-0 truncate sm:col-span-2'>
                  {health.transport.last_error}
                </span>
              )}
            </div>
          </div>

          <div className='rounded-md border px-3 py-2'>
            <div className='text-sm font-medium'>{t('Discovery Health')}</div>
            <div className='text-muted-foreground mt-2 grid gap-2 text-xs sm:grid-cols-2'>
              <span>
                {t('Pending')}: {formatNumber(health.discovery.pending_tools)}
              </span>
              <span>
                {t('Schema Changed')}:{' '}
                {formatNumber(health.discovery.schema_changed_tools)}
              </span>
              <span>
                {t('Error')}: {formatNumber(health.discovery.error_tools)}
              </span>
              <span>
                {t('Updated At')}:{' '}
                <TimestampCell value={health.discovery.last_tool_updated_at} />
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
            {health.recent_errors.length === 0 ? (
              <div className='text-muted-foreground mt-2 text-sm'>
                {t('No recent errors')}
              </div>
            ) : (
              <div className='mt-2 space-y-2'>
                {health.recent_errors.slice(0, 3).map((error) => (
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
                      {error.tool_name}
                      {error.error_message ? ` · ${error.error_message}` : ''}
                    </div>
                    <div className='flex flex-wrap gap-1.5'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='xs'
                        onClick={() =>
                          openToolCalls(error.tool_name, {
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
                          openBillingEvents(error.tool_name, {
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
      )}

      <div className='mt-4 rounded-md border px-3 py-2'>
        <div className='flex items-center justify-between gap-3'>
          <div className='text-sm font-medium'>{t('Discovery History')}</div>
          {isDiscoveryEventsFetching && (
            <RefreshCw className='text-muted-foreground size-3.5 animate-spin' />
          )}
        </div>
        {discoveryEvents.length === 0 ? (
          <div className='text-muted-foreground mt-2 text-sm'>
            {t('No discovery events')}
          </div>
        ) : (
          <div className='mt-2 divide-y'>
            {discoveryEvents.map((event) => (
              <div
                key={event.id}
                className='grid gap-2 py-2 text-xs first:pt-0 last:pb-0 md:grid-cols-[160px_minmax(0,1fr)]'
              >
                <div className='flex min-w-0 flex-wrap items-center gap-2'>
                  <StatusBadge
                    label={t(getProxyDiscoveryEventTypeLabel(event.event_type))}
                    variant='neutral'
                    copyable={false}
                  />
                  <StatusBadge
                    label={t(getProxyDiscoveryEventStatusLabel(event.status))}
                    variant={getProxyDiscoveryEventStatusVariant(event.status)}
                    copyable={false}
                  />
                  <TimestampCell value={event.started_at} />
                </div>
                <div className='text-muted-foreground min-w-0'>
                  <div className='flex flex-wrap gap-x-3 gap-y-1 tabular-nums'>
                    <span>
                      {t('Discovered')}: {formatNumber(event.discovered_count)}
                    </span>
                    <span>
                      {t('Created')}: {formatNumber(event.created_count)}
                    </span>
                    <span>
                      {t('Updated')}: {formatNumber(event.updated_count)}
                    </span>
                    <span>
                      {t('Disabled')}: {formatNumber(event.disabled_count)}
                    </span>
                    <span>
                      {t('Schema Changed')}:{' '}
                      {formatNumber(event.schema_changed)}
                    </span>
                    <span>
                      {t('Duration')}: {formatDuration(event.duration_ms)}
                    </span>
                  </div>
                  {event.server_name || event.protocol_version ? (
                    <div className='mt-1 truncate'>
                      {event.server_name || '-'}
                      {event.protocol_version
                        ? ` · ${event.protocol_version}`
                        : ''}
                    </div>
                  ) : null}
                  {event.message ? (
                    <div className='text-destructive mt-1 truncate'>
                      {event.message}
                    </div>
                  ) : null}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
