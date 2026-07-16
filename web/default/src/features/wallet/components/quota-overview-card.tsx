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
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { CreditCard, KeyRound, Package, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { getSelfQuotaOverview, type QuotaOverview } from '../api'

function formatTokens(value: number | undefined) {
  return new Intl.NumberFormat().format(value ?? 0)
}

function unitLabel(t: (key: string) => string, unit: string | undefined) {
  if (unit === 'llm_tokens') return t('LLM tokens')
  return t('Quota points')
}

function statusBadge(t: (key: string) => string, status: string | undefined) {
  if (status === 'empty') {
    return (
      <Badge variant='outline' className='text-muted-foreground'>
        {t('None')}
      </Badge>
    )
  }
  return (
    <Badge variant='outline' className='border-emerald-500/40 text-emerald-700'>
      {t('Available')}
    </Badge>
  )
}

function parseHref(href: string): { to: string; hash?: string } {
  const hashIndex = href.indexOf('#')
  if (hashIndex < 0) {
    return { to: href || '/wallet' }
  }
  const to = href.slice(0, hashIndex) || '/wallet'
  const hash = href.slice(hashIndex + 1)
  return hash ? { to, hash } : { to }
}

function OverviewTile(props: {
  icon: typeof WalletCards
  title: string
  primary: string
  secondary: string
  unit: string
  status?: string
  href: string
  cta: string
}) {
  const { t } = useTranslation()
  const Icon = props.icon
  const link = parseHref(props.href)
  return (
    <div className='flex min-h-[132px] flex-col justify-between rounded-lg border p-3 sm:p-4'>
      <div className='space-y-2'>
        <div className='flex items-center justify-between gap-2'>
          <div className='flex min-w-0 items-center gap-2'>
            <Icon className='text-muted-foreground size-4 shrink-0' />
            <div className='truncate text-sm font-medium'>{props.title}</div>
          </div>
          {statusBadge(t, props.status)}
        </div>
        <div className='text-xl font-semibold tracking-tight tabular-nums'>
          {props.primary}
        </div>
        <div className='text-muted-foreground text-xs'>{props.secondary}</div>
        <div className='text-muted-foreground text-[11px]'>
          {t('Unit')}: {props.unit}
        </div>
      </div>
      <div className='mt-3'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          render={
            <Link
              to={link.to}
              {...(link.hash ? { hash: link.hash } : {})}
            />
          }
        >
          {props.cta}
        </Button>
      </div>
    </div>
  )
}

export function QuotaOverviewCard(props: { compact?: boolean }) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['user', 'quota-overview'],
    queryFn: async (): Promise<QuotaOverview> => {
      const response = await getSelfQuotaOverview()
      if (!response.success || !response.data) {
        throw new Error(response.message || 'Failed to load quota overview')
      }
      return response.data
    },
  })

  if (query.isLoading) {
    return (
      <div className='overflow-hidden rounded-lg border'>
        <div className='border-b px-4 py-3'>
          <Skeleton className='h-4 w-32' />
          <Skeleton className='mt-2 h-3 w-64' />
        </div>
        <div className='grid gap-3 p-3 sm:grid-cols-2 xl:grid-cols-4'>
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className='h-[132px] w-full' />
          ))}
        </div>
      </div>
    )
  }

  if (query.isError || !query.data) {
    return (
      <div className='text-destructive rounded-lg border p-4 text-sm'>
        {query.error instanceof Error
          ? query.error.message
          : t('Failed to load quota overview')}
      </div>
    )
  }

  const data = query.data
  const topPackages = data.model_token_packages.top_packages ?? []

  return (
    <div id='quota-overview' className='scroll-mt-4 overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-start justify-between gap-3 border-b px-4 py-3'>
        <div className='min-w-0'>
          <h3 className='text-sm font-medium'>{t('Quota Overview')}</h3>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t(
              'Wallet, model token packages, subscriptions, and API key hard limits in one place. Units are not mixed.'
            )}
          </p>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            render={
              <a
                href='/docs/user-quickstart.md'
                target='_blank'
                rel='noopener noreferrer'
              />
            }
          >
            {t('3-minute setup')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => query.refetch()}
            disabled={query.isFetching}
          >
            {t('Refresh')}
          </Button>
        </div>
      </div>
      <div className='grid gap-3 p-3 sm:grid-cols-2 xl:grid-cols-4'>
        <OverviewTile
          icon={WalletCards}
          title={t('Wallet')}
          primary={formatQuota(data.wallet.quota)}
          secondary={`${t('Used')}: ${formatQuota(data.wallet.used_quota)} · ${t('Requests')}: ${data.wallet.request_count.toLocaleString()}`}
          unit={unitLabel(t, data.units.wallet)}
          status={data.wallet.status}
          href={data.links.wallet || '/wallet'}
          cta={t('Open wallet')}
        />
        <OverviewTile
          icon={Package}
          title={t('Model Token Packages')}
          primary={formatTokens(data.model_token_packages.remaining_tokens)}
          secondary={`${t('Active packages')}: ${data.model_token_packages.active_count} · ${t('Listed')}: ${data.model_token_packages.total_packages}`}
          unit={unitLabel(t, data.units.model_token_package)}
          status={data.model_token_packages.status}
          href={
            data.links.model_token_packages || '/wallet#model-token-packages'
          }
          cta={t('View packages')}
        />
        <OverviewTile
          icon={CreditCard}
          title={t('Subscriptions')}
          primary={formatQuota(data.subscriptions.remaining_quota)}
          secondary={`${t('Active')}: ${data.subscriptions.active_count} · ${t('Used')}: ${formatQuota(data.subscriptions.used_quota)}`}
          unit={unitLabel(t, data.units.subscription)}
          status={data.subscriptions.status}
          href={data.links.subscriptions || '/wallet#subscriptions'}
          cta={t('View subscriptions')}
        />
        <OverviewTile
          icon={KeyRound}
          title={t('API Key hard limits')}
          primary={formatQuota(data.api_key_hard_limits.remaining_quota)}
          secondary={`${t('Limited keys')}: ${data.api_key_hard_limits.limited_count}`}
          unit={unitLabel(t, data.units.api_key_hard_limit)}
          status={data.api_key_hard_limits.status}
          href={data.links.api_keys || '/keys'}
          cta={t('Manage keys')}
        />
      </div>
      {!props.compact && topPackages.length > 0 ? (
        <div className='border-t px-4 py-3'>
          <div className='text-muted-foreground mb-2 text-xs font-medium tracking-wide uppercase'>
            {t('Top packages')}
          </div>
          <div className='flex flex-wrap gap-2'>
            {topPackages.map((pkg) => (
              <Badge
                key={pkg.id}
                variant='outline'
                className='max-w-full truncate font-normal'
              >
                {(pkg.name || `#${pkg.id}`) +
                  ': ' +
                  formatTokens(pkg.remaining_tokens) +
                  ' / ' +
                  formatTokens(pkg.total_tokens)}
              </Badge>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  )
}
