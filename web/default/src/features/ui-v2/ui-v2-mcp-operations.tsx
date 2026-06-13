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
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import {
  AlertTriangle,
  ArrowRight,
  Boxes,
  Cable,
  Clock3,
  HardDrive,
  RefreshCw,
  Wrench,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatNumber, formatQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { getMCPSummary, mcpQueryKeys } from '@/features/mcp/api'
import {
  getMCPReviewCategoryLabel,
  getMCPReviewReasonLabel,
} from '@/features/mcp/constants'
import { normalizeMCPSummaryOperationsTrends } from '@/features/mcp/lib/overview-trends'
import {
  mcpQueryError,
  mcpQueryErrorMessage,
} from '@/features/mcp/lib/query-errors'
import type { MCPSectionId } from '@/features/mcp/section-registry'
import type {
  MCPReviewItem,
  MCPSummary,
  MCPSummaryRecentError,
} from '@/features/mcp/types'

const MCP_OPERATIONS_WINDOW_SECONDS = 24 * 60 * 60

type MCPSectionTarget = {
  section: MCPSectionId
  search?: Record<string, unknown>
}

type MetricTone = 'neutral' | 'success' | 'warning' | 'danger' | 'info'

function formatRate(value: number | undefined): string {
  return `${formatNumber(value ?? 0)}%`
}

function formatDuration(value: number | undefined): string {
  if (!value) return '-'
  return `${formatNumber(value)} ms`
}

function formatSize(value: number | undefined): string {
  if (!value) return '-'
  if (value < 1024) return `${formatNumber(value)} B`
  if (value < 1024 * 1024) return `${formatNumber(value / 1024)} KB`
  return `${formatNumber(value / 1024 / 1024)} MB`
}

function metricToneClassName(tone: MetricTone): string {
  switch (tone) {
    case 'success':
      return 'text-success'
    case 'warning':
      return 'text-warning'
    case 'danger':
      return 'text-destructive'
    case 'info':
      return 'text-info'
    default:
      return 'text-foreground'
  }
}

function statusVariantFromTone(tone: MetricTone): StatusVariant {
  switch (tone) {
    case 'success':
      return 'success'
    case 'warning':
      return 'warning'
    case 'danger':
      return 'danger'
    case 'info':
      return 'info'
    default:
      return 'neutral'
  }
}

function isSummaryEmpty(summary?: MCPSummary): boolean {
  if (!summary) return false
  return (
    (summary.calls?.total_calls ?? 0) === 0 &&
    (summary.audit?.total_requests ?? 0) === 0 &&
    (summary.bridge?.total_clients ?? 0) === 0 &&
    (summary.tools?.total ?? 0) === 0 &&
    (summary.top_tools ?? []).length === 0 &&
    (summary.recent_errors ?? []).length === 0 &&
    (summary.review_queue?.items ?? []).length === 0
  )
}

function buildDetailSearch(target: MCPSectionTarget) {
  return {
    filter: undefined,
    toolStatus: undefined,
    clientStatus: undefined,
    callStatus: undefined,
    proxyServerStatus: undefined,
    proxyToolStatus: undefined,
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
    proxyServerFilter: undefined,
    proxyToolFilter: undefined,
    billingSource: undefined,
    callsStartTime: undefined,
    callsEndTime: undefined,
    billingStartTime: undefined,
    billingEndTime: undefined,
    auditStartTime: undefined,
    auditEndTime: undefined,
    ...target.search,
  }
}

function reviewItemTarget(item: MCPReviewItem): MCPSectionTarget {
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
      return { section: 'overview' }
  }
}

function recentErrorTarget(error: MCPSummaryRecentError): MCPSectionTarget {
  if (error.source === 'bridge_audit') {
    return {
      section: 'audit-logs',
      search: {
        requestId: error.request_id,
        toolName: error.tool_name,
        clientId: error.client_id,
        sessionId: error.session_id,
        auditStatus: ['error'],
      },
    }
  }

  return {
    section: 'tool-calls',
    search: {
      requestId: error.request_id,
      toolName: error.tool_name,
      sessionId: error.session_id,
      targetClient: error.client_id,
      callStatus: ['error'],
    },
  }
}

function SurfacePanel(props: {
  title: string
  description?: string
  action?: React.ReactNode
  children: React.ReactNode
  className?: string
}) {
  return (
    <section
      className={cn('border-border bg-card rounded-xl border', props.className)}
    >
      <div className='border-border flex flex-col gap-2 border-b px-4 py-3 sm:flex-row sm:items-center sm:justify-between'>
        <div className='min-w-0'>
          <h2 className='text-foreground text-sm font-semibold'>
            {props.title}
          </h2>
          {props.description ? (
            <p className='text-muted-foreground mt-1 text-xs text-pretty'>
              {props.description}
            </p>
          ) : null}
        </div>
        {props.action ? <div className='shrink-0'>{props.action}</div> : null}
      </div>
      {props.children}
    </section>
  )
}

function MetricTile(props: {
  label: string
  value: string
  detail: string
  tone?: MetricTone
  icon: React.ComponentType<{ className?: string }>
  onOpen?: () => void
}) {
  const Icon = props.icon
  const content = (
    <>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='text-muted-foreground truncate text-xs'>
            {props.label}
          </div>
          <div
            className={cn(
              'mt-1 text-xl font-semibold tabular-nums',
              metricToneClassName(props.tone ?? 'neutral')
            )}
          >
            {props.value}
          </div>
        </div>
        <Icon
          className={cn(
            'mt-0.5 size-4 shrink-0',
            metricToneClassName(props.tone ?? 'neutral')
          )}
        />
      </div>
      <div className='text-muted-foreground mt-2 truncate text-xs'>
        {props.detail}
      </div>
    </>
  )

  if (props.onOpen) {
    return (
      <button
        type='button'
        className='border-border bg-background hover:bg-muted/40 focus-visible:ring-ring/50 rounded-lg border p-3 text-left transition-colors outline-none focus-visible:ring-3'
        onClick={props.onOpen}
      >
        {content}
      </button>
    )
  }

  return (
    <div className='border-border bg-background rounded-lg border p-3'>
      {content}
    </div>
  )
}

function LoadingSurface() {
  return (
    <div className='space-y-4'>
      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
        {Array.from({ length: 4 }).map((_, index) => (
          <Skeleton key={index} className='h-[104px] rounded-xl' />
        ))}
      </div>
      <Skeleton className='h-24 rounded-xl' />
      <div className='grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(320px,0.8fr)]'>
        <Skeleton className='h-80 rounded-xl' />
        <Skeleton className='h-80 rounded-xl' />
      </div>
    </div>
  )
}

function StatePanel(props: {
  title: string
  description: string
  tone?: MetricTone
  action?: React.ReactNode
}) {
  return (
    <div className='border-border bg-card rounded-xl border px-4 py-8 text-center'>
      <StatusBadge
        label={props.title}
        variant={statusVariantFromTone(props.tone ?? 'neutral')}
        copyable={false}
      />
      <p className='text-muted-foreground mx-auto mt-3 max-w-2xl text-sm text-pretty'>
        {props.description}
      </p>
      {props.action ? <div className='mt-4'>{props.action}</div> : null}
    </div>
  )
}

function PartialDataBanner(props: { visible: boolean }) {
  const { t } = useTranslation()
  if (!props.visible) return null

  return (
    <div className='border-warning/40 bg-warning/5 flex flex-col gap-2 rounded-xl border px-4 py-3 text-sm sm:flex-row sm:items-center sm:justify-between'>
      <div className='flex min-w-0 items-center gap-2'>
        <AlertTriangle className='text-warning size-4 shrink-0' />
        <span className='text-foreground font-medium'>
          {t('Partial trend data')}
        </span>
        <span className='text-muted-foreground truncate'>
          {t('Summary metrics loaded, but trend buckets are unavailable.')}
        </span>
      </div>
      <StatusBadge label={t('Partial')} variant='warning' copyable={false} />
    </div>
  )
}

function RiskStrip(props: {
  summary: MCPSummary
  onOpen: (target: MCPSectionTarget) => void
}) {
  const { t } = useTranslation()
  const trends = normalizeMCPSummaryOperationsTrends(
    props.summary.operations_trends
  )
  const reviewQueue = props.summary.review_queue
  const callFailures =
    (props.summary.calls?.error_calls ?? 0) +
    (props.summary.calls?.timeout_calls ?? 0)
  const auditFailures =
    (props.summary.audit?.error ?? 0) + (props.summary.audit?.timeout ?? 0)
  const billingAnomalies =
    trends.billing_anomalies.unsettled_success_calls +
    trends.billing_anomalies.failed_charged_calls +
    trends.billing_anomalies.missing_debit_events

  const risks = [
    {
      key: 'review',
      label: t('Review queue'),
      value: formatNumber(reviewQueue?.total ?? 0),
      detail: `${t('Critical')} ${formatNumber(reviewQueue?.critical_count ?? 0)} · ${t('Warning')} ${formatNumber(reviewQueue?.warning_count ?? 0)}`,
      tone: (reviewQueue?.critical_count ?? 0) > 0 ? 'danger' : 'warning',
      target: { section: 'proxy-servers' } satisfies MCPSectionTarget,
    },
    {
      key: 'calls',
      label: t('Call failures'),
      value: formatNumber(callFailures),
      detail: `${t('Success')} ${formatRate(props.summary.calls?.success_rate)}`,
      tone: callFailures > 0 ? 'warning' : 'success',
      target: {
        section: 'tool-calls',
        search: { callStatus: ['error', 'timeout'] },
      } satisfies MCPSectionTarget,
    },
    {
      key: 'bridge',
      label: t('Bridge failures'),
      value: formatNumber(auditFailures),
      detail: `${t('Online Clients')} ${formatNumber(props.summary.bridge?.online_clients ?? 0)}`,
      tone: auditFailures > 0 ? 'warning' : 'success',
      target: {
        section: 'audit-logs',
        search: { auditStatus: ['error', 'timeout'] },
      } satisfies MCPSectionTarget,
    },
    {
      key: 'billing',
      label: t('Billing anomalies'),
      value: formatNumber(billingAnomalies),
      detail: `${t('Unsettled')} ${formatNumber(props.summary.calls?.unsettled ?? 0)}`,
      tone: billingAnomalies > 0 ? 'danger' : 'success',
      target: {
        section: 'billing-events',
        search: { billingSourceKind: ['mcp_tool_call'] },
      } satisfies MCPSectionTarget,
    },
  ] satisfies Array<{
    key: string
    label: string
    value: string
    detail: string
    tone: MetricTone
    target: MCPSectionTarget
  }>

  return (
    <div className='grid gap-2 md:grid-cols-4'>
      {risks.map((risk) => (
        <button
          key={risk.key}
          type='button'
          className='border-border bg-card hover:bg-muted/30 focus-visible:ring-ring/50 flex min-w-0 items-center justify-between gap-3 rounded-xl border px-3 py-2 text-left transition-colors outline-none focus-visible:ring-3'
          onClick={() => props.onOpen(risk.target)}
        >
          <div className='min-w-0'>
            <div className='text-muted-foreground truncate text-xs'>
              {risk.label}
            </div>
            <div className='text-foreground mt-1 truncate text-xs'>
              {risk.detail}
            </div>
          </div>
          <StatusBadge
            label={risk.value}
            variant={statusVariantFromTone(risk.tone)}
            copyable={false}
          />
        </button>
      ))}
    </div>
  )
}

function MiniBars(props: {
  title: string
  values: number[]
  emptyLabel: string
  tone?: 'neutral' | 'warning'
}) {
  const maxValue = Math.max(1, ...props.values)
  return (
    <div>
      <div className='text-muted-foreground mb-2 flex items-center justify-between gap-2 text-xs'>
        <span>{props.title}</span>
        <span className='tabular-nums'>
          {props.values.length > 0
            ? formatNumber(props.values[props.values.length - 1])
            : props.emptyLabel}
        </span>
      </div>
      <div className='flex h-20 items-end gap-1'>
        {props.values.length > 0 ? (
          props.values.map((value, index) => (
            <div
              key={`${props.title}-${index}`}
              className={cn(
                'min-w-1 flex-1 rounded-sm',
                props.tone === 'warning' ? 'bg-warning/70' : 'bg-foreground/70'
              )}
              style={{
                height: `${Math.max(8, Math.round((value / maxValue) * 100))}%`,
              }}
            />
          ))
        ) : (
          <div className='bg-muted h-2 w-full rounded-sm' />
        )}
      </div>
    </div>
  )
}

function OperationsTrends(props: {
  summary: MCPSummary
  onOpen: (target: MCPSectionTarget) => void
}) {
  const { t } = useTranslation()
  const trends = normalizeMCPSummaryOperationsTrends(
    props.summary.operations_trends
  )
  const latestBridge =
    trends.bridge_online[trends.bridge_online.length - 1]?.online_clients ?? 0
  const objectCount = trends.openapi_storage.reduce(
    (sum, bucket) => sum + bucket.object_count,
    0
  )
  const totalBytes = trends.openapi_storage.reduce(
    (sum, bucket) => sum + bucket.total_bytes,
    0
  )

  return (
    <SurfacePanel
      title={t('Operations trends')}
      description={t('Bridge presence and OpenAPI binary storage over time')}
      action={
        <Button
          size='sm'
          variant='outline'
          onClick={() => props.onOpen({ section: 'openapi-objects' })}
        >
          {t('Open objects')}
          <ArrowRight className='size-3.5' />
        </Button>
      }
    >
      <div className='grid gap-4 p-4 lg:grid-cols-[1fr_1fr]'>
        <div className='grid gap-3 sm:grid-cols-2'>
          <MetricTile
            label={t('Bridge online')}
            value={formatNumber(latestBridge)}
            detail={t('Latest online client count')}
            tone={latestBridge > 0 ? 'success' : 'neutral'}
            icon={Cable}
            onOpen={() =>
              props.onOpen({
                section: 'bridge-clients',
                search: { clientStatus: ['online'] },
              })
            }
          />
          <MetricTile
            label={t('OpenAPI storage')}
            value={formatSize(totalBytes)}
            detail={`${formatNumber(objectCount)} ${t('Objects')}`}
            tone={objectCount > 0 ? 'info' : 'neutral'}
            icon={HardDrive}
            onOpen={() => props.onOpen({ section: 'openapi-objects' })}
          />
        </div>
        <div className='grid gap-4 sm:grid-cols-2'>
          <MiniBars
            title={t('Online clients')}
            values={trends.bridge_online.map((bucket) => bucket.online_clients)}
            emptyLabel={t('No data')}
          />
          <MiniBars
            title={t('Stored bytes')}
            values={trends.openapi_storage.map((bucket) => bucket.total_bytes)}
            emptyLabel={t('No data')}
            tone='warning'
          />
        </div>
      </div>
    </SurfacePanel>
  )
}

function ReviewQueue(props: {
  summary: MCPSummary
  onOpen: (target: MCPSectionTarget) => void
}) {
  const { t } = useTranslation()
  const queue = props.summary.review_queue
  const items = queue?.items ?? []
  const truncated =
    queue?.truncated || (queue?.visible_count ?? 0) < (queue?.total ?? 0)
  let statusLabel = t('Visible')
  let statusVariant: StatusVariant = 'info'

  if (truncated) {
    statusLabel = t('More records')
    statusVariant = 'warning'
  } else if (items.length === 0) {
    statusLabel = t('Clear')
    statusVariant = 'success'
  }

  return (
    <SurfacePanel
      title={t('Review queue')}
      description={t('Operational items sorted by risk and recency')}
      action={
        <StatusBadge
          label={statusLabel}
          variant={statusVariant}
          copyable={false}
        />
      }
    >
      <div className='space-y-2 p-4'>
        {items.length === 0 ? (
          <StatePanel
            title={t('No review items')}
            description={t(
              'No MCP operations items require manual review in this window.'
            )}
            tone='success'
          />
        ) : (
          items.slice(0, 6).map((item) => {
            const critical = item.severity === 'critical'
            return (
              <button
                key={`${item.category}-${item.target_id}-${item.reasons.join('-')}`}
                type='button'
                className='border-border bg-background hover:bg-muted/40 focus-visible:ring-ring/50 grid w-full gap-2 rounded-lg border px-3 py-2 text-left transition-colors outline-none focus-visible:ring-3'
                onClick={() => props.onOpen(reviewItemTarget(item))}
              >
                <div className='flex min-w-0 flex-wrap items-center gap-2'>
                  <StatusBadge
                    label={t(getMCPReviewCategoryLabel(item.category))}
                    variant={critical ? 'danger' : 'warning'}
                    copyable={false}
                  />
                  <span className='text-foreground truncate text-sm font-medium'>
                    {item.target_name || item.target_id}
                  </span>
                </div>
                <div className='flex flex-wrap gap-1'>
                  {item.reasons.map((reason) => (
                    <span
                      key={reason}
                      className='border-border text-muted-foreground rounded-md border px-1.5 py-0.5 text-xs'
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
          })
        )}
      </div>
    </SurfacePanel>
  )
}

function TopTools(props: { summary: MCPSummary }) {
  const { t } = useTranslation()
  const tools = props.summary.top_tools ?? []

  return (
    <SurfacePanel
      title={t('Top tools')}
      description={t('Highest MCP tool traffic in the current window')}
    >
      <div className='space-y-2 p-4'>
        {tools.length === 0 ? (
          <StatePanel
            title={t('No tool calls')}
            description={t('No MCP tools have call volume in this window.')}
          />
        ) : (
          tools.slice(0, 6).map((tool) => (
            <div
              key={tool.tool_name}
              className='border-border bg-background grid gap-2 rounded-lg border px-3 py-2 sm:grid-cols-[minmax(0,1fr)_auto]'
            >
              <div className='min-w-0'>
                <div className='text-foreground truncate text-sm font-medium'>
                  {tool.tool_name}
                </div>
                <div className='text-muted-foreground mt-1 flex flex-wrap gap-2 text-xs'>
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
          ))
        )}
      </div>
    </SurfacePanel>
  )
}

function RecentErrors(props: {
  summary: MCPSummary
  onOpen: (target: MCPSectionTarget) => void
}) {
  const { t } = useTranslation()
  const errors = props.summary.recent_errors ?? []

  return (
    <SurfacePanel
      title={t('Recent activity exceptions')}
      description={t('Latest bridge and tool-call failures')}
      action={
        <Button
          size='sm'
          variant='outline'
          onClick={() =>
            props.onOpen({
              section: 'tool-calls',
              search: { callStatus: ['error', 'timeout'] },
            })
          }
        >
          {t('Open calls')}
          <ArrowRight className='size-3.5' />
        </Button>
      }
    >
      <div className='space-y-2 p-4'>
        {errors.length === 0 ? (
          <StatePanel
            title={t('No recent errors')}
            description={t(
              'No bridge or MCP tool failures were reported in this window.'
            )}
            tone='success'
          />
        ) : (
          errors.slice(0, 8).map((error) => (
            <button
              key={`${error.source}-${error.request_id}-${error.created_at}`}
              type='button'
              className='border-border bg-background hover:bg-muted/40 focus-visible:ring-ring/50 grid w-full gap-2 rounded-lg border px-3 py-2 text-left transition-colors outline-none focus-visible:ring-3 sm:grid-cols-[minmax(0,1fr)_auto]'
              onClick={() => props.onOpen(recentErrorTarget(error))}
            >
              <div className='min-w-0'>
                <div className='flex min-w-0 flex-wrap items-center gap-2'>
                  <StatusBadge
                    label={t(
                      error.source === 'bridge_audit' ? 'Bridge' : 'Tool Call'
                    )}
                    variant={
                      error.source === 'bridge_audit' ? 'info' : 'warning'
                    }
                    copyable={false}
                  />
                  <span className='text-foreground truncate text-sm font-medium'>
                    {error.tool_name || '-'}
                  </span>
                </div>
                <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
                  {error.error_message || error.error_code || '-'}
                </div>
                <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
                  {error.request_id || '-'}
                </div>
              </div>
              <div className='text-muted-foreground text-left text-xs whitespace-nowrap sm:text-right'>
                {formatTimestampToDate(error.created_at)}
              </div>
            </button>
          ))
        )}
      </div>
    </SurfacePanel>
  )
}

function SummaryMetrics(props: {
  summary: MCPSummary
  onOpen: (target: MCPSectionTarget) => void
}) {
  const { t } = useTranslation()
  const summary = props.summary

  return (
    <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
      <MetricTile
        label={t('Enabled tools')}
        value={formatNumber(summary.tools?.enabled ?? 0)}
        detail={`${t('Total')} ${formatNumber(summary.tools?.total ?? 0)} · ${t('Remote')} ${formatNumber(summary.tools?.remote ?? 0)}`}
        icon={Wrench}
        onOpen={() => props.onOpen({ section: 'tools' })}
      />
      <MetricTile
        label={t('Online clients')}
        value={formatNumber(summary.bridge?.online_clients ?? 0)}
        detail={`${t('Registered')} ${formatNumber(summary.bridge?.total_clients ?? 0)} · ${t('Active 24h')} ${formatNumber(summary.bridge?.active_clients ?? 0)}`}
        tone={(summary.bridge?.online_clients ?? 0) > 0 ? 'success' : 'neutral'}
        icon={Cable}
        onOpen={() =>
          props.onOpen({
            section: 'bridge-clients',
            search: { clientStatus: ['online'] },
          })
        }
      />
      <MetricTile
        label={t('Tool calls')}
        value={formatNumber(summary.calls?.total_calls ?? 0)}
        detail={`${t('Success')} ${formatRate(summary.calls?.success_rate)} · ${t('Avg')} ${formatDuration(summary.calls?.avg_duration_ms)}`}
        tone={(summary.calls?.success_rate ?? 0) >= 95 ? 'success' : 'warning'}
        icon={Boxes}
        onOpen={() => props.onOpen({ section: 'tool-calls' })}
      />
      <MetricTile
        label={t('Quota used')}
        value={formatQuota(summary.calls?.quota ?? 0)}
        detail={`${t('Settled')} ${formatNumber(summary.calls?.settled_calls ?? 0)} · ${t('Unsettled')} ${formatNumber(summary.calls?.unsettled ?? 0)}`}
        tone={(summary.calls?.unsettled ?? 0) > 0 ? 'warning' : 'neutral'}
        icon={Clock3}
        onOpen={() =>
          props.onOpen({
            section: 'billing-events',
            search: { billingSourceKind: ['mcp_tool_call'] },
          })
        }
      />
    </div>
  )
}

export function UIV2MCPOperations() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const isAdmin = useIsAdmin()
  const requestParams = useMemo(
    () => ({
      scope: 'all' as const,
      window_seconds: MCP_OPERATIONS_WINDOW_SECONDS,
    }),
    []
  )

  const { data, error, isError, isFetching, isLoading, refetch } = useQuery({
    queryKey: mcpQueryKeys.summary(requestParams),
    queryFn: async () => {
      const result = await getMCPSummary(requestParams)
      if (!result.success) {
        throw mcpQueryError(result.message, 'Failed to load MCP overview')
      }
      return result.data
    },
    enabled: isAdmin,
    refetchInterval: 30 * 1000,
    staleTime: 10 * 1000,
  })

  const openDetail = (target: MCPSectionTarget) => {
    void navigate({
      to: '/mcp/$section',
      params: { section: target.section },
      search: buildDetailSearch(target),
    })
  }

  if (!isAdmin) {
    return (
      <StatePanel
        title={t('Admin access required')}
        description={t(
          'The UI v2 MCP operations pilot is limited to administrators.'
        )}
        tone='warning'
      />
    )
  }

  if (isLoading) {
    return <LoadingSurface />
  }

  if (isError || !data) {
    return (
      <StatePanel
        title={t('Failed to load MCP overview')}
        description={mcpQueryErrorMessage(
          error,
          t('Failed to load MCP overview')
        )}
        tone='danger'
        action={
          <Button variant='outline' onClick={() => void refetch()}>
            <RefreshCw className='size-4' />
            {t('Retry')}
          </Button>
        }
      />
    )
  }

  const trends = normalizeMCPSummaryOperationsTrends(data.operations_trends)
  const partialTrends =
    !data.operations_trends ||
    (trends.bridge_online.length === 0 &&
      trends.openapi_storage.length === 0 &&
      trends.proxy_error_top_n.length === 0)

  return (
    <div className='space-y-4'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex flex-wrap items-center gap-2'>
          <StatusBadge
            label={t('24h window')}
            variant='neutral'
            copyable={false}
          />
          <StatusBadge
            label={
              isFetching
                ? t('Refreshing')
                : `${t('Generated')} ${formatTimestampToDate(data.generated_at)}`
            }
            variant={isFetching ? 'info' : 'neutral'}
            copyable={false}
          />
          {partialTrends ? (
            <StatusBadge
              label={t('Partial data')}
              variant='warning'
              copyable={false}
            />
          ) : null}
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

      <SummaryMetrics summary={data} onOpen={openDetail} />
      <RiskStrip summary={data} onOpen={openDetail} />
      <PartialDataBanner visible={partialTrends} />

      {isSummaryEmpty(data) ? (
        <StatePanel
          title={t('No MCP operations yet')}
          description={t(
            'No MCP tools, Bridge clients, calls, or audit activity were found in this 24 hour window.'
          )}
        />
      ) : null}

      <div className='grid gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(360px,0.85fr)]'>
        <OperationsTrends summary={data} onOpen={openDetail} />
        <ReviewQueue summary={data} onOpen={openDetail} />
      </div>

      <div className='grid gap-4 xl:grid-cols-2'>
        <TopTools summary={data} />
        <RecentErrors summary={data} onOpen={openDetail} />
      </div>

      <SurfacePanel
        title={t('Operational drill-ins')}
        description={t(
          'Jump into the existing detailed MCP sections without leaving the current UI available.'
        )}
      >
        <div className='grid gap-2 p-4 sm:grid-cols-2 lg:grid-cols-4'>
          {[
            {
              label: t('Proxy servers'),
              target: { section: 'proxy-servers' } satisfies MCPSectionTarget,
            },
            {
              label: t('Bridge audit logs'),
              target: { section: 'audit-logs' } satisfies MCPSectionTarget,
            },
            {
              label: t('Binary objects'),
              target: { section: 'openapi-objects' } satisfies MCPSectionTarget,
            },
            {
              label: t('Billing events'),
              target: {
                section: 'billing-events',
                search: { billingSourceKind: ['mcp_tool_call'] },
              } satisfies MCPSectionTarget,
            },
          ].map((item) => (
            <Button
              key={item.label}
              variant='outline'
              className='justify-between'
              onClick={() => openDetail(item.target)}
            >
              {item.label}
              <ArrowRight className='size-4' />
            </Button>
          ))}
        </div>
      </SurfacePanel>
    </div>
  )
}
