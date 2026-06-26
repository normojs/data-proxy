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
import { useMutation, useQuery } from '@tanstack/react-query'
import {
  Copy,
  Check,
  Route,
  FileSearch,
  RefreshCw,
  Settings2,
  AlertTriangle,
  Headphones,
  Monitor,
  Cloud,
  Globe,
  ShieldCheck,
  UserCog,
  Info,
  Activity,
  Download,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import {
  formatLogQuota,
  formatTimestampToDate,
  formatTokens,
  formatUseTime,
} from '@/lib/format'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { DynamicPricingBreakdown } from '@/features/pricing/components/dynamic-pricing-breakdown'
import {
  generateRequestDiagnosticReport,
  getRequestDiagnosticBundleUrl,
  getRequestDiagnosticReport,
  getRequestLogTrace,
} from '../../api'
import type { UsageLog } from '../../data/schema'
import {
  parseLogOther,
  getParamOverrideActionLabel,
  parseAuditLine,
  decodeBillingExprB64,
  getTieredBillingSummary,
  hasAnyCacheTokens,
  isViolationFeeLog,
  getFirstResponseTimeColor,
  getResponseTimeColor,
} from '../../lib/format'
import {
  getLogTypeConfig,
  isPerCallBilling,
  isTimingLogType,
} from '../../lib/utils'
import type {
  ChannelFailoverEvent,
  LogOtherData,
  RequestDiagnosticReport,
  RequestConversionMeta,
  RequestLogTrace,
} from '../../types'

function timingTextColorClass(
  variant: 'success' | 'warning' | 'danger'
): string {
  if (variant === 'success') return 'text-emerald-600'
  if (variant === 'warning') return 'text-amber-600'
  return 'text-rose-600'
}

function DetailRow(props: {
  label: React.ReactNode
  value: React.ReactNode
  mono?: boolean
  muted?: boolean
}) {
  return (
    <div className='grid min-w-0 grid-cols-[5.25rem_minmax(0,1fr)] gap-2 text-sm sm:grid-cols-[7rem_minmax(0,1fr)] sm:gap-3'>
      <span className='text-muted-foreground min-w-0 text-xs'>
        {props.label}
      </span>
      <span
        className={cn(
          'max-w-full min-w-0 text-xs break-all sm:break-words',
          props.mono && 'font-mono',
          props.muted && 'text-muted-foreground'
        )}
      >
        {props.value}
      </span>
    </div>
  )
}

function DetailSection(props: {
  icon?: React.ReactNode
  label: string
  variant?: 'default' | 'danger'
  children: React.ReactNode
}) {
  const isDanger = props.variant === 'danger'
  return (
    <div className='min-w-0 space-y-1.5'>
      <Label
        className={cn(
          'flex items-center gap-1.5 text-xs font-semibold',
          isDanger && 'text-red-500'
        )}
      >
        {props.icon}
        {props.label}
      </Label>
      <div
        className={cn(
          'min-w-0 space-y-1 overflow-hidden rounded-md border p-2.5 max-sm:p-2',
          isDanger
            ? 'border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/20'
            : 'bg-muted/30'
        )}
      >
        {props.children}
      </div>
    </div>
  )
}

function formatRatio(ratio: number | undefined): string {
  if (ratio == null) return '-'
  return ratio.toFixed(4)
}

function hasConversionMeta(other: LogOtherData | null): boolean {
  const meta = other?.request_conversion_meta
  return !!meta && Object.keys(meta).length > 0
}

function stringifyConversionValue(value: unknown): string {
  if (value == null) return ''
  if (Array.isArray(value)) return value.filter(Boolean).join(', ')
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  if (typeof value === 'string') return value
  if (typeof value === 'number') return String(value)
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function formatConversionToken(
  value: string | undefined,
  t: (key: string) => string
): string {
  if (!value) return ''
  const labels: Record<string, string> = {
    auto: t('Auto'),
    native: t('Native'),
    responses: t('Responses'),
    chat_completions: t('Chat Completions'),
    chat_completions_compat: t('Chat Completions Compatibility'),
    disabled: t('Disabled'),
    native_responses: t('Native Responses'),
    unknown: t('Unknown'),
    convert_to_chat_completions: t('Converted to Chat Completions'),
    default: t('Default'),
    legacy: t('Default'),
    off: t('Off'),
    openai: t('OpenAI'),
    previous_response_id: t('Previous Response ID'),
    deepseek: t('DeepSeek'),
    openrouter: t('OpenRouter'),
    qwen_enable_thinking: t('Qwen enable_thinking'),
    minimax_reasoning_split: t('MiniMax reasoning_split'),
    low_high: t('Low/High Mapping'),
    completed: t('Completed'),
    incomplete: t('Incomplete'),
    failed: t('Failed'),
    ok: t('OK'),
    error: t('Error'),
    unique_call_id: t('Unique Call ID'),
    filter_and_direct_answer: t('Filter and direct answer'),
    native_responses_required: t('Native Responses required'),
    executor_bridge: t('Executor bridge'),
    reject_with_clear_error: t('Reject with clear error'),
  }
  return labels[value] || value
}

function formatHostedWebSearchExecutorEvent(
  event: NonNullable<
    RequestConversionMeta['hosted_web_search_executor_events']
  >[number],
  index: number,
  t: (key: string) => string
): string {
  const parts = [
    `#${index + 1}`,
    event.status ? formatConversionToken(event.status, t) : '',
    event.duration_ms != null ? `${t('Duration')}: ${event.duration_ms}ms` : '',
    event.results_count != null
      ? `${t('Results')}: ${event.results_count}`
      : '',
    event.answer_chars != null
      ? `${t('Answer Chars')}: ${event.answer_chars}`
      : '',
    event.error ? `${t('Error')}: ${event.error}` : '',
  ].filter(Boolean)
  return parts.join(' · ')
}

function ConversionMetaRows(props: {
  meta: NonNullable<LogOtherData['request_conversion_meta']>
}) {
  const { t } = useTranslation()
  const { meta } = props
  const rows: Array<{ label: string; value: React.ReactNode; mono?: boolean }> =
    []

  if (meta.responses_protocol || meta.upstream_protocol) {
    const protocol = formatConversionToken(meta.responses_protocol, t)
    const upstream = formatConversionToken(meta.upstream_protocol, t)
    rows.push({
      label: t('Protocol'),
      value:
        protocol && upstream
          ? `${protocol} -> ${upstream}`
          : protocol || upstream,
      mono: true,
    })
  }

  if (meta.responses_protocol_decision) {
    rows.push({
      label: t('Decision'),
      value: formatConversionToken(meta.responses_protocol_decision, t),
      mono: true,
    })
  }

  if (meta.responses_channel_capability) {
    rows.push({
      label: t('Channel Capability'),
      value: formatConversionToken(meta.responses_channel_capability, t),
      mono: true,
    })
  }

  if (
    typeof meta.responses_native_supported === 'boolean' ||
    typeof meta.responses_chat_preferred === 'boolean'
  ) {
    const values = [
      typeof meta.responses_native_supported === 'boolean'
        ? `${t('Native')}: ${meta.responses_native_supported ? t('Yes') : t('No')}`
        : '',
      typeof meta.responses_chat_preferred === 'boolean'
        ? `${t('Chat Preferred')}: ${meta.responses_chat_preferred ? t('Yes') : t('No')}`
        : '',
    ].filter(Boolean)
    rows.push({
      label: t('Auto Check'),
      value: values.join(' · '),
    })
  }

  if (meta.responses_reasoning_adapter) {
    const adapter = formatConversionToken(meta.responses_reasoning_adapter, t)
    rows.push({
      label: t('Reasoning Adapter'),
      value: meta.responses_reasoning_adapter_source
        ? `${adapter} (${formatConversionToken(meta.responses_reasoning_adapter_source, t)})`
        : adapter,
      mono: true,
    })
  }

  if (meta.responses_reasoning_adapter_recommended) {
    rows.push({
      label: t('Recommended Adapter'),
      value: formatConversionToken(
        meta.responses_reasoning_adapter_recommended,
        t
      ),
      mono: true,
    })
  }

  if (
    Array.isArray(meta.reasoning_params) &&
    meta.reasoning_params.length > 0
  ) {
    rows.push({
      label: t('Reasoning Params'),
      value: meta.reasoning_params.join(', '),
      mono: true,
    })
  }

  if (meta.reasoning_effort_mapped) {
    rows.push({
      label: t('Mapped Effort'),
      value: meta.reasoning_effort_mapped,
      mono: true,
    })
  }

  if (
    Array.isArray(meta.hosted_tools_functionized) &&
    meta.hosted_tools_functionized.length > 0
  ) {
    rows.push({
      label: t('Hosted Tools'),
      value: meta.hosted_tools_functionized.join(', '),
      mono: true,
    })
  }

  if (
    Array.isArray(meta.hosted_tools_requested) &&
    meta.hosted_tools_requested.length > 0
  ) {
    rows.push({
      label: t('Requested Hosted Tools'),
      value: meta.hosted_tools_requested.join(', '),
      mono: true,
    })
  }

  if (
    Array.isArray(meta.hosted_tools_filtered) &&
    meta.hosted_tools_filtered.length > 0
  ) {
    rows.push({
      label: t('Filtered Hosted Tools'),
      value: meta.hosted_tools_filtered.join(', '),
      mono: true,
    })
  }

  if (meta.hosted_tools_policy) {
    rows.push({
      label: t('Hosted Tools Policy'),
      value: formatConversionToken(meta.hosted_tools_policy, t),
      mono: true,
    })
  }

  if (meta.hosted_tools_rejected) {
    rows.push({
      label: t('Hosted Tools Action'),
      value: t('Rejected'),
    })
  }

  if (meta.hosted_tools_executor_bridge_requested) {
    rows.push({
      label: t('Executor Bridge'),
      value: meta.hosted_tools_executor_bridge_ready
        ? t('Ready')
        : t('Requested'),
    })
  }

  if (
    meta.hosted_web_search_executor_calls != null ||
    meta.hosted_web_search_executor_error
  ) {
    const parts = [
      meta.hosted_web_search_executor_calls != null
        ? `${t('Calls')}: ${meta.hosted_web_search_executor_calls}`
        : '',
      meta.hosted_web_search_executor_error
        ? `${t('Error')}: ${meta.hosted_web_search_executor_error}`
        : '',
    ].filter(Boolean)
    rows.push({
      label: t('Web Search Executor'),
      value: parts.join(' · '),
      mono: true,
    })
  }

  if (
    Array.isArray(meta.hosted_web_search_executor_events) &&
    meta.hosted_web_search_executor_events.length > 0
  ) {
    const visibleEvents = meta.hosted_web_search_executor_events.slice(0, 4)
    const moreEvents = meta.hosted_web_search_executor_events.length - 4
    rows.push({
      label: t('Executor Events'),
      value: (
        <div className='space-y-0.5'>
          {visibleEvents.map((event, index) => (
            <div key={`${event.tool_call_id || index}-${index}`}>
              {formatHostedWebSearchExecutorEvent(event, index, t)}
            </div>
          ))}
          {moreEvents > 0 && (
            <div className='text-muted-foreground'>
              {t('{{count}} more executor events', { count: moreEvents })}
            </div>
          )}
        </div>
      ),
      mono: true,
    })
  }

  if (meta.hosted_tools_direct_answer_hint) {
    rows.push({
      label: t('Hosted Direct Answer Hint'),
      value: t('Injected'),
    })
  }

  if (
    Array.isArray(meta.unsupported_tools_filtered) &&
    meta.unsupported_tools_filtered.length > 0
  ) {
    rows.push({
      label: t('Filtered Tools'),
      value: meta.unsupported_tools_filtered.join(', '),
      mono: true,
    })
  }

  if (
    Array.isArray(meta.history_restore_sources) &&
    meta.history_restore_sources.length > 0
  ) {
    rows.push({
      label: t('History Source'),
      value: meta.history_restore_sources
        .map((source) => formatConversionToken(source, t))
        .join(' · '),
    })
  }

  const counters = [
    meta.history_restored_count
      ? `${t('Restored')}: ${meta.history_restored_count}`
      : '',
    meta.history_recorded_count
      ? `${t('Recorded')}: ${meta.history_recorded_count}`
      : '',
    meta.input_provided_tools_count
      ? `${t('Loaded Tools')}: ${meta.input_provided_tools_count}`
      : '',
    meta.namespace_tools_flattened
      ? `${t('Namespace')}: ${meta.namespace_tools_flattened}`
      : '',
    meta.reasoning_backfilled_count
      ? `${t('Reasoning Backfilled')}: ${meta.reasoning_backfilled_count}`
      : '',
  ].filter(Boolean)
  if (counters.length > 0) {
    rows.push({
      label: t('Context'),
      value: counters.join(' · '),
    })
  }

  if (meta.chat_sse_fallback) {
    rows.push({
      label: t('Fallback'),
      value: t('Chat SSE aggregated'),
    })
  }

  if (meta.responses_terminal_status) {
    rows.push({
      label: t('Terminal Status'),
      value: formatConversionToken(meta.responses_terminal_status, t),
      mono: true,
    })
  }

  if (meta.responses_incomplete_details) {
    rows.push({
      label: t('Incomplete Details'),
      value: stringifyConversionValue(meta.responses_incomplete_details),
      mono: true,
    })
  }

  if (meta.responses_terminal_error) {
    rows.push({
      label: t('Terminal Error'),
      value: stringifyConversionValue(meta.responses_terminal_error),
      mono: true,
    })
  }

  if (Array.isArray(meta.notes) && meta.notes.length > 0) {
    rows.push({
      label: t('Notes'),
      value: meta.notes.join(' · '),
    })
  }

  if (rows.length === 0) return null

  return (
    <div className='border-border/70 mt-2 space-y-1 border-t pt-2'>
      {rows.map((row, idx) => (
        <DetailRow
          key={idx}
          label={row.label}
          value={row.value}
          mono={row.mono}
        />
      ))}
    </div>
  )
}

function traceStatusVariant(status: string): StatusBadgeProps['variant'] {
  if (status === 'completed') return 'green'
  if (status === 'error') return 'red'
  if (status === 'not_found') return 'grey'
  return 'neutral'
}

function traceStatusLabel(status: string, t: (key: string) => string): string {
  const labels: Record<string, string> = {
    completed: t('Completed'),
    error: t('Error'),
    logged: t('Logged'),
    not_found: t('No trace data'),
  }
  return labels[status] || status
}

function safeTraceMeta(
  meta: RequestLogTrace['diagnostics']['request_conversion_meta']
): RequestConversionMeta | undefined {
  if (!meta || typeof meta !== 'object' || Array.isArray(meta)) return undefined
  return meta as RequestConversionMeta
}

function safeChannelFailoverEvents(
  adminInfo: RequestLogTrace['diagnostics']['admin_info']
): ChannelFailoverEvent[] {
  if (!adminInfo || typeof adminInfo !== 'object' || Array.isArray(adminInfo)) {
    return []
  }
  const events = adminInfo.channel_failover
  if (!Array.isArray(events)) return []
  return events.filter(
    (event): event is ChannelFailoverEvent =>
      !!event && typeof event === 'object' && !Array.isArray(event)
  )
}

function channelFailoverEventLabel(
  event: ChannelFailoverEvent,
  t: (key: string) => string
): string {
  if (event.event === 'selected') return t('Selected')
  if (event.event === 'failed') return t('Failed')
  return event.event || t('Unknown')
}

function channelFailoverEventVariant(
  event: ChannelFailoverEvent
): StatusBadgeProps['variant'] {
  if (event.event === 'failed') return 'red'
  if (event.event === 'selected') return 'green'
  return 'neutral'
}

function channelFailoverChannelText(event: ChannelFailoverEvent): string {
  const id = event.channel_id ? `#${event.channel_id}` : '#-'
  return event.channel_name ? `${id} ${event.channel_name}` : id
}

function compactJoin(parts: Array<string | number | undefined | null>): string {
  return parts
    .filter((part) => part !== undefined && part !== null && `${part}` !== '')
    .map((part) => `${part}`)
    .join(' · ')
}

function ChannelFailoverTraceList(props: {
  events: ChannelFailoverEvent[]
}) {
  const { t } = useTranslation()
  const { events } = props
  if (events.length === 0) return null

  return (
    <div className='border-border/70 space-y-1.5 border-t pt-2'>
      <Label className='text-xs font-semibold'>{t('Channel Failover')}</Label>
      <div className='space-y-1.5'>
        {events.map((event, idx) => {
          const statusText = compactJoin([
            event.status_code,
            event.error_code,
            event.error_type,
          ])
          const healthText = compactJoin([
            event.health_action,
            event.runtime_status,
            event.temporarily_unavailable ? t('Temporary Circuit') : undefined,
          ])
          const cooldownUntil =
            event.cooldown_until || event.health_cooldown_until
          const failureCount =
            event.consecutive_failures || event.health_failure_count
          const reason = event.reason || event.health_reason

          return (
            <div
              key={`${event.ts ?? idx}-${event.event ?? 'event'}-${idx}`}
              className='bg-background/60 min-w-0 space-y-1 rounded border p-2'
            >
              <div className='flex min-w-0 items-center justify-between gap-2'>
                <div className='flex min-w-0 items-center gap-1.5'>
                  <StatusBadge
                    label={channelFailoverEventLabel(event, t)}
                    variant={channelFailoverEventVariant(event)}
                    size='sm'
                    copyable={false}
                  />
                  <span className='min-w-0 truncate font-mono text-[11px]'>
                    {channelFailoverChannelText(event)}
                  </span>
                </div>
                {event.ts ? (
                  <span className='text-muted-foreground shrink-0 font-mono text-[11px]'>
                    {formatTimestampToDate(event.ts, 'seconds')}
                  </span>
                ) : null}
              </div>

              <DetailRow
                label={t('Retry Index')}
                value={event.retry_index ?? '-'}
                mono
              />
              {event.remaining_retries != null && (
                <DetailRow
                  label={t('Remaining Retries')}
                  value={event.remaining_retries}
                  mono
                />
              )}
              {event.selected_group && (
                <DetailRow
                  label={t('Group')}
                  value={event.selected_group}
                  mono
                />
              )}
              {event.excluded_channel_ids &&
                event.excluded_channel_ids.length > 0 && (
                  <DetailRow
                    label={t('Excluded Channels')}
                    value={event.excluded_channel_ids
                      .map((id) => `#${id}`)
                      .join(' · ')}
                    mono
                  />
                )}
              {typeof event.retry_planned === 'boolean' && (
                <DetailRow
                  label={t('Retry Planned')}
                  value={event.retry_planned ? t('Yes') : t('No Retry')}
                />
              )}
              {statusText && (
                <DetailRow label={t('Status')} value={statusText} mono />
              )}
              {healthText && (
                <DetailRow label={t('Health Action')} value={healthText} />
              )}
              {failureCount != null && (
                <DetailRow
                  label={t('Failure Count')}
                  value={failureCount}
                  mono
                />
              )}
              {cooldownUntil ? (
                <DetailRow
                  label={t('Cooldown Until')}
                  value={formatTimestampToDate(cooldownUntil, 'seconds')}
                  mono
                />
              ) : null}
              {reason && <DetailRow label={t('Reason')} value={reason} />}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function traceTypeCountsText(
  trace: RequestLogTrace,
  t: (key: string) => string
): string {
  const counts = trace.summary.type_counts || {}
  const parts = Object.entries(counts)
    .filter(([, count]) => count > 0)
    .map(([type, count]) => `${traceTypeNameLabel(type, t)} ${count}`)
  return parts.length > 0 ? parts.join(' · ') : '-'
}

function traceTypeNameLabel(type: string, t: (key: string) => string): string {
  const labels: Record<string, string> = {
    topup: t('Top-up'),
    consume: t('Consume'),
    manage: t('Manage'),
    system: t('System'),
    error: t('Error'),
    refund: t('Refund'),
    unknown: t('Unknown'),
  }
  return labels[type] || type
}

function diagnosticSeverityVariant(
  severity: string
): StatusBadgeProps['variant'] {
  if (severity === 'error') return 'red'
  if (severity === 'warning') return 'yellow'
  if (severity === 'ok') return 'green'
  return 'neutral'
}

function diagnosticSeverityLabel(
  severity: string,
  t: (key: string) => string
): string {
  const labels: Record<string, string> = {
    ok: t('OK'),
    warning: t('Warning'),
    error: t('Error'),
    info: t('Info'),
  }
  return labels[severity] || severity
}

function formatBytes(value: number | undefined): string {
  if (!value || value <= 0) return '0 B'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`
  return `${(value / 1024 / 1024).toFixed(1)} MiB`
}

function RequestDiagnosticSection(props: {
  report?: RequestDiagnosticReport
  loading: boolean
  generating: boolean
  errorMessage?: string
  copiedText?: string | null
  onCopy: (text: string) => void
  onGenerate: () => void
  onDownload: () => void
}) {
  const { t } = useTranslation()
  const {
    report,
    loading,
    generating,
    errorMessage,
    copiedText,
    onCopy,
    onGenerate,
    onDownload,
  } = props

  if (loading) {
    return (
      <DetailSection
        icon={<FileSearch className='size-3.5' aria-hidden='true' />}
        label={t('Diagnostic Report')}
      >
        <div className='space-y-2'>
          <Skeleton className='h-4 w-36 rounded' />
          <Skeleton className='h-4 w-full rounded' />
        </div>
      </DetailSection>
    )
  }

  const reportCopyText = report ? JSON.stringify(report, null, 2) : ''
  const findings = report?.report?.findings || []
  const capture = report?.report?.capture
  const hasReport = !!report && report.status !== 'not_found'

  return (
    <DetailSection
      icon={<FileSearch className='size-3.5' aria-hidden='true' />}
      label={t('Diagnostic Report')}
      variant={report?.severity === 'error' ? 'danger' : 'default'}
    >
      <div className='relative min-w-0 space-y-2'>
        {hasReport && (
          <Button
            variant='ghost'
            size='sm'
            className='absolute top-0 right-0 h-5 w-5 p-0'
            onClick={() => onCopy(reportCopyText)}
            title={t('Copy to clipboard')}
            aria-label={t('Copy to clipboard')}
          >
            {copiedText === reportCopyText ? (
              <Check className='size-3 text-green-600' />
            ) : (
              <Copy className='size-3' />
            )}
          </Button>
        )}

        <div className='min-w-0 space-y-1 pr-6'>
          <DetailRow
            label={t('Severity')}
            value={
              <StatusBadge
                label={diagnosticSeverityLabel(report?.severity || 'info', t)}
                variant={diagnosticSeverityVariant(report?.severity || 'info')}
                size='sm'
                copyable={false}
              />
            }
          />
          <DetailRow
            label={t('Summary')}
            value={report?.summary || t('No diagnostic report')}
          />
          {capture && (
            <>
              <DetailRow
                label={t('Capture Status')}
                value={capture.capture_status}
                mono
              />
              <DetailRow
                label={t('Captured Bytes')}
                value={`${t('Upstream')}: ${formatBytes(capture.upstream_body_bytes)} · ${t('Downstream')}: ${formatBytes(capture.downstream_body_bytes)}`}
                mono
              />
              <DetailRow
                label={t('Artifacts')}
                value={String(capture.artifacts?.length || 0)}
                mono
              />
            </>
          )}
        </div>

        {findings.length > 0 && (
          <div className='border-border/70 space-y-1 border-t pt-2'>
            <Label className='text-xs font-semibold'>{t('Findings')}</Label>
            {findings.slice(0, 5).map((finding, index) => (
              <div
                key={`${finding.code}-${index}`}
                className='bg-background/60 min-w-0 rounded border p-2'
              >
                <div className='flex min-w-0 items-center gap-1.5'>
                  <StatusBadge
                    label={diagnosticSeverityLabel(finding.level, t)}
                    variant={diagnosticSeverityVariant(finding.level)}
                    size='sm'
                    copyable={false}
                  />
                  <span className='min-w-0 truncate font-mono text-[11px]'>
                    {finding.code}
                  </span>
                </div>
                <p className='mt-1 text-xs break-words'>{finding.message}</p>
                {finding.detail && (
                  <p className='text-muted-foreground mt-0.5 font-mono text-[11px] break-words'>
                    {finding.detail}
                  </p>
                )}
              </div>
            ))}
          </div>
        )}

        {errorMessage && (
          <p className='text-xs break-words text-red-600 dark:text-red-400'>
            {errorMessage}
          </p>
        )}

        <div className='flex flex-wrap gap-2'>
          <Button
            variant='outline'
            size='sm'
            className='h-8 gap-1.5'
            onClick={onGenerate}
            disabled={generating}
          >
            <RefreshCw
              className={cn('size-3.5', generating && 'animate-spin')}
              aria-hidden='true'
            />
            {hasReport ? t('Regenerate Diagnostic') : t('Generate Diagnostic')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            className='h-8 gap-1.5'
            onClick={onDownload}
            disabled={!report}
          >
            <Download className='size-3.5' aria-hidden='true' />
            {t('Download Bundle')}
          </Button>
        </div>
      </div>
    </DetailSection>
  )
}

function RequestTraceSection(props: {
  trace?: RequestLogTrace
  loading: boolean
  errorMessage?: string
  copiedText?: string | null
  onCopy: (text: string) => void
}) {
  const { t } = useTranslation()
  const { trace, loading, errorMessage, copiedText, onCopy } = props

  if (loading) {
    return (
      <DetailSection
        icon={<Activity className='size-3.5' aria-hidden='true' />}
        label={t('Request Trace')}
      >
        <div className='space-y-2'>
          <Skeleton className='h-4 w-40 rounded' />
          <Skeleton className='h-4 w-full rounded' />
          <Skeleton className='h-4 w-2/3 rounded' />
        </div>
      </DetailSection>
    )
  }

  if (errorMessage) {
    return (
      <DetailSection
        icon={<Activity className='size-3.5' aria-hidden='true' />}
        label={t('Request Trace')}
        variant='danger'
      >
        <p className='text-xs break-words text-red-600 dark:text-red-400'>
          {errorMessage}
        </p>
      </DetailSection>
    )
  }

  if (!trace) return null

  const traceCopyText = JSON.stringify(trace, null, 2)
  const meta = safeTraceMeta(trace.diagnostics.request_conversion_meta)
  const failoverEvents = safeChannelFailoverEvents(
    trace.diagnostics.admin_info
  )
  const errors = Array.isArray(trace.diagnostics.errors)
    ? trace.diagnostics.errors.filter(Boolean)
    : []
  const relatedLogs = trace.logs.slice(0, 3)
  const moreLogsCount = Math.max(trace.logs.length - relatedLogs.length, 0)

  return (
    <DetailSection
      icon={<Activity className='size-3.5' aria-hidden='true' />}
      label={t('Request Trace')}
      variant={trace.summary.status === 'error' ? 'danger' : 'default'}
    >
      <div className='relative min-w-0 space-y-2'>
        <Button
          variant='ghost'
          size='sm'
          className='absolute top-0 right-0 h-5 w-5 p-0'
          onClick={() => onCopy(traceCopyText)}
          title={t('Copy to clipboard')}
          aria-label={t('Copy to clipboard')}
        >
          {copiedText === traceCopyText ? (
            <Check className='size-3 text-green-600' />
          ) : (
            <Copy className='size-3' />
          )}
        </Button>

        <div className='min-w-0 space-y-1 pr-6'>
          <DetailRow
            label={t('Trace Status')}
            value={
              <StatusBadge
                label={traceStatusLabel(trace.summary.status, t)}
                variant={traceStatusVariant(trace.summary.status)}
                size='sm'
                copyable={false}
              />
            }
          />
          <DetailRow
            label={t('Matched Logs')}
            value={`${trace.total}${trace.total > 0 ? ` (${traceTypeCountsText(trace, t)})` : ''}`}
            mono
          />
          {trace.request_ids.length > 0 && (
            <DetailRow
              label={t('Request ID')}
              value={trace.request_ids.join(' · ')}
              mono
            />
          )}
          {trace.upstream_request_ids.length > 0 && (
            <DetailRow
              label={t('Upstream Request ID')}
              value={trace.upstream_request_ids.join(' · ')}
              mono
            />
          )}
          {trace.diagnostics.request_path && (
            <DetailRow
              label={t('Path')}
              value={trace.diagnostics.request_path}
              mono
            />
          )}
          {trace.summary.model_name && (
            <DetailRow
              label={t('Model')}
              value={
                trace.diagnostics.upstream_model_name
                  ? `${trace.summary.model_name} -> ${trace.diagnostics.upstream_model_name}`
                  : trace.summary.model_name
              }
              mono
            />
          )}
          {trace.summary.max_use_time > 0 && (
            <DetailRow
              label={t('Max Response Time')}
              value={formatUseTime(trace.summary.max_use_time)}
              mono
            />
          )}
          {trace.summary.quota > 0 && (
            <DetailRow
              label={t('Total Cost')}
              value={formatLogQuota(trace.summary.quota)}
              mono
            />
          )}
        </div>

        {meta && <ConversionMetaRows meta={meta} />}

        <ChannelFailoverTraceList events={failoverEvents} />

        {errors.length > 0 && (
          <div className='border-border/70 border-t pt-2'>
            <Label className='text-xs font-semibold text-red-600 dark:text-red-400'>
              {t('Errors')}
            </Label>
            <pre className='bg-background/60 mt-1 max-h-28 overflow-y-auto rounded border p-2 font-mono text-[11px] leading-relaxed break-words whitespace-pre-wrap'>
              {errors.join('\n')}
            </pre>
          </div>
        )}

        {relatedLogs.length > 0 && (
          <div className='border-border/70 space-y-1.5 border-t pt-2'>
            <Label className='text-xs font-semibold'>{t('Related Logs')}</Label>
            {relatedLogs.map((log) => {
              const config = getLogTypeConfig(log.type)
              return (
                <div
                  key={log.id}
                  className='bg-background/60 flex min-w-0 flex-col gap-1 rounded border p-2 sm:flex-row sm:items-center sm:justify-between'
                >
                  <div className='flex min-w-0 items-center gap-1.5'>
                    <StatusBadge
                      label={t(config.label)}
                      variant={config.color as StatusBadgeProps['variant']}
                      size='sm'
                      copyable={false}
                    />
                    <span className='min-w-0 truncate font-mono text-[11px]'>
                      {log.model_name || log.content || `#${log.id}`}
                    </span>
                  </div>
                  <span className='text-muted-foreground shrink-0 font-mono text-[11px]'>
                    {formatTokens(
                      (log.prompt_tokens || 0) + (log.completion_tokens || 0)
                    )}
                    {log.use_time > 0
                      ? ` · ${formatUseTime(log.use_time)}`
                      : ''}
                  </span>
                </div>
              )
            })}
            {moreLogsCount > 0 && (
              <p className='text-muted-foreground text-xs'>
                {t('{{count}} more related logs', { count: moreLogsCount })}
              </p>
            )}
          </div>
        )}
      </div>
    </DetailSection>
  )
}

function BillingBreakdown(props: {
  log: UsageLog
  other: LogOtherData
  isAdmin: boolean
}) {
  const { t } = useTranslation()
  const { log, other, isAdmin } = props
  const isPerCall = isPerCallBilling(other.model_price)
  const isClaude = other.claude === true
  const isTieredExpr = other.billing_mode === 'tiered_expr'
  const tieredSummary = getTieredBillingSummary(other)

  const rows: Array<{ label: string; value: string }> = []
  const priceOpts = { digitsLarge: 4, digitsSmall: 6, abbreviate: false }
  const fmtPrice = (usd: number) => formatBillingCurrencyFromUSD(usd, priceOpts)
  const baseInputUSD = other.model_ratio != null ? other.model_ratio * 2.0 : 0

  if (isTieredExpr) {
    rows.push({
      label: t('Billing Mode'),
      value: t('Dynamic Pricing'),
    })
    if (tieredSummary) {
      if (tieredSummary.tier.label) {
        rows.push({
          label: t('Matched Tier'),
          value: tieredSummary.tier.label,
        })
      }
      for (const entry of tieredSummary.priceEntries) {
        rows.push({
          label: t(entry.shortLabel),
          value: `${fmtPrice(entry.price)}/M`,
        })
      }
    } else {
      rows.push({
        label: t('Matched Tier'),
        value: t('No matching results'),
      })
    }
  } else if (isPerCall) {
    rows.push({ label: t('Billing Mode'), value: t('Per-call') })
    if (other.model_price != null) {
      rows.push({
        label: t('Model Price'),
        value: fmtPrice(other.model_price),
      })
    }
  } else {
    rows.push({ label: t('Billing Mode'), value: t('Per-token') })
    if (other.model_ratio != null) {
      rows.push({
        label: t('Input'),
        value: `${fmtPrice(baseInputUSD)}/M`,
      })
    }
    if (other.completion_ratio != null && other.model_ratio != null) {
      rows.push({
        label: t('Output'),
        value: `${fmtPrice(baseInputUSD * other.completion_ratio)}/M`,
      })
    }
  }

  const userGR = other.user_group_ratio
  const isUserGR = userGR != null && Number.isFinite(userGR) && userGR !== -1
  const effectiveGR = isUserGR ? userGR : other.group_ratio
  if (effectiveGR != null && Number.isFinite(effectiveGR)) {
    rows.push({
      label: isUserGR ? t('User Exclusive Ratio') : t('Group Ratio'),
      value: `${formatRatio(effectiveGR)}x`,
    })
  }

  if (!isTieredExpr && isClaude && hasAnyCacheTokens(other)) {
    if (other.cache_ratio != null && other.cache_ratio !== 1) {
      rows.push({
        label: t('Cache Read'),
        value: `${fmtPrice(baseInputUSD * other.cache_ratio)}/M`,
      })
    }
    if (
      other.cache_creation_ratio != null &&
      other.cache_creation_ratio !== 1
    ) {
      rows.push({
        label: t('Cache Creation'),
        value: `${fmtPrice(baseInputUSD * other.cache_creation_ratio)}/M`,
      })
    }
    if (
      other.cache_creation_ratio_5m != null &&
      other.cache_creation_ratio_5m !== 0
    ) {
      rows.push({
        label: t('Cache Creation (5m)'),
        value: `${fmtPrice(baseInputUSD * other.cache_creation_ratio_5m)}/M`,
      })
    }
    if (
      other.cache_creation_ratio_1h != null &&
      other.cache_creation_ratio_1h !== 0
    ) {
      rows.push({
        label: t('Cache Creation (1h)'),
        value: `${fmtPrice(baseInputUSD * other.cache_creation_ratio_1h)}/M`,
      })
    }
  }

  if (!isTieredExpr) {
    if (other.audio_ratio != null && other.audio_ratio !== 1) {
      rows.push({
        label: t('Audio input'),
        value: `${fmtPrice(baseInputUSD * other.audio_ratio)}/M`,
      })
    }

    if (
      other.audio_completion_ratio != null &&
      other.audio_completion_ratio !== 1
    ) {
      rows.push({
        label: t('Audio output'),
        value: `${fmtPrice(baseInputUSD * other.audio_completion_ratio)}/M`,
      })
    }

    if (other.image_ratio != null && other.image_ratio !== 1) {
      rows.push({
        label: t('Image input'),
        value: `${fmtPrice(baseInputUSD * other.image_ratio)}/M`,
      })
    }
  }

  if (other.web_search && other.web_search_call_count) {
    rows.push({
      label: t('Web Search'),
      value: `${other.web_search_call_count}x${other.web_search_price ? ` (${fmtPrice(other.web_search_price)})` : ''}`,
    })
  }

  if (other.file_search && other.file_search_call_count) {
    rows.push({
      label: t('File Search'),
      value: `${other.file_search_call_count}x${other.file_search_price ? ` (${fmtPrice(other.file_search_price)})` : ''}`,
    })
  }

  if (other.image_generation_call && other.image_generation_call_price) {
    rows.push({
      label: t('Image Generation'),
      value: fmtPrice(other.image_generation_call_price),
    })
  }

  if (other.audio_input_seperate_price && other.audio_input_price) {
    rows.push({
      label: t('Audio Input Price'),
      value: fmtPrice(other.audio_input_price),
    })
  }

  if (isAdmin && other.admin_info) {
    rows.push({
      label: t('Billing Source'),
      value: other.admin_info.local_count_tokens
        ? t('Local Billing')
        : t('Upstream Response'),
    })
  }

  rows.push({
    label: t('Total Cost'),
    value: formatLogQuota(log.quota),
  })

  if (rows.length === 0) return null

  return (
    <DetailSection label={t('Billing Details')}>
      {rows.map((row, idx) => (
        <DetailRow key={idx} label={row.label} value={row.value} mono />
      ))}
    </DetailSection>
  )
}

function TokenBreakdown(props: { log: UsageLog; other: LogOtherData }) {
  const { t } = useTranslation()
  const { log, other } = props

  const promptTokens = log.prompt_tokens || 0
  const completionTokens = log.completion_tokens || 0
  const cacheRead = other.cache_tokens || 0
  const cacheWrite = other.cache_creation_tokens || 0
  const cacheWrite5m = other.cache_creation_tokens_5m || 0
  const cacheWrite1h = other.cache_creation_tokens_1h || 0
  const hasTokens = promptTokens > 0 || completionTokens > 0

  if (!hasTokens) return null

  const rows: Array<{ label: string; value: string }> = []

  rows.push({ label: t('Input Tokens'), value: promptTokens.toLocaleString() })
  rows.push({
    label: t('Output Tokens'),
    value: completionTokens.toLocaleString(),
  })

  if (cacheRead > 0) {
    rows.push({
      label: t('Cache Read'),
      value: cacheRead.toLocaleString(),
    })
  }

  if (cacheWrite > 0 && cacheWrite5m === 0 && cacheWrite1h === 0) {
    rows.push({
      label: t('Cache Write'),
      value: cacheWrite.toLocaleString(),
    })
  }

  if (cacheWrite5m > 0) {
    rows.push({
      label: t('Cache Write (5m)'),
      value: cacheWrite5m.toLocaleString(),
    })
  }

  if (cacheWrite1h > 0) {
    rows.push({
      label: t('Cache Write (1h)'),
      value: cacheWrite1h.toLocaleString(),
    })
  }

  if (other.image && other.image_output) {
    rows.push({
      label: t('Image Tokens'),
      value: other.image_output.toLocaleString(),
    })
  }

  return (
    <DetailSection label={t('Token Breakdown')}>
      {rows.map((row, idx) => (
        <DetailRow key={idx} label={row.label} value={row.value} mono />
      ))}
    </DetailSection>
  )
}

interface DetailsDialogProps {
  log: UsageLog
  isAdmin: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function DetailsDialog(props: DetailsDialogProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const details = props.log.content ?? ''
  const other = parseLogOther(props.log.other)
  const typeConfig = getLogTypeConfig(props.log.type)

  const isViolation = isViolationFeeLog(other)
  const isRefund = props.log.type === 6
  const isConsume = props.log.type === 2
  const isTopup = props.log.type === 1
  const isManage = props.log.type === 3
  const isSubscription = other?.billing_source === 'subscription'
  const isTieredBilling =
    isConsume &&
    !isViolation &&
    other?.billing_mode === 'tiered_expr' &&
    !!other?.expr_b64
  const hasAudioTokens = other?.ws || other?.audio
  const showTiming = isTimingLogType(props.log.type)
  const showAdminIp =
    !!props.log.ip && (showTiming || (props.isAdmin && isTopup))
  const adminInfo = other?.admin_info
  const topupAuditFields =
    isTopup && props.isAdmin && adminInfo
      ? ([
          adminInfo.payment_method && {
            label: t('Order Payment Method'),
            value: adminInfo.payment_method,
          },
          adminInfo.callback_payment_method && {
            label: t('Callback Payment Method'),
            value: adminInfo.callback_payment_method,
          },
          adminInfo.caller_ip && {
            label: t('Callback Caller IP'),
            value: adminInfo.caller_ip,
          },
          adminInfo.server_ip && {
            label: t('Server IP'),
            value: adminInfo.server_ip,
          },
          adminInfo.node_name && {
            label: t('Node Name'),
            value: adminInfo.node_name,
          },
          adminInfo.version && {
            label: t('System Version'),
            value: adminInfo.version,
          },
        ].filter(Boolean) as Array<{ label: string; value: string }>)
      : []
  const showLegacyTopupWarning = isTopup && props.isAdmin && !adminInfo
  const showTopupAuditSection =
    isTopup &&
    props.isAdmin &&
    (topupAuditFields.length > 0 || showLegacyTopupWarning)
  const manageOperator = (() => {
    if (!isManage || !props.isAdmin || !adminInfo) return null
    const username = adminInfo.admin_username
    const id = adminInfo.admin_id
    const hasUsername = username != null && String(username).trim() !== ''
    const hasId = id != null && String(id).trim() !== ''
    if (!hasUsername && !hasId) return null
    if (hasUsername && hasId) return `${username} (ID: ${id})`
    if (hasUsername) return String(username)
    return `ID: ${id}`
  })()

  const conversionChain =
    other && Array.isArray(other.request_conversion)
      ? other.request_conversion.filter(Boolean)
      : []
  const conversionMeta = other?.request_conversion_meta
  const conversionLabel =
    conversionChain.length <= 1
      ? t('Native format')
      : conversionChain.join(' -> ')
  const conversionCopyText = [
    conversionLabel,
    conversionMeta ? JSON.stringify(conversionMeta, null, 2) : '',
  ]
    .filter(Boolean)
    .join('\n')
  const showConversion =
    props.isAdmin &&
    props.log.type !== 6 &&
    (other?.request_path ||
      conversionChain.length > 0 ||
      hasConversionMeta(other))

  const useChannel = other?.admin_info?.use_channel
  const channelChain =
    useChannel && useChannel.length > 0 ? useChannel.join(' → ') : undefined
  const requestTraceId = props.log.request_id || ''
  const requestTraceQuery = useQuery({
    queryKey: [
      'usage-log-request-trace',
      props.isAdmin ? 'admin' : 'self',
      requestTraceId,
    ],
    queryFn: () => getRequestLogTrace(requestTraceId, props.isAdmin),
    enabled: props.open && requestTraceId.length > 0,
    staleTime: 30_000,
  })
  const requestTraceResponse = requestTraceQuery.data
  const requestTraceErrorMessage = requestTraceQuery.isError
    ? t('Trace unavailable')
    : requestTraceResponse && !requestTraceResponse.success
      ? requestTraceResponse.message || t('Trace unavailable')
      : undefined
  const requestDiagnosticQuery = useQuery({
    queryKey: ['usage-log-request-diagnostic', requestTraceId],
    queryFn: () => getRequestDiagnosticReport(requestTraceId),
    enabled: props.open && props.isAdmin && requestTraceId.length > 0,
    staleTime: 30_000,
  })
  const generateDiagnosticMutation = useMutation({
    mutationFn: () => generateRequestDiagnosticReport(requestTraceId),
    onSuccess: () => {
      void requestDiagnosticQuery.refetch()
    },
  })
  const requestDiagnosticResponse =
    generateDiagnosticMutation.data || requestDiagnosticQuery.data
  const requestDiagnosticErrorMessage = generateDiagnosticMutation.isError
    ? t('Diagnostic unavailable')
    : requestDiagnosticQuery.isError
      ? t('Diagnostic unavailable')
      : requestDiagnosticResponse && !requestDiagnosticResponse.success
        ? requestDiagnosticResponse.message || t('Diagnostic unavailable')
        : undefined

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent
        className={cn(
          'min-w-0 overflow-hidden',
          'max-sm:max-h-[calc(100dvh-1.5rem)] max-sm:w-[calc(100vw-1.5rem)] max-sm:max-w-[calc(100vw-1.5rem)] max-sm:p-4',
          isTieredBilling ? 'sm:max-w-4xl lg:max-w-5xl' : 'sm:max-w-lg'
        )}
      >
        <DialogHeader className='max-sm:gap-1'>
          <DialogTitle className='flex items-center gap-2 text-base'>
            {t('Log Details')}
            <StatusBadge
              label={t(typeConfig.label)}
              variant={typeConfig.color as StatusBadgeProps['variant']}
              size='sm'
              copyable={false}
            />
          </DialogTitle>
          <DialogDescription className='sr-only'>
            {t('View the complete details for this log entry')}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='max-h-[70vh] min-w-0 overflow-hidden pr-2 max-sm:max-h-[calc(100dvh-7rem)] sm:pr-4'>
          <div className='w-full max-w-full min-w-0 space-y-2.5 overflow-hidden py-1 sm:space-y-3'>
            {/* Overview section - key identifiers */}
            <div className='min-w-0 space-y-1'>
              {props.log.request_id && (
                <DetailRow
                  label={t('Request ID')}
                  value={props.log.request_id}
                  mono
                />
              )}
              {props.log.upstream_request_id && (
                <DetailRow
                  label={t('Upstream Request ID')}
                  value={props.log.upstream_request_id}
                  mono
                />
              )}

              {props.isAdmin && props.log.channel > 0 && (
                <DetailRow
                  label={t('Channel')}
                  value={
                    <span>
                      {props.log.channel}
                      {props.log.channel_name && (
                        <span className='text-muted-foreground'>
                          {' '}
                          ({props.log.channel_name})
                        </span>
                      )}
                    </span>
                  }
                  mono
                />
              )}

              {channelChain && props.isAdmin && (
                <DetailRow label={t('Retry Chain')} value={channelChain} mono />
              )}

              {props.log.token_name && (
                <DetailRow
                  label={t('Token')}
                  value={props.log.token_name}
                  mono
                />
              )}

              {(props.log.group || other?.group) && (
                <DetailRow
                  label={t('Group')}
                  value={props.log.group || other?.group || ''}
                  mono
                />
              )}

              {showAdminIp && (
                <DetailRow
                  label={t('IP Address')}
                  value={
                    <span className='flex items-center gap-1'>
                      <Globe
                        className='size-3 text-amber-500'
                        aria-hidden='true'
                      />
                      {props.log.ip}
                    </span>
                  }
                  mono
                />
              )}

              {showTiming && props.log.use_time > 0 && (
                <DetailRow
                  label={t('Response Time')}
                  value={
                    <span
                      className={cn(
                        'font-medium',
                        timingTextColorClass(
                          getResponseTimeColor(
                            props.log.use_time,
                            props.log.completion_tokens
                          )
                        )
                      )}
                    >
                      {formatUseTime(props.log.use_time)}
                      {props.log.is_stream &&
                        other?.frt != null &&
                        other.frt > 0 && (
                          <span
                            className={cn(
                              'font-normal',
                              timingTextColorClass(
                                getFirstResponseTimeColor(other.frt / 1000)
                              )
                            )}
                          >
                            {' '}
                            (FRT: {formatUseTime(other.frt / 1000)})
                          </span>
                        )}
                    </span>
                  }
                />
              )}
            </div>

            {/* Request conversion (admin only, not for refund) */}
            {showConversion && (
              <DetailSection label={t('Request Conversion')}>
                <div className='relative min-w-0'>
                  <Button
                    variant='ghost'
                    size='sm'
                    className='absolute top-0 right-0 h-5 w-5 p-0'
                    onClick={() => copyToClipboard(conversionCopyText)}
                    title={t('Copy to clipboard')}
                    aria-label={t('Copy to clipboard')}
                  >
                    {copiedText === conversionCopyText ? (
                      <Check className='size-3 text-green-600' />
                    ) : (
                      <Copy className='size-3' />
                    )}
                  </Button>
                  <div className='min-w-0 space-y-1 pr-6'>
                    {other?.request_path && (
                      <DetailRow
                        label={t('Path')}
                        value={other.request_path}
                        mono
                      />
                    )}
                    <div className='flex min-w-0 items-center gap-1.5 text-xs'>
                      <Route
                        className='text-muted-foreground size-3'
                        aria-hidden='true'
                      />
                      <span className='min-w-0 break-all sm:break-words'>
                        {conversionLabel}
                      </span>
                    </div>
                    {conversionMeta && (
                      <ConversionMetaRows meta={conversionMeta} />
                    )}
                  </div>
                </div>
              </DetailSection>
            )}

            {props.log.request_id && (
              <RequestTraceSection
                trace={
                  requestTraceResponse?.success
                    ? requestTraceResponse.data
                    : undefined
                }
                loading={requestTraceQuery.isLoading}
                errorMessage={requestTraceErrorMessage}
                copiedText={copiedText}
                onCopy={copyToClipboard}
              />
            )}

            {props.isAdmin && props.log.request_id && (
              <RequestDiagnosticSection
                report={
                  requestDiagnosticResponse?.success
                    ? requestDiagnosticResponse.data
                    : undefined
                }
                loading={requestDiagnosticQuery.isLoading}
                generating={generateDiagnosticMutation.isPending}
                errorMessage={requestDiagnosticErrorMessage}
                copiedText={copiedText}
                onCopy={copyToClipboard}
                onGenerate={() => generateDiagnosticMutation.mutate()}
                onDownload={() => {
                  window.open(
                    getRequestDiagnosticBundleUrl(requestTraceId),
                    '_blank',
                    'noopener,noreferrer'
                  )
                }}
              />
            )}

            {/* Reject reason (admin only) */}
            {props.isAdmin && other?.reject_reason && (
              <DetailSection
                icon={<AlertTriangle className='size-3.5' aria-hidden='true' />}
                label={t('Reject Reason')}
                variant='danger'
              >
                <p className='text-xs break-words'>{other.reject_reason}</p>
              </DetailSection>
            )}

            {/* Violation fee info */}
            {isViolation && other && (
              <DetailSection
                icon={<AlertTriangle className='size-3.5' aria-hidden='true' />}
                label={t('Violation Fee')}
                variant='danger'
              >
                {other.violation_fee_code && (
                  <DetailRow
                    label={t('Violation Code')}
                    value={other.violation_fee_code}
                    mono
                  />
                )}
                {other.violation_fee_marker && (
                  <DetailRow
                    label={t('Violation Marker')}
                    value={other.violation_fee_marker}
                  />
                )}
                <DetailRow
                  label={t('Fee Amount')}
                  value={formatLogQuota(other.fee_quota ?? props.log.quota)}
                  mono
                />
              </DetailSection>
            )}

            {/* Refund details (type=6) */}
            {isRefund && other && (other.task_id || other.reason) && (
              <DetailSection label={t('Refund Details')}>
                {other.task_id && (
                  <DetailRow label={t('Task ID')} value={other.task_id} mono />
                )}
                {other.reason && (
                  <DetailRow label={t('Reason')} value={other.reason} />
                )}
              </DetailSection>
            )}

            {/* Top-up audit info (type=1, admin only) */}
            {showTopupAuditSection && (
              <DetailSection
                icon={<ShieldCheck className='size-3.5' aria-hidden='true' />}
                label={t('Top-up Audit Info')}
              >
                {topupAuditFields.map((field, idx) => (
                  <DetailRow
                    key={idx}
                    label={field.label}
                    value={field.value}
                    mono
                  />
                ))}
                {showLegacyTopupWarning && (
                  <div className='flex items-start gap-1.5 text-xs text-amber-600 dark:text-amber-400'>
                    <Info
                      className='mt-0.5 size-3.5 shrink-0'
                      aria-hidden='true'
                    />
                    <span>
                      {t(
                        'This record was written by a pre-upgrade instance and lacks audit info. Upgrade the instance to record server IP, callback IP, payment method and system version.'
                      )}
                    </span>
                  </div>
                )}
              </DetailSection>
            )}

            {/* Manage operator (type=3, admin only) */}
            {manageOperator && (
              <DetailRow
                label={
                  <span className='flex items-center gap-1.5'>
                    <UserCog
                      className='text-muted-foreground size-3.5'
                      aria-hidden='true'
                    />
                    {t('Operator Admin')}
                  </span>
                }
                value={manageOperator}
                mono
              />
            )}

            {/* Audio/WebSocket token breakdown */}
            {hasAudioTokens && other && (
              <DetailSection
                icon={<Headphones className='size-3.5' aria-hidden='true' />}
                label={t('Audio Tokens')}
              >
                {other.audio_input != null && other.audio_input > 0 && (
                  <DetailRow
                    label={t('Audio Input')}
                    value={formatTokens(other.audio_input)}
                    mono
                  />
                )}
                {other.audio_output != null && other.audio_output > 0 && (
                  <DetailRow
                    label={t('Audio Output')}
                    value={formatTokens(other.audio_output)}
                    mono
                  />
                )}
                {other.text_input != null && other.text_input > 0 && (
                  <DetailRow
                    label={t('Text Input')}
                    value={formatTokens(other.text_input)}
                    mono
                  />
                )}
                {other.text_output != null && other.text_output > 0 && (
                  <DetailRow
                    label={t('Text Output')}
                    value={formatTokens(other.text_output)}
                    mono
                  />
                )}
              </DetailSection>
            )}

            {/* Reasoning effort */}
            {other?.reasoning_effort && (
              <DetailRow
                label={t('Reasoning Effort')}
                value={
                  <StatusBadge
                    label={other.reasoning_effort}
                    variant={
                      other.reasoning_effort === 'high'
                        ? 'orange'
                        : other.reasoning_effort === 'medium'
                          ? 'yellow'
                          : 'green'
                    }
                    size='sm'
                    copyable={false}
                  />
                }
              />
            )}

            {/* System prompt override */}
            {other?.is_system_prompt_overwritten && (
              <DetailRow
                label={t('System Prompt')}
                value={
                  <StatusBadge
                    label={t('Overwritten')}
                    variant='orange'
                    size='sm'
                    copyable={false}
                  />
                }
              />
            )}

            {/* Model mapping */}
            {other?.is_model_mapped && other?.upstream_model_name && (
              <DetailSection label={t('Model Mapping')}>
                <DetailRow
                  label={t('Request Model')}
                  value={props.log.model_name}
                  mono
                />
                <DetailRow
                  label={t('Actual Model')}
                  value={other.upstream_model_name}
                  mono
                />
              </DetailSection>
            )}

            {/* Token breakdown (for consume/error types with token data) */}
            {isDisplayableType(props.log.type) && other && (
              <TokenBreakdown log={props.log} other={other} />
            )}

            {/* Billing breakdown (consume type) */}
            {isConsume && other && !isViolation && (
              <BillingBreakdown
                log={props.log}
                other={other}
                isAdmin={props.isAdmin}
              />
            )}

            {/* Tiered pricing breakdown (when billing_mode is tiered_expr) */}
            {isTieredBilling && other?.expr_b64 && (
              <div className='bg-muted/30 min-w-0 overflow-hidden rounded-md border px-3 max-sm:px-2'>
                <DynamicPricingBreakdown
                  billingExpr={decodeBillingExprB64(other.expr_b64)}
                  matchedTierLabel={other.matched_tier}
                  hideCacheColumns={!hasAnyCacheTokens(other)}
                />
              </div>
            )}

            {/* Admin billing mode indicator for non-consume */}
            {props.isAdmin &&
              !isConsume &&
              props.log.type !== 6 &&
              other?.admin_info && (
                <DetailRow
                  label={t('Billing Source')}
                  value={
                    <span className='flex items-center gap-1'>
                      {other.admin_info.local_count_tokens ? (
                        <Monitor className='size-3 text-blue-500' />
                      ) : (
                        <Cloud className='size-3 text-emerald-500' />
                      )}
                      <span className='text-xs'>
                        {other.admin_info.local_count_tokens
                          ? t('Local Billing')
                          : t('Upstream Response')}
                      </span>
                    </span>
                  }
                />
              )}

            {/* Stream status details (admin only) */}
            {props.isAdmin &&
              other?.stream_status &&
              other.stream_status.status !== 'ok' && (
                <DetailSection label={t('Stream Status')}>
                  <DetailRow
                    label={t('Status')}
                    value={
                      <StatusBadge
                        label={other.stream_status.status || t('Error')}
                        variant='red'
                        size='sm'
                        copyable={false}
                      />
                    }
                  />
                  {other.stream_status.end_reason && (
                    <DetailRow
                      label={t('End Reason')}
                      value={other.stream_status.end_reason}
                    />
                  )}
                  {(other.stream_status.error_count ?? 0) > 0 && (
                    <DetailRow
                      label={t('Soft Errors')}
                      value={String(other.stream_status.error_count)}
                    />
                  )}
                  {other.stream_status.end_error && (
                    <DetailRow
                      label={t('End Error')}
                      value={other.stream_status.end_error}
                    />
                  )}
                  {Array.isArray(other.stream_status.errors) &&
                    other.stream_status.errors.length > 0 && (
                      <pre className='bg-background/60 mt-1 max-h-32 overflow-y-auto rounded border p-2 font-mono text-[11px] leading-relaxed break-words whitespace-pre-wrap'>
                        {other.stream_status.errors.join('\n')}
                      </pre>
                    )}
                </DetailSection>
              )}

            {/* Subscription billing details */}
            {isSubscription && other && (
              <DetailSection label={t('Subscription Billing')}>
                {other.subscription_plan_id && (
                  <DetailRow
                    label={t('Plan')}
                    value={`#${other.subscription_plan_id} ${other.subscription_plan_title || ''}`.trim()}
                  />
                )}
                {other.subscription_id && (
                  <DetailRow
                    label={t('Instance')}
                    value={`#${other.subscription_id}`}
                    mono
                  />
                )}
                {other.subscription_pre_consumed != null && (
                  <DetailRow
                    label={t('Pre-consumed')}
                    value={formatLogQuota(other.subscription_pre_consumed)}
                    mono
                  />
                )}
                {other.subscription_post_delta != null &&
                  other.subscription_post_delta !== 0 && (
                    <DetailRow
                      label={t('Post Delta')}
                      value={formatLogQuota(other.subscription_post_delta)}
                      mono
                    />
                  )}
                {other.subscription_consumed != null && (
                  <DetailRow
                    label={t('Final Consumed')}
                    value={formatLogQuota(other.subscription_consumed)}
                    mono
                  />
                )}
                {other.subscription_remain != null && (
                  <DetailRow
                    label={t('Remaining')}
                    value={`${formatLogQuota(other.subscription_remain)}${other.subscription_total != null ? ` / ${formatLogQuota(other.subscription_total)}` : ''}`}
                    mono
                  />
                )}
              </DetailSection>
            )}

            {/* Param override */}
            {other?.po && Array.isArray(other.po) && other.po.length > 0 && (
              <DetailSection
                icon={<Settings2 className='size-3.5' aria-hidden='true' />}
                label={`${t('Param Override')} (${other.po.length})`}
              >
                {other.po.filter(Boolean).map((line, idx) => {
                  const parsed = parseAuditLine(line)
                  if (!parsed) return null
                  return (
                    <div
                      key={idx}
                      className='bg-background/60 flex min-w-0 flex-col gap-1.5 rounded border p-2 sm:flex-row sm:items-start sm:gap-2'
                    >
                      <StatusBadge
                        variant='neutral'
                        label={getParamOverrideActionLabel(parsed.action, t)}
                        className='shrink-0 font-medium'
                        copyable={false}
                      />
                      <span className='min-w-0 font-mono text-[11px] leading-relaxed break-all sm:break-words'>
                        {parsed.content}
                      </span>
                    </div>
                  )
                })}
              </DetailSection>
            )}

            {/* Content */}
            {details && (
              <div className='space-y-1.5'>
                <Label className='text-xs font-semibold'>{t('Content')}</Label>
                <div className='bg-muted/30 relative min-w-0 overflow-hidden rounded-md border p-2.5'>
                  <Button
                    variant='ghost'
                    size='sm'
                    className='absolute top-1.5 right-1.5 h-5 w-5 p-0'
                    onClick={() => copyToClipboard(details)}
                    title={t('Copy to clipboard')}
                    aria-label={t('Copy to clipboard')}
                  >
                    {copiedText === details ? (
                      <Check className='size-3 text-green-600' />
                    ) : (
                      <Copy className='size-3' />
                    )}
                  </Button>
                  <p className='min-w-0 pr-6 text-xs leading-relaxed break-all whitespace-pre-wrap sm:break-words'>
                    {details}
                  </p>
                </div>
              </div>
            )}
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}

function isDisplayableType(type: number): boolean {
  return [0, 2, 5, 6].includes(type)
}
