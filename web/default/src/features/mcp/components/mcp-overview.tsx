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
import { useNavigate } from '@tanstack/react-router'
import {
  AlertTriangle,
  Boxes,
  Cable,
  CheckCircle2,
  Clock3,
  Coins,
  RefreshCw,
  ShieldAlert,
  Wrench,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatNumber, formatQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge } from '@/components/status-badge'
import { PanelWrapper } from '@/features/dashboard/components/ui/panel-wrapper'
import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import { getMCPSummary, mcpQueryKeys } from '../api'
import {
  getMCPReviewCategoryLabel,
  getMCPReviewReasonLabel,
} from '../constants'
import { mcpQueryError, mcpQueryErrorMessage } from '../lib/query-errors'
import type { MCPSectionId } from '../section-registry'
import type {
  MCPReviewItem,
  MCPReviewQueue,
  MCPSummary,
  MCPSummaryRecentError,
} from '../types'

const MCP_OVERVIEW_WINDOW_SECONDS = 24 * 60 * 60

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

type OverviewCardTarget = {
  section: MCPSectionId
  search?: Record<string, unknown>
}

function buildSummaryCards(summary?: MCPSummary) {
  return [
    {
      key: 'tools',
      target: { section: 'tools' } satisfies OverviewCardTarget,
      titleKey: 'Tools',
      value: formatNumber(summary?.tools.enabled ?? 0),
      descriptionKey: 'Enabled MCP tools',
      icon: Wrench,
      tone: 'gray' as const,
      details: [
        {
          label: 'Total',
          value: formatNumber(summary?.tools.total ?? 0),
        },
        {
          label: 'Remote',
          value: formatNumber(summary?.tools.remote ?? 0),
        },
      ],
    },
    {
      key: 'clients',
      target: {
        section: 'bridge-clients',
        search: { clientStatus: ['online'] },
      } satisfies OverviewCardTarget,
      titleKey: 'Online Clients',
      value: formatNumber(summary?.bridge.online_clients ?? 0),
      descriptionKey: 'Local bridge clients online',
      icon: Cable,
      tone: 'teal' as const,
      details: [
        {
          label: 'Registered',
          value: formatNumber(summary?.bridge.total_clients ?? 0),
        },
        {
          label: 'Active 24h',
          value: formatNumber(summary?.bridge.active_clients ?? 0),
        },
      ],
    },
    {
      key: 'calls',
      target: { section: 'tool-calls' } satisfies OverviewCardTarget,
      titleKey: 'Tool Calls',
      value: formatNumber(summary?.calls.total_calls ?? 0),
      descriptionKey: 'MCP tool calls in 24h',
      icon: Boxes,
      tone: 'rose' as const,
      details: [
        {
          label: 'Success',
          value: formatRate(summary?.calls.success_rate ?? 0),
          tone: 'success' as const,
        },
        {
          label: 'Errors',
          value: formatNumber(
            (summary?.calls.error_calls ?? 0) +
              (summary?.calls.timeout_calls ?? 0)
          ),
          tone: 'warning' as const,
        },
      ],
    },
    {
      key: 'quota',
      target: {
        section: 'billing-events',
        search: {
          billingSourceKind: ['mcp_tool_call'],
        },
      } satisfies OverviewCardTarget,
      titleKey: 'Quota Used',
      value: formatQuota(summary?.calls.quota ?? 0),
      descriptionKey: 'MCP quota charged in 24h',
      icon: Coins,
      tone: 'gray' as const,
      details: [
        {
          label: 'Settled',
          value: formatNumber(summary?.calls.settled_calls ?? 0),
          tone: 'success' as const,
        },
        {
          label: 'Unsettled',
          value: formatNumber(summary?.calls.unsettled ?? 0),
          tone: ((summary?.calls.unsettled ?? 0) > 0 ? 'warning' : 'muted') as
            | 'muted'
            | 'warning',
        },
      ],
    },
  ]
}

function TopToolsPanel(props: { loading: boolean; summary?: MCPSummary }) {
  const { t } = useTranslation()
  const tools = props.summary?.top_tools ?? []

  return (
    <PanelWrapper
      title={t('Top Tools')}
      description={t('Highest traffic MCP tools in 24h')}
      loading={props.loading}
      empty={!props.loading && tools.length === 0}
      emptyMessage={t('No tool calls in this window')}
      height='h-56'
    >
      <div className='space-y-3'>
        {tools.map((tool) => (
          <div
            key={tool.tool_name}
            className='bg-muted/20 grid gap-2 rounded-lg border px-3 py-2.5 sm:grid-cols-[minmax(0,1fr)_auto]'
          >
            <div className='min-w-0'>
              <div className='truncate text-sm font-medium'>
                {tool.tool_name}
              </div>
              <div className='text-muted-foreground mt-1 flex flex-wrap items-center gap-2 text-xs'>
                <span>
                  {t('Calls')} {formatNumber(tool.calls)}
                </span>
                <span>
                  {t('Success')} {formatRate(tool.success_rate)}
                </span>
                <span>
                  {t('Avg')} {formatDuration(tool.avg_duration_ms)}
                </span>
              </div>
            </div>
            <div className='text-left sm:text-right'>
              <div className='text-sm font-medium tabular-nums'>
                {formatQuota(tool.quota)}
              </div>
              <div className='text-muted-foreground text-xs tabular-nums'>
                {t('Cost')} {tool.cost.toFixed(4)}
              </div>
            </div>
          </div>
        ))}
      </div>
    </PanelWrapper>
  )
}

function ErrorSourceBadge(props: { source: string }) {
  const { t } = useTranslation()
  const isAudit = props.source === 'bridge_audit'
  return (
    <StatusBadge
      label={t(isAudit ? 'Bridge' : 'Tool Call')}
      variant={isAudit ? 'info' : 'warning'}
      copyable={false}
    />
  )
}

function RecentErrorRow(props: {
  error: MCPSummaryRecentError
  onOpen: (error: MCPSummaryRecentError) => void
}) {
  const error = props.error
  return (
    <button
      type='button'
      className='bg-muted/20 hover:bg-muted/40 focus-visible:ring-ring/50 grid w-full gap-2 rounded-lg border px-3 py-2.5 text-left transition-colors outline-none focus-visible:ring-3 sm:grid-cols-[minmax(0,1fr)_auto]'
      onClick={() => props.onOpen(error)}
    >
      <div className='min-w-0'>
        <div className='flex min-w-0 flex-wrap items-center gap-2'>
          <ErrorSourceBadge source={error.source} />
          <span className='truncate text-sm font-medium'>
            {error.tool_name}
          </span>
        </div>
        <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
          {error.error_message || error.error_code || '-'}
        </div>
        <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
          {error.request_id}
        </div>
      </div>
      <div className='text-muted-foreground text-left text-xs whitespace-nowrap sm:text-right'>
        {formatTimestampToDate(error.created_at)}
      </div>
    </button>
  )
}

function RecentErrorsPanel(props: {
  loading: boolean
  summary?: MCPSummary
  onOpenError: (error: MCPSummaryRecentError) => void
}) {
  const { t } = useTranslation()
  const errors = props.summary?.recent_errors ?? []

  return (
    <PanelWrapper
      title={t('Recent Errors')}
      description={t('Latest MCP and bridge failures in 24h')}
      loading={props.loading}
      empty={!props.loading && errors.length === 0}
      emptyMessage={t('No recent errors')}
      height='h-56'
    >
      <div className='space-y-3'>
        {errors.map((error) => (
          <RecentErrorRow
            key={`${error.source}-${error.request_id}-${error.created_at}`}
            error={error}
            onOpen={props.onOpenError}
          />
        ))}
      </div>
    </PanelWrapper>
  )
}

function HealthStrip(props: { loading: boolean; summary?: MCPSummary }) {
  const { t } = useTranslation()
  const summary = props.summary
  const callErrors =
    (summary?.calls.error_calls ?? 0) + (summary?.calls.timeout_calls ?? 0)
  const auditErrors =
    (summary?.audit.error ?? 0) + (summary?.audit.timeout ?? 0)

  if (props.loading) {
    return <Skeleton className='h-16 w-full rounded-xl' />
  }

  return (
    <div className='grid gap-3 md:grid-cols-3'>
      <div className='bg-card rounded-xl border px-4 py-3'>
        <div className='flex items-center gap-2 text-sm font-medium'>
          <CheckCircle2 className='text-success size-4' />
          {t('Call Success Rate')}
        </div>
        <div className='mt-2 text-2xl font-semibold tabular-nums'>
          {formatRate(summary?.calls.success_rate ?? 0)}
        </div>
      </div>
      <div className='bg-card rounded-xl border px-4 py-3'>
        <div className='flex items-center gap-2 text-sm font-medium'>
          <Clock3 className='text-muted-foreground size-4' />
          {t('Average Duration')}
        </div>
        <div className='mt-2 text-2xl font-semibold tabular-nums'>
          {formatDuration(summary?.calls.avg_duration_ms ?? 0)}
        </div>
      </div>
      <div
        className={cn(
          'bg-card rounded-xl border px-4 py-3',
          callErrors + auditErrors > 0 && 'border-warning/50 bg-warning/5'
        )}
      >
        <div className='flex items-center gap-2 text-sm font-medium'>
          <AlertTriangle className='text-warning size-4' />
          {t('Failures')}
        </div>
        <div className='mt-2 text-2xl font-semibold tabular-nums'>
          {formatNumber(callErrors + auditErrors)}
        </div>
      </div>
    </div>
  )
}

function reviewItemTarget(item: MCPReviewItem): OverviewCardTarget {
  switch (item.category) {
    case 'bridge_client':
      return {
        section: 'bridge-clients',
        search: { clientId: item.target_id },
      }
    case 'tool':
      return {
        section: 'tool-calls',
        search: { toolName: item.target_id, callStatus: ['error'] },
      }
    case 'proxy_server':
      return {
        section: 'proxy-servers',
        search: item.target_name
          ? { proxyServerFilter: item.target_name }
          : undefined,
      }
    default:
      return { section: 'proxy-servers' }
  }
}

function ReviewQueueRow(props: {
  item: MCPReviewItem
  onOpen: (target: OverviewCardTarget) => void
}) {
  const { t } = useTranslation()
  const item = props.item
  const isCritical = item.severity === 'critical'
  return (
    <button
      type='button'
      className='bg-muted/20 hover:bg-muted/40 focus-visible:ring-ring/50 grid w-full gap-1.5 rounded-lg border px-3 py-2.5 text-left transition-colors outline-none focus-visible:ring-3'
      onClick={() => props.onOpen(reviewItemTarget(item))}
    >
      <div className='flex min-w-0 flex-wrap items-center gap-2'>
        <span
          className={cn(
            'size-2 shrink-0 rounded-full',
            isCritical ? 'bg-destructive' : 'bg-warning'
          )}
        />
        <span className='text-muted-foreground text-xs'>
          {t(getMCPReviewCategoryLabel(item.category))}
        </span>
        <span className='truncate text-sm font-medium'>
          {item.target_name || item.target_id}
        </span>
      </div>
      <div className='flex flex-wrap gap-1'>
        {item.reasons.map((reason) => (
          <span
            key={reason}
            className='text-muted-foreground rounded border px-1.5 py-0.5 text-xs'
          >
            {t(getMCPReviewReasonLabel(reason))}
          </span>
        ))}
      </div>
      {item.detail ? (
        <div className='text-muted-foreground truncate font-mono text-xs'>
          {item.detail}
        </div>
      ) : null}
    </button>
  )
}

function ReviewQueuePanel(props: {
  loading: boolean
  queue?: MCPReviewQueue
  onOpen: (target: OverviewCardTarget) => void
}) {
  const { t } = useTranslation()
  const items = props.queue?.items ?? []
  const critical = props.queue?.critical_count ?? 0
  const warning = props.queue?.warning_count ?? 0

  return (
    <PanelWrapper
      title={t('Review Queue')}
      description={t('MCP operations items that need attention')}
      loading={props.loading}
      empty={!props.loading && items.length === 0}
      emptyMessage={t('No review items')}
      height='h-72'
    >
      <div className='space-y-3'>
        <div className='flex flex-wrap items-center gap-4 text-xs'>
          <span className='flex items-center gap-1.5'>
            <ShieldAlert className='text-destructive size-3.5' />
            {t('Critical')} {formatNumber(critical)}
          </span>
          <span className='flex items-center gap-1.5'>
            <AlertTriangle className='text-warning size-3.5' />
            {t('Warning')} {formatNumber(warning)}
          </span>
        </div>
        <div className='space-y-2'>
          {items.map((item) => (
            <ReviewQueueRow
              key={`${item.category}-${item.target_id}-${item.reasons.join('-')}`}
              item={item}
              onOpen={props.onOpen}
            />
          ))}
        </div>
      </div>
    </PanelWrapper>
  )
}

export function MCPOverview() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const navigate = useNavigate()

  const openSection = (target: OverviewCardTarget) => {
    void navigate({
      to: '/mcp/$section',
      params: { section: target.section },
      search: {
        filter: undefined,
        toolStatus: undefined,
        clientStatus: undefined,
        callStatus: undefined,
        billingSourceKind: undefined,
        billingEventType: undefined,
        billingStatus: undefined,
        billingUsageKind: undefined,
        auditStatus: undefined,
        source: undefined,
        toolName: undefined,
        requestId: undefined,
        tokenId: undefined,
        sessionId: undefined,
        targetClient: undefined,
        clientId: undefined,
        billingSource: undefined,
        callsStartTime: undefined,
        callsEndTime: undefined,
        billingStartTime: undefined,
        billingEndTime: undefined,
        auditStartTime: undefined,
        auditEndTime: undefined,
        ...target.search,
      },
    })
  }

  const openRecentError = (error: MCPSummaryRecentError) => {
    if (error.source === 'bridge_audit') {
      openSection({
        section: 'audit-logs',
        search: {
          requestId: error.request_id,
          toolName: error.tool_name,
          clientId: error.client_id,
          sessionId: error.session_id,
          auditStatus: ['error'],
        },
      })
      return
    }

    openSection({
      section: 'tool-calls',
      search: {
        requestId: error.request_id,
        toolName: error.tool_name,
        sessionId: error.session_id,
        targetClient: error.client_id,
        callStatus: ['error'],
      },
    })
  }

  const requestParams = useMemo(
    () => ({
      scope: isAdmin ? ('all' as const) : undefined,
      window_seconds: MCP_OVERVIEW_WINDOW_SECONDS,
    }),
    [isAdmin]
  )

  const {
    data,
    error: overviewError,
    isError: isOverviewError,
    isLoading,
    isFetching,
    refetch,
  } = useQuery({
    queryKey: mcpQueryKeys.summary(requestParams),
    queryFn: async () => {
      const result = await getMCPSummary(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load MCP overview')
      }
      return result.data
    },
    refetchInterval: 30 * 1000,
    staleTime: 10 * 1000,
  })

  useEffect(() => {
    if (!isOverviewError) return
    toast.error(
      mcpQueryErrorMessage(overviewError, t('Failed to load MCP overview'))
    )
  }, [isOverviewError, overviewError, t])

  const cards = useMemo(() => buildSummaryCards(data), [data])

  return (
    <div className='space-y-4'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='text-muted-foreground text-sm'>
          {t('MCP operations overview for the last 24 hours')}
        </div>
        <Button
          type='button'
          variant='outline'
          onClick={() => void refetch()}
          disabled={isFetching}
          className={cn(isFetching && 'opacity-80')}
        >
          <RefreshCw className={cn('size-4', isFetching && 'animate-spin')} />
          {t('Refresh')}
        </Button>
      </div>

      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
        {cards.map((card) => (
          <button
            key={card.key}
            type='button'
            className='bg-card hover:bg-muted/20 focus-visible:ring-ring/50 rounded-xl border p-3 text-left transition-colors outline-none focus-visible:ring-3'
            onClick={() => openSection(card.target)}
          >
            <StatCard
              title={t(card.titleKey)}
              value={card.value}
              description={t(card.descriptionKey)}
              icon={card.icon}
              tone={card.tone}
              details={card.details.map((detail) => ({
                ...detail,
                label: t(detail.label),
              }))}
              loading={isLoading}
            />
          </button>
        ))}
      </div>

      <HealthStrip loading={isLoading} summary={data} />

      {isAdmin ? (
        <ReviewQueuePanel
          loading={isLoading}
          queue={data?.review_queue}
          onOpen={openSection}
        />
      ) : null}

      <div className='grid gap-4 xl:grid-cols-2'>
        <TopToolsPanel loading={isLoading} summary={data} />
        <RecentErrorsPanel
          loading={isLoading}
          summary={data}
          onOpenError={openRecentError}
        />
      </div>

      <div className='grid gap-4 xl:grid-cols-2'>
        <button
          type='button'
          className='text-left outline-none'
          onClick={() => openSection({ section: 'audit-logs' })}
        >
          <PanelWrapper
            title={t('Bridge Throughput')}
            description={t('Remote bridge request volume in 24h')}
            empty={false}
            className='hover:bg-muted/20 focus-visible:ring-ring/50 transition-colors focus-visible:ring-3'
          >
            <div className='grid grid-cols-2 gap-3'>
              <Metric
                label={t('Requests')}
                value={formatNumber(data?.audit.total_requests ?? 0)}
              />
              <Metric
                label={t('Success')}
                value={formatRate(data?.audit.success_rate ?? 0)}
              />
              <Metric
                label={t('Avg Duration')}
                value={formatDuration(data?.audit.avg_duration_ms ?? 0)}
              />
              <Metric
                label={t('Result Size')}
                value={formatSize(data?.audit.result_size ?? 0)}
              />
            </div>
          </PanelWrapper>
        </button>
        <button
          type='button'
          className='text-left outline-none'
          onClick={() =>
            openSection({
              section: 'billing-events',
              search: { billingSourceKind: ['mcp_tool_call'] },
            })
          }
        >
          <PanelWrapper
            title={t('Billing Health')}
            description={t('MCP settlement and daily free quota usage in 24h')}
            empty={false}
            className='hover:bg-muted/20 focus-visible:ring-ring/50 transition-colors focus-visible:ring-3'
          >
            <div className='grid grid-cols-2 gap-3'>
              <Metric
                label={t('Settled')}
                value={formatNumber(data?.calls.settled_calls ?? 0)}
              />
              <Metric
                label={t('Unsettled')}
                value={formatNumber(data?.calls.unsettled ?? 0)}
              />
              <Metric
                label={t('Daily Free Used')}
                value={formatNumber(data?.calls.free_calls ?? 0)}
              />
              <Metric
                label={t('Cost')}
                value={(data?.calls.cost ?? 0).toFixed(4)}
              />
            </div>
          </PanelWrapper>
        </button>
      </div>
    </div>
  )
}

function Metric(props: { label: string; value: string }) {
  return (
    <div className='bg-muted/20 rounded-lg border px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='mt-1 text-sm font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}
