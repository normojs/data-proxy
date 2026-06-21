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
import { useMemo, useState, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Activity,
  Ban,
  BellRing,
  Check,
  Clock,
  Plus,
  RefreshCw,
  RotateCcw,
  Send,
  Webhook,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import {
  type ConnectedAppNotificationOutbox,
  type ConnectedAppNotificationOutboxWorkerMetrics,
  type ConnectedAppNotificationPreference,
  type ConnectedAppNotificationPreferencePayload,
  type ConnectedAppNotificationRecipientScope,
  type ConnectedAppWebhook,
  type ConnectedAppWebhookPayload,
  type ConnectedAppWebhookTestResult,
  createConnectedAppWebhook,
  disableConnectedAppWebhook,
  getConnectedAppNotificationOutboxWorkerMetrics,
  listConnectedAppNotificationOutbox,
  listConnectedAppNotificationPreferences,
  listConnectedAppWebhooks,
  retryConnectedAppNotificationOutbox,
  testConnectedAppWebhook,
  updateConnectedAppNotificationPreference,
  updateConnectedAppWebhook,
} from './connected-apps-api'

const CONNECTED_APP_NOTIFICATION_APP_ID = 0
const CONNECTED_APP_WEBHOOK_ENABLED = 1
const CONNECTED_APP_WEBHOOK_DISABLED = 2
const CONNECTED_APP_DELIVERY_PAGE_SIZE = 20
const ALL_VALUE = '__all__'

const CONNECTED_APP_NOTIFICATION_EVENTS = [
  'connected_app_request.approve',
  'connected_app_request.reject',
  'connected_app_device.authorized',
  'connected_app_device.denied',
  'connected_app.health.warning',
] as const

type ConnectedAppNotificationEvent =
  (typeof CONNECTED_APP_NOTIFICATION_EVENTS)[number]

type QueryViewState = {
  isLoading: boolean
  isError: boolean
  error: unknown
  refetch?: () => unknown
}

type WebhookFormState = {
  name: string
  url: string
  secret: string
  eventTypes: string[]
  status: string
}

export function ConnectedAppNotificationsSection() {
  return (
    <div className='space-y-3'>
      <NotificationPreferencesPanel />
      <ConnectedAppWebhooksPanel />
      <ConnectedAppDeliveriesPanel />
    </div>
  )
}

function NotificationPreferencesPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const preferencesQuery = useQuery({
    queryKey: ['connected-apps', 'notification-preferences', 0],
    queryFn: () =>
      listConnectedAppNotificationPreferences(
        CONNECTED_APP_NOTIFICATION_APP_ID
      ),
  })
  const preferences = useMemo(
    () => preferencesQuery.data ?? [],
    [preferencesQuery.data]
  )
  const byKey = useMemo(() => {
    const result = new Map<string, ConnectedAppNotificationPreference>()
    preferences.forEach((preference) => {
      result.set(
        notificationPreferenceKey(preference.channel, preference.event_type),
        preference
      )
    })
    return result
  }, [preferences])

  const mutation = useMutation({
    mutationFn: updateConnectedAppNotificationPreference,
    onSuccess: async () => {
      toast.success(t('Saved'))
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['connected-apps', 'notification-preferences'],
        }),
        queryClient.invalidateQueries({
          queryKey: connectedAppNotificationAuditQueryKey(),
        }),
      ])
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    },
  })

  return (
    <NotificationPanel
      title={t('Notification preferences')}
      description={t('Global defaults for connected app notification events')}
    >
      <QueryState query={preferencesQuery}>
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Event')}</TableHead>
                <TableHead>{t('Email')}</TableHead>
                <TableHead>{t('Recipients')}</TableHead>
                <TableHead>{t('Webhook')}</TableHead>
                <TableHead>{t('Updated')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {CONNECTED_APP_NOTIFICATION_EVENTS.map((eventType) => {
                const emailPreference = byKey.get(
                  notificationPreferenceKey('email', eventType)
                )
                const webhookPreference = byKey.get(
                  notificationPreferenceKey('webhook', eventType)
                )
                if (!emailPreference || !webhookPreference) return null

                return (
                  <NotificationPreferenceRow
                    key={`${eventType}:${emailPreference.updated_at}:${webhookPreference.updated_at}`}
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
        </div>
      </QueryState>
    </NotificationPanel>
  )
}

function NotificationPreferenceRow({
  eventType,
  emailPreference,
  webhookPreference,
  isSaving,
  onSave,
}: {
  eventType: ConnectedAppNotificationEvent
  emailPreference: ConnectedAppNotificationPreference
  webhookPreference: ConnectedAppNotificationPreference
  isSaving: boolean
  onSave: (payload: ConnectedAppNotificationPreferencePayload) => void
}) {
  const { t } = useTranslation()
  const [explicitEmails, setExplicitEmails] = useState(
    emailsToText(emailPreference.recipient_scope.explicit_emails)
  )
  const emailScope = emailPreference.recipient_scope

  const updateEmailPreference = (
    patch:
      | Partial<ConnectedAppNotificationRecipientScope>
      | { enabled: boolean }
  ) => {
    if ('enabled' in patch) {
      onSave(preferencePayload(emailPreference, emailScope, patch.enabled))
      return
    }
    onSave(preferencePayload(emailPreference, { ...emailScope, ...patch }))
  }

  return (
    <TableRow>
      <TableCell className='min-w-64'>
        <div className='flex min-w-0 flex-col gap-1'>
          <span className='font-medium'>
            {t(notificationEventLabel(eventType))}
          </span>
          <span className='text-muted-foreground font-mono text-xs'>
            {eventType}
          </span>
        </div>
      </TableCell>
      <TableCell>
        <Switch
          checked={emailPreference.enabled}
          onCheckedChange={(checked) =>
            updateEmailPreference({ enabled: checked })
          }
          disabled={isSaving}
          aria-label={t('Email')}
        />
      </TableCell>
      <TableCell className='min-w-[26rem]'>
        <div className='flex flex-col gap-2'>
          <div className='flex flex-wrap gap-x-3 gap-y-2'>
            <ScopeCheckbox
              label={t('Applicant')}
              checked={emailScope.applicant}
              disabled={isSaving || !emailPreference.enabled}
              onCheckedChange={(checked) =>
                updateEmailPreference({ applicant: checked })
              }
            />
            <ScopeCheckbox
              label={t('Authorizing user')}
              checked={emailScope.authorizing_user}
              disabled={isSaving || !emailPreference.enabled}
              onCheckedChange={(checked) =>
                updateEmailPreference({ authorizing_user: checked })
              }
            />
            <ScopeCheckbox
              label={t('App developers')}
              checked={emailScope.app_developers}
              disabled={isSaving || !emailPreference.enabled}
              onCheckedChange={(checked) =>
                updateEmailPreference({ app_developers: checked })
              }
            />
          </div>
          <div className='flex min-w-0 gap-2'>
            <Input
              value={explicitEmails}
              onChange={(event) => setExplicitEmails(event.target.value)}
              disabled={isSaving || !emailPreference.enabled}
              placeholder='ops@example.com, dev@example.com'
              className='h-8 min-w-64 font-mono text-xs'
            />
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={isSaving || !emailPreference.enabled}
              onClick={() =>
                updateEmailPreference({
                  explicit_emails: parseEmailList(explicitEmails),
                })
              }
            >
              <Check data-icon='inline-start' />
              <span>{t('Save')}</span>
            </Button>
          </div>
        </div>
      </TableCell>
      <TableCell>
        <Switch
          checked={webhookPreference.enabled}
          onCheckedChange={(checked) =>
            onSave(preferencePayload(webhookPreference, undefined, checked))
          }
          disabled={isSaving}
          aria-label={t('Webhook')}
        />
      </TableCell>
      <TableCell className='text-muted-foreground text-xs'>
        {formatOptionalTimestamp(
          Math.max(emailPreference.updated_at, webhookPreference.updated_at)
        )}
      </TableCell>
    </TableRow>
  )
}

function ScopeCheckbox({
  label,
  checked,
  disabled,
  onCheckedChange,
}: {
  label: string
  checked: boolean
  disabled: boolean
  onCheckedChange: (checked: boolean) => void
}) {
  return (
    <label className='flex items-center gap-2 text-xs'>
      <Checkbox
        checked={checked}
        disabled={disabled}
        onCheckedChange={(value) => onCheckedChange(value === true)}
      />
      <span>{label}</span>
    </label>
  )
}

function ConnectedAppWebhooksPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editingWebhook, setEditingWebhook] =
    useState<ConnectedAppWebhook | null>(null)
  const [testResults, setTestResults] = useState<
    Record<number, ConnectedAppWebhookTestResult>
  >({})

  const webhooksQuery = useQuery({
    queryKey: ['connected-apps', 'webhooks', 0],
    queryFn: () => listConnectedAppWebhooks(CONNECTED_APP_NOTIFICATION_APP_ID),
  })
  const webhooks = webhooksQuery.data ?? []

  const disableMutation = useMutation({
    mutationFn: disableConnectedAppWebhook,
    onSuccess: async () => {
      toast.success(t('Disabled'))
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['connected-apps', 'webhooks'],
        }),
        queryClient.invalidateQueries({
          queryKey: connectedAppNotificationAuditQueryKey(),
        }),
      ])
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    },
  })
  const testMutation = useMutation({
    mutationFn: testConnectedAppWebhook,
    onSuccess: async (result, id) => {
      setTestResults((current) => ({ ...current, [id]: result }))
      toast[result.success ? 'success' : 'error'](
        t(result.success ? 'Webhook test sent' : 'Webhook test failed')
      )
      await queryClient.invalidateQueries({
        queryKey: connectedAppNotificationAuditQueryKey(),
      })
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    },
  })

  return (
    <>
      <NotificationPanel
        title={t('Webhooks')}
        description={t('Signed outbound events for connected app automation')}
        actions={
          <Button
            type='button'
            size='sm'
            onClick={() => {
              setEditingWebhook(null)
              setSheetOpen(true)
            }}
          >
            <Plus data-icon='inline-start' />
            <span>{t('Webhook')}</span>
          </Button>
        }
      >
        <QueryState
          query={webhooksQuery}
          empty={webhooks.length === 0}
          emptyTitle={t('No webhooks')}
          emptyDescription={t('Create a webhook before enabling delivery.')}
        >
          <div className='overflow-x-auto'>
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
                {webhooks.map((webhook) => (
                  <ConnectedAppWebhookRow
                    key={webhook.id}
                    webhook={webhook}
                    testResult={testResults[webhook.id]}
                    isTesting={testMutation.isPending}
                    isDisabling={disableMutation.isPending}
                    onTest={() => testMutation.mutate(webhook.id)}
                    onEdit={() => {
                      setEditingWebhook(webhook)
                      setSheetOpen(true)
                    }}
                    onDisable={() => disableMutation.mutate(webhook.id)}
                  />
                ))}
              </TableBody>
            </Table>
          </div>
        </QueryState>
      </NotificationPanel>
      {sheetOpen ? (
        <ConnectedAppWebhookSheet
          key={`connected-app-webhook:${editingWebhook?.id ?? 'new'}`}
          webhook={editingWebhook}
          open={sheetOpen}
          onOpenChange={setSheetOpen}
        />
      ) : null}
    </>
  )
}

function ConnectedAppWebhookRow({
  webhook,
  testResult,
  isTesting,
  isDisabling,
  onTest,
  onEdit,
  onDisable,
}: {
  webhook: ConnectedAppWebhook
  testResult?: ConnectedAppWebhookTestResult
  isTesting: boolean
  isDisabling: boolean
  onTest: () => void
  onEdit: () => void
  onDisable: () => void
}) {
  const { t } = useTranslation()

  return (
    <TableRow>
      <TableCell className='min-w-44'>
        <div className='flex min-w-0 flex-col gap-1'>
          <span className='truncate font-medium'>{webhook.name}</span>
          <span className='text-muted-foreground text-xs'>
            #{webhook.id} · {formatOptionalTimestamp(webhook.updated_at)}
          </span>
        </div>
      </TableCell>
      <TableCell>
        <span className='block max-w-80 truncate font-mono text-xs'>
          {webhook.url}
        </span>
      </TableCell>
      <TableCell className='min-w-72'>
        <div className='flex max-w-72 flex-wrap gap-1'>
          {webhook.event_types.length === 0 ? (
            <Badge variant='secondary'>{t('All events')}</Badge>
          ) : (
            webhook.event_types.map((eventType) => (
              <Badge key={eventType} variant='secondary'>
                {eventType}
              </Badge>
            ))
          )}
        </div>
      </TableCell>
      <TableCell>
        <StatusBadge
          copyable={false}
          label={t(webhook.has_secret ? 'Set' : 'Not set')}
          variant={webhook.has_secret ? 'success' : 'neutral'}
        />
      </TableCell>
      <TableCell>
        <StatusBadge
          copyable={false}
          label={t(
            webhook.status === CONNECTED_APP_WEBHOOK_ENABLED
              ? 'Enabled'
              : 'Disabled'
          )}
          variant={
            webhook.status === CONNECTED_APP_WEBHOOK_ENABLED
              ? 'success'
              : 'neutral'
          }
        />
      </TableCell>
      <TableCell className='min-w-40'>
        {testResult ? (
          <div className='min-w-0 text-xs'>
            <div
              className={cn(
                'font-medium',
                testResult.success ? 'text-success' : 'text-destructive'
              )}
            >
              {t(testResult.success ? 'Success' : 'Failed')}{' '}
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
            type='button'
            variant='ghost'
            size='sm'
            disabled={isTesting}
            onClick={onTest}
          >
            <Send data-icon='inline-start' />
            <span>{t('Test')}</span>
          </Button>
          <Button type='button' variant='ghost' size='sm' onClick={onEdit}>
            <Webhook data-icon='inline-start' />
            <span>{t('Edit')}</span>
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            disabled={
              isDisabling || webhook.status === CONNECTED_APP_WEBHOOK_DISABLED
            }
            onClick={onDisable}
          >
            <Ban data-icon='inline-start' />
            <span>{t('Disable')}</span>
          </Button>
        </div>
      </TableCell>
    </TableRow>
  )
}

function ConnectedAppWebhookSheet({
  webhook,
  open,
  onOpenChange,
}: {
  webhook: ConnectedAppWebhook | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<WebhookFormState>(() => ({
    name: webhook?.name ?? '',
    url: webhook?.url ?? '',
    secret: '',
    eventTypes: webhookEventTypesForForm(webhook),
    status: String(webhook?.status ?? CONNECTED_APP_WEBHOOK_ENABLED),
  }))
  const editing = Boolean(webhook)

  const mutation = useMutation({
    mutationFn: () => {
      const payload = buildWebhookPayload(form)
      if (webhook) return updateConnectedAppWebhook(webhook.id, payload)
      return createConnectedAppWebhook(payload)
    },
    onSuccess: async () => {
      toast.success(t('Saved'))
      onOpenChange(false)
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['connected-apps', 'webhooks'],
        }),
        queryClient.invalidateQueries({
          queryKey: connectedAppNotificationAuditQueryKey(),
        }),
      ])
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    },
  })

  const toggleEvent = (eventType: string, checked: boolean) => {
    setForm((current) => ({
      ...current,
      eventTypes: checked
        ? Array.from(new Set([...current.eventTypes, eventType]))
        : current.eventTypes.filter((item) => item !== eventType),
    }))
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-2xl'>
        <SheetHeader>
          <SheetTitle>{t(editing ? 'Edit webhook' : 'New webhook')}</SheetTitle>
          <SheetDescription>
            {t('Configure a signed connected app notification endpoint')}
          </SheetDescription>
        </SheetHeader>
        <form
          className='flex min-h-0 flex-1 flex-col'
          onSubmit={(event) => {
            event.preventDefault()
            mutation.mutate()
          }}
        >
          <div className='flex-1 space-y-4 overflow-y-auto px-4 pb-4'>
            <div className='grid gap-3 sm:grid-cols-2'>
              <Field label={t('Name')}>
                <Input
                  value={form.name}
                  disabled={mutation.isPending}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      name: event.target.value,
                    }))
                  }
                />
              </Field>
              <Field label={t('Status')}>
                <Select
                  value={form.status}
                  onValueChange={(value) =>
                    setForm((current) => ({
                      ...current,
                      status: value ?? String(CONNECTED_APP_WEBHOOK_ENABLED),
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
            <Field label={t('Endpoint URL')}>
              <Input
                value={form.url}
                disabled={mutation.isPending}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    url: event.target.value,
                  }))
                }
                placeholder='https://example.com/webhooks/data-proxy'
              />
            </Field>
            <Field
              label={t(editing ? 'Secret (leave blank to keep)' : 'Secret')}
            >
              <Input
                type='password'
                value={form.secret}
                disabled={mutation.isPending}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    secret: event.target.value,
                  }))
                }
              />
            </Field>
            <div className='grid gap-2'>
              <div className='text-sm font-medium'>
                {t('Subscribed events')}
              </div>
              <div className='grid gap-2 rounded-lg border p-3 sm:grid-cols-2'>
                {CONNECTED_APP_NOTIFICATION_EVENTS.map((eventType) => (
                  <ScopeCheckbox
                    key={eventType}
                    label={eventType}
                    checked={form.eventTypes.includes(eventType)}
                    disabled={mutation.isPending}
                    onCheckedChange={(checked) =>
                      toggleEvent(eventType, checked)
                    }
                  />
                ))}
              </div>
            </div>
          </div>
          <SheetFooter className='border-t'>
            <Button
              type='button'
              variant='outline'
              disabled={mutation.isPending}
              onClick={() => onOpenChange(false)}
            >
              <X data-icon='inline-start' />
              <span>{t('Cancel')}</span>
            </Button>
            <Button type='submit' disabled={mutation.isPending}>
              <Check data-icon='inline-start' />
              <span>
                {t(mutation.isPending ? 'Saving...' : 'Save Changes')}
              </span>
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}

function ConnectedAppDeliveriesPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [channel, setChannel] = useState('')
  const [status, setStatus] = useState('')
  const [eventType, setEventType] = useState('')

  const outboxQuery = useQuery({
    queryKey: [
      'connected-apps',
      'notification-outbox',
      page,
      channel,
      status,
      eventType,
    ],
    queryFn: () =>
      listConnectedAppNotificationOutbox({
        p: page,
        page_size: CONNECTED_APP_DELIVERY_PAGE_SIZE,
        channel,
        status,
        event_type: eventType,
      }),
  })
  const metricsQuery = useQuery({
    queryKey: ['connected-apps', 'notification-outbox', 'worker-metrics'],
    queryFn: getConnectedAppNotificationOutboxWorkerMetrics,
  })
  const rows = outboxQuery.data?.items ?? []
  const total = outboxQuery.data?.total ?? 0
  const metrics = metricsQuery.data

  const retryMutation = useMutation({
    mutationFn: retryConnectedAppNotificationOutbox,
    onSuccess: async () => {
      toast.success(t('Queued for retry'))
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['connected-apps', 'notification-outbox'],
        }),
        queryClient.invalidateQueries({
          queryKey: connectedAppNotificationAuditQueryKey(),
        }),
      ])
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    },
  })

  const resetPage = () => setPage(1)

  return (
    <div className='space-y-3'>
      <ConnectedAppDeliveryMetrics query={metricsQuery} metrics={metrics} />
      <NotificationPanel
        title={t('Deliveries')}
        description={t('Recent connected app notification outbox rows')}
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
            <RefreshCw data-icon='inline-start' />
            <span>{t('Refresh')}</span>
          </Button>
        }
      >
        <div className='space-y-3'>
          <div className='flex flex-wrap gap-2'>
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
              <SelectTrigger className='w-64'>
                <SelectValue placeholder={t('Event Type')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value={ALL_VALUE}>{t('All Events')}</SelectItem>
                  {CONNECTED_APP_NOTIFICATION_EVENTS.map((item) => (
                    <SelectItem key={item} value={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <QueryState
            query={outboxQuery}
            empty={rows.length === 0}
            emptyTitle={t('No deliveries')}
            emptyDescription={t('Outbox rows will appear after events fire.')}
          >
            <div className='overflow-x-auto'>
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
                  {rows.map((row) => (
                    <ConnectedAppDeliveryRow
                      key={row.id}
                      row={row}
                      isRetrying={retryMutation.isPending}
                      onRetry={() => retryMutation.mutate(row.id)}
                    />
                  ))}
                </TableBody>
              </Table>
            </div>
            <Pager
              page={page}
              pageSize={CONNECTED_APP_DELIVERY_PAGE_SIZE}
              total={total}
              onPageChange={setPage}
            />
          </QueryState>
        </div>
      </NotificationPanel>
    </div>
  )
}

function ConnectedAppDeliveryMetrics({
  query,
  metrics,
}: {
  query: QueryViewState
  metrics?: ConnectedAppNotificationOutboxWorkerMetrics
}) {
  const { t } = useTranslation()

  return (
    <QueryState query={query}>
      <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-5'>
        <MetricCell
          icon={Activity}
          label={t('Last Claimed')}
          value={formatNumber(metrics?.last_run.claimed)}
          detail={formatOptionalTimestamp(metrics?.last_run.started_at)}
        />
        <MetricCell
          icon={Check}
          label={t('Last Sent')}
          value={formatNumber(metrics?.last_run.sent)}
          detail={`${t('Total')} ${formatNumber(metrics?.total_sent)}`}
        />
        <MetricCell
          icon={X}
          label={t('Last Failed')}
          value={formatNumber(metrics?.last_run.failed)}
          detail={`${t('Total')} ${formatNumber(metrics?.total_failed)}`}
        />
        <MetricCell
          icon={RotateCcw}
          label={t('Permanent Failed')}
          value={formatNumber(metrics?.total_permanent_failed)}
          detail={`${t('Runs')} ${formatNumber(metrics?.total_runs)}`}
        />
        <MetricCell
          icon={Clock}
          label={t('Duration')}
          value={formatDurationMs(metrics?.last_run.duration_ms)}
          detail={formatOptionalTimestamp(metrics?.last_run.finished_at)}
        />
      </div>
    </QueryState>
  )
}

function ConnectedAppDeliveryRow({
  row,
  isRetrying,
  onRetry,
}: {
  row: ConnectedAppNotificationOutbox
  isRetrying: boolean
  onRetry: () => void
}) {
  const { t } = useTranslation()

  return (
    <TableRow>
      <TableCell className='min-w-64'>
        <div className='min-w-0'>
          <div className='truncate font-mono text-xs'>{row.event_type}</div>
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
        <StatusBadge
          copyable={false}
          label={t(formatOutboxStatus(row.status))}
          variant={outboxStatusVariant(row.status)}
        />
      </TableCell>
      <TableCell className='tabular-nums'>{row.retry_count}</TableCell>
      <TableCell className='text-muted-foreground text-xs'>
        {formatOptionalTimestamp(row.next_retry_at)}
      </TableCell>
      <TableCell>
        <span className='block max-w-72 truncate text-xs'>
          {row.last_error || '-'}
        </span>
      </TableCell>
      <TableCell className='text-muted-foreground text-xs'>
        {formatOptionalTimestamp(row.created_at)}
      </TableCell>
      <TableCell>
        <div className='flex justify-end'>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            disabled={!isRetryableOutboxStatus(row.status) || isRetrying}
            onClick={onRetry}
          >
            <RotateCcw data-icon='inline-start' />
            <span>{t('Retry')}</span>
          </Button>
        </div>
      </TableCell>
    </TableRow>
  )
}

function NotificationPanel({
  title,
  description,
  actions,
  children,
}: {
  title: string
  description?: string
  actions?: ReactNode
  children: ReactNode
}) {
  return (
    <div className='bg-card overflow-hidden rounded-xl border'>
      <div className='flex flex-col gap-2 border-b px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between'>
        <div className='min-w-0'>
          <h3 className='text-sm font-medium'>{title}</h3>
          {description ? (
            <p className='text-muted-foreground text-xs'>{description}</p>
          ) : null}
        </div>
        {actions ? <div className='flex shrink-0 gap-2'>{actions}</div> : null}
      </div>
      <div className='p-3'>{children}</div>
    </div>
  )
}

function QueryState({
  query,
  empty,
  emptyTitle,
  emptyDescription,
  children,
}: {
  query: QueryViewState
  empty?: boolean
  emptyTitle?: string
  emptyDescription?: string
  children: ReactNode
}) {
  const { t } = useTranslation()

  if (query.isLoading) {
    return <ConnectedAppNotificationSkeleton />
  }

  if (query.isError) {
    return (
      <Alert variant='destructive'>
        <AlertTitle>{t('Failed to load data')}</AlertTitle>
        <AlertDescription>
          {query.error instanceof Error
            ? query.error.message
            : t('Request failed')}
        </AlertDescription>
      </Alert>
    )
  }

  if (empty) {
    return (
      <Empty className='border-0 py-8'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <BellRing />
          </EmptyMedia>
          <EmptyTitle>{emptyTitle}</EmptyTitle>
          {emptyDescription ? (
            <EmptyDescription>{emptyDescription}</EmptyDescription>
          ) : null}
        </EmptyHeader>
      </Empty>
    )
  }

  return <>{children}</>
}

function ConnectedAppNotificationSkeleton() {
  return (
    <div className='space-y-2'>
      {Array.from({ length: 4 }).map((_, index) => (
        <Skeleton key={index} className='h-10 w-full' />
      ))}
    </div>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className='grid gap-1.5'>
      <span className='text-sm font-medium'>{label}</span>
      {children}
    </label>
  )
}

function MetricCell({
  icon: Icon,
  label,
  value,
  detail,
}: {
  icon: typeof Activity
  label: string
  value: string
  detail: string
}) {
  return (
    <div className='rounded-xl border px-3 py-2.5'>
      <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
        <Icon className='size-3.5' />
        <span>{label}</span>
      </div>
      <div className='mt-2 text-base font-semibold tabular-nums'>{value}</div>
      <div className='text-muted-foreground mt-1 truncate text-xs'>
        {detail}
      </div>
    </div>
  )
}

function Pager({
  page,
  pageSize,
  total,
  onPageChange,
}: {
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation()
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  return (
    <div className='flex flex-col gap-2 border-t pt-3 sm:flex-row sm:items-center sm:justify-between'>
      <div className='text-muted-foreground text-xs tabular-nums'>
        {t('Total')}: {total.toLocaleString()}
      </div>
      <div className='flex items-center gap-2'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        >
          {t('Previous')}
        </Button>
        <span className='text-muted-foreground text-xs tabular-nums'>
          {page} / {totalPages}
        </span>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
        >
          {t('Next')}
        </Button>
      </div>
    </div>
  )
}

function connectedAppNotificationAuditQueryKey() {
  return ['connected-app-audit-logs']
}

function notificationPreferenceKey(channel: string, eventType: string) {
  return `${channel}:${eventType}`
}

function preferencePayload(
  preference: ConnectedAppNotificationPreference,
  scope: ConnectedAppNotificationRecipientScope = preference.recipient_scope,
  enabled = preference.enabled
): ConnectedAppNotificationPreferencePayload {
  return {
    app_id: preference.app_id,
    channel: preference.channel,
    event_type: preference.event_type,
    enabled,
    recipient_scope: scope,
  }
}

function buildWebhookPayload(
  form: WebhookFormState
): ConnectedAppWebhookPayload {
  const payload: ConnectedAppWebhookPayload = {
    app_id: CONNECTED_APP_NOTIFICATION_APP_ID,
    name: form.name.trim(),
    url: form.url.trim(),
    event_types: form.eventTypes,
    status: Number(form.status),
  }
  if (form.secret.trim()) {
    payload.secret = form.secret.trim()
  }
  return payload
}

function webhookEventTypesForForm(webhook: ConnectedAppWebhook | null) {
  if (!webhook || webhook.event_types.length === 0) {
    return [...CONNECTED_APP_NOTIFICATION_EVENTS]
  }
  if (webhook.event_types.includes('*')) {
    return [...CONNECTED_APP_NOTIFICATION_EVENTS]
  }
  return webhook.event_types
}

function parseEmailList(value: string) {
  return value
    .split(/[\s,，;；]+/)
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean)
}

function emailsToText(value: string[] = []) {
  return value.join(', ')
}

function notificationEventLabel(eventType: string) {
  switch (eventType) {
    case 'connected_app_request.approve':
      return 'Request approved'
    case 'connected_app_request.reject':
      return 'Request rejected'
    case 'connected_app_device.authorized':
      return 'Device authorized'
    case 'connected_app_device.denied':
      return 'Device denied'
    case 'connected_app.health.warning':
      return 'Health warning'
    default:
      return eventType
  }
}

function formatOptionalTimestamp(value?: number) {
  return value ? formatTimestampToDate(value) : '-'
}

function formatDurationMs(value?: number) {
  if (value === undefined || value === null) return '-'
  return `${value.toLocaleString()} ms`
}

function formatNumber(value?: number) {
  return (value ?? 0).toLocaleString()
}

function normalizeOptionalSelectValue(value: string | null) {
  if (!value || value === ALL_VALUE) return ''
  return value
}

function formatChannel(channel: string) {
  if (channel === 'in_app') return 'In-app'
  if (channel === 'email') return 'Email'
  if (channel === 'webhook') return 'Webhook'
  return channel || '-'
}

function formatRecipient(row: ConnectedAppNotificationOutbox) {
  if (row.channel === 'webhook') return row.recipient_email || '-'
  if (row.recipient_email) return row.recipient_email
  return row.recipient_user_id ? `user:${row.recipient_user_id}` : '-'
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

function outboxStatusVariant(status: string): StatusVariant {
  switch (status) {
    case 'sent':
      return 'success'
    case 'failed':
    case 'permanent_failed':
      return 'danger'
    case 'processing':
      return 'info'
    default:
      return 'warning'
  }
}

function isRetryableOutboxStatus(status: string) {
  return status === 'failed' || status === 'permanent_failed'
}
