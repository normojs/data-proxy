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
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Route } from '@/routes/_authenticated/system-settings/operations/$section'
import {
  BellRing,
  Check,
  ClipboardList,
  Edit3,
  FileClock,
  Plus,
  RefreshCw,
  ShieldAlert,
  ShieldCheck,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
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
import { StatusBadge } from '@/components/status-badge'
import {
  SettingsPageActionsPortal,
  SettingsPageTitleStatusPortal,
} from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { ConnectedAppNotificationsSection } from './connected-app-notifications-section'
import {
  CONNECTED_APP_STATUS_DISABLED,
  CONNECTED_APP_STATUS_ENABLED,
  type ConnectedAppAuditLog,
  type ConnectedApp,
  type ConnectedAppPayload,
  type ConnectedAppRequest,
  type ConnectedAppReviewDecision,
  type ConnectedAppReviewPayload,
  type ConnectedAppStatus,
  createConnectedApp,
  listConnectedAppAuditLogs,
  listConnectedApps,
  listConnectedAppRequests,
  reviewConnectedAppRequest,
  updateConnectedApp,
} from './connected-apps-api'

const connectedAppsQueryKey = ['connected-apps']
const connectedAppRequestsQueryKey = ['connected-app-requests']
const connectedAppAuditLogsQueryKey = ['connected-app-audit-logs']
const scopePattern = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/
const connectedAppTabs = ['apps', 'requests', 'notifications', 'audit'] as const

type ConnectedAppsTab = (typeof connectedAppTabs)[number]

const connectedAppFormSchema = z
  .object({
    slug: z
      .string()
      .trim()
      .min(1, 'Slug is required')
      .regex(
        /^[a-z0-9][a-z0-9_-]{0,63}$/,
        'Use lowercase letters, numbers, underscores or hyphens'
      ),
    name: z.string().trim().min(1, 'Name is required').max(128),
    description: z.string().trim().max(512),
    allowedScopesText: z.string(),
    defaultScopesText: z.string(),
    trusted: z.boolean(),
    enabled: z.boolean(),
  })
  .superRefine((values, ctx) => {
    const allowedScopes = parseScopesText(values.allowedScopesText)
    const defaultScopes = parseScopesText(values.defaultScopesText)

    if (allowedScopes.length === 0) {
      ctx.addIssue({
        code: 'custom',
        path: ['allowedScopesText'],
        message: 'Add at least one allowed scope',
      })
    }

    for (const scope of [...allowedScopes, ...defaultScopes]) {
      if (!scopePattern.test(scope)) {
        ctx.addIssue({
          code: 'custom',
          path: ['allowedScopesText'],
          message: `Invalid scope: ${scope}`,
        })
        break
      }
    }

    const allowedSet = new Set(allowedScopes)
    for (const scope of defaultScopes) {
      if (!allowedSet.has(scope)) {
        ctx.addIssue({
          code: 'custom',
          path: ['defaultScopesText'],
          message: `Default scope is not allowed: ${scope}`,
        })
        break
      }
    }
  })

const connectedAppReviewSchema = z
  .object({
    decision: z.enum(['approved', 'rejected']),
    reviewNote: z.string().trim().max(1024),
    name: z.string().trim().max(128),
    description: z.string().trim().max(512),
    allowedScopesText: z.string(),
    defaultScopesText: z.string(),
  })
  .superRefine((values, ctx) => {
    if (values.decision === 'rejected') return

    if (!values.name.trim()) {
      ctx.addIssue({
        code: 'custom',
        path: ['name'],
        message: 'Name is required',
      })
    }

    const allowedScopes = parseScopesText(values.allowedScopesText)
    const defaultScopes = parseScopesText(values.defaultScopesText)

    if (allowedScopes.length === 0) {
      ctx.addIssue({
        code: 'custom',
        path: ['allowedScopesText'],
        message: 'Add at least one allowed scope',
      })
    }

    for (const scope of [...allowedScopes, ...defaultScopes]) {
      if (!scopePattern.test(scope)) {
        ctx.addIssue({
          code: 'custom',
          path: ['allowedScopesText'],
          message: `Invalid scope: ${scope}`,
        })
        break
      }
    }

    const allowedSet = new Set(allowedScopes)
    for (const scope of defaultScopes) {
      if (!allowedSet.has(scope)) {
        ctx.addIssue({
          code: 'custom',
          path: ['defaultScopesText'],
          message: `Default scope is not allowed: ${scope}`,
        })
        break
      }
    }
  })

type ConnectedAppFormValues = z.infer<typeof connectedAppFormSchema>
type ConnectedAppReviewFormValues = z.infer<typeof connectedAppReviewSchema>

function isConnectedAppsTab(value: unknown): value is ConnectedAppsTab {
  return typeof value === 'string' && connectedAppTabs.includes(value as never)
}

function parseScopesText(value: string) {
  const seen = new Set<string>()
  const scopes: string[] = []
  for (const raw of value.split(/[\s,]+/)) {
    const scope = raw.trim()
    if (!scope || seen.has(scope)) continue
    seen.add(scope)
    scopes.push(scope)
  }
  return scopes
}

function scopesToText(scopes: string[]) {
  return scopes.join('\n')
}

function buildFormDefaults(app: ConnectedApp | null): ConnectedAppFormValues {
  return {
    slug: app?.slug ?? '',
    name: app?.name ?? '',
    description: app?.description ?? '',
    allowedScopesText: scopesToText(app?.allowed_scopes ?? []),
    defaultScopesText: scopesToText(app?.default_scopes ?? []),
    trusted: app?.trusted ?? false,
    enabled:
      (app?.status ?? CONNECTED_APP_STATUS_ENABLED) ===
      CONNECTED_APP_STATUS_ENABLED,
  }
}

function buildPayload(values: ConnectedAppFormValues): ConnectedAppPayload {
  return {
    slug: values.slug.trim().toLowerCase(),
    name: values.name.trim(),
    description: values.description.trim(),
    allowed_scopes: parseScopesText(values.allowedScopesText),
    default_scopes: parseScopesText(values.defaultScopesText),
    authorization_flow: 'device_code',
    trusted: values.trusted,
    status: values.enabled
      ? CONNECTED_APP_STATUS_ENABLED
      : CONNECTED_APP_STATUS_DISABLED,
  }
}

function buildReviewDefaults(
  request: ConnectedAppRequest | null,
  decision: ConnectedAppReviewDecision
): ConnectedAppReviewFormValues {
  return {
    decision,
    reviewNote: '',
    name: request?.name ?? '',
    description: request?.description ?? '',
    allowedScopesText: scopesToText(request?.requested_scopes ?? []),
    defaultScopesText: scopesToText(request?.default_scopes ?? []),
  }
}

function buildReviewPayload(
  values: ConnectedAppReviewFormValues
): ConnectedAppReviewPayload {
  if (values.decision === 'rejected') {
    return {
      decision: 'rejected',
      review_note: values.reviewNote.trim(),
    }
  }
  return {
    decision: 'approved',
    review_note: values.reviewNote.trim(),
    name: values.name.trim(),
    description: values.description.trim(),
    allowed_scopes: parseScopesText(values.allowedScopesText),
    default_scopes: parseScopesText(values.defaultScopesText),
    authorization_flow: 'device_code',
  }
}

function appStatusMeta(status: ConnectedAppStatus) {
  if (status === CONNECTED_APP_STATUS_ENABLED) {
    return { label: 'Enabled', variant: 'success' as const }
  }
  return { label: 'Disabled', variant: 'neutral' as const }
}

function requestStatusMeta(status: string) {
  switch (status) {
    case 'approved':
      return { label: 'Approved', variant: 'success' as const }
    case 'rejected':
      return { label: 'Rejected', variant: 'danger' as const }
    default:
      return { label: 'Pending', variant: 'warning' as const }
  }
}

function auditActionLabel(action: string) {
  switch (action) {
    case 'connected_app_request.submit':
      return 'Submitted'
    case 'connected_app_request.approve':
      return 'Approved'
    case 'connected_app_request.reject':
      return 'Rejected'
    default:
      return action
  }
}

function authorizationFlowLabel(flow: string) {
  if (flow === 'device_code') return 'Device code'
  return flow
}

function formatOptionalTimestamp(value: number) {
  return value ? formatTimestampToDate(value) : '-'
}

function formatJsonSnippet(value: string) {
  if (!value || value === '{}') return '{}'
  try {
    const parsed = JSON.parse(value)
    return JSON.stringify(parsed).slice(0, 160)
  } catch {
    return value.slice(0, 160)
  }
}

export function ConnectedAppsSection() {
  const { t } = useTranslation()
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editingApp, setEditingApp] = useState<ConnectedApp | null>(null)
  const [reviewContext, setReviewContext] = useState<{
    request: ConnectedAppRequest
    decision: ConnectedAppReviewDecision
  } | null>(null)
  const appsQuery = useQuery({
    queryKey: connectedAppsQueryKey,
    queryFn: listConnectedApps,
  })
  const requestsQuery = useQuery({
    queryKey: connectedAppRequestsQueryKey,
    queryFn: () => listConnectedAppRequests({ page_size: 50 }),
  })
  const focusedRequestId = search.connected_app_request_id
  const auditQuery = useQuery({
    queryKey: [
      ...connectedAppAuditLogsQueryKey,
      { targetId: focusedRequestId ?? 0 },
    ],
    queryFn: () =>
      listConnectedAppAuditLogs({
        page_size: 50,
        target_type: focusedRequestId ? 'connected_app_request' : undefined,
        target_id: focusedRequestId,
      }),
  })

  const apps = appsQuery.data ?? []
  const requests = requestsQuery.data?.items ?? []
  const auditLogs = auditQuery.data?.items ?? []
  const enabledCount = apps.filter(
    (app) => app.status === CONNECTED_APP_STATUS_ENABLED
  ).length
  const pendingCount = requests.filter(
    (request) => request.status === 'pending'
  ).length
  const activeTab = isConnectedAppsTab(search.tab) ? search.tab : 'apps'
  const isFetching =
    appsQuery.isFetching || requestsQuery.isFetching || auditQuery.isFetching

  const openCreate = () => {
    setEditingApp(null)
    setSheetOpen(true)
  }

  const openEdit = (app: ConnectedApp) => {
    setEditingApp(app)
    setSheetOpen(true)
  }

  const setActiveTab = (tab: ConnectedAppsTab) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        tab,
      }),
    })
  }

  const refreshAll = () => {
    void appsQuery.refetch()
    void requestsQuery.refetch()
    void auditQuery.refetch()
  }

  const openReview = (
    request: ConnectedAppRequest,
    decision: ConnectedAppReviewDecision
  ) => {
    setReviewContext({ request, decision })
  }

  return (
    <SettingsSection title={t('Connected Apps')}>
      <SettingsPageTitleStatusPortal>
        <div className='flex items-center gap-1.5'>
          <StatusBadge
            copyable={false}
            label={`${enabledCount}/${apps.length} ${t('enabled')}`}
            variant={enabledCount === apps.length ? 'success' : 'warning'}
          />
          <StatusBadge
            copyable={false}
            label={`${pendingCount} ${t('pending')}`}
            variant={pendingCount > 0 ? 'warning' : 'neutral'}
          />
        </div>
      </SettingsPageTitleStatusPortal>
      <SettingsPageActionsPortal>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={refreshAll}
          disabled={isFetching}
        >
          <RefreshCw data-icon='inline-start' />
          <span>{t(isFetching ? 'Refreshing' : 'Refresh')}</span>
        </Button>
        <Button type='button' size='sm' onClick={openCreate}>
          <Plus data-icon='inline-start' />
          <span>{t('New app')}</span>
        </Button>
      </SettingsPageActionsPortal>

      <Tabs
        value={activeTab}
        onValueChange={(tab) => setActiveTab(tab as ConnectedAppsTab)}
      >
        <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
          <TabsTrigger value='apps'>
            <ShieldCheck data-icon='inline-start' />
            {t('Apps')}
          </TabsTrigger>
          <TabsTrigger value='requests'>
            <ClipboardList data-icon='inline-start' />
            {t('Requests')}
            {pendingCount > 0 ? (
              <Badge variant='secondary' className='ml-1'>
                {pendingCount}
              </Badge>
            ) : null}
          </TabsTrigger>
          <TabsTrigger value='notifications'>
            <BellRing data-icon='inline-start' />
            {t('Notifications')}
          </TabsTrigger>
          <TabsTrigger value='audit'>
            <FileClock data-icon='inline-start' />
            {t('Audit')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value='apps'>
          <ConnectedAppsTable apps={apps} query={appsQuery} onEdit={openEdit} />
        </TabsContent>
        <TabsContent value='requests'>
          <ConnectedAppRequestsTable
            focusedRequestId={focusedRequestId}
            requests={requests}
            query={requestsQuery}
            onReview={openReview}
          />
        </TabsContent>
        <TabsContent value='notifications'>
          <ConnectedAppNotificationsSection />
        </TabsContent>
        <TabsContent value='audit'>
          <ConnectedAppAuditTable
            focusedRequestId={focusedRequestId}
            logs={auditLogs}
            query={auditQuery}
          />
        </TabsContent>
      </Tabs>

      <ConnectedAppSheet
        app={editingApp}
        open={sheetOpen}
        onOpenChange={setSheetOpen}
      />
      <ConnectedAppReviewSheet
        context={reviewContext}
        open={Boolean(reviewContext)}
        onOpenChange={(open) => {
          if (!open) setReviewContext(null)
        }}
      />
    </SettingsSection>
  )
}

type QueryViewState = {
  isLoading: boolean
  isError: boolean
  error: unknown
}

function ConnectedAppsTable({
  apps,
  query,
  onEdit,
}: {
  apps: ConnectedApp[]
  query: QueryViewState
  onEdit: (app: ConnectedApp) => void
}) {
  const { t } = useTranslation()

  return (
    <div className='bg-card overflow-hidden rounded-xl border'>
      <div className='flex flex-col gap-1 border-b px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between'>
        <div className='min-w-0'>
          <h3 className='text-sm font-medium'>{t('Connected Apps')}</h3>
          <p className='text-muted-foreground text-xs'>
            {t('Scoped device access for approved applications')}
          </p>
        </div>
        <div className='text-muted-foreground flex items-center gap-3 text-xs tabular-nums'>
          <span>
            {t('Grants')}: {sumApps(apps, 'grant_count')}
          </span>
          <span>
            {t('Active devices')}: {sumApps(apps, 'active_device_count')}
          </span>
        </div>
      </div>

      {query.isLoading ? (
        <ConnectedAppsSkeleton />
      ) : query.isError ? (
        <div className='p-3'>
          <Alert variant='destructive'>
            <AlertTitle>{t('Failed to load connected apps')}</AlertTitle>
            <AlertDescription>
              {query.error instanceof Error
                ? query.error.message
                : t('Request failed')}
            </AlertDescription>
          </Alert>
        </div>
      ) : apps.length === 0 ? (
        <Empty className='border-0 py-10'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <ShieldAlert />
            </EmptyMedia>
            <EmptyTitle>{t('No connected apps')}</EmptyTitle>
            <EmptyDescription>
              {t('Create an app before granting device access.')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('App')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Trusted')}</TableHead>
              <TableHead>{t('Scopes')}</TableHead>
              <TableHead className='text-right'>{t('Grants')}</TableHead>
              <TableHead className='text-right'>{t('Devices')}</TableHead>
              <TableHead>{t('Updated')}</TableHead>
              <TableHead className='w-20 text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {apps.map((app) => (
              <ConnectedAppRow
                key={app.id}
                app={app}
                onEdit={() => onEdit(app)}
              />
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

function ConnectedAppRequestsTable({
  requests,
  query,
  focusedRequestId,
  onReview,
}: {
  requests: ConnectedAppRequest[]
  query: QueryViewState
  focusedRequestId?: number
  onReview: (
    request: ConnectedAppRequest,
    decision: ConnectedAppReviewDecision
  ) => void
}) {
  const { t } = useTranslation()
  const pendingCount = requests.filter(
    (request) => request.status === 'pending'
  ).length

  return (
    <div className='bg-card overflow-hidden rounded-xl border'>
      <div className='flex flex-col gap-1 border-b px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between'>
        <div className='min-w-0'>
          <h3 className='text-sm font-medium'>{t('App access requests')}</h3>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Review submitted integrations before they can create device keys'
            )}
          </p>
        </div>
        <div className='text-muted-foreground flex items-center gap-3 text-xs tabular-nums'>
          <span>
            {t('Pending')}: {pendingCount.toLocaleString()}
          </span>
          <span>
            {t('Total')}: {requests.length.toLocaleString()}
          </span>
          {focusedRequestId ? (
            <span>
              {t('Focused')} #{focusedRequestId}
            </span>
          ) : null}
        </div>
      </div>

      {query.isLoading ? (
        <ConnectedAppsSkeleton />
      ) : query.isError ? (
        <div className='p-3'>
          <Alert variant='destructive'>
            <AlertTitle>{t('Failed to load app requests')}</AlertTitle>
            <AlertDescription>
              {query.error instanceof Error
                ? query.error.message
                : t('Request failed')}
            </AlertDescription>
          </Alert>
        </div>
      ) : requests.length === 0 ? (
        <Empty className='border-0 py-10'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <ClipboardList />
            </EmptyMedia>
            <EmptyTitle>{t('No app access requests')}</EmptyTitle>
            <EmptyDescription>
              {t(
                'Submitted third-party application requests will appear here.'
              )}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Request')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Applicant')}</TableHead>
              <TableHead>{t('Scopes')}</TableHead>
              <TableHead>{t('Submitted')}</TableHead>
              <TableHead>{t('Reviewed')}</TableHead>
              <TableHead className='w-32 text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {requests.map((request) => (
              <ConnectedAppRequestRow
                key={request.id}
                focused={focusedRequestId === request.id}
                request={request}
                onReview={onReview}
              />
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

function ConnectedAppAuditTable({
  logs,
  query,
  focusedRequestId,
}: {
  logs: ConnectedAppAuditLog[]
  query: QueryViewState
  focusedRequestId?: number
}) {
  const { t } = useTranslation()

  return (
    <div className='bg-card overflow-hidden rounded-xl border'>
      <div className='flex flex-col gap-1 border-b px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between'>
        <div className='min-w-0'>
          <h3 className='text-sm font-medium'>{t('Connected app audit')}</h3>
          <p className='text-muted-foreground text-xs'>
            {t('Submission and review events for application access')}
          </p>
        </div>
        <div className='text-muted-foreground flex items-center gap-3 text-xs tabular-nums'>
          <span>
            {t('Events')}: {logs.length.toLocaleString()}
          </span>
          {focusedRequestId ? (
            <span>
              {t('Request')} #{focusedRequestId}
            </span>
          ) : null}
        </div>
      </div>

      {query.isLoading ? (
        <ConnectedAppsSkeleton />
      ) : query.isError ? (
        <div className='p-3'>
          <Alert variant='destructive'>
            <AlertTitle>{t('Failed to load app audit logs')}</AlertTitle>
            <AlertDescription>
              {query.error instanceof Error
                ? query.error.message
                : t('Request failed')}
            </AlertDescription>
          </Alert>
        </div>
      ) : logs.length === 0 ? (
        <Empty className='border-0 py-10'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <FileClock />
            </EmptyMedia>
            <EmptyTitle>{t('No app audit events')}</EmptyTitle>
            <EmptyDescription>
              {t(
                'Approval events will appear here after requests are reviewed.'
              )}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Event')}</TableHead>
              <TableHead>{t('Actor')}</TableHead>
              <TableHead>{t('Target')}</TableHead>
              <TableHead>{t('After')}</TableHead>
              <TableHead>{t('Request ID')}</TableHead>
              <TableHead>{t('Created')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {logs.map((log) => (
              <ConnectedAppAuditRow key={log.id} log={log} />
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

function sumApps(
  apps: ConnectedApp[],
  key: 'grant_count' | 'active_device_count'
) {
  return apps.reduce((sum, app) => sum + app[key], 0).toLocaleString()
}

function ConnectedAppRow({
  app,
  onEdit,
}: {
  app: ConnectedApp
  onEdit: () => void
}) {
  const { t } = useTranslation()
  const status = appStatusMeta(app.status)
  const updatedAt = app.updated_at ? formatTimestampToDate(app.updated_at) : '-'

  return (
    <TableRow>
      <TableCell className='min-w-56'>
        <div className='flex min-w-0 flex-col gap-1'>
          <div className='flex min-w-0 items-center gap-2'>
            <span className='truncate font-medium'>{app.name}</span>
            <Badge variant='outline' className='font-mono'>
              {app.slug}
            </Badge>
          </div>
          <span className='text-muted-foreground truncate text-xs'>
            {app.description || authorizationFlowLabel(app.authorization_flow)}
          </span>
        </div>
      </TableCell>
      <TableCell>
        <StatusBadge
          copyable={false}
          label={t(status.label)}
          variant={status.variant}
        />
      </TableCell>
      <TableCell>
        <StatusBadge
          copyable={false}
          icon={app.trusted ? ShieldCheck : ShieldAlert}
          label={app.trusted ? t('Trusted') : t('Review')}
          variant={app.trusted ? 'success' : 'neutral'}
        />
      </TableCell>
      <TableCell className='min-w-72 whitespace-normal'>
        <ScopeBadges
          allowedScopes={app.allowed_scopes}
          defaultScopes={app.default_scopes}
        />
      </TableCell>
      <TableCell className='text-right tabular-nums'>
        {app.grant_count.toLocaleString()}
      </TableCell>
      <TableCell className='text-right tabular-nums'>
        <span>{app.active_device_count.toLocaleString()}</span>
        <span className='text-muted-foreground'>
          {' / '}
          {app.device_count.toLocaleString()}
        </span>
      </TableCell>
      <TableCell className='text-muted-foreground text-xs'>
        {updatedAt}
      </TableCell>
      <TableCell className='text-right'>
        <Button type='button' size='sm' variant='ghost' onClick={onEdit}>
          <Edit3 data-icon='inline-start' />
          <span>{t('Edit')}</span>
        </Button>
      </TableCell>
    </TableRow>
  )
}

function ConnectedAppRequestRow({
  request,
  focused,
  onReview,
}: {
  request: ConnectedAppRequest
  focused: boolean
  onReview: (
    request: ConnectedAppRequest,
    decision: ConnectedAppReviewDecision
  ) => void
}) {
  const { t } = useTranslation()
  const status = requestStatusMeta(request.status)
  const isPending = request.status === 'pending'

  return (
    <TableRow className={cn(focused && 'bg-muted/60')}>
      <TableCell className='min-w-64'>
        <div className='flex min-w-0 flex-col gap-1'>
          <div className='flex min-w-0 items-center gap-2'>
            <span className='truncate font-medium'>{request.name}</span>
            <Badge variant='outline' className='font-mono'>
              {request.slug}
            </Badge>
          </div>
          <span className='text-muted-foreground truncate text-xs'>
            {request.reason || request.description || t('No request note')}
          </span>
        </div>
      </TableCell>
      <TableCell>
        <StatusBadge
          copyable={false}
          label={t(status.label)}
          variant={status.variant}
        />
      </TableCell>
      <TableCell className='text-muted-foreground min-w-28 text-xs'>
        {request.applicant_name || `#${request.applicant_user_id}`}
      </TableCell>
      <TableCell className='min-w-72 whitespace-normal'>
        <ScopeBadges
          allowedScopes={request.requested_scopes}
          defaultScopes={request.default_scopes}
        />
      </TableCell>
      <TableCell className='text-muted-foreground text-xs'>
        {formatOptionalTimestamp(request.created_at)}
      </TableCell>
      <TableCell className='text-muted-foreground text-xs'>
        <div className='flex flex-col gap-1'>
          <span>{formatOptionalTimestamp(request.reviewed_at)}</span>
          {request.reviewer_name ? <span>{request.reviewer_name}</span> : null}
        </div>
      </TableCell>
      <TableCell className='text-right'>
        {isPending ? (
          <div className='flex justify-end gap-1'>
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={() => onReview(request, 'approved')}
            >
              <Check data-icon='inline-start' />
              <span>{t('Approve')}</span>
            </Button>
            <Button
              type='button'
              size='sm'
              variant='destructive'
              onClick={() => onReview(request, 'rejected')}
            >
              <X data-icon='inline-start' />
              <span>{t('Reject')}</span>
            </Button>
          </div>
        ) : request.review_note ? (
          <span className='text-muted-foreground line-clamp-2 text-xs'>
            {request.review_note}
          </span>
        ) : (
          <span className='text-muted-foreground text-xs'>-</span>
        )}
      </TableCell>
    </TableRow>
  )
}

function ConnectedAppAuditRow({ log }: { log: ConnectedAppAuditLog }) {
  const { t } = useTranslation()

  return (
    <TableRow>
      <TableCell className='min-w-44'>
        <div className='flex min-w-0 flex-col gap-1'>
          <span className='truncate font-medium'>
            {t(auditActionLabel(log.action))}
          </span>
          <span className='text-muted-foreground truncate font-mono text-xs'>
            {log.action}
          </span>
        </div>
      </TableCell>
      <TableCell className='text-muted-foreground min-w-28 text-xs'>
        {log.actor_name || `#${log.actor_user_id}`}
      </TableCell>
      <TableCell className='text-muted-foreground text-xs'>
        <span className='font-mono'>{log.target_type}</span>
        <span> #{log.target_id}</span>
      </TableCell>
      <TableCell className='max-w-xl'>
        <code className='bg-muted text-muted-foreground line-clamp-2 rounded px-1.5 py-1 text-xs break-all'>
          {formatJsonSnippet(log.after_json)}
        </code>
      </TableCell>
      <TableCell className='text-muted-foreground max-w-48 truncate font-mono text-xs'>
        {log.request_id || '-'}
      </TableCell>
      <TableCell className='text-muted-foreground text-xs'>
        {formatOptionalTimestamp(log.created_at)}
      </TableCell>
    </TableRow>
  )
}

function ScopeBadges({
  allowedScopes,
  defaultScopes,
}: {
  allowedScopes: string[]
  defaultScopes: string[]
}) {
  const { t } = useTranslation()
  const visible = allowedScopes.slice(0, 4)
  const hidden = allowedScopes.length - visible.length
  const defaultSet = useMemo(() => new Set(defaultScopes), [defaultScopes])

  return (
    <div className='flex max-w-xl flex-wrap items-center gap-1'>
      {visible.map((scope) => (
        <Badge
          key={scope}
          variant={defaultSet.has(scope) ? 'default' : 'outline'}
          className='font-mono'
          title={
            defaultSet.has(scope) ? t('Default scope') : t('Allowed scope')
          }
        >
          {scope}
        </Badge>
      ))}
      {hidden > 0 && (
        <Badge variant='secondary' className='font-mono'>
          +{hidden}
        </Badge>
      )}
    </div>
  )
}

function ConnectedAppsSkeleton() {
  return (
    <div className='space-y-0'>
      {Array.from({ length: 4 }).map((_, index) => (
        <div
          key={index}
          className='grid grid-cols-[minmax(12rem,1.2fr)_7rem_8rem_minmax(16rem,1.4fr)_5rem_6rem_8rem_5rem] items-center gap-3 border-b px-2 py-3 last:border-b-0'
        >
          <Skeleton className='h-5 w-44' />
          <Skeleton className='h-5 w-16' />
          <Skeleton className='h-5 w-20' />
          <Skeleton className='h-5 w-64' />
          <Skeleton className='h-5 w-8 justify-self-end' />
          <Skeleton className='h-5 w-12 justify-self-end' />
          <Skeleton className='h-5 w-28' />
          <Skeleton className='h-7 w-16 justify-self-end' />
        </div>
      ))}
    </div>
  )
}

function ConnectedAppSheet({
  app,
  open,
  onOpenChange,
}: {
  app: ConnectedApp | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isEdit = Boolean(app)
  const formDefaults = useMemo(() => buildFormDefaults(app), [app])
  const form = useForm<ConnectedAppFormValues>({
    resolver: zodResolver(connectedAppFormSchema),
    defaultValues: formDefaults,
  })

  useEffect(() => {
    if (open) {
      form.reset(formDefaults)
    }
  }, [form, formDefaults, open])

  const mutation = useMutation({
    mutationFn: (values: ConnectedAppFormValues) => {
      const payload = buildPayload(values)
      if (app) return updateConnectedApp(app.id, payload)
      return createConnectedApp(payload)
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: connectedAppsQueryKey })
      toast.success(
        t(isEdit ? 'Connected app updated' : 'Connected app created')
      )
      onOpenChange(false)
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    },
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-xl'>
        <SheetHeader>
          <SheetTitle>
            {t(isEdit ? 'Edit connected app' : 'New connected app')}
          </SheetTitle>
          <SheetDescription>
            {t('Device code authorization with scoped native tokens')}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            className='flex min-h-0 flex-1 flex-col'
            onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
          >
            <div className='flex-1 space-y-5 overflow-y-auto px-4 pb-4'>
              <FormField
                control={form.control}
                name='slug'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Slug')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        disabled={isEdit || mutation.isPending}
                        placeholder='snapless'
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Stable app identifier used by integrations')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} disabled={mutation.isPending} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='description'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Description')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        disabled={mutation.isPending}
                        className='min-h-20 resize-y'
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='trusted'
                  render={({ field }) => (
                    <FormItem className='flex items-center justify-between gap-3 rounded-lg border px-3 py-2.5'>
                      <div className='min-w-0 space-y-0.5'>
                        <FormLabel>{t('Trusted')}</FormLabel>
                        <FormDescription>
                          {t('Approved app can request configured scopes')}
                        </FormDescription>
                      </div>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                          disabled={mutation.isPending}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='enabled'
                  render={({ field }) => (
                    <FormItem className='flex items-center justify-between gap-3 rounded-lg border px-3 py-2.5'>
                      <div className='min-w-0 space-y-0.5'>
                        <FormLabel>{t('Enabled')}</FormLabel>
                        <FormDescription>
                          {t('Disabled apps cannot create new access')}
                        </FormDescription>
                      </div>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                          disabled={mutation.isPending}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name='allowedScopesText'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Allowed scopes')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        disabled={mutation.isPending}
                        className='min-h-28 resize-y font-mono text-xs'
                        placeholder={'openai.chat\nquota.read'}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Separate scopes with spaces, commas or new lines')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='defaultScopesText'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default scopes')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        disabled={mutation.isPending}
                        className='min-h-24 resize-y font-mono text-xs'
                        placeholder='openai.chat'
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Every default scope must also be allowed')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <SheetFooter className='border-t'>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
                disabled={mutation.isPending}
              >
                {t('Cancel')}
              </Button>
              <Button type='submit' disabled={mutation.isPending}>
                {t(mutation.isPending ? 'Saving...' : 'Save Changes')}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}

function ConnectedAppReviewSheet({
  context,
  open,
  onOpenChange,
}: {
  context: {
    request: ConnectedAppRequest
    decision: ConnectedAppReviewDecision
  } | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const request = context?.request ?? null
  const decision = context?.decision ?? 'approved'
  const formDefaults = useMemo(
    () => buildReviewDefaults(request, decision),
    [decision, request]
  )
  const form = useForm<ConnectedAppReviewFormValues>({
    resolver: zodResolver(connectedAppReviewSchema),
    defaultValues: formDefaults,
  })
  const currentDecision = form.watch('decision')
  const isApproval = currentDecision === 'approved'

  useEffect(() => {
    if (open) {
      form.reset(formDefaults)
    }
  }, [form, formDefaults, open])

  const mutation = useMutation({
    mutationFn: (values: ConnectedAppReviewFormValues) => {
      if (!request) throw new Error('Request is required')
      return reviewConnectedAppRequest(request.id, buildReviewPayload(values))
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: connectedAppsQueryKey }),
        queryClient.invalidateQueries({
          queryKey: connectedAppRequestsQueryKey,
        }),
        queryClient.invalidateQueries({
          queryKey: connectedAppAuditLogsQueryKey,
        }),
        queryClient.invalidateQueries({
          queryKey: ['notifications', 'connected-app-requests'],
        }),
      ])
      toast.success(
        t(
          isApproval
            ? 'Connected app request approved'
            : 'Connected app request rejected'
        )
      )
      onOpenChange(false)
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    },
  })

  if (!request) return null

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-xl'>
        <SheetHeader>
          <SheetTitle>
            {t(isApproval ? 'Approve app request' : 'Reject app request')}
          </SheetTitle>
          <SheetDescription>
            {t('Review requested device access before creating a trusted app')}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            className='flex min-h-0 flex-1 flex-col'
            onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
          >
            <div className='flex-1 space-y-5 overflow-y-auto px-4 pb-4'>
              <div className='rounded-lg border px-3 py-2.5'>
                <div className='flex min-w-0 flex-wrap items-center gap-2'>
                  <span className='truncate text-sm font-medium'>
                    {request.name}
                  </span>
                  <Badge variant='outline' className='font-mono'>
                    {request.slug}
                  </Badge>
                  <StatusBadge
                    copyable={false}
                    label={t(requestStatusMeta(request.status).label)}
                    variant={requestStatusMeta(request.status).variant}
                  />
                </div>
                <div className='text-muted-foreground mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs'>
                  <span>
                    {t('Applicant')}:{' '}
                    {request.applicant_name || `#${request.applicant_user_id}`}
                  </span>
                  <span>
                    {t('Submitted')}:{' '}
                    {formatOptionalTimestamp(request.created_at)}
                  </span>
                </div>
                {request.reason ? (
                  <p className='text-muted-foreground mt-2 text-xs'>
                    {request.reason}
                  </p>
                ) : null}
              </div>

              <FormField
                control={form.control}
                name='decision'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Decision')}</FormLabel>
                    <FormControl>
                      <Input
                        value={t(
                          field.value === 'approved' ? 'Approve' : 'Reject'
                        )}
                        disabled
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {isApproval ? (
                <>
                  <FormField
                    control={form.control}
                    name='name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Name')}</FormLabel>
                        <FormControl>
                          <Input {...field} disabled={mutation.isPending} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='description'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Description')}</FormLabel>
                        <FormControl>
                          <Textarea
                            {...field}
                            disabled={mutation.isPending}
                            className='min-h-20 resize-y'
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='allowedScopesText'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Allowed scopes')}</FormLabel>
                        <FormControl>
                          <Textarea
                            {...field}
                            disabled={mutation.isPending}
                            className='min-h-28 resize-y font-mono text-xs'
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'Separate scopes with spaces, commas or new lines'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='defaultScopesText'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Default scopes')}</FormLabel>
                        <FormControl>
                          <Textarea
                            {...field}
                            disabled={mutation.isPending}
                            className='min-h-24 resize-y font-mono text-xs'
                          />
                        </FormControl>
                        <FormDescription>
                          {t('Every default scope must also be allowed')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </>
              ) : null}

              <FormField
                control={form.control}
                name='reviewNote'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Review note')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        disabled={mutation.isPending}
                        className='min-h-24 resize-y'
                      />
                    </FormControl>
                    <FormDescription>
                      {t('This note is visible on the request record.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <SheetFooter className='border-t'>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
                disabled={mutation.isPending}
              >
                {t('Cancel')}
              </Button>
              <Button
                type='submit'
                variant={isApproval ? 'default' : 'destructive'}
                disabled={mutation.isPending}
              >
                {t(
                  mutation.isPending
                    ? 'Saving...'
                    : isApproval
                      ? 'Approve'
                      : 'Reject'
                )}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}
