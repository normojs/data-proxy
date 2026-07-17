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
import { z } from 'zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import {
  Check,
  Clock3,
  KeyRound,
  Loader2,
  Monitor,
  RefreshCw,
  ShieldCheck,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { ErrorState } from '@/components/error-state'
import { PublicLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { SnaplessActionNotice } from '@/features/snapless-device/action-notice'
import {
  authorizeSnaplessDevice,
  getSnaplessDeviceStatus,
  type SnaplessDeviceStatus,
  type SnaplessDeviceStatusResponse,
} from '@/features/snapless-device/api'

const searchSchema = z.object({
  user_code: z.string().optional(),
  app_slug: z.string().optional(),
})

export const Route = createFileRoute('/connect/device')({
  validateSearch: searchSchema,
  beforeLoad: ({ location }) => {
    const { auth } = useAuthStore.getState()
    if (!auth.user) {
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
      })
    }
  },
  component: ConnectDevicePage,
})

type StatusMeta = {
  label: string
  variant: StatusVariant
  icon: typeof Clock3
  actionText: string
}

function statusMeta(status: SnaplessDeviceStatus | undefined): StatusMeta {
  switch (status) {
    case 'authorized':
      return {
        label: 'Approved',
        variant: 'success',
        icon: ShieldCheck,
        actionText: 'You can return to the app.',
      }
    case 'consumed':
      return {
        label: 'Completed',
        variant: 'success',
        icon: Check,
        actionText: 'This authorization has already been used.',
      }
    case 'denied':
      return {
        label: 'Denied',
        variant: 'danger',
        icon: X,
        actionText: 'This device was not granted access.',
      }
    case 'expired':
      return {
        label: 'Expired',
        variant: 'warning',
        icon: Clock3,
        actionText: 'This authorization code is no longer active.',
      }
    default:
      return {
        label: 'Pending',
        variant: 'info',
        icon: Clock3,
        actionText: 'Review the app and scopes before approving.',
      }
  }
}

function normalizeUserCode(value: string) {
  const cleaned = value.trim().toUpperCase().replace(/\s+/g, '')
  if (cleaned.length === 8 && !cleaned.includes('-')) {
    return `${cleaned.slice(0, 4)}-${cleaned.slice(4)}`
  }
  return cleaned
}

function normalizeAppSlug(value?: string) {
  const slug = value?.trim()
  if (!slug || slug === 'snapless') return undefined
  return slug
}

function ConnectDevicePage() {
  const { t } = useTranslation()
  const search = Route.useSearch()
  const navigate = useNavigate({ from: Route.fullPath })
  const queryClient = useQueryClient()
  const userCode = normalizeUserCode(search.user_code ?? '')
  const appSlug = normalizeAppSlug(search.app_slug)
  const isConnectedApp = appSlug != null
  const title = isConnectedApp
    ? 'Authorize connected app'
    : 'Authorize device'
  const deviceStatusQueryKey = [
    'connect-device-status',
    appSlug ?? 'snapless',
    userCode,
  ]
  const [manualCode, setManualCode] = useState(userCode)

  const statusQuery = useQuery({
    queryKey: deviceStatusQueryKey,
    queryFn: () => getSnaplessDeviceStatus(userCode, appSlug),
    enabled: userCode.length > 0,
    retry: false,
  })

  const mutation = useMutation({
    mutationFn: (approve: boolean) =>
      authorizeSnaplessDevice({ user_code: userCode, approve }, appSlug),
    onSuccess: (data) => {
      queryClient.setQueryData(deviceStatusQueryKey, data)
      toast.success(
        data.status === 'authorized'
          ? t('Device authorization approved')
          : t('Device authorization denied')
      )
    },
  })

  const submitManualCode = () => {
    const nextCode = normalizeUserCode(manualCode)
    if (!nextCode) return
    navigate({
      search: {
        user_code: nextCode,
        ...(appSlug ? { app_slug: appSlug } : {}),
      },
    })
  }

  return (
    <PublicLayout
      showAuthButtons={false}
      showNotifications
      showThemeSwitch
      showMainContainer={false}
    >
      <main className='bg-muted/30 flex min-h-svh items-center justify-center px-4 py-20'>
        <div className='w-full max-w-[560px]'>
          {!userCode ? (
            <Card>
              <CardHeader>
                <CardTitle>{t(title)}</CardTitle>
                <CardDescription>
                  {t('Enter the code shown in the app requesting access.')}
                </CardDescription>
              </CardHeader>
              <CardContent className='space-y-3'>
                <Input
                  value={manualCode}
                  onChange={(event) => setManualCode(event.target.value)}
                  placeholder='ABCD-1234'
                  autoCapitalize='characters'
                  className='font-mono tracking-normal'
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') submitManualCode()
                  }}
                />
                <Button onClick={submitManualCode} disabled={!manualCode.trim()}>
                  <KeyRound data-icon='inline-start' />
                  {t('Continue')}
                </Button>
              </CardContent>
            </Card>
          ) : statusQuery.isLoading ? (
            <Card>
              <CardHeader>
                <CardTitle>{t(title)}</CardTitle>
                <CardDescription>
                  {t('Checking authorization code')}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <LoadingState inline message={t('Loading...')} />
              </CardContent>
            </Card>
          ) : statusQuery.isError ? (
            <Card>
              <ErrorState
                className='min-h-[260px]'
                title={t('Authorization code unavailable')}
                description={(statusQuery.error as Error).message}
                onRetry={() => statusQuery.refetch()}
              />
            </Card>
          ) : (
            <DeviceAuthorizationCard
              data={statusQuery.data}
              userCode={userCode}
              title={title}
              fallbackAppName={isConnectedApp ? 'Connected app' : 'Snapless Desktop'}
              isRefreshing={statusQuery.isFetching}
              isMutating={mutation.isPending}
              mutationError={mutation.error}
              onApprove={() => mutation.mutate(true)}
              onDeny={() => mutation.mutate(false)}
              onRefresh={() => statusQuery.refetch()}
            />
          )}
        </div>
      </main>
    </PublicLayout>
  )
}

function DeviceAuthorizationCard(props: {
  data?: SnaplessDeviceStatusResponse
  userCode: string
  title: string
  fallbackAppName: string
  isRefreshing: boolean
  isMutating: boolean
  mutationError: Error | null
  onApprove: () => void
  onDeny: () => void
  onRefresh: () => void
}) {
  const { t } = useTranslation()
  const data = props.data
  const appName = data?.app.name ?? props.fallbackAppName
  const meta = statusMeta(data?.status)
  const StatusIcon = meta.icon
  const canAct = data?.status === 'pending'
  const readiness = data?.readiness
  const canApprove = canAct && (readiness?.ok ?? true)
  const expiresAt = data?.expires_at
    ? new Date(data.expires_at * 1000).toLocaleString()
    : '-'
  const scopes =
    data?.app.default_scopes?.length
      ? data.app.default_scopes
      : data?.app.allowed_scopes?.length
        ? data.app.allowed_scopes
        : []

  return (
    <Card>
      <CardHeader>
        <div className='flex items-center gap-2'>
          <div className='bg-muted flex size-8 shrink-0 items-center justify-center rounded-lg'>
            <Monitor className='size-4' />
          </div>
          <div className='min-w-0'>
            <CardTitle>{t(props.title)}</CardTitle>
            <CardDescription className='truncate'>
              {appName}
              {data?.app.trusted ? ` · ${t('Trusted')}` : ''}
            </CardDescription>
          </div>
        </div>
        <CardAction>
          <StatusBadge
            copyable={false}
            icon={StatusIcon}
            label={t(meta.label)}
            variant={meta.variant}
          />
        </CardAction>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='grid gap-3 text-sm sm:grid-cols-2'>
          <InfoRow label={t('User code')} value={props.userCode} mono />
          <InfoRow label={t('Expires')} value={expiresAt} />
          <InfoRow
            label={t('Device')}
            value={data?.device.device_name || appName}
          />
          <InfoRow label={t('Platform')} value={data?.device.platform || '-'} />
        </div>

        {scopes.length > 0 ? (
          <div className='space-y-2'>
            <div className='text-muted-foreground text-xs font-medium'>
              {t('Permissions requested')}
            </div>
            <div className='flex flex-wrap gap-1.5'>
              {scopes.map((scope) => (
                <span
                  key={scope}
                  className='bg-muted rounded-md px-2 py-0.5 font-mono text-[11px]'
                >
                  {scope}
                </span>
              ))}
            </div>
          </div>
        ) : null}

        <Separator />

        <SnaplessActionNotice actions={readiness?.actions} />

        <div className='flex items-start gap-3 text-sm'>
          <KeyRound className='text-muted-foreground mt-0.5 size-4 shrink-0' />
          <div className='min-w-0 space-y-1'>
            <p className='font-medium'>
              {data?.status === 'authorized'
                ? t('{{appName}} can continue. You may close this tab.', {
                    appName,
                  })
                : t(meta.actionText)}
            </p>
            <p className='text-muted-foreground'>
              {t(
                'Approval issues a scoped API key for this app. You can revoke access later from Profile.'
              )}
            </p>
          </div>
        </div>

        {props.mutationError != null && (
          <p className='text-destructive text-sm'>
            {props.mutationError.message}
          </p>
        )}
      </CardContent>
      <CardFooter className='flex flex-col gap-2 sm:flex-row sm:justify-between'>
        <Button
          variant='outline'
          onClick={props.onRefresh}
          disabled={props.isRefreshing || props.isMutating}
          className='w-full sm:w-auto'
        >
          <RefreshCw
            data-icon='inline-start'
            className={cn(props.isRefreshing && 'animate-spin')}
          />
          {t('Refresh')}
        </Button>
        <div className='flex w-full gap-2 sm:w-auto'>
          <Button
            variant='outline'
            onClick={props.onDeny}
            disabled={!canAct || props.isMutating}
            className='flex-1 sm:flex-none'
          >
            {props.isMutating ? (
              <Loader2 data-icon='inline-start' className='animate-spin' />
            ) : (
              <X data-icon='inline-start' />
            )}
            {t('Deny')}
          </Button>
          <Button
            onClick={props.onApprove}
            disabled={!canApprove || props.isMutating}
            className='flex-1 sm:flex-none'
          >
            {props.isMutating ? (
              <Loader2 data-icon='inline-start' className='animate-spin' />
            ) : (
              <Check data-icon='inline-start' />
            )}
            {t('Approve')}
          </Button>
        </div>
      </CardFooter>
    </Card>
  )
}

function InfoRow(props: { label: string; value: string; mono?: boolean }) {
  return (
    <div className='min-w-0 space-y-1'>
      <div className='text-muted-foreground text-xs font-medium'>
        {props.label}
      </div>
      <div
        className={cn(
          'truncate text-sm font-medium',
          props.mono && 'font-mono tracking-normal'
        )}
        title={props.value}
      >
        {props.value}
      </div>
    </div>
  )
}
