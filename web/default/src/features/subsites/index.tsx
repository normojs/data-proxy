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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import {
  AlertCircle,
  ArrowRight,
  CalendarX,
  Clock3,
  Eye,
  EyeOff,
  ExternalLink,
  Gauge,
  KeyRound,
  LockKeyhole,
  Megaphone,
  Plus,
  RefreshCw,
  ShieldCheck,
  UserX,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { formatLogQuota, formatTokenVolume, formatUseTime } from '@/lib/format'
import { formatDateTimeObject } from '@/lib/time'
import { cn } from '@/lib/utils'
import { useStatus } from '@/hooks/use-status'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
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
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { PublicLayout } from '@/components/layout'
import { TermsFooter } from '@/features/auth/components/terms-footer'
import { UserAuthForm } from '@/features/auth/sign-in/components/user-auth-form'
import { SignUpForm } from '@/features/auth/sign-up/components/sign-up-form'
import {
  ensureSubsiteToken,
  getSubsiteDashboard,
  getPublicSubsite,
  getSubsiteTokenKey,
  registerSubsiteUser,
  rotateSubsiteToken,
} from './api'
import type {
  PublicSubsite,
  SubsiteDashboard as SubsiteDashboardData,
  SubsiteDashboardResponse,
  SubsiteQuotaMetric,
  SubsiteRecentLog,
  SubsiteRuntimeStatus,
  SubsiteTokenInfo,
} from './types'

type SubsiteEntryProps = {
  slug: string
}

type StatusView = {
  icon: React.ComponentType<{ className?: string; 'aria-hidden'?: boolean }>
  tone: 'neutral' | 'success' | 'warning' | 'destructive'
  label: string
  title: string
  description: string
}

function sanitizeThemeColor(value?: string) {
  const color = value?.trim()
  if (!color) return undefined
  return /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.test(color)
    ? color
    : undefined
}

function statusView(
  status: SubsiteRuntimeStatus | 'not_found',
  reason?: string
): StatusView {
  switch (status) {
    case 'enabled':
      return {
        icon: ShieldCheck,
        tone: 'success',
        label: 'Available',
        title: 'Subsite is open',
        description:
          'This subsite is available. Sign in or continue to the subsite console.',
      }
    case 'disabled':
      return {
        icon: LockKeyhole,
        tone: 'warning',
        label: 'Closed',
        title: 'Subsite is closed',
        description:
          reason ||
          'The administrator has temporarily closed this subsite. Please check back later.',
      }
    case 'not_started':
      return {
        icon: Clock3,
        tone: 'neutral',
        label: 'Not started',
        title: 'Subsite has not started',
        description:
          'The access window for this subsite has not started yet. Please return after it opens.',
      }
    case 'expired':
      return {
        icon: CalendarX,
        tone: 'destructive',
        label: 'Ended',
        title: 'Subsite has ended',
        description:
          'The access window for this subsite has ended. Existing links are no longer available.',
      }
    case 'draft':
      return {
        icon: AlertCircle,
        tone: 'neutral',
        label: 'Draft',
        title: 'Subsite is not published',
        description:
          'This subsite is still in draft mode and is not available to visitors.',
      }
    default:
      return {
        icon: AlertCircle,
        tone: 'destructive',
        label: 'Not found',
        title: 'Subsite not found',
        description:
          'The subsite link is incorrect, removed, or no longer available.',
      }
  }
}

function toneClasses(tone: StatusView['tone']) {
  switch (tone) {
    case 'success':
      return {
        icon: 'bg-success/10 text-success ring-success/20',
        badge: 'border-success/25 bg-success/10 text-success',
      }
    case 'warning':
      return {
        icon: 'bg-warning/15 text-warning ring-warning/25',
        badge: 'border-warning/30 bg-warning/15 text-warning-foreground',
      }
    case 'destructive':
      return {
        icon: 'bg-destructive/10 text-destructive ring-destructive/20',
        badge: 'border-destructive/25 bg-destructive/10 text-destructive',
      }
    default:
      return {
        icon: 'bg-muted text-muted-foreground ring-border',
        badge: 'border-border bg-muted text-muted-foreground',
      }
  }
}

function SubsiteLogo({ subsite }: { subsite?: PublicSubsite }) {
  const title = subsite?.title || subsite?.name || 'Data Proxy'
  const initial = title.trim().slice(0, 1).toUpperCase() || 'D'

  if (subsite?.logo_url) {
    return (
      <img
        src={subsite.logo_url}
        alt=''
        className='size-7 rounded-md object-cover'
      />
    )
  }

  return (
    <div className='bg-primary text-primary-foreground flex size-7 items-center justify-center rounded-md text-xs font-semibold'>
      {initial}
    </div>
  )
}

function SubsiteSkeleton({ slug }: SubsiteEntryProps) {
  return (
    <PublicLayout
      showAuthButtons={false}
      showNotifications={false}
      siteName={slug}
    >
      <div className='mx-auto flex min-h-[calc(100svh-8rem)] max-w-3xl items-center justify-center py-10'>
        <section className='w-full rounded-xl border p-5 md:p-6'>
          <div className='flex items-start gap-4'>
            <Skeleton className='size-12 rounded-lg' />
            <div className='min-w-0 flex-1 space-y-3'>
              <Skeleton className='h-5 w-28' />
              <Skeleton className='h-8 w-3/4' />
              <Skeleton className='h-4 w-full' />
              <Skeleton className='h-4 w-2/3' />
            </div>
          </div>
        </section>
      </div>
    </PublicLayout>
  )
}

function SubsiteStatusPage({
  subsite,
  slug,
  notFound = false,
}: SubsiteEntryProps & {
  subsite?: PublicSubsite
  notFound?: boolean
}) {
  const { t } = useTranslation()
  const runtimeStatus = notFound
    ? 'not_found'
    : subsite?.runtime_status || subsite?.access.status || 'disabled'
  const view = statusView(
    runtimeStatus,
    subsite?.disabled_reason || subsite?.access.message
  )
  const classes = toneClasses(view.tone)
  const Icon = view.icon
  const displayName = subsite?.title || subsite?.name || slug
  const themeColor = sanitizeThemeColor(subsite?.theme_color)
  const isOpen = subsite?.access.allowed === true

  const customAccentStyle = useMemo(
    () =>
      themeColor
        ? ({
            '--subsite-accent': themeColor,
          } as React.CSSProperties)
        : undefined,
    [themeColor]
  )

  return (
    <PublicLayout
      showAuthButtons={false}
      showNotifications={false}
      siteName={displayName}
      logo={<SubsiteLogo subsite={subsite} />}
      headerProps={{ homeUrl: `/s/${slug}` }}
    >
      <div className='mx-auto flex min-h-[calc(100svh-8rem)] max-w-3xl items-center justify-center py-10'>
        <section
          className='bg-card text-card-foreground w-full rounded-xl border p-5 md:p-6'
          style={customAccentStyle}
        >
          <div className='flex flex-col gap-5 md:flex-row md:items-start'>
            <div
              className={cn(
                'flex size-14 shrink-0 items-center justify-center rounded-xl ring-1',
                classes.icon
              )}
              style={
                themeColor && isOpen
                  ? {
                      backgroundColor:
                        'color-mix(in srgb, var(--subsite-accent) 12%, transparent)',
                      color: 'var(--subsite-accent)',
                    }
                  : undefined
              }
            >
              <Icon className='size-7' aria-hidden />
            </div>

            <div className='min-w-0 flex-1 space-y-5'>
              <div className='space-y-3'>
                <div className='flex flex-wrap items-center gap-2'>
                  <Badge variant='outline' className={classes.badge}>
                    {t(view.label)}
                  </Badge>
                  <span className='text-muted-foreground text-xs'>
                    /s/{slug}
                  </span>
                </div>
                <div className='space-y-2'>
                  <h1 className='text-wrap-balance text-2xl leading-tight font-semibold tracking-normal'>
                    {t(view.title)}
                  </h1>
                  <p className='text-muted-foreground text-wrap-pretty max-w-2xl text-sm'>
                    {t(view.description)}
                  </p>
                </div>
              </div>

              <div className='bg-muted/50 grid gap-3 rounded-lg border p-3 text-sm md:grid-cols-[8rem_1fr]'>
                <div className='text-muted-foreground'>{t('Subsite')}</div>
                <div className='min-w-0 font-medium break-words'>
                  {displayName}
                </div>
                {subsite?.starts_at ? (
                  <>
                    <div className='text-muted-foreground'>
                      {t('Starts at')}
                    </div>
                    <div>
                      {new Date(subsite.starts_at * 1000).toLocaleString()}
                    </div>
                  </>
                ) : null}
                {subsite?.ends_at ? (
                  <>
                    <div className='text-muted-foreground'>{t('Ends at')}</div>
                    <div>
                      {new Date(subsite.ends_at * 1000).toLocaleString()}
                    </div>
                  </>
                ) : null}
              </div>

              {subsite?.announcement_title || subsite?.announcement_body ? (
                <div className='rounded-lg border p-3'>
                  {subsite.announcement_title ? (
                    <h2 className='text-sm font-medium'>
                      {subsite.announcement_title}
                    </h2>
                  ) : null}
                  {subsite.announcement_body ? (
                    <p className='text-muted-foreground mt-1 text-sm whitespace-pre-wrap'>
                      {subsite.announcement_body}
                    </p>
                  ) : null}
                </div>
              ) : null}

              <div className='flex flex-col gap-2 sm:flex-row'>
                {isOpen ? (
                  <>
                    <Button
                      render={<Link to='/s/$slug/login' params={{ slug }} />}
                    >
                      <span>{t('Sign in')}</span>
                      <ArrowRight className='size-4' aria-hidden />
                    </Button>
                    <Button
                      variant='outline'
                      render={<Link to='/s/$slug/register' params={{ slug }} />}
                    >
                      {t('Create account')}
                    </Button>
                  </>
                ) : subsite?.contact_url ? (
                  <Button
                    variant='outline'
                    render={<a href={subsite.contact_url} />}
                  >
                    <span>{t('Contact administrator')}</span>
                    <ExternalLink className='size-4' aria-hidden />
                  </Button>
                ) : (
                  <Button variant='outline' render={<Link to='/' />}>
                    {t('Back to home')}
                  </Button>
                )}
              </div>
            </div>
          </div>
        </section>
      </div>
    </PublicLayout>
  )
}

function SubsiteAuthShell({
  subsite,
  slug,
  title,
  description,
  children,
}: SubsiteEntryProps & {
  subsite: PublicSubsite
  title: string
  description?: string
  children: React.ReactNode
}) {
  const { t } = useTranslation()
  const displayName = subsite.title || subsite.name || slug

  return (
    <PublicLayout
      showAuthButtons={false}
      showNotifications={false}
      siteName={displayName}
      logo={<SubsiteLogo subsite={subsite} />}
      headerProps={{ homeUrl: `/s/${slug}` }}
    >
      <div className='mx-auto flex min-h-[calc(100svh-8rem)] max-w-md items-center justify-center py-10'>
        <section className='bg-card text-card-foreground w-full space-y-6 rounded-xl border p-5 md:p-6'>
          <div className='space-y-2'>
            <Badge variant='outline'>{displayName}</Badge>
            <h1 className='text-2xl leading-tight font-semibold tracking-normal'>
              {t(title)}
            </h1>
            {description ? (
              <p className='text-muted-foreground text-sm'>{t(description)}</p>
            ) : null}
          </div>
          {children}
        </section>
      </div>
    </PublicLayout>
  )
}

export function SubsiteAuthPage({
  slug,
  mode,
}: SubsiteEntryProps & {
  mode: 'login' | 'register'
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { status } = useStatus()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['subsite-public', slug],
    queryFn: () => getPublicSubsite(slug),
    retry: false,
  })

  if (isLoading) {
    return <SubsiteSkeleton slug={slug} />
  }

  if (isError || !data?.success || !data.data) {
    return <SubsiteStatusPage slug={slug} notFound />
  }

  const subsite = data.data
  if (!subsite.access.allowed) {
    return <SubsiteStatusPage slug={slug} subsite={subsite} />
  }

  if (mode === 'register' && subsite.registration_policy === 'closed') {
    return (
      <SubsiteAuthShell
        slug={slug}
        subsite={subsite}
        title='Registration is closed'
        description='This subsite is not accepting new registrations.'
      >
        <Button
          variant='outline'
          render={<Link to='/s/$slug/login' params={{ slug }} />}
          className='w-full'
        >
          {t('Sign in')}
        </Button>
      </SubsiteAuthShell>
    )
  }

  if (mode === 'login') {
    return (
      <SubsiteAuthShell
        slug={slug}
        subsite={subsite}
        title='Sign in'
        description='Continue to your subsite console.'
      >
        <UserAuthForm redirectTo={`/s/${slug}/dashboard`} />
        {subsite.registration_policy !== 'closed' ? (
          <p className='text-muted-foreground text-sm'>
            {t("Don't have an account?")}{' '}
            <Link
              to='/s/$slug/register'
              params={{ slug }}
              className='hover:text-primary font-medium underline underline-offset-4'
            >
              {t('Sign up')}
            </Link>
            .
          </p>
        ) : null}
        <TermsFooter
          variant='sign-in'
          status={status}
          className='text-center'
        />
      </SubsiteAuthShell>
    )
  }

  return (
    <SubsiteAuthShell
      slug={slug}
      subsite={subsite}
      title='Create an account'
      description='Create a main account and join this subsite.'
    >
      <SignUpForm
        inviteCodeRequired={subsite.registration_policy === 'invite'}
        onRegister={(payload) => registerSubsiteUser(slug, payload)}
        onSuccess={() =>
          navigate({
            to: '/s/$slug/login',
            params: { slug },
            replace: true,
          })
        }
      />
      <p className='text-muted-foreground text-sm'>
        {t('Already have an account?')}{' '}
        <Link
          to='/s/$slug/login'
          params={{ slug }}
          className='hover:text-primary font-medium underline underline-offset-4'
        >
          {t('Sign in')}
        </Link>
        .
      </p>
      <TermsFooter variant='sign-up' status={status} className='text-center' />
    </SubsiteAuthShell>
  )
}

function SubsiteDashboardMessage({
  slug,
  subsite,
  title,
  description,
  icon,
}: SubsiteEntryProps & {
  subsite?: PublicSubsite
  title: string
  description: string
  icon: React.ReactNode
}) {
  const { t } = useTranslation()

  if (!subsite) {
    return (
      <PublicLayout
        showAuthButtons={false}
        showNotifications={false}
        siteName={slug}
        headerProps={{ homeUrl: `/s/${slug}` }}
      >
        <div className='mx-auto flex min-h-[calc(100svh-8rem)] max-w-md items-center justify-center py-10'>
          <section className='bg-card text-card-foreground w-full space-y-6 rounded-xl border p-5 md:p-6'>
            <div className='space-y-2'>
              <Badge variant='outline'>/s/{slug}</Badge>
              <h1 className='text-2xl leading-tight font-semibold tracking-normal'>
                {t(title)}
              </h1>
              <p className='text-muted-foreground text-sm'>{t(description)}</p>
            </div>
            <div className='bg-muted/50 flex items-center gap-3 rounded-lg border p-3 text-sm'>
              <div className='bg-background flex size-10 shrink-0 items-center justify-center rounded-lg border'>
                {icon}
              </div>
              <div className='min-w-0'>
                <div className='font-medium'>{t('Subsite dashboard')}</div>
                <div className='text-muted-foreground break-words'>
                  /s/{slug}
                </div>
              </div>
            </div>
          </section>
        </div>
      </PublicLayout>
    )
  }

  return (
    <SubsiteAuthShell
      slug={slug}
      subsite={subsite}
      title={title}
      description={description}
    >
      <div className='bg-muted/50 flex items-center gap-3 rounded-lg border p-3 text-sm'>
        <div className='bg-background flex size-10 shrink-0 items-center justify-center rounded-lg border'>
          {icon}
        </div>
        <div className='min-w-0'>
          <div className='font-medium'>{t('Subsite dashboard')}</div>
          <div className='text-muted-foreground break-words'>/s/{slug}</div>
        </div>
      </div>
    </SubsiteAuthShell>
  )
}

type DashboardQueryKey = readonly [
  'subsite-dashboard',
  string,
  number | undefined,
]

function formatCount(value: number) {
  if (!Number.isFinite(value)) return '0'
  return Math.max(0, Math.trunc(value)).toLocaleString()
}

function formatDashboardTime(timestamp?: number) {
  if (!timestamp || timestamp <= 0) return '-'
  return formatDateTimeObject(new Date(timestamp * 1000))
}

function formatWindowSeconds(seconds: number) {
  if (!seconds || seconds <= 0) return '24h'
  if (seconds % 86400 === 0) return `${seconds / 86400}d`
  if (seconds % 3600 === 0) return `${seconds / 3600}h`
  if (seconds % 60 === 0) return `${seconds / 60}m`
  return `${seconds}s`
}

function metricAmount(value: number, kind: 'quota' | 'requests') {
  return kind === 'quota' ? formatLogQuota(value) : formatCount(value)
}

function metricPercent(metric: SubsiteQuotaMetric) {
  if (!metric.limit || metric.limit <= 0) return 0
  return Math.min(100, Math.max(0, (metric.used / metric.limit) * 100))
}

function humanizeValue(value?: string) {
  if (!value) return '-'
  return value
    .split('_')
    .filter(Boolean)
    .map((part) => part.slice(0, 1).toUpperCase() + part.slice(1))
    .join(' ')
}

function DashboardPanel({
  title,
  description,
  action,
  children,
  className,
}: {
  title: string
  description?: string
  action?: React.ReactNode
  children: React.ReactNode
  className?: string
}) {
  const { t } = useTranslation()

  return (
    <section
      className={cn(
        'bg-card text-card-foreground rounded-xl border',
        className
      )}
    >
      <div className='flex flex-col gap-3 border-b p-4 sm:flex-row sm:items-start sm:justify-between'>
        <div className='min-w-0 space-y-1'>
          <h2 className='text-base leading-snug font-semibold'>{t(title)}</h2>
          {description ? (
            <p className='text-muted-foreground text-sm'>{t(description)}</p>
          ) : null}
        </div>
        {action ? <div className='shrink-0'>{action}</div> : null}
      </div>
      <div className='p-4'>{children}</div>
    </section>
  )
}

function DashboardStat({
  label,
  value,
  detail,
}: {
  label: string
  value: string
  detail: string
}) {
  const { t } = useTranslation()

  return (
    <div className='bg-background rounded-lg border p-3'>
      <div className='text-muted-foreground text-xs font-medium'>
        {t(label)}
      </div>
      <div className='mt-2 truncate text-lg leading-tight font-semibold tabular-nums'>
        {value}
      </div>
      <div className='text-muted-foreground mt-1 truncate text-xs'>
        {detail}
      </div>
    </div>
  )
}

function QuotaMetricTile({
  label,
  metric,
  kind,
}: {
  label: string
  metric: SubsiteQuotaMetric
  kind: 'quota' | 'requests'
}) {
  const { t } = useTranslation()
  const hasLimit = metric.limit > 0
  const percent = metricPercent(metric)
  const used = metricAmount(metric.used, kind)
  const limit = hasLimit ? metricAmount(metric.limit, kind) : t('Unlimited')
  const remaining = hasLimit
    ? metricAmount(metric.remaining, kind)
    : t('No configured limit')
  const resetAt = formatDashboardTime(metric.next_reset_time)

  return (
    <div className='bg-background rounded-lg border p-3'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='truncate text-sm font-medium'>{t(label)}</div>
          <div className='text-muted-foreground mt-1 text-xs'>
            {hasLimit ? t('Resets at') : t('Window')}:{' '}
            {hasLimit
              ? resetAt
              : formatWindowSeconds(metric.window_seconds || 86400)}
          </div>
        </div>
        <Badge variant='outline' className='shrink-0'>
          {hasLimit ? `${Math.round(percent)}%` : t('Unlimited')}
        </Badge>
      </div>
      <div className='mt-3 flex items-baseline gap-1.5'>
        <span className='font-mono text-base font-semibold tabular-nums'>
          {used}
        </span>
        <span className='text-muted-foreground text-xs'>/ {limit}</span>
      </div>
      <Progress
        value={percent}
        className='mt-3 gap-2 [&_[data-slot=progress-track]]:h-1.5'
      />
      <div className='text-muted-foreground mt-2 text-xs'>
        {hasLimit ? t('Remaining') : t('Limit')}: {remaining}
      </div>
    </div>
  )
}

function AnnouncementPanel({ subsite }: { subsite: PublicSubsite }) {
  const { t } = useTranslation()
  const hasAnnouncement = Boolean(
    subsite.announcement_title ||
    subsite.announcement_body ||
    subsite.announcement_url
  )

  return (
    <DashboardPanel
      title='Announcement'
      description='Latest notice for this subsite'
      action={
        <Megaphone className='text-muted-foreground size-4' aria-hidden />
      }
    >
      {hasAnnouncement ? (
        <div className='space-y-3'>
          {subsite.announcement_title ? (
            <h3 className='text-sm font-medium'>
              {subsite.announcement_title}
            </h3>
          ) : null}
          {subsite.announcement_body ? (
            <p className='text-muted-foreground max-w-3xl text-sm whitespace-pre-wrap'>
              {subsite.announcement_body}
            </p>
          ) : null}
          {subsite.announcement_url ? (
            <Button
              size='sm'
              variant='outline'
              render={<a href={subsite.announcement_url} />}
            >
              <span>{t('Open announcement')}</span>
              <ExternalLink className='size-4' aria-hidden />
            </Button>
          ) : null}
        </div>
      ) : (
        <p className='text-muted-foreground text-sm'>
          {t('No active announcement.')}
        </p>
      )}
    </DashboardPanel>
  )
}

function SubsiteApiAccessPanel({
  slug,
  dashboard,
  dashboardQueryKey,
}: {
  slug: string
  dashboard: SubsiteDashboardData
  dashboardQueryKey: DashboardQueryKey
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [revealedKey, setRevealedKey] = useState<string | null>(
    dashboard.token?.key ?? null
  )

  useEffect(() => {
    setRevealedKey(dashboard.token?.key ?? null)
  }, [dashboard.token?.id, dashboard.token?.key])

  const updateCachedToken = (token: SubsiteTokenInfo) => {
    queryClient.setQueryData<SubsiteDashboardResponse>(
      dashboardQueryKey,
      (current) =>
        current?.data
          ? {
              ...current,
              data: {
                ...current.data,
                token,
              },
            }
          : current
    )
  }

  const handleMutationError = (error: unknown) => {
    toast.error(error instanceof Error ? error.message : t('Request failed'))
  }

  const ensureTokenMutation = useMutation({
    mutationFn: () => ensureSubsiteToken(slug),
    onSuccess: (response) => {
      if (!response.success || !response.data?.token) {
        toast.error(response.message ?? t('Unable to create API key'))
        return
      }
      updateCachedToken(response.data.token)
      if (response.data.token.key) {
        setRevealedKey(response.data.token.key)
      }
      toast.success(
        response.data.created
          ? t('API key created')
          : t('API key already exists')
      )
    },
    onError: handleMutationError,
  })

  const revealTokenMutation = useMutation({
    mutationFn: () => getSubsiteTokenKey(slug),
    onSuccess: (response) => {
      if (!response.success || !response.data?.key) {
        toast.error(response.message ?? t('Unable to reveal API key'))
        return
      }
      updateCachedToken(response.data)
      setRevealedKey(response.data.key)
    },
    onError: handleMutationError,
  })

  const rotateTokenMutation = useMutation({
    mutationFn: () => rotateSubsiteToken(slug),
    onSuccess: (response) => {
      if (!response.success || !response.data?.token) {
        toast.error(response.message ?? t('Unable to rotate API key'))
        return
      }
      updateCachedToken(response.data.token)
      setRevealedKey(response.data.token.key ?? null)
      toast.success(t('API key rotated'))
    },
    onError: handleMutationError,
  })

  const token = dashboard.token
  const isBusy =
    ensureTokenMutation.isPending ||
    revealTokenMutation.isPending ||
    rotateTokenMutation.isPending
  const keyValue = revealedKey || token?.key || ''
  const keyDisplay = token
    ? keyValue || token.masked_key || '••••••••'
    : t('No API key created')

  const rotateKey = () => {
    if (
      !window.confirm(
        t(
          'Rotate this API key? The previous key will stop working immediately.'
        )
      )
    ) {
      return
    }
    rotateTokenMutation.mutate()
  }

  return (
    <DashboardPanel
      title='API access'
      description='Use this scoped endpoint and key for subsite traffic'
      action={
        token ? (
          <Badge variant='outline'>
            {token.status === 1 ? t('Enabled') : t('Disabled')}
          </Badge>
        ) : (
          <Badge variant='outline'>{t('No key')}</Badge>
        )
      }
    >
      <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]'>
        <div className='space-y-2'>
          <div className='text-muted-foreground text-xs font-medium'>
            {t('Base URL')}
          </div>
          <div className='flex min-w-0 items-center gap-2'>
            <code className='bg-muted/60 min-w-0 flex-1 truncate rounded-lg border px-3 py-2 font-mono text-xs'>
              {dashboard.base_url}
            </code>
            <CopyButton
              value={dashboard.base_url}
              variant='outline'
              size='icon'
              tooltip={t('Copy base URL')}
            />
          </div>
        </div>

        <div className='space-y-2'>
          <div className='flex items-center justify-between gap-2'>
            <div className='text-muted-foreground text-xs font-medium'>
              {t('API key')}
            </div>
            {token ? (
              <span className='text-muted-foreground text-xs'>
                {t('Created at')} {formatDashboardTime(token.created_time)}
              </span>
            ) : null}
          </div>
          <div className='flex min-w-0 flex-col gap-2 sm:flex-row'>
            <code
              className={cn(
                'bg-muted/60 min-w-0 flex-1 truncate rounded-lg border px-3 py-2 font-mono text-xs',
                !token && 'text-muted-foreground'
              )}
            >
              {keyDisplay}
            </code>
            <div className='flex shrink-0 gap-2'>
              {token ? (
                <>
                  <Button
                    size='icon'
                    variant='outline'
                    onClick={() =>
                      revealedKey
                        ? setRevealedKey(null)
                        : revealTokenMutation.mutate()
                    }
                    disabled={isBusy}
                    aria-label={revealedKey ? t('Hide key') : t('Show key')}
                  >
                    {revealedKey ? (
                      <EyeOff className='size-4' aria-hidden />
                    ) : (
                      <Eye className='size-4' aria-hidden />
                    )}
                  </Button>
                  {keyValue ? (
                    <CopyButton
                      value={keyValue}
                      variant='outline'
                      size='icon'
                      tooltip={t('Copy API key')}
                    />
                  ) : null}
                  <Button
                    size='icon'
                    variant='destructive'
                    onClick={rotateKey}
                    disabled={isBusy}
                    aria-label={t('Rotate key')}
                  >
                    <RefreshCw
                      className={cn(
                        'size-4',
                        rotateTokenMutation.isPending && 'animate-spin'
                      )}
                      aria-hidden
                    />
                  </Button>
                </>
              ) : (
                <Button
                  variant='default'
                  onClick={() => ensureTokenMutation.mutate()}
                  disabled={isBusy}
                >
                  <Plus className='size-4' aria-hidden />
                  <span>{t('Create key')}</span>
                </Button>
              )}
            </div>
          </div>
          <div className='text-muted-foreground text-xs'>
            {token
              ? `${t('Last used')}: ${formatDashboardTime(token.accessed_time)}`
              : t(
                  'Create a scoped key before sending requests to this subsite.'
                )}
          </div>
        </div>
      </div>
    </DashboardPanel>
  )
}

function SubsiteQuotaPanels({
  dashboard,
}: {
  dashboard: SubsiteDashboardData
}) {
  return (
    <div className='grid gap-5 xl:grid-cols-2'>
      <DashboardPanel
        title='Quota usage'
        description='Quota consumed by this subsite and by your account'
        action={<Gauge className='text-muted-foreground size-4' aria-hidden />}
      >
        <div className='grid gap-3 md:grid-cols-2'>
          <QuotaMetricTile
            label='Site daily quota'
            metric={dashboard.quota.site_daily_quota}
            kind='quota'
          />
          <QuotaMetricTile
            label='Site window quota'
            metric={dashboard.quota.site_window_quota}
            kind='quota'
          />
          <QuotaMetricTile
            label='User daily quota'
            metric={dashboard.quota.user_daily_quota}
            kind='quota'
          />
          <QuotaMetricTile
            label='User window quota'
            metric={dashboard.quota.user_window_quota}
            kind='quota'
          />
        </div>
      </DashboardPanel>

      <DashboardPanel
        title='Request limits'
        description='Request count limits for daily and rolling windows'
        action={<Gauge className='text-muted-foreground size-4' aria-hidden />}
      >
        <div className='grid gap-3 md:grid-cols-2'>
          <QuotaMetricTile
            label='Site daily requests'
            metric={dashboard.quota.site_daily_requests}
            kind='requests'
          />
          <QuotaMetricTile
            label='Site window requests'
            metric={dashboard.quota.site_window_requests}
            kind='requests'
          />
          <QuotaMetricTile
            label='User daily requests'
            metric={dashboard.quota.user_daily_requests}
            kind='requests'
          />
          <QuotaMetricTile
            label='User window requests'
            metric={dashboard.quota.user_window_requests}
            kind='requests'
          />
        </div>
      </DashboardPanel>
    </div>
  )
}

function RecentLogsTable({ logs }: { logs: SubsiteRecentLog[] }) {
  const { t } = useTranslation()

  if (logs.length === 0) {
    return (
      <EmptyState
        className='min-h-[180px]'
        title={t('No recent requests')}
        description={t('Requests made through this subsite will appear here.')}
      />
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Time')}</TableHead>
          <TableHead>{t('Model')}</TableHead>
          <TableHead>{t('Tokens')}</TableHead>
          <TableHead>{t('Fee')}</TableHead>
          <TableHead>{t('Latency')}</TableHead>
          <TableHead className='text-right'>{t('Status')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {logs.map((log) => (
          <TableRow key={log.id}>
            <TableCell className='text-muted-foreground'>
              {formatDashboardTime(log.created_at)}
            </TableCell>
            <TableCell className='max-w-[220px] truncate font-medium'>
              {log.model_name || '-'}
            </TableCell>
            <TableCell>
              <div className='space-y-0.5'>
                <div className='font-medium tabular-nums'>
                  {formatTokenVolume(log.total_tokens)} {t('total')}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {t('Input')} {formatTokenVolume(log.prompt_tokens)} ·{' '}
                  {t('Output')} {formatTokenVolume(log.completion_tokens)} ·{' '}
                  {t('Cache')} {formatTokenVolume(log.cache_tokens)} ·{' '}
                  {t('Reasoning')} {formatTokenVolume(log.reasoning_tokens)}
                </div>
              </div>
            </TableCell>
            <TableCell className='font-mono tabular-nums'>
              {formatLogQuota(log.quota)}
            </TableCell>
            <TableCell>{formatUseTime(log.use_time)}</TableCell>
            <TableCell className='text-right'>
              <Badge
                variant={log.status === 'error' ? 'destructive' : 'outline'}
                className={
                  log.status === 'error'
                    ? undefined
                    : 'border-success/25 bg-success/10 text-success'
                }
              >
                {log.status === 'error' ? t('Error') : t('Success')}
              </Badge>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function SubsiteDashboardLoading({ slug }: SubsiteEntryProps) {
  return (
    <PublicLayout
      showAuthButtons={false}
      showNotifications={false}
      siteName={slug}
      headerProps={{ homeUrl: `/s/${slug}` }}
    >
      <div className='mx-auto max-w-7xl space-y-5 py-2'>
        <div className='flex flex-col gap-4 border-b pb-5 md:flex-row md:items-start md:justify-between'>
          <div className='flex items-start gap-3'>
            <Skeleton className='size-10 rounded-lg' />
            <div className='space-y-2'>
              <Skeleton className='h-5 w-36' />
              <Skeleton className='h-4 w-56' />
            </div>
          </div>
          <Skeleton className='h-8 w-32' />
        </div>
        <Skeleton className='h-40 rounded-xl' />
        <div className='grid gap-3 md:grid-cols-4'>
          <Skeleton className='h-24 rounded-lg' />
          <Skeleton className='h-24 rounded-lg' />
          <Skeleton className='h-24 rounded-lg' />
          <Skeleton className='h-24 rounded-lg' />
        </div>
        <div className='grid gap-5 xl:grid-cols-2'>
          <Skeleton className='h-80 rounded-xl' />
          <Skeleton className='h-80 rounded-xl' />
        </div>
      </div>
    </PublicLayout>
  )
}

function SubsiteDashboardConsole({
  slug,
  dashboard,
  dashboardQueryKey,
}: SubsiteEntryProps & {
  dashboard: SubsiteDashboardData
  dashboardQueryKey: DashboardQueryKey
}) {
  const { t } = useTranslation()
  const displayName = dashboard.subsite.title || dashboard.subsite.name || slug
  const stats = dashboard.stats_24h

  return (
    <PublicLayout
      showAuthButtons={false}
      showNotifications={false}
      siteName={displayName}
      logo={<SubsiteLogo subsite={dashboard.subsite} />}
      headerProps={{ homeUrl: `/s/${slug}` }}
    >
      <div className='mx-auto max-w-7xl space-y-5 py-2'>
        <div className='flex flex-col gap-4 border-b pb-5 md:flex-row md:items-start md:justify-between'>
          <div className='flex min-w-0 items-start gap-3'>
            <div className='bg-muted flex size-10 shrink-0 items-center justify-center rounded-lg border'>
              <SubsiteLogo subsite={dashboard.subsite} />
            </div>
            <div className='min-w-0 space-y-1'>
              <div className='flex flex-wrap items-center gap-2'>
                <Badge variant='outline'>{displayName}</Badge>
                <Badge variant='outline'>
                  {t(humanizeValue(dashboard.member.role))}
                </Badge>
              </div>
              <h1 className='truncate text-xl leading-tight font-semibold tracking-normal'>
                {t('Subsite console')}
              </h1>
              <p className='text-muted-foreground max-w-3xl text-sm'>
                /s/{slug} · {t('Scoped API access, quota, and recent usage')}
              </p>
            </div>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Badge
              variant='outline'
              className={
                dashboard.subsite.runtime_status === 'enabled'
                  ? 'border-success/25 bg-success/10 text-success'
                  : undefined
              }
            >
              {t(humanizeValue(dashboard.subsite.runtime_status))}
            </Badge>
            <Badge variant='outline'>
              {dashboard.member.can_access
                ? t('Member active')
                : t('No access')}
            </Badge>
          </div>
        </div>

        <SubsiteApiAccessPanel
          slug={slug}
          dashboard={dashboard}
          dashboardQueryKey={dashboardQueryKey}
        />

        <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
          <DashboardStat
            label='Calls in 24h'
            value={formatCount(stats.calls)}
            detail={
              t('Last request') +
              ': ' +
              formatDashboardTime(stats.last_request_at)
            }
          />
          <DashboardStat
            label='Tokens in 24h'
            value={formatTokenVolume(stats.total_tokens)}
            detail={`${t('Input')} ${formatTokenVolume(stats.prompt_tokens)} · ${t(
              'Output'
            )} ${formatTokenVolume(stats.output_tokens)}`}
          />
          <DashboardStat
            label='Quota in 24h'
            value={formatLogQuota(stats.quota)}
            detail={t('Deducted from scoped usage')}
          />
          <DashboardStat
            label='Window'
            value={formatWindowSeconds(stats.window_seconds)}
            detail={t('Rolling usage summary')}
          />
        </div>

        <div className='grid gap-5 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]'>
          <AnnouncementPanel subsite={dashboard.subsite} />
          <DashboardPanel
            title='Member'
            description='Current account membership for this subsite'
            action={
              <KeyRound className='text-muted-foreground size-4' aria-hidden />
            }
          >
            <div className='grid gap-3 sm:grid-cols-3'>
              <div>
                <div className='text-muted-foreground text-xs'>{t('Role')}</div>
                <div className='mt-1 font-medium'>
                  {t(humanizeValue(dashboard.member.role))}
                </div>
              </div>
              <div>
                <div className='text-muted-foreground text-xs'>
                  {t('Status')}
                </div>
                <div className='mt-1 font-medium'>
                  {t(humanizeValue(dashboard.member.status))}
                </div>
              </div>
              <div>
                <div className='text-muted-foreground text-xs'>
                  {t('Manage')}
                </div>
                <div className='mt-1 font-medium'>
                  {dashboard.member.can_manage
                    ? t('Allowed')
                    : t('Member only')}
                </div>
              </div>
            </div>
          </DashboardPanel>
        </div>

        <SubsiteQuotaPanels dashboard={dashboard} />

        <DashboardPanel
          title='Recent requests'
          description='Latest subsite-scoped consume and error logs'
        >
          <RecentLogsTable logs={dashboard.recent_logs ?? []} />
        </DashboardPanel>
      </div>
    </PublicLayout>
  )
}

export function SubsiteDashboard({ slug }: SubsiteEntryProps) {
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.auth.user)
  const dashboardQueryKey = useMemo(
    () => ['subsite-dashboard', slug, user?.id] as const,
    [slug, user?.id]
  )
  const dashboardQuery = useQuery({
    queryKey: dashboardQueryKey,
    queryFn: () => getSubsiteDashboard(slug),
    enabled: Boolean(user),
    retry: false,
    staleTime: 30_000,
  })

  useEffect(() => {
    if (!user) {
      navigate({
        to: '/s/$slug/login',
        params: { slug },
        replace: true,
      })
    }
  }, [navigate, slug, user])

  if (!user || dashboardQuery.isLoading) {
    return <SubsiteDashboardLoading slug={slug} />
  }

  if (dashboardQuery.isError) {
    return (
      <PublicLayout
        showAuthButtons={false}
        showNotifications={false}
        siteName={slug}
        headerProps={{ homeUrl: `/s/${slug}` }}
      >
        <div className='mx-auto max-w-3xl py-10'>
          <ErrorState
            className='border'
            title='Unable to load subsite console'
            description='The dashboard request failed. Please retry after checking your session and network.'
            onRetry={() => void dashboardQuery.refetch()}
          />
        </div>
      </PublicLayout>
    )
  }

  if (!dashboardQuery.data?.success || !dashboardQuery.data.data) {
    const message =
      dashboardQuery.data?.message || 'Unable to open this subsite dashboard.'
    const normalized = message.toLowerCase()
    const title = normalized.includes('disabled')
      ? 'Membership disabled'
      : normalized.includes('quota') || normalized.includes('rate_limited')
        ? 'Quota exceeded'
        : normalized.includes('member')
          ? 'Membership required'
          : 'Console unavailable'

    return (
      <SubsiteDashboardMessage
        slug={slug}
        title={title}
        description={message}
        icon={<UserX className='text-destructive size-5' aria-hidden />}
      />
    )
  }

  if (!dashboardQuery.data.data.member.can_access) {
    return (
      <SubsiteDashboardMessage
        slug={slug}
        subsite={dashboardQuery.data.data.subsite}
        title='Membership disabled'
        description='Your access to this subsite has been disabled.'
        icon={<UserX className='text-warning size-5' aria-hidden />}
      />
    )
  }

  return (
    <SubsiteDashboardConsole
      slug={slug}
      dashboard={dashboardQuery.data.data}
      dashboardQueryKey={dashboardQueryKey}
    />
  )
}

export function SubsiteEntry({ slug }: SubsiteEntryProps) {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['subsite-public', slug],
    queryFn: () => getPublicSubsite(slug),
    retry: false,
  })

  if (isLoading) {
    return <SubsiteSkeleton slug={slug} />
  }

  if (isError || !data?.success || !data.data) {
    return <SubsiteStatusPage slug={slug} notFound />
  }

  return <SubsiteStatusPage slug={slug} subsite={data.data} />
}
