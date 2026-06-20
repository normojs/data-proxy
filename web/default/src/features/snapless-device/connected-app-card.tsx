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
  AlertTriangle,
  CheckCircle2,
  Copy,
  KeyRound,
  Laptop,
  Loader2,
  RefreshCw,
  RotateCw,
  ShieldCheck,
  Trash2,
  XCircle,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import {
  getSnaplessDevices,
  revokeSnaplessDevice,
  rotateSnaplessDevice,
  type SnaplessHealthStatus,
  type SnaplessManagedDevice,
} from './api'

const QUERY_KEY = ['snapless-connected-app']
const TOKEN_STATUS_ENABLED = 1

type ConfirmAction = {
  type: 'rotate' | 'revoke'
  device: SnaplessManagedDevice
}

type RotatedKey = {
  apiKey: string
  deviceName: string
  tokenId?: number
}

function healthStatusMeta(status: SnaplessHealthStatus): {
  label: string
  variant: StatusVariant
} {
  switch (status) {
    case 'ok':
      return { label: 'Healthy', variant: 'success' }
    case 'app_disabled':
      return { label: 'App disabled', variant: 'danger' }
    case 'token_disabled':
      return { label: 'Token disabled', variant: 'danger' }
    case 'token_expired':
      return { label: 'Token expired', variant: 'warning' }
    case 'token_quota_insufficient':
      return { label: 'Token quota low', variant: 'warning' }
    case 'user_disabled':
      return { label: 'User disabled', variant: 'danger' }
    case 'grant_revoked':
      return { label: 'Grant revoked', variant: 'danger' }
    case 'binding_revoked':
      return { label: 'Device revoked', variant: 'danger' }
    case 'quota_insufficient':
      return { label: 'Balance low', variant: 'warning' }
    case 'models_unavailable':
      return { label: 'Models unavailable', variant: 'warning' }
    case 'token_not_found':
      return { label: 'Token missing', variant: 'danger' }
    case 'not_snapless_token':
      return { label: 'Not Snapless', variant: 'danger' }
    default:
      return { label: status || 'Unknown', variant: 'neutral' }
  }
}

function tokenStatusMeta(status?: number): {
  label: string
  variant: StatusVariant
} {
  switch (status) {
    case 1:
      return { label: 'Enabled', variant: 'success' }
    case 2:
      return { label: 'Disabled', variant: 'danger' }
    case 3:
      return { label: 'Expired', variant: 'warning' }
    case 4:
      return { label: 'Exhausted', variant: 'warning' }
    default:
      return { label: 'Unknown', variant: 'neutral' }
  }
}

function formatRelativeTimestamp(timestamp?: number) {
  if (!timestamp || timestamp <= 0) return 'Not used yet'
  return dayjs(timestamp * 1000).fromNow()
}

function shortFingerprint(fingerprint: string) {
  if (!fingerprint) return '-'
  if (fingerprint.length <= 14) return fingerprint
  return `${fingerprint.slice(0, 8)}...${fingerprint.slice(-6)}`
}

function deviceTitle(device: SnaplessManagedDevice) {
  return device.device.device_name || 'Snapless device'
}

function deviceSubtitle(device: SnaplessManagedDevice) {
  return [device.device.platform, device.device.app_version]
    .filter(Boolean)
    .join(' · ')
}

function SummaryItem({
  label,
  children,
}: {
  label: string
  children: ReactNode
}) {
  return (
    <div className='min-w-0 space-y-1'>
      <p className='text-muted-foreground text-xs'>{label}</p>
      <div className='flex min-h-6 min-w-0 items-center gap-2 text-sm font-medium'>
        {children}
      </div>
    </div>
  )
}

export function SnaplessConnectedAppCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { copyToClipboard } = useCopyToClipboard()
  const [confirm, setConfirm] = useState<ConfirmAction | null>(null)
  const [rotatedKey, setRotatedKey] = useState<RotatedKey | null>(null)

  const devicesQuery = useQuery({
    queryKey: QUERY_KEY,
    queryFn: getSnaplessDevices,
    retry: false,
  })

  const rotateMutation = useMutation({
    mutationFn: rotateSnaplessDevice,
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY })
      setConfirm(null)
      if (data.api_key) {
        setRotatedKey({
          apiKey: data.api_key,
          deviceName: data.device.device_name || t('Snapless device'),
          tokenId: data.token.id,
        })
      }
      toast.success(t('Snapless device key rotated'))
    },
    onError: (error) => {
      toast.error((error as Error).message || t('Failed to rotate device key'))
    },
  })

  const revokeMutation = useMutation({
    mutationFn: revokeSnaplessDevice,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY })
      setConfirm(null)
      toast.success(t('Snapless device revoked'))
    },
    onError: (error) => {
      toast.error((error as Error).message || t('Failed to revoke device'))
    },
  })

  const data = devicesQuery.data
  const activeCount = useMemo(
    () =>
      data?.devices.filter((item) => item.checks?.binding_active === true)
        .length ?? 0,
    [data?.devices]
  )
  const grantMeta =
    data?.grant.status === 'authorized'
      ? { label: 'Authorized', variant: 'success' as StatusVariant }
      : { label: 'Revoked', variant: 'danger' as StatusVariant }
  const appEnabled = data?.app.status === 1
  const isActionPending = rotateMutation.isPending || revokeMutation.isPending

  const executeConfirmAction = () => {
    if (!confirm) return
    const fingerprint = confirm.device.device.fingerprint
    if (confirm.type === 'rotate') {
      rotateMutation.mutate(fingerprint)
    } else {
      revokeMutation.mutate(fingerprint)
    }
  }

  return (
    <>
      <TitledCard
        title={t('Snapless Connected App')}
        description={t('Manage desktop device access and generated keys')}
        icon={<Laptop className='h-4 w-4' />}
        action={
          <Button
            variant='outline'
            size='sm'
            onClick={() => devicesQuery.refetch()}
            disabled={devicesQuery.isFetching}
          >
            <RefreshCw
              className={cn(
                'h-3.5 w-3.5',
                devicesQuery.isFetching && 'animate-spin'
              )}
            />
            {t('Refresh')}
          </Button>
        }
      >
        {devicesQuery.isLoading ? (
          <SnaplessConnectedAppSkeleton />
        ) : devicesQuery.isError ? (
          <div className='border-destructive/30 bg-destructive/5 flex items-start gap-3 rounded-lg border p-3'>
            <AlertTriangle className='text-destructive mt-0.5 h-4 w-4 shrink-0' />
            <div className='min-w-0 flex-1 space-y-2'>
              <div>
                <p className='text-sm font-medium'>
                  {t('Unable to load Snapless devices')}
                </p>
                <p className='text-muted-foreground text-xs'>
                  {(devicesQuery.error as Error).message}
                </p>
              </div>
              <Button
                variant='outline'
                size='sm'
                onClick={() => devicesQuery.refetch()}
              >
                {t('Retry')}
              </Button>
            </div>
          </div>
        ) : data ? (
          <div className='space-y-4'>
            <div className='grid gap-3 sm:grid-cols-3'>
              <SummaryItem label={t('Application')}>
                <span className='truncate'>{data.app.name}</span>
                <StatusBadge
                  label={t(appEnabled ? 'Enabled' : 'Disabled')}
                  variant={appEnabled ? 'success' : 'danger'}
                  copyable={false}
                />
              </SummaryItem>
              <SummaryItem label={t('Grant')}>
                <StatusBadge
                  label={t(grantMeta.label)}
                  variant={grantMeta.variant}
                  copyable={false}
                />
                <span className='text-muted-foreground truncate text-xs font-normal'>
                  {data.grant.scopes?.join(', ') || '-'}
                </span>
              </SummaryItem>
              <SummaryItem label={t('Devices')}>
                <span>
                  {activeCount}/{data.devices.length}
                </span>
                <span className='text-muted-foreground text-xs font-normal'>
                  {t('active')}
                </span>
              </SummaryItem>
            </div>

            {rotatedKey != null && (
              <Alert>
                <KeyRound className='h-4 w-4' />
                <AlertTitle>{t('New Snapless key shown once')}</AlertTitle>
                <AlertDescription>
                  <div className='mt-2 flex flex-col gap-2'>
                    <p>
                      {t('Device')}: {rotatedKey.deviceName}
                      {rotatedKey.tokenId ? ` · #${rotatedKey.tokenId}` : ''}
                    </p>
                    <div className='flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center'>
                      <code className='bg-muted min-w-0 flex-1 rounded-md px-2 py-1 text-xs break-all'>
                        {rotatedKey.apiKey}
                      </code>
                      <div className='flex gap-2'>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => copyToClipboard(rotatedKey.apiKey)}
                        >
                          <Copy className='h-3.5 w-3.5' />
                          {t('Copy')}
                        </Button>
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() => setRotatedKey(null)}
                        >
                          {t('Dismiss')}
                        </Button>
                      </div>
                    </div>
                  </div>
                </AlertDescription>
              </Alert>
            )}

            {data.devices.length === 0 ? (
              <div className='border-border/70 flex items-start gap-3 border-t pt-4'>
                <ShieldCheck className='text-muted-foreground mt-0.5 h-4 w-4 shrink-0' />
                <div>
                  <p className='text-sm font-medium'>
                    {t('No Snapless devices yet')}
                  </p>
                  <p className='text-muted-foreground text-xs'>
                    {t('Approved desktop devices will appear here.')}
                  </p>
                </div>
              </div>
            ) : (
              <div className='border-border/80 divide-border/80 divide-y overflow-hidden rounded-lg border'>
                {data.devices.map((device) => (
                  <SnaplessDeviceRow
                    key={device.device.fingerprint}
                    device={device}
                    actionPending={isActionPending}
                    rotating={
                      rotateMutation.isPending &&
                      confirm?.device.device.fingerprint ===
                        device.device.fingerprint
                    }
                    revoking={
                      revokeMutation.isPending &&
                      confirm?.device.device.fingerprint ===
                        device.device.fingerprint
                    }
                    onRotate={() => setConfirm({ type: 'rotate', device })}
                    onRevoke={() => setConfirm({ type: 'revoke', device })}
                  />
                ))}
              </div>
            )}
          </div>
        ) : null}
      </TitledCard>

      <AlertDialog
        open={confirm != null}
        onOpenChange={(open) => {
          if (!open && !isActionPending) setConfirm(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirm?.type === 'rotate'
                ? t('Rotate Snapless device key?')
                : t('Revoke Snapless device?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {confirm?.type === 'rotate'
                ? t(
                    'The current key for this device will stop working. The replacement key will be shown once.'
                  )
                : t(
                    'This removes Snapless access from this device and disables its key immediately.'
                  )}
              {confirm != null && (
                <span className='text-foreground mt-2 block font-medium'>
                  {deviceTitle(confirm.device)}
                </span>
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isActionPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault()
                executeConfirmAction()
              }}
              disabled={isActionPending}
              className={cn(
                confirm?.type === 'revoke' &&
                  'bg-destructive text-destructive-foreground hover:bg-destructive/90'
              )}
            >
              {isActionPending && (
                <Loader2 className='h-3.5 w-3.5 animate-spin' />
              )}
              {confirm?.type === 'rotate' ? t('Rotate key') : t('Revoke')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

function SnaplessDeviceRow({
  device,
  actionPending,
  rotating,
  revoking,
  onRotate,
  onRevoke,
}: {
  device: SnaplessManagedDevice
  actionPending: boolean
  rotating: boolean
  revoking: boolean
  onRotate: () => void
  onRevoke: () => void
}) {
  const { t } = useTranslation()
  const health = healthStatusMeta(device.status)
  const token = tokenStatusMeta(device.token.status)
  const active =
    device.checks?.binding_active === true &&
    device.token.binding_status === 'active' &&
    device.token.status === TOKEN_STATUS_ENABLED
  const subtitle = deviceSubtitle(device)
  const lastUsed = formatRelativeTimestamp(device.last_used_at)

  return (
    <div className='divide-border/80 border-border/80 divide-y'>
      <div className='grid gap-3 p-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-start'>
        <div className='min-w-0 space-y-2'>
          <div className='flex min-w-0 items-start gap-3'>
            <div
              className={cn(
                'bg-muted flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
                device.ok ? 'text-success' : 'text-muted-foreground'
              )}
            >
              {device.ok ? (
                <CheckCircle2 className='h-4 w-4' />
              ) : (
                <XCircle className='h-4 w-4' />
              )}
            </div>
            <div className='min-w-0 space-y-1'>
              <div className='flex flex-wrap items-center gap-2'>
                <p className='truncate text-sm font-medium'>
                  {deviceTitle(device)}
                </p>
                <StatusBadge
                  label={t(health.label)}
                  variant={health.variant}
                  copyable={false}
                />
              </div>
              <div className='text-muted-foreground flex flex-wrap items-center gap-x-2 gap-y-1 text-xs'>
                {subtitle && <span>{subtitle}</span>}
                <span title={device.device.fingerprint}>
                  {shortFingerprint(device.device.fingerprint)}
                </span>
              </div>
            </div>
          </div>

          <div className='text-muted-foreground grid gap-2 pl-11 text-xs sm:grid-cols-2'>
            <span>
              {t('Last used')}: {t(lastUsed)}
            </span>
            <span title={formatTimestampToDate(device.updated_at)}>
              {t('Updated')}: {formatTimestampToDate(device.updated_at)}
            </span>
          </div>
        </div>

        <div className='flex flex-wrap items-center gap-2 sm:justify-end'>
          <Button
            variant='outline'
            size='sm'
            onClick={onRotate}
            disabled={!active || actionPending}
          >
            {rotating ? (
              <Loader2 className='h-3.5 w-3.5 animate-spin' />
            ) : (
              <RotateCw className='h-3.5 w-3.5' />
            )}
            {t('Rotate')}
          </Button>
          <Button
            variant='destructive'
            size='sm'
            onClick={onRevoke}
            disabled={!active || actionPending}
          >
            {revoking ? (
              <Loader2 className='h-3.5 w-3.5 animate-spin' />
            ) : (
              <Trash2 className='h-3.5 w-3.5' />
            )}
            {t('Revoke')}
          </Button>
        </div>
      </div>

      <div className='bg-muted/25 flex flex-wrap items-center gap-2 px-3 py-2 pl-14 text-xs'>
        <StatusBadge
          label={t(token.label)}
          variant={token.variant}
          copyable={false}
        />
        <span className='text-muted-foreground'>
          {device.token.masked_key || t('No token')}
        </span>
        {device.token.model_limits && (
          <span className='text-muted-foreground truncate'>
            {device.token.model_limits}
          </span>
        )}
      </div>
    </div>
  )
}

function SnaplessConnectedAppSkeleton() {
  return (
    <div className='space-y-4'>
      <div className='grid gap-3 sm:grid-cols-3'>
        {Array.from({ length: 3 }).map((_, index) => (
          <div key={index} className='space-y-2'>
            <Skeleton className='h-3 w-20' />
            <Skeleton className='h-6 w-full' />
          </div>
        ))}
      </div>
      <div className='space-y-2'>
        {Array.from({ length: 2 }).map((_, index) => (
          <Skeleton key={index} className='h-24 w-full' />
        ))}
      </div>
    </div>
  )
}
