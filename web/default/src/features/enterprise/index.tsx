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
import {
  type FormEvent,
  type ReactNode,
  useEffect,
  useMemo,
  useState,
} from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Route } from '@/routes/_authenticated/enterprise'
import {
  Activity,
  Check,
  Ban,
  Building2,
  ClipboardList,
  Download,
  Eye,
  Gauge,
  Mail,
  Layers3,
  Pencil,
  Plus,
  RefreshCcw,
  Save,
  Search,
  Send,
  ShieldCheck,
  TimerReset,
  Trash2,
  UserPlus,
  Users,
  Webhook,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import {
  createEnterpriseOrgUnit,
  createEnterprisePolicyGroup,
  createEnterprisePolicyGroupShareRequest,
  createEnterpriseProject,
  createEnterpriseQuotaPolicy,
  createEnterpriseWebhook,
  addEnterprisePolicyGroupMembers,
  approveEnterprisePolicyGroupShareRequest,
  approveEnterpriseQuotaRequest,
  batchApproveEnterpriseQuotaRequests,
  batchRejectEnterpriseQuotaRequests,
  cancelEnterpriseQueueAdmission,
  deleteEnterprisePolicyGroupMember,
  deleteEnterpriseProjectMember,
  disableEnterpriseWebhook,
  disableEnterpriseOrgUnit,
  disableEnterprisePolicyGroup,
  disableEnterpriseProject,
  disableEnterpriseQuotaPolicy,
  getEnterpriseAuditLogs,
  getEnterpriseCurrent,
  getEnterpriseMembers,
  getEnterpriseNotificationOutbox,
  getEnterpriseNotificationOutboxWorkerMetrics,
  getEnterpriseNotificationPreferences,
  getEnterpriseOrgUnits,
  getEnterprisePolicyGroupMembers,
  getEnterprisePolicyGroups,
  getEnterprisePolicyGroupShareRequests,
  getEnterpriseProjectMembers,
  getEnterpriseProjects,
  getEnterpriseQuotaPolicies,
  getEnterpriseQuotaRequests,
  getEnterpriseQueueAdmissions,
  getEnterpriseUsageBreakdown,
  getEnterpriseUsageSummary,
  getEnterpriseWebhooks,
  previewEnterpriseOrgSync,
  retryEnterpriseNotificationOutbox,
  testEnterpriseWebhook,
  updateEnterpriseCurrent,
  updateEnterpriseNotificationPreference,
  updateEnterpriseWebhook,
  updateEnterpriseMemberOrgUnit,
  updateEnterpriseOrgUnit,
  updateEnterprisePolicyGroup,
  upsertEnterpriseProjectMember,
  updateEnterpriseProject,
  updateEnterpriseQuotaPolicy,
  rejectEnterpriseQuotaRequest,
  rejectEnterprisePolicyGroupShareRequest,
  submitEnterpriseQuotaRequest,
  withdrawEnterpriseQuotaRequest,
  applyEnterpriseOrgSync,
  downloadEnterpriseUsageBreakdownExport,
} from './api'
import { QuotaRequestDetailSheet } from './components/quota-request-detail-sheet'
import type {
  ApiResponse,
  Enterprise,
  EnterpriseAnomalyThrottleConfig,
  EnterpriseAuditLog,
  EnterpriseMember,
  EnterpriseNotificationOutbox,
  EnterpriseNotificationOutboxWorkerMetrics,
  EnterpriseNotificationPreference,
  EnterpriseNotificationPreferencePayload,
  EnterpriseNotificationRecipientScope,
  EnterpriseOrgUnit,
  EnterpriseOrgUnitPayload,
  EnterpriseOrgSyncPayload,
  EnterpriseOrgSyncResult,
  EnterprisePolicyGroup,
  EnterprisePolicyGroupPayload,
  EnterprisePolicyGroupShareRole,
  EnterprisePolicyGroupShareRequest,
  EnterprisePolicyGroupShareRequestDecisionPayload,
  EnterprisePolicyGroupShareRequestPayload,
  EnterprisePolicyGroupShareRequestStatus,
  EnterpriseProject,
  EnterpriseProjectPayload,
  EnterpriseQuotaPolicy,
  EnterpriseQuotaPolicyPayload,
  EnterpriseQuotaRequest,
  EnterpriseQuotaRequestBatchDecisionResult,
  EnterpriseQuotaRequestDecisionPayload,
  EnterpriseQuotaRequestPayload,
  EnterpriseQuotaRequestStatus,
  EnterpriseQueueAdmission,
  EnterpriseUsageBreakdownItem,
  EnterpriseUsageSummary,
  EnterpriseWebhook,
  EnterpriseWebhookPayload,
  EnterpriseWebhookTestResult,
  PageInfo,
  PolicyAction,
  PolicyConditionMode,
  PolicyMetric,
  PolicyModelScope,
  PolicyPeriod,
  PolicyTargetType,
} from './types'

type EnterpriseTab =
  | 'overview'
  | 'organization'
  | 'projects'
  | 'policy-groups'
  | 'quota-policies'
  | 'quota-requests'
  | 'notifications'
  | 'webhooks'
  | 'deliveries'
  | 'usage'
  | 'audit'

type UsageDimension =
  | 'org_unit'
  | 'project'
  | 'policy_group'
  | 'user'
  | 'model'
  | 'status'
  | 'channel'
  | 'api_key'
  | 'time'

type UsageGranularity = 'day' | 'month'

type QueryResult<T> = {
  data?: ApiResponse<T>
  isLoading: boolean
  isError: boolean
  error: Error | null
  refetch: () => void
}

type SelectOption = {
  value: string
  label: string
}

type QuotaRequestInitialValues = {
  policyId?: number
  limitDelta?: number
  reason?: string
}

type PolicyStructuredCondition = {
  abilities?: string[]
  runtime_groups?: string[]
  model_prefixes?: string[]
  model_names?: string[]
  channel_ids?: number[]
  is_playground?: boolean
}

const PAGE_SIZE = 10
const ENABLED_STATUS = 1
const DISABLED_STATUS = 2
const ALL_VALUE = '__all__'
const ROOT_VALUE = '__root__'
const UNASSIGNED_VALUE = '__unassigned__'
const EMPTY_ORG_UNITS: EnterpriseOrgUnit[] = []
const policyActionOptions: { value: PolicyAction; label: string }[] = [
  { value: 'reject', label: 'Reject' },
  { value: 'alert', label: 'Alert' },
  { value: 'fallback_model', label: 'Fallback Model' },
  { value: 'queue', label: 'Queue' },
  { value: 'shared_pool', label: 'Shared Pool' },
]
const ORG_SYNC_PAYLOAD_TEMPLATE = `{
  "org_units": [
    {
      "external_id": "engineering",
      "name": "Engineering",
      "slug": "engineering",
      "sort": 10
    },
    {
      "external_id": "platform",
      "parent_external_id": "engineering",
      "name": "Platform",
      "slug": "platform",
      "sort": 20
    }
  ],
  "members": [
    {
      "provider_user_id": "hstation-user-id",
      "org_unit_external_id": "engineering",
      "role": "owner"
    }
  ]
}`

const tabs: { value: EnterpriseTab; label: string; icon: typeof Activity }[] = [
  { value: 'overview', label: 'Overview', icon: Activity },
  { value: 'organization', label: 'Organization', icon: Building2 },
  { value: 'projects', label: 'Projects', icon: Layers3 },
  { value: 'policy-groups', label: 'Policy Groups', icon: Users },
  { value: 'quota-policies', label: 'Quota Policies', icon: ShieldCheck },
  { value: 'quota-requests', label: 'Quota Requests', icon: TimerReset },
  { value: 'notifications', label: 'Notifications', icon: Mail },
  { value: 'webhooks', label: 'Webhooks', icon: Webhook },
  { value: 'deliveries', label: 'Deliveries', icon: Send },
  { value: 'usage', label: 'Usage', icon: Gauge },
  { value: 'audit', label: 'Audit', icon: ClipboardList },
]

const WEBHOOK_EVENT_TYPES = [
  'quota_request.submit',
  'quota_request.approve',
  'quota_request.reject',
  'quota_request.withdraw',
  'quota_request.expire',
  'quota_request.expiring_soon',
]

const DEFAULT_ANOMALY_THROTTLE_CONFIG: EnterpriseAnomalyThrottleConfig = {
  enabled: true,
  current_window_seconds: 300,
  baseline_window_seconds: 1800,
  cooldown_seconds: 60,
  min_current_requests: 100,
  min_baseline_requests: 100,
  request_spike_ratio: 8,
  min_current_quota: 1000000,
  min_baseline_quota: 1000000,
  cost_spike_ratio: 8,
  min_failure_requests: 50,
  min_failures: 25,
  failure_rate: 0.5,
}

function todayInputValue() {
  return new Date().toISOString().slice(0, 10)
}

function daysAgoInputValue(days: number) {
  const date = new Date()
  date.setDate(date.getDate() - days)
  return date.toISOString().slice(0, 10)
}

function startOfDayUnix(value: string) {
  const date = value ? new Date(`${value}T00:00:00`) : new Date()
  return Math.floor(date.getTime() / 1000)
}

function endOfDayUnix(value: string) {
  const date = value ? new Date(`${value}T23:59:59`) : new Date()
  return Math.floor(date.getTime() / 1000)
}

function dateInputValueFromUnix(value: number | undefined) {
  if (!value) return ''
  return new Date(value * 1000).toISOString().slice(0, 10)
}

function formatNumber(value: number | undefined) {
  return new Intl.NumberFormat().format(value ?? 0)
}

function formatPercent(value: number) {
  return `${Math.round(value)}%`
}

function formatDateTime(value: number | undefined) {
  if (!value) return '-'
  return new Intl.DateTimeFormat(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value * 1000))
}

function formatDurationMs(value: number | undefined) {
  if (!value) return '0ms'
  return `${formatNumber(value)}ms`
}

function formatRemainingTime(expiresAt: number | undefined) {
  if (!expiresAt) return '-'
  const seconds = Math.max(0, expiresAt - Math.floor(Date.now() / 1000))
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}

function enterpriseAnomalyConfig(
  config: EnterpriseAnomalyThrottleConfig | undefined
) {
  return config ?? DEFAULT_ANOMALY_THROTTLE_CONFIG
}

function positiveNumberInput(value: string, fallback: number) {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

function positiveIntegerInput(value: string, fallback: number) {
  return Math.trunc(positiveNumberInput(value, fallback))
}

function rateNumberInput(value: string, fallback: number) {
  return Math.min(1, positiveNumberInput(value, fallback))
}

function formatAuditJson(value: string | undefined) {
  const raw = value?.trim()
  if (!raw) return '{}'
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

function parseAuditObject(value: string | undefined) {
  const raw = value?.trim()
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw) as unknown
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    return {}
  }
  return {}
}

function auditString(payload: Record<string, unknown>, key: string) {
  const value = payload[key]
  return typeof value === 'string' ? value : ''
}

function auditNumber(payload: Record<string, unknown>, key: string) {
  const value = payload[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

function auditBoolean(payload: Record<string, unknown>, key: string) {
  const value = payload[key]
  return typeof value === 'boolean' ? value : undefined
}

function auditList(payload: Record<string, unknown>, key: string) {
  const value = payload[key]
  return Array.isArray(value) ? value.map(String).join(', ') : ''
}

function auditObjectList(payload: Record<string, unknown>, key: string) {
  const value = payload[key]
  return Array.isArray(value)
    ? value.filter(
        (item): item is Record<string, unknown> =>
          Boolean(item) && typeof item === 'object' && !Array.isArray(item)
      )
    : []
}

function formatAuditNumberValue(payload: Record<string, unknown>, key: string) {
  const value = auditNumber(payload, key)
  return value === undefined ? '-' : formatNumber(value)
}

function getPolicyDryRunHits(
  policy: EnterpriseQuotaPolicy,
  observations: EnterpriseAuditLog[]
) {
  return observations.filter((log) => {
    const payload = parseAuditObject(log.after_json)
    return auditNumber(payload, 'policy_id') === policy.id
  }).length
}

function quotaRequestInitialValuesFromAudit(
  log: EnterpriseAuditLog
): QuotaRequestInitialValues {
  const payload = parseAuditObject(log.after_json)
  const policyId = auditNumber(payload, 'policy_id')
  const suggested = auditNumber(payload, 'suggested_limit_value')
  const limit = auditNumber(payload, 'limit_value')
  const requested = auditNumber(payload, 'requested_value')
  const model = auditString(payload, 'model')
  const limitDelta =
    suggested !== undefined && limit !== undefined
      ? Math.max(0, suggested - limit)
      : requested

  return {
    policyId,
    limitDelta,
    reason: [
      log.request_id ? `request_id=${log.request_id}` : '',
      model ? `model=${model}` : '',
      'quota exceeded audit',
    ]
      .filter(Boolean)
      .join('; '),
  }
}

function getPolicyRiskLabel(policy: EnterpriseQuotaPolicy) {
  if (policy.limit_value <= 0) return 'No limit configured'
  const ratio = policy.used_value / policy.limit_value
  if (ratio >= 1) return 'Limit already reached'
  if (ratio >= 0.8) return 'Near limit'
  if (policy.used_value > 0) return 'Active usage'
  return 'No recent usage'
}

function slugify(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

function normalizeSelectValue(value: string | null) {
  return value ?? ''
}

function normalizeOptionalSelectValue(
  value: string | null,
  emptyValue = ALL_VALUE
) {
  const normalized = normalizeSelectValue(value)
  return normalized === emptyValue ? '' : normalized
}

function parsePositiveSearchId(value: string) {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
}

function parseUserIdList(value: string) {
  return parseNumberList(value)
}

function parseStringList(value: string) {
  return value
    .split(/[\s,，]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseNumberList(value: string) {
  return parseStringList(value)
    .map((item) => Number(item))
    .filter((item) => Number.isInteger(item) && item > 0)
}

function stringifyStringList(values?: string[]) {
  return values?.join(', ') ?? ''
}

function stringifyNumberList(values?: number[]) {
  return values?.join(', ') ?? ''
}

function parseStructuredCondition(raw: string): PolicyStructuredCondition {
  if (!raw.trim()) return {}
  try {
    const parsed = JSON.parse(raw) as PolicyStructuredCondition
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

function buildStructuredConditionJson(form: QuotaPolicyFormState) {
  const condition: PolicyStructuredCondition = {}
  const abilities = parseStringList(form.condition_abilities)
  const runtimeGroups = parseStringList(form.condition_runtime_groups)
  const modelPrefixes = parseStringList(form.condition_model_prefixes)
  const modelNames = parseStringList(form.condition_model_names)
  const channelIds = parseNumberList(form.condition_channel_ids)

  if (abilities.length > 0) condition.abilities = abilities
  if (runtimeGroups.length > 0) condition.runtime_groups = runtimeGroups
  if (modelPrefixes.length > 0) condition.model_prefixes = modelPrefixes
  if (modelNames.length > 0) condition.model_names = modelNames
  if (channelIds.length > 0) condition.channel_ids = channelIds
  if (form.condition_is_playground !== '') {
    condition.is_playground = form.condition_is_playground === 'true'
  }

  return Object.keys(condition).length > 0 ? JSON.stringify(condition) : ''
}

function getErrorMessage(error: Error | null) {
  return error?.message || 'Request failed'
}

function getPageItems<T>(response?: ApiResponse<PageInfo<T>>) {
  return response?.data?.items ?? []
}

function getPageTotal<T>(response?: ApiResponse<PageInfo<T>>) {
  return response?.data?.total ?? 0
}

function getStatusLabel(status: number) {
  return status === DISABLED_STATUS ? 'Disabled' : 'Enabled'
}

function StatusBadge(props: { status: number }) {
  const { t } = useTranslation()
  const disabled = props.status === DISABLED_STATUS
  return (
    <Badge
      variant={disabled ? 'secondary' : 'outline'}
      className={cn(!disabled && 'border-emerald-500/40 text-emerald-700')}
    >
      {t(getStatusLabel(props.status))}
    </Badge>
  )
}

function OutboxStatusBadge(props: { status: string }) {
  const { t } = useTranslation()
  const className = {
    pending: 'border-amber-500/40 text-amber-700',
    processing: 'border-blue-500/40 text-blue-700',
    sent: 'border-emerald-500/40 text-emerald-700',
    failed: 'border-destructive/40 text-destructive',
    permanent_failed: 'border-destructive/40 text-destructive',
  }[props.status]

  return (
    <Badge variant='outline' className={className}>
      {t(formatOutboxStatus(props.status))}
    </Badge>
  )
}

function formatOutboxStatus(status: string) {
  switch (status) {
    case 'processing':
      return 'Processing'
    case 'sent':
      return 'Sent'
    case 'failed':
      return 'Failed'
    case 'permanent_failed':
      return 'Permanent Failed'
    default:
      return 'Pending'
  }
}

function formatChannel(channel: string) {
  if (channel === 'in_app') return 'In-app'
  if (channel === 'email') return 'Email'
  if (channel === 'webhook') return 'Webhook'
  return channel || '-'
}

function formatRecipient(row: EnterpriseNotificationOutbox) {
  if (row.channel === 'webhook') return row.recipient_email || '-'
  if (row.recipient_email) return row.recipient_email
  return row.recipient_user_id ? `user:${row.recipient_user_id}` : '-'
}

function isRetryableOutboxStatus(status: string) {
  return status === 'failed' || status === 'permanent_failed'
}

function QuotaRequestStatusBadge(props: {
  status: EnterpriseQuotaRequestStatus
}) {
  const { t } = useTranslation()
  const className = {
    pending: 'border-amber-500/40 text-amber-700',
    approved: 'border-emerald-500/40 text-emerald-700',
    rejected: 'border-destructive/40 text-destructive',
    withdrawn: 'text-muted-foreground',
    expired: 'text-muted-foreground',
  }[props.status]
  return (
    <Badge variant='outline' className={className}>
      {t(formatQuotaRequestStatus(props.status))}
    </Badge>
  )
}

function formatQuotaRequestStatus(status: EnterpriseQuotaRequestStatus) {
  switch (status) {
    case 'approved':
      return 'Approved'
    case 'rejected':
      return 'Rejected'
    case 'withdrawn':
      return 'Withdrawn'
    case 'expired':
      return 'Expired'
    default:
      return 'Pending'
  }
}

function PolicyGroupShareRequestStatusBadge(props: {
  status: EnterprisePolicyGroupShareRequestStatus
}) {
  const { t } = useTranslation()
  const className = {
    pending: 'border-amber-500/40 text-amber-700',
    approved: 'border-emerald-500/40 text-emerald-700',
    rejected: 'border-destructive/40 text-destructive',
    withdrawn: 'text-muted-foreground',
  }[props.status]
  return (
    <Badge variant='outline' className={className}>
      {t(formatPolicyGroupShareRequestStatus(props.status))}
    </Badge>
  )
}

function formatPolicyGroupShareRequestStatus(
  status: EnterprisePolicyGroupShareRequestStatus
) {
  switch (status) {
    case 'approved':
      return 'Approved'
    case 'rejected':
      return 'Rejected'
    case 'withdrawn':
      return 'Withdrawn'
    default:
      return 'Pending'
  }
}

function normalizePolicyGroupShareRole(
  role?: EnterprisePolicyGroupShareRole | string | null
): EnterprisePolicyGroupShareRole {
  return role === 'viewer' ? 'viewer' : 'editor'
}

function formatPolicyGroupShareRole(
  role?: EnterprisePolicyGroupShareRole | string | null
) {
  return normalizePolicyGroupShareRole(role) === 'viewer' ? 'Viewer' : 'Editor'
}

function PolicyGroupShareRoleBadge(props: {
  role?: EnterprisePolicyGroupShareRole | string
}) {
  const { t } = useTranslation()
  const role = normalizePolicyGroupShareRole(props.role)
  return (
    <Badge
      variant='outline'
      className={
        role === 'editor'
          ? 'border-emerald-500/40 text-emerald-700'
          : 'text-muted-foreground'
      }
    >
      {t(formatPolicyGroupShareRole(role))}
    </Badge>
  )
}

function QueryState<T>(props: {
  query: QueryResult<T>
  empty?: boolean
  emptyTitle?: string
  emptyDescription?: string
  emptyContent?: ReactNode
  children: ReactNode
}) {
  const { t } = useTranslation()

  if (props.query.isLoading) {
    return <TableSkeleton />
  }

  if (props.query.isError) {
    return (
      <ErrorState
        className='min-h-[220px]'
        title={t('Failed to load data')}
        description={t(getErrorMessage(props.query.error))}
        onRetry={props.query.refetch}
      />
    )
  }

  if (props.empty) {
    if (props.emptyContent) return <>{props.emptyContent}</>

    return (
      <EmptyState
        className='min-h-[220px]'
        title={props.emptyTitle ? t(props.emptyTitle) : undefined}
        description={
          props.emptyDescription ? t(props.emptyDescription) : undefined
        }
      />
    )
  }

  return <>{props.children}</>
}

function TableSkeleton() {
  return (
    <div className='space-y-2 rounded-lg border p-3'>
      {Array.from({ length: 6 }).map((_, index) => (
        <Skeleton key={index} className='h-8 w-full' />
      ))}
    </div>
  )
}

function Panel(props: {
  title?: string
  description?: string
  actions?: ReactNode
  children: ReactNode
  className?: string
}) {
  const { t } = useTranslation()
  return (
    <section className={cn('bg-background rounded-lg border', props.className)}>
      {(props.title || props.description || props.actions) && (
        <div className='flex flex-wrap items-start justify-between gap-3 border-b px-3 py-2.5'>
          <div className='min-w-0'>
            {props.title && (
              <h3 className='text-sm font-semibold'>{t(props.title)}</h3>
            )}
            {props.description && (
              <p className='text-muted-foreground mt-1 text-xs'>
                {t(props.description)}
              </p>
            )}
          </div>
          {props.actions && (
            <div className='flex flex-wrap items-center gap-2'>
              {props.actions}
            </div>
          )}
        </div>
      )}
      <div className='p-3'>{props.children}</div>
    </section>
  )
}

function FilterBar(props: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        'bg-muted/20 flex flex-wrap items-center gap-2 rounded-lg border p-2',
        props.className
      )}
    >
      {props.children}
    </div>
  )
}

function SearchInput(props: {
  value: string
  onChange: (value: string) => void
  placeholder: string
}) {
  const { t } = useTranslation()
  return (
    <div className='relative min-w-52 flex-1 sm:max-w-72'>
      <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2' />
      <Input
        value={props.value}
        onChange={(event) => props.onChange(event.target.value)}
        placeholder={t(props.placeholder)}
        className='pl-7'
      />
    </div>
  )
}

function StatusFilter(props: {
  value: string
  onChange: (value: string) => void
}) {
  const { t } = useTranslation()
  return (
    <Select
      value={props.value || ALL_VALUE}
      onValueChange={(value) =>
        props.onChange(normalizeOptionalSelectValue(value))
      }
    >
      <SelectTrigger className='w-36'>
        <SelectValue placeholder={t('Status')} />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          <SelectItem value={ALL_VALUE}>{t('All Statuses')}</SelectItem>
          <SelectItem value='1'>{t('Enabled')}</SelectItem>
          <SelectItem value='2'>{t('Disabled')}</SelectItem>
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

function Pager(props: {
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation()
  const totalPages = Math.max(1, Math.ceil(props.total / props.pageSize))
  return (
    <div className='flex flex-wrap items-center justify-between gap-2 border-t px-3 py-2 text-xs'>
      <span className='text-muted-foreground'>
        {t('Total')} {formatNumber(props.total)}
      </span>
      <div className='flex items-center gap-2'>
        <Button
          variant='outline'
          size='xs'
          disabled={props.page <= 1}
          onClick={() => props.onPageChange(props.page - 1)}
        >
          {t('Previous')}
        </Button>
        <span className='text-muted-foreground tabular-nums'>
          {props.page} / {totalPages}
        </span>
        <Button
          variant='outline'
          size='xs'
          disabled={props.page >= totalPages}
          onClick={() => props.onPageChange(props.page + 1)}
        >
          {t('Next')}
        </Button>
      </div>
    </div>
  )
}

function StatCell(props: {
  icon: typeof Activity
  label: string
  value: ReactNode
  detail?: string
}) {
  const { t } = useTranslation()
  const Icon = props.icon
  return (
    <div className='bg-muted/10 rounded-lg border p-3'>
      <div className='flex items-center gap-2'>
        <Icon className='text-muted-foreground size-4' />
        <span className='text-muted-foreground text-xs'>{t(props.label)}</span>
      </div>
      <div className='mt-2 text-base font-semibold'>{props.value}</div>
      {props.detail && (
        <div className='text-muted-foreground mt-1 text-xs'>
          {t(props.detail)}
        </div>
      )}
    </div>
  )
}

function OrgUnitName(props: { unit: EnterpriseOrgUnit }) {
  return (
    <div
      className='flex items-center gap-2'
      style={{ paddingLeft: props.unit.depth * 14 }}
    >
      <span className='text-muted-foreground text-xs tabular-nums'>
        {props.unit.depth > 0 ? '└' : ''}
      </span>
      <div className='min-w-0'>
        <div className='truncate font-medium'>{props.unit.name}</div>
        <div className='text-muted-foreground truncate text-xs'>
          {props.unit.slug}
        </div>
      </div>
    </div>
  )
}

function parseModelsJson(policy: EnterpriseQuotaPolicy) {
  if (!policy.models_json) return []
  try {
    const parsed = JSON.parse(policy.models_json) as unknown
    return Array.isArray(parsed)
      ? parsed.filter((item): item is string => typeof item === 'string')
      : []
  } catch {
    return []
  }
}

function formatPolicyTarget(policy: EnterpriseQuotaPolicy) {
  if (policy.target_type === 'enterprise') return 'Enterprise'
  return policy.target_name || `${policy.target_type} #${policy.target_id}`
}

function formatMetric(metric: PolicyMetric) {
  return metric === 'request_count' ? 'Requests' : 'Quota'
}

function formatPeriod(period: PolicyPeriod) {
  return period === 'month' ? 'Monthly' : 'Daily'
}

function formatPolicyAction(action: PolicyAction | string) {
  return (
    policyActionOptions.find((option) => option.value === action)?.label ||
    action ||
    'Reject'
  )
}

function UsageBar(props: { used: number; limit: number }) {
  const percent =
    props.limit > 0 ? Math.min(100, (props.used / props.limit) * 100) : 0

  return (
    <div className='min-w-36 space-y-1'>
      <div className='flex items-center justify-between gap-2 text-xs'>
        <span className='tabular-nums'>{formatNumber(props.used)}</span>
        <span className='text-muted-foreground tabular-nums'>
          {formatPercent(percent)}
        </span>
      </div>
      <Progress value={percent} />
    </div>
  )
}

function buildOrgOptions(orgUnits: EnterpriseOrgUnit[]) {
  return orgUnits
    .filter((unit) => unit.status === ENABLED_STATUS)
    .map((unit) => ({
      value: String(unit.id),
      label: `${'  '.repeat(unit.depth)}${unit.name}`,
    }))
}

function EnterpriseHeader(props: {
  enterprise?: Enterprise
  isLoading: boolean
  membersTotal: number
  orgUnitsTotal: number
  policyGroupsTotal: number
  quotaPoliciesTotal: number
  canManage: boolean
  onEdit: () => void
}) {
  const { t } = useTranslation()

  if (props.isLoading) {
    return (
      <div className='grid gap-2 md:grid-cols-5'>
        {Array.from({ length: 5 }).map((_, index) => (
          <Skeleton key={index} className='h-24 w-full' />
        ))}
      </div>
    )
  }

  return (
    <div className='grid gap-2 md:grid-cols-5'>
      <div className='bg-background rounded-lg border p-3 md:col-span-2'>
        <div className='flex items-start justify-between gap-3'>
          <div className='min-w-0'>
            <div className='flex items-center gap-2'>
              <Building2 className='text-muted-foreground size-4' />
              <span className='text-muted-foreground text-xs'>
                {t('Current Enterprise')}
              </span>
            </div>
            <div className='mt-2 truncate text-base font-semibold'>
              {props.enterprise?.name ?? '-'}
            </div>
            <div className='text-muted-foreground mt-1 truncate text-xs'>
              {props.enterprise?.slug ?? '-'} ·{' '}
              {props.enterprise?.timezone ?? '-'}
            </div>
          </div>
          <div className='flex shrink-0 items-center gap-2'>
            {props.enterprise && (
              <StatusBadge status={props.enterprise.status} />
            )}
            {props.canManage && (
              <Button variant='outline' size='icon-sm' onClick={props.onEdit}>
                <Pencil className='size-3.5' />
                <span className='sr-only'>{t('Edit')}</span>
              </Button>
            )}
          </div>
        </div>
      </div>
      <StatCell
        icon={Users}
        label='Members'
        value={formatNumber(props.membersTotal)}
      />
      <StatCell
        icon={Layers3}
        label='Org Units'
        value={formatNumber(props.orgUnitsTotal)}
      />
      <StatCell
        icon={ShieldCheck}
        label='Policy Groups'
        value={formatNumber(props.policyGroupsTotal)}
      />
      <StatCell
        icon={Gauge}
        label='Quota Policies'
        value={formatNumber(props.quotaPoliciesTotal)}
      />
    </div>
  )
}

function OverviewTab(props: {
  enterprise?: Enterprise
  usage?: EnterpriseUsageSummary
  orgUnits: EnterpriseOrgUnit[]
  membersTotal: number
  policyGroupsTotal: number
  quotaPoliciesTotal: number
  usageQuery: QueryResult<EnterpriseUsageSummary>
}) {
  const { t } = useTranslation()
  const enabledOrgUnits = props.orgUnits.filter(
    (unit) => unit.status === ENABLED_STATUS
  ).length
  const totalUsage = props.usage?.total

  return (
    <div className='grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(320px,420px)]'>
      <Panel title='Governance Snapshot'>
        <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-3'>
          <StatCell
            icon={Activity}
            label='Enterprise Status'
            value={
              props.enterprise ? (
                <StatusBadge status={props.enterprise.status} />
              ) : (
                '-'
              )
            }
            detail={props.enterprise?.timezone}
          />
          <StatCell
            icon={Building2}
            label='Enabled Org Units'
            value={`${formatNumber(enabledOrgUnits)} / ${formatNumber(
              props.orgUnits.length
            )}`}
          />
          <StatCell
            icon={Users}
            label='Managed Members'
            value={formatNumber(props.membersTotal)}
          />
          <StatCell
            icon={ShieldCheck}
            label='Policy Groups'
            value={formatNumber(props.policyGroupsTotal)}
          />
          <StatCell
            icon={Gauge}
            label='Quota Policies'
            value={formatNumber(props.quotaPoliciesTotal)}
          />
          <StatCell
            icon={ClipboardList}
            label='Window Requests'
            value={formatNumber(totalUsage?.request_count)}
            detail='Current usage filter'
          />
        </div>
      </Panel>

      <Panel title='Usage Mix' description='Last selected usage window'>
        <QueryState
          query={props.usageQuery}
          empty={!props.usage}
          emptyTitle='No usage yet'
          emptyDescription='Relay attribution will appear after governed traffic is processed.'
        >
          <div className='space-y-3'>
            <div className='grid grid-cols-2 gap-2'>
              <StatCell
                icon={Gauge}
                label='Quota'
                value={formatNumber(totalUsage?.quota)}
              />
              <StatCell
                icon={Activity}
                label='Tokens'
                value={formatNumber(totalUsage?.total_tokens)}
              />
            </div>
            <div className='space-y-2'>
              <h4 className='text-xs font-medium'>{t('By Model')}</h4>
              {(props.usage?.by_model ?? []).slice(0, 5).map((item) => (
                <div
                  key={item.model_name || item.target_name}
                  className='flex items-center justify-between gap-3 text-xs'
                >
                  <span className='truncate'>
                    {item.model_name || item.target_name || '-'}
                  </span>
                  <span className='text-muted-foreground tabular-nums'>
                    {formatNumber(item.quota)}
                  </span>
                </div>
              ))}
            </div>
            <div className='space-y-2'>
              <h4 className='text-xs font-medium'>{t('By Status')}</h4>
              {(props.usage?.by_status ?? []).slice(0, 5).map((item) => (
                <div
                  key={item.status || item.target_name}
                  className='flex items-center justify-between gap-3 text-xs'
                >
                  <span className='truncate'>
                    {item.status || item.target_name || '-'}
                  </span>
                  <span className='text-muted-foreground tabular-nums'>
                    {formatNumber(item.request_count)}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </QueryState>
      </Panel>
    </div>
  )
}

function SsoOrgSyncPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [provider, setProvider] = useState('hstation')
  const [payloadText, setPayloadText] = useState(ORG_SYNC_PAYLOAD_TEMPLATE)
  const [allowConflicts, setAllowConflicts] = useState(false)
  const [parseError, setParseError] = useState('')
  const [result, setResult] = useState<EnterpriseOrgSyncResult | null>(null)

  const buildPayload = (): EnterpriseOrgSyncPayload | null => {
    try {
      const parsed = JSON.parse(
        payloadText
      ) as Partial<EnterpriseOrgSyncPayload>
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        throw new Error(t('Invalid JSON payload'))
      }
      const orgUnits = Array.isArray(parsed.org_units) ? parsed.org_units : []
      const members = Array.isArray(parsed.members) ? parsed.members : []
      setParseError('')
      return {
        provider,
        snapshot_at: parsed.snapshot_at,
        org_units: orgUnits,
        members,
        allow_conflicts: allowConflicts,
      }
    } catch (error) {
      const message =
        error instanceof Error ? error.message : t('Invalid JSON payload')
      setParseError(message)
      toast.error(message)
      return null
    }
  }

  const previewMutation = useMutation({
    mutationFn: previewEnterpriseOrgSync,
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message ?? t('Preview failed'))
        return
      }
      setResult(response.data)
      toast.success(t('Preview ready'))
    },
  })

  const applyMutation = useMutation({
    mutationFn: applyEnterpriseOrgSync,
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message ?? t('Sync failed'))
        return
      }
      setResult(response.data)
      toast.success(t('Synced'))
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  const isPending = previewMutation.isPending || applyMutation.isPending
  const summary = result?.summary
  const operations = result?.operations ?? []
  const conflicts = result?.conflicts ?? []

  return (
    <Panel
      className='xl:col-span-2'
      title='SSO Sync'
      description='Preview and apply organization snapshots from an SSO source.'
      actions={
        <>
          <Button
            variant='outline'
            size='sm'
            disabled={isPending}
            onClick={() => {
              const payload = buildPayload()
              if (payload) previewMutation.mutate(payload)
            }}
          >
            <Search className='size-3.5' />
            {t('Preview')}
          </Button>
          <Button
            size='sm'
            disabled={isPending || (conflicts.length > 0 && !allowConflicts)}
            onClick={() => {
              const payload = buildPayload()
              if (payload) applyMutation.mutate(payload)
            }}
          >
            <Check className='size-3.5' />
            {t('Apply')}
          </Button>
        </>
      }
    >
      <div className='grid gap-3 lg:grid-cols-[minmax(0,0.9fr)_minmax(280px,0.6fr)]'>
        <div className='space-y-3'>
          <div className='grid gap-2 sm:grid-cols-[180px_minmax(0,1fr)]'>
            <Field label='Provider'>
              <Select
                value={provider}
                onValueChange={(value) => {
                  if (value) setProvider(value)
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('Provider')} />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='hstation'>HStation</SelectItem>
                    <SelectItem value='oidc'>OIDC</SelectItem>
                    <SelectItem value='github'>GitHub</SelectItem>
                    <SelectItem value='manual'>Manual</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field label='Snapshot JSON'>
              <Textarea
                value={payloadText}
                onChange={(event) => setPayloadText(event.target.value)}
                spellCheck={false}
                className='min-h-56 resize-y font-mono text-xs leading-5'
              />
            </Field>
          </div>
          <label className='flex items-center gap-2 text-sm'>
            <Checkbox
              checked={allowConflicts}
              onCheckedChange={(checked) => setAllowConflicts(checked === true)}
            />
            <span>{t('Apply non-conflicting rows')}</span>
          </label>
          {parseError && (
            <div className='text-destructive text-xs'>{parseError}</div>
          )}
        </div>

        <div className='space-y-3'>
          {summary ? (
            <>
              <div className='grid grid-cols-2 gap-2'>
                <StatCell
                  icon={Building2}
                  label='Create Org Units'
                  value={formatNumber(summary.create_org_units)}
                />
                <StatCell
                  icon={Pencil}
                  label='Update Org Units'
                  value={formatNumber(summary.update_org_units)}
                />
                <StatCell
                  icon={Users}
                  label='Assign Members'
                  value={formatNumber(summary.assign_members)}
                />
                <StatCell
                  icon={Ban}
                  label='Conflicts'
                  value={formatNumber(summary.conflicts)}
                />
              </div>
              <div className='text-muted-foreground text-xs'>
                {result?.dry_run
                  ? t('Dry-run result')
                  : `${t('Applied')} ${formatDateTime(result?.applied_at)}`}
              </div>
            </>
          ) : (
            <div className='text-muted-foreground rounded-lg border border-dashed p-3 text-sm'>
              {t('No preview result')}
            </div>
          )}

          {conflicts.length > 0 && (
            <div className='rounded-lg border'>
              <div className='border-b px-3 py-2 text-xs font-medium'>
                {t('Conflicts')}
              </div>
              <div className='max-h-44 overflow-auto'>
                {conflicts.slice(0, 8).map((conflict, index) => (
                  <div
                    key={`${conflict.type}:${conflict.external_id ?? conflict.user_id ?? index}:${index}`}
                    className='border-b px-3 py-2 text-xs last:border-b-0'
                  >
                    <div className='font-medium'>
                      {conflict.field || conflict.type}
                    </div>
                    <div className='text-muted-foreground mt-0.5'>
                      {conflict.message}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {operations.length > 0 && (
            <div className='rounded-lg border'>
              <div className='border-b px-3 py-2 text-xs font-medium'>
                {t('Operations')}
              </div>
              <div className='max-h-44 overflow-auto'>
                {operations.slice(0, 8).map((operation, index) => (
                  <div
                    key={`${operation.type}:${operation.action}:${operation.slug ?? operation.user_id ?? index}`}
                    className='flex items-center justify-between gap-3 border-b px-3 py-2 text-xs last:border-b-0'
                  >
                    <span className='truncate'>
                      {operation.target_name ||
                        operation.slug ||
                        operation.external_id ||
                        `#${operation.user_id}`}
                    </span>
                    <Badge variant='outline'>
                      {operation.type}.{operation.action}
                    </Badge>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </Panel>
  )
}

function OrganizationTab(props: {
  orgUnits: EnterpriseOrgUnit[]
  orgUnitsQuery: QueryResult<EnterpriseOrgUnit[]>
  members: EnterpriseMember[]
  membersQuery: QueryResult<PageInfo<EnterpriseMember>>
  membersTotal: number
  memberPage: number
  setMemberPage: (page: number) => void
  orgKeyword: string
  setOrgKeyword: (value: string) => void
  orgStatus: string
  setOrgStatus: (value: string) => void
  memberKeyword: string
  setMemberKeyword: (value: string) => void
  memberOrgFilter: string
  setMemberOrgFilter: (value: string) => void
  onCreateOrgUnit: () => void
  onEditOrgUnit: (unit: EnterpriseOrgUnit) => void
  onDisableOrgUnit: (unit: EnterpriseOrgUnit) => void
  orgOptions: SelectOption[]
  canManageEnterprise: boolean
}) {
  const { t } = useTranslation()
  const filteredOrgUnits = props.orgUnits.filter((unit) => {
    const keyword = props.orgKeyword.trim().toLowerCase()
    const keywordMatched =
      !keyword ||
      unit.name.toLowerCase().includes(keyword) ||
      unit.slug.toLowerCase().includes(keyword)
    const statusMatched =
      !props.orgStatus || String(unit.status) === props.orgStatus
    return keywordMatched && statusMatched
  })

  return (
    <div className='grid gap-3 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]'>
      {props.canManageEnterprise && <SsoOrgSyncPanel />}

      <Panel
        title='Organization'
        description='Department tree and ownership boundaries.'
        actions={
          props.canManageEnterprise ? (
            <Button size='sm' onClick={props.onCreateOrgUnit}>
              <Plus className='size-3.5' />
              {t('Org Unit')}
            </Button>
          ) : undefined
        }
      >
        <div className='space-y-3'>
          <FilterBar>
            <SearchInput
              value={props.orgKeyword}
              onChange={props.setOrgKeyword}
              placeholder='Search org units'
            />
            <StatusFilter
              value={props.orgStatus}
              onChange={props.setOrgStatus}
            />
          </FilterBar>
          <QueryState
            query={props.orgUnitsQuery}
            empty={filteredOrgUnits.length === 0}
            emptyTitle='No org units'
            emptyDescription='Create departments to attach users and quota policies.'
          >
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Name')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Sort')}</TableHead>
                  {props.canManageEnterprise && (
                    <TableHead className='text-right'>{t('Actions')}</TableHead>
                  )}
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredOrgUnits.map((unit) => (
                  <TableRow key={unit.id}>
                    <TableCell>
                      <OrgUnitName unit={unit} />
                    </TableCell>
                    <TableCell>
                      <StatusBadge status={unit.status} />
                    </TableCell>
                    <TableCell>{unit.sort}</TableCell>
                    {props.canManageEnterprise && (
                      <TableCell>
                        <div className='flex justify-end gap-1'>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            onClick={() => props.onEditOrgUnit(unit)}
                          >
                            <Pencil className='size-3.5' />
                            <span className='sr-only'>{t('Edit')}</span>
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            disabled={unit.status === DISABLED_STATUS}
                            onClick={() => props.onDisableOrgUnit(unit)}
                          >
                            <Ban className='size-3.5' />
                            <span className='sr-only'>{t('Disable')}</span>
                          </Button>
                        </div>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </QueryState>
        </div>
      </Panel>

      <Panel
        title='Members'
        description='Assign users to their primary org unit.'
      >
        <div className='space-y-3'>
          <FilterBar>
            <SearchInput
              value={props.memberKeyword}
              onChange={(value) => {
                props.setMemberKeyword(value)
                props.setMemberPage(1)
              }}
              placeholder='Search members'
            />
            <Select
              value={props.memberOrgFilter || ALL_VALUE}
              onValueChange={(value) => {
                props.setMemberOrgFilter(normalizeOptionalSelectValue(value))
                props.setMemberPage(1)
              }}
            >
              <SelectTrigger className='w-44'>
                <SelectValue placeholder={t('Org Unit')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL_VALUE}>
                    {t('All Org Units')}
                  </SelectItem>
                  {props.canManageEnterprise && (
                    <SelectItem value={UNASSIGNED_VALUE}>
                      {t('Unassigned')}
                    </SelectItem>
                  )}
                  {props.orgOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </FilterBar>
          <QueryState
            query={props.membersQuery}
            empty={props.members.length === 0}
            emptyTitle='No members'
            emptyDescription='Users will appear here after account creation.'
          >
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('User')}</TableHead>
                  <TableHead>{t('Org Unit')}</TableHead>
                  <TableHead>{t('Policy Groups')}</TableHead>
                  <TableHead className='text-right'>{t('Assign')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {props.members.map((member) => (
                  <MemberRow
                    key={`${member.user_id}:${member.org_unit_id || 0}`}
                    member={member}
                    orgOptions={props.orgOptions}
                    allowUnassigned={props.canManageEnterprise}
                  />
                ))}
              </TableBody>
            </Table>
            <Pager
              page={props.memberPage}
              pageSize={PAGE_SIZE}
              total={props.membersTotal}
              onPageChange={props.setMemberPage}
            />
          </QueryState>
        </div>
      </Panel>
    </div>
  )
}

function MemberRow(props: {
  member: EnterpriseMember
  orgOptions: SelectOption[]
  allowUnassigned: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [selectedOrgUnit, setSelectedOrgUnit] = useState(
    String(props.member.org_unit_id || '')
  )

  const mutation = useMutation({
    mutationFn: () =>
      updateEnterpriseMemberOrgUnit(
        props.member.user_id,
        selectedOrgUnit ? Number(selectedOrgUnit) : 0
      ),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Saved'))
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  const changed = selectedOrgUnit !== String(props.member.org_unit_id || '')

  return (
    <TableRow>
      <TableCell>
        <div className='min-w-0'>
          <div className='truncate font-medium'>
            {props.member.display_name || props.member.username}
          </div>
          <div className='text-muted-foreground truncate text-xs'>
            #{props.member.user_id} ·{' '}
            {props.member.email || props.member.username}
          </div>
        </div>
      </TableCell>
      <TableCell>
        <Select
          value={selectedOrgUnit || UNASSIGNED_VALUE}
          onValueChange={(value) =>
            setSelectedOrgUnit(
              normalizeOptionalSelectValue(value, UNASSIGNED_VALUE)
            )
          }
        >
          <SelectTrigger className='w-44'>
            <SelectValue placeholder={t('Unassigned')} />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {props.allowUnassigned && (
                <SelectItem value={UNASSIGNED_VALUE}>
                  {t('Unassigned')}
                </SelectItem>
              )}
              {props.orgOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </TableCell>
      <TableCell>{formatNumber(props.member.policy_group_count)}</TableCell>
      <TableCell>
        <div className='flex justify-end'>
          <Button
            variant='outline'
            size='xs'
            disabled={!changed || mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            <Save className='size-3' />
            {t('Save')}
          </Button>
        </div>
      </TableCell>
    </TableRow>
  )
}

function PolicyGroupsTab(props: {
  groups: EnterprisePolicyGroup[]
  query: QueryResult<PageInfo<EnterprisePolicyGroup>>
  shareRequests: EnterprisePolicyGroupShareRequest[]
  shareRequestsQuery: QueryResult<PageInfo<EnterprisePolicyGroupShareRequest>>
  page: number
  total: number
  setPage: (page: number) => void
  shareRequestPage: number
  shareRequestTotal: number
  setShareRequestPage: (page: number) => void
  shareRequestStatus: string
  setShareRequestStatus: (value: string) => void
  keyword: string
  setKeyword: (value: string) => void
  status: string
  setStatus: (value: string) => void
  onCreate: () => void
  onEdit: (group: EnterprisePolicyGroup) => void
  onDisable: (group: EnterprisePolicyGroup) => void
  onManageMembers: (group: EnterprisePolicyGroup) => void
  onCreateShareRequest: (group: EnterprisePolicyGroup) => void
  onDecideShareRequest: (
    request: EnterprisePolicyGroupShareRequest,
    action: 'approve' | 'reject'
  ) => void
}) {
  const { t } = useTranslation()

  return (
    <Panel
      title='Policy Groups'
      description='Operational cohorts that can receive their own quota policies.'
      actions={
        <Button size='sm' onClick={props.onCreate}>
          <Plus className='size-3.5' />
          {t('Policy Group')}
        </Button>
      }
    >
      <div className='space-y-3'>
        <FilterBar>
          <SearchInput
            value={props.keyword}
            onChange={(value) => {
              props.setKeyword(value)
              props.setPage(1)
            }}
            placeholder='Search policy groups'
          />
          <StatusFilter
            value={props.status}
            onChange={(value) => {
              props.setStatus(value)
              props.setPage(1)
            }}
          />
        </FilterBar>
        <QueryState
          query={props.query}
          empty={props.groups.length === 0}
          emptyTitle='No policy groups'
          emptyDescription='Create groups for project, duty, or workload-based allocation.'
        >
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Name')}</TableHead>
                <TableHead>{t('Shared')}</TableHead>
                <TableHead>{t('Members')}</TableHead>
                <TableHead>{t('Policies')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Updated')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {props.groups.map((group) => (
                <TableRow key={group.id}>
                  <TableCell>
                    <div className='min-w-0'>
                      <div className='truncate font-medium'>{group.name}</div>
                      <div className='text-muted-foreground truncate text-xs'>
                        {group.slug}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className='space-y-1'>
                      <PolicyGroupSharedOrgUnitList group={group} />
                      {group.shared_expires_at > 0 ? (
                        <div className='text-muted-foreground text-xs'>
                          {t('Until')} {formatDateTime(group.shared_expires_at)}
                        </div>
                      ) : null}
                    </div>
                  </TableCell>
                  <TableCell>{formatNumber(group.member_count)}</TableCell>
                  <TableCell>{formatNumber(group.policy_count)}</TableCell>
                  <TableCell>
                    <StatusBadge status={group.status} />
                  </TableCell>
                  <TableCell>{formatDateTime(group.updated_at)}</TableCell>
                  <TableCell>
                    <div className='flex justify-end gap-1'>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        onClick={() => props.onManageMembers(group)}
                      >
                        <Users className='size-3.5' />
                        <span className='sr-only'>{t('Members')}</span>
                      </Button>
                      {group.can_manage ? (
                        <>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            onClick={() => props.onCreateShareRequest(group)}
                          >
                            <Send className='size-3.5' />
                            <span className='sr-only'>
                              {t('Share Request')}
                            </span>
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            onClick={() => props.onEdit(group)}
                          >
                            <Pencil className='size-3.5' />
                            <span className='sr-only'>{t('Edit')}</span>
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            disabled={group.status === DISABLED_STATUS}
                            onClick={() => props.onDisable(group)}
                          >
                            <Ban className='size-3.5' />
                            <span className='sr-only'>{t('Disable')}</span>
                          </Button>
                        </>
                      ) : null}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <Pager
            page={props.page}
            pageSize={PAGE_SIZE}
            total={props.total}
            onPageChange={props.setPage}
          />
        </QueryState>
        <div className='space-y-3 border-t pt-3'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div>
              <h4 className='text-sm font-semibold'>{t('Share Requests')}</h4>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t('Review cross-department policy group sharing.')}
              </p>
            </div>
            <div className='flex items-center gap-2'>
              <Select
                value={props.shareRequestStatus || ALL_VALUE}
                onValueChange={(value) => {
                  props.setShareRequestStatus(
                    normalizeOptionalSelectValue(value)
                  )
                  props.setShareRequestPage(1)
                }}
              >
                <SelectTrigger className='w-36'>
                  <SelectValue placeholder={t('Status')} />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value={ALL_VALUE}>
                      {t('All Statuses')}
                    </SelectItem>
                    <SelectItem value='pending'>{t('Pending')}</SelectItem>
                    <SelectItem value='approved'>{t('Approved')}</SelectItem>
                    <SelectItem value='rejected'>{t('Rejected')}</SelectItem>
                    <SelectItem value='withdrawn'>{t('Withdrawn')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <Button
                variant='outline'
                size='icon-sm'
                onClick={() => props.shareRequestsQuery.refetch()}
              >
                <RefreshCcw className='size-3.5' />
                <span className='sr-only'>{t('Refresh')}</span>
              </Button>
            </div>
          </div>
          <QueryState
            query={props.shareRequestsQuery}
            empty={props.shareRequests.length === 0}
            emptyTitle='No share requests'
            emptyDescription='Create a request from a policy group row to share it with another department.'
          >
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Policy Group')}</TableHead>
                  <TableHead>{t('From')}</TableHead>
                  <TableHead>{t('To')}</TableHead>
                  <TableHead>{t('Role')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Expires')}</TableHead>
                  <TableHead>{t('Updated')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {props.shareRequests.map((request) => (
                  <TableRow key={request.id}>
                    <TableCell>
                      <div className='min-w-0'>
                        <div className='truncate font-medium'>
                          {request.policy_group_name ||
                            `#${request.policy_group_id}`}
                        </div>
                        <div className='text-muted-foreground truncate text-xs'>
                          {request.reason || request.requester_name || '-'}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      {request.requester_org_unit_name || '-'}
                    </TableCell>
                    <TableCell>{request.target_org_unit_name || '-'}</TableCell>
                    <TableCell>
                      <PolicyGroupShareRoleBadge role={request.role} />
                    </TableCell>
                    <TableCell>
                      <PolicyGroupShareRequestStatusBadge
                        status={request.status}
                      />
                    </TableCell>
                    <TableCell>
                      {request.shared_expires_at > 0
                        ? formatDateTime(request.shared_expires_at)
                        : t('Never')}
                    </TableCell>
                    <TableCell>{formatDateTime(request.updated_at)}</TableCell>
                    <TableCell>
                      <div className='flex justify-end gap-1'>
                        {request.can_decide && request.status === 'pending' ? (
                          <>
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              onClick={() =>
                                props.onDecideShareRequest(request, 'approve')
                              }
                            >
                              <Check className='size-3.5' />
                              <span className='sr-only'>{t('Approve')}</span>
                            </Button>
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              onClick={() =>
                                props.onDecideShareRequest(request, 'reject')
                              }
                            >
                              <Ban className='size-3.5' />
                              <span className='sr-only'>{t('Reject')}</span>
                            </Button>
                          </>
                        ) : (
                          <span className='text-muted-foreground text-xs'>
                            -
                          </span>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <Pager
              page={props.shareRequestPage}
              pageSize={PAGE_SIZE}
              total={props.shareRequestTotal}
              onPageChange={props.setShareRequestPage}
            />
          </QueryState>
        </div>
      </div>
    </Panel>
  )
}

function ProjectsTab(props: {
  projects: EnterpriseProject[]
  orgOptions: SelectOption[]
  query: QueryResult<PageInfo<EnterpriseProject>>
  page: number
  total: number
  setPage: (page: number) => void
  keyword: string
  setKeyword: (value: string) => void
  status: string
  setStatus: (value: string) => void
  orgUnitId: string
  setOrgUnitId: (value: string) => void
  onCreate: () => void
  onEdit: (project: EnterpriseProject) => void
  onManageMembers: (project: EnterpriseProject) => void
  onDisable: (project: EnterpriseProject) => void
  onViewUsage: (project: EnterpriseProject) => void
  canCreateProject: boolean
}) {
  const { t } = useTranslation()
  const [confirmProject, setConfirmProject] =
    useState<EnterpriseProject | null>(null)

  return (
    <>
      <Panel
        title='Projects'
        description='Cost centers and application scopes for API Key defaults, request attribution, and project limits.'
        actions={
          props.canCreateProject ? (
            <Button size='sm' onClick={props.onCreate}>
              <Plus className='size-3.5' />
              {t('Project')}
            </Button>
          ) : null
        }
      >
        <div className='space-y-3'>
          <FilterBar>
            <SearchInput
              value={props.keyword}
              onChange={(value) => {
                props.setKeyword(value)
                props.setPage(1)
              }}
              placeholder='Search projects'
            />
            <StatusFilter
              value={props.status}
              onChange={(value) => {
                props.setStatus(value)
                props.setPage(1)
              }}
            />
            <Select
              value={props.orgUnitId || ALL_VALUE}
              onValueChange={(value) => {
                props.setOrgUnitId(normalizeOptionalSelectValue(value))
                props.setPage(1)
              }}
            >
              <SelectTrigger className='w-44'>
                <SelectValue placeholder={t('Org Unit')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL_VALUE}>
                    {t('All Org Units')}
                  </SelectItem>
                  {props.orgOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </FilterBar>
          <QueryState
            query={props.query}
            empty={props.projects.length === 0}
            emptyTitle='No projects'
            emptyDescription='Create cost centers for API Key defaults and project-level quota policies.'
          >
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Project')}</TableHead>
                  <TableHead>{t('Org Units')}</TableHead>
                  <TableHead>{t('Owner')}</TableHead>
                  <TableHead>{t('Members')}</TableHead>
                  <TableHead>{t('Policies')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Updated')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {props.projects.map((project) => (
                  <TableRow key={project.id}>
                    <TableCell>
                      <div className='min-w-56'>
                        <div className='truncate font-medium'>
                          {project.name}
                        </div>
                        <div className='text-muted-foreground truncate text-xs'>
                          {project.slug}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <ProjectOrgUnitList names={project.org_unit_names} />
                    </TableCell>
                    <TableCell>
                      {project.owner_name ||
                        (project.owner_user_id > 0
                          ? `#${project.owner_user_id}`
                          : '-')}
                    </TableCell>
                    <TableCell>{formatNumber(project.member_count)}</TableCell>
                    <TableCell>{formatNumber(project.policy_count)}</TableCell>
                    <TableCell>
                      <StatusBadge status={project.status} />
                    </TableCell>
                    <TableCell>{formatDateTime(project.updated_at)}</TableCell>
                    <TableCell>
                      <div className='flex justify-end gap-1'>
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          onClick={() => props.onViewUsage(project)}
                        >
                          <Gauge className='size-3.5' />
                          <span className='sr-only'>{t('Usage')}</span>
                        </Button>
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          onClick={() => props.onManageMembers(project)}
                        >
                          <Users className='size-3.5' />
                          <span className='sr-only'>{t('Members')}</span>
                        </Button>
                        {project.can_manage ? (
                          <>
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              onClick={() => props.onEdit(project)}
                            >
                              <Pencil className='size-3.5' />
                              <span className='sr-only'>{t('Edit')}</span>
                            </Button>
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              disabled={project.status === DISABLED_STATUS}
                              onClick={() => setConfirmProject(project)}
                            >
                              <Ban className='size-3.5' />
                              <span className='sr-only'>{t('Disable')}</span>
                            </Button>
                          </>
                        ) : null}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <Pager
              page={props.page}
              pageSize={PAGE_SIZE}
              total={props.total}
              onPageChange={props.setPage}
            />
          </QueryState>
        </div>
      </Panel>
      <ProjectDisableDialog
        project={confirmProject}
        onOpenChange={(open) => {
          if (!open) setConfirmProject(null)
        }}
        onConfirm={(project) => props.onDisable(project)}
      />
    </>
  )
}

function ProjectOrgUnitList(props: { names: string[] }) {
  const { t } = useTranslation()
  if (props.names.length === 0) {
    return <span className='text-muted-foreground'>{t('Unassigned')}</span>
  }
  const visible = props.names.slice(0, 2)
  const remaining = props.names.length - visible.length
  return (
    <div className='flex max-w-72 flex-wrap gap-1'>
      {visible.map((name) => (
        <Badge key={name} variant='secondary' className='max-w-40 truncate'>
          {name}
        </Badge>
      ))}
      {remaining > 0 && (
        <Badge variant='outline'>+{formatNumber(remaining)}</Badge>
      )}
    </div>
  )
}

function ProjectDisableDialog(props: {
  project: EnterpriseProject | null
  onOpenChange: (open: boolean) => void
  onConfirm: (project: EnterpriseProject) => void
}) {
  const { t } = useTranslation()
  const project = props.project

  return (
    <AlertDialog open={Boolean(project)} onOpenChange={props.onOpenChange}>
      <AlertDialogContent className='sm:max-w-lg'>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('Confirm disable')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              'Disabling this project stops new API Key defaults and request attribution overrides from using it.'
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        {project && (
          <div className='space-y-3 rounded-lg border p-3 text-sm'>
            <div className='min-w-0'>
              <div className='truncate font-medium'>{project.name}</div>
              <div className='text-muted-foreground mt-1 text-xs'>
                {project.slug}
              </div>
            </div>
            <div className='grid gap-3 sm:grid-cols-2'>
              <AuditDetailField
                label='Policies'
                value={formatNumber(project.policy_count)}
              />
              <AuditDetailField
                label='Org Units'
                value={formatNumber(project.org_unit_ids.length)}
              />
            </div>
          </div>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
          <AlertDialogAction
            className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            onClick={() => {
              if (project) props.onConfirm(project)
            }}
          >
            {t('Disable')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

function PolicyGroupSharedOrgUnitList(props: { group: EnterprisePolicyGroup }) {
  const { t } = useTranslation()
  if (props.group.shared_org_unit_ids.length === 0) {
    return <span className='text-muted-foreground'>{t('Unassigned')}</span>
  }
  const visible = props.group.shared_org_unit_ids.slice(0, 2)
  const remaining = props.group.shared_org_unit_ids.length - visible.length
  return (
    <div className='flex max-w-80 flex-wrap gap-1'>
      {visible.map((orgUnitId, index) => (
        <Badge
          key={orgUnitId}
          variant='secondary'
          className='max-w-56 gap-1 truncate'
        >
          <span className='truncate'>
            {props.group.shared_org_unit_names[index] || `#${orgUnitId}`}
          </span>
          <span className='text-muted-foreground'>
            {t(
              formatPolicyGroupShareRole(
                props.group.shared_org_unit_roles?.[String(orgUnitId)]
              )
            )}
          </span>
        </Badge>
      ))}
      {remaining > 0 && (
        <Badge variant='outline'>+{formatNumber(remaining)}</Badge>
      )}
    </div>
  )
}

function ProjectMembersDialog(props: {
  open: boolean
  project?: EnterpriseProject | null
  onOpenChange: (open: boolean) => void
  canManage: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const projectId = props.project?.id ?? 0
  const [userId, setUserId] = useState('')
  const [role, setRole] = useState('member')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)

  const membersQuery = useQuery({
    queryKey: ['enterprise', 'projects', projectId, 'members', page, keyword],
    queryFn: () =>
      getEnterpriseProjectMembers(projectId, {
        p: page,
        page_size: PAGE_SIZE,
        keyword,
      }),
    enabled: props.open && projectId > 0,
  })
  const members = getPageItems(membersQuery.data)
  const total = getPageTotal(membersQuery.data)

  const upsertMutation = useMutation({
    mutationFn: () =>
      upsertEnterpriseProjectMember(projectId, {
        user_id: Number(userId),
        role,
      }),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Saved'))
      setUserId('')
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })
  const deleteMutation = useMutation({
    mutationFn: (targetUserId: number) =>
      deleteEnterpriseProjectMember(projectId, targetUserId),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Removed'))
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!Number(userId)) return
    upsertMutation.mutate()
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[85vh] overflow-y-auto sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Project Members')}</DialogTitle>
          <DialogDescription>
            {props.project?.name
              ? t('Manage members for {{name}}.', { name: props.project.name })
              : t('Manage project members.')}
          </DialogDescription>
        </DialogHeader>

        {props.canManage ? (
          <form
            className='flex flex-wrap items-end gap-2'
            onSubmit={handleSubmit}
          >
            <Field label='User ID'>
              <Input
                type='number'
                value={userId}
                onChange={(event) => setUserId(event.target.value)}
                placeholder={t('User ID')}
                className='w-36'
              />
            </Field>
            <Field label='Role'>
              <Select
                value={role}
                onValueChange={(value) =>
                  setRole(normalizeSelectValue(value) || 'member')
                }
              >
                <SelectTrigger className='w-40'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='member'>{t('Member')}</SelectItem>
                    <SelectItem value='admin'>{t('Admin')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Button type='submit' disabled={upsertMutation.isPending}>
              <UserPlus className='size-3.5' />
              {t('Save')}
            </Button>
          </form>
        ) : null}

        <FilterBar>
          <SearchInput
            value={keyword}
            onChange={(value) => {
              setKeyword(value)
              setPage(1)
            }}
            placeholder='Search members'
          />
        </FilterBar>

        <QueryState
          query={{
            data: membersQuery.data,
            isLoading: membersQuery.isLoading,
            isError: membersQuery.isError,
            error: membersQuery.error,
            refetch: membersQuery.refetch,
          }}
          empty={members.length === 0}
          emptyTitle='No members'
          emptyDescription='Add users by ID to delegate project access.'
        >
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('User')}</TableHead>
                <TableHead>{t('Role')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                {props.canManage ? (
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                ) : null}
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.map((member) => (
                <TableRow key={member.user_id}>
                  <TableCell>
                    <div className='min-w-0'>
                      <div className='truncate font-medium'>
                        {member.display_name || member.username}
                      </div>
                      <div className='text-muted-foreground truncate text-xs'>
                        #{member.user_id} · {member.email || member.username}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant='secondary'>
                      {t(member.role === 'admin' ? 'Admin' : 'Member')}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={member.status} />
                  </TableCell>
                  {props.canManage ? (
                    <TableCell>
                      <div className='flex justify-end'>
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          disabled={deleteMutation.isPending}
                          onClick={() => deleteMutation.mutate(member.user_id)}
                        >
                          <Trash2 className='size-3.5' />
                          <span className='sr-only'>{t('Remove')}</span>
                        </Button>
                      </div>
                    </TableCell>
                  ) : null}
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <Pager
            page={page}
            pageSize={PAGE_SIZE}
            total={total}
            onPageChange={setPage}
          />
        </QueryState>
      </DialogContent>
    </Dialog>
  )
}

function QuotaPoliciesTab(props: {
  policies: EnterpriseQuotaPolicy[]
  orgUnits: EnterpriseOrgUnit[]
  projects: EnterpriseProject[]
  membersTotal: number
  policyGroupsTotal: number
  dryRunObservations: EnterpriseAuditLog[]
  query: QueryResult<PageInfo<EnterpriseQuotaPolicy>>
  page: number
  total: number
  setPage: (page: number) => void
  keyword: string
  setKeyword: (value: string) => void
  status: string
  setStatus: (value: string) => void
  targetType: string
  setTargetType: (value: string) => void
  metric: string
  setMetric: (value: string) => void
  onCreate: () => void
  onEdit: (policy: EnterpriseQuotaPolicy) => void
  onDisable: (policy: EnterpriseQuotaPolicy) => void
}) {
  const { t } = useTranslation()
  const [confirmPolicy, setConfirmPolicy] =
    useState<EnterpriseQuotaPolicy | null>(null)

  return (
    <>
      <Panel
        title='Quota Policies'
        description='Priority-ordered limits enforced before relay billing settlement.'
        actions={
          <Button size='sm' onClick={props.onCreate}>
            <Plus className='size-3.5' />
            {t('Quota Policy')}
          </Button>
        }
      >
        <div className='space-y-3'>
          <FilterBar>
            <SearchInput
              value={props.keyword}
              onChange={(value) => {
                props.setKeyword(value)
                props.setPage(1)
              }}
              placeholder='Search quota policies'
            />
            <StatusFilter
              value={props.status}
              onChange={(value) => {
                props.setStatus(value)
                props.setPage(1)
              }}
            />
            <Select
              value={props.targetType || ALL_VALUE}
              onValueChange={(value) => {
                props.setTargetType(normalizeOptionalSelectValue(value))
                props.setPage(1)
              }}
            >
              <SelectTrigger className='w-40'>
                <SelectValue placeholder={t('Target')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL_VALUE}>{t('All Targets')}</SelectItem>
                  <SelectItem value='enterprise'>{t('Enterprise')}</SelectItem>
                  <SelectItem value='org_unit'>{t('Org Unit')}</SelectItem>
                  <SelectItem value='project'>{t('Project')}</SelectItem>
                  <SelectItem value='policy_group'>
                    {t('Policy Group')}
                  </SelectItem>
                  <SelectItem value='user'>{t('User')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
            <Select
              value={props.metric || ALL_VALUE}
              onValueChange={(value) => {
                props.setMetric(normalizeOptionalSelectValue(value))
                props.setPage(1)
              }}
            >
              <SelectTrigger className='w-36'>
                <SelectValue placeholder={t('Metric')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL_VALUE}>{t('All Metrics')}</SelectItem>
                  <SelectItem value='request_count'>{t('Requests')}</SelectItem>
                  <SelectItem value='quota'>{t('Quota')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </FilterBar>
          <QueryState
            query={props.query}
            empty={props.policies.length === 0}
            emptyContent={
              <FirstQuotaPolicyGuide
                orgUnits={props.orgUnits}
                membersTotal={props.membersTotal}
                policyGroupsTotal={props.policyGroupsTotal}
                onCreate={props.onCreate}
              />
            }
          >
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Policy')}</TableHead>
                  <TableHead>{t('Target')}</TableHead>
                  <TableHead>{t('Metric')}</TableHead>
                  <TableHead>{t('Usage')}</TableHead>
                  <TableHead>{t('Priority')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {props.policies.map((policy) => {
                  const models = parseModelsJson(policy)
                  return (
                    <TableRow key={policy.id}>
                      <TableCell>
                        <div className='min-w-56'>
                          <div className='truncate font-medium'>
                            {policy.name}
                          </div>
                          <div className='text-muted-foreground truncate text-xs'>
                            {policy.condition_mode.toUpperCase()} ·{' '}
                            {policy.model_scope === 'all'
                              ? t('All Models')
                              : models.join(', ') || '-'}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>{t(formatPolicyTarget(policy))}</TableCell>
                      <TableCell>
                        <div className='flex flex-wrap items-center gap-1'>
                          <Badge variant='outline'>
                            {t(formatMetric(policy.metric))}
                          </Badge>
                          <Badge variant='secondary'>
                            {t(formatPeriod(policy.period))}
                          </Badge>
                          <Badge variant='outline'>
                            {t(formatPolicyAction(policy.action))}
                          </Badge>
                        </div>
                      </TableCell>
                      <TableCell>
                        <UsageBar
                          used={policy.used_value}
                          limit={policy.limit_value}
                        />
                      </TableCell>
                      <TableCell>{policy.priority}</TableCell>
                      <TableCell>
                        <StatusBadge status={policy.status} />
                      </TableCell>
                      <TableCell>
                        <div className='flex justify-end gap-1'>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            onClick={() => props.onEdit(policy)}
                          >
                            <Pencil className='size-3.5' />
                            <span className='sr-only'>{t('Edit')}</span>
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            disabled={policy.status === DISABLED_STATUS}
                            onClick={() => setConfirmPolicy(policy)}
                          >
                            <Ban className='size-3.5' />
                            <span className='sr-only'>{t('Disable')}</span>
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
            <Pager
              page={props.page}
              pageSize={PAGE_SIZE}
              total={props.total}
              onPageChange={props.setPage}
            />
          </QueryState>
        </div>
      </Panel>
      <QuotaPolicyDisableDialog
        policy={confirmPolicy}
        dryRunObservations={props.dryRunObservations}
        onOpenChange={(open) => {
          if (!open) setConfirmPolicy(null)
        }}
        onConfirm={(policy) => props.onDisable(policy)}
      />
    </>
  )
}

function FirstQuotaPolicyGuide(props: {
  orgUnits: EnterpriseOrgUnit[]
  membersTotal: number
  policyGroupsTotal: number
  onCreate: () => void
}) {
  const { t } = useTranslation()
  const activeOrgUnits = props.orgUnits.filter(
    (unit) => unit.status !== DISABLED_STATUS
  ).length
  const steps = [
    {
      label: 'Create departments',
      done: activeOrgUnits > 0,
      detail: `${formatNumber(activeOrgUnits)} ${t('active org units')}`,
    },
    {
      label: 'Assign members',
      done: props.membersTotal > 0,
      detail: `${formatNumber(props.membersTotal)} ${t('members')}`,
    },
    {
      label: 'Create policy groups',
      done: props.policyGroupsTotal > 0,
      detail: `${formatNumber(props.policyGroupsTotal)} ${t('policy groups')}`,
    },
    {
      label: 'Create quota policy',
      done: false,
      detail: t('Start with dry-run traffic review, then enforce hard limit'),
    },
  ]

  return (
    <div className='bg-muted/15 rounded-lg border p-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='min-w-0'>
          <h4 className='text-sm font-semibold'>
            {t('Create the first quota policy')}
          </h4>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t(
              'Prepare org ownership first, then add a conservative daily or monthly limit.'
            )}
          </p>
        </div>
        <Button size='sm' onClick={props.onCreate}>
          <Plus className='size-3.5' />
          {t('Quota Policy')}
        </Button>
      </div>
      <div className='mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-4'>
        {steps.map((step, index) => (
          <div key={step.label} className='bg-background rounded-md border p-3'>
            <div className='flex items-center justify-between gap-2'>
              <span className='text-muted-foreground text-xs'>
                {t('Step')} {index + 1}
              </span>
              <Badge variant={step.done ? 'outline' : 'secondary'}>
                {step.done ? t('Ready') : t('Pending')}
              </Badge>
            </div>
            <div className='mt-2 text-sm font-medium'>{t(step.label)}</div>
            <div className='text-muted-foreground mt-1 text-xs'>
              {step.detail}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function QuotaPolicyDisableDialog(props: {
  policy: EnterpriseQuotaPolicy | null
  dryRunObservations: EnterpriseAuditLog[]
  onOpenChange: (open: boolean) => void
  onConfirm: (policy: EnterpriseQuotaPolicy) => void
}) {
  const { t } = useTranslation()
  const policy = props.policy
  const dryRunHits = policy
    ? getPolicyDryRunHits(policy, props.dryRunObservations)
    : 0

  return (
    <AlertDialog open={Boolean(policy)} onOpenChange={props.onOpenChange}>
      <AlertDialogContent className='sm:max-w-lg'>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('Confirm disable')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              'Disabling this hard limit immediately stops enforcement for matching relay requests.'
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        {policy && (
          <div className='space-y-3 rounded-lg border p-3 text-sm'>
            <div className='min-w-0'>
              <div className='truncate font-medium'>{policy.name}</div>
              <div className='text-muted-foreground mt-1 text-xs'>
                {t(formatPolicyTarget(policy))} ·{' '}
                {t(formatMetric(policy.metric))} ·{' '}
                {t(formatPeriod(policy.period))}
              </div>
            </div>
            <div className='grid gap-3 sm:grid-cols-2'>
              <AuditDetailField
                label='Current Usage'
                value={`${formatNumber(policy.used_value)} / ${formatNumber(policy.limit_value)}`}
              />
              <AuditDetailField
                label='Impact Risk'
                value={t(getPolicyRiskLabel(policy))}
              />
              <AuditDetailField
                label='Recent dry-run hits'
                value={formatNumber(dryRunHits)}
              />
              <AuditDetailField
                label='Target'
                value={`${policy.target_type} #${policy.target_id || '-'}`}
                mono
              />
            </div>
          </div>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
          <AlertDialogAction
            className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            onClick={() => {
              if (policy) props.onConfirm(policy)
            }}
          >
            {t('Disable')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

function QuotaRequestsTab(props: {
  requests: EnterpriseQuotaRequest[]
  policies: EnterpriseQuotaPolicy[]
  currentUserId: number
  isAdmin: boolean
  query: QueryResult<PageInfo<EnterpriseQuotaRequest>>
  page: number
  total: number
  setPage: (page: number) => void
  status: string
  setStatus: (value: string) => void
  requestId: string
  setRequestId: (value: string) => void
  policyId: string
  setPolicyId: (value: string) => void
  projectId: string
  setProjectId: (value: string) => void
  targetType: string
  setTargetType: (value: string) => void
  targetId: string
  setTargetId: (value: string) => void
  applicantUserId: string
  setApplicantUserId: (value: string) => void
  selectedRequestIds: number[]
  setSelectedRequestIds: (ids: number[]) => void
  onCreate: () => void
  onApprove: (request: EnterpriseQuotaRequest) => void
  onReject: (request: EnterpriseQuotaRequest) => void
  onBatchApprove: (ids: number[]) => void
  onBatchReject: (ids: number[]) => void
  onWithdraw: (request: EnterpriseQuotaRequest) => void
  onViewDetails: (request: EnterpriseQuotaRequest) => void
}) {
  const { t } = useTranslation()
  const pendingRequests = props.requests.filter(
    (request) => request.status === 'pending'
  )
  const pendingRequestIds = new Set(
    pendingRequests.map((request) => request.id)
  )
  const selectedPendingIds = props.selectedRequestIds.filter((id) =>
    pendingRequestIds.has(id)
  )
  const selectedPendingCount = selectedPendingIds.length
  const allPendingSelected =
    pendingRequests.length > 0 &&
    pendingRequests.every((request) =>
      props.selectedRequestIds.includes(request.id)
    )
  const toggleRequestSelection = (request: EnterpriseQuotaRequest) => {
    if (request.status !== 'pending') return
    props.setSelectedRequestIds(
      props.selectedRequestIds.includes(request.id)
        ? props.selectedRequestIds.filter((id) => id !== request.id)
        : [...props.selectedRequestIds, request.id]
    )
  }
  const toggleAllPending = (checked: boolean) => {
    if (!checked) {
      props.setSelectedRequestIds(
        props.selectedRequestIds.filter((id) => !pendingRequestIds.has(id))
      )
      return
    }
    props.setSelectedRequestIds([
      ...props.selectedRequestIds.filter((id) => !pendingRequestIds.has(id)),
      ...pendingRequests.map((request) => request.id),
    ])
  }

  return (
    <Panel
      title='Quota Requests'
      description='Temporary quota approvals that extend matching hard limits until expiration.'
      actions={
        <div className='flex flex-wrap items-center gap-2'>
          {props.isAdmin && selectedPendingCount > 0 && (
            <>
              <Button
                variant='outline'
                size='sm'
                onClick={() => props.onBatchApprove(selectedPendingIds)}
              >
                <Check className='size-3.5' />
                {t('Approve Selected')} ({selectedPendingCount})
              </Button>
              <Button
                variant='outline'
                size='sm'
                onClick={() => props.onBatchReject(selectedPendingIds)}
              >
                <Ban className='size-3.5' />
                {t('Reject Selected')} ({selectedPendingCount})
              </Button>
            </>
          )}
          <Button size='sm' onClick={props.onCreate}>
            <Plus className='size-3.5' />
            {t('Quota Request')}
          </Button>
        </div>
      }
    >
      <div className='space-y-3'>
        <FilterBar>
          <Select
            value={props.status || ALL_VALUE}
            onValueChange={(value) => {
              props.setStatus(normalizeOptionalSelectValue(value))
              props.setPage(1)
            }}
          >
            <SelectTrigger className='w-40'>
              <SelectValue placeholder={t('Status')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value={ALL_VALUE}>{t('All Statuses')}</SelectItem>
                <SelectItem value='pending'>{t('Pending')}</SelectItem>
                <SelectItem value='approved'>{t('Approved')}</SelectItem>
                <SelectItem value='rejected'>{t('Rejected')}</SelectItem>
                <SelectItem value='withdrawn'>{t('Withdrawn')}</SelectItem>
                <SelectItem value='expired'>{t('Expired')}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
          <Select
            value={props.policyId || ALL_VALUE}
            onValueChange={(value) => {
              props.setPolicyId(normalizeOptionalSelectValue(value))
              props.setPage(1)
            }}
          >
            <SelectTrigger className='w-56'>
              <SelectValue placeholder={t('Quota Policy')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value={ALL_VALUE}>{t('All Policies')}</SelectItem>
                {props.policies.map((policy) => (
                  <SelectItem key={policy.id} value={String(policy.id)}>
                    {policy.name}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Input
            type='number'
            value={props.requestId}
            onChange={(event) => {
              props.setRequestId(event.target.value)
              props.setPage(1)
            }}
            placeholder={t('Request ID')}
            className='w-36'
          />
          <Input
            type='number'
            value={props.projectId}
            onChange={(event) => {
              props.setProjectId(event.target.value)
              props.setPage(1)
            }}
            placeholder={t('Project ID')}
            className='w-36'
          />
          <Select
            value={props.targetType || ALL_VALUE}
            onValueChange={(value) => {
              props.setTargetType(normalizeOptionalSelectValue(value))
              props.setPage(1)
            }}
          >
            <SelectTrigger className='w-44'>
              <SelectValue placeholder={t('Target Type')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value={ALL_VALUE}>{t('All Targets')}</SelectItem>
                <SelectItem value='enterprise'>{t('Enterprise')}</SelectItem>
                <SelectItem value='org_unit'>{t('Org Unit')}</SelectItem>
                <SelectItem value='project'>{t('Project')}</SelectItem>
                <SelectItem value='policy_group'>
                  {t('Policy Group')}
                </SelectItem>
                <SelectItem value='user'>{t('User')}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
          <Input
            type='number'
            value={props.targetId}
            onChange={(event) => {
              props.setTargetId(event.target.value)
              props.setPage(1)
            }}
            placeholder={t('Target ID')}
            className='w-36'
          />
          {props.isAdmin && (
            <Input
              type='number'
              value={props.applicantUserId}
              onChange={(event) => {
                props.setApplicantUserId(event.target.value)
                props.setPage(1)
              }}
              placeholder={t('Applicant User ID')}
              className='w-44'
            />
          )}
        </FilterBar>
        <QueryState
          query={props.query}
          empty={props.requests.length === 0}
          emptyTitle='No quota requests'
          emptyDescription='Submit temporary quota requests for controlled over-limit access.'
        >
          <Table>
            <TableHeader>
              <TableRow>
                {props.isAdmin && (
                  <TableHead className='w-10'>
                    <Checkbox
                      checked={allPendingSelected}
                      disabled={pendingRequests.length === 0}
                      onCheckedChange={(checked) =>
                        toggleAllPending(checked === true)
                      }
                    />
                  </TableHead>
                )}
                <TableHead>{t('Request')}</TableHead>
                <TableHead>{t('Policy')}</TableHead>
                <TableHead>{t('Target')}</TableHead>
                <TableHead>{t('Extra Limit')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Expires')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {props.requests.map((request) => (
                <TableRow key={request.id}>
                  {props.isAdmin && (
                    <TableCell>
                      <Checkbox
                        checked={props.selectedRequestIds.includes(request.id)}
                        disabled={request.status !== 'pending'}
                        onCheckedChange={() => toggleRequestSelection(request)}
                      />
                    </TableCell>
                  )}
                  <TableCell>
                    <div className='min-w-56'>
                      <div className='truncate font-medium'>
                        {request.applicant_name ||
                          `#${request.applicant_user_id}`}
                      </div>
                      <div className='text-muted-foreground truncate text-xs'>
                        {request.reason || t('No reason provided')}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className='min-w-48'>
                      <div className='truncate'>
                        {request.policy_name || `#${request.policy_id}`}
                      </div>
                      <div className='text-muted-foreground text-xs'>
                        {t(formatMetric(request.metric))} ·{' '}
                        {t(formatPeriod(request.period))}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    {request.target_name ||
                      `${request.target_type} #${request.target_id}`}
                  </TableCell>
                  <TableCell>{formatNumber(request.limit_delta)}</TableCell>
                  <TableCell>
                    <QuotaRequestStatusBadge status={request.status} />
                  </TableCell>
                  <TableCell>{formatDateTime(request.expires_at)}</TableCell>
                  <TableCell>
                    <div className='flex justify-end gap-1'>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        onClick={() => props.onViewDetails(request)}
                      >
                        <Eye className='size-3.5' />
                        <span className='sr-only'>{t('Details')}</span>
                      </Button>
                      {props.isAdmin && (
                        <>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            disabled={request.status !== 'pending'}
                            onClick={() => props.onApprove(request)}
                          >
                            <Check className='size-3.5' />
                            <span className='sr-only'>{t('Approve')}</span>
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            disabled={request.status !== 'pending'}
                            onClick={() => props.onReject(request)}
                          >
                            <Ban className='size-3.5' />
                            <span className='sr-only'>{t('Reject')}</span>
                          </Button>
                        </>
                      )}
                      {(props.isAdmin ||
                        request.applicant_user_id === props.currentUserId) && (
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          disabled={request.status !== 'pending'}
                          onClick={() => props.onWithdraw(request)}
                        >
                          <Trash2 className='size-3.5' />
                          <span className='sr-only'>{t('Withdraw')}</span>
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <Pager
            page={props.page}
            pageSize={PAGE_SIZE}
            total={props.total}
            onPageChange={props.setPage}
          />
        </QueryState>
      </div>
    </Panel>
  )
}

function UsageTab(props: {
  summary?: EnterpriseUsageSummary
  projects: EnterpriseProject[]
  summaryQuery: QueryResult<EnterpriseUsageSummary>
  breakdown: EnterpriseUsageBreakdownItem[]
  breakdownQuery: QueryResult<PageInfo<EnterpriseUsageBreakdownItem>>
  breakdownTotal: number
  page: number
  setPage: (page: number) => void
  startDate: string
  setStartDate: (value: string) => void
  endDate: string
  setEndDate: (value: string) => void
  dimension: UsageDimension
  setDimension: (value: UsageDimension) => void
  granularity: UsageGranularity
  setGranularity: (value: UsageGranularity) => void
  modelName: string
  setModelName: (value: string) => void
  projectId: string
  setProjectId: (value: string) => void
  channelId: string
  setChannelId: (value: string) => void
  tokenId: string
  setTokenId: (value: string) => void
  status: string
  setStatus: (value: string) => void
  onExport: () => void
  isExporting: boolean
}) {
  const { t } = useTranslation()
  const total = props.summary?.total

  return (
    <div className='space-y-3'>
      <FilterBar>
        <Input
          type='date'
          value={props.startDate}
          onChange={(event) => {
            props.setStartDate(event.target.value)
            props.setPage(1)
          }}
          className='w-40'
        />
        <Input
          type='date'
          value={props.endDate}
          onChange={(event) => {
            props.setEndDate(event.target.value)
            props.setPage(1)
          }}
          className='w-40'
        />
        <Select
          value={props.granularity}
          onValueChange={(value) => {
            props.setGranularity(
              normalizeSelectValue(value) as UsageGranularity
            )
            props.setPage(1)
          }}
        >
          <SelectTrigger className='w-32'>
            <SelectValue placeholder={t('Granularity')} />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              <SelectItem value='day'>{t('Daily')}</SelectItem>
              <SelectItem value='month'>{t('Monthly')}</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
        <Select
          value={props.dimension}
          onValueChange={(value) => {
            props.setDimension(normalizeSelectValue(value) as UsageDimension)
            props.setPage(1)
          }}
        >
          <SelectTrigger className='w-44'>
            <SelectValue placeholder={t('Dimension')} />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              <SelectItem value='org_unit'>{t('Org Unit')}</SelectItem>
              <SelectItem value='project'>{t('Project')}</SelectItem>
              <SelectItem value='policy_group'>{t('Policy Group')}</SelectItem>
              <SelectItem value='user'>{t('User')}</SelectItem>
              <SelectItem value='model'>{t('Model')}</SelectItem>
              <SelectItem value='channel'>{t('Channel')}</SelectItem>
              <SelectItem value='api_key'>{t('API Key')}</SelectItem>
              <SelectItem value='time'>{t('Time')}</SelectItem>
              <SelectItem value='status'>{t('Status')}</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
        <Select
          value={props.projectId || ALL_VALUE}
          onValueChange={(value) => {
            props.setProjectId(normalizeOptionalSelectValue(value))
            props.setPage(1)
          }}
        >
          <SelectTrigger className='w-44'>
            <SelectValue placeholder={t('Project')} />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              <SelectItem value={ALL_VALUE}>{t('All Projects')}</SelectItem>
              {props.projects.map((project) => (
                <SelectItem key={project.id} value={String(project.id)}>
                  {project.name}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
        <Input
          value={props.modelName}
          onChange={(event) => {
            props.setModelName(event.target.value)
            props.setPage(1)
          }}
          placeholder={t('Model')}
          className='w-44'
        />
        <Input
          type='number'
          value={props.channelId}
          onChange={(event) => {
            props.setChannelId(event.target.value)
            props.setPage(1)
          }}
          placeholder={t('Channel ID')}
          className='w-36'
        />
        <Input
          type='number'
          value={props.tokenId}
          onChange={(event) => {
            props.setTokenId(event.target.value)
            props.setPage(1)
          }}
          placeholder={t('API Key ID')}
          className='w-36'
        />
        <Select
          value={props.status || ALL_VALUE}
          onValueChange={(value) => {
            props.setStatus(normalizeOptionalSelectValue(value))
            props.setPage(1)
          }}
        >
          <SelectTrigger className='w-36'>
            <SelectValue placeholder={t('Status')} />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              <SelectItem value={ALL_VALUE}>{t('All Status')}</SelectItem>
              <SelectItem value='success'>{t('Success')}</SelectItem>
              <SelectItem value='failed'>{t('Failed')}</SelectItem>
              <SelectItem value='rejected'>{t('Rejected')}</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
      </FilterBar>

      <div className='grid gap-2 md:grid-cols-4'>
        <StatCell
          icon={ClipboardList}
          label='Requests'
          value={formatNumber(total?.request_count)}
        />
        <StatCell
          icon={Gauge}
          label='Quota'
          value={formatNumber(total?.quota)}
        />
        <StatCell
          icon={Activity}
          label='Prompt Tokens'
          value={formatNumber(total?.prompt_tokens)}
        />
        <StatCell
          icon={Activity}
          label='Total Tokens'
          value={formatNumber(total?.total_tokens)}
        />
      </div>

      <Panel
        title='Usage Breakdown'
        actions={
          <Button
            variant='outline'
            size='sm'
            disabled={props.isExporting}
            onClick={props.onExport}
          >
            <Download className='size-3.5' />
            {t('Export')}
          </Button>
        }
      >
        <QueryState
          query={props.breakdownQuery}
          empty={props.breakdown.length === 0}
          emptyTitle='No usage records'
          emptyDescription='Governance usage attribution has not produced rows for this filter.'
        >
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Target')}</TableHead>
                <TableHead>{t('Requests')}</TableHead>
                <TableHead>{t('Quota')}</TableHead>
                <TableHead>{t('Prompt')}</TableHead>
                <TableHead>{t('Completion')}</TableHead>
                <TableHead>{t('Tokens')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {props.breakdown.map((item) => (
                <TableRow
                  key={`${item.dimension}-${item.target_id}-${item.model_name}-${item.status}`}
                >
                  <TableCell>
                    {item.target_name ||
                      item.model_name ||
                      item.time_bucket ||
                      item.status ||
                      '-'}
                  </TableCell>
                  <TableCell>{formatNumber(item.request_count)}</TableCell>
                  <TableCell>{formatNumber(item.quota)}</TableCell>
                  <TableCell>{formatNumber(item.prompt_tokens)}</TableCell>
                  <TableCell>{formatNumber(item.completion_tokens)}</TableCell>
                  <TableCell>{formatNumber(item.total_tokens)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <Pager
            page={props.page}
            pageSize={PAGE_SIZE}
            total={props.breakdownTotal}
            onPageChange={props.setPage}
          />
        </QueryState>
      </Panel>
    </div>
  )
}

function AuditTab(props: {
  logs: EnterpriseAuditLog[]
  dryRunObservations: EnterpriseAuditLog[]
  queueAdmissions: EnterpriseQueueAdmission[]
  query: QueryResult<PageInfo<EnterpriseAuditLog>>
  dryRunQuery: QueryResult<PageInfo<EnterpriseAuditLog>>
  queueAdmissionsQuery: QueryResult<PageInfo<EnterpriseQueueAdmission>>
  page: number
  total: number
  setPage: (page: number) => void
  action: string
  setAction: (value: string) => void
  targetType: string
  setTargetType: (value: string) => void
  targetId: string
  setTargetId: (value: string) => void
  actorUserId: string
  setActorUserId: (value: string) => void
  requestId: string
  setRequestId: (value: string) => void
  startDate: string
  setStartDate: (value: string) => void
  endDate: string
  setEndDate: (value: string) => void
  onRequestQuota: (initialValues: QuotaRequestInitialValues) => void
  canManageQueue: boolean
}) {
  const { t } = useTranslation()
  const [selectedLog, setSelectedLog] = useState<EnterpriseAuditLog | null>(
    null
  )

  return (
    <Panel
      title='Audit Logs'
      description='Administrative changes and relay governance audit events.'
    >
      <div className='space-y-3'>
        <DryRunObservationsPanel
          logs={props.dryRunObservations}
          query={props.dryRunQuery}
          onView={setSelectedLog}
          onRequestQuota={props.onRequestQuota}
        />
        <QueueAdmissionsPanel
          admissions={props.queueAdmissions}
          query={props.queueAdmissionsQuery}
          canManage={props.canManageQueue}
        />
        <FilterBar>
          <Input
            value={props.action}
            onChange={(event) => {
              props.setAction(event.target.value)
              props.setPage(1)
            }}
            placeholder={t('Action')}
            className='w-52'
          />
          <Select
            value={props.targetType || ALL_VALUE}
            onValueChange={(value) => {
              props.setTargetType(normalizeOptionalSelectValue(value))
              props.setPage(1)
            }}
          >
            <SelectTrigger className='w-44'>
              <SelectValue placeholder={t('Target Type')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value={ALL_VALUE}>{t('All Targets')}</SelectItem>
                <SelectItem value='enterprise'>{t('Enterprise')}</SelectItem>
                <SelectItem value='org_unit'>{t('Org Unit')}</SelectItem>
                <SelectItem value='policy_group'>
                  {t('Policy Group')}
                </SelectItem>
                <SelectItem value='quota_policy'>
                  {t('Quota Policy')}
                </SelectItem>
                <SelectItem value='quota_request'>
                  {t('Quota Request')}
                </SelectItem>
                <SelectItem value='user'>{t('User')}</SelectItem>
                <SelectItem value='relay_request'>
                  {t('Relay Request')}
                </SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
          <Input
            type='number'
            value={props.targetId}
            onChange={(event) => {
              props.setTargetId(event.target.value)
              props.setPage(1)
            }}
            placeholder={t('Target ID')}
            className='w-36'
          />
          <Input
            value={props.actorUserId}
            onChange={(event) => {
              props.setActorUserId(event.target.value)
              props.setPage(1)
            }}
            placeholder={t('Actor User ID')}
            className='w-44'
          />
          <Input
            value={props.requestId}
            onChange={(event) => {
              props.setRequestId(event.target.value)
              props.setPage(1)
            }}
            placeholder={t('Request ID')}
            className='w-52'
          />
          <Input
            type='date'
            value={props.startDate}
            onChange={(event) => {
              props.setStartDate(event.target.value)
              props.setPage(1)
            }}
            className='w-40'
          />
          <Input
            type='date'
            value={props.endDate}
            onChange={(event) => {
              props.setEndDate(event.target.value)
              props.setPage(1)
            }}
            className='w-40'
          />
        </FilterBar>
        <QueryState
          query={props.query}
          empty={props.logs.length === 0}
          emptyTitle='No audit logs'
          emptyDescription='Audited changes will appear here.'
        >
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Time')}</TableHead>
                <TableHead>{t('Actor')}</TableHead>
                <TableHead>{t('Action')}</TableHead>
                <TableHead>{t('Target')}</TableHead>
                <TableHead>{t('Request ID')}</TableHead>
                <TableHead className='w-24 text-right'>
                  {t('Details')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {props.logs.map((log) => (
                <TableRow key={log.id}>
                  <TableCell>{formatDateTime(log.created_at)}</TableCell>
                  <TableCell>{log.actor_user_id || '-'}</TableCell>
                  <TableCell>
                    <Badge variant='outline'>{log.action}</Badge>
                  </TableCell>
                  <TableCell>
                    {log.target_type} #{log.target_id || '-'}
                  </TableCell>
                  <TableCell>
                    <span className='text-muted-foreground font-mono text-xs'>
                      {log.request_id || '-'}
                    </span>
                  </TableCell>
                  <TableCell className='text-right'>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => setSelectedLog(log)}
                    >
                      {t('View')}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <Pager
            page={props.page}
            pageSize={PAGE_SIZE}
            total={props.total}
            onPageChange={props.setPage}
          />
        </QueryState>
      </div>
      <AuditLogDetailDialog
        log={selectedLog}
        onRequestQuota={(initialValues) => {
          setSelectedLog(null)
          props.onRequestQuota(initialValues)
        }}
        onOpenChange={(open) => {
          if (!open) setSelectedLog(null)
        }}
      />
    </Panel>
  )
}

function DryRunObservationsPanel(props: {
  logs: EnterpriseAuditLog[]
  query: QueryResult<PageInfo<EnterpriseAuditLog>>
  onView: (log: EnterpriseAuditLog) => void
  onRequestQuota: (initialValues: QuotaRequestInitialValues) => void
}) {
  const { t } = useTranslation()

  return (
    <div className='bg-muted/20 rounded-lg border'>
      <div className='flex flex-wrap items-start justify-between gap-3 border-b px-3 py-2.5'>
        <div className='min-w-0'>
          <h4 className='text-sm font-semibold'>{t('Dry-run Observations')}</h4>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t(
              'Recent requests that would be blocked after hard-limit rollout.'
            )}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={props.query.refetch}
          disabled={props.query.isLoading}
        >
          <RefreshCcw className='size-4' />
          {t('Refresh')}
        </Button>
      </div>
      <div className='p-3'>
        <QueryState
          query={props.query}
          empty={props.logs.length === 0}
          emptyTitle='No dry-run observations'
          emptyDescription='Dry-run policy misses will appear here before hard-limit rollout.'
        >
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Time')}</TableHead>
                <TableHead>{t('Actor')}</TableHead>
                <TableHead>{t('Model')}</TableHead>
                <TableHead>{t('Policy')}</TableHead>
                <TableHead>{t('Usage')}</TableHead>
                <TableHead>{t('Suggested Limit')}</TableHead>
                <TableHead className='w-36 text-right'>
                  {t('Actions')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {props.logs.map((log) => {
                const payload = parseAuditObject(log.after_json)
                const metric = auditString(payload, 'metric') || '-'
                const policyId = auditNumber(payload, 'policy_id')
                const model = auditString(payload, 'model') || '-'
                const initialValues = quotaRequestInitialValuesFromAudit(log)
                const usage = `${formatAuditNumberValue(
                  payload,
                  'used_value'
                )} + ${formatAuditNumberValue(
                  payload,
                  'reserved_value'
                )} + ${formatAuditNumberValue(payload, 'requested_value')} / ${formatAuditNumberValue(
                  payload,
                  'limit_value'
                )}`

                return (
                  <TableRow key={log.id}>
                    <TableCell>{formatDateTime(log.created_at)}</TableCell>
                    <TableCell>{log.actor_user_id || '-'}</TableCell>
                    <TableCell>
                      <span className='font-mono text-xs'>{model}</span>
                    </TableCell>
                    <TableCell>
                      <div className='flex flex-wrap items-center gap-2'>
                        <Badge variant='outline'>{metric}</Badge>
                        <span className='text-muted-foreground font-mono text-xs'>
                          {policyId ? `#${policyId}` : '-'}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <span className='font-mono text-xs'>{usage}</span>
                    </TableCell>
                    <TableCell>
                      {formatAuditNumberValue(payload, 'suggested_limit_value')}
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex justify-end gap-1'>
                        <Button
                          type='button'
                          variant='ghost'
                          size='sm'
                          disabled={!initialValues.policyId}
                          onClick={() => props.onRequestQuota(initialValues)}
                        >
                          {t('Request')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() => props.onView(log)}
                        >
                          {t('View')}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </QueryState>
      </div>
    </div>
  )
}

function QueueAdmissionsPanel(props: {
  admissions: EnterpriseQueueAdmission[]
  query: QueryResult<PageInfo<EnterpriseQueueAdmission>>
  canManage: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const cancelMutation = useMutation({
    mutationFn: cancelEnterpriseQueueAdmission,
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Canceled'))
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
      queryClient.invalidateQueries({ queryKey: ['enterprise', 'audit-logs'] })
    },
  })

  return (
    <div className='bg-muted/20 rounded-lg border'>
      <div className='flex flex-wrap items-start justify-between gap-3 border-b px-3 py-2.5'>
        <div className='min-w-0'>
          <h4 className='text-sm font-semibold'>{t('Queue Admissions')}</h4>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t(
              'Recent enterprise queue lifecycle records, including queued, released, timeout, and canceled requests.'
            )}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={props.query.refetch}
          disabled={props.query.isLoading}
        >
          <RefreshCcw className='size-4' />
          {t('Refresh')}
        </Button>
      </div>
      <div className='p-3'>
        <QueryState
          query={props.query}
          empty={props.admissions.length === 0}
          emptyTitle='No queue admissions'
          emptyDescription='Queue action admissions will appear here after matching traffic.'
        >
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Time')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Request ID')}</TableHead>
                <TableHead>{t('Model')}</TableHead>
                <TableHead>{t('Policy')}</TableHead>
                <TableHead>{t('Scope')}</TableHead>
                <TableHead className='text-right'>{t('Wait')}</TableHead>
                <TableHead className='text-right'>{t('Run')}</TableHead>
                {props.canManage && (
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                )}
              </TableRow>
            </TableHeader>
            <TableBody>
              {props.admissions.map((admission) => (
                <TableRow key={admission.id}>
                  <TableCell>{formatDateTime(admission.created_at)}</TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        admission.status === 'timeout' ||
                        admission.status === 'canceled'
                          ? 'destructive'
                          : admission.status === 'released'
                            ? 'secondary'
                            : 'outline'
                      }
                    >
                      {admission.status || '-'}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <span className='text-muted-foreground font-mono text-xs'>
                      {admission.request_id || '-'}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className='font-mono text-xs'>
                      {admission.model_name || '-'}
                    </span>
                  </TableCell>
                  <TableCell>
                    {admission.policy_id ? `#${admission.policy_id}` : '-'}
                  </TableCell>
                  <TableCell>
                    <div className='text-muted-foreground flex flex-wrap gap-2 text-xs'>
                      <span>
                        {t('User')} #{admission.user_id || '-'}
                      </span>
                      <span>
                        {t('Org')} #{admission.org_unit_id || '-'}
                      </span>
                      <span>
                        {t('Project')} #{admission.project_id || '-'}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell className='text-right'>
                    <span className='font-mono text-xs'>
                      {formatDurationMs(admission.wait_ms)} /{' '}
                      {formatDurationMs(admission.timeout_ms)}
                    </span>
                  </TableCell>
                  <TableCell className='text-right'>
                    <span className='font-mono text-xs'>
                      {admission.run_ms
                        ? formatDurationMs(admission.run_ms)
                        : '-'}
                    </span>
                  </TableCell>
                  {props.canManage && (
                    <TableCell>
                      <div className='flex justify-end'>
                        {admission.status === 'queued' ? (
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon-sm'
                            disabled={cancelMutation.isPending}
                            onClick={() => cancelMutation.mutate(admission.id)}
                          >
                            <Ban className='size-3.5' />
                            <span className='sr-only'>{t('Cancel')}</span>
                          </Button>
                        ) : (
                          <span className='text-muted-foreground text-xs'>
                            -
                          </span>
                        )}
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </QueryState>
      </div>
    </div>
  )
}

function AuditLogDetailDialog(props: {
  log: EnterpriseAuditLog | null
  onRequestQuota: (initialValues: QuotaRequestInitialValues) => void
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const log = props.log

  return (
    <Dialog open={Boolean(log)} onOpenChange={props.onOpenChange}>
      <DialogContent className='flex max-h-[calc(100vh-2rem)] flex-col overflow-hidden sm:max-w-4xl'>
        <DialogHeader>
          <DialogTitle>{t('Audit Log Details')}</DialogTitle>
          <DialogDescription>
            {t(
              'Inspect the exact before and after payload recorded for this governance event.'
            )}
          </DialogDescription>
        </DialogHeader>
        {log && (
          <div className='min-h-0 space-y-4 overflow-y-auto pr-1'>
            <div className='grid gap-3 rounded-lg border p-3 text-sm sm:grid-cols-2 lg:grid-cols-4'>
              <AuditDetailField
                label='Time'
                value={formatDateTime(log.created_at)}
              />
              <AuditDetailField
                label='Actor'
                value={log.actor_user_id ? String(log.actor_user_id) : '-'}
              />
              <AuditDetailField label='Action' value={log.action} mono />
              <AuditDetailField
                label='Target'
                value={`${log.target_type} #${log.target_id || '-'}`}
                mono
              />
              <AuditDetailField
                label='Request ID'
                value={log.request_id || '-'}
                mono
                className='sm:col-span-2 lg:col-span-4'
              />
            </div>
            <AuditRejectExplanation
              log={log}
              onRequestQuota={props.onRequestQuota}
            />
            <div className='grid gap-4 lg:grid-cols-2'>
              <AuditJsonBlock title='Before' value={log.before_json} />
              <AuditJsonBlock title='After' value={log.after_json} />
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

function AuditRejectExplanation(props: {
  log: EnterpriseAuditLog
  onRequestQuota: (initialValues: QuotaRequestInitialValues) => void
}) {
  const { t } = useTranslation()
  const payload = parseAuditObject(props.log.after_json)
  const denyReason = auditString(payload, 'deny_reason')
  const denyType = auditString(payload, 'deny_type')
  const dryRun = auditBoolean(payload, 'dry_run')
  if (props.log.action === 'enterprise_governance.policy_action') {
    return <AuditPolicyActionExplanation payload={payload} />
  }
  const isGovernanceReject =
    props.log.action === 'enterprise_governance.dry_run_reject' ||
    props.log.action === 'enterprise_governance.hard_limit_reject'

  if (!isGovernanceReject && !denyReason && !denyType) return null

  const policyId =
    auditNumber(payload, 'policy_id') ?? auditNumber(payload, 'target_id')
  const initialValues = quotaRequestInitialValuesFromAudit(props.log)
  const periodStart = auditNumber(payload, 'period_start')
  const periodEnd = auditNumber(payload, 'period_end')
  const periodValue =
    periodStart || periodEnd
      ? `${formatDateTime(periodStart)} - ${formatDateTime(periodEnd)}`
      : '-'

  return (
    <div className='space-y-3 rounded-lg border p-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='min-w-0'>
          <div className='text-sm font-semibold'>
            {dryRun ? t('Dry-run would reject') : t('Hard-limit rejection')}
          </div>
          <div className='text-muted-foreground mt-1 text-xs'>
            {denyReason || t('No deny reason recorded')}
          </div>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          {denyType === 'quota_exceeded' && initialValues.policyId && (
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => props.onRequestQuota(initialValues)}
            >
              {t('Request Temporary Quota')}
            </Button>
          )}
          <Badge variant={dryRun ? 'secondary' : 'destructive'}>
            {denyType || t('Unknown')}
          </Badge>
        </div>
      </div>
      {denyType === 'quota_exceeded' ? (
        <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
          <AuditDetailField
            label='Policy'
            value={policyId ? `#${policyId}` : '-'}
            mono
          />
          <AuditDetailField
            label='Metric'
            value={auditString(payload, 'metric') || '-'}
            mono
          />
          <AuditDetailField
            label='Target'
            value={`${auditString(payload, 'target_type') || '-'} #${
              auditNumber(payload, 'target_id') ?? '-'
            }`}
            mono
          />
          <AuditDetailField label='Period' value={periodValue} />
          <AuditDetailField
            label='Limit'
            value={formatNumber(auditNumber(payload, 'limit_value'))}
          />
          <AuditDetailField
            label='Used'
            value={formatNumber(auditNumber(payload, 'used_value'))}
          />
          <AuditDetailField
            label='Reserved'
            value={formatNumber(auditNumber(payload, 'reserved_value'))}
          />
          <AuditDetailField
            label='Requested'
            value={formatNumber(auditNumber(payload, 'requested_value'))}
          />
        </div>
      ) : (
        <div className='grid gap-3 sm:grid-cols-2'>
          <AuditDetailField
            label='Policy IDs'
            value={
              auditList(payload, 'policy_ids') ||
              auditList(payload, 'matched_policy_ids') ||
              '-'
            }
            mono
          />
          <AuditDetailField
            label='Allowed Models'
            value={auditList(payload, 'allowed_models') || '-'}
            mono
          />
        </div>
      )}
    </div>
  )
}

function AuditPolicyActionExplanation(props: {
  payload: Record<string, unknown>
}) {
  const { t } = useTranslation()
  const actions = auditObjectList(props.payload, 'policy_actions')

  if (actions.length === 0) return null

  return (
    <div className='space-y-3 rounded-lg border p-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='min-w-0'>
          <div className='text-sm font-semibold'>
            {t('Policy action observed')}
          </div>
          <div className='text-muted-foreground mt-1 text-xs'>
            {t(auditString(props.payload, 'user_message_key'))}
          </div>
        </div>
        <Badge variant='secondary'>{t('Allowed')}</Badge>
      </div>
      <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
        {actions.map((action, index) => (
          <div
            key={`${auditString(action, 'action')}:${index}`}
            className='space-y-2 rounded-md border p-2'
          >
            <AuditDetailField
              label='Policy'
              value={`#${auditNumber(action, 'policy_id') ?? '-'}`}
              mono
            />
            <AuditDetailField
              label='Action'
              value={formatPolicyAction(auditString(action, 'action'))}
            />
            <AuditDetailField
              label='Trigger'
              value={auditString(action, 'trigger') || '-'}
              mono
            />
            <AuditDetailField
              label='Fallback Model'
              value={auditString(action, 'fallback_model') || '-'}
              mono
            />
          </div>
        ))}
      </div>
    </div>
  )
}

function AuditDetailField(props: {
  label: string
  value: string
  mono?: boolean
  className?: string
}) {
  const { t } = useTranslation()
  return (
    <div className={cn('min-w-0 space-y-1', props.className)}>
      <div className='text-muted-foreground text-xs'>{t(props.label)}</div>
      <div
        className={cn(
          'truncate text-sm font-medium',
          props.mono && 'font-mono text-xs'
        )}
      >
        {props.value}
      </div>
    </div>
  )
}

function AuditJsonBlock(props: { title: string; value: string }) {
  const { t } = useTranslation()
  return (
    <div className='min-w-0 overflow-hidden rounded-lg border'>
      <div className='bg-muted/40 border-b px-3 py-2 text-sm font-medium'>
        {t(props.title)}
      </div>
      <pre className='max-h-[420px] overflow-auto p-3 text-xs leading-relaxed'>
        <code>{formatAuditJson(props.value)}</code>
      </pre>
    </div>
  )
}

function notificationPreferenceKey(channel: string, eventType: string) {
  return `${channel}:${eventType}`
}

function preferencePayload(
  preference: EnterpriseNotificationPreference,
  scope: EnterpriseNotificationRecipientScope = preference.recipient_scope
): EnterpriseNotificationPreferencePayload {
  return {
    channel: preference.channel,
    event_type: preference.event_type,
    enabled: preference.enabled,
    recipient_scope: scope,
  }
}

function parseEmailList(value: string) {
  return value
    .split(/[\s,，;；]+/)
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean)
}

function NotificationsTab() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const preferencesQuery = useQuery({
    queryKey: ['enterprise', 'notification-preferences'],
    queryFn: getEnterpriseNotificationPreferences,
  })
  const preferences = preferencesQuery.data?.data ?? []
  const byKey = useMemo(() => {
    const result = new Map<string, EnterpriseNotificationPreference>()
    preferences.forEach((preference) => {
      result.set(
        notificationPreferenceKey(preference.channel, preference.event_type),
        preference
      )
    })
    return result
  }, [preferences])
  const mutation = useMutation({
    mutationFn: updateEnterpriseNotificationPreference,
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Saved'))
      queryClient.invalidateQueries({
        queryKey: ['enterprise', 'notification-preferences'],
      })
      queryClient.invalidateQueries({ queryKey: ['enterprise', 'audit-logs'] })
    },
  })

  return (
    <Panel
      title='Notifications'
      description='Control external approval notifications. In-app notifications stay enabled for auditability.'
    >
      <QueryState
        query={{
          data: preferencesQuery.data,
          isLoading: preferencesQuery.isLoading,
          isError: preferencesQuery.isError,
          error: preferencesQuery.error,
          refetch: preferencesQuery.refetch,
        }}
        empty={false}
      >
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Event')}</TableHead>
              <TableHead>{t('Email')}</TableHead>
              <TableHead>{t('Email Recipients')}</TableHead>
              <TableHead>{t('Webhook')}</TableHead>
              <TableHead>{t('Updated')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {WEBHOOK_EVENT_TYPES.map((eventType) => {
              const emailPreference = byKey.get(
                notificationPreferenceKey('email', eventType)
              )
              const webhookPreference = byKey.get(
                notificationPreferenceKey('webhook', eventType)
              )
              if (!emailPreference || !webhookPreference) return null
              return (
                <NotificationPreferenceRow
                  key={eventType}
                  eventType={eventType}
                  emailPreference={emailPreference}
                  webhookPreference={webhookPreference}
                  isSaving={mutation.isPending}
                  onSave={(payload) => mutation.mutate(payload)}
                />
              )
            })}
          </TableBody>
        </Table>
      </QueryState>
    </Panel>
  )
}

function NotificationPreferenceRow(props: {
  eventType: string
  emailPreference: EnterpriseNotificationPreference
  webhookPreference: EnterpriseNotificationPreference
  isSaving: boolean
  onSave: (payload: EnterpriseNotificationPreferencePayload) => void
}) {
  const { t } = useTranslation()
  const [emailScope, setEmailScope] = useState(() =>
    stringifyStringList(props.emailPreference.recipient_scope.explicit_emails)
  )
  const [applicant, setApplicant] = useState(
    props.emailPreference.recipient_scope.applicant
  )
  const [enterpriseAdmins, setEnterpriseAdmins] = useState(
    props.emailPreference.recipient_scope.enterprise_admins
  )
  const emailScopePayload = (): EnterpriseNotificationRecipientScope => ({
    applicant,
    enterprise_admins: enterpriseAdmins,
    explicit_emails: parseEmailList(emailScope),
  })

  return (
    <TableRow>
      <TableCell>
        <div className='font-mono text-xs'>{props.eventType}</div>
      </TableCell>
      <TableCell>
        <label className='flex items-center gap-2 text-sm'>
          <Checkbox
            checked={props.emailPreference.enabled}
            disabled={props.isSaving}
            onCheckedChange={(checked) =>
              props.onSave({
                ...preferencePayload(
                  props.emailPreference,
                  emailScopePayload()
                ),
                enabled: checked === true,
              })
            }
          />
          <span>
            {t(props.emailPreference.enabled ? 'Enabled' : 'Disabled')}
          </span>
        </label>
      </TableCell>
      <TableCell>
        <div className='grid min-w-80 gap-2'>
          <div className='flex flex-wrap gap-3 text-xs'>
            <label className='flex items-center gap-2'>
              <Checkbox
                checked={applicant}
                disabled={props.isSaving}
                onCheckedChange={(checked) => {
                  const next = checked === true
                  setApplicant(next)
                  props.onSave({
                    ...preferencePayload(props.emailPreference, {
                      ...emailScopePayload(),
                      applicant: next,
                    }),
                  })
                }}
              />
              <span>{t('Applicant')}</span>
            </label>
            <label className='flex items-center gap-2'>
              <Checkbox
                checked={enterpriseAdmins}
                disabled={props.isSaving}
                onCheckedChange={(checked) => {
                  const next = checked === true
                  setEnterpriseAdmins(next)
                  props.onSave({
                    ...preferencePayload(props.emailPreference, {
                      ...emailScopePayload(),
                      enterprise_admins: next,
                    }),
                  })
                }}
              />
              <span>{t('Enterprise Admins')}</span>
            </label>
          </div>
          <div className='flex gap-2'>
            <Input
              value={emailScope}
              onChange={(event) => setEmailScope(event.target.value)}
              placeholder={t('ops@example.com, owner@example.com')}
              className='h-8 font-mono text-xs'
            />
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={props.isSaving}
              onClick={() =>
                props.onSave(
                  preferencePayload(props.emailPreference, emailScopePayload())
                )
              }
            >
              <Save className='size-3.5' />
              {t('Save')}
            </Button>
          </div>
        </div>
      </TableCell>
      <TableCell>
        <label className='flex items-center gap-2 text-sm'>
          <Checkbox
            checked={props.webhookPreference.enabled}
            disabled={props.isSaving}
            onCheckedChange={(checked) =>
              props.onSave({
                ...preferencePayload(props.webhookPreference),
                enabled: checked === true,
              })
            }
          />
          <span>
            {t(props.webhookPreference.enabled ? 'Enabled' : 'Disabled')}
          </span>
        </label>
      </TableCell>
      <TableCell>
        {formatDateTime(
          Math.max(
            props.emailPreference.updated_at || 0,
            props.webhookPreference.updated_at || 0
          )
        )}
      </TableCell>
    </TableRow>
  )
}

function WebhooksTab() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingWebhook, setEditingWebhook] =
    useState<EnterpriseWebhook | null>(null)
  const [confirmWebhook, setConfirmWebhook] =
    useState<EnterpriseWebhook | null>(null)
  const [testResults, setTestResults] = useState<
    Record<number, EnterpriseWebhookTestResult>
  >({})

  const webhooksQuery = useQuery({
    queryKey: ['enterprise', 'webhooks'],
    queryFn: getEnterpriseWebhooks,
  })
  const webhooks = webhooksQuery.data?.data ?? []

  const disableMutation = useMutation({
    mutationFn: disableEnterpriseWebhook,
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Disabled'))
      setConfirmWebhook(null)
      queryClient.invalidateQueries({ queryKey: ['enterprise', 'webhooks'] })
      queryClient.invalidateQueries({ queryKey: ['enterprise', 'audit-logs'] })
    },
  })
  const testMutation = useMutation({
    mutationFn: testEnterpriseWebhook,
    onSuccess: (response, id) => {
      const result = response.data
      if (!response.success || !result) return
      setTestResults((current) => ({ ...current, [id]: result }))
      toast[result.success ? 'success' : 'error'](
        t(result.success ? 'Webhook test sent' : 'Webhook test failed')
      )
      queryClient.invalidateQueries({ queryKey: ['enterprise', 'audit-logs'] })
    },
  })

  return (
    <>
      <Panel
        title='Webhooks'
        description='Signed outbound approval events for enterprise integrations.'
        actions={
          <Button
            size='sm'
            onClick={() => {
              setEditingWebhook(null)
              setDialogOpen(true)
            }}
          >
            <Plus className='size-3.5' />
            {t('Webhook')}
          </Button>
        }
      >
        <QueryState
          query={{
            data: webhooksQuery.data,
            isLoading: webhooksQuery.isLoading,
            isError: webhooksQuery.isError,
            error: webhooksQuery.error,
            refetch: webhooksQuery.refetch,
          }}
          empty={webhooks.length === 0}
          emptyTitle='No webhooks'
          emptyDescription='Create a webhook to receive signed approval lifecycle events.'
        >
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Name')}</TableHead>
                <TableHead>{t('Endpoint')}</TableHead>
                <TableHead>{t('Events')}</TableHead>
                <TableHead>{t('Secret')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Last Test')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {webhooks.map((webhook) => {
                const testResult = testResults[webhook.id]
                return (
                  <TableRow key={webhook.id}>
                    <TableCell>
                      <div className='min-w-0'>
                        <div className='truncate font-medium'>
                          {webhook.name}
                        </div>
                        <div className='text-muted-foreground truncate text-xs'>
                          #{webhook.id} · {formatDateTime(webhook.updated_at)}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <span className='block max-w-80 truncate font-mono text-xs'>
                        {webhook.url}
                      </span>
                    </TableCell>
                    <TableCell>
                      <div className='flex max-w-72 flex-wrap gap-1'>
                        {webhook.event_types.map((eventType) => (
                          <Badge key={eventType} variant='secondary'>
                            {eventType}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={webhook.has_secret ? 'outline' : 'secondary'}
                      >
                        {t(webhook.has_secret ? 'Set' : 'Not set')}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <StatusBadge status={webhook.status} />
                    </TableCell>
                    <TableCell>
                      {testResult ? (
                        <div className='min-w-0 text-xs'>
                          <div
                            className={cn(
                              'font-medium',
                              testResult.success
                                ? 'text-emerald-700'
                                : 'text-destructive'
                            )}
                          >
                            {testResult.success ? t('Success') : t('Failed')}{' '}
                            {testResult.status_code || '-'}
                          </div>
                          <div className='text-muted-foreground truncate'>
                            {formatDurationMs(testResult.duration_ms)}
                            {testResult.error ? ` · ${testResult.error}` : ''}
                          </div>
                        </div>
                      ) : (
                        <span className='text-muted-foreground text-xs'>-</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className='flex justify-end gap-1'>
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          disabled={testMutation.isPending}
                          onClick={() => testMutation.mutate(webhook.id)}
                        >
                          <Send className='size-3.5' />
                          <span className='sr-only'>{t('Test')}</span>
                        </Button>
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          onClick={() => {
                            setEditingWebhook(webhook)
                            setDialogOpen(true)
                          }}
                        >
                          <Pencil className='size-3.5' />
                          <span className='sr-only'>{t('Edit')}</span>
                        </Button>
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          disabled={webhook.status === DISABLED_STATUS}
                          onClick={() => setConfirmWebhook(webhook)}
                        >
                          <Ban className='size-3.5' />
                          <span className='sr-only'>{t('Disable')}</span>
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </QueryState>
      </Panel>
      {dialogOpen ? (
        <WebhookDialog
          key={`webhook:${editingWebhook?.id ?? 'new'}`}
          open={dialogOpen}
          webhook={editingWebhook}
          onOpenChange={setDialogOpen}
        />
      ) : null}
      <AlertDialog
        open={Boolean(confirmWebhook)}
        onOpenChange={(open) => {
          if (!open) setConfirmWebhook(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Disable webhook?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This stops new webhook deliveries for this endpoint. Existing outbox rows remain available for audit and retry.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {confirmWebhook && (
            <div className='rounded-lg border p-3 text-sm'>
              <div className='font-medium'>{confirmWebhook.name}</div>
              <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
                {confirmWebhook.url}
              </div>
            </div>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={disableMutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={disableMutation.isPending || !confirmWebhook}
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
              onClick={() => {
                if (confirmWebhook) disableMutation.mutate(confirmWebhook.id)
              }}
            >
              {t('Disable')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

type WebhookFormState = {
  name: string
  url: string
  secret: string
  event_types: string[]
  status: string
}

function WebhookDialog(props: {
  open: boolean
  webhook: EnterpriseWebhook | null
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<WebhookFormState>(() => ({
    name: props.webhook?.name ?? '',
    url: props.webhook?.url ?? '',
    secret: '',
    event_types: props.webhook?.event_types ?? WEBHOOK_EVENT_TYPES,
    status: String(props.webhook?.status ?? ENABLED_STATUS),
  }))
  const editing = Boolean(props.webhook)

  const payload = (): EnterpriseWebhookPayload => {
    const nextPayload: EnterpriseWebhookPayload = {
      name: form.name.trim(),
      url: form.url.trim(),
      event_types: form.event_types,
      status: Number(form.status),
    }
    if (form.secret.trim()) nextPayload.secret = form.secret.trim()
    return nextPayload
  }

  const mutation = useMutation({
    mutationFn: () =>
      props.webhook
        ? updateEnterpriseWebhook(props.webhook.id, payload())
        : createEnterpriseWebhook(payload()),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Saved'))
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['enterprise', 'webhooks'] })
      queryClient.invalidateQueries({ queryKey: ['enterprise', 'audit-logs'] })
    },
  })

  const toggleEventType = (eventType: string, checked: boolean) => {
    setForm((current) => ({
      ...current,
      event_types: checked
        ? Array.from(new Set([...current.event_types, eventType]))
        : current.event_types.filter((item) => item !== eventType),
    }))
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>
            {t(editing ? 'Edit Webhook' : 'Create Webhook')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Configure a signed endpoint for approval lifecycle notifications.'
            )}
          </DialogDescription>
        </DialogHeader>
        <form
          className='grid gap-3'
          onSubmit={(event) => {
            event.preventDefault()
            mutation.mutate()
          }}
        >
          <div className='grid gap-3 sm:grid-cols-2'>
            <Field label='Name'>
              <Input
                value={form.name}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    name: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Status'>
              <Select
                value={form.status}
                onValueChange={(value) =>
                  setForm((current) => ({
                    ...current,
                    status: normalizeSelectValue(value),
                  }))
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='1'>{t('Enabled')}</SelectItem>
                    <SelectItem value='2'>{t('Disabled')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
          </div>
          <Field label='Endpoint URL'>
            <Input
              value={form.url}
              onChange={(event) =>
                setForm((current) => ({ ...current, url: event.target.value }))
              }
              placeholder='https://example.com/webhooks/data-proxy'
            />
          </Field>
          <Field label={editing ? 'Secret (leave blank to keep)' : 'Secret'}>
            <Input
              type='password'
              value={form.secret}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  secret: event.target.value,
                }))
              }
            />
          </Field>
          <div className='grid gap-2'>
            <Label>{t('Subscribed Events')}</Label>
            <div className='grid gap-2 rounded-lg border p-3 sm:grid-cols-2'>
              {WEBHOOK_EVENT_TYPES.map((eventType) => (
                <label
                  key={eventType}
                  className='flex items-center gap-2 text-sm'
                >
                  <Checkbox
                    checked={form.event_types.includes(eventType)}
                    onCheckedChange={(checked) =>
                      toggleEventType(eventType, checked === true)
                    }
                  />
                  <span className='font-mono text-xs'>{eventType}</span>
                </label>
              ))}
            </div>
          </div>
          <DialogFooter>
            <Button type='submit' disabled={mutation.isPending}>
              {t('Save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function DeliveriesTab() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [channel, setChannel] = useState('')
  const [status, setStatus] = useState('')
  const [eventType, setEventType] = useState('')
  const [webhookId, setWebhookId] = useState('')
  const [targetId, setTargetId] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const targetIdValue = targetId ? Number(targetId) : undefined
  const webhookIdValue = webhookId ? Number(webhookId) : undefined
  const startTime = startDate ? startOfDayUnix(startDate) : undefined
  const endTime = endDate ? endOfDayUnix(endDate) : undefined

  const outboxQuery = useQuery({
    queryKey: [
      'enterprise',
      'notification-outbox',
      page,
      channel,
      status,
      eventType,
      webhookIdValue,
      targetIdValue,
      startTime,
      endTime,
    ],
    queryFn: () =>
      getEnterpriseNotificationOutbox({
        p: page,
        page_size: PAGE_SIZE,
        channel,
        status,
        event_type: eventType,
        webhook_id: webhookIdValue,
        target_id: targetIdValue,
        start_time: startTime,
        end_time: endTime,
      }),
  })
  const metricsQuery = useQuery({
    queryKey: ['enterprise', 'notification-outbox', 'worker-metrics'],
    queryFn: getEnterpriseNotificationOutboxWorkerMetrics,
  })
  const webhooksQuery = useQuery({
    queryKey: ['enterprise', 'webhooks'],
    queryFn: getEnterpriseWebhooks,
  })
  const deliveries = getPageItems(outboxQuery.data)
  const webhooks = webhooksQuery.data?.data ?? []
  const metrics = metricsQuery.data?.data

  const retryMutation = useMutation({
    mutationFn: retryEnterpriseNotificationOutbox,
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Queued for retry'))
      queryClient.invalidateQueries({
        queryKey: ['enterprise', 'notification-outbox'],
      })
      queryClient.invalidateQueries({ queryKey: ['enterprise', 'audit-logs'] })
    },
  })
  const resetPage = () => setPage(1)

  return (
    <div className='space-y-3'>
      <DeliveryMetricsPanel metrics={metrics} query={metricsQuery} />
      <Panel
        title='Deliveries'
        description='Recent notification outbox rows across in-app, email, and webhook channels.'
        actions={
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => {
              void outboxQuery.refetch()
              void metricsQuery.refetch()
            }}
          >
            <RefreshCcw className='size-3.5' />
            {t('Refresh')}
          </Button>
        }
      >
        <div className='space-y-3'>
          <FilterBar>
            <Select
              value={channel || ALL_VALUE}
              onValueChange={(value) => {
                setChannel(normalizeOptionalSelectValue(value))
                resetPage()
              }}
            >
              <SelectTrigger className='w-36'>
                <SelectValue placeholder={t('Channel')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL_VALUE}>{t('All Channels')}</SelectItem>
                  <SelectItem value='in_app'>{t('In-app')}</SelectItem>
                  <SelectItem value='email'>{t('Email')}</SelectItem>
                  <SelectItem value='webhook'>{t('Webhook')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
            <Select
              value={status || ALL_VALUE}
              onValueChange={(value) => {
                setStatus(normalizeOptionalSelectValue(value))
                resetPage()
              }}
            >
              <SelectTrigger className='w-44'>
                <SelectValue placeholder={t('Status')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL_VALUE}>{t('All Statuses')}</SelectItem>
                  <SelectItem value='pending'>{t('Pending')}</SelectItem>
                  <SelectItem value='processing'>{t('Processing')}</SelectItem>
                  <SelectItem value='sent'>{t('Sent')}</SelectItem>
                  <SelectItem value='failed'>{t('Failed')}</SelectItem>
                  <SelectItem value='permanent_failed'>
                    {t('Permanent Failed')}
                  </SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
            <Select
              value={eventType || ALL_VALUE}
              onValueChange={(value) => {
                setEventType(normalizeOptionalSelectValue(value))
                resetPage()
              }}
            >
              <SelectTrigger className='w-56'>
                <SelectValue placeholder={t('Event Type')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL_VALUE}>{t('All Events')}</SelectItem>
                  {WEBHOOK_EVENT_TYPES.map((item) => (
                    <SelectItem key={item} value={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <Select
              value={webhookId || ALL_VALUE}
              onValueChange={(value) => {
                setWebhookId(normalizeOptionalSelectValue(value))
                resetPage()
              }}
            >
              <SelectTrigger className='w-48'>
                <SelectValue placeholder={t('Webhook')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL_VALUE}>{t('All Webhooks')}</SelectItem>
                  {webhooks.map((webhook) => (
                    <SelectItem key={webhook.id} value={String(webhook.id)}>
                      {webhook.name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <Input
              type='number'
              value={targetId}
              onChange={(event) => {
                setTargetId(event.target.value)
                resetPage()
              }}
              placeholder={t('Target ID')}
              className='w-32'
            />
            <Input
              type='date'
              value={startDate}
              onChange={(event) => {
                setStartDate(event.target.value)
                resetPage()
              }}
              className='w-40'
            />
            <Input
              type='date'
              value={endDate}
              onChange={(event) => {
                setEndDate(event.target.value)
                resetPage()
              }}
              className='w-40'
            />
          </FilterBar>
          <QueryState
            query={{
              data: outboxQuery.data,
              isLoading: outboxQuery.isLoading,
              isError: outboxQuery.isError,
              error: outboxQuery.error,
              refetch: outboxQuery.refetch,
            }}
            empty={deliveries.length === 0}
            emptyTitle='No deliveries'
            emptyDescription='Notification outbox rows will appear here after approval events are generated.'
          >
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Event')}</TableHead>
                  <TableHead>{t('Channel')}</TableHead>
                  <TableHead>{t('Recipient')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Retry')}</TableHead>
                  <TableHead>{t('Next Retry')}</TableHead>
                  <TableHead>{t('Last Error')}</TableHead>
                  <TableHead>{t('Created')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {deliveries.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell>
                      <div className='min-w-0'>
                        <div className='truncate font-mono text-xs'>
                          {row.event_type}
                        </div>
                        <div className='text-muted-foreground truncate text-xs'>
                          {row.target_type} #{row.target_id || '-'}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>{t(formatChannel(row.channel))}</TableCell>
                    <TableCell>
                      <span className='block max-w-48 truncate font-mono text-xs'>
                        {formatRecipient(row)}
                      </span>
                    </TableCell>
                    <TableCell>
                      <OutboxStatusBadge status={row.status} />
                    </TableCell>
                    <TableCell>{formatNumber(row.retry_count)}</TableCell>
                    <TableCell>{formatDateTime(row.next_retry_at)}</TableCell>
                    <TableCell>
                      <span className='block max-w-72 truncate text-xs'>
                        {row.last_error || '-'}
                      </span>
                    </TableCell>
                    <TableCell>{formatDateTime(row.created_at)}</TableCell>
                    <TableCell>
                      <div className='flex justify-end'>
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          disabled={
                            !isRetryableOutboxStatus(row.status) ||
                            retryMutation.isPending
                          }
                          onClick={() => retryMutation.mutate(row.id)}
                        >
                          <RefreshCcw className='size-3.5' />
                          <span className='sr-only'>{t('Retry')}</span>
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <Pager
              page={page}
              pageSize={PAGE_SIZE}
              total={getPageTotal(outboxQuery.data)}
              onPageChange={setPage}
            />
          </QueryState>
        </div>
      </Panel>
    </div>
  )
}

function DeliveryMetricsPanel(props: {
  metrics?: EnterpriseNotificationOutboxWorkerMetrics
  query: QueryResult<EnterpriseNotificationOutboxWorkerMetrics>
}) {
  const metrics = props.metrics

  return (
    <QueryState query={props.query} empty={false}>
      <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-5'>
        <StatCell
          icon={Activity}
          label='Last Claimed'
          value={formatNumber(metrics?.last_run.claimed)}
          detail={formatDateTime(metrics?.last_run.started_at)}
        />
        <StatCell
          icon={Check}
          label='Last Sent'
          value={formatNumber(metrics?.last_run.sent)}
          detail={`Total ${formatNumber(metrics?.total_sent)}`}
        />
        <StatCell
          icon={Mail}
          label='Last Failed'
          value={formatNumber(metrics?.last_run.failed)}
          detail={`Total ${formatNumber(metrics?.total_failed)}`}
        />
        <StatCell
          icon={Ban}
          label='Permanent Failed'
          value={formatNumber(metrics?.last_run.permanent_failed)}
          detail={`Total ${formatNumber(metrics?.total_permanent_failed)}`}
        />
        <StatCell
          icon={TimerReset}
          label='Duration'
          value={formatDurationMs(metrics?.last_run.duration_ms)}
          detail={`Runs ${formatNumber(metrics?.total_runs)}`}
        />
      </div>
    </QueryState>
  )
}

type EnterpriseFormState = {
  name: string
  timezone: string
  status: string
  anomaly_enabled: boolean
  anomaly_current_window_seconds: string
  anomaly_baseline_window_seconds: string
  anomaly_cooldown_seconds: string
  anomaly_min_current_requests: string
  anomaly_min_baseline_requests: string
  anomaly_request_spike_ratio: string
  anomaly_min_current_quota: string
  anomaly_min_baseline_quota: string
  anomaly_cost_spike_ratio: string
  anomaly_min_failure_requests: string
  anomaly_min_failures: string
  anomaly_failure_rate: string
}

function buildEnterpriseFormState(
  enterprise?: Enterprise
): EnterpriseFormState {
  const anomalyConfig = enterpriseAnomalyConfig(
    enterprise?.anomaly_throttle_config
  )
  return {
    name: enterprise?.name ?? '',
    timezone: enterprise?.timezone ?? 'Asia/Shanghai',
    status: String(enterprise?.status ?? ENABLED_STATUS),
    anomaly_enabled: anomalyConfig.enabled,
    anomaly_current_window_seconds: String(
      anomalyConfig.current_window_seconds
    ),
    anomaly_baseline_window_seconds: String(
      anomalyConfig.baseline_window_seconds
    ),
    anomaly_cooldown_seconds: String(anomalyConfig.cooldown_seconds),
    anomaly_min_current_requests: String(anomalyConfig.min_current_requests),
    anomaly_min_baseline_requests: String(anomalyConfig.min_baseline_requests),
    anomaly_request_spike_ratio: String(anomalyConfig.request_spike_ratio),
    anomaly_min_current_quota: String(anomalyConfig.min_current_quota),
    anomaly_min_baseline_quota: String(anomalyConfig.min_baseline_quota),
    anomaly_cost_spike_ratio: String(anomalyConfig.cost_spike_ratio),
    anomaly_min_failure_requests: String(anomalyConfig.min_failure_requests),
    anomaly_min_failures: String(anomalyConfig.min_failures),
    anomaly_failure_rate: String(anomalyConfig.failure_rate),
  }
}

function EnterpriseDialog(props: {
  open: boolean
  enterprise?: Enterprise
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const anomalyConfig = enterpriseAnomalyConfig(
    props.enterprise?.anomaly_throttle_config
  )
  const [form, setForm] = useState<EnterpriseFormState>(() =>
    buildEnterpriseFormState(props.enterprise)
  )

  useEffect(() => {
    if (!props.open) return
    setForm(buildEnterpriseFormState(props.enterprise))
  }, [props.open, props.enterprise])

  const mutation = useMutation({
    mutationFn: () =>
      updateEnterpriseCurrent({
        name: form.name.trim(),
        timezone: form.timezone.trim(),
        status: Number(form.status),
        anomaly_throttle_config: {
          enabled: form.anomaly_enabled,
          current_window_seconds: positiveIntegerInput(
            form.anomaly_current_window_seconds,
            anomalyConfig.current_window_seconds
          ),
          baseline_window_seconds: positiveIntegerInput(
            form.anomaly_baseline_window_seconds,
            anomalyConfig.baseline_window_seconds
          ),
          cooldown_seconds: positiveIntegerInput(
            form.anomaly_cooldown_seconds,
            anomalyConfig.cooldown_seconds
          ),
          min_current_requests: positiveIntegerInput(
            form.anomaly_min_current_requests,
            anomalyConfig.min_current_requests
          ),
          min_baseline_requests: positiveIntegerInput(
            form.anomaly_min_baseline_requests,
            anomalyConfig.min_baseline_requests
          ),
          request_spike_ratio: positiveNumberInput(
            form.anomaly_request_spike_ratio,
            anomalyConfig.request_spike_ratio
          ),
          min_current_quota: positiveIntegerInput(
            form.anomaly_min_current_quota,
            anomalyConfig.min_current_quota
          ),
          min_baseline_quota: positiveIntegerInput(
            form.anomaly_min_baseline_quota,
            anomalyConfig.min_baseline_quota
          ),
          cost_spike_ratio: positiveNumberInput(
            form.anomaly_cost_spike_ratio,
            anomalyConfig.cost_spike_ratio
          ),
          min_failure_requests: positiveIntegerInput(
            form.anomaly_min_failure_requests,
            anomalyConfig.min_failure_requests
          ),
          min_failures: positiveIntegerInput(
            form.anomaly_min_failures,
            anomalyConfig.min_failures
          ),
          failure_rate: rateNumberInput(
            form.anomaly_failure_rate,
            anomalyConfig.failure_rate
          ),
        },
      }),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Saved'))
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Enterprise Settings')}</DialogTitle>
          <DialogDescription>
            {t('Configure the default enterprise governance scope.')}
          </DialogDescription>
        </DialogHeader>
        <form
          className='space-y-3'
          onSubmit={(event) => {
            event.preventDefault()
            mutation.mutate()
          }}
        >
          <Field label='Name'>
            <Input
              value={form.name}
              onChange={(event) =>
                setForm((current) => ({ ...current, name: event.target.value }))
              }
            />
          </Field>
          <Field label='Timezone'>
            <Input
              value={form.timezone}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  timezone: event.target.value,
                }))
              }
            />
          </Field>
          <Field label='Status'>
            <Select
              value={form.status}
              onValueChange={(value) =>
                setForm((current) => ({
                  ...current,
                  status: normalizeSelectValue(value),
                }))
              }
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value='1'>{t('Enabled')}</SelectItem>
                  <SelectItem value='2'>{t('Disabled')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <Field label='Anomaly Throttle'>
            <label className='flex h-10 items-center gap-2 rounded-md border px-3 text-sm'>
              <Checkbox
                checked={form.anomaly_enabled}
                onCheckedChange={(checked) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_enabled: checked === true,
                  }))
                }
              />
              <span>{t('Enabled')}</span>
            </label>
          </Field>
          <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
            <Field label='Current Window Seconds'>
              <Input
                type='number'
                min={1}
                value={form.anomaly_current_window_seconds}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_current_window_seconds: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Baseline Window Seconds'>
              <Input
                type='number'
                min={1}
                value={form.anomaly_baseline_window_seconds}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_baseline_window_seconds: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Cooldown Seconds'>
              <Input
                type='number'
                min={1}
                value={form.anomaly_cooldown_seconds}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_cooldown_seconds: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Request Spike Ratio'>
              <Input
                type='number'
                min={0.1}
                step={0.1}
                value={form.anomaly_request_spike_ratio}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_request_spike_ratio: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Min Current Requests'>
              <Input
                type='number'
                min={1}
                value={form.anomaly_min_current_requests}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_min_current_requests: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Min Baseline Requests'>
              <Input
                type='number'
                min={1}
                value={form.anomaly_min_baseline_requests}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_min_baseline_requests: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Cost Spike Ratio'>
              <Input
                type='number'
                min={0.1}
                step={0.1}
                value={form.anomaly_cost_spike_ratio}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_cost_spike_ratio: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Min Current Quota'>
              <Input
                type='number'
                min={1}
                value={form.anomaly_min_current_quota}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_min_current_quota: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Min Baseline Quota'>
              <Input
                type='number'
                min={1}
                value={form.anomaly_min_baseline_quota}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_min_baseline_quota: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Failure Rate'>
              <Input
                type='number'
                min={0.01}
                max={1}
                step={0.01}
                value={form.anomaly_failure_rate}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_failure_rate: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Min Failure Requests'>
              <Input
                type='number'
                min={1}
                value={form.anomaly_min_failure_requests}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_min_failure_requests: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Min Failures'>
              <Input
                type='number'
                min={1}
                value={form.anomaly_min_failures}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    anomaly_min_failures: event.target.value,
                  }))
                }
              />
            </Field>
          </div>
          <DialogFooter>
            <Button type='submit' disabled={mutation.isPending}>
              {t('Save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

type OrgUnitFormState = {
  parent_id: string
  name: string
  slug: string
  description: string
  status: string
  sort: string
}

function OrgUnitDialog(props: {
  open: boolean
  unit?: EnterpriseOrgUnit | null
  orgUnits: EnterpriseOrgUnit[]
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<OrgUnitFormState>(() => ({
    parent_id: props.unit?.parent_id ? String(props.unit.parent_id) : '',
    name: props.unit?.name ?? '',
    slug: props.unit?.slug ?? '',
    description: props.unit?.description ?? '',
    status: String(props.unit?.status ?? ENABLED_STATUS),
    sort: String(props.unit?.sort ?? 0),
  }))
  const editing = Boolean(props.unit)

  const parentOptions = props.orgUnits
    .filter(
      (unit) => unit.id !== props.unit?.id && unit.status === ENABLED_STATUS
    )
    .map((unit) => ({
      value: String(unit.id),
      label: `${'  '.repeat(unit.depth)}${unit.name}`,
    }))

  const payload = (): EnterpriseOrgUnitPayload => ({
    parent_id: form.parent_id ? Number(form.parent_id) : 0,
    name: form.name.trim(),
    slug: (form.slug.trim() || slugify(form.name)).trim(),
    description: form.description,
    status: Number(form.status),
    sort: Number(form.sort || 0),
  })

  const mutation = useMutation({
    mutationFn: () =>
      props.unit
        ? updateEnterpriseOrgUnit(props.unit.id, payload())
        : createEnterpriseOrgUnit(payload()),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Saved'))
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>
            {t(editing ? 'Edit Org Unit' : 'Create Org Unit')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Org units provide the primary allocation boundary for members.'
            )}
          </DialogDescription>
        </DialogHeader>
        <form
          className='grid gap-3'
          onSubmit={(event) => {
            event.preventDefault()
            mutation.mutate()
          }}
        >
          <Field label='Name'>
            <Input
              value={form.name}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  name: event.target.value,
                  slug: current.slug || slugify(event.target.value),
                }))
              }
            />
          </Field>
          <Field label='Slug'>
            <Input
              value={form.slug}
              onChange={(event) =>
                setForm((current) => ({ ...current, slug: event.target.value }))
              }
            />
          </Field>
          <Field label='Parent'>
            <Select
              value={form.parent_id || ROOT_VALUE}
              onValueChange={(value) =>
                setForm((current) => ({
                  ...current,
                  parent_id: normalizeOptionalSelectValue(value, ROOT_VALUE),
                }))
              }
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder={t('Root')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ROOT_VALUE}>{t('Root')}</SelectItem>
                  {parentOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <div className='grid gap-3 sm:grid-cols-2'>
            <Field label='Status'>
              <Select
                value={form.status}
                onValueChange={(value) =>
                  setForm((current) => ({
                    ...current,
                    status: normalizeSelectValue(value),
                  }))
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='1'>{t('Enabled')}</SelectItem>
                    <SelectItem value='2'>{t('Disabled')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field label='Sort'>
              <Input
                type='number'
                value={form.sort}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    sort: event.target.value,
                  }))
                }
              />
            </Field>
          </div>
          <Field label='Description'>
            <Textarea
              value={form.description}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  description: event.target.value,
                }))
              }
            />
          </Field>
          <DialogFooter>
            <Button type='submit' disabled={mutation.isPending}>
              {t('Save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

type PolicyGroupFormState = {
  name: string
  slug: string
  description: string
  shared_org_unit_ids: string[]
  shared_org_unit_roles: Record<string, EnterprisePolicyGroupShareRole | string>
  shared_expires_at: string
  status: string
}

function PolicyGroupDialog(props: {
  open: boolean
  group?: EnterprisePolicyGroup | null
  orgUnits: EnterpriseOrgUnit[]
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const orgOptions = buildOrgOptions(props.orgUnits)
  const [form, setForm] = useState<PolicyGroupFormState>(() => ({
    name: props.group?.name ?? '',
    slug: props.group?.slug ?? '',
    description: props.group?.description ?? '',
    shared_org_unit_ids: (props.group?.shared_org_unit_ids ?? []).map(String),
    shared_org_unit_roles: props.group?.shared_org_unit_roles ?? {},
    shared_expires_at: dateInputValueFromUnix(props.group?.shared_expires_at),
    status: String(props.group?.status ?? ENABLED_STATUS),
  }))
  const editing = Boolean(props.group)

  const payload = (): EnterprisePolicyGroupPayload => ({
    name: form.name.trim(),
    slug: (form.slug.trim() || slugify(form.name)).trim(),
    description: form.description,
    shared_org_unit_ids: form.shared_org_unit_ids.map(Number),
    shared_org_unit_roles: Object.fromEntries(
      form.shared_org_unit_ids.map((id) => [
        id,
        normalizePolicyGroupShareRole(form.shared_org_unit_roles[id]),
      ])
    ),
    shared_expires_at:
      form.shared_org_unit_ids.length > 0 && form.shared_expires_at
        ? endOfDayUnix(form.shared_expires_at)
        : 0,
    status: Number(form.status),
  })

  const mutation = useMutation({
    mutationFn: () =>
      props.group
        ? updateEnterprisePolicyGroup(props.group.id, payload())
        : createEnterprisePolicyGroup(payload()),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Saved'))
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {t(editing ? 'Edit Policy Group' : 'Create Policy Group')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Groups let you allocate quota across cross-department cohorts.'
            )}
          </DialogDescription>
        </DialogHeader>
        <form
          className='space-y-3'
          onSubmit={(event) => {
            event.preventDefault()
            mutation.mutate()
          }}
        >
          <Field label='Name'>
            <Input
              value={form.name}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  name: event.target.value,
                  slug: current.slug || slugify(event.target.value),
                }))
              }
            />
          </Field>
          <Field label='Slug'>
            <Input
              value={form.slug}
              onChange={(event) =>
                setForm((current) => ({ ...current, slug: event.target.value }))
              }
            />
          </Field>
          <Field label='Status'>
            <Select
              value={form.status}
              onValueChange={(value) =>
                setForm((current) => ({
                  ...current,
                  status: normalizeSelectValue(value),
                }))
              }
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value='1'>{t('Enabled')}</SelectItem>
                  <SelectItem value='2'>{t('Disabled')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <Field label='Description'>
            <Textarea
              value={form.description}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  description: event.target.value,
                }))
              }
            />
          </Field>
          <Field label='Shared Org Units'>
            <div className='max-h-44 space-y-2 overflow-y-auto rounded-md border p-2'>
              {orgOptions.length === 0 ? (
                <p className='text-muted-foreground text-sm'>
                  {t('No org units')}
                </p>
              ) : (
                orgOptions.map((option) => {
                  const checked = form.shared_org_unit_ids.includes(
                    option.value
                  )
                  return (
                    <div
                      key={option.value}
                      className='flex items-center gap-2 text-sm'
                    >
                      <Checkbox
                        checked={checked}
                        onCheckedChange={(checked) =>
                          setForm((current) => {
                            const nextRoles = {
                              ...current.shared_org_unit_roles,
                            }
                            const nextIds = checked
                              ? Array.from(
                                  new Set([
                                    ...current.shared_org_unit_ids,
                                    option.value,
                                  ])
                                )
                              : current.shared_org_unit_ids.filter(
                                  (item) => item !== option.value
                                )
                            if (checked) {
                              nextRoles[option.value] =
                                normalizePolicyGroupShareRole(
                                  nextRoles[option.value]
                                )
                            } else {
                              delete nextRoles[option.value]
                            }
                            return {
                              ...current,
                              shared_org_unit_ids: nextIds,
                              shared_org_unit_roles: nextRoles,
                            }
                          })
                        }
                      />
                      <span className='min-w-0 flex-1 truncate'>
                        {option.label}
                      </span>
                      <Select
                        value={normalizePolicyGroupShareRole(
                          form.shared_org_unit_roles[option.value]
                        )}
                        disabled={!checked}
                        onValueChange={(value) =>
                          setForm((current) => ({
                            ...current,
                            shared_org_unit_roles: {
                              ...current.shared_org_unit_roles,
                              [option.value]:
                                normalizePolicyGroupShareRole(value),
                            },
                          }))
                        }
                      >
                        <SelectTrigger className='h-8 w-28'>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            <SelectItem value='editor'>
                              {t('Editor')}
                            </SelectItem>
                            <SelectItem value='viewer'>
                              {t('Viewer')}
                            </SelectItem>
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </div>
                  )
                })
              )}
            </div>
          </Field>
          <Field label='Shared Until'>
            <Input
              type='date'
              value={form.shared_expires_at}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  shared_expires_at: event.target.value,
                }))
              }
            />
          </Field>
          <DialogFooter>
            <Button type='submit' disabled={mutation.isPending}>
              {t('Save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

type PolicyGroupShareRequestFormState = {
  org_unit_id: string
  role: EnterprisePolicyGroupShareRole
  shared_expires_at: string
  reason: string
}

function PolicyGroupShareRequestDialog(props: {
  open: boolean
  group?: EnterprisePolicyGroup | null
  orgUnits: EnterpriseOrgUnit[]
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const orgOptions = buildOrgOptions(props.orgUnits).filter(
    (option) => Number(option.value) !== props.group?.org_unit_id
  )
  const [form, setForm] = useState<PolicyGroupShareRequestFormState>(() => ({
    org_unit_id: '',
    role: 'editor',
    shared_expires_at: '',
    reason: '',
  }))

  const payload = (): EnterprisePolicyGroupShareRequestPayload => ({
    org_unit_id: Number(form.org_unit_id),
    role: form.role,
    shared_expires_at: form.shared_expires_at
      ? endOfDayUnix(form.shared_expires_at)
      : 0,
    reason: form.reason.trim(),
  })

  const mutation = useMutation({
    mutationFn: () => {
      if (!props.group) throw new Error('missing policy group')
      return createEnterprisePolicyGroupShareRequest(props.group.id, payload())
    },
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Submitted'))
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('Create Share Request')}</DialogTitle>
          <DialogDescription>
            {t(
              'Ask another department to approve access to this policy group.'
            )}
          </DialogDescription>
        </DialogHeader>
        {props.group && (
          <form
            className='space-y-3'
            onSubmit={(event) => {
              event.preventDefault()
              mutation.mutate()
            }}
          >
            <div className='rounded-lg border p-3 text-sm'>
              <div className='font-medium'>{props.group.name}</div>
              <div className='text-muted-foreground mt-1 text-xs'>
                {props.group.slug}
              </div>
            </div>
            <Field label='Target Org Unit'>
              <Select
                value={form.org_unit_id || ALL_VALUE}
                onValueChange={(value) =>
                  setForm((current) => ({
                    ...current,
                    org_unit_id: normalizeOptionalSelectValue(value),
                  }))
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue placeholder={t('Select org unit')} />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value={ALL_VALUE}>
                      {t('Select org unit')}
                    </SelectItem>
                    {orgOptions.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field label='Role'>
              <Select
                value={form.role}
                onValueChange={(value) =>
                  setForm((current) => ({
                    ...current,
                    role: normalizePolicyGroupShareRole(value),
                  }))
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='editor'>{t('Editor')}</SelectItem>
                    <SelectItem value='viewer'>{t('Viewer')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field label='Shared Until'>
              <Input
                type='date'
                value={form.shared_expires_at}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    shared_expires_at: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Reason'>
              <Textarea
                value={form.reason}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    reason: event.target.value,
                  }))
                }
              />
            </Field>
            <DialogFooter>
              <Button
                type='submit'
                disabled={mutation.isPending || !form.org_unit_id}
              >
                {t('Submit')}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  )
}

function PolicyGroupShareRequestDecisionDialog(props: {
  request: EnterprisePolicyGroupShareRequest | null
  action: 'approve' | 'reject'
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [decisionReason, setDecisionReason] = useState('')
  const request = props.request

  const payload = (): EnterprisePolicyGroupShareRequestDecisionPayload => ({
    decision_reason: decisionReason.trim(),
  })

  const mutation = useMutation({
    mutationFn: () => {
      if (!request) throw new Error('missing share request')
      return props.action === 'approve'
        ? approveEnterprisePolicyGroupShareRequest(request.id, payload())
        : rejectEnterprisePolicyGroupShareRequest(request.id, payload())
    },
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t(props.action === 'approve' ? 'Approved' : 'Rejected'))
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  return (
    <Dialog open={Boolean(request)} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>
            {t(
              props.action === 'approve'
                ? 'Approve Share Request'
                : 'Reject Share Request'
            )}
          </DialogTitle>
          <DialogDescription>
            {t(
              props.action === 'approve'
                ? 'Approved sharing becomes visible to the target department immediately.'
                : 'Rejected requests remain visible in sharing history.'
            )}
          </DialogDescription>
        </DialogHeader>
        {request && (
          <div className='space-y-3'>
            <div className='rounded-lg border p-3 text-sm'>
              <div className='font-medium'>
                {request.policy_group_name || `#${request.policy_group_id}`}
              </div>
              <div className='text-muted-foreground mt-1 text-xs'>
                {request.requester_org_unit_name || '-'} {'->'}{' '}
                {request.target_org_unit_name || '-'}
              </div>
            </div>
            <Field label='Decision Reason'>
              <Textarea
                value={decisionReason}
                onChange={(event) => setDecisionReason(event.target.value)}
              />
            </Field>
            <DialogFooter>
              <Button
                type='button'
                variant={props.action === 'approve' ? 'default' : 'destructive'}
                disabled={mutation.isPending}
                onClick={() => mutation.mutate()}
              >
                {t(props.action === 'approve' ? 'Approve' : 'Reject')}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

function PolicyGroupMembersDialog(props: {
  open: boolean
  group?: EnterprisePolicyGroup | null
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [userIds, setUserIds] = useState('')
  const [role, setRole] = useState('viewer')
  const groupId = props.group?.id ?? 0
  const canManageMembers = props.group?.can_manage_members ?? true

  const membersQuery = useQuery({
    queryKey: [
      'enterprise',
      'policy-groups',
      groupId,
      'members',
      page,
      keyword,
    ],
    queryFn: () =>
      getEnterprisePolicyGroupMembers(groupId, {
        p: page,
        page_size: PAGE_SIZE,
        keyword,
      }),
    enabled: props.open && groupId > 0,
  })

  const addMutation = useMutation({
    mutationFn: () =>
      addEnterprisePolicyGroupMembers(groupId, {
        user_ids: parseUserIdList(userIds),
        role,
      }),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Saved'))
      setUserIds('')
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (userId: number) =>
      deleteEnterprisePolicyGroupMember(groupId, userId),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Removed'))
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  const members = getPageItems(membersQuery.data)
  const total = getPageTotal(membersQuery.data)
  const parsedUserIds = parseUserIdList(userIds)

  function handleAdd(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (parsedUserIds.length === 0) {
      toast.error(t('User ID is required'))
      return
    }
    addMutation.mutate()
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Policy Group Members')}</DialogTitle>
          <DialogDescription>
            {props.group?.name
              ? t('Manage members for {{name}}.', { name: props.group.name })
              : t('Manage policy group members.')}
          </DialogDescription>
        </DialogHeader>

        {canManageMembers ? (
          <form className='flex flex-wrap items-end gap-2' onSubmit={handleAdd}>
            <Field label='User IDs'>
              <Input
                value={userIds}
                onChange={(event) => setUserIds(event.target.value)}
                placeholder={t('1, 2, 3')}
                className='min-w-64'
              />
            </Field>
            <Field label='Role'>
              <Select
                value={role}
                onValueChange={(value) =>
                  setRole(normalizeSelectValue(value) || 'viewer')
                }
              >
                <SelectTrigger className='w-40'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='viewer'>{t('Viewer')}</SelectItem>
                    <SelectItem value='editor'>{t('Editor')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Button type='submit' disabled={addMutation.isPending}>
              <UserPlus className='size-3.5' />
              {t('Add')}
            </Button>
          </form>
        ) : null}

        <FilterBar>
          <SearchInput
            value={keyword}
            onChange={(value) => {
              setKeyword(value)
              setPage(1)
            }}
            placeholder='Search members'
          />
        </FilterBar>

        <QueryState
          query={{
            data: membersQuery.data,
            isLoading: membersQuery.isLoading,
            isError: membersQuery.isError,
            error: membersQuery.error,
            refetch: membersQuery.refetch,
          }}
          empty={members.length === 0}
          emptyTitle='No members'
          emptyDescription='Add users by ID to include them in this policy group.'
        >
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('User')}</TableHead>
                <TableHead>{t('Role')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.map((member) => (
                <TableRow key={member.user_id}>
                  <TableCell>
                    <div className='min-w-0'>
                      <div className='truncate font-medium'>
                        {member.display_name || member.username}
                      </div>
                      <div className='text-muted-foreground truncate text-xs'>
                        #{member.user_id} · {member.email || member.username}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant='secondary'>
                      {t(member.role === 'editor' ? 'Editor' : 'Viewer')}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={member.status} />
                  </TableCell>
                  <TableCell>
                    <div className='flex justify-end'>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        disabled={deleteMutation.isPending || !canManageMembers}
                        onClick={() => deleteMutation.mutate(member.user_id)}
                      >
                        <Trash2 className='size-3.5' />
                        <span className='sr-only'>{t('Remove')}</span>
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <Pager
            page={page}
            pageSize={PAGE_SIZE}
            total={total}
            onPageChange={setPage}
          />
        </QueryState>
      </DialogContent>
    </Dialog>
  )
}

type ProjectFormState = {
  name: string
  slug: string
  description: string
  owner_user_id: string
  org_unit_ids: string[]
  status: string
}

function ProjectDialog(props: {
  open: boolean
  project?: EnterpriseProject | null
  orgUnits: EnterpriseOrgUnit[]
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<ProjectFormState>(() => ({
    name: props.project?.name ?? '',
    slug: props.project?.slug ?? '',
    description: props.project?.description ?? '',
    owner_user_id: props.project?.owner_user_id
      ? String(props.project.owner_user_id)
      : '',
    org_unit_ids: (props.project?.org_unit_ids ?? []).map(String),
    status: String(props.project?.status ?? ENABLED_STATUS),
  }))
  const editing = Boolean(props.project)

  const orgOptions = buildOrgOptions(props.orgUnits)
  const payload = (): EnterpriseProjectPayload => ({
    name: form.name.trim(),
    slug: (form.slug.trim() || slugify(form.name)).trim(),
    description: form.description,
    owner_user_id: Number(form.owner_user_id || 0),
    org_unit_ids: form.org_unit_ids
      .map((value) => Number(value))
      .filter((value) => value > 0),
    status: Number(form.status),
  })

  const mutation = useMutation({
    mutationFn: () =>
      props.project
        ? updateEnterpriseProject(props.project.id, payload())
        : createEnterpriseProject(payload()),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Saved'))
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  function toggleOrgUnit(value: string, checked: boolean) {
    setForm((current) => ({
      ...current,
      org_unit_ids: checked
        ? Array.from(new Set([...current.org_unit_ids, value]))
        : current.org_unit_ids.filter((item) => item !== value),
    }))
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>
            {t(editing ? 'Edit Project' : 'Create Project')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Projects define cost centers for API Key defaults, request attribution, and project-level quota policies.'
            )}
          </DialogDescription>
        </DialogHeader>
        <form
          className='grid gap-3'
          onSubmit={(event) => {
            event.preventDefault()
            mutation.mutate()
          }}
        >
          <div className='grid gap-3 sm:grid-cols-2'>
            <Field label='Name'>
              <Input
                value={form.name}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    name: event.target.value,
                    slug: current.slug || slugify(event.target.value),
                  }))
                }
              />
            </Field>
            <Field label='Slug'>
              <Input
                value={form.slug}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    slug: event.target.value,
                  }))
                }
              />
            </Field>
          </div>
          <div className='grid gap-3 sm:grid-cols-2'>
            <Field label='Owner User ID'>
              <Input
                type='number'
                value={form.owner_user_id}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    owner_user_id: event.target.value,
                  }))
                }
                placeholder='0'
              />
            </Field>
            <Field label='Status'>
              <Select
                value={form.status}
                onValueChange={(value) =>
                  setForm((current) => ({
                    ...current,
                    status: normalizeSelectValue(value),
                  }))
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='1'>{t('Enabled')}</SelectItem>
                    <SelectItem value='2'>{t('Disabled')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
          </div>
          <Field label='Org Units'>
            <div className='max-h-56 overflow-y-auto rounded-lg border p-2'>
              {orgOptions.length === 0 ? (
                <div className='text-muted-foreground px-1 py-2 text-sm'>
                  {t('No enabled org units')}
                </div>
              ) : (
                <div className='grid gap-1'>
                  {orgOptions.map((option) => (
                    <label
                      key={option.value}
                      className='hover:bg-muted/50 flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm'
                    >
                      <Checkbox
                        checked={form.org_unit_ids.includes(option.value)}
                        onCheckedChange={(checked) =>
                          toggleOrgUnit(option.value, checked === true)
                        }
                      />
                      <span className='min-w-0 truncate'>{option.label}</span>
                    </label>
                  ))}
                </div>
              )}
            </div>
          </Field>
          <Field label='Description'>
            <Textarea
              value={form.description}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  description: event.target.value,
                }))
              }
            />
          </Field>
          <DialogFooter>
            <Button type='submit' disabled={mutation.isPending}>
              {t('Save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

type QuotaPolicyFormState = {
  name: string
  description: string
  target_type: PolicyTargetType
  target_id: string
  metric: PolicyMetric
  period: PolicyPeriod
  limit_value: string
  timezone: string
  model_scope: PolicyModelScope
  models: string
  condition_mode: PolicyConditionMode
  condition_json: string
  condition_expr: string
  condition_abilities: string
  condition_runtime_groups: string
  condition_model_prefixes: string
  condition_model_names: string
  condition_channel_ids: string
  condition_is_playground: string
  action: PolicyAction
  priority: string
  status: string
  effective_at: string
  expires_at: string
}

function policyToQuotaForm(
  policy?: EnterpriseQuotaPolicy | null
): QuotaPolicyFormState {
  const structuredCondition = parseStructuredCondition(
    policy?.condition_json ?? ''
  )
  return {
    name: policy?.name ?? '',
    description: policy?.description ?? '',
    target_type: policy?.target_type ?? 'enterprise',
    target_id: policy?.target_id ? String(policy.target_id) : '',
    metric: policy?.metric ?? 'quota',
    period: policy?.period ?? 'day',
    limit_value: policy?.limit_value ? String(policy.limit_value) : '',
    timezone: policy?.timezone ?? 'Asia/Shanghai',
    model_scope: policy?.model_scope ?? 'all',
    models: policy ? parseModelsJson(policy).join(', ') : '',
    condition_mode: policy?.condition_mode ?? 'structured',
    condition_json: policy?.condition_json ?? '',
    condition_expr: policy?.condition_expr ?? '',
    condition_abilities: stringifyStringList(structuredCondition.abilities),
    condition_runtime_groups: stringifyStringList(
      structuredCondition.runtime_groups
    ),
    condition_model_prefixes: stringifyStringList(
      structuredCondition.model_prefixes
    ),
    condition_model_names: stringifyStringList(structuredCondition.model_names),
    condition_channel_ids: stringifyNumberList(structuredCondition.channel_ids),
    condition_is_playground:
      structuredCondition.is_playground === undefined
        ? ''
        : String(structuredCondition.is_playground),
    action: policy?.action ?? 'reject',
    priority: String(policy?.priority ?? 0),
    status: String(policy?.status ?? ENABLED_STATUS),
    effective_at: policy?.effective_at ? String(policy.effective_at) : '',
    expires_at: policy?.expires_at ? String(policy.expires_at) : '',
  }
}

function QuotaPolicyDialog(props: {
  open: boolean
  policy?: EnterpriseQuotaPolicy | null
  orgUnits: EnterpriseOrgUnit[]
  projects: EnterpriseProject[]
  policyGroups: EnterprisePolicyGroup[]
  dryRunObservations: EnterpriseAuditLog[]
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<QuotaPolicyFormState>(() =>
    policyToQuotaForm(props.policy)
  )
  const [confirmStatusChange, setConfirmStatusChange] = useState(false)
  const editing = Boolean(props.policy)
  const orgOptions = buildOrgOptions(props.orgUnits)
  const projectOptions = props.projects
    .filter((project) => project.status === ENABLED_STATUS)
    .map((project) => ({ value: String(project.id), label: project.name }))
  const groupOptions = props.policyGroups
    .filter((group) => group.status === ENABLED_STATUS)
    .map((group) => ({ value: String(group.id), label: group.name }))

  const targetOptions = useMemo(() => {
    if (form.target_type === 'org_unit') return orgOptions
    if (form.target_type === 'project') return projectOptions
    if (form.target_type === 'policy_group') return groupOptions
    return []
  }, [form.target_type, groupOptions, orgOptions, projectOptions])

  const payload = (): EnterpriseQuotaPolicyPayload => ({
    name: form.name.trim(),
    description: form.description,
    target_type: form.target_type,
    target_id:
      form.target_type === 'enterprise' ? 0 : Number(form.target_id || 0),
    metric: form.metric,
    period: form.period,
    limit_value: Number(form.limit_value || 0),
    timezone: form.timezone.trim() || 'Asia/Shanghai',
    model_scope: form.model_scope,
    models:
      form.model_scope === 'specific'
        ? form.models
            .split(/[\n,]/)
            .map((model) => model.trim())
            .filter(Boolean)
        : [],
    condition_mode: form.condition_mode,
    condition_json:
      form.condition_mode === 'structured'
        ? buildStructuredConditionJson(form)
        : '',
    condition_expr:
      form.condition_mode === 'cel' ? form.condition_expr.trim() : '',
    action: form.action,
    priority: Number(form.priority || 0),
    status: Number(form.status),
    effective_at: Number(form.effective_at || 0),
    expires_at: Number(form.expires_at || 0),
  })

  const mutation = useMutation({
    mutationFn: () =>
      props.policy
        ? updateEnterpriseQuotaPolicy(props.policy.id, payload())
        : createEnterpriseQuotaPolicy(payload()),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Saved'))
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  const setField = <K extends keyof QuotaPolicyFormState>(
    key: K,
    value: QuotaPolicyFormState[K]
  ) => setForm((current) => ({ ...current, [key]: value }))

  const statusChanged =
    editing && Number(form.status) !== (props.policy?.status ?? ENABLED_STATUS)
  const submitPolicy = () => {
    if (statusChanged) {
      setConfirmStatusChange(true)
      return
    }
    mutation.mutate()
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>
            {t(editing ? 'Edit Quota Policy' : 'Create Quota Policy')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Create a hard limit with optional structured or CEL conditions.'
            )}
          </DialogDescription>
        </DialogHeader>
        <form
          className='grid gap-3'
          onSubmit={(event) => {
            event.preventDefault()
            submitPolicy()
          }}
        >
          <div className='grid gap-3 sm:grid-cols-2'>
            <Field label='Name'>
              <Input
                value={form.name}
                onChange={(event) => setField('name', event.target.value)}
              />
            </Field>
            <Field label='Timezone'>
              <Input
                value={form.timezone}
                onChange={(event) => setField('timezone', event.target.value)}
              />
            </Field>
          </div>
          <Field label='Description'>
            <Textarea
              value={form.description}
              onChange={(event) => setField('description', event.target.value)}
            />
          </Field>
          <div className='grid gap-3 sm:grid-cols-3'>
            <Field label='Target Type'>
              <Select
                value={form.target_type}
                onValueChange={(value) =>
                  setForm((current) => ({
                    ...current,
                    target_type: normalizeSelectValue(
                      value
                    ) as PolicyTargetType,
                    target_id: '',
                  }))
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='enterprise'>
                      {t('Enterprise')}
                    </SelectItem>
                    <SelectItem value='org_unit'>{t('Org Unit')}</SelectItem>
                    <SelectItem value='project'>{t('Project')}</SelectItem>
                    <SelectItem value='policy_group'>
                      {t('Policy Group')}
                    </SelectItem>
                    <SelectItem value='user'>{t('User')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field label='Target'>
              {form.target_type === 'enterprise' ? (
                <Input value='Enterprise' disabled />
              ) : form.target_type === 'user' ? (
                <Input
                  type='number'
                  value={form.target_id}
                  onChange={(event) =>
                    setField('target_id', event.target.value)
                  }
                  placeholder={t('User ID')}
                />
              ) : (
                <Select
                  value={form.target_id}
                  onValueChange={(value) =>
                    setField('target_id', normalizeSelectValue(value))
                  }
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue placeholder={t('Select target')} />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {targetOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              )}
            </Field>
            <Field label='Status'>
              <Select
                value={form.status}
                onValueChange={(value) =>
                  setField('status', normalizeSelectValue(value))
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='1'>{t('Enabled')}</SelectItem>
                    <SelectItem value='2'>{t('Disabled')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
          </div>
          <div className='grid gap-3 sm:grid-cols-4'>
            <Field label='Metric'>
              <Select
                value={form.metric}
                onValueChange={(value) =>
                  setField(
                    'metric',
                    normalizeSelectValue(value) as PolicyMetric
                  )
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='quota'>{t('Quota')}</SelectItem>
                    <SelectItem value='request_count'>
                      {t('Requests')}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field label='Period'>
              <Select
                value={form.period}
                onValueChange={(value) =>
                  setField(
                    'period',
                    normalizeSelectValue(value) as PolicyPeriod
                  )
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='day'>{t('Daily')}</SelectItem>
                    <SelectItem value='month'>{t('Monthly')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field label='Limit'>
              <Input
                type='number'
                value={form.limit_value}
                onChange={(event) =>
                  setField('limit_value', event.target.value)
                }
              />
            </Field>
            <Field label='Priority'>
              <Input
                type='number'
                value={form.priority}
                onChange={(event) => setField('priority', event.target.value)}
              />
            </Field>
          </div>
          <div className='grid gap-3 sm:grid-cols-3'>
            <Field label='Action'>
              <Select
                value={form.action}
                onValueChange={(value) =>
                  setField(
                    'action',
                    normalizeSelectValue(value) as PolicyAction
                  )
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {policyActionOptions.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {t(option.label)}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field label='Model Scope'>
              <Select
                value={form.model_scope}
                onValueChange={(value) =>
                  setField(
                    'model_scope',
                    normalizeSelectValue(value) as PolicyModelScope
                  )
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='all'>{t('All Models')}</SelectItem>
                    <SelectItem value='specific'>
                      {t('Specific Models')}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field label='Models'>
              <Input
                value={form.models}
                disabled={form.model_scope === 'all'}
                onChange={(event) => setField('models', event.target.value)}
                placeholder={t('model-a, model-b')}
              />
            </Field>
          </div>
          <div className='grid gap-3 sm:grid-cols-2'>
            <Field label='Condition Mode'>
              <Select
                value={form.condition_mode}
                onValueChange={(value) =>
                  setField(
                    'condition_mode',
                    normalizeSelectValue(value) as PolicyConditionMode
                  )
                }
              >
                <SelectTrigger className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='structured'>
                      {t('Structured')}
                    </SelectItem>
                    <SelectItem value='cel'>{t('CEL')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <div className='grid gap-3 sm:grid-cols-2'>
              <Field label='Effective At'>
                <Input
                  type='number'
                  value={form.effective_at}
                  onChange={(event) =>
                    setField('effective_at', event.target.value)
                  }
                  placeholder='0'
                />
              </Field>
              <Field label='Expires At'>
                <Input
                  type='number'
                  value={form.expires_at}
                  onChange={(event) =>
                    setField('expires_at', event.target.value)
                  }
                  placeholder='0'
                />
              </Field>
            </div>
          </div>
          {form.condition_mode === 'structured' ? (
            <div className='bg-muted/15 rounded-lg border p-3'>
              <div className='mb-3'>
                <h4 className='text-sm font-medium'>
                  {t('Structured Conditions')}
                </h4>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t('Leave fields empty to match all requests.')}
                </p>
              </div>
              <div className='grid gap-3 sm:grid-cols-2'>
                <Field label='Abilities'>
                  <Input
                    value={form.condition_abilities}
                    onChange={(event) =>
                      setField('condition_abilities', event.target.value)
                    }
                    placeholder={t('chat, image')}
                  />
                </Field>
                <Field label='Runtime Groups'>
                  <Input
                    value={form.condition_runtime_groups}
                    onChange={(event) =>
                      setField('condition_runtime_groups', event.target.value)
                    }
                    placeholder={t('default, vip')}
                  />
                </Field>
                <Field label='Model Names'>
                  <Input
                    value={form.condition_model_names}
                    onChange={(event) =>
                      setField('condition_model_names', event.target.value)
                    }
                    placeholder={t('gpt-4o, claude-3-5-sonnet')}
                  />
                </Field>
                <Field label='Model Prefixes'>
                  <Input
                    value={form.condition_model_prefixes}
                    onChange={(event) =>
                      setField('condition_model_prefixes', event.target.value)
                    }
                    placeholder={t('gpt-4, claude')}
                  />
                </Field>
                <Field label='Channel IDs'>
                  <Input
                    value={form.condition_channel_ids}
                    onChange={(event) =>
                      setField('condition_channel_ids', event.target.value)
                    }
                    placeholder='1, 2, 3'
                  />
                </Field>
                <Field label='Playground'>
                  <Select
                    value={form.condition_is_playground || ALL_VALUE}
                    onValueChange={(value) =>
                      setField(
                        'condition_is_playground',
                        normalizeOptionalSelectValue(value)
                      )
                    }
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        <SelectItem value={ALL_VALUE}>
                          {t('Any Source')}
                        </SelectItem>
                        <SelectItem value='true'>{t('Playground')}</SelectItem>
                        <SelectItem value='false'>{t('API Key')}</SelectItem>
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </Field>
              </div>
              <div className='bg-muted mt-3 rounded-md px-2.5 py-2'>
                <div className='text-muted-foreground mb-1 text-xs'>
                  {t('Condition JSON Preview')}
                </div>
                <code className='block overflow-x-auto text-xs'>
                  {buildStructuredConditionJson(form) || '{}'}
                </code>
              </div>
            </div>
          ) : (
            <Field label='CEL Expression'>
              <Textarea
                value={form.condition_expr}
                onChange={(event) =>
                  setField('condition_expr', event.target.value)
                }
                placeholder='request.model in ["gpt-4o"]'
                className='min-h-24 font-mono text-xs'
              />
            </Field>
          )}
          <DialogFooter>
            <Button type='submit' disabled={mutation.isPending}>
              {t('Save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
      <QuotaPolicyStatusChangeDialog
        open={confirmStatusChange}
        policy={props.policy ?? null}
        nextStatus={Number(form.status)}
        dryRunObservations={props.dryRunObservations}
        isPending={mutation.isPending}
        onOpenChange={setConfirmStatusChange}
        onConfirm={() => mutation.mutate()}
      />
    </Dialog>
  )
}

type QuotaRequestFormState = {
  policy_id: string
  limit_delta: string
  expires_at: string
  reason: string
}

function QuotaRequestDialog(props: {
  open: boolean
  policies: EnterpriseQuotaPolicy[]
  initialValues?: QuotaRequestInitialValues | null
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<QuotaRequestFormState>(() => ({
    policy_id: props.initialValues?.policyId
      ? String(props.initialValues.policyId)
      : props.policies[0]?.id
        ? String(props.policies[0].id)
        : '',
    limit_delta: props.initialValues?.limitDelta
      ? String(props.initialValues.limitDelta)
      : '',
    expires_at: todayInputValue(),
    reason: props.initialValues?.reason ?? '',
  }))

  const payload = (): EnterpriseQuotaRequestPayload => ({
    policy_id: Number(form.policy_id || 0),
    limit_delta: Number(form.limit_delta || 0),
    reason: form.reason.trim(),
    expires_at: endOfDayUnix(form.expires_at),
  })

  const mutation = useMutation({
    mutationFn: () => submitEnterpriseQuotaRequest(payload()),
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Submitted'))
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('Submit Quota Request')}</DialogTitle>
          <DialogDescription>
            {t(
              'Request a temporary limit increase for an existing quota policy.'
            )}
          </DialogDescription>
        </DialogHeader>
        <form
          className='grid gap-3'
          onSubmit={(event) => {
            event.preventDefault()
            mutation.mutate()
          }}
        >
          <Field label='Quota Policy'>
            <Select
              value={form.policy_id}
              onValueChange={(value) =>
                setForm((current) => ({
                  ...current,
                  policy_id: normalizeSelectValue(value),
                }))
              }
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder={t('Select policy')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {props.policies.map((policy) => (
                    <SelectItem key={policy.id} value={String(policy.id)}>
                      {policy.name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <div className='grid gap-3 sm:grid-cols-2'>
            <Field label='Extra Limit'>
              <Input
                type='number'
                value={form.limit_delta}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    limit_delta: event.target.value,
                  }))
                }
              />
            </Field>
            <Field label='Expires'>
              <Input
                type='date'
                value={form.expires_at}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    expires_at: event.target.value,
                  }))
                }
              />
            </Field>
          </div>
          <Field label='Reason'>
            <Textarea
              value={form.reason}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  reason: event.target.value,
                }))
              }
            />
          </Field>
          <DialogFooter>
            <Button type='submit' disabled={mutation.isPending}>
              {t('Submit')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function QuotaRequestDecisionDialog(props: {
  request: EnterpriseQuotaRequest | null
  action: 'approve' | 'reject'
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [decisionReason, setDecisionReason] = useState('')
  const request = props.request

  const payload = (): EnterpriseQuotaRequestDecisionPayload => ({
    decision_reason: decisionReason.trim(),
  })

  const mutation = useMutation({
    mutationFn: () => {
      if (!request) throw new Error('missing quota request')
      return props.action === 'approve'
        ? approveEnterpriseQuotaRequest(request.id, payload())
        : rejectEnterpriseQuotaRequest(request.id, payload())
    },
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t(props.action === 'approve' ? 'Approved' : 'Rejected'))
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
      queryClient.invalidateQueries({
        queryKey: ['notifications', 'enterprise-quota-requests'],
      })
    },
  })

  return (
    <Dialog open={Boolean(request)} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>
            {t(
              props.action === 'approve' ? 'Approve Request' : 'Reject Request'
            )}
          </DialogTitle>
          <DialogDescription>
            {t(
              props.action === 'approve'
                ? 'Approved temporary quota takes effect immediately and expires at the requested time.'
                : 'Rejected requests remain visible in the approval history.'
            )}
          </DialogDescription>
        </DialogHeader>
        {request && (
          <div className='space-y-3'>
            <div className='rounded-lg border p-3 text-sm'>
              <div className='font-medium'>{request.policy_name}</div>
              <div className='text-muted-foreground mt-1 text-xs'>
                {formatNumber(request.limit_delta)} ·{' '}
                {formatDateTime(request.expires_at)}
              </div>
            </div>
            <QuotaRequestRiskSummary request={request} />
            <Field label='Decision Reason'>
              <Textarea
                value={decisionReason}
                onChange={(event) => setDecisionReason(event.target.value)}
              />
            </Field>
            <DialogFooter>
              <Button
                type='button'
                variant={props.action === 'approve' ? 'default' : 'destructive'}
                disabled={mutation.isPending}
                onClick={() => mutation.mutate()}
              >
                {t(props.action === 'approve' ? 'Approve' : 'Reject')}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

function QuotaRequestRiskSummary(props: { request: EnterpriseQuotaRequest }) {
  const { t } = useTranslation()
  const request = props.request
  const stackedLimit =
    request.stacked_limit_value ||
    (request.policy_limit_value || 0) + request.limit_delta

  return (
    <div className='rounded-lg border p-3 text-sm'>
      <div className='mb-3 font-medium'>{t('Risk Summary')}</div>
      <div className='grid gap-3 sm:grid-cols-2'>
        <AuditDetailField
          label='Current Usage'
          value={`${formatNumber(request.policy_used_value)} / ${formatNumber(request.policy_limit_value)}`}
        />
        <AuditDetailField
          label='Limit after approval'
          value={formatNumber(stackedLimit)}
        />
        <AuditDetailField
          label='Remaining validity'
          value={formatRemainingTime(request.expires_at)}
        />
        <AuditDetailField
          label='Recent policy hits'
          value={formatNumber(request.recent_policy_hits)}
        />
        <AuditDetailField
          label='Recent dry-run hits'
          value={formatNumber(request.recent_dry_run_hits)}
        />
        <AuditDetailField
          label='Target'
          value={`${request.target_type} #${request.target_id || '-'}`}
          mono
        />
      </div>
    </div>
  )
}

function QuotaRequestBatchDecisionDialog(props: {
  ids: number[]
  action: 'approve' | 'reject'
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [decisionReason, setDecisionReason] = useState('')
  const [result, setResult] =
    useState<EnterpriseQuotaRequestBatchDecisionResult | null>(null)
  const open = props.ids.length > 0
  const failedItems = result?.items.filter((item) => !item.success) ?? []

  useEffect(() => {
    setDecisionReason('')
    setResult(null)
  }, [props.action, props.ids])

  const mutation = useMutation({
    mutationFn: () =>
      props.action === 'approve'
        ? batchApproveEnterpriseQuotaRequests({
            ids: props.ids,
            decision_reason: decisionReason.trim(),
          })
        : batchRejectEnterpriseQuotaRequests({
            ids: props.ids,
            decision_reason: decisionReason.trim(),
          }),
    onSuccess: (response) => {
      if (!response.success || !response.data) return
      const data = response.data
      setResult(data)
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
      queryClient.invalidateQueries({
        queryKey: ['notifications', 'enterprise-quota-requests'],
      })
      if (data.failure_count === 0) {
        toast.success(
          t(props.action === 'approve' ? 'Approved' : 'Rejected') +
            ` ${data.success_count}`
        )
        props.onOpenChange(false)
        return
      }
      toast.error(t('Some requests failed') + ` (${data.failure_count})`)
    },
  })

  return (
    <Dialog open={open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>
            {t(
              props.action === 'approve'
                ? 'Approve Selected Requests'
                : 'Reject Selected Requests'
            )}
          </DialogTitle>
          <DialogDescription>
            {t('Each selected request is processed independently.')}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-3'>
          <div className='rounded-lg border p-3 text-sm'>
            <div className='font-medium'>
              {t('Selected')} {props.ids.length}
            </div>
            <div className='text-muted-foreground mt-1 truncate text-xs'>
              {props.ids.map((id) => `#${id}`).join(', ')}
            </div>
          </div>
          <Field label='Decision Reason'>
            <Textarea
              value={decisionReason}
              onChange={(event) => setDecisionReason(event.target.value)}
            />
          </Field>
          {failedItems.length > 0 && (
            <div className='rounded-lg border p-3 text-sm'>
              <div className='font-medium'>{t('Failed Requests')}</div>
              <div className='mt-2 space-y-1'>
                {failedItems.map((item) => (
                  <div
                    key={item.id}
                    className='text-muted-foreground flex items-center justify-between gap-2 text-xs'
                  >
                    <span className='font-mono'>#{item.id}</span>
                    <span className='truncate'>{item.message || '-'}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => props.onOpenChange(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              variant={props.action === 'approve' ? 'default' : 'destructive'}
              disabled={mutation.isPending || props.ids.length === 0}
              onClick={() => mutation.mutate()}
            >
              {t(props.action === 'approve' ? 'Approve' : 'Reject')}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  )
}

function QuotaPolicyStatusChangeDialog(props: {
  open: boolean
  policy: EnterpriseQuotaPolicy | null
  nextStatus: number
  dryRunObservations: EnterpriseAuditLog[]
  isPending: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
}) {
  const { t } = useTranslation()
  const policy = props.policy
  const enabling = props.nextStatus === ENABLED_STATUS
  const dryRunHits = policy
    ? getPolicyDryRunHits(policy, props.dryRunObservations)
    : 0

  return (
    <AlertDialog open={props.open} onOpenChange={props.onOpenChange}>
      <AlertDialogContent className='sm:max-w-lg'>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {t(enabling ? 'Confirm enable' : 'Confirm disable')}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              enabling
                ? 'Enabling this hard limit may start rejecting matching relay requests immediately.'
                : 'Disabling this hard limit immediately stops enforcement for matching relay requests.'
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        {policy && (
          <div className='space-y-3 rounded-lg border p-3 text-sm'>
            <div className='min-w-0'>
              <div className='truncate font-medium'>{policy.name}</div>
              <div className='text-muted-foreground mt-1 text-xs'>
                {t(formatPolicyTarget(policy))} ·{' '}
                {t(formatMetric(policy.metric))} ·{' '}
                {t(formatPeriod(policy.period))}
              </div>
            </div>
            <div className='grid gap-3 sm:grid-cols-2'>
              <AuditDetailField
                label='Current Usage'
                value={`${formatNumber(policy.used_value)} / ${formatNumber(policy.limit_value)}`}
              />
              <AuditDetailField
                label='Impact Risk'
                value={t(getPolicyRiskLabel(policy))}
              />
              <AuditDetailField
                label='Recent dry-run hits'
                value={formatNumber(dryRunHits)}
              />
              <AuditDetailField
                label='Next Status'
                value={t(enabling ? 'Enabled' : 'Disabled')}
              />
            </div>
          </div>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={props.isPending}>
            {t('Cancel')}
          </AlertDialogCancel>
          <AlertDialogAction
            disabled={props.isPending}
            className={cn(
              !enabling &&
                'bg-destructive text-destructive-foreground hover:bg-destructive/90'
            )}
            onClick={props.onConfirm}
          >
            {t(enabling ? 'Enable' : 'Disable')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

function Field(props: { label: string; children: ReactNode }) {
  const { t } = useTranslation()
  return (
    <div className='grid gap-1.5'>
      <Label>{t(props.label)}</Label>
      {props.children}
    </div>
  )
}

export function EnterpriseGovernance() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const currentUser = useAuthStore((state) => state.auth.user)
  const currentUserId = currentUser?.id ?? 0
  const isSystemAdmin = (currentUser?.role ?? 0) >= ROLE.ADMIN
  const enterprisePermissions = currentUser?.permissions?.enterprise_governance
  const canReadEnterprise =
    isSystemAdmin || enterprisePermissions?.read === true
  const canManageEnterprise =
    isSystemAdmin || enterprisePermissions?.manage === true
  const canManageDepartment =
    canManageEnterprise || enterprisePermissions?.department_manage === true
  const canReadFinance =
    canManageEnterprise ||
    enterprisePermissions?.finance_read === true ||
    enterprisePermissions?.project_read === true
  const canReadAudit =
    canManageEnterprise ||
    enterprisePermissions?.audit_read === true ||
    enterprisePermissions?.department_manage === true ||
    enterprisePermissions?.project_read === true ||
    enterprisePermissions?.project_manage === true
  const canApproveQuota =
    canManageEnterprise || enterprisePermissions?.quota_approve === true
  const canManageProjects =
    canManageEnterprise || enterprisePermissions?.project_manage === true
  const canReadProjects =
    canManageProjects || enterprisePermissions?.project_read === true
  const availableTabs = useMemo(
    () =>
      tabs.filter((tab) => {
        switch (tab.value) {
          case 'overview':
          case 'quota-requests':
            return canReadEnterprise
          case 'projects':
            return canReadProjects
          case 'usage':
            return canReadFinance
          case 'audit':
            return canReadAudit
          case 'organization':
          case 'policy-groups':
          case 'quota-policies':
            return canManageDepartment
          case 'notifications':
          case 'webhooks':
          case 'deliveries':
            return canManageEnterprise
          default:
            return false
        }
      }),
    [
      canManageEnterprise,
      canManageDepartment,
      canManageProjects,
      canReadProjects,
      canReadAudit,
      canReadEnterprise,
      canReadFinance,
    ]
  )
  const fallbackTab = availableTabs[0]?.value ?? 'overview'
  const requestedTab = search.tab ?? fallbackTab
  const activeTab = availableTabs.some((tab) => tab.value === requestedTab)
    ? requestedTab
    : fallbackTab
  useEffect(() => {
    if (search.tab && search.tab !== activeTab) {
      void navigate({
        search: (prev) => ({
          ...prev,
          tab: activeTab,
        }),
      })
    }
  }, [activeTab, navigate, search.tab])
  const setActiveTab = (tab: EnterpriseTab) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        tab,
      }),
    })
  }
  const [enterpriseDialogOpen, setEnterpriseDialogOpen] = useState(false)
  const [orgDialogOpen, setOrgDialogOpen] = useState(false)
  const [editingOrgUnit, setEditingOrgUnit] =
    useState<EnterpriseOrgUnit | null>(null)
  const [policyGroupDialogOpen, setPolicyGroupDialogOpen] = useState(false)
  const [editingPolicyGroup, setEditingPolicyGroup] =
    useState<EnterprisePolicyGroup | null>(null)
  const [policyGroupMembersDialogOpen, setPolicyGroupMembersDialogOpen] =
    useState(false)
  const [memberPolicyGroup, setMemberPolicyGroup] =
    useState<EnterprisePolicyGroup | null>(null)
  const [
    policyGroupShareRequestDialogOpen,
    setPolicyGroupShareRequestDialogOpen,
  ] = useState(false)
  const [sharingPolicyGroup, setSharingPolicyGroup] =
    useState<EnterprisePolicyGroup | null>(null)
  const [decidingPolicyGroupShareRequest, setDecidingPolicyGroupShareRequest] =
    useState<EnterprisePolicyGroupShareRequest | null>(null)
  const [
    policyGroupShareRequestDecisionAction,
    setPolicyGroupShareRequestDecisionAction,
  ] = useState<'approve' | 'reject'>('approve')
  const [projectDialogOpen, setProjectDialogOpen] = useState(false)
  const [editingProject, setEditingProject] =
    useState<EnterpriseProject | null>(null)
  const [projectMembersDialogOpen, setProjectMembersDialogOpen] =
    useState(false)
  const [memberProject, setMemberProject] = useState<EnterpriseProject | null>(
    null
  )
  const [quotaPolicyDialogOpen, setQuotaPolicyDialogOpen] = useState(false)
  const [editingQuotaPolicy, setEditingQuotaPolicy] =
    useState<EnterpriseQuotaPolicy | null>(null)
  const [quotaRequestDialogOpen, setQuotaRequestDialogOpen] = useState(false)
  const [quotaRequestInitialValues, setQuotaRequestInitialValues] =
    useState<QuotaRequestInitialValues | null>(null)
  const [decidingQuotaRequest, setDecidingQuotaRequest] =
    useState<EnterpriseQuotaRequest | null>(null)
  const [batchDecidingQuotaRequestIds, setBatchDecidingQuotaRequestIds] =
    useState<number[]>([])
  const [viewingQuotaRequest, setViewingQuotaRequest] =
    useState<EnterpriseQuotaRequest | null>(null)
  const [quotaRequestDecisionAction, setQuotaRequestDecisionAction] = useState<
    'approve' | 'reject'
  >('approve')
  const [selectedQuotaRequestIds, setSelectedQuotaRequestIds] = useState<
    number[]
  >([])

  const [orgKeyword, setOrgKeyword] = useState('')
  const [orgStatus, setOrgStatus] = useState('')
  const [memberKeyword, setMemberKeyword] = useState('')
  const [memberOrgFilter, setMemberOrgFilter] = useState('')
  const [memberPage, setMemberPage] = useState(1)

  const [policyGroupKeyword, setPolicyGroupKeyword] = useState('')
  const [policyGroupStatus, setPolicyGroupStatus] = useState('')
  const [policyGroupPage, setPolicyGroupPage] = useState(1)
  const policyGroupShareRequestStatus =
    search.policy_group_share_request_status ?? ''
  const setPolicyGroupShareRequestStatus = (value: string) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        policy_group_share_request_status: value || undefined,
      }),
    })
  }
  const [
    policyGroupShareRequestPagination,
    setPolicyGroupShareRequestPagination,
  ] = useState({
    filterKey: policyGroupShareRequestStatus,
    page: 1,
  })
  const policyGroupShareRequestPage =
    policyGroupShareRequestPagination.filterKey ===
    policyGroupShareRequestStatus
      ? policyGroupShareRequestPagination.page
      : 1
  const setPolicyGroupShareRequestPage = (page: number) => {
    setPolicyGroupShareRequestPagination({
      filterKey: policyGroupShareRequestStatus,
      page,
    })
  }

  const [projectKeyword, setProjectKeyword] = useState('')
  const [projectStatus, setProjectStatus] = useState('')
  const [projectOrgUnitId, setProjectOrgUnitId] = useState('')
  const [projectPage, setProjectPage] = useState(1)

  const [quotaPolicyKeyword, setQuotaPolicyKeyword] = useState('')
  const [quotaPolicyStatus, setQuotaPolicyStatus] = useState('')
  const [quotaPolicyTargetType, setQuotaPolicyTargetType] = useState('')
  const [quotaPolicyMetric, setQuotaPolicyMetric] = useState('')
  const [quotaPolicyPage, setQuotaPolicyPage] = useState(1)

  const quotaRequestStatus = search.quota_request_status ?? ''
  const [quotaRequestPolicyId, setQuotaRequestPolicyId] = useState('')
  const quotaRequestId = search.quota_request_id
    ? String(search.quota_request_id)
    : ''
  const quotaRequestPolicyIdValue = quotaRequestPolicyId
    ? Number(quotaRequestPolicyId)
    : undefined
  const quotaRequestIdValue = search.quota_request_id
  const quotaRequestProjectId = search.quota_request_project_id
    ? String(search.quota_request_project_id)
    : ''
  const quotaRequestProjectIdValue = search.quota_request_project_id
  const quotaRequestTargetType = search.quota_request_target_type ?? ''
  const quotaRequestTargetId = search.quota_request_target_id
    ? String(search.quota_request_target_id)
    : ''
  const quotaRequestTargetIdValue = search.quota_request_target_id
  const quotaRequestApplicantUserId = search.quota_request_applicant_user_id
    ? String(search.quota_request_applicant_user_id)
    : ''
  const quotaRequestApplicantUserIdValue =
    search.quota_request_applicant_user_id
  const quotaRequestFilterKey = [
    quotaRequestStatus,
    quotaRequestPolicyIdValue ?? '',
    quotaRequestIdValue ?? '',
    quotaRequestProjectIdValue ?? '',
    quotaRequestTargetType,
    quotaRequestTargetIdValue ?? '',
    quotaRequestApplicantUserIdValue ?? '',
  ].join(':')
  const setQuotaRequestStatus = (value: string) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        quota_request_status: value || undefined,
      }),
    })
  }
  const setQuotaRequestId = (value: string) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        quota_request_id: parsePositiveSearchId(value),
      }),
    })
  }
  const setQuotaRequestProjectId = (value: string) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        quota_request_project_id: parsePositiveSearchId(value),
      }),
    })
  }
  const setQuotaRequestTargetType = (value: string) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        quota_request_target_type: value || undefined,
      }),
    })
  }
  const setQuotaRequestTargetId = (value: string) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        quota_request_target_id: parsePositiveSearchId(value),
      }),
    })
  }
  const setQuotaRequestApplicantUserId = (value: string) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        quota_request_applicant_user_id: parsePositiveSearchId(value),
      }),
    })
  }
  const [quotaRequestPagination, setQuotaRequestPagination] = useState({
    filterKey: quotaRequestFilterKey,
    page: 1,
  })
  const quotaRequestPage =
    quotaRequestPagination.filterKey === quotaRequestFilterKey
      ? quotaRequestPagination.page
      : 1
  const setQuotaRequestPage = (page: number) => {
    setQuotaRequestPagination({
      filterKey: quotaRequestFilterKey,
      page,
    })
  }

  const [usageStartDate, setUsageStartDate] = useState(() =>
    daysAgoInputValue(30)
  )
  const [usageEndDate, setUsageEndDate] = useState(() => todayInputValue())
  const [usageDimension, setUsageDimension] =
    useState<UsageDimension>('org_unit')
  const [usageGranularity, setUsageGranularity] =
    useState<UsageGranularity>('day')
  const [usageModelName, setUsageModelName] = useState('')
  const [usageProjectId, setUsageProjectId] = useState('')
  const [usageChannelId, setUsageChannelId] = useState('')
  const [usageTokenId, setUsageTokenId] = useState('')
  const [usageStatus, setUsageStatus] = useState('')
  const [usagePage, setUsagePage] = useState(1)

  const auditTargetType = search.audit_target_type ?? ''
  const auditTargetId = search.audit_target_id
    ? String(search.audit_target_id)
    : ''
  const auditAction = search.audit_action ?? ''
  const setAuditTargetType = (value: string) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        audit_target_type: value || undefined,
      }),
    })
  }
  const setAuditTargetId = (value: string) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        audit_target_id: parsePositiveSearchId(value),
      }),
    })
  }
  const setAuditAction = (value: string) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        audit_action: value || undefined,
      }),
    })
  }
  const [auditActorUserId, setAuditActorUserId] = useState('')
  const [auditRequestId, setAuditRequestId] = useState('')
  const [auditStartDate, setAuditStartDate] = useState('')
  const [auditEndDate, setAuditEndDate] = useState('')
  const viewQuotaRequestAudit = (request: EnterpriseQuotaRequest) => {
    setViewingQuotaRequest(null)
    setAuditActorUserId('')
    setAuditRequestId('')
    setAuditStartDate('')
    setAuditEndDate('')
    void navigate({
      search: (prev) => ({
        ...prev,
        tab: 'audit',
        audit_target_type: 'quota_request',
        audit_target_id: request.id,
        audit_action: undefined,
      }),
    })
  }
  const auditActorUserIdValue = auditActorUserId
    ? Number(auditActorUserId)
    : undefined
  const auditTargetIdValue = search.audit_target_id
  const auditFilterKey = [
    auditAction,
    auditTargetType,
    auditTargetIdValue ?? '',
    auditActorUserIdValue ?? '',
    auditRequestId,
    auditStartDate,
    auditEndDate,
  ].join(':')
  const [auditPagination, setAuditPagination] = useState({
    filterKey: auditFilterKey,
    page: 1,
  })
  const auditPage =
    auditPagination.filterKey === auditFilterKey ? auditPagination.page : 1
  const setAuditPage = (page: number) => {
    setAuditPagination({
      filterKey: auditFilterKey,
      page,
    })
  }

  const currentQuery = useQuery({
    queryKey: ['enterprise', 'current'],
    queryFn: getEnterpriseCurrent,
    enabled: canReadEnterprise,
  })
  const orgUnitsQuery = useQuery({
    queryKey: ['enterprise', 'org-units', 'all'],
    queryFn: () => getEnterpriseOrgUnits(),
    enabled: canReadEnterprise,
  })
  const membersSummaryQuery = useQuery({
    queryKey: ['enterprise', 'members', 'summary'],
    queryFn: () => getEnterpriseMembers({ p: 1, page_size: 1 }),
    enabled: canManageDepartment,
  })
  const memberOrgUnitId =
    memberOrgFilter && memberOrgFilter !== UNASSIGNED_VALUE
      ? Number(memberOrgFilter)
      : undefined
  const membersQuery = useQuery({
    queryKey: [
      'enterprise',
      'members',
      memberPage,
      memberKeyword,
      memberOrgUnitId,
      memberOrgFilter === UNASSIGNED_VALUE,
    ],
    queryFn: () =>
      getEnterpriseMembers({
        p: memberPage,
        page_size: PAGE_SIZE,
        keyword: memberKeyword,
        org_unit_id: memberOrgUnitId,
        unassigned: memberOrgFilter === UNASSIGNED_VALUE,
      }),
    enabled: canManageDepartment,
  })
  const policyGroupsSummaryQuery = useQuery({
    queryKey: ['enterprise', 'policy-groups', 'summary'],
    queryFn: () => getEnterprisePolicyGroups({ p: 1, page_size: 1 }),
    enabled: canManageDepartment,
  })
  const allPolicyGroupsQuery = useQuery({
    queryKey: ['enterprise', 'policy-groups', 'all'],
    queryFn: () =>
      getEnterprisePolicyGroups({
        p: 1,
        page_size: 100,
        status: ENABLED_STATUS,
      }),
    enabled: canManageDepartment,
  })
  const policyGroupsQuery = useQuery({
    queryKey: [
      'enterprise',
      'policy-groups',
      policyGroupPage,
      policyGroupKeyword,
      policyGroupStatus,
    ],
    queryFn: () =>
      getEnterprisePolicyGroups({
        p: policyGroupPage,
        page_size: PAGE_SIZE,
        keyword: policyGroupKeyword,
        status: policyGroupStatus,
      }),
    enabled: canManageDepartment,
  })
  const policyGroupShareRequestsQuery = useQuery({
    queryKey: [
      'enterprise',
      'policy-group-share-requests',
      policyGroupShareRequestPage,
      policyGroupShareRequestStatus,
    ],
    queryFn: () =>
      getEnterprisePolicyGroupShareRequests({
        p: policyGroupShareRequestPage,
        page_size: PAGE_SIZE,
        status: policyGroupShareRequestStatus,
      }),
    enabled: canManageDepartment,
  })
  const allProjectsQuery = useQuery({
    queryKey: ['enterprise', 'projects', 'all'],
    queryFn: () =>
      getEnterpriseProjects({
        p: 1,
        page_size: 100,
        status: ENABLED_STATUS,
      }),
    enabled: canReadEnterprise,
  })
  const projectOrgUnitIdValue = projectOrgUnitId
    ? Number(projectOrgUnitId)
    : undefined
  const projectsQuery = useQuery({
    queryKey: [
      'enterprise',
      'projects',
      projectPage,
      projectKeyword,
      projectStatus,
      projectOrgUnitIdValue,
    ],
    queryFn: () =>
      getEnterpriseProjects({
        p: projectPage,
        page_size: PAGE_SIZE,
        keyword: projectKeyword,
        status: projectStatus,
        org_unit_id: projectOrgUnitIdValue,
      }),
    enabled: canReadProjects,
  })
  const quotaPoliciesSummaryQuery = useQuery({
    queryKey: ['enterprise', 'quota-policies', 'summary'],
    queryFn: () => getEnterpriseQuotaPolicies({ p: 1, page_size: 1 }),
    enabled: canManageDepartment,
  })
  const quotaPoliciesQuery = useQuery({
    queryKey: [
      'enterprise',
      'quota-policies',
      quotaPolicyPage,
      quotaPolicyKeyword,
      quotaPolicyStatus,
      quotaPolicyTargetType,
      quotaPolicyMetric,
    ],
    queryFn: () =>
      getEnterpriseQuotaPolicies({
        p: quotaPolicyPage,
        page_size: PAGE_SIZE,
        keyword: quotaPolicyKeyword,
        status: quotaPolicyStatus,
        target_type: quotaPolicyTargetType,
        metric: quotaPolicyMetric,
      }),
    enabled: canManageDepartment,
  })
  const quotaRequestsQuery = useQuery({
    queryKey: [
      'enterprise',
      'quota-requests',
      quotaRequestPage,
      quotaRequestStatus,
      quotaRequestPolicyIdValue,
      quotaRequestIdValue,
      quotaRequestProjectIdValue,
      quotaRequestTargetType,
      quotaRequestTargetIdValue,
      quotaRequestApplicantUserIdValue,
    ],
    queryFn: () =>
      getEnterpriseQuotaRequests({
        p: quotaRequestPage,
        page_size: PAGE_SIZE,
        id: quotaRequestIdValue,
        status: quotaRequestStatus,
        policy_id: quotaRequestPolicyIdValue,
        project_id: quotaRequestProjectIdValue,
        target_type: quotaRequestTargetType,
        target_id: quotaRequestTargetIdValue,
        applicant_user_id: quotaRequestApplicantUserIdValue,
      }),
    enabled: canReadEnterprise,
  })

  const usageStartTime = startOfDayUnix(usageStartDate)
  const usageEndTime = endOfDayUnix(usageEndDate)
  const usageProjectIdValue = usageProjectId
    ? Number(usageProjectId)
    : undefined
  const usageChannelIdValue = usageChannelId
    ? Number(usageChannelId)
    : undefined
  const usageTokenIdValue = usageTokenId ? Number(usageTokenId) : undefined
  const usageSummaryQuery = useQuery({
    queryKey: [
      'enterprise',
      'usage',
      'summary',
      usageStartTime,
      usageEndTime,
      usageModelName,
      usageProjectIdValue,
      usageChannelIdValue,
      usageTokenIdValue,
      usageStatus,
    ],
    queryFn: () =>
      getEnterpriseUsageSummary({
        start_time: usageStartTime,
        end_time: usageEndTime,
        model_name: usageModelName,
        project_id: usageProjectIdValue,
        channel_id: usageChannelIdValue,
        token_id: usageTokenIdValue,
        status: usageStatus,
      }),
    enabled: canReadFinance,
  })
  const usageBreakdownQuery = useQuery({
    queryKey: [
      'enterprise',
      'usage',
      'breakdown',
      usagePage,
      usageStartTime,
      usageEndTime,
      usageDimension,
      usageGranularity,
      usageModelName,
      usageProjectIdValue,
      usageChannelIdValue,
      usageTokenIdValue,
      usageStatus,
    ],
    queryFn: () =>
      getEnterpriseUsageBreakdown({
        p: usagePage,
        page_size: PAGE_SIZE,
        start_time: usageStartTime,
        end_time: usageEndTime,
        dimension: usageDimension,
        granularity: usageGranularity,
        model_name: usageModelName,
        project_id: usageProjectIdValue,
        channel_id: usageChannelIdValue,
        token_id: usageTokenIdValue,
        status: usageStatus,
        sort_by: 'quota',
        sort_order: 'desc',
      }),
    enabled: canReadFinance,
  })
  const exportUsageMutation = useMutation({
    mutationFn: () =>
      downloadEnterpriseUsageBreakdownExport({
        start_time: usageStartTime,
        end_time: usageEndTime,
        dimension: usageDimension,
        granularity: usageGranularity,
        model_name: usageModelName,
        project_id: usageProjectIdValue,
        channel_id: usageChannelIdValue,
        token_id: usageTokenIdValue,
        status: usageStatus,
        sort_by: 'quota',
        sort_order: 'desc',
      }),
    onSuccess: (blob) => {
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `enterprise-usage-${usageDimension}-${usageStartDate}-${usageEndDate}.csv`
      document.body.appendChild(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(url)
      toast.success(t('Exported'))
    },
  })
  const auditStartTime = auditStartDate
    ? startOfDayUnix(auditStartDate)
    : undefined
  const auditEndTime = auditEndDate ? endOfDayUnix(auditEndDate) : undefined
  const auditLogsQuery = useQuery({
    queryKey: [
      'enterprise',
      'audit-logs',
      auditPage,
      auditAction,
      auditTargetType,
      auditTargetIdValue,
      auditActorUserIdValue,
      auditRequestId,
      auditStartTime,
      auditEndTime,
    ],
    queryFn: () =>
      getEnterpriseAuditLogs({
        p: auditPage,
        page_size: PAGE_SIZE,
        action: auditAction,
        target_type: auditTargetType,
        target_id: auditTargetIdValue,
        actor_user_id: auditActorUserIdValue,
        request_id: auditRequestId,
        start_time: auditStartTime,
        end_time: auditEndTime,
      }),
    enabled: canReadAudit,
  })
  const dryRunObservationStartTime = startOfDayUnix(daysAgoInputValue(7))
  const dryRunObservationEndTime = endOfDayUnix(todayInputValue())
  const dryRunObservationsQuery = useQuery({
    queryKey: [
      'enterprise',
      'audit-logs',
      'dry-run-observations',
      dryRunObservationStartTime,
      dryRunObservationEndTime,
    ],
    queryFn: () =>
      getEnterpriseAuditLogs({
        p: 1,
        page_size: 5,
        action: 'enterprise_governance.dry_run_reject',
        target_type: 'relay_request',
        start_time: dryRunObservationStartTime,
        end_time: dryRunObservationEndTime,
      }),
    enabled: canReadAudit,
  })
  const queueAdmissionsQuery = useQuery({
    queryKey: [
      'enterprise',
      'queue-admissions',
      dryRunObservationStartTime,
      dryRunObservationEndTime,
    ],
    queryFn: () =>
      getEnterpriseQueueAdmissions({
        p: 1,
        page_size: 10,
        start_time: dryRunObservationStartTime,
        end_time: dryRunObservationEndTime,
      }),
    enabled: canReadAudit,
  })

  const enterprise = currentQuery.data?.data
  const orgUnits = orgUnitsQuery.data?.data ?? EMPTY_ORG_UNITS
  const orgOptions = useMemo(() => buildOrgOptions(orgUnits), [orgUnits])
  const members = getPageItems(membersQuery.data)
  const policyGroups = getPageItems(policyGroupsQuery.data)
  const policyGroupShareRequests = getPageItems(
    policyGroupShareRequestsQuery.data
  )
  const allPolicyGroups = getPageItems(allPolicyGroupsQuery.data)
  const projects = getPageItems(projectsQuery.data)
  const allProjects = getPageItems(allProjectsQuery.data)
  const quotaPolicies = getPageItems(quotaPoliciesQuery.data)
  const quotaRequests = getPageItems(quotaRequestsQuery.data)
  const usageSummary = usageSummaryQuery.data?.data
  const usageBreakdown = getPageItems(usageBreakdownQuery.data)
  const auditLogs = getPageItems(auditLogsQuery.data)
  const dryRunObservations = getPageItems(dryRunObservationsQuery.data)
  const queueAdmissions = getPageItems(queueAdmissionsQuery.data)

  const disableOrgMutation = useMutation({
    mutationFn: disableEnterpriseOrgUnit,
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Disabled'))
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })
  const disablePolicyGroupMutation = useMutation({
    mutationFn: disableEnterprisePolicyGroup,
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Disabled'))
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })
  const disableProjectMutation = useMutation({
    mutationFn: disableEnterpriseProject,
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Disabled'))
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })
  const disableQuotaPolicyMutation = useMutation({
    mutationFn: disableEnterpriseQuotaPolicy,
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Disabled'))
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
    },
  })
  const withdrawQuotaRequestMutation = useMutation({
    mutationFn: withdrawEnterpriseQuotaRequest,
    onSuccess: (response) => {
      if (!response.success) return
      toast.success(t('Withdrawn'))
      queryClient.invalidateQueries({ queryKey: ['enterprise'] })
      queryClient.invalidateQueries({
        queryKey: ['notifications', 'enterprise-quota-requests'],
      })
    },
  })

  const refreshAll = () => {
    queryClient.invalidateQueries({ queryKey: ['enterprise'] })
  }

  const confirmDisable = (label: string) =>
    window.confirm(t('Disable {{label}}?', { label }))

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Enterprise Governance')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button variant='outline' size='sm' onClick={refreshAll}>
            <RefreshCcw className='size-3.5' />
            {t('Refresh')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-3'>
            <EnterpriseHeader
              enterprise={enterprise}
              isLoading={currentQuery.isLoading}
              membersTotal={getPageTotal(membersSummaryQuery.data)}
              orgUnitsTotal={orgUnits.length}
              policyGroupsTotal={getPageTotal(policyGroupsSummaryQuery.data)}
              quotaPoliciesTotal={getPageTotal(quotaPoliciesSummaryQuery.data)}
              canManage={canManageEnterprise}
              onEdit={() => setEnterpriseDialogOpen(true)}
            />

            <Tabs
              value={activeTab}
              onValueChange={(value) =>
                setActiveTab(normalizeSelectValue(value) as EnterpriseTab)
              }
            >
              <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
                {availableTabs.map((tab) => {
                  const Icon = tab.icon
                  return (
                    <TabsTrigger key={tab.value} value={tab.value}>
                      <Icon className='size-3.5' />
                      {t(tab.label)}
                    </TabsTrigger>
                  )
                })}
              </TabsList>

              <TabsContent value='overview'>
                <OverviewTab
                  enterprise={enterprise}
                  usage={usageSummary}
                  orgUnits={orgUnits}
                  membersTotal={getPageTotal(membersSummaryQuery.data)}
                  policyGroupsTotal={getPageTotal(
                    policyGroupsSummaryQuery.data
                  )}
                  quotaPoliciesTotal={getPageTotal(
                    quotaPoliciesSummaryQuery.data
                  )}
                  usageQuery={{
                    data: usageSummaryQuery.data,
                    isLoading: usageSummaryQuery.isLoading,
                    isError: usageSummaryQuery.isError,
                    error: usageSummaryQuery.error,
                    refetch: usageSummaryQuery.refetch,
                  }}
                />
              </TabsContent>

              <TabsContent value='organization'>
                <OrganizationTab
                  orgUnits={orgUnits}
                  orgUnitsQuery={{
                    data: orgUnitsQuery.data,
                    isLoading: orgUnitsQuery.isLoading,
                    isError: orgUnitsQuery.isError,
                    error: orgUnitsQuery.error,
                    refetch: orgUnitsQuery.refetch,
                  }}
                  members={members}
                  membersQuery={{
                    data: membersQuery.data,
                    isLoading: membersQuery.isLoading,
                    isError: membersQuery.isError,
                    error: membersQuery.error,
                    refetch: membersQuery.refetch,
                  }}
                  membersTotal={getPageTotal(membersQuery.data)}
                  memberPage={memberPage}
                  setMemberPage={setMemberPage}
                  orgKeyword={orgKeyword}
                  setOrgKeyword={setOrgKeyword}
                  orgStatus={orgStatus}
                  setOrgStatus={setOrgStatus}
                  memberKeyword={memberKeyword}
                  setMemberKeyword={setMemberKeyword}
                  memberOrgFilter={memberOrgFilter}
                  setMemberOrgFilter={setMemberOrgFilter}
                  onCreateOrgUnit={() => {
                    setEditingOrgUnit(null)
                    setOrgDialogOpen(true)
                  }}
                  onEditOrgUnit={(unit) => {
                    setEditingOrgUnit(unit)
                    setOrgDialogOpen(true)
                  }}
                  onDisableOrgUnit={(unit) => {
                    if (confirmDisable(unit.name))
                      disableOrgMutation.mutate(unit.id)
                  }}
                  orgOptions={orgOptions}
                  canManageEnterprise={canManageEnterprise}
                />
              </TabsContent>

              <TabsContent value='projects'>
                <ProjectsTab
                  projects={projects}
                  orgOptions={orgOptions}
                  query={{
                    data: projectsQuery.data,
                    isLoading: projectsQuery.isLoading,
                    isError: projectsQuery.isError,
                    error: projectsQuery.error,
                    refetch: projectsQuery.refetch,
                  }}
                  page={projectPage}
                  total={getPageTotal(projectsQuery.data)}
                  setPage={setProjectPage}
                  keyword={projectKeyword}
                  setKeyword={setProjectKeyword}
                  status={projectStatus}
                  setStatus={setProjectStatus}
                  orgUnitId={projectOrgUnitId}
                  setOrgUnitId={setProjectOrgUnitId}
                  onCreate={() => {
                    setEditingProject(null)
                    setProjectDialogOpen(true)
                  }}
                  onEdit={(project) => {
                    setEditingProject(project)
                    setProjectDialogOpen(true)
                  }}
                  onManageMembers={(project) => {
                    setMemberProject(project)
                    setProjectMembersDialogOpen(true)
                  }}
                  onDisable={(project) => {
                    disableProjectMutation.mutate(project.id)
                  }}
                  onViewUsage={(project) => {
                    setUsageProjectId(String(project.id))
                    setUsageDimension('project')
                    setUsagePage(1)
                    setActiveTab('usage')
                  }}
                  canCreateProject={canManageProjects}
                />
              </TabsContent>

              <TabsContent value='policy-groups'>
                <PolicyGroupsTab
                  groups={policyGroups}
                  query={{
                    data: policyGroupsQuery.data,
                    isLoading: policyGroupsQuery.isLoading,
                    isError: policyGroupsQuery.isError,
                    error: policyGroupsQuery.error,
                    refetch: policyGroupsQuery.refetch,
                  }}
                  shareRequests={policyGroupShareRequests}
                  shareRequestsQuery={{
                    data: policyGroupShareRequestsQuery.data,
                    isLoading: policyGroupShareRequestsQuery.isLoading,
                    isError: policyGroupShareRequestsQuery.isError,
                    error: policyGroupShareRequestsQuery.error,
                    refetch: policyGroupShareRequestsQuery.refetch,
                  }}
                  page={policyGroupPage}
                  total={getPageTotal(policyGroupsQuery.data)}
                  setPage={setPolicyGroupPage}
                  shareRequestPage={policyGroupShareRequestPage}
                  shareRequestTotal={getPageTotal(
                    policyGroupShareRequestsQuery.data
                  )}
                  setShareRequestPage={setPolicyGroupShareRequestPage}
                  shareRequestStatus={policyGroupShareRequestStatus}
                  setShareRequestStatus={setPolicyGroupShareRequestStatus}
                  keyword={policyGroupKeyword}
                  setKeyword={setPolicyGroupKeyword}
                  status={policyGroupStatus}
                  setStatus={setPolicyGroupStatus}
                  onCreate={() => {
                    setEditingPolicyGroup(null)
                    setPolicyGroupDialogOpen(true)
                  }}
                  onEdit={(group) => {
                    setEditingPolicyGroup(group)
                    setPolicyGroupDialogOpen(true)
                  }}
                  onDisable={(group) => {
                    if (confirmDisable(group.name)) {
                      disablePolicyGroupMutation.mutate(group.id)
                    }
                  }}
                  onManageMembers={(group) => {
                    setMemberPolicyGroup(group)
                    setPolicyGroupMembersDialogOpen(true)
                  }}
                  onCreateShareRequest={(group) => {
                    setSharingPolicyGroup(group)
                    setPolicyGroupShareRequestDialogOpen(true)
                  }}
                  onDecideShareRequest={(request, action) => {
                    setPolicyGroupShareRequestDecisionAction(action)
                    setDecidingPolicyGroupShareRequest(request)
                  }}
                />
              </TabsContent>

              <TabsContent value='quota-policies'>
                <QuotaPoliciesTab
                  policies={quotaPolicies}
                  orgUnits={orgUnits}
                  projects={allProjects}
                  membersTotal={getPageTotal(membersSummaryQuery.data)}
                  policyGroupsTotal={getPageTotal(
                    policyGroupsSummaryQuery.data
                  )}
                  dryRunObservations={dryRunObservations}
                  query={{
                    data: quotaPoliciesQuery.data,
                    isLoading: quotaPoliciesQuery.isLoading,
                    isError: quotaPoliciesQuery.isError,
                    error: quotaPoliciesQuery.error,
                    refetch: quotaPoliciesQuery.refetch,
                  }}
                  page={quotaPolicyPage}
                  total={getPageTotal(quotaPoliciesQuery.data)}
                  setPage={setQuotaPolicyPage}
                  keyword={quotaPolicyKeyword}
                  setKeyword={setQuotaPolicyKeyword}
                  status={quotaPolicyStatus}
                  setStatus={setQuotaPolicyStatus}
                  targetType={quotaPolicyTargetType}
                  setTargetType={setQuotaPolicyTargetType}
                  metric={quotaPolicyMetric}
                  setMetric={setQuotaPolicyMetric}
                  onCreate={() => {
                    setEditingQuotaPolicy(null)
                    setQuotaPolicyDialogOpen(true)
                  }}
                  onEdit={(policy) => {
                    setEditingQuotaPolicy(policy)
                    setQuotaPolicyDialogOpen(true)
                  }}
                  onDisable={(policy) => {
                    disableQuotaPolicyMutation.mutate(policy.id)
                  }}
                />
              </TabsContent>

              <TabsContent value='quota-requests'>
                <QuotaRequestsTab
                  requests={quotaRequests}
                  policies={quotaPolicies}
                  currentUserId={currentUserId}
                  isAdmin={canApproveQuota}
                  query={{
                    data: quotaRequestsQuery.data,
                    isLoading: quotaRequestsQuery.isLoading,
                    isError: quotaRequestsQuery.isError,
                    error: quotaRequestsQuery.error,
                    refetch: quotaRequestsQuery.refetch,
                  }}
                  page={quotaRequestPage}
                  total={getPageTotal(quotaRequestsQuery.data)}
                  setPage={setQuotaRequestPage}
                  status={quotaRequestStatus}
                  setStatus={setQuotaRequestStatus}
                  requestId={quotaRequestId}
                  setRequestId={setQuotaRequestId}
                  policyId={quotaRequestPolicyId}
                  setPolicyId={setQuotaRequestPolicyId}
                  projectId={quotaRequestProjectId}
                  setProjectId={setQuotaRequestProjectId}
                  targetType={quotaRequestTargetType}
                  setTargetType={setQuotaRequestTargetType}
                  targetId={quotaRequestTargetId}
                  setTargetId={setQuotaRequestTargetId}
                  applicantUserId={quotaRequestApplicantUserId}
                  setApplicantUserId={setQuotaRequestApplicantUserId}
                  selectedRequestIds={selectedQuotaRequestIds}
                  setSelectedRequestIds={setSelectedQuotaRequestIds}
                  onCreate={() => {
                    setQuotaRequestInitialValues(null)
                    setQuotaRequestDialogOpen(true)
                  }}
                  onApprove={(request) => {
                    setQuotaRequestDecisionAction('approve')
                    setDecidingQuotaRequest(request)
                  }}
                  onReject={(request) => {
                    setQuotaRequestDecisionAction('reject')
                    setDecidingQuotaRequest(request)
                  }}
                  onBatchApprove={(ids) => {
                    setQuotaRequestDecisionAction('approve')
                    setBatchDecidingQuotaRequestIds(ids)
                  }}
                  onBatchReject={(ids) => {
                    setQuotaRequestDecisionAction('reject')
                    setBatchDecidingQuotaRequestIds(ids)
                  }}
                  onWithdraw={(request) => {
                    withdrawQuotaRequestMutation.mutate(request.id)
                  }}
                  onViewDetails={setViewingQuotaRequest}
                />
              </TabsContent>

              <TabsContent value='notifications'>
                <NotificationsTab />
              </TabsContent>

              <TabsContent value='webhooks'>
                <WebhooksTab />
              </TabsContent>

              <TabsContent value='deliveries'>
                <DeliveriesTab />
              </TabsContent>

              <TabsContent value='usage'>
                <UsageTab
                  summary={usageSummary}
                  projects={allProjects}
                  summaryQuery={{
                    data: usageSummaryQuery.data,
                    isLoading: usageSummaryQuery.isLoading,
                    isError: usageSummaryQuery.isError,
                    error: usageSummaryQuery.error,
                    refetch: usageSummaryQuery.refetch,
                  }}
                  breakdown={usageBreakdown}
                  breakdownQuery={{
                    data: usageBreakdownQuery.data,
                    isLoading: usageBreakdownQuery.isLoading,
                    isError: usageBreakdownQuery.isError,
                    error: usageBreakdownQuery.error,
                    refetch: usageBreakdownQuery.refetch,
                  }}
                  breakdownTotal={getPageTotal(usageBreakdownQuery.data)}
                  page={usagePage}
                  setPage={setUsagePage}
                  startDate={usageStartDate}
                  setStartDate={setUsageStartDate}
                  endDate={usageEndDate}
                  setEndDate={setUsageEndDate}
                  dimension={usageDimension}
                  setDimension={setUsageDimension}
                  granularity={usageGranularity}
                  setGranularity={setUsageGranularity}
                  modelName={usageModelName}
                  setModelName={setUsageModelName}
                  projectId={usageProjectId}
                  setProjectId={setUsageProjectId}
                  channelId={usageChannelId}
                  setChannelId={setUsageChannelId}
                  tokenId={usageTokenId}
                  setTokenId={setUsageTokenId}
                  status={usageStatus}
                  setStatus={setUsageStatus}
                  onExport={() => exportUsageMutation.mutate()}
                  isExporting={exportUsageMutation.isPending}
                />
              </TabsContent>

              <TabsContent value='audit'>
                <AuditTab
                  logs={auditLogs}
                  dryRunObservations={dryRunObservations}
                  queueAdmissions={queueAdmissions}
                  query={{
                    data: auditLogsQuery.data,
                    isLoading: auditLogsQuery.isLoading,
                    isError: auditLogsQuery.isError,
                    error: auditLogsQuery.error,
                    refetch: auditLogsQuery.refetch,
                  }}
                  dryRunQuery={{
                    data: dryRunObservationsQuery.data,
                    isLoading: dryRunObservationsQuery.isLoading,
                    isError: dryRunObservationsQuery.isError,
                    error: dryRunObservationsQuery.error,
                    refetch: dryRunObservationsQuery.refetch,
                  }}
                  queueAdmissionsQuery={{
                    data: queueAdmissionsQuery.data,
                    isLoading: queueAdmissionsQuery.isLoading,
                    isError: queueAdmissionsQuery.isError,
                    error: queueAdmissionsQuery.error,
                    refetch: queueAdmissionsQuery.refetch,
                  }}
                  page={auditPage}
                  total={getPageTotal(auditLogsQuery.data)}
                  setPage={setAuditPage}
                  action={auditAction}
                  setAction={setAuditAction}
                  targetType={auditTargetType}
                  setTargetType={setAuditTargetType}
                  targetId={auditTargetId}
                  setTargetId={setAuditTargetId}
                  actorUserId={auditActorUserId}
                  setActorUserId={setAuditActorUserId}
                  requestId={auditRequestId}
                  setRequestId={setAuditRequestId}
                  startDate={auditStartDate}
                  setStartDate={setAuditStartDate}
                  endDate={auditEndDate}
                  setEndDate={setAuditEndDate}
                  onRequestQuota={(initialValues) => {
                    setQuotaRequestInitialValues(initialValues)
                    setQuotaRequestDialogOpen(true)
                    setActiveTab('quota-requests')
                  }}
                  canManageQueue={canManageEnterprise}
                />
              </TabsContent>
            </Tabs>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {enterpriseDialogOpen ? (
        <EnterpriseDialog
          key={`enterprise:${enterprise?.id ?? 'new'}`}
          open={enterpriseDialogOpen}
          enterprise={enterprise}
          onOpenChange={setEnterpriseDialogOpen}
        />
      ) : null}
      {orgDialogOpen ? (
        <OrgUnitDialog
          key={`org:${editingOrgUnit?.id ?? 'new'}`}
          open={orgDialogOpen}
          unit={editingOrgUnit}
          orgUnits={orgUnits}
          onOpenChange={setOrgDialogOpen}
        />
      ) : null}
      {policyGroupDialogOpen ? (
        <PolicyGroupDialog
          key={`policy-group:${editingPolicyGroup?.id ?? 'new'}`}
          open={policyGroupDialogOpen}
          group={editingPolicyGroup}
          orgUnits={orgUnits}
          onOpenChange={setPolicyGroupDialogOpen}
        />
      ) : null}
      {policyGroupMembersDialogOpen ? (
        <PolicyGroupMembersDialog
          key={`policy-group-members:${memberPolicyGroup?.id ?? 'none'}`}
          open={policyGroupMembersDialogOpen}
          group={memberPolicyGroup}
          onOpenChange={setPolicyGroupMembersDialogOpen}
        />
      ) : null}
      {policyGroupShareRequestDialogOpen ? (
        <PolicyGroupShareRequestDialog
          key={`policy-group-share-request:${sharingPolicyGroup?.id ?? 'none'}`}
          open={policyGroupShareRequestDialogOpen}
          group={sharingPolicyGroup}
          orgUnits={orgUnits}
          onOpenChange={(open) => {
            setPolicyGroupShareRequestDialogOpen(open)
            if (!open) setSharingPolicyGroup(null)
          }}
        />
      ) : null}
      {decidingPolicyGroupShareRequest ? (
        <PolicyGroupShareRequestDecisionDialog
          key={`policy-group-share-request-decision:${decidingPolicyGroupShareRequest.id}:${policyGroupShareRequestDecisionAction}`}
          request={decidingPolicyGroupShareRequest}
          action={policyGroupShareRequestDecisionAction}
          onOpenChange={(open) => {
            if (!open) setDecidingPolicyGroupShareRequest(null)
          }}
        />
      ) : null}
      {projectDialogOpen ? (
        <ProjectDialog
          key={`project:${editingProject?.id ?? 'new'}`}
          open={projectDialogOpen}
          project={editingProject}
          orgUnits={orgUnits}
          onOpenChange={setProjectDialogOpen}
        />
      ) : null}
      {projectMembersDialogOpen ? (
        <ProjectMembersDialog
          key={`project-members:${memberProject?.id ?? 'none'}`}
          open={projectMembersDialogOpen}
          project={memberProject}
          onOpenChange={setProjectMembersDialogOpen}
          canManage={memberProject?.can_manage === true}
        />
      ) : null}
      {quotaPolicyDialogOpen ? (
        <QuotaPolicyDialog
          key={`quota-policy:${editingQuotaPolicy?.id ?? 'new'}`}
          open={quotaPolicyDialogOpen}
          policy={editingQuotaPolicy}
          orgUnits={orgUnits}
          projects={allProjects}
          policyGroups={allPolicyGroups}
          dryRunObservations={dryRunObservations}
          onOpenChange={setQuotaPolicyDialogOpen}
        />
      ) : null}
      {quotaRequestDialogOpen ? (
        <QuotaRequestDialog
          key={`quota-request:${quotaRequestInitialValues?.policyId ?? 'new'}:${quotaRequestInitialValues?.limitDelta ?? ''}:${quotaRequestInitialValues?.reason ?? ''}`}
          open={quotaRequestDialogOpen}
          policies={quotaPolicies}
          initialValues={quotaRequestInitialValues}
          onOpenChange={setQuotaRequestDialogOpen}
        />
      ) : null}
      {decidingQuotaRequest ? (
        <QuotaRequestDecisionDialog
          key={`quota-request-decision:${decidingQuotaRequest.id}:${quotaRequestDecisionAction}`}
          request={decidingQuotaRequest}
          action={quotaRequestDecisionAction}
          onOpenChange={(open) => {
            if (!open) setDecidingQuotaRequest(null)
          }}
        />
      ) : null}
      {batchDecidingQuotaRequestIds.length > 0 ? (
        <QuotaRequestBatchDecisionDialog
          key={`quota-request-batch-decision:${quotaRequestDecisionAction}:${batchDecidingQuotaRequestIds.join(',')}`}
          ids={batchDecidingQuotaRequestIds}
          action={quotaRequestDecisionAction}
          onOpenChange={(open) => {
            if (open) return
            setSelectedQuotaRequestIds((ids) =>
              ids.filter((id) => !batchDecidingQuotaRequestIds.includes(id))
            )
            setBatchDecidingQuotaRequestIds([])
          }}
        />
      ) : null}
      <QuotaRequestDetailSheet
        open={Boolean(viewingQuotaRequest)}
        request={viewingQuotaRequest}
        canViewAudit={canReadAudit}
        onViewAudit={canReadAudit ? viewQuotaRequestAudit : undefined}
        onOpenChange={(open) => {
          if (!open) setViewingQuotaRequest(null)
        }}
      />
    </>
  )
}

export { EnterpriseGovernance as Enterprise }
