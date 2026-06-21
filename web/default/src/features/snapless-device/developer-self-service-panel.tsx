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
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  BarChart3,
  Download,
  Filter,
  KeyRound,
  RefreshCw,
  RotateCw,
  ShieldCheck,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  formatCompactNumber,
  formatQuota,
  formatTimestampToDate,
} from '@/lib/format'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
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
import { CopyButton } from '@/components/copy-button'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import {
  createConnectedAppDeveloperKey,
  getConnectedAppDeveloperOpenAPI,
  getConnectedAppDeveloperSDKConfig,
  getConnectedAppDeveloperUsage,
  listConnectedAppDeveloperAuthorizations,
  listConnectedAppDeveloperDeviceSessions,
  type ConnectedAppDeveloperAuthorization,
  type ConnectedAppDeveloperAuthorizationDevice,
  type ConnectedAppDeveloperDeviceSession,
  type ConnectedAppDeveloperKeyResponse,
  type ConnectedAppDeveloperSDKConfig,
  type ConnectedAppDeveloperUsageParams,
  type ConnectedAppDeveloperUsageByModel,
  type ConnectedAppDeveloperUsageByToken,
  type ConnectedAppRequest,
} from '@/features/system-settings/operations/connected-apps-api'

const developerSDKQueryKey = (appSlug: string) => [
  'connected-app-developer',
  'sdk-config',
  appSlug,
]

const developerUsageQueryKey = (appSlug: string) => [
  'connected-app-developer',
  'usage',
  appSlug,
]

const developerAuthorizationsQueryKey = (appSlug: string) => [
  'connected-app-developer',
  'authorizations',
  appSlug,
]

const developerDeviceSessionsQueryKey = (appSlug: string) => [
  'connected-app-developer',
  'device-sessions',
  appSlug,
]

const DEVICE_SESSION_STATUS_OPTIONS = [
  'all',
  'pending',
  'authorized',
  'consumed',
  'denied',
  'expired',
]

type DeveloperUsageFiltersState = {
  startDate: string
  endDate: string
  modelName: string
  tokenId: string
}

const EMPTY_DEVELOPER_USAGE_FILTERS: DeveloperUsageFiltersState = {
  startDate: '',
  endDate: '',
  modelName: '',
  tokenId: '',
}

export function ConnectedAppDeveloperSelfServicePanel({
  app,
}: {
  app: ConnectedAppRequest
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [deviceName, setDeviceName] = useState('Developer self-service')
  const [lastKeyResponse, setLastKeyResponse] =
    useState<ConnectedAppDeveloperKeyResponse | null>(null)

  const sdkConfigQuery = useQuery({
    queryKey: developerSDKQueryKey(app.slug),
    queryFn: () => getConnectedAppDeveloperSDKConfig(app.slug),
    enabled: Boolean(app.slug),
    retry: false,
  })

  const openAPIMutation = useMutation({
    mutationFn: () => getConnectedAppDeveloperOpenAPI(app.slug),
    onSuccess: (spec) => {
      downloadJSON(spec, `${app.slug || 'connected-app'}-openapi.json`)
      toast.success(t('OpenAPI spec downloaded'))
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Download failed'))
    },
  })

  const keyMutation = useMutation({
    mutationFn: (rotate: boolean) =>
      createConnectedAppDeveloperKey(app.slug, {
        device_name: deviceName.trim() || undefined,
        platform: 'web',
        client: 'profile-developer',
        rotate,
      }),
    onSuccess: (response) => {
      setLastKeyResponse(response)
      queryClient.invalidateQueries({
        queryKey: developerUsageQueryKey(app.slug),
      })
      if (response.api_key) {
        toast.success(
          response.rotated
            ? t('Developer key rotated')
            : t('Developer key created')
        )
        return
      }
      toast.success(t('Developer key reused'))
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Developer key request failed')
      )
    },
  })

  const sdkConfig = sdkConfigQuery.data
  const canCreateKey = sdkConfig?.permissions.can_create_key === true
  const canReadUsage = sdkConfig?.permissions.can_read_usage === true

  return (
    <div className='space-y-3'>
      <div className='grid gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(280px,0.42fr)]'>
        <DeveloperSDKPanel
          appSlug={app.slug}
          config={sdkConfig}
          loading={sdkConfigQuery.isLoading}
          error={sdkConfigQuery.error}
          downloading={openAPIMutation.isPending}
          onDownloadOpenAPI={() => openAPIMutation.mutate()}
        />

        <DeveloperKeyPanel
          canCreateKey={canCreateKey}
          loadingPermissions={sdkConfigQuery.isLoading}
          deviceName={deviceName}
          onDeviceNameChange={setDeviceName}
          pending={keyMutation.isPending}
          lastKeyResponse={lastKeyResponse}
          onCreate={() => keyMutation.mutate(false)}
          onRotate={() => keyMutation.mutate(true)}
        />
      </div>

      <DeveloperUsagePanel
        appSlug={app.slug}
        canReadUsage={canReadUsage}
        loadingPermissions={sdkConfigQuery.isLoading}
      />

      <DeveloperAuthorizationDiagnosticsPanel appSlug={app.slug} />
    </div>
  )
}

function DeveloperSDKPanel({
  appSlug,
  config,
  loading,
  error,
  downloading,
  onDownloadOpenAPI,
}: {
  appSlug: string
  config?: ConnectedAppDeveloperSDKConfig
  loading: boolean
  error: unknown
  downloading: boolean
  onDownloadOpenAPI: () => void
}) {
  const { t } = useTranslation()

  return (
    <section className='rounded-xl border p-3'>
      <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
        <div className='min-w-0'>
          <h4 className='text-sm font-medium'>{t('SDK & OpenAPI')}</h4>
          <p className='text-muted-foreground text-xs'>
            {t('OpenAI-compatible config for this connected app.')}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={onDownloadOpenAPI}
          disabled={downloading || loading || !appSlug}
        >
          {downloading ? (
            <RefreshCw className='h-3.5 w-3.5 animate-spin' />
          ) : (
            <Download className='h-3.5 w-3.5' />
          )}
          {t('OpenAPI')}
        </Button>
      </div>

      {loading ? (
        <DeveloperSDKSkeleton />
      ) : error ? (
        <Alert className='mt-3'>
          <AlertTitle>{t('Unable to load SDK config')}</AlertTitle>
          <AlertDescription>
            {error instanceof Error ? error.message : t('Request failed')}
          </AlertDescription>
        </Alert>
      ) : config ? (
        <div className='mt-3 space-y-3'>
          <div className='grid gap-2 sm:grid-cols-2'>
            <DeveloperConfigValue
              label={t('Base URL')}
              value={config.base_url}
              monospace
            />
            <DeveloperConfigValue
              label={t('API key env')}
              value={config.sdk.api_key_env}
              monospace
            />
            <DeveloperConfigValue
              label={t('Authorization')}
              value={config.sdk.authorization}
              monospace
            />
            <DeveloperConfigValue
              label={t('Compatible')}
              value={config.sdk.openai_compatible ? t('OpenAI') : '-'}
            />
          </div>

          <div className='space-y-1.5'>
            <div className='text-muted-foreground text-xs'>
              {t('Scoped API endpoints')}
            </div>
            <div className='rounded-lg border'>
              {Object.entries(config.api_endpoints).length > 0 ? (
                Object.entries(config.api_endpoints).map(([name, endpoint]) => (
                  <div
                    key={name}
                    className='flex min-w-0 items-center gap-2 border-b px-2.5 py-2 last:border-b-0'
                  >
                    <StatusBadge
                      copyable={false}
                      label={name}
                      variant='neutral'
                    />
                    <span
                      className='min-w-0 flex-1 truncate font-mono text-xs'
                      title={endpoint}
                    >
                      {endpoint}
                    </span>
                    <CopyButton
                      value={endpoint}
                      className='size-6'
                      tooltip={t('Copy endpoint')}
                    />
                  </div>
                ))
              ) : (
                <div className='text-muted-foreground px-2.5 py-2 text-xs'>
                  {t('No scoped endpoints')}
                </div>
              )}
            </div>
          </div>
        </div>
      ) : null}
    </section>
  )
}

function DeveloperKeyPanel({
  canCreateKey,
  loadingPermissions,
  deviceName,
  onDeviceNameChange,
  pending,
  lastKeyResponse,
  onCreate,
  onRotate,
}: {
  canCreateKey: boolean
  loadingPermissions: boolean
  deviceName: string
  onDeviceNameChange: (value: string) => void
  pending: boolean
  lastKeyResponse: ConnectedAppDeveloperKeyResponse | null
  onCreate: () => void
  onRotate: () => void
}) {
  const { t } = useTranslation()

  return (
    <section className='rounded-xl border p-3'>
      <div className='min-w-0'>
        <h4 className='text-sm font-medium'>{t('Developer key')}</h4>
        <p className='text-muted-foreground text-xs'>
          {t('Create, reuse, or rotate your own app-bound key.')}
        </p>
      </div>

      <div className='mt-3 space-y-2.5'>
        <label className='block space-y-1'>
          <span className='text-muted-foreground text-xs'>
            {t('Device name')}
          </span>
          <Input
            value={deviceName}
            onChange={(event) => onDeviceNameChange(event.target.value)}
            placeholder={t('Developer self-service')}
            disabled={!canCreateKey || pending}
          />
        </label>

        <div className='flex flex-col gap-2 sm:flex-row'>
          <Button
            type='button'
            size='sm'
            className='w-full sm:w-auto'
            disabled={!canCreateKey || pending || loadingPermissions}
            onClick={onCreate}
          >
            {pending ? (
              <RefreshCw className='h-3.5 w-3.5 animate-spin' />
            ) : (
              <KeyRound className='h-3.5 w-3.5' />
            )}
            {t('Create key')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='w-full sm:w-auto'
            disabled={!canCreateKey || pending || loadingPermissions}
            onClick={onRotate}
          >
            <RotateCw className='h-3.5 w-3.5' />
            {t('Rotate key')}
          </Button>
        </div>

        {!loadingPermissions && !canCreateKey && (
          <Alert>
            <AlertTitle>{t('token.manage scope required')}</AlertTitle>
            <AlertDescription>
              {t('This app cannot create developer keys.')}
            </AlertDescription>
          </Alert>
        )}

        {lastKeyResponse?.api_key ? (
          <Alert className='bg-muted/40'>
            <AlertTitle>{t('New API key shown once')}</AlertTitle>
            <AlertDescription>
              <div className='bg-background mt-2 flex min-w-0 items-center gap-2 rounded-lg border px-2 py-1.5'>
                <code className='min-w-0 flex-1 truncate text-xs'>
                  {lastKeyResponse.api_key}
                </code>
                <CopyButton
                  value={lastKeyResponse.api_key}
                  className='size-6'
                  tooltip={t('Copy API key')}
                />
              </div>
            </AlertDescription>
          </Alert>
        ) : lastKeyResponse ? (
          <p className='text-muted-foreground text-xs'>
            {t('Existing developer token reused. Rotate to reveal a new key.')}
          </p>
        ) : null}
      </div>
    </section>
  )
}

function DeveloperUsagePanel({
  appSlug,
  canReadUsage,
  loadingPermissions,
}: {
  appSlug: string
  canReadUsage: boolean
  loadingPermissions: boolean
}) {
  const { t } = useTranslation()
  const [draftFilters, setDraftFilters] = useState<DeveloperUsageFiltersState>(
    () => ({ ...EMPTY_DEVELOPER_USAGE_FILTERS })
  )
  const [appliedFilters, setAppliedFilters] =
    useState<DeveloperUsageFiltersState>(() => ({
      ...EMPTY_DEVELOPER_USAGE_FILTERS,
    }))
  const usageParams = developerUsageParams(appliedFilters)
  const invalidRange =
    Boolean(draftFilters.startDate && draftFilters.endDate) &&
    draftFilters.startDate > draftFilters.endDate
  const usageQuery = useQuery({
    queryKey: [...developerUsageQueryKey(appSlug), usageParams],
    queryFn: () => getConnectedAppDeveloperUsage(appSlug, usageParams),
    enabled: Boolean(appSlug) && canReadUsage,
    retry: false,
  })
  const setDraftFilter = (
    key: keyof DeveloperUsageFiltersState,
    value: string
  ) => {
    setDraftFilters((current) => ({ ...current, [key]: value }))
  }
  const clearFilters = () => {
    const emptyFilters = { ...EMPTY_DEVELOPER_USAGE_FILTERS }
    setDraftFilters(emptyFilters)
    setAppliedFilters(emptyFilters)
  }

  return (
    <section className='rounded-xl border p-3'>
      <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
        <div className='min-w-0'>
          <h4 className='flex items-center gap-2 text-sm font-medium'>
            <BarChart3 className='h-4 w-4' />
            {t('Usage summary')}
          </h4>
          <p className='text-muted-foreground text-xs'>
            {t('Current and historical app-bound token usage.')}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => usageQuery.refetch()}
          disabled={!canReadUsage || usageQuery.isFetching}
        >
          <RefreshCw
            className={
              usageQuery.isFetching ? 'h-3.5 w-3.5 animate-spin' : 'h-3.5 w-3.5'
            }
          />
          {t('Refresh')}
        </Button>
      </div>

      {!loadingPermissions && canReadUsage ? (
        <DeveloperUsageFilters
          filters={draftFilters}
          invalidRange={invalidRange}
          applying={usageQuery.isFetching}
          canClear={
            hasDeveloperUsageFilters(draftFilters) ||
            hasDeveloperUsageFilters(appliedFilters)
          }
          onChange={setDraftFilter}
          onApply={() => setAppliedFilters({ ...draftFilters })}
          onClear={clearFilters}
        />
      ) : null}

      {loadingPermissions ? (
        <DeveloperUsageSkeleton />
      ) : !canReadUsage ? (
        <Alert className='mt-3'>
          <AlertTitle>{t('quota.read scope required')}</AlertTitle>
          <AlertDescription>
            {t('This app cannot read developer usage.')}
          </AlertDescription>
        </Alert>
      ) : usageQuery.isLoading ? (
        <DeveloperUsageSkeleton />
      ) : usageQuery.isError ? (
        <Alert className='mt-3'>
          <AlertTitle>{t('Unable to load developer usage')}</AlertTitle>
          <AlertDescription>
            {usageQuery.error instanceof Error
              ? usageQuery.error.message
              : t('Request failed')}
          </AlertDescription>
        </Alert>
      ) : usageQuery.data ? (
        <div className='mt-3 space-y-3'>
          <div className='grid gap-2 sm:grid-cols-2 xl:grid-cols-4'>
            <DeveloperMetric
              label={t('Requests')}
              value={formatCompactNumber(usageQuery.data.total.request_count)}
            />
            <DeveloperMetric
              label={t('Quota')}
              value={formatQuota(usageQuery.data.total.quota)}
            />
            <DeveloperMetric
              label={t('Tokens')}
              value={formatCompactNumber(
                usageQuery.data.total.prompt_tokens +
                  usageQuery.data.total.completion_tokens
              )}
            />
            <DeveloperMetric
              label={t('Tracked keys')}
              value={formatCompactNumber(usageQuery.data.token_count)}
            />
          </div>

          <div className='grid gap-3 xl:grid-cols-2'>
            <UsageByModelTable items={usageQuery.data.by_model} />
            <UsageByTokenTable items={usageQuery.data.by_token} />
          </div>
        </div>
      ) : null}
    </section>
  )
}

function DeveloperUsageFilters({
  filters,
  invalidRange,
  applying,
  canClear,
  onChange,
  onApply,
  onClear,
}: {
  filters: DeveloperUsageFiltersState
  invalidRange: boolean
  applying: boolean
  canClear: boolean
  onChange: (key: keyof DeveloperUsageFiltersState, value: string) => void
  onApply: () => void
  onClear: () => void
}) {
  const { t } = useTranslation()

  return (
    <div className='mt-3 space-y-2'>
      <div className='grid gap-2 sm:grid-cols-2 xl:grid-cols-[9rem_9rem_minmax(12rem,1fr)_9rem_auto]'>
        <label className='block min-w-0 space-y-1'>
          <span className='text-muted-foreground text-xs'>
            {t('Start date')}
          </span>
          <Input
            type='date'
            value={filters.startDate}
            onChange={(event) => onChange('startDate', event.target.value)}
            className='h-8'
          />
        </label>
        <label className='block min-w-0 space-y-1'>
          <span className='text-muted-foreground text-xs'>{t('End date')}</span>
          <Input
            type='date'
            value={filters.endDate}
            onChange={(event) => onChange('endDate', event.target.value)}
            className='h-8'
          />
        </label>
        <label className='block min-w-0 space-y-1'>
          <span className='text-muted-foreground text-xs'>
            {t('Model name')}
          </span>
          <Input
            value={filters.modelName}
            onChange={(event) => onChange('modelName', event.target.value)}
            placeholder={t('All models')}
            className='h-8'
          />
        </label>
        <label className='block min-w-0 space-y-1'>
          <span className='text-muted-foreground text-xs'>{t('Token ID')}</span>
          <Input
            type='number'
            min={1}
            step={1}
            value={filters.tokenId}
            onChange={(event) => onChange('tokenId', event.target.value)}
            placeholder={t('All')}
            className='h-8'
          />
        </label>
        <div className='flex items-end gap-2'>
          <Button
            type='button'
            size='sm'
            onClick={onApply}
            disabled={invalidRange || applying}
          >
            <Filter className='h-3.5 w-3.5' />
            {t('Apply')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={onClear}
            disabled={!canClear || applying}
          >
            <X className='h-3.5 w-3.5' />
            {t('Clear')}
          </Button>
        </div>
      </div>
      {invalidRange ? (
        <p className='text-destructive text-xs'>
          {t('End date must be after start date.')}
        </p>
      ) : null}
    </div>
  )
}

function DeveloperConfigValue({
  label,
  value,
  monospace,
}: {
  label: string
  value: string
  monospace?: boolean
}) {
  return (
    <div className='bg-muted/20 min-w-0 rounded-lg border px-2.5 py-2'>
      <div className='flex min-w-0 items-center justify-between gap-2'>
        <span className='text-muted-foreground text-xs'>{label}</span>
        {value ? (
          <CopyButton value={value} className='size-6' tooltip='Copy value' />
        ) : null}
      </div>
      <div
        className={
          monospace
            ? 'mt-1 truncate font-mono text-xs'
            : 'mt-1 truncate text-sm font-medium'
        }
        title={value}
      >
        {value || '-'}
      </div>
    </div>
  )
}

function DeveloperAuthorizationDiagnosticsPanel({
  appSlug,
}: {
  appSlug: string
}) {
  const { t } = useTranslation()
  const [sessionStatus, setSessionStatus] = useState('all')
  const sessionStatusParam = sessionStatus === 'all' ? undefined : sessionStatus
  const authorizationsQuery = useQuery({
    queryKey: [...developerAuthorizationsQueryKey(appSlug), 5],
    queryFn: () =>
      listConnectedAppDeveloperAuthorizations(appSlug, { page_size: 5 }),
    enabled: Boolean(appSlug),
    retry: false,
  })
  const sessionsQuery = useQuery({
    queryKey: [
      ...developerDeviceSessionsQueryKey(appSlug),
      sessionStatusParam ?? '',
      8,
    ],
    queryFn: () =>
      listConnectedAppDeveloperDeviceSessions(appSlug, {
        page_size: 8,
        status: sessionStatusParam,
      }),
    enabled: Boolean(appSlug),
    retry: false,
  })
  const refreshing = authorizationsQuery.isFetching || sessionsQuery.isFetching

  return (
    <section className='rounded-xl border p-3'>
      <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
        <div className='min-w-0'>
          <h4 className='flex items-center gap-2 text-sm font-medium'>
            <ShieldCheck className='h-4 w-4' />
            {t('Authorization diagnostics')}
          </h4>
          <p className='text-muted-foreground text-xs'>
            {t('Recent grants, devices, and device sessions.')}
          </p>
        </div>
        <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
          <Select
            value={sessionStatus}
            onValueChange={(value) => setSessionStatus(value ?? 'all')}
          >
            <SelectTrigger className='w-full sm:w-40' size='sm'>
              <SelectValue placeholder={t('Session status')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {DEVICE_SESSION_STATUS_OPTIONS.map((status) => (
                  <SelectItem key={status} value={status}>
                    {status === 'all' ? t('All sessions') : t(status)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => {
              void authorizationsQuery.refetch()
              void sessionsQuery.refetch()
            }}
            disabled={refreshing}
          >
            <RefreshCw
              className={
                refreshing ? 'h-3.5 w-3.5 animate-spin' : 'h-3.5 w-3.5'
              }
            />
            {t('Refresh')}
          </Button>
        </div>
      </div>

      <div className='mt-3 grid gap-3 xl:grid-cols-2'>
        <DeveloperAuthorizationsTable
          items={authorizationsQuery.data?.items ?? []}
          loading={authorizationsQuery.isLoading}
          error={authorizationsQuery.error}
        />
        <DeveloperDeviceSessionsTable
          items={sessionsQuery.data?.items ?? []}
          loading={sessionsQuery.isLoading}
          error={sessionsQuery.error}
        />
      </div>
    </section>
  )
}

function DeveloperAuthorizationsTable({
  items,
  loading,
  error,
}: {
  items: ConnectedAppDeveloperAuthorization[]
  loading: boolean
  error: unknown
}) {
  const { t } = useTranslation()

  if (loading) {
    return <Skeleton className='h-56 w-full' />
  }

  if (error) {
    return (
      <Alert>
        <AlertTitle>{t('Unable to load authorizations')}</AlertTitle>
        <AlertDescription>
          {error instanceof Error ? error.message : t('Request failed')}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className='rounded-lg border'>
      <div className='border-b px-2.5 py-2 text-sm font-medium'>
        {t('Authorizations')}
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('User')}</TableHead>
            <TableHead>{t('Grant')}</TableHead>
            <TableHead>{t('Devices')}</TableHead>
            <TableHead>{t('Last used')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.length > 0 ? (
            items.map((item) => (
              <TableRow key={item.user_id}>
                <TableCell className='max-w-[9rem]'>
                  <div className='truncate font-medium'>
                    {item.user_name || `#${item.user_id}`}
                  </div>
                  <div className='text-muted-foreground truncate text-xs'>
                    #{item.user_id}
                  </div>
                </TableCell>
                <TableCell className='max-w-[10rem]'>
                  <StatusBadge
                    copyable={false}
                    label={t(item.grant.status || 'unknown')}
                    variant={connectedAppStatusVariant(item.grant.status)}
                  />
                  <div className='text-muted-foreground mt-1 truncate text-xs'>
                    {item.grant.scopes.join(', ') || '-'}
                  </div>
                </TableCell>
                <TableCell className='max-w-[12rem]'>
                  <DeveloperAuthorizationDeviceSummary devices={item.devices} />
                </TableCell>
                <TableCell className='text-muted-foreground text-xs whitespace-nowrap'>
                  {formatTimestampToDate(
                    item.grant.last_used_at || item.grant.authorized_at
                  )}
                </TableCell>
              </TableRow>
            ))
          ) : (
            <TableRow>
              <TableCell
                className='text-muted-foreground text-center'
                colSpan={4}
              >
                {t('No authorizations yet')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}

function DeveloperAuthorizationDeviceSummary({
  devices,
}: {
  devices: ConnectedAppDeveloperAuthorizationDevice[]
}) {
  const { t } = useTranslation()

  if (devices.length === 0) {
    return <span className='text-muted-foreground text-xs'>-</span>
  }

  return (
    <div className='space-y-1'>
      {devices.slice(0, 2).map((device) => (
        <div
          key={`${device.device.fingerprint}-${device.token.id ?? 0}`}
          className='min-w-0'
        >
          <div className='flex min-w-0 items-center gap-1.5'>
            <StatusBadge
              copyable={false}
              label={t(device.status || 'unknown')}
              variant={connectedAppStatusVariant(device.status)}
            />
            <span className='truncate text-xs font-medium'>
              {device.device.device_name || device.device.fingerprint || '-'}
            </span>
          </div>
          <div className='text-muted-foreground truncate text-xs'>
            {device.token.id ? `#${device.token.id}` : '-'}
          </div>
        </div>
      ))}
      {devices.length > 2 ? (
        <div className='text-muted-foreground text-xs'>
          {t('{{count}} more devices', { count: devices.length - 2 })}
        </div>
      ) : null}
    </div>
  )
}

function DeveloperDeviceSessionsTable({
  items,
  loading,
  error,
}: {
  items: ConnectedAppDeveloperDeviceSession[]
  loading: boolean
  error: unknown
}) {
  const { t } = useTranslation()

  if (loading) {
    return <Skeleton className='h-56 w-full' />
  }

  if (error) {
    return (
      <Alert>
        <AlertTitle>{t('Unable to load device sessions')}</AlertTitle>
        <AlertDescription>
          {error instanceof Error ? error.message : t('Request failed')}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className='rounded-lg border'>
      <div className='border-b px-2.5 py-2 text-sm font-medium'>
        {t('Device sessions')}
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Session')}</TableHead>
            <TableHead>{t('Device')}</TableHead>
            <TableHead>{t('Token')}</TableHead>
            <TableHead>{t('Latest event')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.length > 0 ? (
            items.map((item) => {
              const latestEvent = latestDeviceSessionEvent(item)
              return (
                <TableRow key={item.id}>
                  <TableCell className='max-w-[9rem]'>
                    <StatusBadge
                      copyable={false}
                      label={t(item.status || 'unknown')}
                      variant={connectedAppStatusVariant(item.status)}
                    />
                    <div className='text-muted-foreground mt-1 truncate text-xs'>
                      #{item.id}
                    </div>
                  </TableCell>
                  <TableCell className='max-w-[11rem]'>
                    <div className='truncate font-medium'>
                      {item.device.device_name || '-'}
                    </div>
                    <div className='text-muted-foreground truncate text-xs'>
                      {item.user_name || `#${item.user_id || 0}`}
                    </div>
                  </TableCell>
                  <TableCell className='text-xs whitespace-nowrap'>
                    {item.token_id > 0 ? `#${item.token_id}` : '-'}
                    {item.token_created ? (
                      <div className='text-muted-foreground'>
                        {t('created')}
                      </div>
                    ) : null}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-xs whitespace-nowrap'>
                    <div>{t(latestEvent.label)}</div>
                    <div>{formatTimestampToDate(latestEvent.timestamp)}</div>
                  </TableCell>
                </TableRow>
              )
            })
          ) : (
            <TableRow>
              <TableCell
                className='text-muted-foreground text-center'
                colSpan={4}
              >
                {t('No device sessions yet')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}

function DeveloperMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className='bg-muted/20 min-w-0 rounded-lg border px-2.5 py-2'>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className='mt-1 truncate text-sm font-semibold tabular-nums'>
        {value}
      </div>
    </div>
  )
}

function UsageByModelTable({
  items,
}: {
  items: ConnectedAppDeveloperUsageByModel[]
}) {
  const { t } = useTranslation()

  return (
    <div className='rounded-lg border'>
      <div className='border-b px-2.5 py-2 text-sm font-medium'>
        {t('By model')}
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Model')}</TableHead>
            <TableHead className='text-right'>{t('Requests')}</TableHead>
            <TableHead className='text-right'>{t('Quota')}</TableHead>
            <TableHead className='text-right'>{t('Tokens')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.length > 0 ? (
            items.slice(0, 8).map((item) => (
              <TableRow key={item.model_name}>
                <TableCell className='max-w-[12rem] truncate font-medium'>
                  {item.model_name || '-'}
                </TableCell>
                <TableCell className='text-right'>
                  {formatCompactNumber(item.request_count)}
                </TableCell>
                <TableCell className='text-right'>
                  {formatQuota(item.quota)}
                </TableCell>
                <TableCell className='text-right'>
                  {formatCompactNumber(
                    item.prompt_tokens + item.completion_tokens
                  )}
                </TableCell>
              </TableRow>
            ))
          ) : (
            <TableRow>
              <TableCell
                className='text-muted-foreground text-center'
                colSpan={4}
              >
                {t('No usage yet')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}

function UsageByTokenTable({
  items,
}: {
  items: ConnectedAppDeveloperUsageByToken[]
}) {
  const { t } = useTranslation()

  return (
    <div className='rounded-lg border'>
      <div className='border-b px-2.5 py-2 text-sm font-medium'>
        {t('By token')}
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Token')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead className='text-right'>{t('Requests')}</TableHead>
            <TableHead className='text-right'>{t('Quota')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.length > 0 ? (
            items.slice(0, 8).map((item) => (
              <TableRow key={item.token_id}>
                <TableCell className='max-w-[12rem]'>
                  <div className='truncate font-medium'>
                    {item.token_name || `#${item.token_id}`}
                  </div>
                  <div className='text-muted-foreground truncate text-xs'>
                    {item.device.device_name || `#${item.token_id}`}
                  </div>
                </TableCell>
                <TableCell>
                  <StatusBadge
                    copyable={false}
                    label={t(item.status || 'unknown')}
                    variant={tokenStatusVariant(item.status)}
                  />
                </TableCell>
                <TableCell className='text-right'>
                  {formatCompactNumber(item.request_count)}
                </TableCell>
                <TableCell className='text-right'>
                  {formatQuota(item.quota)}
                </TableCell>
              </TableRow>
            ))
          ) : (
            <TableRow>
              <TableCell
                className='text-muted-foreground text-center'
                colSpan={4}
              >
                {t('No token usage yet')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}

function DeveloperSDKSkeleton() {
  return (
    <div className='mt-3 space-y-3'>
      <div className='grid gap-2 sm:grid-cols-2'>
        {Array.from({ length: 4 }).map((_, index) => (
          <Skeleton key={index} className='h-14 w-full' />
        ))}
      </div>
      <Skeleton className='h-28 w-full' />
    </div>
  )
}

function DeveloperUsageSkeleton() {
  return (
    <div className='mt-3 space-y-3'>
      <div className='grid gap-2 sm:grid-cols-2 xl:grid-cols-4'>
        {Array.from({ length: 4 }).map((_, index) => (
          <Skeleton key={index} className='h-14 w-full' />
        ))}
      </div>
      <div className='grid gap-3 xl:grid-cols-2'>
        <Skeleton className='h-48 w-full' />
        <Skeleton className='h-48 w-full' />
      </div>
    </div>
  )
}

function tokenStatusVariant(status: string): StatusVariant {
  switch (status) {
    case 'active':
      return 'success'
    case 'revoked':
      return 'danger'
    case 'historical':
      return 'neutral'
    default:
      return 'info'
  }
}

function connectedAppStatusVariant(status: string): StatusVariant {
  switch (status) {
    case 'active':
    case 'authorized':
    case 'consumed':
      return 'success'
    case 'pending':
      return 'warning'
    case 'denied':
    case 'rejected':
    case 'revoked':
      return 'danger'
    case 'expired':
    case 'historical':
      return 'neutral'
    default:
      return 'info'
  }
}

function latestDeviceSessionEvent(session: ConnectedAppDeveloperDeviceSession) {
  if (session.consumed_at > 0) {
    return { label: 'Consumed', timestamp: session.consumed_at }
  }
  if (session.authorized_at > 0) {
    return { label: 'Authorized', timestamp: session.authorized_at }
  }
  if (session.last_polled_at > 0) {
    return { label: 'Polled', timestamp: session.last_polled_at }
  }
  if (session.expires_at > 0 && session.status === 'expired') {
    return { label: 'Expired', timestamp: session.expires_at }
  }
  return { label: 'Created', timestamp: session.created_at }
}

function hasDeveloperUsageFilters(filters: DeveloperUsageFiltersState) {
  return Object.values(filters).some((value) => value.trim() !== '')
}

function developerUsageParams(
  filters: DeveloperUsageFiltersState
): ConnectedAppDeveloperUsageParams {
  return {
    start_time: startOfDayUnix(filters.startDate),
    end_time: endOfDayUnix(filters.endDate),
    model_name: filters.modelName.trim() || undefined,
    token_id: parsePositiveIntInput(filters.tokenId),
  }
}

function startOfDayUnix(value: string) {
  if (!value) return undefined
  const date = new Date(`${value}T00:00:00`)
  if (Number.isNaN(date.getTime())) return undefined
  return Math.floor(date.getTime() / 1000)
}

function endOfDayUnix(value: string) {
  if (!value) return undefined
  const date = new Date(`${value}T23:59:59`)
  if (Number.isNaN(date.getTime())) return undefined
  return Math.floor(date.getTime() / 1000)
}

function parsePositiveIntInput(value: string) {
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed <= 0) return undefined
  return parsed
}

function downloadJSON(data: unknown, filename: string) {
  if (typeof document === 'undefined') return
  const blob = new Blob([JSON.stringify(data, null, 2)], {
    type: 'application/json',
  })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
}
